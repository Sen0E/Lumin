package desktop

import (
	"errors"
	"image"
	"image/color"
	"image/png"
	"os"
	"path/filepath"
	"testing"

	"github.com/egoist/mygo/ui"
	"lumin/internal/capture"
)

func TestSelectionDragResizeAndCancel(t *testing.T) {
	img := image.NewRGBA(image.Rect(0, 0, 2000, 1000))
	for y := 0; y < 1000; y++ {
		for x := 0; x < 2000; x++ {
			img.SetRGBA(x, y, color.RGBA{R: uint8(x / 8), G: uint8(y / 4), B: 120, A: 255})
		}
	}
	var chosen capture.Selection
	var canceled error
	count := 0
	s := NewSelection(img, func(sel capture.Selection, err error) { chosen = sel; canceled = err; count++ })
	tt := ui.NewTester(s.View, 1100, 800)
	tt.Key(0, ui.KeyEnter)
	if count != 0 {
		t.Fatal("empty selection confirmed")
	}
	canvas, ok := tt.Find("截图框选画布")
	if !ok {
		t.Fatal("missing canvas")
	}
	fit := fitImage(canvas, img.Bounds())
	tt.Press(fit.X+fit.W*.8, fit.Y+fit.H*.8)
	tt.Move(fit.X+fit.W*.2, fit.Y+fit.H*.2)
	tt.Release(fit.X+fit.W*.2, fit.Y+fit.H*.2)
	pixels := s.selection.Pixels(img.Bounds())
	if pixels.Dx() < 1198 || pixels.Dx() > 1202 || pixels.Dy() < 598 || pixels.Dy() > 602 {
		t.Fatalf("coordinates %+v => %v canvas=%v", s.selection, pixels, canvas)
	}
	tt.SetSize(920, 680)
	tt.Frame()
	tt.Frame()
	if s.selection.Pixels(img.Bounds()) != pixels {
		t.Fatal("resize changed original pixel selection")
	}
	if dir := os.Getenv("LUMIN_TEST_SCREENSHOTS"); dir != "" {
		if err := os.MkdirAll(dir, 0755); err != nil {
			t.Fatal(err)
		}
		f, err := os.Create(filepath.Join(dir, "selection.png"))
		if err != nil {
			t.Fatal(err)
		}
		defer f.Close()
		if err := png.Encode(f, tt.Image()); err != nil {
			t.Fatal(err)
		}
	}
	tt.Key(0, ui.KeyEnter)
	if count != 1 || canceled != nil || chosen.Pixels(img.Bounds()) != pixels {
		t.Fatal("confirm failed")
	}
	canvas, _ = tt.Find("截图框选画布")
	fit = fitImage(canvas, img.Bounds())
	tt.Press(fit.X+20, fit.Y+20)
	tt.Move(fit.X+100, fit.Y+100)
	tt.CancelPress()
	if !s.selection.Pixels(img.Bounds()).Empty() {
		t.Fatal("canceled pointer retained selection")
	}
	tt.Key(0, ui.KeyEscape)
	if !errors.Is(canceled, capture.ErrCanceled) {
		t.Fatal("Escape failed")
	}
}
