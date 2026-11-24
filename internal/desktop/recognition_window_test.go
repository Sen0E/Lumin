package desktop

import (
	"context"
	"image"
	"image/color"
	"image/png"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/egoist/mygo/ui"
	"lumin/internal/capture"
)

type blockingOCR struct{ started, release chan struct{} }

func (b blockingOCR) Recognize(context.Context, string, string) (string, error) {
	close(b.started)
	<-b.release
	return "late result", nil
}

func TestRecognitionCloseRejectsLateResultAndLeavesTranslationRunning(t *testing.T) {
	a, q := testApp(t)
	b := blockingOCR{make(chan struct{}), make(chan struct{})}
	a.ocr = b
	a.busy, a.text = true, "translation in progress"
	shown := 0
	a.ports.ShowRecognizer = func() { shown++ }
	a.startRecognition(false, image.NewRGBA(image.Rect(0, 0, 30, 20)))
	<-b.started
	// Apply preview before closing; queued completion must not reopen the window.
	(<-q)()
	a.DismissRecognition()
	before := shown
	close(b.release)
	select {
	case fn := <-q:
		fn()
	case <-time.After(time.Second):
		t.Fatal("missing canceled completion")
	}
	if shown != before || !a.busy || a.text != "translation in progress" || a.recognition.text != "" {
		t.Fatal("OCR dismissal changed translation or published stale text")
	}
}

func TestRecognitionRoutingRetryAndClipboard(t *testing.T) {
	a, q := testApp(t)
	a.ocr = testOCR{}
	a.text = "existing translation"
	a.status = "translation status"
	translated, shown := 0, 0
	a.ports.ShowTranslator = func() { translated++ }
	a.ports.ShowRecognizer = func() { shown++ }
	a.ports.CaptureArea = func(context.Context) (image.Image, error) { return image.NewRGBA(image.Rect(0, 0, 30, 20)), nil }
	a.Screenshot(false)
	drainUntil(t, q, func() bool { return !a.recognition.busy })
	if shown == 0 || translated != 0 || a.text != "existing translation" || a.recognition.text != "Recognized text" {
		t.Fatal("wrong OCR destination")
	}
	a.startRecognition(false, a.recognition.image)
	drainUntil(t, q, func() bool { return !a.recognition.busy })
	clip := ""
	a.ports.WriteClipboard = func(s string) { clip = s }
	a.ports.ReadClipboard = func() string { return clip }
	tt := ui.NewTester(a.RecognitionView, 480, 560)
	tt.Frame()
	click(t, tt, "复制文本")
	tt.Frame()
	if clip != "Recognized text" || translated != 0 || a.status != "translation status" {
		t.Fatal("copy leaked to translator")
	}
	a.recognition.text = "edited result"
	tt.Frame()
	click(t, tt, "翻译文本")
	tt.Frame()
	if translated != 1 || a.text != "edited result" {
		t.Fatal("edited OCR did not transfer")
	}
}

func TestRecognitionCancelCaptureKeepsWindowsHidden(t *testing.T) {
	a, q := testApp(t)
	a.ocr = testOCR{}
	shown := 0
	a.ports.ShowRecognizer = func() { shown++ }
	a.ports.CaptureArea = func(context.Context) (image.Image, error) { return nil, capture.ErrCanceled }
	a.Screenshot(false)
	drainUntil(t, q, func() bool { return !a.recognition.busy })
	if shown != 0 {
		t.Fatal("canceled capture revealed OCR window")
	}
}

func TestRecognitionLayout(t *testing.T) {
	a, _ := testApp(t)
	img := image.NewRGBA(image.Rect(0, 0, 320, 80))
	for y := 0; y < 80; y++ {
		for x := 0; x < 320; x++ {
			img.Set(x, y, color.RGBA{230, 235, 245, 255})
		}
	}
	a.recognition.image, a.recognition.bitmap = img, ui.NewBitmap(img)
	a.recognition.text, a.recognition.status = "Hello 世界\n识别结果可以编辑后翻译。", "识别完成，可编辑、复制或转入翻译"
	tt := ui.NewTester(a.RecognitionView, 480, 560)
	for _, busy := range []bool{false, true, false} {
		a.recognition.busy = busy
		tt.Frame()
		tt.Frame()
		labels := []string{"识别图片预览", "识别结果", "选择图片", "截图识字", "重新识别", "复制文本", "去换行", "翻译文本", "关闭"}
		if busy {
			labels = append(labels, "取消")
		}
		for _, label := range labels {
			b, ok := tt.Find(label)
			if !ok || b.W <= 0 || b.H <= 0 || b.X < 0 || b.Y < 0 || b.X+b.W > 480 || b.Y+b.H > 560 {
				t.Fatalf("%s: %+v", label, b)
			}
		}
		if dir := os.Getenv("LUMIN_TEST_SCREENSHOTS"); dir != "" && !busy {
			if err := os.MkdirAll(dir, 0755); err != nil {
				t.Fatal(err)
			}
			f, err := os.Create(filepath.Join(dir, "recognition.png"))
			if err != nil {
				t.Fatal(err)
			}
			err = png.Encode(f, tt.Image())
			f.Close()
			if err != nil {
				t.Fatal(err)
			}
		}
	}
	a.recognition.busy = false
}
