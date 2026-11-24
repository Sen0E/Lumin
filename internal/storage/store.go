// Package storage persists configuration and history as one atomic snapshot.
package storage

import (
	"crypto/rand"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"sync/atomic"

	"lumin/internal/backup"
	"lumin/internal/config"
	"lumin/internal/domain"
	"lumin/internal/historyexport"
)

type Store struct {
	dir      string
	mu       sync.Mutex
	revision atomic.Uint64
	// Injectable atomic writer for deterministic commit-failure tests.
	write func(string, any) error
}

type Snapshot struct {
	Version int            `json:"version"`
	Config  config.Config  `json:"config"`
	History []domain.Entry `json:"history"`
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
	return &Store{dir: dir, write: writeJSON}, nil
}
func (s *Store) Dir() string      { return s.dir }
func (s *Store) Revision() uint64 { return s.revision.Load() }

// Legacy files are read only until the first successful write of state.json.
// Never fall back to legacy data if a current snapshot exists but is corrupt.
func (s *Store) snapshot() (Snapshot, error) {
	var snap Snapshot
	b, err := os.ReadFile(filepath.Join(s.dir, "state.json"))
	if err == nil {
		if err = json.Unmarshal(b, &snap); err != nil {
			return snap, err
		}
		if snap.Version != 1 {
			return snap, errors.New("不支持的存储版本")
		}
		return snap, snap.Config.Validate()
	}
	if !errors.Is(err, os.ErrNotExist) {
		return snap, err
	}
	snap = Snapshot{Version: 1, Config: config.Default()}
	for _, item := range []struct {
		name   string
		target any
	}{{"config.json", &snap.Config}, {"history.json", &snap.History}} {
		data, err := os.ReadFile(filepath.Join(s.dir, item.name))
		if errors.Is(err, os.ErrNotExist) {
			continue
		}
		if err != nil {
			return snap, err
		}
		if item.name == "config.json" {
			snap.Config = config.Config{}
		}
		if err = json.Unmarshal(data, item.target); err != nil {
			return snap, err
		}
	}
	return snap, snap.Config.Validate()
}
func (s *Store) Snapshot() (Snapshot, error) { s.mu.Lock(); defer s.mu.Unlock(); return s.snapshot() }
func (s *Store) LoadConfig() (config.Config, error) {
	snap, err := s.Snapshot()
	return snap.Config, err
}
func (s *Store) History() ([]domain.Entry, error) {
	snap, err := s.Snapshot()
	return snap.History, err
}
func (s *Store) save(snap Snapshot) error { return s.write(filepath.Join(s.dir, "state.json"), snap) }
func (s *Store) SaveConfig(c config.Config) error {
	if err := c.Validate(); err != nil {
		return err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	snap, err := s.snapshot()
	if err != nil {
		return err
	}
	snap.Config = c
	return s.save(snap)
}
func (s *Store) Add(entry domain.Entry, limit int) error { return s.add(entry, limit, nil) }

// AddIfRevision rejects results started before a successful restore.
func (s *Store) AddIfRevision(entry domain.Entry, limit int, revision uint64) error {
	return s.add(entry, limit, &revision)
}
func (s *Store) add(entry domain.Entry, limit int, revision *uint64) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if revision != nil && *revision != s.revision.Load() {
		return nil
	}
	snap, err := s.snapshot()
	if err != nil {
		return err
	}
	entries := append([]domain.Entry{entry}, snap.History...)
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
	snap.History = kept
	return s.save(snap)
}
func (s *Store) Favorite(id string, value bool) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	snap, err := s.snapshot()
	if err != nil {
		return err
	}
	for i := range snap.History {
		if snap.History[i].ID == id {
			snap.History[i].Favorite = value
			return s.save(snap)
		}
	}
	return errors.New("历史记录不存在")
}
func (s *Store) Clear() error {
	s.mu.Lock()
	defer s.mu.Unlock()
	snap, err := s.snapshot()
	if err != nil {
		return err
	}
	kept := []domain.Entry{}
	for _, e := range snap.History {
		if e.Favorite {
			kept = append(kept, e)
		}
	}
	snap.History = kept
	return s.save(snap)
}

// Restore writes an importable recovery backup before one atomic commit.
// Both writes run under the same lock as translation/history persistence.
func (s *Store) Restore(d backup.Document) (string, error) {
	if err := d.Validate(); err != nil {
		return "", err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	old, err := s.snapshot()
	if err != nil {
		return "", err
	}
	dir := filepath.Join(s.dir, "backups")
	if err = os.MkdirAll(dir, 0700); err != nil {
		return "", err
	}
	path := filepath.Join(dir, "before-restore-"+rand.Text()+".luminbackup")
	if err = backup.Write(path, backup.New(old.Config, old.History, true)); err != nil {
		return "", err
	}
	if err = s.save(Snapshot{Version: 1, Config: d.Config, History: d.History}); err != nil {
		return path, err
	}
	s.revision.Add(1)
	return path, nil
}
func writeJSON(path string, v any) error {
	data, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		return err
	}
	return backup.AtomicWrite(path, append(data, '\n'))
}

// Export cannot overwrite live files or plugin resources in the data directory.
func (s *Store) exportPath(path string) (string, error) {
	target, err := filepath.EvalSymlinks(filepath.Dir(path))
	if err != nil {
		return "", err
	}
	root, err := filepath.EvalSymlinks(s.dir)
	if err != nil {
		return "", err
	}
	target, err = filepath.Abs(filepath.Join(target, filepath.Base(path)))
	if err != nil {
		return "", err
	}
	root, err = filepath.Abs(root)
	if err != nil {
		return "", err
	}
	rel, err := filepath.Rel(root, target)
	if err != nil {
		if strings.EqualFold(filepath.VolumeName(root), filepath.VolumeName(target)) {
			return "", err
		}
		rel = ".."
	}
	if rel != ".." && !strings.HasPrefix(rel, ".."+string(filepath.Separator)) && filepath.Dir(rel) != "backups" {
		return "", errors.New("请将导出文件保存到数据目录之外或 backups 子目录")
	}
	return target, nil
}

func (s *Store) Export(path string, secrets bool) error {
	target, err := s.exportPath(path)
	if err != nil {
		return err
	}
	snap, err := s.Snapshot()
	if err != nil {
		return err
	}
	return backup.Write(target, backup.New(snap.Config, snap.History, secrets))
}

// Delete checks favorite protection under the same lock as the actual removal.
func (s *Store) Delete(id string, allowFavorite bool) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	snap, err := s.snapshot()
	if err != nil {
		return err
	}
	for i, e := range snap.History {
		if e.ID != id {
			continue
		}
		if e.Favorite && !allowFavorite {
			return errors.New("记录已收藏，请重新确认删除收藏")
		}
		snap.History = append(snap.History[:i], snap.History[i+1:]...)
		return s.save(snap)
	}
	return errors.New("历史记录不存在")
}

// ExportHistory writes the immutable UI snapshot, never live configuration.
func (s *Store) ExportHistory(path string, entries []domain.Entry, format string) error {
	target, err := s.exportPath(path)
	if err != nil {
		return err
	}
	data, err := historyexport.Encode(entries, format)
	if err != nil {
		return err
	}
	return backup.AtomicWrite(target, data)
}
