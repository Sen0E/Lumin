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
	a.Paste()
	a.Screenshot(false)
	drainUntil(t, q, func() bool { return !a.recognition.busy })
	if shown != 3 || preferences != 0 || a.page != "常规设置" {
		t.Fatalf("unexpected routing: translation=%d settings=%d page=%s", shown, preferences, a.page)
	}
	// Empty clipboard still opens the translation window to show the error.
	a.ports.ReadClipboard = func() string { return "" }
	a.Paste()
	if shown != 4 || a.status != "剪贴板中没有文本" {
		t.Fatal("empty clipboard error hidden")
	}
}

func TestScreenshotCancellationDoesNotRevealHiddenWindows(t *testing.T) {
	for _, err := range []error{capture.ErrCanceled, errors.New("screen permission denied")} {
		a, q := testApp(t)
		a.ocr = testOCR{}
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
	click(t, main, "历史记录")
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
		for _, busy := range []bool{false, true} {
			a.busy = busy
			tt := ui.NewTester(a.TranslationView, size.w, size.h)
			tt.Frame()
			tt.Frame()
			labels := []string{"置顶", "关闭", "待翻译文本", "源语言", "目标语言", "粘贴", "复制原文", "清空", "翻译文本"}
			if busy {
				labels = append(labels, "取消")
			}
			for _, label := range labels {
				b, ok := tt.Find(label)
				if !ok || b.W <= 0 || b.H <= 0 || b.X < 0 || b.Y < 0 || b.X+b.W > float32(size.w) || b.Y+b.H > float32(size.h) {
					t.Fatalf("%dx%d busy=%v %s out of bounds %+v", size.w, size.h, busy, label, b)
				}
			}
			for _, text := range tt.Texts() {
				if text == "历史记录" || text == "生词本" || text == "设置" || text == "截图翻译" || text == "图片识字" {
					t.Fatalf("misplaced control %q", text)
				}
			}
			if dir := os.Getenv("LUMIN_TEST_SCREENSHOTS"); dir != "" && size.w == 460 && busy {
				if err := os.MkdirAll(dir, 0755); err != nil {
					t.Fatal(err)
				}
				f, err := os.Create(filepath.Join(dir, "compact-翻译.png"))
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
	a.busy = false
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

// Reading a result should use most of a compact window, with source/actions
// available on demand instead of permanently taking the first half of it.
func TestCompactReadingAndEditing(t *testing.T) {
	for _, dark := range []bool{false, true} {
		for _, size := range []struct{ w, h int }{{320, 360}, {350, 420}, {420, 520}} {
			a, _ := testApp(t)
			if dark {
				a.settings.Appearance = "dark"
			} else {
				a.settings.Appearance = "light"
			}
			a.text = "Make the translation the first thing you see. Keep the original available when you need it."
			a.sourceCollapsed = true
			translated := "让译文成为第一眼看到的内容。需要时，仍可查看和编辑原文。"
			a.results = []domain.Result{{ProviderID: "one", ProviderName: "彩云小译", Text: translated, DurationMS: 260}, {ProviderID: "two", ProviderName: "第二个翻译服务", Text: "优先呈现翻译结果，让阅读更轻松。", DurationMS: 430}}
			copied := ""
			a.ports.WriteClipboard = func(text string) { copied = text }
			tt := ui.NewTester(a.TranslationView, size.w, size.h)
			tt.Frame()
			body, ok := tt.Find(translated)
			if !ok || body.Y > 135 || body.X < 0 || body.X+body.W > float32(size.w) {
				t.Fatalf("dark=%v %dx%d result not prominent: %+v", dark, size.w, size.h, body)
			}
			for _, label := range []string{"置顶", "关闭", "展开原文", "复制译文"} {
				b, ok := tt.Find(label)
				if !ok || b.X < 0 || b.Y < 0 || b.X+b.W > float32(size.w)+1 || b.Y+b.H > float32(size.h)+1 {
					t.Fatalf("clipped %s: %+v", label, b)
				}
			}
			if _, ok := tt.Find("待翻译文本"); ok {
				t.Fatal("reading mode kept the editor visible")
			}
			click(t, tt, "复制译文")
			if copied != translated {
				t.Fatalf("copied wrong result: %q", copied)
			}
			if dir := os.Getenv("LUMIN_TEST_SCREENSHOTS"); dir != "" && size.w == 350 {
				if err := os.MkdirAll(dir, 0755); err != nil {
					t.Fatal(err)
				}
				name := "compact-light.png"
				if dark {
					name = "compact-dark.png"
				}
				f, err := os.Create(filepath.Join(dir, name))
				if err != nil {
					t.Fatal(err)
				}
				err = png.Encode(f, tt.Image())
				f.Close()
				if err != nil {
					t.Fatal(err)
				}
			}
			click(t, tt, "展开原文")
			tt.Frame()
			tt.Frame()
			for _, label := range []string{"待翻译文本", "源语言", "目标语言", "翻译文本", "粘贴", "清空"} {
				b, ok := tt.Find(label)
				if !ok || b.X < 0 || b.Y < 0 || b.X+b.W > float32(size.w)+1 || b.Y+b.H > float32(size.h)+1 {
					t.Fatalf("editing clipped %s: %+v", label, b)
				}
			}
			click(t, tt, "收起原文")
			tt.Frame()
			tt.Frame()
			if a.text == "" || len(a.results) != 2 {
				t.Fatal("mode switch discarded content")
			}
		}
	}
}

func TestTranslationEntersReadingOnlyForValidRequest(t *testing.T) {
	a, q := testApp(t)
	a.text = "hello"
	a.Translate()
	if a.sourceCollapsed {
		t.Fatal("missing services must leave setup/editor accessible")
	}
	a.ports.Translators = func(config.Config) []domain.Translator {
		return []domain.Translator{testTranslator{func(context.Context, domain.Request) (string, error) { return "你好", nil }}}
	}
	a.TranslateText("hello")
	if !a.sourceCollapsed || a.focusInput {
		t.Fatal("automatic translation did not enter reading mode")
	}
	drainUntil(t, q, func() bool { return !a.busy })
	if !a.sourceCollapsed || len(a.results) != 1 || a.results[0].Text != "你好" {
		t.Fatal("completion lost reading mode/results")
	}
	a.ShowTranslation()
	if a.sourceCollapsed || !a.focusInput {
		t.Fatal("manual input shortcut must reopen the editor")
	}
}
