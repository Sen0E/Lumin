package plugin

import (
	"context"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

func TestRuntimeAcceptsLinkedStorageAncestor(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("test helper uses an extensionless entrypoint")
	}
	root := t.TempDir()
	actual := filepath.Join(root, "actual")
	if err := os.Mkdir(actual, 0700); err != nil {
		t.Fatal(err)
	}
	alias := filepath.Join(root, "alias")
	if err := os.Symlink(actual, alias); err != nil {
		t.Fatal(err)
	}
	manager, err := New(filepath.Join(alias, "plugins"))
	if err != nil {
		t.Fatal(err)
	}
	executable, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	bin, err := os.ReadFile(executable)
	if err != nil {
		t.Fatal(err)
	}
	manifest := testManifest()
	if _, err := manager.Install(archive(t, manifest, "", 0, bin)); err != nil {
		t.Fatal(err)
	}
	got, err := manager.Call(context.Background(), manifest.Ref(), Request{Operation: "translate", Text: "Hello", Source: "auto", Target: "zh"})
	if err != nil || got != "Hello translated" {
		t.Fatalf("plugin under linked storage ancestor: %q %v", got, err)
	}
}

func TestRuntimeRejectsLinksInsideInstalledPlugin(t *testing.T) {
	for _, directory := range []bool{false, true} {
		t.Run(map[bool]string{false: "entrypoint", true: "entry-directory"}[directory], func(t *testing.T) {
			manager, err := New(t.TempDir())
			if err != nil {
				t.Fatal(err)
			}
			manifest := testManifest()
			if _, err := manager.Install(archive(t, manifest, "", 0, []byte("unused"))); err != nil {
				t.Fatal(err)
			}
			entry := filepath.Join(manager.root, manifest.Ref(), "bin", "plugin-helper")
			if directory {
				entry = filepath.Dir(entry)
			}
			target := filepath.Join(t.TempDir(), "moved")
			if err := os.Rename(entry, target); err != nil {
				t.Fatal(err)
			}
			if err := os.Symlink(target, entry); err != nil {
				if runtime.GOOS == "windows" {
					t.Skipf("symlink creation requires privileges: %v", err)
				}
				t.Fatal(err)
			}
			_, err = manager.Call(context.Background(), manifest.Ref(), Request{Operation: "translate", Text: "Hello", Source: "auto", Target: "zh"})
			if err == nil || !strings.Contains(err.Error(), "符号链接") {
				t.Fatalf("installed plugin link was not rejected: %v", err)
			}
		})
	}
}
