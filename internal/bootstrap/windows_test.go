package bootstrap

import "testing"

type fakeWindow struct {
	visible        bool
	shown, focused int
}

func (w *fakeWindow) Show()           { w.visible = true; w.shown++ }
func (w *fakeWindow) ShowInactive()   { w.visible = true; w.shown++ }
func (w *fakeWindow) Hide()           { w.visible = false }
func (w *fakeWindow) Focus()          { w.focused++ }
func (w *fakeWindow) IsVisible() bool { return w.visible }

func TestResidentWindowsCloseReopenAndQuit(t *testing.T) {
	settings, translation := &fakeWindow{visible: true}, &fakeWindow{}
	w := windowSet{preferences: settings, translator: translation}
	if !w.hideOnClose(settings) || settings.visible {
		t.Fatal("settings close must hide and prevent destruction")
	}
	w.show(translation)
	if settings.visible || !translation.visible {
		t.Fatal("translation must not reveal settings")
	}
	if !w.hideOnClose(translation) || translation.visible {
		t.Fatal("translation close must hide")
	}
	w.show(settings) // second instance / tray settings
	if !settings.visible || translation.visible {
		t.Fatal("settings cannot be recovered independently")
	}
	w.quitting = true
	if w.hideOnClose(settings) || w.hideOnClose(translation) {
		t.Fatal("resident windows blocked explicit quit")
	}
	settings.Hide()
	w.show(settings)
	if settings.visible {
		t.Fatal("window reopened during quit")
	}
}

func TestCaptureRestoresOnlyPreviouslyVisibleWindows(t *testing.T) {
	for _, prefs := range []bool{false, true} {
		for _, trans := range []bool{false, true} {
			settings, translation := &fakeWindow{visible: prefs}, &fakeWindow{visible: trans}
			w := windowSet{preferences: settings, translator: translation}
			w.beginCapture()
			if settings.visible || translation.visible {
				t.Fatal("Lumin would appear in screenshot")
			}
			w.endCapture()
			if settings.visible != prefs || translation.visible != trans {
				t.Fatal("capture opened a previously hidden window")
			}
			if settings.focused != 0 || translation.focused != 0 {
				t.Fatal("restoration stole focus")
			}
		}
	}
}

func TestCaptureDefersRequestsAndHonorsDismissalAndQuit(t *testing.T) {
	settings, translation := &fakeWindow{}, &fakeWindow{}
	w := windowSet{preferences: settings, translator: translation}
	w.beginCapture()
	w.show(translation)
	if translation.visible {
		t.Fatal("request exposed window during capture")
	}
	w.endCapture()
	if !translation.visible || settings.visible {
		t.Fatal("deferred translation request routed incorrectly")
	}
	w.beginCapture()
	w.hideOnClose(translation)
	w.endCapture()
	if translation.visible {
		t.Fatal("capture restored dismissed window")
	}
	w.show(settings)
	w.beginCapture()
	w.quitting = true
	w.endCapture()
	if settings.visible || translation.visible {
		t.Fatal("capture reopened windows during quit")
	}
}
