package bootstrap

// residentWindow is the native surface owned by the application, not a view.
// All windowSet operations run on the UI thread, including capture restoration.
type residentWindow interface {
	Show()
	ShowInactive()
	Hide()
	Focus()
	IsVisible() bool
}

type windowSet struct {
	preferences, translator, recognizer residentWindow
	quitting, capturing                 bool
	restorePreferences                  bool
	restoreTranslator                   bool
	restoreRecognizer                   bool
}

func (w *windowSet) show(target residentWindow) {
	if target == nil || w.quitting {
		return
	}
	if w.capturing {
		if target == w.preferences {
			w.restorePreferences = true
		} else if target == w.recognizer {
			w.restoreRecognizer = true
		} else {
			w.restoreTranslator = true
		}
		return
	}
	target.Show()
	target.Focus()
}

// hideOnClose reports whether the native close must be prevented. An explicit
// application quit must pass through to native destruction on every platform.
func (w *windowSet) hideOnClose(target residentWindow) bool {
	if w.quitting {
		return false
	}
	if target == w.preferences {
		w.restorePreferences = false
	} else if target == w.recognizer {
		w.restoreRecognizer = false
	} else {
		w.restoreTranslator = false
	}
	target.Hide()
	return true
}

func (w *windowSet) beginCapture() bool {
	if w.quitting {
		return false
	}
	w.capturing = true
	w.restorePreferences = w.preferences.IsVisible()
	w.restoreTranslator = w.translator.IsVisible()
	if w.recognizer != nil {
		w.restoreRecognizer = w.recognizer.IsVisible()
		w.recognizer.Hide()
	}
	w.preferences.Hide()
	w.translator.Hide()
	return w.restorePreferences || w.restoreTranslator || w.restoreRecognizer
}

func (w *windowSet) endCapture() {
	w.capturing = false
	if w.quitting {
		return
	}
	// Restoring a settings window must not steal focus from the selected app.
	if w.restorePreferences {
		w.preferences.ShowInactive()
	}
	if w.restoreTranslator {
		w.translator.ShowInactive()
	}
	if w.restoreRecognizer && w.recognizer != nil {
		w.recognizer.ShowInactive()
	}
	w.restorePreferences, w.restoreTranslator, w.restoreRecognizer = false, false, false
}

// Ignore focus transitions caused by capture surfaces, quit or already-hidden
// windows. The desktop owns pin/auto-hide preferences and cancellation.
func (w *windowSet) translationLostFocus(focused bool, dismiss func()) {
	if focused || w.capturing || w.quitting || w.translator == nil || !w.translator.IsVisible() {
		return
	}
	dismiss()
}
