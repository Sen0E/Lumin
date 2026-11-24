package main

import (
	"archive/zip"
	"os"
	"path/filepath"
	"testing"
)

func TestPackOutsideSourceAndRejectNestedOutput(t *testing.T) {
	root := t.TempDir()
	dir := filepath.Join(root, "plugin")
	if err := os.MkdirAll(filepath.Join(dir, "bin"), 0755); err != nil {
		t.Fatal(err)
	}
	manifest := `{"schema":1,"id":"test.pack","name":"Test","version":"1.0.0","capabilities":["translate"],"entrypoints":{"windows/amd64":"bin/plugin.exe"}}`
	for name, content := range map[string]string{"plugin.json": manifest, "bin/plugin.exe": "entrypoint"} {
		if err := os.WriteFile(filepath.Join(dir, name), []byte(content), 0644); err != nil {
			t.Fatal(err)
		}
	}
	out := filepath.Join(root, "packages", "test.luminext")
	if err := pack(dir, out); err != nil {
		t.Fatal(err)
	}
	archive, err := zip.OpenReader(out)
	if err != nil {
		t.Fatal(err)
	}
	defer archive.Close()
	if len(archive.File) != 2 {
		t.Fatalf("archive files: %v", archive.File)
	}
	if err := pack(dir, out); err == nil {
		t.Fatal("existing versioned package was overwritten")
	}
	if err := pack(dir, filepath.Join(dir, "nested", "test.luminext")); err == nil {
		t.Fatal("accepted output inside plugin source")
	}
}
