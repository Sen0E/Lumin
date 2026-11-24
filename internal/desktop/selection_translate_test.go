package desktop

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/egoist/mygo/ui"
	"lumin/internal/config"
	"lumin/internal/domain"
	"lumin/internal/selection"
)

func TestSelectionAcquiredBeforePopupAndTranslatedWithoutClipboard(t *testing.T) {
	a, q := testApp(t)
	started, release := make(chan struct{}), make(chan struct{})
	shown, preferences := 0, 0
	a.ports.ShowTranslator = func() { shown++ }
	a.ports.ShowPreferences = func() { preferences++ }
	a.ports.ReadClipboard = func() string { t.Error("selection read clipboard"); return "stale clipboard" }
	a.ports.WriteClipboard = func(string) { t.Error("selection overwrote clipboard") }
	a.ports.ReadSelection = func(context.Context) (string, error) { close(started); <-release; return "fresh selected 世界", nil }
	a.ports.Translators = func(config.Config) []domain.Translator {
		return []domain.Translator{testTranslator{func(_ context.Context, r domain.Request) (string, error) { return r.Text + " translated", nil }}}
	}
	a.TranslateSelection()
	<-started
	if shown != 0 {
		t.Fatal("popup stole source focus before selection read")
	}
	a.TranslationBlurred() // A delayed source blur must not cancel acquisition.
	if !a.busy || !a.readingSelection {
		t.Fatal("source blur canceled pending selection")
	}
	a.TranslateSelection() // Busy re-entry must not start another reader.
	close(release)
	drainUntil(t, q, func() bool { return !a.busy && len(a.results) == 1 })
	if shown != 1 || preferences != 0 || a.text != "fresh selected 世界" || a.results[0].Text != "fresh selected 世界 translated" {
		t.Fatalf("wrong destination: shown=%d text=%q results=%+v", shown, a.text, a.results)
	}
}
func TestEmptyOrUnsupportedSelectionNeverTranslatesOldInput(t *testing.T) {
	for _, problem := range []error{nil, selection.ErrEmpty, selection.ErrUnavailable} {
		a, q := testApp(t)
		a.text = "old input"
		shown, calls := 0, 0
		a.ports.ShowTranslator = func() { shown++ }
		a.ports.ReadSelection = func(context.Context) (string, error) { return "", problem }
		a.ports.ReadClipboard = func() string { t.Error("fallback to old clipboard"); return "stale" }
		a.ports.Translators = func(config.Config) []domain.Translator { calls++; return nil }
		a.TranslateSelection()
		drainUntil(t, q, func() bool { return !a.busy })
		if calls != 0 || shown != 1 || a.text != "" {
			t.Fatal("empty/failed selection retained or translated existing content")
		}
	}
}
func TestSelectionLateResultCannotReopenDismissedWindow(t *testing.T) {
	a, q := testApp(t)
	started, release := make(chan struct{}), make(chan struct{})
	shown := 0
	a.ports.ShowTranslator = func() { shown++ }
	a.ports.ReadSelection = func(context.Context) (string, error) { close(started); <-release; return "late", nil }
	a.TranslateSelection()
	<-started
	a.DismissTranslation()
	a.text = "new input"
	close(release)
	select {
	case fn := <-q:
		fn()
	case <-time.After(time.Second):
		t.Fatal("missing completion")
	}
	if shown != 0 || a.text != "new input" || a.busy {
		t.Fatal("late selection escaped cancellation")
	}
}
func TestSelectionReaderCancellationAndHotkeyLabel(t *testing.T) {
	a, q := testApp(t)
	started := make(chan struct{})
	a.ports.ReadSelection = func(ctx context.Context) (string, error) { close(started); <-ctx.Done(); return "", ctx.Err() }
	a.TranslateSelection()
	<-started
	a.Cancel()
	select {
	case fn := <-q:
		fn()
	case <-time.After(time.Second):
		t.Fatal("reader not canceled")
	}
	if a.busy {
		t.Fatal("still busy")
	}
	a.page = "热键设置"
	tt := ui.NewTester(a.View, 1100, 800)
	if !tt.HasText("选中文字翻译快捷键") || tt.HasText("翻译剪贴板快捷键") {
		t.Fatal("hotkey label still describes clipboard")
	}
	a.ports.ReadSelection = func(context.Context) (string, error) { return "", errors.New("permission denied") }
	a.TranslateSelection()
	drainUntil(t, q, func() bool { return !a.busy })
	if a.status != "permission denied" {
		t.Fatal(a.status)
	}
}
