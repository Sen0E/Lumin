package capture

import (
	"image"
	"image/color"
	"testing"
)

func TestCropDisplayLayouts(t *testing.T) {
	tests := []struct {
		name   string
		size   image.Point
		target DisplayTarget
		want   image.Rectangle
	}{
		{
			name: "horizontal right",
			size: image.Pt(3840, 1080),
			target: DisplayTarget{
				Bounds:        image.Rect(1920, 0, 3840, 1080),
				VirtualBounds: image.Rect(0, 0, 3840, 1080),
				ScaleFactor:   1,
			},
			want: image.Rect(0, 0, 1920, 1080),
		},
		{
			name: "vertical above with negative y",
			size: image.Pt(1920, 2160),
			target: DisplayTarget{
				Bounds:        image.Rect(0, -1080, 1920, 0),
				VirtualBounds: image.Rect(0, -1080, 1920, 1080),
				ScaleFactor:   1,
			},
			want: image.Rect(0, 0, 1920, 1080),
		},
		{
			name: "negative x left display",
			size: image.Pt(5120, 1440),
			target: DisplayTarget{
				Bounds:        image.Rect(-2560, 0, 0, 1440),
				VirtualBounds: image.Rect(-2560, 0, 2560, 1440),
				ScaleFactor:   1,
			},
			want: image.Rect(0, 0, 2560, 1440),
		},
		{
			name: "scaled desktop",
			size: image.Pt(7680, 2160),
			target: DisplayTarget{
				Bounds:        image.Rect(1920, 0, 3840, 1080),
				VirtualBounds: image.Rect(0, 0, 3840, 1080),
				ScaleFactor:   2,
			},
			want: image.Rect(0, 0, 3840, 2160),
		},
		{
			name: "already one display",
			size: image.Pt(3840, 2160),
			target: DisplayTarget{
				Bounds:        image.Rect(1920, 0, 3840, 1080),
				VirtualBounds: image.Rect(0, 0, 3840, 1080),
				ScaleFactor:   2,
			},
			want: image.Rect(0, 0, 3840, 2160),
		},
		{
			name: "single display",
			size: image.Pt(2560, 1440),
			target: DisplayTarget{
				Bounds:        image.Rect(0, 0, 2560, 1440),
				VirtualBounds: image.Rect(0, 0, 2560, 1440),
				ScaleFactor:   1,
			},
			want: image.Rect(0, 0, 2560, 1440),
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			img := image.NewRGBA(image.Rect(0, 0, tt.size.X, tt.size.Y))
			got := CropDisplay(img, tt.target)
			if got.Bounds() != tt.want {
				t.Fatalf("bounds = %v, want %v", got.Bounds(), tt.want)
			}
		})
	}
}

func TestCropDisplayKeepsPixelsFromTarget(t *testing.T) {
	img := image.NewRGBA(image.Rect(0, 0, 4, 2))
	img.Set(2, 0, color.RGBA{R: 200, A: 255})
	got := CropDisplay(img, DisplayTarget{
		Bounds:        image.Rect(2, 0, 4, 2),
		VirtualBounds: image.Rect(0, 0, 4, 2),
		ScaleFactor:   1,
	})
	if got.Bounds() != image.Rect(0, 0, 2, 2) || got.At(0, 0) != (color.RGBA{R: 200, A: 255}) {
		t.Fatal("display crop lost its source pixels")
	}
}

func TestCropDisplayWithoutTargetKeepsImage(t *testing.T) {
	img := image.NewRGBA(image.Rect(0, 0, 4, 2))
	if got := CropDisplay(img, DisplayTarget{}); got != img {
		t.Fatal("missing display target changed the image")
	}
}
