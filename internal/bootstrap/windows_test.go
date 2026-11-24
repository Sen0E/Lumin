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

func TestRecognitionWindowCaptureAndLifecycle(t *testing.T) {
	for _, visible := range []bool{false, true} {
		recognition := &fakeWindow{visible: visible}
		w := windowSet{preferences: &fakeWindow{}, translator: &fakeWindow{}, recognizer: recognition}
		w.beginCapture()
		if recognition.visible {
			t.Fatal("recognition included in screenshot")
		}
		w.endCapture()
		if recognition.visible != visible || recognition.focused != 0 {
			t.Fatal("wrong recognition restore")
		}
		w.beginCapture()
		w.show(recognition)
		if recognition.visible {
			t.Fatal("show during screenshot")
		}
		w.endCapture()
		if !recognition.visible {
			t.Fatal("deferred OCR lost")
		}
		w.beginCapture()
		w.hideOnClose(recognition)
		w.endCapture()
		if recognition.visible {
			t.Fatal("dismissed recognition restored")
		}
		w.quitting = true
		if w.hideOnClose(recognition) {
			t.Fatal("recognition blocked quit")
		}
	}
}

func TestCaptureOnlyWaitsWhenVisibleSurfaceWasHidden(t *testing.T) {
	for bits := 0; bits < 8; bits++ {
		w := windowSet{
			preferences: &fakeWindow{visible: bits&1 != 0},
			translator:  &fakeWindow{visible: bits&2 != 0},
			recognizer:  &fakeWindow{visible: bits&4 != 0},
		}
		if needsSettle := w.beginCapture(); needsSettle != (bits != 0) {
			t.Fatalf("visibility %03b: settle=%v", bits, needsSettle)
		}
		if w.preferences.IsVisible() || w.translator.IsVisible() || w.recognizer.IsVisible() {
			t.Fatal("visible window was not hidden")
		}
	}
}
