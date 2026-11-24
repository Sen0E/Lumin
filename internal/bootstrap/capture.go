package bootstrap

import (
	"context"
	"image"
	"sync/atomic"
	"time"

	"github.com/egoist/mygo"
	"github.com/egoist/mygo/ui"
	"lumin/internal/capture"
	"lumin/internal/desktop"
)

// captureArea runs on a worker; all window operations are dispatched to main.
func captureArea(ctx context.Context, windows *windowSet, closed *atomic.Bool) (image.Image, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	var target capture.DisplayTarget
	var hiddenWindows bool
	mygo.RunOnMain(func() {
		if !closed.Load() {
			target = captureDisplayTarget()
			hiddenWindows = windows.beginCapture()
		}
	})
	defer mygo.RunOnMain(func() {
		if !closed.Load() {
			windows.endCapture()
		}
	})
	// Only visible surfaces need compositor settling; background hotkeys can
	// capture immediately. Keep the existing margin when we actually hid a window.
	if hiddenWindows {
		timer := time.NewTimer(200 * time.Millisecond)
		defer timer.Stop()
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		case <-timer.C:
		}
	}
	captureDone := capture.TracePhase("screen")
	img, err := capture.Screen(ctx, target)
	captureDone()
	if err != nil {
		return nil, err
	}
	type outcome struct {
		selection capture.Selection
		err       error
	}
	result := make(chan outcome, 1)
	var overlay *mygo.Window
	var finish func(capture.Selection, error)
	// Bitmap conversion is CPU work; prepare it before entering the UI thread.
	prepareDone := capture.TracePhase("prepare-selection")
	view := desktop.NewSelection(img, func(sel capture.Selection, err error) { finish(sel, err) })
	prepareDone()
	showDone := capture.TracePhase("show-overlay")
	mygo.RunOnMain(func() {
		if closed.Load() || ctx.Err() != nil {
			return
		}
		finished := false
		finish = func(sel capture.Selection, err error) {
			if finished {
				return
			}
			finished = true
			result <- outcome{sel, err}
			if overlay != nil {
				overlay.Close()
			}
		}
		options := captureOverlayOptions(target)
		options.Content = ui.View(view.View)
		overlay = mygo.NewWindow(options)
		overlay.OnClosed(func() { finish(capture.Selection{}, capture.ErrCanceled) })
		placeCaptureOverlay(overlay, target)
		overlay.Show()
		overlay.Focus()
	})
	showDone()
	defer mygo.RunOnMain(func() {
		if overlay != nil {
			overlay.Close()
		}
	})
	select {
	case <-ctx.Done():
		return nil, ctx.Err()
	case r := <-result:
		if r.err != nil {
			return nil, r.err
		}
		return capture.Crop(img, r.selection)
	}
}

// Fullscreen is applied on the fixed display before showing this surface.
func captureOverlayOptions(target capture.DisplayTarget) mygo.WindowOptions {
	x, y, width, height := 0, 0, 1100, 760
	if !target.Bounds.Empty() {
		x, y = target.Bounds.Min.X, target.Bounds.Min.Y
		width, height = target.Bounds.Dx(), target.Bounds.Dy()
	}
	return mygo.WindowOptions{Title: "Lumin · 截图框选", X: x, Y: y, Width: width, Height: height, Frameless: true, Hidden: true, AlwaysOnTop: true}
}
