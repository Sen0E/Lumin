package desktop

import (
	"testing"

	"github.com/egoist/mygo/ui"
	"lumin/internal/domain"
)

func numberInput(t *testing.T, tt *ui.Tester, label, text string) {
	t.Helper()
	r, ok := tt.Find(label)
	if !ok {
		t.Fatal("missing number input", label)
	}
	tt.ClickAt(r.X+20, r.Y+r.H/2)
	tt.Key(ui.Cmd, ui.KeyA)
	tt.Type(text)
	tt.Frame()
	tt.Frame()
}
func TestPreferencesEditSaveAndRestart(t *testing.T) {
	a, q := testApp(t)
	a.page = "设置"
	a.text = "unsaved translation input"
	tt := ui.NewTester(a.View, 1100, 800)
	numberInput(t, tt, "请求超时输入", "42")
	numberInput(t, tt, "历史上限输入", "123")
	click(t, tt, "默认源语言选择")
	click(t, tt, "English")
	click(t, tt, "默认目标语言选择")
	click(t, tt, "日本語")
	tt.Frame()
	tt.Frame()
	if a.draft.TimeoutSeconds != 42 || a.draft.HistoryLimit != 123 || a.draft.Source != "en" || a.draft.Target != "ja" {
		t.Fatalf("lost preference edits: %+v", a.draft)
	}
	if a.settings.TimeoutSeconds != 60 || a.source != domain.LanguageName("auto") {
		t.Fatal("draft changed active configuration")
	}
	click(t, tt, "保存设置")
	drainUntil(t, q, func() bool { return !a.saving })
	cfg, err := a.store.LoadConfig()
	if err != nil {
		t.Fatal(err)
	}
	b := New(a.store, cfg, nil, a.ports)
	defer b.Close()
	if b.source != "English" || b.target != "日本語" || b.settings.TimeoutSeconds != 42 || b.settings.HistoryLimit != 123 {
		t.Fatal("saved preferences not restored on restart")
	}
	if a.text != "unsaved translation input" || a.source != "自动检测" {
		t.Fatal("save overwrote current input or current language")
	}
}
func TestPreferencesBoundsAndDiscard(t *testing.T) {
	a, _ := testApp(t)
	a.page = "设置"
	tt := ui.NewTester(a.View, 920, 680)
	numberInput(t, tt, "请求超时输入", "20")
	numberInput(t, tt, "请求超时输入", "301")
	numberInput(t, tt, "历史上限输入", "0")
	if a.draft.TimeoutSeconds != 20 || a.draft.HistoryLimit != 500 {
		t.Fatal("out-of-range values committed")
	}
	click(t, tt, "放弃未保存修改")
	tt.Frame()
	tt.Frame()
	if a.draft.TimeoutSeconds != 60 || a.preferences.Timeout != 60 || a.preferences.History != 500 {
		t.Fatal("discard lost editor synchronization")
	}
	click(t, tt, "翻译服务")
	click(t, tt, "设置")
	tt.Frame()
	if a.preferences.Timeout != 60 {
		t.Fatal("navigation restored stale preferences")
	}
}
