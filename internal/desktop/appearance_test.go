package desktop

import (
	"context"
	"image/png"
	"math"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/egoist/mygo/ui"
	"lumin/internal/config"
	"lumin/internal/domain"
)

func TestTranslationBlurPinAndPreference(t *testing.T) {
	for _, tc := range []struct {
		name                string
		pin, keep, wantHide bool
	}{
		{"auto hide", false, false, true}, {"pinned", true, false, false}, {"keep visible", false, true, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			a, _ := testApp(t)
			hidden := 0
			a.pinned = tc.pin
			a.settings.KeepTranslationOnBlur = tc.keep
			a.ports.HideTranslator = func() { hidden++ }
			a.text = "keep source"
			a.results = []domain.Result{{ProviderID: "test", Text: "keep translation"}}
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			a.cancel = cancel
			a.busy = true
			a.TranslationBlurred()
			if (hidden == 1) != tc.wantHide {
				t.Fatal("wrong blur behavior", hidden)
			}
			if a.text != "keep source" || len(a.results) != 1 {
				t.Fatal("auto-hide discarded content")
			}
			if tc.wantHide {
				if ctx.Err() == nil || a.busy {
					t.Fatal("hidden task not canceled")
				}
			} else if ctx.Err() != nil || !a.busy {
				t.Fatal("pin/preference did not preserve task")
			}
		})
	}
}
func TestSelectionCancelsPendingBlurBeforeReading(t *testing.T) {
	a, q := testApp(t)
	prepared := false
	started := make(chan bool, 1)
	a.ports.CancelTranslationBlur = func() { prepared = true }
	// Channel publication in TranslateSelection orders this read after prepare.
	a.ports.ReadSelection = func(context.Context) (string, error) { started <- prepared; return "new source", nil }
	a.TranslateSelection()
	select {
	case before := <-started:
		if !before {
			t.Fatal("old blur can cancel new selection")
		}
	case <-time.After(time.Second):
		t.Fatal("reader not called")
	}
	drainUntil(t, q, func() bool { return !a.busy })
}
func TestWindowPreferencesSaveDiscardAndRestore(t *testing.T) {
	a, q := testApp(t)
	var applied config.Config
	a.ports.ApplyConfig = func(c config.Config) error { applied = c; return nil }
	a.page = "翻译设置"
	tt := ui.NewTester(a.View, 1100, 900)
	numberInput(t, tt, "翻译窗口宽度输入", "640")
	numberInput(t, tt, "翻译窗口高度输入", "720")
	numberInput(t, tt, "译文字号输入", "20")
	click(t, tt, "点击其他窗口后自动隐藏（未置顶）")
	tt.Frame()
	tt.Frame()
	if !a.draft.KeepTranslationOnBlur {
		t.Fatal("inverted checkbox lost edit")
	}
	click(t, tt, "常规设置")
	click(t, tt, "外观主题选择")
	click(t, tt, "深色")
	tt.Frame()
	tt.Frame()
	if a.draft.Appearance != "dark" || a.settings.Appearance == "dark" {
		t.Fatal("theme draft not isolated")
	}
	click(t, tt, "保存设置")
	drainUntil(t, q, func() bool { return !a.saving })
	cfg, err := a.store.LoadConfig()
	if err != nil {
		t.Fatal(err)
	}
	if cfg.TranslationWidth != 640 || cfg.TranslationHeight != 720 || cfg.ResultFontSize() != 20 || !cfg.KeepTranslationOnBlur || cfg.Appearance != "dark" || applied.Appearance != "dark" {
		t.Fatalf("lost display preferences: %+v", cfg)
	}
	b := New(a.store, cfg, nil, a.ports)
	defer b.Close()
	if b.preferences.WindowWidth != 640 || b.preferences.Appearance != "深色" || b.preferences.AutoHide {
		t.Fatal("restart lost display preferences")
	}
	click(t, tt, "外观主题选择")
	click(t, tt, "浅色")
	click(t, tt, "放弃未保存修改")
	tt.Frame()
	tt.Frame()
	if a.preferences.Appearance != "深色" || a.draft.Appearance != "dark" {
		t.Fatal("discard did not restore theme")
	}
}
func TestTranslationLayoutCollapseThemesAndSmallWindow(t *testing.T) {
	a, _ := testApp(t)
	a.text = "The selected source text."
	a.settings.TranslationFontSize = 20
	a.results = []domain.Result{{ProviderID: "one", ProviderName: "翻译服务", Text: "选中文字的翻译结果。"}, {ProviderID: "two", ProviderName: "失败服务", Error: "连接失败，请稍后重试"}}
	for _, theme := range []struct {
		name  string
		theme *ui.Theme
	}{{"light", ui.LightTheme()}, {"dark", ui.DarkTheme()}} {
		for _, size := range []struct{ w, h int }{{460, 480}, {520, 620}, {640, 720}} {
			a.sourceCollapsed = false
			tt := ui.NewTester(func(c *ui.Context) { c.SetTheme(theme.theme); a.TranslationView(c) }, size.w, size.h)
			for _, label := range []string{"置顶", "关闭", "翻译文本", "待翻译文本", "源语言", "目标语言"} {
				r, ok := tt.Find(label)
				if !ok || r.X < 0 || r.Y < 0 || r.X+r.W > float32(size.w)+1 || r.Y+r.H > float32(size.h)+1 {
					t.Fatalf("%s %dx%d control clipped: %s %+v", theme.name, size.w, size.h, label, r)
				}
			}
			click(t, tt, "收起原文")
			tt.Frame()
			tt.Frame()
			if _, ok := tt.Find("待翻译文本"); ok {
				t.Fatal("collapsed editor still built")
			}
			click(t, tt, "展开原文")
			tt.Frame()
			tt.Frame()
			if _, ok := tt.Find("待翻译文本"); !ok || a.text != "The selected source text." {
				t.Fatal("expansion lost source/identity")
			}
			if dir := os.Getenv("LUMIN_TEST_SCREENSHOTS"); dir != "" && size.w == 520 {
				if err := os.MkdirAll(dir, 0755); err != nil {
					t.Fatal(err)
				}
				file, err := os.Create(filepath.Join(dir, "translation-ux-"+theme.name+".png"))
				if err != nil {
					t.Fatal(err)
				}
				err = png.Encode(file, tt.Image())
				file.Close()
				if err != nil {
					t.Fatal(err)
				}
			}
		}
	}
}

func TestReadableThemeContrastAndNativeMetrics(t *testing.T) {
	luminance := func(c ui.Color) float64 {
		vals := []uint8{c.R, c.G, c.B}
		weights := []float64{.2126, .7152, .0722}
		sum := 0.0
		for i, v := range vals {
			x := float64(v) / 255
			if x <= .04045 {
				x /= 12.92
			} else {
				x = math.Pow((x+.055)/1.055, 2.4)
			}
			sum += x * weights[i]
		}
		return sum
	}
	for _, base := range []*ui.Theme{ui.LightTheme(), ui.DarkTheme()} {
		theme := readableTheme(base)
		for _, pair := range [][2]ui.Color{{theme.Text, theme.Background}, {theme.Text, theme.Surface}, {theme.TextMuted, theme.Surface}, {theme.Danger, theme.Surface}, {theme.AccentText, theme.Accent}} {
			lo, hi := luminance(pair[0]), luminance(pair[1])
			if lo > hi {
				lo, hi = hi, lo
			}
			if ratio := (hi + .05) / (lo + .05); ratio < 4.5 {
				t.Fatalf("dark=%v low text contrast: %.2f", theme.Dark, ratio)
			}
		}
		if theme.FontSize != base.FontSize || theme.Spacing != base.Spacing || theme.Dark != base.Dark {
			t.Fatal("theme adaptation changed native metrics")
		}
	}
}
func TestSavedThemeCanSwitchBackToSystem(t *testing.T) {
	a, _ := testApp(t)
	a.settings.Appearance = "dark"
	var dark bool
	tt := ui.NewTester(func(c *ui.Context) { a.applyAppearance(c); dark = c.Theme().Dark; ui.Text(c, "theme") }, 200, 100)
	if !dark {
		t.Fatal("saved dark preference ignored")
	}
	a.settings.Appearance = "light"
	tt.Frame()
	if dark {
		t.Fatal("saved light preference ignored")
	}
	a.settings.Appearance = "system"
	tt.Frame()
	if dark {
		t.Fatal("forced theme leaked into system default")
	}
}

func TestExplicitPresetReappliesSizeButThemeSaveKeepsNativeResize(t *testing.T) {
	a, q := testApp(t)
	a.page = "翻译设置"
	calls := 0
	a.ports.SetTranslationSize = func(w, h int) {
		calls++
		if w != 350 || h != 420 {
			t.Fatal("unexpected native size", w, h)
		}
	}
	tt := ui.NewTester(a.View, 1100, 900)
	click(t, tt, "紧凑")
	click(t, tt, "保存设置")
	drainUntil(t, q, func() bool { return !a.saving })
	if calls != 1 || a.preferences.WindowSizeEdited {
		t.Fatal("same-default preset did not reset manual size")
	}
	a.draft.Appearance = "dark"
	a.saveSettings()
	drainUntil(t, q, func() bool { return !a.saving })
	if calls != 1 {
		t.Fatal("unrelated save reset manually sized window")
	}
	previous := a.settings.Clone()
	a.settings.TranslationWidth = 640
	a.ports.SetTranslationSize = func(w, h int) {
		calls++
		if w != 640 || h != 420 {
			t.Fatal("restore/default change not applied")
		}
	}
	if err := a.applyDesktopSettings(previous, false); err != nil || calls != 2 {
		t.Fatal("size change ignored", err)
	}
}
