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

func selectionImage() image.Image {
	img := image.NewRGBA(image.Rect(0, 0, 1600, 900))
	for y := 0; y < 900; y++ {
		for x := 0; x < 1600; x++ {
			img.SetRGBA(x, y, color.RGBA{R: uint8(x / 8), G: uint8(y / 4), B: 120, A: 255})
		}
	}
	return img
}
func TestSelectionDragResizeAndCancel(t *testing.T) {
	img := selectionImage()
	count := 0
	var chosen capture.Selection
	var resultErr error
	s := NewSelection(img, func(sel capture.Selection, err error) { count++; chosen = sel; resultErr = err })
	tt := ui.NewTester(s.View, 1280, 720)
	tt.Frame()
	tt.Frame()
	canvas, ok := tt.Find("截图框选画布")
	if !ok || canvas.X != 0 || canvas.Y != 0 || canvas.W != 1280 || canvas.H != 720 {
		t.Fatal("canvas does not fill viewport", canvas)
	}
	for _, name := range []string{"识别选区", "取消截图"} {
		if _, ok := tt.Find(name); ok {
			t.Fatal("toolbar still present", name)
		}
	}
	tt.Key(0, ui.KeyEnter)
	if count != 0 {
		t.Fatal("empty selection submitted")
	}
	tt.Press(1024, 576)
	tt.Move(256, 144)
	if count != 0 {
		t.Fatal("submitted before release")
	}
	pixels := s.selection.Pixels(img.Bounds())
	if pixels != image.Rect(320, 180, 1280, 720) {
		t.Fatal("wrong original pixels", pixels)
	}
	if dir := os.Getenv("LUMIN_TEST_SCREENSHOTS"); dir != "" {
		if err := os.MkdirAll(dir, 0755); err != nil {
			t.Fatal(err)
		}
		f, err := os.Create(filepath.Join(dir, "selection.png"))
		if err != nil {
			t.Fatal(err)
		}
		err = png.Encode(f, tt.Image())
		f.Close()
		if err != nil {
			t.Fatal(err)
		}
	}
	tt.Release(256, 144)
	if count != 1 || resultErr != nil || chosen.Pixels(img.Bounds()) != pixels {
		t.Fatal("release did not auto-submit", count, resultErr)
	}
	tt.Key(0, ui.KeyEnter)
	tt.Key(0, ui.KeyEscape)
	tt.RightClickAt(40, 40)
	if count != 1 {
		t.Fatal("completion repeated after release")
	}
	tt.SetSize(920, 680)
	tt.Frame()
	tt.Frame()
	if s.selection.Pixels(img.Bounds()) != pixels {
		t.Fatal("resize changed source coordinates")
	}
}
func TestSelectionPointerCanceledAndTinyClickCanRetry(t *testing.T) {
	count := 0
	s := NewSelection(selectionImage(), func(capture.Selection, error) { count++ })
	tt := ui.NewTester(s.View, 1280, 720)
	tt.ClickAt(100, 100)
	if count != 0 || s.finished {
		t.Fatal("zero-area click submitted")
	}
	tt.Press(100, 100)
	tt.Move(300, 300)
	tt.CancelPress()
	if count != 0 || s.dragging || !s.selection.Pixels(s.bounds).Empty() {
		t.Fatal("pointer cancellation retained selection or submitted")
	}
	tt.Press(100, 100)
	tt.Move(300, 300)
	tt.Release(300, 300)
	if count != 1 {
		t.Fatal("retry after canceled pointer failed")
	}
}
func TestSelectionCancelWithRightClickAndEscape(t *testing.T) {
	for _, right := range []bool{false, true} {
		count := 0
		var resultErr error
		s := NewSelection(selectionImage(), func(_ capture.Selection, err error) { count++; resultErr = err })
		tt := ui.NewTester(s.View, 1280, 720)
		tt.Press(100, 100)
		tt.Move(300, 300)
		if right {
			tt.RightClickAt(200, 200)
		} else {
			tt.Key(0, ui.KeyEscape)
		}
		tt.Release(300, 300)
		tt.Key(0, ui.KeyEscape)
		if count != 1 || !errors.Is(resultErr, capture.ErrCanceled) {
			t.Fatal("cancel did not finish once", right, count, resultErr)
		}
	}
}
func TestSelectionEdgesAndLetterboxMapping(t *testing.T) {
	for _, size := range []struct{ w, h int }{{1280, 720}, {920, 680}} {
		count := 0
		var chosen capture.Selection
		s := NewSelection(selectionImage(), func(sel capture.Selection, err error) {
			if err != nil {
				t.Fatal(err)
			}
			count++
			chosen = sel
		})
		tt := ui.NewTester(s.View, size.w, size.h)
		canvas, _ := tt.Find("截图框选画布")
		fit := fitImage(canvas, s.bounds)
		if fit.Y > 0 {
			tt.ClickAt(10, 1)
			if s.dragging || count != 0 {
				t.Fatal("letterbox started selection")
			}
		}
		tt.Press(fit.X, fit.Y)
		tt.Move(fit.X+fit.W-1, fit.Y+fit.H-1)
		// Captured pointer clamps release outside the image to its last pixel edge.
		tt.Release(fit.X+fit.W+20, fit.Y+fit.H+20)
		if count != 1 || chosen.Pixels(s.bounds) != s.bounds {
			t.Fatal("screen edge mapping", size, count, chosen.Pixels(s.bounds))
		}
	}
}
