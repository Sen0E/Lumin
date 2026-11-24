package selection

import (
	"context"
	"fmt"
	"runtime"
	"sync"
	"unsafe"

	"github.com/ebitengine/purego"
)

var ax struct {
	once        sync.Once
	err         error
	trusted     func() bool
	root        func() uintptr
	copy        func(uintptr, uintptr, *uintptr) int32
	timeout     func(uintptr, float32) int32
	makeString  func(uintptr, string, uint32) uintptr
	length      func(uintptr) int64
	stringBytes func(uintptr, *byte, int64, uint32) bool
	getType     func(uintptr) uintptr
	stringType  func() uintptr
	release     func(uintptr)
}

func loadAX() {
	ax.once.Do(func() {
		for _, lib := range []struct {
			path    string
			symbols []struct {
				name string
				fn   any
			}
		}{
			{"/System/Library/Frameworks/ApplicationServices.framework/ApplicationServices", []struct {
				name string
				fn   any
			}{
				{"AXIsProcessTrusted", &ax.trusted}, {"AXUIElementCreateSystemWide", &ax.root}, {"AXUIElementCopyAttributeValue", &ax.copy}, {"AXUIElementSetMessagingTimeout", &ax.timeout}}},
			{"/System/Library/Frameworks/CoreFoundation.framework/CoreFoundation", []struct {
				name string
				fn   any
			}{
				{"CFStringCreateWithCString", &ax.makeString}, {"CFStringGetLength", &ax.length}, {"CFStringGetCString", &ax.stringBytes}, {"CFGetTypeID", &ax.getType}, {"CFStringGetTypeID", &ax.stringType}, {"CFRelease", &ax.release}}},
		} {
			h, err := purego.Dlopen(lib.path, purego.RTLD_NOW|purego.RTLD_GLOBAL)
			if err != nil {
				ax.err = err
				return
			}
			for _, s := range lib.symbols {
				p, err := purego.Dlsym(h, s.name)
				if err != nil {
					ax.err = err
					return
				}
				purego.RegisterFunc(s.fn, p)
			}
		}
	})
}
func axAttribute(element uintptr, name string) uintptr {
	key := ax.makeString(0, name, 0x08000100)
	if key == 0 {
		return 0
	}
	defer ax.release(key)
	var value uintptr
	if ax.copy(element, key, &value) != 0 {
		return 0
	}
	return value
}
func axText(value uintptr) string {
	if value == 0 || ax.getType(value) != ax.stringType() {
		return ""
	}
	n := ax.length(value)
	if n < 0 || n > MaxCharacters*2 {
		return ""
	}
	buf := make([]byte, n*4+1)
	if !ax.stringBytes(value, (*byte)(unsafe.Pointer(&buf[0])), int64(len(buf)), 0x08000100) {
		return ""
	}
	for i, b := range buf {
		if b == 0 {
			return string(buf[:i])
		}
	}
	return ""
}
func readNative(ctx context.Context) (string, error) {
	runtime.LockOSThread()
	defer runtime.UnlockOSThread()
	loadAX()
	if ax.err != nil {
		return "", ErrUnavailable
	}
	if !ax.trusted() {
		return "", fmt.Errorf("%w：请在系统设置的辅助功能中允许 Lumin 访问", ErrUnavailable)
	}
	root := ax.root()
	if root == 0 {
		return "", ErrUnavailable
	}
	defer ax.release(root)
	ax.timeout(root, 0.3)
	if err := ctx.Err(); err != nil {
		return "", err
	}
	focused := axAttribute(root, "AXFocusedUIElement")
	if focused == 0 {
		return "", ErrUnavailable
	}
	defer ax.release(focused)
	ax.timeout(focused, 0.3)
	subrole := axAttribute(focused, "AXSubrole")
	if subrole != 0 {
		defer ax.release(subrole)
		if axText(subrole) == "AXSecureTextField" {
			return "", ErrEmpty
		}
	}
	selected := axAttribute(focused, "AXSelectedText")
	if selected == 0 {
		return "", ErrUnavailable
	}
	defer ax.release(selected)
	if ax.getType(selected) != ax.stringType() {
		return "", ErrUnavailable
	}
	if ax.length(selected) > MaxCharacters*2 {
		return "", ErrTooLarge
	}
	return axText(selected), nil
}
