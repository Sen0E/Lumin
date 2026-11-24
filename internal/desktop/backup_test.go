package desktop

import (
	"image/png"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/egoist/mygo/ui"
	"lumin/internal/backup"
	"lumin/internal/config"
	"lumin/internal/domain"
)

func TestBackupPreviewConfirmationAndRestore(t *testing.T) {
	a, q := testApp(t)
	a.settings.Providers[0].APIKey = "old-key"
	if err := a.store.SaveConfig(a.settings); err != nil {
		t.Fatal(err)
	}
	cfg := config.Default()
	cfg.TimeoutSeconds = 17
	d := backup.New(cfg, []domain.Entry{{ID: "restored", CreatedAt: time.Now(), Request: domain.Request{Text: "hello", Source: "en", Target: "zh"}, Favorite: true}}, true)
	path := filepath.Join(t.TempDir(), "test.luminbackup")
	if err := backup.Write(path, d); err != nil {
		t.Fatal(err)
	}
	a.ports.OpenBackup = func() (string, error) { return path, nil }
	a.previewBackup()
	drainUntil(t, q, func() bool { return !a.backup.busy })
	if a.backup.preview == nil || a.settings.TimeoutSeconds == 17 {
		t.Fatal("preview applied data")
	}
	a.restoreBackup()
	if a.backup.restoring {
		t.Fatal("restored without confirmation")
	}
	// Restore exactly the validated preview, not a subsequently replaced file.
	if err := os.WriteFile(path, []byte("changed after preview"), 0600); err != nil {
		t.Fatal(err)
	}
	a.draft.TimeoutSeconds = 99
	a.page = "备份设置"
	tt := ui.NewTester(a.View, 920, 680)
	click(t, tt, "确认覆盖当前配置、全部历史和未保存草稿")
	applied := 0
	a.ports.ApplyConfig = func(c config.Config) error { applied++; return nil }
	click(t, tt, "覆盖并恢复")
	a.Translate() // Entry points must not start new work during the transaction.
	if a.busy {
		t.Fatal("translation started during restore")
	}
	drainUntil(t, q, func() bool { return !a.backup.busy })
	if a.settings.TimeoutSeconds != 17 || a.draft.TimeoutSeconds != 17 || len(a.history) != 1 || !a.history[0].Favorite || applied != 1 || a.backup.preview != nil {
		t.Fatal("restore not reflected in UI", a.backup.status)
	}
	paths, err := filepath.Glob(filepath.Join(a.store.Dir(), "backups", "*.luminbackup"))
	if err != nil || len(paths) != 1 {
		t.Fatal("missing recovery")
	}
	old, err := backup.Read(paths[0])
	if err != nil || old.Config.Providers[0].APIKey != "old-key" {
		t.Fatal("lost old credentials")
	}
}
func TestBackupExportDefaultsAndInvalidPreview(t *testing.T) {
	a, q := testApp(t)
	a.settings.Providers[0].APIKey = "secret"
	if err := a.store.SaveConfig(a.settings); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(t.TempDir(), "export.luminbackup")
	a.ports.SaveBackup = func() (string, error) { return path, nil }
	a.exportBackup()
	drainUntil(t, q, func() bool { return !a.backup.busy })
	d, err := backup.Read(path)
	if err != nil || d.Config.Providers[0].APIKey != "" || d.IncludesSecrets {
		t.Fatal("default export leaks keys", err)
	}
	a.backup.preview = d
	a.backup.confirmed = true
	if err := os.WriteFile(path, []byte("bad"), 0600); err != nil {
		t.Fatal(err)
	}
	a.ports.OpenBackup = func() (string, error) { return path, nil }
	a.previewBackup()
	drainUntil(t, q, func() bool { return !a.backup.busy })
	if a.backup.preview != nil || a.backup.confirmed {
		t.Fatal("old preview survived bad selection")
	}
	if a.settings.Providers[0].APIKey != "secret" {
		t.Fatal("preview modified settings")
	}
}
func TestBackupPageLayout(t *testing.T) {
	a, _ := testApp(t)
	a.page = "备份设置"
	d := backup.New(config.Default(), nil, false)
	a.backup.preview = &d
	a.backup.filename = "Lumin-test.luminbackup"
	a.ports.OpenBackup = func() (string, error) { return "", nil }
	a.ports.SaveBackup = func() (string, error) { return "", nil }
	for _, size := range []struct{ w, h int }{{1100, 800}, {920, 680}} {
		tt := ui.NewTester(a.View, size.w, size.h)
		tt.Frame()
		tt.Frame()
		for _, label := range []string{"导出备份文件", "选择备份并预览"} {
			b, ok := tt.Find(label)
			if !ok || b.X < 0 || b.Y < 0 || b.X+b.W > float32(size.w) || b.Y+b.H > float32(size.h) {
				t.Fatalf("%s out of bounds %+v", label, b)
			}
		}
		if dir := os.Getenv("LUMIN_TEST_SCREENSHOTS"); dir != "" && size.w == 920 {
			if err := os.MkdirAll(dir, 0755); err != nil {
				t.Fatal(err)
			}
			f, err := os.Create(filepath.Join(dir, "backup-preview.png"))
			if err != nil {
				t.Fatal(err)
			}
			err = png.Encode(f, tt.Image())
			f.Close()
			if err != nil {
				t.Fatal(err)
			}
		}
		a.backup.preview = nil
		tt.Frame()
		a.backup.preview = &d
		tt.Frame()
	}
}
