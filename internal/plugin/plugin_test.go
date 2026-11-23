package plugin

import (
	"archive/zip"
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"

	"lumin/pkg/pluginapi"
)

func TestMain(m *testing.M) {
	if strings.HasPrefix(filepath.Base(os.Args[0]), "plugin-helper") {
		_ = pluginapi.Serve(func(ctx context.Context, r pluginapi.Request) (string, error) {
			switch r.Settings["mode"] {
			case "sleep":
				time.Sleep(10 * time.Second)
			case "noise":
				os.Stdout.WriteString("not JSON\n")
			case "overflow":
				os.Stdout.WriteString(strings.Repeat("x", (2<<20)+1))
			case "wrong-id":
				_ = json.NewEncoder(os.Stdout).Encode(Response{Protocol: 1, ID: "wrong", Text: "x"})
				os.Exit(0)
			}
			return r.Text + " translated", nil
		})
		os.Exit(0)
	}
	os.Exit(m.Run())
}

func testManifest() Manifest {
	return Manifest{Schema: 1, ID: "test.plugin", Name: "Test", Version: "1.0.0", Capabilities: []string{"translate"}, Entrypoints: map[string]string{runtime.GOOS + "/" + runtime.GOARCH: "bin/plugin-helper"}, Fields: []Field{{Key: "mode", Label: "Mode"}}}
}
func archive(t *testing.T, m Manifest, extra string, mode os.FileMode, executable []byte) string {
	t.Helper()
	file := filepath.Join(t.TempDir(), "test.luminext")
	f, err := os.Create(file)
	if err != nil {
		t.Fatal(err)
	}
	z := zip.NewWriter(f)
	data, _ := json.Marshal(m)
	for _, e := range []struct {
		name string
		data []byte
		mode os.FileMode
	}{{"plugin.json", data, 0600}, {"bin/plugin-helper", executable, 0700}, {extra, []byte("extra"), mode}} {
		if e.name == "" {
			continue
		}
		h := &zip.FileHeader{Name: e.name, Method: zip.Deflate}
		h.SetMode(e.mode)
		w, err := z.CreateHeader(h)
		if err != nil {
			t.Fatal(err)
		}
		if _, err = w.Write(e.data); err != nil {
			t.Fatal(err)
		}
	}
	if err := z.Close(); err != nil {
		t.Fatal(err)
	}
	if err := f.Close(); err != nil {
		t.Fatal(err)
	}
	return file
}
func TestArchiveValidationAndVersionedInstall(t *testing.T) {
	m := testManifest()
	for _, entry := range []struct {
		name string
		mode os.FileMode
	}{{"../escape", 0600}, {"/absolute", 0600}, {"bin\\escape", 0600}, {"PLUGIN.JSON", 0600}, {"link", os.ModeSymlink | 0777}} {
		if _, err := Inspect(archive(t, m, entry.name, entry.mode, nil)); err == nil {
			t.Fatalf("accepted %q", entry.name)
		}
	}
	bad := m
	bad.Entrypoints = map[string]string{"linux/amd64": "missing"}
	if _, err := Inspect(archive(t, bad, "", 0, nil)); err == nil {
		t.Fatal("missing entry accepted")
	}
	manager, err := New(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	file := archive(t, m, "", 0, []byte("fake binary, install must not run it"))
	if _, err := manager.Install(file); err != nil {
		t.Fatal(err)
	}
	if _, err := manager.Install(file); err == nil {
		t.Fatal("version silently overwritten")
	}
	m.Version = "1.0.1"
	if _, err := manager.Install(archive(t, m, "", 0, nil)); err != nil {
		t.Fatal(err)
	}
	list, err := manager.List()
	if err != nil || len(list) != 2 {
		t.Fatalf("%v %v", list, err)
	}
	if err := manager.Remove("../escape"); err == nil {
		t.Fatal("unsafe remove")
	}
	if err := manager.Remove(m.Ref()); err != nil {
		t.Fatal(err)
	}
	list, _ = manager.List()
	if len(list) != 1 || list[0].Version != "1.0.0" {
		t.Fatal(list)
	}
	if _, err := Inspect("anything.potext"); err == nil || !strings.Contains(err.Error(), "Tauri") {
		t.Fatal(err)
	}
}
func TestRuntimeProtocolAndCancellation(t *testing.T) {
	exe, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	bin, err := os.ReadFile(exe)
	if err != nil {
		t.Fatal(err)
	}
	m := testManifest()
	if runtime.GOOS == "windows" {
		t.Skip("helper fixture uses extensionless entry; Linux CI executes this test")
	}
	manager, err := New(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	if _, err = manager.Install(archive(t, m, "", 0, bin)); err != nil {
		t.Fatal(err)
	}
	request := Request{Operation: "translate", Text: "Hello", Source: "auto", Target: "zh"}
	got, err := manager.Call(context.Background(), m.Ref(), request)
	if err != nil || got != "Hello translated" {
		t.Fatalf("%q %v", got, err)
	}
	for _, mode := range []string{"noise", "wrong-id", "overflow"} {
		t.Run(mode, func(t *testing.T) {
			request.Settings = map[string]string{"mode": mode}
			if _, err := manager.Call(context.Background(), m.Ref(), request); err == nil {
				t.Fatal("bad response accepted")
			}
		})
	}
	ctx, cancel := context.WithTimeout(context.Background(), 150*time.Millisecond)
	defer cancel()
	request.Settings = map[string]string{"mode": "sleep"}
	start := time.Now()
	_, err = manager.Call(ctx, m.Ref(), request)
	if !errors.Is(err, context.DeadlineExceeded) || time.Since(start) > 3*time.Second {
		t.Fatalf("cancellation: %v %s", err, time.Since(start))
	}
	if err := manager.Remove(m.Ref()); err != nil {
		t.Fatal(err)
	}
}
func TestDeclaredSettingsAndEnvironment(t *testing.T) {
	m := testManifest()
	m.Fields = []Field{{Key: "token", Label: "Token", Required: true}, {Key: "model", Label: "Model", Default: "default"}}
	if _, err := m.Settings(nil); err == nil {
		t.Fatal("required setting ignored")
	}
	values, err := m.Settings(map[string]string{"token": "secret", "unknown": "ignored"})
	if err != nil || values["model"] != "default" || len(values) != 2 {
		t.Fatalf("%v %v", values, err)
	}
	t.Setenv("LUMIN_TEST_SECRET", "should-not-leak")
	for _, v := range pluginEnv() {
		if strings.HasPrefix(v, "LUMIN_TEST_SECRET=") {
			t.Fatal("inherited unrelated secret")
		}
	}
}

// Set LUMIN_TEST_OCR_PLUGIN to a package produced by scripts/build-plugins.sh.
func TestBuiltOCRExample(t *testing.T) {
	file := os.Getenv("LUMIN_TEST_OCR_PLUGIN")
	if file == "" {
		t.Skip("build example packages to run integration test")
	}
	manager, err := New(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	manifest, err := manager.Install(file)
	if err != nil {
		t.Fatal(err)
	}
	path, err := filepath.Abs("../ocr/testdata/english.png")
	if err != nil {
		t.Fatal(err)
	}
	text, err := manager.Call(context.Background(), manifest.Ref(), Request{Operation: "recognize", ImagePath: path, Languages: "eng"})
	if err != nil || !strings.Contains(text, "Lumin translation test") {
		t.Fatalf("OCR plugin: %q %v", text, err)
	}
}

// Explicit opt-in: this test sends only the fixed non-sensitive text below to Caiyun.
func TestBuiltCaiyunPlugin(t *testing.T) {
	file := os.Getenv("LUMIN_TEST_CAIYUN_PLUGIN")
	if file == "" {
		t.Skip("set LUMIN_TEST_CAIYUN_PLUGIN to opt into the live service smoke test")
	}
	manager, err := New(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	manifest, err := manager.Install(file)
	if err != nil {
		t.Fatal(err)
	}
	if err := manager.ValidateUse(manifest.Ref(), "translate", nil); err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Second)
	defer cancel()
	text, err := manager.Call(ctx, manifest.Ref(), Request{Operation: "translate", Text: "Hello world", Source: "auto", Target: "zh"})
	if err != nil {
		t.Fatal(err)
	}
	if strings.TrimSpace(text) == "" || text == "Hello world" {
		t.Fatalf("unexpected untranslated result: %q", text)
	}
	t.Logf("Installed Caiyun plugin translated fixed test text: %q", text)
}

// Verifies the packaged executable through the real manager without network access.
func TestBuiltCaiyunPackage(t *testing.T) {
	file := os.Getenv("LUMIN_TEST_CAIYUN_PACKAGE")
	if file == "" {
		t.Skip("build Caiyun package to run the installed process test")
	}
	manager, err := New(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	manifest, err := manager.Install(file)
	if err != nil {
		t.Fatal(err)
	}
	if manifest.Ref() != "lumin.caiyun@1.0.0" {
		t.Fatal(manifest.Ref())
	}
	if err := manager.ValidateUse(manifest.Ref(), "translate", nil); err != nil {
		t.Fatal(err)
	}
	if err := manager.ValidateUse(manifest.Ref(), "recognize", nil); err == nil {
		t.Fatal("translator accepted OCR")
	}
	// The host accepts de, while the migrated plugin deliberately rejects it before HTTP.
	_, err = manager.Call(context.Background(), manifest.Ref(), Request{Operation: "translate", Text: "Hello", Source: "de", Target: "zh"})
	if err == nil || !strings.Contains(err.Error(), "彩云插件支持") {
		t.Fatalf("packaged program/stdio validation failed: %v", err)
	}
	t.Log("Installed package executed and returned its structured language error through JSON stdio")
}
