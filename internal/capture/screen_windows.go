package capture

import (
	"context"
	"errors"
	"fmt"
	"image"
	"runtime"
	"unsafe"

	"golang.org/x/sys/windows"
)

var user32 = windows.NewLazySystemDLL("user32.dll")
var gdi32 = windows.NewLazySystemDLL("gdi32.dll")

type winRect struct{ Left, Top, Right, Bottom int32 }
type monitorInfo struct {
	Size          uint32
	Monitor, Work winRect
	Flags         uint32
}
type bitmapInfo struct {
	Size                   uint32
	Width, Height          int32
	Planes, BitCount       uint16
	Compression, SizeImage uint32
	XPels, YPels           int32
	ClrUsed, ClrImportant  uint32
}

// Screen copies the fixed monitor through Win32 GDI. All handles and pixel
// buffers belong to this call; no PowerShell, shell, or image tool is launched.
func Screen(ctx context.Context, target DisplayTarget) (image.Image, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	runtime.LockOSThread()
	defer runtime.UnlockOSThread()
	setDPI := user32.NewProc("SetThreadDpiAwarenessContext")
	if setDPI.Find() == nil {
		old, _, _ := setDPI.Call(^uintptr(3)) // DPI_AWARENESS_CONTEXT_PER_MONITOR_AWARE_V2
		if old != 0 {
			defer setDPI.Call(old)
		}
	}
	monitor := uintptr(target.ID)
	if !target.valid() || monitor == 0 {
		var point struct{ X, Y int32 }
		if ok, _, _ := user32.NewProc("GetCursorPos").Call(uintptr(unsafe.Pointer(&point))); ok == 0 {
			return nil, errors.New("无法读取鼠标位置")
		}
		packed := uint64(uint32(point.X)) | uint64(uint32(point.Y))<<32
		monitor, _, _ = user32.NewProc("MonitorFromPoint").Call(uintptr(packed), 2)
	}
	info := monitorInfo{Size: uint32(unsafe.Sizeof(monitorInfo{}))}
	if ok, _, _ := user32.NewProc("GetMonitorInfoW").Call(monitor, uintptr(unsafe.Pointer(&info))); ok == 0 {
		return nil, errors.New("目标显示器已不可用")
	}
	w, h := int(info.Monitor.Right-info.Monitor.Left), int(info.Monitor.Bottom-info.Monitor.Top)
	if w <= 0 || h <= 0 || int64(w)*int64(h) > MaxPixels {
		return nil, errors.New("屏幕分辨率超出限制")
	}
	screen, _, _ := user32.NewProc("GetDC").Call(0)
	if screen == 0 {
		return nil, errors.New("无法获取屏幕绘图上下文")
	}
	defer user32.NewProc("ReleaseDC").Call(0, screen)
	memory, _, _ := gdi32.NewProc("CreateCompatibleDC").Call(screen)
	if memory == 0 {
		return nil, errors.New("无法创建截图绘图上下文")
	}
	defer gdi32.NewProc("DeleteDC").Call(memory)
	bi := bitmapInfo{Size: 40, Width: int32(w), Height: -int32(h), Planes: 1, BitCount: 32}
	var bits unsafe.Pointer
	bitmap, _, _ := gdi32.NewProc("CreateDIBSection").Call(screen, uintptr(unsafe.Pointer(&bi)), 0, uintptr(unsafe.Pointer(&bits)), 0, 0)
	if bitmap == 0 || bits == nil {
		return nil, errors.New("无法创建截图位图")
	}
	defer gdi32.NewProc("DeleteObject").Call(bitmap)
	previous, _, _ := gdi32.NewProc("SelectObject").Call(memory, bitmap)
	if previous == 0 || previous == ^uintptr(0) {
		return nil, errors.New("无法选择截图位图")
	}
	defer gdi32.NewProc("SelectObject").Call(memory, previous)
	ok, _, err := gdi32.NewProc("BitBlt").Call(memory, 0, 0, uintptr(w), uintptr(h), screen, uintptr(info.Monitor.Left), uintptr(info.Monitor.Top), 0x00CC0020|0x40000000)
	if ok == 0 {
		return nil, fmt.Errorf("Windows 原生截图失败：%w", err)
	}
	gdi32.NewProc("GdiFlush").Call()
	pixels := unsafe.Slice((*byte)(bits), w*h*4)
	out := image.NewRGBA(image.Rect(0, 0, w, h))
	for y := 0; y < h; y++ {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		for x := 0; x < w; x++ {
			i := (y*w + x) * 4
			out.Pix[i], out.Pix[i+1], out.Pix[i+2], out.Pix[i+3] = pixels[i+2], pixels[i+1], pixels[i], 255
		}
	}
	return out, nil
}
