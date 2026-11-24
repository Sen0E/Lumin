package ocrsetup

import (
	"archive/zip"
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"lumin/internal/config"
	"lumin/internal/plugin"
	"lumin/internal/storage"
)

func bundle(t *testing.T, dir string) string {
	t.Helper()
	path := filepath.Join(dir, "bundle.luminext")
	f, err := os.Create(path)
	if err != nil {
		t.Fatal(err)
	}
	z := zip.NewWriter(f)
	m := plugin.Manifest{Schema: 1, ID: "lumin.ppocrv6-tiny", Version: "1.0.0", Name: "Default OCR", Capabilities: []string{"recognize"}, Entrypoints: map[string]string{runtime.GOOS + "/" + runtime.GOARCH: "bin/plugin"}}
	w, err := z.Create("plugin.json")
	if err != nil {
		t.Fatal(err)
	}
	if err = json.NewEncoder(w).Encode(m); err != nil {
		t.Fatal(err)
	}
	w, err = z.Create("bin/plugin")
	if err != nil {
		t.Fatal(err)
	}
	w.Write([]byte("must never execute during installation"))
	z.Close()
	f.Close()
	return path
}
func TestDefaultProvisionPreservesPreferencesAndDisable(t *testing.T) {
	dir := t.TempDir()
	s, err := storage.New(filepath.Join(dir, "data"))
	if err != nil {
		t.Fatal(err)
	}
	m, err := plugin.New(filepath.Join(s.Dir(), "plugins"))
	if err != nil {
		t.Fatal(err)
	}
	cfg := config.Default()
	cfg.TimeoutSeconds = 23
	archive := bundle(t, dir)
	got, err := Ensure(s, m, cfg, archive)
	if err != nil {
		t.Fatal(err)
	}
	if got.OCRPlugin != DefaultRef || got.TimeoutSeconds != 23 {
		t.Fatal("default not configured", got)
	}
	saved, err := s.LoadConfig()
	if err != nil || saved.OCRPlugin != DefaultRef {
		t.Fatal("not persisted", err)
	}
	// Already installed default works even if a development run has no bundle.
	got, err = Ensure(s, m, cfg, "missing")
	if err != nil || got.OCRPlugin != DefaultRef {
		t.Fatal(err)
	}
	for _, c := range []config.Config{func() config.Config { c := cfg.Clone(); c.OCRDisabled = true; return c }(), func() config.Config { c := cfg.Clone(); c.OCRPlugin = "custom.ocr@1.0.0"; return c }()} {
		got, err = Ensure(s, m, c, "missing")
		if err != nil || got.OCRPlugin != c.OCRPlugin || got.OCRDisabled != c.OCRDisabled {
			t.Fatal("user choice replaced")
		}
	}
}
func TestMissingBundleDoesNotEnableSystemFallback(t *testing.T) {
	s, err := storage.New(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	m, err := plugin.New(filepath.Join(s.Dir(), "plugins"))
	if err != nil {
		t.Fatal(err)
	}
	cfg, err := Ensure(s, m, config.Default(), "missing")
	if err != nil || cfg.OCRPlugin != "" {
		t.Fatal("unexpected engine", err)
	}
}

func TestRealBundledDefaultRunsWithoutSystemTools(t *testing.T) {
	archive := os.Getenv("LUMIN_PPOCR_PACKAGE")
	if archive == "" {
		t.Skip("set LUMIN_PPOCR_PACKAGE for real bundled default")
	}
	s, err := storage.New(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	m, err := plugin.New(filepath.Join(s.Dir(), "plugins"))
	if err != nil {
		t.Fatal(err)
	}
	cfg, err := Ensure(s, m, config.Default(), archive)
	if err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", "")
	text, err := (plugin.Recognizer{Manager: m, Ref: cfg.OCRPlugin, Settings: cfg.OCRSettings}).Recognize(context.Background(), "../ocr/testdata/english.png", cfg.OCRLanguages)
	if err != nil || !strings.Contains(text, "Lumin translation test.") {
		t.Fatal("bundled default failed", text, err)
	}
}

func TestLegacyToolSelectionMovesToBundledEngine(t *testing.T) {
	dir := t.TempDir()
	s, err := storage.New(filepath.Join(dir, "data"))
	if err != nil {
		t.Fatal(err)
	}
	m, err := plugin.New(filepath.Join(s.Dir(), "plugins"))
	if err != nil {
		t.Fatal(err)
	}
	cfg := config.Default()
	cfg.OCRPlugin = LegacyToolRef
	cfg.OCRSettings = map[string]string{"languages": "eng"}
	got, err := Ensure(s, m, cfg, bundle(t, dir))
	if err != nil {
		t.Fatal(err)
	}
	if got.OCRPlugin != DefaultRef || got.OCRSettings["threads"] != "4" || got.OCRSettings["languages"] != "" {
		t.Fatal("legacy selection not migrated", got)
	}
}
