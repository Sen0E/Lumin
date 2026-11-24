package backup

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"lumin/internal/config"
	"lumin/internal/domain"
)

func fixture() Document {
	c := config.Default()
	c.Providers[0].APIKey = "key-secret"
	c.Providers[0].Settings = map[string]string{"custom": "plugin-secret"}
	c.Providers[0].Endpoint = "https://user:pass@example.com/api?token=url-secret#private"
	c.ServerToken = "server-secret-long-enough-token"
	c.OCRSettings = map[string]string{"password": "ocr-secret"}
	return New(c, []domain.Entry{{ID: "one", CreatedAt: time.Now(), Request: domain.Request{Text: "hello", Source: "en", Target: "zh"}, Favorite: true}}, true)
}
func TestExportRedactsWithoutMutatingSource(t *testing.T) {
	full := fixture()
	redacted := New(full.Config, full.History, false)
	path := filepath.Join(t.TempDir(), "backup.luminbackup")
	if err := Write(path, redacted); err != nil {
		t.Fatal(err)
	}
	b, _ := os.ReadFile(path)
	for _, secret := range []string{"key-secret", "plugin-secret", "url-secret", "user:pass", "server-secret", "ocr-secret"} {
		if strings.Contains(string(b), secret) {
			t.Fatalf("leaked %s", secret)
		}
	}
	got, err := Read(path)
	if err != nil {
		t.Fatal(err)
	}
	if got.IncludesSecrets || got.Config.Providers[0].Enabled || got.Config.ServerEnabled || !got.History[0].Favorite {
		t.Fatal("redacted semantics")
	}
	if full.Config.Providers[0].APIKey != "key-secret" || full.Config.OCRSettings["password"] != "ocr-secret" {
		t.Fatal("modified live config")
	}
	if err := Write(path, full); err != nil {
		t.Fatal(err)
	}
	got, err = Read(path)
	if err != nil || got.Config.Providers[0].APIKey != "key-secret" {
		t.Fatal("full round trip", err)
	}
}
func TestInvalidImportAndFailedExportPreserveFile(t *testing.T) {
	d := fixture()
	path := filepath.Join(t.TempDir(), "data.luminbackup")
	if err := Write(path, d); err != nil {
		t.Fatal(err)
	}
	original, _ := os.ReadFile(path)
	for _, mutate := range []func(*Document){
		func(d *Document) { d.Version++ },
		func(d *Document) { d.History = append(d.History, d.History[0]) },
		func(d *Document) { d.Config.TimeoutSeconds = 0 },
		func(d *Document) { d.IncludesSecrets = false },
	} {
		bad := fixture()
		mutate(&bad)
		if err := Write(path, bad); err == nil {
			t.Fatal("accepted invalid document")
		}
		got, _ := os.ReadFile(path)
		if string(got) != string(original) {
			t.Fatal("failed export overwrote original")
		}
	}
	for _, content := range []string{"broken", string(original) + " {}", strings.Replace(string(original), `"version": 1`, `"version": 999`, 1), strings.Replace(string(original), `"format":`, `"path": "../../state.json", "format":`, 1)} {
		if err := os.WriteFile(path, []byte(content), 0600); err != nil {
			t.Fatal(err)
		}
		if _, err := Read(path); err == nil {
			t.Fatal("accepted corrupt/unknown input")
		}
	}
	f, err := os.Create(path)
	if err != nil {
		t.Fatal(err)
	}
	if err = f.Truncate(MaxBytes + 1); err != nil {
		t.Fatal(err)
	}
	f.Close()
	if _, err = Read(path); err == nil {
		t.Fatal("accepted oversized input")
	}
}
