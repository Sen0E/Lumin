package bootstrap

import (
	"image"

	"github.com/egoist/mygo"
	"lumin/internal/capture"
)

func captureDisplayTarget() capture.DisplayTarget {
	display := mygo.Screen.DisplayNearestPoint(mygo.Screen.CursorScreenPoint())
	bounds := screenBounds(display.Bounds)
	var virtual image.Rectangle
	for _, current := range mygo.Screen.Displays() {
		currentBounds := screenBounds(current.Bounds)
		if currentBounds.Empty() {
			continue
		}
		if virtual.Empty() {
			virtual = currentBounds
		} else {
			virtual = virtual.Union(currentBounds)
		}
	}
	return capture.DisplayTarget{
		ID:            display.ID,
		Bounds:        bounds,
		ScaleFactor:   display.ScaleFactor,
		VirtualBounds: virtual,
	}
}

func screenBounds(r mygo.Rectangle) image.Rectangle {
	if r.Width <= 0 || r.Height <= 0 {
		return image.Rectangle{}
	}
	return image.Rect(r.X, r.Y, r.X+r.Width, r.Y+r.Height)
}

func placeCaptureOverlay(overlay *mygo.Window, target capture.DisplayTarget) {
	if overlay == nil {
		return
	}
	if target.Bounds.Dx() > 0 && target.Bounds.Dy() > 0 {
		overlay.SetBounds(mygo.Rectangle{
			X:      target.Bounds.Min.X,
			Y:      target.Bounds.Min.Y,
			Width:  target.Bounds.Dx(),
			Height: target.Bounds.Dy(),
		})
	}
	if !fullscreenOnDisplay(overlay, target) {
		overlay.SetFullScreen(true)
	}
}
