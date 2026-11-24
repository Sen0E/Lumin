// Package capture acquires screen images and maps native selection coordinates
// back to original pixels. It has no UI dependency.
package capture

import (
	"errors"
	"image"
	"image/draw"
	_ "image/jpeg"
	"image/png"
	"math"
	"os"

	_ "golang.org/x/image/bmp"
	_ "golang.org/x/image/tiff"
	_ "golang.org/x/image/webp"
)

var ErrCanceled = errors.New("已取消截图")
var ErrUnavailable = errors.New("系统截图接口不可用")

const MaxPixels = 64_000_000

func ReadImage(path string) (image.Image, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	info, err := f.Stat()
	if err != nil {
		return nil, err
	}
	if !info.Mode().IsRegular() || info.Size() > 64<<20 {
		return nil, errors.New("截图文件过大或不是普通文件")
	}
	cfg, _, err := image.DecodeConfig(f)
	if err != nil {
		return nil, err
	}
	if cfg.Width <= 0 || cfg.Height <= 0 || int64(cfg.Width)*int64(cfg.Height) > MaxPixels {
		return nil, errors.New("截图分辨率超出限制")
	}
	if _, err = f.Seek(0, 0); err != nil {
		return nil, err
	}
	img, _, err := image.Decode(f)
	return img, err
}

// Selection uses normalized image coordinates, independent of monitor scale.
type Selection struct{ X0, Y0, X1, Y1 float64 }

func (s Selection) Pixels(bounds image.Rectangle) image.Rectangle {
	values := []float64{s.X0, s.Y0, s.X1, s.Y1}
	for _, v := range values {
		if math.IsNaN(v) || math.IsInf(v, 0) {
			return image.Rectangle{}
		}
	}
	x0, x1 := min(s.X0, s.X1), max(s.X0, s.X1)
	y0, y1 := min(s.Y0, s.Y1), max(s.Y0, s.Y1)
	x0 = max(0, min(1, x0))
	x1 = max(0, min(1, x1))
	y0 = max(0, min(1, y0))
	y1 = max(0, min(1, y1))
	return image.Rect(bounds.Min.X+int(math.Floor(x0*float64(bounds.Dx()))), bounds.Min.Y+int(math.Floor(y0*float64(bounds.Dy()))), bounds.Min.X+int(math.Ceil(x1*float64(bounds.Dx()))), bounds.Min.Y+int(math.Ceil(y1*float64(bounds.Dy())))).Intersect(bounds)
}

func Crop(img image.Image, s Selection) (image.Image, error) {
	r := s.Pixels(img.Bounds())
	if r.Dx() < 2 || r.Dy() < 2 {
		return nil, errors.New("请框选至少 2 × 2 像素的区域")
	}
	out := image.NewRGBA(image.Rect(0, 0, r.Dx(), r.Dy()))
	draw.Draw(out, out.Bounds(), img, r.Min, draw.Src)
	return out, nil
}

// TempPNG returns a private file and an idempotent cleanup function.
func TempPNG(img image.Image) (string, func(), error) {
	defer TracePhase("encode-selection-png")()
	f, err := os.CreateTemp("", "lumin-capture-*.png")
	if err != nil {
		return "", func() {}, err
	}
	cleanup := func() { _ = os.Remove(f.Name()) }
	// This file is short lived and local: favor latency over compressed size.
	encoder := png.Encoder{CompressionLevel: png.BestSpeed}
	if err := encoder.Encode(f, img); err != nil {
		_ = f.Close()
		cleanup()
		return "", func() {}, err
	}
	if err := f.Close(); err != nil {
		cleanup()
		return "", func() {}, err
	}
	return f.Name(), cleanup, nil
}
