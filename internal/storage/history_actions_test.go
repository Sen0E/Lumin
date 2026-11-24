package storage

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"lumin/internal/domain"
)

func TestDeleteByIDAndFavoriteProtection(t *testing.T) {
	s := newStore(t)
	for _, id := range []string{"one", "two"} {
		if err := s.Add(validEntry(id), 100); err != nil {
			t.Fatal(err)
		}
	}
	if err := s.Favorite("one", true); err != nil {
		t.Fatal(err)
	}
	if err := s.Delete("one", false); err == nil {
		t.Fatal("deleted favorite without acknowledgement")
	}
	if err := s.Delete("two", false); err != nil {
		t.Fatal(err)
	}
	h, err := s.History()
	if err != nil || len(h) != 1 || h[0].ID != "one" || !h[0].Favorite {
		t.Fatal("wrong ID removed", h, err)
	}
	if err := s.Delete("missing", true); err == nil {
		t.Fatal("missing ID accepted")
	}
	if err := s.Delete("one", true); err != nil {
		t.Fatal(err)
	}
	reopened, err := New(s.dir)
	if err != nil {
		t.Fatal(err)
	}
	h, err = reopened.History()
	if err != nil || len(h) != 0 {
		t.Fatal("deletion not persisted")
	}
}
func TestFailedDeletePreservesData(t *testing.T) {
	s := newStore(t)
	if err := s.Add(validEntry("one"), 100); err != nil {
		t.Fatal(err)
	}
	original, err := os.ReadFile(filepath.Join(s.dir, "state.json"))
	if err != nil {
		t.Fatal(err)
	}
	s.write = func(string, any) error { return errors.New("disk failure") }
	if err := s.Delete("one", false); err == nil {
		t.Fatal("ignored commit failure")
	}
	current, _ := os.ReadFile(filepath.Join(s.dir, "state.json"))
	if string(current) != string(original) {
		t.Fatal("failed delete changed disk")
	}
}
func TestHistoryExportGuardAndNoConfiguration(t *testing.T) {
	s := newStore(t)
	c, err := s.LoadConfig()
	if err != nil {
		t.Fatal(err)
	}
	c.Providers[0].APIKey = "SECRET-KEY"
	c.ServerToken = "SECRET-TOKEN"
	if err = s.SaveConfig(c); err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{"state.json", "config.json", "plugins/model.onnx"} {
		if err = s.ExportHistory(filepath.Join(s.dir, name), []domain.Entry{validEntry("one")}, "JSON"); err == nil {
			t.Fatal("export into live directory allowed", name)
		}
	}
	path := filepath.Join(t.TempDir(), "export.json")
	if err = s.ExportHistory(path, []domain.Entry{validEntry("one")}, "JSON"); err != nil {
		t.Fatal(err)
	}
	data, _ := os.ReadFile(path)
	if strings.Contains(string(data), "SECRET") || strings.Contains(string(data), "providers") {
		t.Fatal("exported configuration")
	}
	original := string(data)
	if err = s.ExportHistory(path, nil, "invalid"); err == nil {
		t.Fatal("unknown format accepted")
	}
	data, _ = os.ReadFile(path)
	if string(data) != original {
		t.Fatal("failed export overwrote file")
	}
}
