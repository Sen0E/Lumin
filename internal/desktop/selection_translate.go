package desktop

import (
	"context"
	"errors"
	"strings"
	"time"

	"lumin/internal/selection"
)

// TranslateSelection must acquire the source selection before showing/focusing
// our window. It never uses Paste or a clipboard fallback.
func (a *App) TranslateSelection() {
	selection.Trace("trigger", 0, nil)
	if a.closed || a.backup.restoring || a.busy {
		return
	}
	a.cancelTranslationBlur()
	a.sourceChanged()
	if a.ports.ReadSelection == nil {
		a.text = ""
		a.ShowTranslation()
		a.status = selection.ErrUnavailable.Error()
		return
	}
	a.readingSelection = true
	a.busy = true
	a.status = "正在读取选中文字…"
	ctx, cancel := context.WithTimeout(a.root, 3*time.Second)
	a.cancel = cancel
	gen := a.generation
	go func() {
		defer cancel()
		text, err := a.ports.ReadSelection(ctx)
		if err == nil {
			err = ctx.Err()
		}
		if err == nil && strings.TrimSpace(text) == "" {
			err = selection.ErrEmpty
		}
		a.post(func() {
			if a.generation != gen {
				return
			}
			a.readingSelection = false
			a.busy, a.cancel = false, nil
			if errors.Is(err, context.Canceled) {
				return
			}
			if err != nil {
				a.text = ""
				a.ShowTranslation()
				a.status = err.Error()
				return
			}
			a.TranslateText(text)
		})
	}()
}
