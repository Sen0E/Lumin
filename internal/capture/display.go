package capture

import (
	"image"
	"image/draw"
	"math"
)

// DisplayTarget identifies the display under the pointer when a capture
// starts. Coordinates use the same screen space as the native display API.
// VirtualBounds is the union of every display and is used to crop a desktop
// image when the portal or a helper returns all displays at once.
type DisplayTarget struct {
	ID            int64
	Bounds        image.Rectangle
	ScaleFactor   float64
	VirtualBounds image.Rectangle
}

func (t DisplayTarget) valid() bool {
	return !t.Bounds.Empty() && t.Bounds.Dx() > 0 && t.Bounds.Dy() > 0
}

// CropDisplay returns the image for the target display. It accepts a capture
// of only that display as well as a scaled capture of the whole desktop.
func CropDisplay(img image.Image, target DisplayTarget) image.Image {
	if img == nil || !target.valid() {
		return img
	}
	bounds := img.Bounds()
	if bounds.Empty() {
		return img
	}
	targetWidth, targetHeight := target.Bounds.Dx(), target.Bounds.Dy()
	if bounds.Dx() == targetWidth && bounds.Dy() == targetHeight {
		return img
	}
	if target.ScaleFactor > 0 &&
		closeDimension(float64(bounds.Dx()), float64(targetWidth)*target.ScaleFactor) &&
		closeDimension(float64(bounds.Dy()), float64(targetHeight)*target.ScaleFactor) {
		return img
	}

	virtual := target.VirtualBounds
	if virtual.Empty() {
		virtual = target.Bounds
	}
	if virtual.Dx() <= 0 || virtual.Dy() <= 0 || !virtual.Overlaps(target.Bounds) {
		return img
	}
	if virtual == target.Bounds {
		return img
	}

	scaleX := float64(bounds.Dx()) / float64(virtual.Dx())
	scaleY := float64(bounds.Dy()) / float64(virtual.Dy())
	crop := image.Rect(
		bounds.Min.X+int(math.Round(float64(target.Bounds.Min.X-virtual.Min.X)*scaleX)),
		bounds.Min.Y+int(math.Round(float64(target.Bounds.Min.Y-virtual.Min.Y)*scaleY)),
		bounds.Min.X+int(math.Round(float64(target.Bounds.Max.X-virtual.Min.X)*scaleX)),
		bounds.Min.Y+int(math.Round(float64(target.Bounds.Max.Y-virtual.Min.Y)*scaleY)),
	).Intersect(bounds)
	if crop.Empty() || crop == bounds {
		return img
	}
	out := image.NewRGBA(image.Rect(0, 0, crop.Dx(), crop.Dy()))
	draw.Draw(out, out.Bounds(), img, crop.Min, draw.Src)
	return out
}

func closeDimension(got, want float64) bool {
	return math.Abs(got-want) <= 1
}
