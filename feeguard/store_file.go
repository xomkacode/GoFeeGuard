package feeguard

import (
	"os"
	"path/filepath"
	"strings"
	"time"
)

type FileStore struct {
	Path string
}

func (s FileStore) Load() (time.Time, error) {
	data, err := os.ReadFile(s.Path)
	if os.IsNotExist(err) {
		return time.Time{}, nil
	}
	if err != nil {
		return time.Time{}, err
	}
	return time.Parse(time.RFC3339Nano, strings.TrimSpace(string(data)))
}

func (s FileStore) Save(value time.Time) error {
	if err := os.MkdirAll(filepath.Dir(s.Path), 0700); err != nil {
		return err
	}
	return os.WriteFile(s.Path, []byte(value.UTC().Format(time.RFC3339Nano)), 0600)
}
