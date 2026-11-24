package selection

import (
	"context"
	"runtime"
	"syscall"
	"unicode/utf16"
	"unsafe"

	"golang.org/x/sys/windows"
)

var ole32 = windows.NewLazySystemDLL("ole32.dll")
var oleaut32 = windows.NewLazySystemDLL("oleaut32.dll")

// COM vtable slots follow the Windows SDK UIAutomationClient.h, IUIAutomation,
// IUIAutomationElement, IUIAutomationTextPattern and IUIAutomationTextRange.
type comObject struct{ table *[96]uintptr }

func (o *comObject) call(slot int, args ...uintptr) bool {
	args = append([]uintptr{uintptr(unsafe.Pointer(o))}, args...)
	hr, _, _ := syscall.SyscallN(o.table[slot], args...)
	return int32(hr) >= 0
}
func (o *comObject) release() {
	if o != nil {
		o.call(2)
	}
}

func readNative(ctx context.Context) (string, error) {
	runtime.LockOSThread()
	defer runtime.UnlockOSThread()
	hr, _, _ := ole32.NewProc("CoInitializeEx").Call(0, 0) // COINIT_MULTITHREADED
	if int32(hr) < 0 {
		return "", ErrUnavailable
	}
	defer ole32.NewProc("CoUninitialize").Call()
	clsid := windows.GUID{Data1: 0xff48dba4, Data2: 0x60ef, Data3: 0x4201, Data4: [8]byte{0xaa, 0x87, 0x54, 0x10, 0x3e, 0xef, 0x59, 0x4e}}
	iid := windows.GUID{Data1: 0x30cbe57d, Data2: 0xd9d0, Data3: 0x452a, Data4: [8]byte{0xab, 0x13, 0x7a, 0xc5, 0xac, 0x48, 0x25, 0xee}}
	var automation *comObject
	hr, _, _ = ole32.NewProc("CoCreateInstance").Call(uintptr(unsafe.Pointer(&clsid)), 0, 1, uintptr(unsafe.Pointer(&iid)), uintptr(unsafe.Pointer(&automation)))
	if int32(hr) < 0 || automation == nil {
		return "", ErrUnavailable
	}
	defer automation.release()
	var element *comObject
	if !automation.call(8, uintptr(unsafe.Pointer(&element))) || element == nil {
		return "", ErrUnavailable
	}
	defer element.release()
	if err := ctx.Err(); err != nil {
		return "", err
	}
	var password int32
	if !element.call(35, uintptr(unsafe.Pointer(&password))) || password != 0 {
		return "", ErrEmpty
	}
	iidText := windows.GUID{Data1: 0x32eba289, Data2: 0x3583, Data3: 0x42c9, Data4: [8]byte{0x9c, 0x59, 0x3b, 0x6d, 0x9a, 0x1e, 0x9b, 0x6a}}
	var pattern *comObject
	if !element.call(14, 10014, uintptr(unsafe.Pointer(&iidText)), uintptr(unsafe.Pointer(&pattern))) || pattern == nil {
		return "", ErrUnavailable
	}
	defer pattern.release()
	var ranges *comObject
	if !pattern.call(5, uintptr(unsafe.Pointer(&ranges))) || ranges == nil {
		return "", ErrUnavailable
	}
	defer ranges.release()
	var count int32
	if !ranges.call(3, uintptr(unsafe.Pointer(&count))) {
		return "", ErrUnavailable
	}
	if count == 0 {
		return "", ErrEmpty
	}
	if count != 1 {
		return "", ErrUnavailable
	}
	var span *comObject
	if !ranges.call(4, 0, uintptr(unsafe.Pointer(&span))) || span == nil {
		return "", ErrUnavailable
	}
	defer span.release()
	var text uintptr // BSTR, UTF-16 length owned by oleaut32
	if !span.call(12, MaxCharacters*2+1, uintptr(unsafe.Pointer(&text))) {
		return "", ErrUnavailable
	}
	if text == 0 {
		return "", ErrEmpty
	}
	defer oleaut32.NewProc("SysFreeString").Call(text)
	length, _, _ := oleaut32.NewProc("SysStringLen").Call(text)
	if length > MaxCharacters*2 {
		return "", ErrTooLarge
	}
	return string(utf16.Decode(unsafe.Slice((*uint16)(unsafe.Pointer(text)), int(length)))), nil
}
