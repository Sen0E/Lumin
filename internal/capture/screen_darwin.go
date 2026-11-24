package capture

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"image"
	"runtime"
	"sync"
	"unsafe"

	"github.com/ebitengine/purego"
	"github.com/ebitengine/purego/objc"
)

var macCapture struct {
	once                   sync.Once
	err                    error
	width, height          func(uintptr) uintptr
	pixelsWide, pixelsHigh func(uint32) uintptr
	mainDisplay            func() uint32
}

func loadMacCapture() {
	macCapture.once.Do(func() {
		for _, name := range []string{"Foundation", "AppKit", "CoreGraphics", "ScreenCaptureKit"} {
			if _, err := purego.Dlopen("/System/Library/Frameworks/"+name+".framework/"+name, purego.RTLD_NOW|purego.RTLD_GLOBAL); err != nil {
				macCapture.err = err
				return
			}
		}
		cg, err := purego.Dlopen("/System/Library/Frameworks/CoreGraphics.framework/CoreGraphics", purego.RTLD_NOW|purego.RTLD_LOCAL)
		if err != nil {
			macCapture.err = err
			return
		}
		for _, binding := range []struct {
			name string
			fn   any
		}{{"CGImageGetWidth", &macCapture.width}, {"CGImageGetHeight", &macCapture.height}, {"CGDisplayPixelsWide", &macCapture.pixelsWide}, {"CGDisplayPixelsHigh", &macCapture.pixelsHigh}, {"CGMainDisplayID", &macCapture.mainDisplay}} {
			ptr, err := purego.Dlsym(cg, binding.name)
			if err != nil {
				macCapture.err = err
				return
			}
			purego.RegisterFunc(binding.fn, ptr)
		}
		if objc.GetClass("SCScreenshotManager") == 0 {
			macCapture.err = errors.New("原生截图需要 macOS 14 或以上")
		}
	})
}
func macSend(id objc.ID, name string, args ...any) objc.ID {
	return id.Send(objc.RegisterName(name), args...)
}
func macScreenshotError(err objc.ID) error {
	if err == 0 {
		return errors.New("macOS 未返回截图")
	}
	description := macSend(err, "localizedDescription")
	return fmt.Errorf("macOS 原生截图失败，请检查屏幕录制授权：%s", objc.Send[string](description, objc.RegisterName("UTF8String")))
}

// Screen uses ScreenCaptureKit's asynchronous screenshot API. The returned
// CGImage is encoded while its completion callback owns it; no CLI is launched.
func Screen(ctx context.Context, target DisplayTarget) (image.Image, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	loadMacCapture()
	if macCapture.err != nil {
		return nil, fmt.Errorf("%w：%v", ErrUnavailable, macCapture.err)
	}
	displayID := uint32(target.ID)
	if !target.valid() {
		displayID = macCapture.mainDisplay()
	}
	type outcome struct {
		img image.Image
		err error
	}
	result := make(chan outcome, 1)
	deliver := func(img image.Image, err error) {
		select {
		case result <- outcome{img, err}:
		default:
		}
	}
	callback := objc.NewBlock(func(_ objc.Block, content, nativeErr objc.ID) {
		if ctx.Err() != nil {
			return
		}
		runtime.LockOSThread()
		defer runtime.UnlockOSThread()
		pool := macSend(objc.ID(objc.GetClass("NSAutoreleasePool")), "new")
		defer macSend(pool, "drain")
		if nativeErr != 0 || content == 0 {
			deliver(nil, macScreenshotError(nativeErr))
			return
		}
		displays := macSend(content, "displays")
		count := objc.Send[uintptr](displays, objc.RegisterName("count"))
		var display objc.ID
		for i := uintptr(0); i < count; i++ {
			candidate := macSend(displays, "objectAtIndex:", i)
			if objc.Send[uint32](candidate, objc.RegisterName("displayID")) == displayID {
				display = candidate
				break
			}
		}
		if display == 0 {
			deliver(nil, errors.New("目标显示器已不可用"))
			return
		}
		w, h := macCapture.pixelsWide(displayID), macCapture.pixelsHigh(displayID)
		if w == 0 || h == 0 || uint64(w)*uint64(h) > MaxPixels {
			deliver(nil, errors.New("屏幕分辨率超出限制"))
			return
		}
		empty := macSend(objc.ID(objc.GetClass("NSArray")), "array")
		filter := macSend(macSend(objc.ID(objc.GetClass("SCContentFilter")), "alloc"), "initWithDisplay:excludingWindows:", display, empty)
		config := macSend(objc.ID(objc.GetClass("SCStreamConfiguration")), "new")
		macSend(config, "setWidth:", w)
		macSend(config, "setHeight:", h)
		macSend(config, "setShowsCursor:", false)
		imageCallback := objc.NewBlock(func(_ objc.Block, cgImage uintptr, nativeErr objc.ID) {
			runtime.LockOSThread()
			defer runtime.UnlockOSThread()
			pool := macSend(objc.ID(objc.GetClass("NSAutoreleasePool")), "new")
			defer macSend(pool, "drain")
			defer macSend(filter, "release")
			defer macSend(config, "release")
			if ctx.Err() != nil {
				return
			}
			if nativeErr != 0 || cgImage == 0 {
				deliver(nil, macScreenshotError(nativeErr))
				return
			}
			width, height := macCapture.width(cgImage), macCapture.height(cgImage)
			if width == 0 || height == 0 || uint64(width)*uint64(height) > MaxPixels {
				deliver(nil, errors.New("截图分辨率超出限制"))
				return
			}
			rep := macSend(macSend(objc.ID(objc.GetClass("NSBitmapImageRep")), "alloc"), "initWithCGImage:", cgImage)
			if rep == 0 {
				deliver(nil, errors.New("无法转换原生截图"))
				return
			}
			defer macSend(rep, "release")
			properties := macSend(objc.ID(objc.GetClass("NSDictionary")), "dictionary")
			data := macSend(rep, "representationUsingType:properties:", uintptr(4), properties) // NSBitmapImageFileTypePNG
			n := objc.Send[uintptr](data, objc.RegisterName("length"))
			if n == 0 || n > 64<<20 {
				deliver(nil, errors.New("截图图片大小超出限制"))
				return
			}
			ptr := objc.Send[unsafe.Pointer](data, objc.RegisterName("bytes"))
			if ptr == nil {
				deliver(nil, errors.New("原生截图像素为空"))
				return
			}
			img, _, err := image.Decode(bytes.NewReader(unsafe.Slice((*byte)(ptr), int(n))))
			deliver(img, err)
		})
		macSend(objc.ID(objc.GetClass("SCScreenshotManager")), "captureImageWithFilter:configuration:completionHandler:", filter, config, imageCallback)
		imageCallback.Release() // The asynchronous API retains its own block copy.
	})
	defer callback.Release()
	macSend(objc.ID(objc.GetClass("SCShareableContent")), "getShareableContentExcludingDesktopWindows:onScreenWindowsOnly:completionHandler:", false, true, callback)
	select {
	case <-ctx.Done():
		return nil, ctx.Err()
	case out := <-result:
		return out.img, out.err
	}
}
