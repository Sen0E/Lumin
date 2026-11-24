package storage

import (
	"os"
	"path/filepath"
	"runtime"
	"sync"
	"testing"

	"lumin/internal/config"
	"lumin/internal/domain"
)

func newStore(t *testing.T) *Store {
	t.Helper()
	s, err := New(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	return s
}

func TestConfigRoundTripAndPermissions(t *testing.T) {
	s := newStore(t)
	c := config.Default()
	c.Providers[0].APIKey = "sensitive"
	if err := s.SaveConfig(c); err != nil {
		t.Fatal(err)
	}
	got, err := s.LoadConfig()
	if err != nil || got.Providers[0].APIKey != "sensitive" {
		t.Fatalf("%+v %v", got, err)
	}
	info, err := os.Stat(filepath.Join(s.dir, "state.json"))
	if err != nil {
		t.Fatal(err)
	}
	if runtime.GOOS != "windows" && info.Mode().Perm() != 0600 {
		t.Fatalf("mode %v", info.Mode())
	}
}

func TestCorruptDataIsNotOverwritten(t *testing.T) {
	s := newStore(t)
	for _, name := range []string{"config.json", "history.json"} {
		if err := os.WriteFile(filepath.Join(s.dir, name), []byte("broken"), 0600); err != nil {
			t.Fatal(err)
		}
	}
	if _, err := s.LoadConfig(); err == nil {
		t.Fatal("accepted corrupt configuration")
	}
	if err := s.Add(domain.Entry{ID: "one"}, 5); err == nil {
		t.Fatal("overwrote corrupt history")
	}
	b, _ := os.ReadFile(filepath.Join(s.dir, "history.json"))
	if string(b) != "broken" {
		t.Fatal("lost corrupt history")
	}
}

func TestHistoryRetentionAndFavorites(t *testing.T) {
	s := newStore(t)
	for _, id := range []string{"saved", "old", "new"} {
		if err := s.Add(domain.Entry{ID: id, Favorite: id == "saved"}, 1); err != nil {
			t.Fatal(err)
		}
	}
	h, err := s.History()
	if err != nil || len(h) != 2 || h[0].ID != "new" || h[1].ID != "saved" {
		t.Fatalf("%+v %v", h, err)
	}
	if err := s.Clear(); err != nil {
		t.Fatal(err)
	}
	h, _ = s.History()
	if len(h) != 1 || h[0].ID != "saved" {
		t.Fatalf("%+v", h)
	}
	if err := s.Favorite("saved", false); err != nil {
		t.Fatal(err)
	}
	if err := s.Clear(); err != nil {
		t.Fatal(err)
	}
	h, _ = s.History()
	if len(h) != 0 {
		t.Fatal("history not cleared")
	}
}

func TestConcurrentHistoryWrites(t *testing.T) {
	s := newStore(t)
	var wg sync.WaitGroup
	for i := 0; i < 20; i++ {
		wg.Go(func() {
			if err := s.Add(domain.Entry{ID: "entry"}, 50); err != nil {
				t.Error(err)
			}
		})
	}
	wg.Wait()
	h, err := s.History()
	if err != nil || len(h) != 20 {
		t.Fatalf("lost writes: %d %v", len(h), err)
	}
}
