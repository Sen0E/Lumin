package capture

import (
	"context"
	"os"
	"testing"
	"time"
)

func TestNativeCaptureCanceledBeforeStart(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := Screen(ctx, DisplayTarget{}); err == nil {
		t.Fatal("canceled capture succeeded")
	}
}

// Explicit opt-in on the target desktop; native permission UI may be shown.
func TestNativeCaptureOnDesktop(t *testing.T) {
	if os.Getenv("LUMIN_TEST_NATIVE_CAPTURE") != "1" {
		t.Skip("set LUMIN_TEST_NATIVE_CAPTURE=1 on a desktop to verify native capture")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	img, err := Screen(ctx, DisplayTarget{})
	if err != nil {
		t.Fatal(err)
	}
	if img == nil || img.Bounds().Empty() || int64(img.Bounds().Dx())*int64(img.Bounds().Dy()) > MaxPixels {
		t.Fatal("invalid captured image")
	}
}
