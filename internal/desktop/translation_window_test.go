package desktop

import (
	"context"
	"errors"
	"image"
	"image/png"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/egoist/mygo/ui"
	"lumin/internal/capture"
	"lumin/internal/config"
	"lumin/internal/domain"
)

func TestTranslationEntrypointsKeepPreferencesHidden(t *testing.T) {
	a, q := testApp(t)
	shown, preferences := 0, 0
	a.ports.ShowTranslator = func() { shown++ }
	a.ports.ShowPreferences = func() { preferences++ }
	a.ports.ReadClipboard = func() string { return "clipboard" }
	a.ports.CaptureArea = func(context.Context) (image.Image, error) { return image.NewRGBA(image.Rect(0, 0, 30, 30)), nil }
	a.ocr = testOCR{}
	a.ShowTranslation()
	a.TranslateText("external HTTP text")
	a.Paste(true)
	a.watchClipboard = true
	a.ClipboardChanged()
	a.Screenshot(false)
	drainUntil(t, q, func() bool { return !a.busy })
	if shown != 5 || preferences != 0 || a.page != "设置" {
		t.Fatalf("unexpected routing: translation=%d settings=%d page=%s", shown, preferences, a.page)
	}
	// Empty clipboard still opens the translation window to show the error.
	a.ports.ReadClipboard = func() string { return "" }
	a.Paste(true)
	if shown != 6 || a.status != "剪贴板中没有文本" {
		t.Fatal("empty clipboard error hidden")
	}
}

func TestScreenshotCancellationDoesNotRevealHiddenWindows(t *testing.T) {
	for _, err := range []error{capture.ErrCanceled, errors.New("screen permission denied")} {
		a, q := testApp(t)
		shown, preferences := 0, 0
		a.ports.ShowTranslator = func() { shown++ }
		a.ports.ShowPreferences = func() { preferences++ }
		a.ports.CaptureArea = func(context.Context) (image.Image, error) { return nil, err }
		a.Screenshot(true)
		drainUntil(t, q, func() bool { return !a.busy })
		if preferences != 0 {
			t.Fatal("screenshot revealed settings")
		}
		if errors.Is(err, capture.ErrCanceled) && shown != 0 {
			t.Fatal("canceled screenshot opened translator")
		}
		if !errors.Is(err, capture.ErrCanceled) && shown != 1 {
			t.Fatal("screenshot failure has no visible explanation")
		}
	}
}

func TestDismissCancelsJobButAllowsNextTranslation(t *testing.T) {
	a, q := testApp(t)
	hidden := 0
	a.ports.HideTranslator = func() { hidden++ }
	a.settings.SaveHistory = false
	started, stopped := make(chan struct{}), make(chan struct{})
	a.ports.Translators = func(config.Config) []domain.Translator {
		return []domain.Translator{testTranslator{func(ctx context.Context, _ domain.Request) (string, error) {
			close(started)
			<-ctx.Done()
			close(stopped)
			return "stale", ctx.Err()
		}}}
	}
	a.TranslateText("first")
	<-started
	a.DismissTranslation()
	select {
	case <-stopped:
	case <-time.After(time.Second):
		t.Fatal("dismiss did not cancel request")
	}
	if hidden != 1 || a.closed || a.root.Err() != nil || a.busy {
		t.Fatal("dismiss stopped background application")
	}
	a.ports.Translators = func(config.Config) []domain.Translator {
		return []domain.Translator{testTranslator{func(context.Context, domain.Request) (string, error) { return "new", nil }}}
	}
	a.TranslateText("second")
	drainUntil(t, q, func() bool { return !a.busy })
	if len(a.results) != 1 || a.results[0].Text != "new" {
		t.Fatal("background reopen failed or stale result committed")
	}
}

func TestInputShortcutFocusAndExplicitQuit(t *testing.T) {
	a, _ := testApp(t)
	quits := 0
	a.ports.Quit = func() { quits++ }
	main := ui.NewTester(a.View, 920, 680)
	tt := ui.NewTester(a.TranslationView, 460, 480)
	a.ShowTranslation()
	tt.Frame()
	tt.Type("无需点击即可输入")
	if a.text != "无需点击即可输入" {
		t.Fatal("translation input did not receive focus")
	}
	click(t, tt, "历史记录")
	a.ShowTranslation()
	tt.Frame()
	tt.Frame()
	tt.Type("！")
	if a.text != "无需点击即可输入！" {
		t.Fatalf("input lost focus on reopen: %q", a.text)
	}
	click(t, main, "退出 Lumin")
	if quits != 1 {
		t.Fatal("explicit quit action was not delivered")
	}
}

func TestCompactWindowLayouts(t *testing.T) {
	a, _ := testApp(t)
	a.text = "Hello 世界"
	a.history = []domain.Entry{{ID: "one", CreatedAt: time.Now(), Favorite: true, Request: domain.Request{Text: "Hello", Source: "en", Target: "zh"}, Results: []domain.Result{{ProviderID: "test", ProviderName: "测试服务", Text: "你好"}}}}
	for _, size := range []struct{ w, h int }{{520, 620}, {460, 480}} {
		for _, page := range translationPages {
			for _, busy := range []bool{false, true} {
				a.translationPage, a.busy = page, busy
				tt := ui.NewTester(a.TranslationView, size.w, size.h)
				tt.Frame()
				tt.Frame()
				labels := []string{"翻译", "历史记录", "生词本", "设置"}
				if page == "翻译" {
					labels = append(labels, "待翻译文本", "源语言", "目标语言", "粘贴", "图片识字", "清空", "翻译文本", "截图翻译")
				}
				if page != "翻译" {
					labels = append(labels, "搜索记录", "载入", "取消收藏")
				}
				if page == "翻译" && busy {
					labels = append(labels, "取消")
				}
				for _, label := range labels {
					b, ok := tt.Find(label)
					if !ok || b.W <= 0 || b.H <= 0 || b.X < 0 || b.Y < 0 || b.X+b.W > float32(size.w) || b.Y+b.H > float32(size.h) {
						t.Fatalf("%s %dx%d busy=%v %s out of bounds %+v", page, size.w, size.h, busy, label, b)
					}
				}
				if dir := os.Getenv("LUMIN_TEST_SCREENSHOTS"); dir != "" && size.w == 460 && busy {
					if err := os.MkdirAll(dir, 0755); err != nil {
						t.Fatal(err)
					}
					f, err := os.Create(filepath.Join(dir, "compact-"+page+".png"))
					if err != nil {
						t.Fatal(err)
					}
					err = png.Encode(f, tt.Image())
					_ = f.Close()
					if err != nil {
						t.Fatal(err)
					}
				}
			}
		}
	}
	a.busy = false
	a.translationPage = "翻译"
	hidden := 0
	a.ports.HideTranslator = func() { hidden++ }
	tt := ui.NewTester(a.TranslationView, 460, 480)
	a.busy = true
	tt.Frame()
	tt.Key(0, ui.KeyEscape)
	if a.busy || hidden != 0 {
		t.Fatal("first Escape should cancel task")
	}
	tt.Key(0, ui.KeyEscape)
	if hidden != 1 {
		t.Fatal("idle Escape should hide translator")
	}
}
