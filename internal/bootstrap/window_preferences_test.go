package bootstrap

import (
	"lumin/internal/config"
	"testing"
)

func TestTranslationBlurIgnoresCaptureRefocusHiddenAndQuit(t *testing.T) {
	for _, tc := range []struct {
		name                                string
		focus, visible, capture, quit, want bool
	}{
		{"outside", false, true, false, false, true}, {"focus returned", true, true, false, false, false},
		{"hidden", false, false, false, false, false}, {"screenshot", false, true, true, false, false}, {"quit", false, true, false, true, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			w := windowSet{translator: &fakeWindow{visible: tc.visible}, capturing: tc.capture, quitting: tc.quit}
			called := false
			w.translationLostFocus(tc.focus, func() { called = true })
			if called != tc.want {
				t.Fatal("incorrect blur routing")
			}
		})
	}
}
func TestWindowSizeOptionsAndSettingsApplyChanges(t *testing.T) {
	old := config.Default()
	next := old.Clone()
	next.Appearance = "dark"
	next.TranslationFontSize = 20
	if shortcutsChanged(old, next) {
		t.Fatal("appearance edit would reset size/hotkeys")
	}
	next.TranslationWidth = 640
	next.TranslationHeight = 720
	if shortcutsChanged(old, next) {
		t.Fatal("size change not isolated")
	}
	opts := translationWindowOptions(next)
	if opts.Width != 640 || opts.Height != 720 || opts.MinWidth != 320 || opts.MinHeight != 360 || opts.StateKey != "translation" || !opts.Hidden {
		t.Fatal("invalid native window policy")
	}
	next.SelectionShortcut = "Ctrl+Alt+Y"
	if !shortcutsChanged(old, next) {
		t.Fatal("hotkey edit ignored")
	}
}
