package state

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/rbright/waybar-agent-usage/internal/domain"
)

var ErrNotFound = errors.New("cache not found")

type Snapshot struct {
	Provider  domain.Provider `json:"provider"`
	FetchedAt time.Time       `json:"fetched_at"`
	Metrics   domain.Metrics  `json:"metrics"`
}

type Store struct {
	dir string
}

func NewStore(dir string) *Store {
	return &Store{dir: dir}
}

func (s *Store) Load(provider domain.Provider) (*Snapshot, error) {
	path := s.pathFor(provider)
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, ErrNotFound
		}
		return nil, fmt.Errorf("read cache file %s: %w", path, err)
	}

	var snapshot Snapshot
	if err := json.Unmarshal(data, &snapshot); err != nil {
		return nil, fmt.Errorf("decode cache file %s: %w", path, err)
	}

	if snapshot.Provider == "" {
		snapshot.Provider = provider
	}
	return &snapshot, nil
}

func (s *Store) Save(provider domain.Provider, metrics domain.Metrics, fetchedAt time.Time) error {
	if err := os.MkdirAll(s.dir, 0o755); err != nil {
		return fmt.Errorf("create state dir %s: %w", s.dir, err)
	}

	snapshot := Snapshot{
		Provider:  provider,
		FetchedAt: fetchedAt.UTC(),
		Metrics:   metrics,
	}

	payload, err := json.MarshalIndent(snapshot, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal cache payload: %w", err)
	}
	payload = append(payload, '\n')

	tmpFile, err := os.CreateTemp(s.dir, string(provider)+"-*.json")
	if err != nil {
		return fmt.Errorf("create temp cache file: %w", err)
	}
	tmpPath := tmpFile.Name()
	defer func() {
		_ = os.Remove(tmpPath)
	}()

	if _, err := tmpFile.Write(payload); err != nil {
		_ = tmpFile.Close()
		return fmt.Errorf("write temp cache file: %w", err)
	}
	if err := tmpFile.Close(); err != nil {
		return fmt.Errorf("close temp cache file: %w", err)
	}
	if err := os.Chmod(tmpPath, 0o600); err != nil {
		return fmt.Errorf("chmod temp cache file: %w", err)
	}

	path := s.pathFor(provider)
	if err := os.Rename(tmpPath, path); err != nil {
		return fmt.Errorf("replace cache file %s: %w", path, err)
	}

	return nil
}

func (s *Store) pathFor(provider domain.Provider) string {
	return filepath.Join(s.dir, string(provider)+".json")
}
