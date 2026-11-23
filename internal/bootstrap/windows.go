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
	preferences, translator residentWindow
	quitting, capturing     bool
	restorePreferences      bool
	restoreTranslator       bool
}

func (w *windowSet) show(target residentWindow) {
	if target == nil || w.quitting {
		return
	}
	if w.capturing {
		if target == w.preferences {
			w.restorePreferences = true
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
	} else {
		w.restoreTranslator = false
	}
	target.Hide()
	return true
}

func (w *windowSet) beginCapture() {
	if w.quitting {
		return
	}
	w.capturing = true
	w.restorePreferences = w.preferences.IsVisible()
	w.restoreTranslator = w.translator.IsVisible()
	w.preferences.Hide()
	w.translator.Hide()
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
	w.restorePreferences, w.restoreTranslator = false, false
}
