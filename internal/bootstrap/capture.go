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
	mygo.RunOnMain(func() {
		if !closed.Load() {
			windows.beginCapture()
		}
	})
	defer mygo.RunOnMain(func() {
		if !closed.Load() {
			windows.endCapture()
		}
	})
	// Allow the compositor to remove our window before it captures the desktop.
	timer := time.NewTimer(200 * time.Millisecond)
	defer timer.Stop()
	select {
	case <-ctx.Done():
		return nil, ctx.Err()
	case <-timer.C:
	}
	img, err := capture.Screen(ctx)
	if err != nil {
		return nil, err
	}
	type outcome struct {
		selection capture.Selection
		err       error
	}
	result := make(chan outcome, 1)
	var overlay *mygo.Window
	mygo.RunOnMain(func() {
		if closed.Load() || ctx.Err() != nil {
			return
		}
		finished := false
		finish := func(sel capture.Selection, err error) {
			if finished {
				return
			}
			finished = true
			result <- outcome{sel, err}
			if overlay != nil {
				overlay.Close()
			}
		}
		view := desktop.NewSelection(img, finish)
		overlay = mygo.NewWindow(mygo.WindowOptions{Title: "Lumin · 截图框选", Width: 1100, Height: 760, FullScreen: true, AlwaysOnTop: true, Content: ui.View(view.View)})
		overlay.OnClosed(func() { finish(capture.Selection{}, capture.ErrCanceled) })
	})
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
