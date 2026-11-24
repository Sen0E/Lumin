package capture

import (
	"image"
	"image/color"
	"math"
	"os"
	"testing"
)

func TestSelectionOriginalPixels(t *testing.T) {
	bounds := image.Rect(10, 20, 2010, 1020)
	for _, s := range []Selection{{.25, .2, .75, .8}, {.75, .8, .25, .2}} {
		if got := s.Pixels(bounds); got != image.Rect(510, 220, 1510, 820) {
			t.Fatal(got)
		}
	}
	if got := (Selection{-1, -1, 2, 2}).Pixels(bounds); got != bounds {
		t.Fatal(got)
	}
	if got := (Selection{math.NaN(), 0, 1, 1}).Pixels(bounds); !got.Empty() {
		t.Fatal(got)
	}
	img := image.NewRGBA(bounds)
	img.Set(510, 220, color.RGBA{R: 250, A: 255})
	crop, err := Crop(img, Selection{.25, .2, .75, .8})
	if err != nil {
		t.Fatal(err)
	}
	if crop.Bounds() != image.Rect(0, 0, 1000, 600) || crop.At(0, 0) != (color.RGBA{R: 250, A: 255}) {
		t.Fatal("crop lost pixel coordinates")
	}
	if _, err := Crop(img, Selection{.5, .5, .5, .5}); err == nil {
		t.Fatal("accepted empty selection")
	}
	file, cleanup, err := TempPNG(crop)
	if err != nil {
		t.Fatal(err)
	}
	defer cleanup()
	st, _ := os.Stat(file)
	if st.Mode().Perm()&0077 != 0 {
		t.Fatal("temporary screenshot is public")
	}
	read, err := ReadImage(file)
	if err != nil || read.Bounds() != crop.Bounds() {
		t.Fatalf("round trip %v", err)
	}
	cleanup()
	cleanup()
	if _, err := os.Stat(file); !os.IsNotExist(err) {
		t.Fatal("temporary screenshot leaked")
	}
}

// Screenshot-like RGB gradients and text edges exercise the real temporary
// file path; keep this benchmark when changing image formats or compression.
func BenchmarkTempPNGFullHD(b *testing.B) {
	img := image.NewRGBA(image.Rect(0, 0, 1920, 1080))
	for y := 0; y < 1080; y++ {
		for x := 0; x < 1920; x++ {
			i := img.PixOffset(x, y)
			v := byte((x/8 + y/16) % 256)
			img.Pix[i], img.Pix[i+1], img.Pix[i+2], img.Pix[i+3] = v, byte(x/16), byte(y/8), 255
		}
	}
	b.ResetTimer()
	for b.Loop() {
		_, cleanup, err := TempPNG(img)
		cleanup()
		if err != nil {
			b.Fatal(err)
		}
	}
}
