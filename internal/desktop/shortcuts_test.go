package desktop

import (
	"testing"

	"github.com/egoist/mygo/ui"
)

func TestShortcutPlatformDisplay(t *testing.T) {
	for _, tc := range []struct{ platform, input, want string }{
		{"linux", "CmdOrCtrl+Alt+Space", "Ctrl + Alt + Space"},
		{"windows", "CmdOrCtrl+Alt+T", "Ctrl + Alt + T"},
		{"darwin", "CmdOrCtrl+Alt+S", "Cmd + Alt + S"},
		{"darwin", "Ctrl+Alt+S", "Ctrl + Alt + S"},
		{"linux", "", ""},
		{"linux", "Ctrl++", "Ctrl +  + "},
	} {
		if got := shortcutDisplayFor(tc.input, tc.platform); got != tc.want {
			t.Errorf("%s %q: %q, want %q", tc.platform, tc.input, got, tc.want)
		}
	}
}

func TestShortcutEditorPersistsAcrossFramesAndDiscard(t *testing.T) {
	a, q := testApp(t)
	a.page = "热键设置"
	original := a.draft.InputShortcut
	tt := ui.NewTester(a.View, 1100, 800)
	tt.Frame()
	tt.Frame()
	if a.preferences.InputShortcut != shortcutDisplay(original) || a.draft.InputShortcut != original {
		t.Fatal("display rewrote portable config or exposed raw accelerator")
	}
	edit := func(text string) {
		t.Helper()
		r, ok := tt.Find("打开翻译窗口快捷键输入")
		if !ok {
			t.Fatal("missing shortcut editor")
		}
		tt.ClickAt(r.X+20, r.Y+r.H/2)
		tt.Key(ui.Cmd, ui.KeyA)
		tt.Type(text)
		tt.Frame()
		tt.Frame()
	}
	edit(shortcutDisplay(original))
	if a.draft.InputShortcut != original {
		t.Fatal("retyping display discarded portable alias")
	}
	edit("Ctrl + Alt + K")
	if a.draft.InputShortcut != "Ctrl+Alt+K" || a.settings.InputShortcut != original {
		t.Fatalf("editor lost changes or changed active settings: editor=%q draft=%q active=%q", a.preferences.InputShortcut, a.draft.InputShortcut, a.settings.InputShortcut)
	}
	click(t, tt, "翻译设置")
	click(t, tt, "热键设置")
	tt.Frame()
	if a.preferences.InputShortcut != "Ctrl + Alt + K" {
		t.Fatal("navigation lost shortcut edit")
	}
	click(t, tt, "保存设置")
	drainUntil(t, q, func() bool { return !a.saving })
	cfg, err := a.store.LoadConfig()
	if err != nil || cfg.InputShortcut != "Ctrl+Alt+K" {
		t.Fatalf("save: %v %+v", err, cfg)
	}
	edit("Ctrl + Alt + J")
	click(t, tt, "放弃未保存修改")
	tt.Frame()
	if a.preferences.InputShortcut != "Ctrl + Alt + K" || a.draft.InputShortcut != cfg.InputShortcut {
		t.Fatal("discard failed to resynchronize shortcut editor")
	}
	edit("")
	if a.draft.InputShortcut != "" {
		t.Fatal("cannot disable shortcut")
	}
}
