package desktop

import (
	"context"
	"image"
	"image/png"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"

	"github.com/egoist/mygo/ui"
	"lumin/internal/config"
	"lumin/internal/ocr"
	"lumin/internal/plugin"
)

func ocrManifest() plugin.Manifest {
	return plugin.Manifest{Schema: 1, ID: "test.ocr", Version: "1.0.0", Name: "测试离线 OCR", Capabilities: []string{"recognize"}, Entrypoints: map[string]string{runtime.GOOS + "/" + runtime.GOARCH: "bin/plugin"}, Fields: []plugin.Field{{Key: "threads", Label: "线程数", Default: "4"}}}
}
func TestOCRSettingsNoSystemFallback(t *testing.T) {
	a, _ := testApp(t)
	t.Setenv("PATH", "")
	selected := 0
	a.ports.ShowPreferences = func() { selected++ }
	captured := 0
	a.ports.CaptureArea = func(context.Context) (image.Image, error) {
		captured++
		return image.NewRGBA(image.Rect(0, 0, 20, 20)), nil
	}
	a.Screenshot(false)
	if a.recognition.busy || (a.page != "服务设置" || a.servicePage != "OCR 服务") || selected != 1 || captured != 0 {
		t.Fatal("unconfigured OCR captured screen or started work")
	}
	a.Screenshot(true)
	if a.busy || captured != 0 || selected != 2 {
		t.Fatal("translation OCR did not guide setup")
	}
	if _, err := a.ocrEngine(config.Default()); err == nil {
		t.Fatal("system fallback still active")
	}
	tt := ui.NewTester(a.View, 920, 680)
	tt.Frame()
	tt.Frame()
	for _, text := range tt.Texts() {
		if strings.Contains(text, "Tesseract") || strings.Contains(text, "语言包") {
			t.Fatal("obsolete OCR setting", text)
		}
	}
	for _, page := range pages {
		if page == "文字识别" {
			t.Fatal("duplicate OCR settings navigation")
		}
	}
	click(t, tt, "管理插件")
	if a.page != "服务设置" || a.servicePage != "插件" {
		t.Fatal("wrong plugin management destination")
	}
}
func TestOCRDraftSelectionAndDisable(t *testing.T) {
	a, _ := testApp(t)
	a.installed = []plugin.Manifest{ocrManifest()}
	a.page = "服务设置"
	a.servicePage = "OCR 服务"
	tt := ui.NewTester(a.View, 920, 680)
	click(t, tt, "选择 测试离线 OCR · 1.0.0")
	tt.Frame()
	tt.Frame()
	if a.draft.OCRPlugin != "test.ocr@1.0.0" || a.draft.OCRSettings["threads"] != "4" || a.settings.OCRPlugin != "" {
		t.Fatal("selection changed runtime or lost defaults")
	}
	click(t, tt, "插件设置：线程数")
	tt.Key(ui.Cmd, ui.KeyA)
	tt.Type("8")
	tt.Frame()
	tt.Frame()
	if a.draft.OCRSettings["threads"] != "8" {
		t.Fatal("plugin parameter did not survive frames")
	}
	click(t, tt, "停用 OCR")
	tt.Frame()
	if !a.draft.OCRDisabled || a.draft.OCRPlugin != "" {
		t.Fatal("disable not explicit")
	}
	a.discardSettings()
	tt.Frame()
	if a.draft.OCRDisabled || a.draft.OCRPlugin != "" {
		t.Fatal("discard failed")
	}
}

type probeOCR struct{ path chan string }

func (p probeOCR) Recognize(_ context.Context, path, _ string) (string, error) {
	b, err := os.ReadFile(path)
	if err != nil {
		return "", err
	}
	if string(b) != string(ocr.ProbeImage) {
		return "", os.ErrInvalid
	}
	p.path <- path
	return "Lumin translation test.", nil
}
func TestOCRProbeIsolationAndCleanup(t *testing.T) {
	a, q := testApp(t)
	paths := make(chan string, 1)
	a.ocr = probeOCR{paths}
	a.text = "keep translation"
	a.recognition.text = "keep OCR result"
	a.status = "keep status"
	a.draft.OCRPlugin = "test.ocr@1.0.0"
	a.testOCR()
	drainUntil(t, q, func() bool { return !a.ocrProbe.Busy })
	if !strings.Contains(a.ocrProbe.Message, "Lumin translation test.") || a.text != "keep translation" || a.recognition.text != "keep OCR result" || a.status != "keep status" || len(a.history) != 0 || a.settings.OCRPlugin != "" {
		t.Fatal("probe changed application state", a.ocrProbe.Message)
	}
	path := <-paths
	deadline := time.Now().Add(time.Second)
	for {
		if _, err := os.Stat(path); os.IsNotExist(err) {
			break
		}
		if time.Now().After(deadline) {
			t.Fatal("probe image leaked")
		}
		time.Sleep(time.Millisecond)
	}
	a.draft.OCRSettings = map[string]string{"threads": "8"}
	if a.ocrProbeMatches() {
		t.Fatal("edited draft still matches old probe")
	}
}
func TestOCRProbeCancellationRejectsLateCompletion(t *testing.T) {
	a, q := testApp(t)
	b := blockingOCR{make(chan struct{}), make(chan struct{})}
	a.ocr = b
	a.testOCR()
	<-b.started
	a.cancelOCRProbe()
	close(b.release)
	select {
	case fn := <-q:
		fn()
	case <-time.After(time.Second):
		t.Fatal("no completion")
	}
	if a.ocrProbe.Busy || a.ocrProbe.Message != "已取消 OCR 测试" {
		t.Fatal("canceled result published", a.ocrProbe.Message)
	}
}
func TestOCRSettingsLayout(t *testing.T) {
	a, _ := testApp(t)
	a.installed = []plugin.Manifest{ocrManifest()}
	a.selectOCR("test.ocr@1.0.0")
	for _, page := range []string{"服务设置"} {
		a.page = page
		a.servicePage = "OCR 服务"
		tt := ui.NewTester(a.View, 920, 680)
		tt.Frame()
		tt.Frame()
		label := "测试 OCR 引擎"
		b, ok := tt.Find(label)
		if !ok || b.W <= 0 || b.H <= 0 || b.X < 0 || b.Y < 0 || b.X+b.W > 920 || b.Y+b.H > 680 {
			t.Fatalf("%s %+v", label, b)
		}
		if dir := os.Getenv("LUMIN_TEST_SCREENSHOTS"); dir != "" {
			if err := os.MkdirAll(dir, 0755); err != nil {
				t.Fatal(err)
			}
			f, err := os.Create(filepath.Join(dir, "ocr-"+page+".png"))
			if err != nil {
				t.Fatal(err)
			}
			err = png.Encode(f, tt.Image())
			f.Close()
			if err != nil {
				t.Fatal(err)
			}
		}
	}
}
func TestRealPluginOCRProbe(t *testing.T) {
	path := os.Getenv("LUMIN_PPOCR_PACKAGE")
	if path == "" {
		t.Skip("set LUMIN_PPOCR_PACKAGE for real model integration")
	}
	a, q := testApp(t)
	m, err := plugin.New(filepath.Join(t.TempDir(), "plugins"))
	if err != nil {
		t.Fatal(err)
	}
	manifest, err := m.Install(path)
	if err != nil {
		t.Fatal(err)
	}
	a.ports.Plugins = m
	a.installed = []plugin.Manifest{manifest}
	a.selectOCR(manifest.Ref())
	t.Setenv("PATH", "")
	a.testOCR()
	drainUntil(t, q, func() bool { return !a.ocrProbe.Busy })
	if !strings.Contains(a.ocrProbe.Message, "Lumin translation test.") {
		t.Fatal(a.ocrProbe.Message)
	}
}
