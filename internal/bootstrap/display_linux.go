//go:build linux

package bootstrap

import (
	"sync"

	"github.com/ebitengine/purego"
	"github.com/egoist/mygo"
	"lumin/internal/capture"
)

var captureGTK struct {
	once                      sync.Once
	available                 bool
	widgetGetScreen           func(uintptr) uintptr
	windowFullscreenOnMonitor func(uintptr, uintptr, int32)
}

func loadCaptureGTK() {
	captureGTK.once.Do(func() {
		lib, err := purego.Dlopen("libgtk-3.so.0", purego.RTLD_NOW|purego.RTLD_GLOBAL)
		if err != nil {
			return
		}
		getscreen, err := purego.Dlsym(lib, "gtk_widget_get_screen")
		if err != nil {
			return
		}
		fullscreen, err := purego.Dlsym(lib, "gtk_window_fullscreen_on_monitor")
		if err != nil {
			return
		}
		purego.RegisterFunc(&captureGTK.widgetGetScreen, getscreen)
		purego.RegisterFunc(&captureGTK.windowFullscreenOnMonitor, fullscreen)
		captureGTK.available = true
	})
}

func fullscreenOnDisplay(overlay *mygo.Window, target capture.DisplayTarget) bool {
	if target.ID < 0 || target.ID > 1<<31-1 {
		return false
	}
	loadCaptureGTK()
	if !captureGTK.available {
		return false
	}
	handle := overlay.NativeHandle()
	if handle == 0 {
		return false
	}
	screen := captureGTK.widgetGetScreen(handle)
	if screen == 0 {
		return false
	}
	captureGTK.windowFullscreenOnMonitor(handle, screen, int32(target.ID))
	return true
}
