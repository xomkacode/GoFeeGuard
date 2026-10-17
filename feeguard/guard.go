package feeguard

import (
	"context"
	"errors"
	"fmt"
	"time"
)

var (
	ErrExpired       = errors.New("license period has expired")
	ErrClockRollback = errors.New("observed time moved backwards")
)

type Guard struct {
	expiresAt time.Time
	source    TimeSource
	store     Store
	rollback  time.Duration
}

type Option func(*Guard)

type TimeSource interface {
	Now(context.Context) (time.Time, error)
}

type Store interface {
	Load() (time.Time, error)
	Save(time.Time) error
}

type systemClock struct{}

func (systemClock) Now(context.Context) (time.Time, error) { return time.Now(), nil }

func New(expirationDate string, options ...Option) (*Guard, error) {
	date, err := time.ParseInLocation("2006-01-02", expirationDate, time.Local)
	if err != nil {
		return nil, fmt.Errorf("parse expiration date: %w", err)
	}
	g := &Guard{expiresAt: date.AddDate(0, 0, 1).Add(-time.Nanosecond), source: systemClock{}, rollback: 2 * time.Minute}
	for _, option := range options {
		if option != nil {
			option(g)
		}
	}
	return g, nil
}

func WithTimeSource(source TimeSource) Option {
	return func(g *Guard) {
		if source != nil {
			g.source = source
		}
	}
}

func WithStore(store Store) Option { return func(g *Guard) { g.store = store } }

func WithRollbackTolerance(tolerance time.Duration) Option {
	return func(g *Guard) {
		if tolerance >= 0 {
			g.rollback = tolerance
		}
	}
}

func (g *Guard) Check() error { return g.CheckContext(context.Background()) }

func (g *Guard) Active(ctx context.Context) bool {
	return g.CheckContext(ctx) == nil
}

func (g *Guard) CheckContext(ctx context.Context) error {
	now, err := g.source.Now(ctx)
	if err != nil {
		return fmt.Errorf("get current time: %w", err)
	}
	if now.After(g.expiresAt) {
		return ErrExpired
	}
	if g.store == nil {
		return nil
	}
	last, err := g.store.Load()
	if err != nil {
		return fmt.Errorf("load license timestamp: %w", err)
	}
	if !last.IsZero() && now.Add(g.rollback).Before(last) {
		return ErrClockRollback
	}
	if last.IsZero() || now.After(last) {
		if err := g.store.Save(now); err != nil {
			return fmt.Errorf("save license timestamp: %w", err)
		}
	}
	return nil
}

func (g *Guard) ExpiresAt() time.Time { return g.expiresAt }
