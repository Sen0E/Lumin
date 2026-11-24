package bootstrap

import (
	"image"
	"lumin/internal/capture"
	"testing"
)

func TestCaptureOverlayStartsHiddenAndFramelessOnTarget(t *testing.T) {
	target := capture.DisplayTarget{Bounds: image.Rect(-1920, -1080, 0, 0)}
	opts := captureOverlayOptions(target)
	if !opts.Hidden || !opts.Frameless || !opts.AlwaysOnTop || opts.X != -1920 || opts.Y != -1080 || opts.Width != 1920 || opts.Height != 1080 {
		t.Fatal("overlay would flash chrome or use wrong display", opts)
	}
	fallback := captureOverlayOptions(capture.DisplayTarget{})
	if fallback.Width <= 0 || fallback.Height <= 0 || !fallback.Frameless || !fallback.Hidden {
		t.Fatal("invalid fallback overlay")
	}
}
