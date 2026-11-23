package desktop

import (
	"context"
	"image/png"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/egoist/mygo/ui"
	"lumin/internal/config"
	"lumin/internal/domain"
	"lumin/internal/storage"
)

func testApp(t *testing.T) (*App, chan func()) {
	t.Helper()
	s, err := storage.New(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	queue := make(chan func(), 100)
	a := New(s, config.Default(), nil, Ports{Dispatch: func(fn func()) { queue <- fn }})
	t.Cleanup(a.Close)
	return a, queue
}

func drainUntil(t *testing.T, q chan func(), done func() bool) {
	t.Helper()
	timeout := time.NewTimer(3 * time.Second)
	defer timeout.Stop()
	for !done() {
		select {
		case fn := <-q:
			fn()
		case <-timeout.C:
			t.Fatal("timed out waiting for UI update")
		}
	}
}

func click(t *testing.T, tt *ui.Tester, label string) {
	t.Helper()
	if err := tt.Click(label); err != nil {
		t.Fatal(err)
	}
}

func TestNavigationTypingAndClipboard(t *testing.T) {
	a, _ := testApp(t)
	a.ports.ReadClipboard = func() string { return "来自剪贴板" }
	preferencesOpened := 0
	a.ports.ShowPreferences = func() { preferencesOpened++ }
	a.draft.Providers[0].Name = "未保存的配置"
	main := ui.NewTester(a.View, 1100, 800)
	tt := ui.NewTester(a.TranslationView, 520, 620)
	click(t, tt, "待翻译文本")
	tt.Type("Hello 世界")
	if a.text != "Hello 世界" {
		t.Fatalf("lost input %q", a.text)
	}
	click(t, tt, "翻译文本")
	if preferencesOpened != 0 || a.page != "设置" || !strings.Contains(a.status, "请先") {
		t.Fatal("unconfigured translation must explain the problem in the small window")
	}
	click(t, tt, "配置翻译服务")
	main.Frame()
	if preferencesOpened != 1 || a.page != "翻译服务" {
		t.Fatal("configuration link did not open preferences")
	}
	for _, page := range []string{"历史记录", "生词本", "翻译"} {
		click(t, tt, page)
		tt.Frame()
		tt.Frame()
		main.Frame()
		if a.translationPage != page || a.page != "翻译服务" {
			t.Fatal("window navigation leaked")
		}
	}
	for _, page := range pages {
		click(t, main, page)
		main.Frame()
		main.Frame()
		tt.Frame()
		if a.page != page || a.translationPage != "翻译" {
			t.Fatal("preferences changed translation navigation")
		}
		for _, text := range main.Texts() {
			if text == "历史记录" || text == "翻译文本" || text == "截图翻译" || text == "粘贴" {
				t.Fatalf("preferences contains translation control %q", text)
			}
		}
	}
	click(t, tt, "粘贴")
	if a.text != "来自剪贴板" {
		t.Fatal(a.text)
	}
	click(t, tt, "清空")
	if a.text != "" {
		t.Fatal("clear failed")
	}
	if a.draft.Providers[0].Name != "未保存的配置" {
		t.Fatal("draft was lost")
	}
}

type testTranslator struct {
	fn func(context.Context, domain.Request) (string, error)
}

func (testTranslator) ID() string   { return "test" }
func (testTranslator) Name() string { return "Test provider" }
func (p testTranslator) Translate(ctx context.Context, r domain.Request) (string, error) {
	return p.fn(ctx, r)
}

func TestTranslationCompletionAndPersistence(t *testing.T) {
	a, q := testApp(t)
	a.ports.Translators = func(config.Config) []domain.Translator {
		return []domain.Translator{testTranslator{func(context.Context, domain.Request) (string, error) { return "你好", nil }}}
	}
	a.TranslateText("Hello")
	drainUntil(t, q, func() bool { return !a.busy && len(a.history) == 1 })
	if len(a.results) != 1 || a.results[0].Text != "你好" || a.history[0].Request.Text != "Hello" {
		t.Fatalf("results %+v history %+v", a.results, a.history)
	}
}

func TestQueuedOldResultsCannotOverwriteNewInput(t *testing.T) {
	a, q := testApp(t)
	a.settings.SaveHistory = false
	a.ports.Translators = func(config.Config) []domain.Translator {
		return []domain.Translator{testTranslator{func(context.Context, domain.Request) (string, error) { return "old result", nil }}}
	}
	a.TranslateText("old")
	// Both the result and completion are queued, but have not reached the UI.
	var first, second func()
	select {
	case first = <-q:
	case <-time.After(time.Second):
		t.Fatal("missing result")
	}
	select {
	case second = <-q:
	case <-time.After(time.Second):
		t.Fatal("missing completion")
	}
	a.sourceChanged()
	a.text = "new input"
	first()
	second()
	if len(a.results) != 0 || a.text != "new input" || a.busy {
		t.Fatalf("stale result committed: %+v", a.results)
	}
}

func TestClosedWindowRejectsQueuedResult(t *testing.T) {
	a, q := testApp(t)
	a.status = "before"
	a.post(func() { a.status = "after" })
	a.Close()
	(<-q)()
	if a.status != "before" {
		t.Fatal("closed window mutated")
	}
}

type testOCR struct{}

func (testOCR) Recognize(context.Context, string, string) (string, error) {
	return "Recognized text", nil
}

func TestOCRSuccessSurvivesWorkerCancellation(t *testing.T) {
	a, q := testApp(t)
	a.ocr = testOCR{}
	a.ports.ChooseImage = func() (string, error) { return "example.png", nil }
	a.recognizeImage()
	drainUntil(t, q, func() bool { return !a.busy })
	if a.text != "Recognized text" {
		t.Fatalf("OCR text %q status %q", a.text, a.status)
	}
}

func TestOwnClipboardWritesDoNotTranslate(t *testing.T) {
	a, _ := testApp(t)
	a.watchClipboard = true
	clip := ""
	a.ports.WriteClipboard = func(s string) { clip = s }
	a.ports.ReadClipboard = func() string { return clip }
	a.copy("translation")
	a.ClipboardChanged()
	if a.text != "" || a.busy {
		t.Fatal("copy triggered translation loop")
	}
}

func TestRenderPages(t *testing.T) {
	a, _ := testApp(t)
	for _, size := range []struct{ w, h int }{{1100, 800}, {920, 680}} {
		for _, page := range pages {
			a.page = page
			tt := ui.NewTester(a.View, size.w, size.h)
			tt.Frame()
			if !tt.HasText("Lumin") {
				t.Fatal("missing app title")
			}
			if bounds, ok := tt.Find("Lumin"); !ok || bounds.Y > 80 {
				t.Fatalf("sidebar not aligned to top: %+v", bounds)
			}
			if page == "翻译服务" {
				if bounds, ok := tt.Find("保存服务"); !ok || bounds.Y < 0 || bounds.Y+bounds.H > float32(size.h) {
					t.Fatalf("save button outside window: %+v", bounds)
				}
			}
			if dir := os.Getenv("LUMIN_TEST_SCREENSHOTS"); dir != "" && size.w == 1100 {
				if err := os.MkdirAll(dir, 0755); err != nil {
					t.Fatal(err)
				}
				f, err := os.Create(filepath.Join(dir, page+".png"))
				if err != nil {
					t.Fatal(err)
				}
				if err := png.Encode(f, tt.Image()); err != nil {
					t.Fatal(err)
				}
				_ = f.Close()
			}
		}
	}
	a.translationPage = "翻译"
	a.text = "Translation helps us understand each other."
	a.results = []domain.Result{{ProviderID: "one", ProviderName: "Ollama", Text: "翻译帮助我们理解彼此。", DurationMS: 1200}, {ProviderID: "two", ProviderName: "DeepL", Error: "翻译服务返回 HTTP 403，请检查密钥、模型、配额和接口地址"}}
	tt := ui.NewTester(a.TranslationView, 520, 620)
	if !strings.Contains(strings.Join(tt.Texts(), " "), "翻译帮助我们") {
		t.Fatal("result not rendered")
	}
	if dir := os.Getenv("LUMIN_TEST_SCREENSHOTS"); dir != "" {
		f, err := os.Create(filepath.Join(dir, "results.png"))
		if err != nil {
			t.Fatal(err)
		}
		defer f.Close()
		if err := png.Encode(f, tt.Image()); err != nil {
			t.Fatal(err)
		}
	}
	dark := ui.NewTester(func(c *ui.Context) { c.SetTheme(ui.DarkTheme()); a.TranslationView(c) }, 520, 620)
	if !dark.HasText("翻译") {
		t.Fatal("dark theme did not render")
	}
	if dir := os.Getenv("LUMIN_TEST_SCREENSHOTS"); dir != "" {
		f, err := os.Create(filepath.Join(dir, "dark.png"))
		if err != nil {
			t.Fatal(err)
		}
		defer f.Close()
		if err := png.Encode(f, dark.Image()); err != nil {
			t.Fatal(err)
		}
	}
}
