// Package storage persists configuration and history with atomic replacement.
package storage

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sync"

	"lumin/internal/config"
	"lumin/internal/domain"
)

type Store struct {
	dir string
	mu  sync.Mutex
}

func DefaultDir() (string, error) {
	if dir := os.Getenv("LUMIN_CONFIG_DIR"); dir != "" {
		return filepath.Abs(dir)
	}
	dir, err := os.UserConfigDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, "lumin"), nil
}

func New(dir string) (*Store, error) {
	if err := os.MkdirAll(dir, 0700); err != nil {
		return nil, err
	}
	return &Store{dir: dir}, nil
}

func (s *Store) Dir() string { return s.dir }

func (s *Store) LoadConfig() (config.Config, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	c := config.Default()
	data, err := os.ReadFile(filepath.Join(s.dir, "config.json"))
	if errors.Is(err, os.ErrNotExist) {
		return c, nil
	}
	if err != nil {
		return c, err
	}
	// Decode into a zero value: never silently upgrade unknown/incomplete files.
	var loaded config.Config
	if err := json.Unmarshal(data, &loaded); err != nil {
		return c, fmt.Errorf("config.json 无法解析（原文件已保留）：%w", err)
	}
	if err := loaded.Validate(); err != nil {
		return c, err
	}
	return loaded, nil
}

func (s *Store) SaveConfig(c config.Config) error {
	if err := c.Validate(); err != nil {
		return err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	return writeJSON(filepath.Join(s.dir, "config.json"), c)
}

func (s *Store) History() ([]domain.Entry, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.history()
}

func (s *Store) history() ([]domain.Entry, error) {
	data, err := os.ReadFile(filepath.Join(s.dir, "history.json"))
	if errors.Is(err, os.ErrNotExist) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	var entries []domain.Entry
	if err := json.Unmarshal(data, &entries); err != nil {
		return nil, fmt.Errorf("history.json 无法解析（原文件已保留）：%w", err)
	}
	return entries, nil
}

// Add bounds ordinary history. Favorites are retained independently of that bound.
func (s *Store) Add(entry domain.Entry, limit int) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	entries, err := s.history()
	if err != nil {
		return err
	}
	entries = append([]domain.Entry{entry}, entries...)
	kept := make([]domain.Entry, 0, len(entries))
	ordinary := 0
	for _, e := range entries {
		if e.Favorite {
			kept = append(kept, e)
		} else if ordinary < limit {
			kept = append(kept, e)
			ordinary++
		}
	}
	return writeJSON(filepath.Join(s.dir, "history.json"), kept)
}

func (s *Store) Favorite(id string, value bool) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	entries, err := s.history()
	if err != nil {
		return err
	}
	for i := range entries {
		if entries[i].ID == id {
			entries[i].Favorite = value
			return writeJSON(filepath.Join(s.dir, "history.json"), entries)
		}
	}
	return errors.New("历史记录不存在")
}

// Clear removes ordinary history while preserving explicitly saved vocabulary.
func (s *Store) Clear() error {
	s.mu.Lock()
	defer s.mu.Unlock()
	entries, err := s.history()
	if err != nil {
		return err
	}
	kept := []domain.Entry{}
	for _, e := range entries {
		if e.Favorite {
			kept = append(kept, e)
		}
	}
	return writeJSON(filepath.Join(s.dir, "history.json"), kept)
}

func writeJSON(path string, v any) (err error) {
	data, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		return err
	}
	f, err := os.CreateTemp(filepath.Dir(path), ".lumin-*.tmp")
	if err != nil {
		return err
	}
	defer func() { _ = f.Close(); _ = os.Remove(f.Name()) }()
	if _, err = f.Write(append(data, '\n')); err != nil {
		return err
	}
	if err = f.Sync(); err != nil {
		return err
	}
	if err = f.Close(); err != nil {
		return err
	}
	return os.Rename(f.Name(), path)
}
