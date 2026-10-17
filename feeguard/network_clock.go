package feeguard

import (
	"context"
	"crypto/tls"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"
)

var ErrNoTimeQuorum = errors.New("not enough trusted time sources agree")

var defaultTimeSources = []string{
	"https://www.google.com/generate_204",
	"https://www.cloudflare.com/",
	"https://www.wikipedia.org/",
}

type NetworkClock struct {
	sources   []string
	client    *http.Client
	quorum    int
	tolerance time.Duration
}

func NewNetworkClock(sources ...string) (*NetworkClock, error) {
	if len(sources) == 0 {
		sources = append([]string(nil), defaultTimeSources...)
	}
	seen := make(map[string]struct{}, len(sources))
	for _, raw := range sources {
		u, err := url.Parse(raw)
		if err != nil || u.Scheme != "https" || u.Host == "" {
			return nil, fmt.Errorf("time source must be an HTTPS URL: %q", raw)
		}
		host := strings.ToLower(u.Host)
		if _, exists := seen[host]; exists {
			return nil, fmt.Errorf("duplicate time source: %q", raw)
		}
		seen[host] = struct{}{}
	}
	if len(sources) < 2 {
		return nil, errors.New("at least two independent HTTPS time sources are required")
	}
	return &NetworkClock{
		sources: sources,
		quorum:  2,
		tolerance: 2 * time.Minute,
		client: &http.Client{
			Timeout: 4 * time.Second,
			Transport: &http.Transport{TLSClientConfig: &tls.Config{MinVersion: tls.VersionTLS12}},
			CheckRedirect: func(req *http.Request, via []*http.Request) error {
				if req.URL.Scheme != "https" {
					return errors.New("refusing non-HTTPS time source redirect")
				}
				if len(via) >= 3 {
					return errors.New("too many redirects")
				}
				return nil
			},
		},
	}, nil
}

func (c *NetworkClock) Now(ctx context.Context) (time.Time, error) {
	type result struct {
		at  time.Time
		err error
	}

	results := make(chan result, len(c.sources))
	var wg sync.WaitGroup
	for _, source := range c.sources {
		wg.Add(1)
		go func(endpoint string) {
			defer wg.Done()
			at, err := c.fetch(ctx, endpoint)
			results <- result{at: at, err: err}
		}(source)
	}
	go func() {
		wg.Wait()
		close(results)
	}()

	var times []time.Time
	for result := range results {
		if result.err == nil {
			times = append(times, result.at)
		}
	}
	if len(times) < c.quorum {
		return time.Time{}, fmt.Errorf("%w: received %d of %d responses", ErrNoTimeQuorum, len(times), c.quorum)
	}
	for _, candidate := range times {
		agree := 0
		for _, other := range times {
			if absDuration(candidate.Sub(other)) <= c.tolerance {
				agree++
			}
		}
		if agree >= c.quorum {
			return candidate, nil
		}
	}
	return time.Time{}, ErrNoTimeQuorum
}

func (c *NetworkClock) fetch(ctx context.Context, endpoint string) (time.Time, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodHead, endpoint, nil)
	if err != nil {
		return time.Time{}, err
	}
	resp, err := c.client.Do(req)
	if err != nil {
		return time.Time{}, err
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 400 {
		return time.Time{}, fmt.Errorf("unexpected status %s", resp.Status)
	}
	date, err := http.ParseTime(resp.Header.Get("Date"))
	if err != nil {
		return time.Time{}, fmt.Errorf("invalid Date header: %w", err)
	}
	return date.UTC(), nil
}

func absDuration(value time.Duration) time.Duration {
	if value < 0 {
		return -value
	}
	return value
}
