package storage

import (
	"errors"
	"os"
	"path/filepath"
	"testing"
	"time"

	"lumin/internal/backup"
	"lumin/internal/config"
	"lumin/internal/domain"
)

func validEntry(id string) domain.Entry {
	return domain.Entry{ID: id, CreatedAt: time.Now().UTC(), Request: domain.Request{Text: id, Source: "en", Target: "zh"}}
}
func TestLegacyMigrationAndAtomicRestore(t *testing.T) {
	s := newStore(t)
	old := config.Default()
	old.Providers[0].APIKey = "old-key"
	if err := writeJSON(filepath.Join(s.dir, "config.json"), old); err != nil {
		t.Fatal(err)
	}
	if err := writeJSON(filepath.Join(s.dir, "history.json"), []domain.Entry{validEntry("old")}); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(s.dir, "state.json")); !os.IsNotExist(err) {
		t.Fatal("migration should be lazy")
	}
	c := config.Default()
	c.TimeoutSeconds = 23
	d := backup.New(c, []domain.Entry{validEntry("new")}, true)
	rev := s.Revision()
	path, err := s.Restore(d)
	if err != nil {
		t.Fatal(err)
	}
	recovery, err := backup.Read(path)
	if err != nil || recovery.Config.Providers[0].APIKey != "old-key" || recovery.History[0].ID != "old" {
		t.Fatal("invalid recovery", err)
	}
	// A canceled translator completing late cannot reintroduce pre-restore history.
	if err := s.AddIfRevision(validEntry("stale"), 500, rev); err != nil {
		t.Fatal(err)
	}
	reopened, err := New(s.dir)
	if err != nil {
		t.Fatal(err)
	}
	snap, err := reopened.Snapshot()
	if err != nil || snap.Config.TimeoutSeconds != 23 || len(snap.History) != 1 || snap.History[0].ID != "new" {
		t.Fatal("partial restore", snap, err)
	}
	if _, err := s.Restore(*recovery); err != nil {
		t.Fatal(err)
	}
	snap, _ = s.Snapshot()
	if snap.History[0].ID != "old" || snap.Config.Providers[0].APIKey != "old-key" {
		t.Fatal("recovery round trip failed")
	}
}
func TestRestoreFailurePreservesSnapshotAndRevision(t *testing.T) {
	s := newStore(t)
	if err := s.Add(validEntry("old"), 500); err != nil {
		t.Fatal(err)
	}
	before, _ := os.ReadFile(filepath.Join(s.dir, "state.json"))
	rev := s.Revision()
	s.write = func(string, any) error { return errors.New("simulated atomic commit failure") }
	path, err := s.Restore(backup.New(config.Default(), []domain.Entry{validEntry("new")}, true))
	if err == nil || path == "" {
		t.Fatal("missing failure/recovery")
	}
	after, _ := os.ReadFile(filepath.Join(s.dir, "state.json"))
	if string(after) != string(before) || s.Revision() != rev {
		t.Fatal("failed restore changed data")
	}
	if _, err := backup.Read(path); err != nil {
		t.Fatal("missing recovery", err)
	}
}
func TestRecoveryCreationFailureAndExportPathGuard(t *testing.T) {
	s := newStore(t)
	if err := s.Add(validEntry("old"), 500); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(s.dir, "backups"), []byte("block"), 0600); err != nil {
		t.Fatal(err)
	}
	if _, err := s.Restore(backup.New(config.Default(), nil, true)); err == nil {
		t.Fatal("restored without recovery")
	}
	h, _ := s.History()
	if len(h) != 1 || h[0].ID != "old" {
		t.Fatal("lost data")
	}
	if err := s.Export(filepath.Join(s.dir, "state.json"), true); err == nil {
		t.Fatal("export overwrote live file")
	}
	path := filepath.Join(t.TempDir(), "export.luminbackup")
	if err := s.Export(path, false); err != nil {
		t.Fatal(err)
	}
	if _, err := backup.Read(path); err != nil {
		t.Fatal(err)
	}
}
func TestCorruptSnapshotNeverFallsBackAndIncompleteLegacyRejected(t *testing.T) {
	s := newStore(t)
	if err := os.WriteFile(filepath.Join(s.dir, "config.json"), []byte(`{"version":1}`), 0600); err != nil {
		t.Fatal(err)
	}
	if _, err := s.LoadConfig(); err == nil {
		t.Fatal("accepted incomplete config")
	}
	if err := writeJSON(filepath.Join(s.dir, "config.json"), config.Default()); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(s.dir, "state.json"), []byte("broken"), 0600); err != nil {
		t.Fatal(err)
	}
	if _, err := s.LoadConfig(); err == nil {
		t.Fatal("fell back to stale config")
	}
	if err := s.SaveConfig(config.Default()); err == nil {
		t.Fatal("overwrote corrupt data")
	}
}
