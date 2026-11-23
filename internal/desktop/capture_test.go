package desktop

import (
	"context"
	"errors"
	"image"
	"os"
	"testing"
	"time"

	"lumin/internal/capture"
	"lumin/internal/config"
	"lumin/internal/domain"
)

type pathOCR struct{ path chan string }

func (o pathOCR) Recognize(ctx context.Context, path, lang string) (string, error) {
	if _, err := capture.ReadImage(path); err != nil {
		return "", err
	}
	o.path <- path
	return "Screenshot text", nil
}
func TestScreenshotOCRTranslateAndCleanup(t *testing.T) {
	a, q := testApp(t)
	paths := make(chan string, 1)
	a.ocr = pathOCR{paths}
	a.ports.CaptureArea = func(context.Context) (image.Image, error) { return image.NewRGBA(image.Rect(0, 0, 50, 30)), nil }
	a.ports.Translators = func(config.Config) []domain.Translator {
		return []domain.Translator{testTranslator{func(_ context.Context, r domain.Request) (string, error) { return r.Text + " translated", nil }}}
	}
	a.Screenshot(true)
	drainUntil(t, q, func() bool { return !a.busy && len(a.results) == 1 })
	if a.text != "Screenshot text" || a.results[0].Text != "Screenshot text translated" {
		t.Fatalf("%s %+v", a.text, a.results)
	}
	path := <-paths
	// The cleanup runs after publishing the completion; wait for the worker's defer.
	deadline := time.Now().Add(time.Second)
	for {
		_, err := os.Stat(path)
		if errors.Is(err, os.ErrNotExist) {
			break
		}
		if time.Now().After(deadline) {
			t.Fatal("screenshot temporary file leaked")
		}
		time.Sleep(time.Millisecond)
	}
}
func TestScreenshotCanceledResultCannotReplaceInput(t *testing.T) {
	a, q := testApp(t)
	a.ocr = testOCR{}
	started, release := make(chan struct{}), make(chan struct{})
	a.ports.CaptureArea = func(context.Context) (image.Image, error) { close(started); <-release; return nil, capture.ErrCanceled }
	a.Screenshot(false)
	<-started
	a.sourceChanged()
	a.text = "new input"
	close(release)
	select {
	case fn := <-q:
		fn()
	case <-time.After(time.Second):
		t.Fatal("no completion")
	}
	if a.text != "new input" || a.busy {
		t.Fatal("canceled capture overwrote newer input")
	}
}
