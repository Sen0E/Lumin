// PP-OCRv6 tiny is a self-contained recognize plugin. The host owns cancellation
// by terminating this one-request process (including its native worker threads).
package main

import (
	"context"
	"errors"
	"fmt"
	"image"
	"os"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"unicode/utf8"
	"unsafe"

	"github.com/ebitengine/purego"
	"lumin/internal/capture"
	"lumin/pkg/pluginapi"
)

// ABI prefix through rec_fast_mode, fixed at 128 bytes. init_ex and struct_size
// allow newer native fields to keep their defaults. See pinned SDK header.
type options struct {
	Size, Backend                                                                                                                       int32
	Threshold, BoxThreshold, Unclip                                                                                                     float32
	Side, RecHeight, RecWidth, RecBatch, RecBucket, RecParallel, HybridParallel, DetBatch, PreprocessParallel, ImageParallel, LimitType int32
	DetMean, DetStd, RecMean, RecStd                                                                                                    [3]float32
	Dilation, Candidates, Device, Fast                                                                                                  int32
}
type nativeImage struct {
	Width, Height int32
	RGB           *byte
}
type textBox struct {
	Text       *byte
	Confidence float32
	X, Y, W, H int32
	Points     [8]float32
}
type result struct {
	Items *textBox
	Count uintptr
}
type nativeAPI struct {
	init      func(*options, uintptr)
	create    func(string, string, string, *options) uintptr
	destroy   func(uintptr)
	recognize func(uintptr, *nativeImage, *result) int32
	free      func(*result)
	lastError func() string
}

func bind(resolve func(string) (uintptr, error)) (nativeAPI, error) {
	var api nativeAPI
	for _, s := range []struct {
		name string
		out  any
	}{
		{"rpocr_options_init_ex", &api.init}, {"rpocr_create", &api.create},
		{"rpocr_destroy", &api.destroy}, {"rpocr_recognize", &api.recognize},
		{"rpocr_result_free", &api.free}, {"rpocr_last_error", &api.lastError},
	} {
		ptr, err := resolve(s.name)
		if err != nil {
			return api, err
		}
		purego.RegisterFunc(s.out, ptr)
	}
	return api, nil
}
func intSetting(values map[string]string, key string, def, min, max int) (int, error) {
	raw := values[key]
	if raw == "" {
		return def, nil
	}
	v, err := strconv.Atoi(raw)
	if err != nil || v < min || v > max {
		return 0, fmt.Errorf("%s 须为 %d–%d 的整数", key, min, max)
	}
	return v, nil
}
func rgbPixels(img image.Image) []byte {
	b := img.Bounds()
	pixels := make([]byte, b.Dx()*b.Dy()*3)
	i := 0
	for y := b.Min.Y; y < b.Max.Y; y++ {
		for x := b.Min.X; x < b.Max.X; x++ {
			r, g, bl, a := img.At(x, y).RGBA()
			white := uint32(65535) - a
			pixels[i], pixels[i+1], pixels[i+2] = byte((r+white)>>8), byte((g+white)>>8), byte((bl+white)>>8)
			i += 3
		}
	}
	return pixels
}
func nativeText(p *byte) (string, error) {
	if p == nil {
		return "", errors.New("OCR 返回空文本指针")
	}
	// Native C strings belong to the result, never retained beyond result_free.
	var b []byte
	for n := 0; n < 1<<20; n++ {
		v := *(*byte)(unsafe.Add(unsafe.Pointer(p), n))
		if v == 0 {
			if !utf8.Valid(b) {
				return "", errors.New("OCR 返回无效 UTF-8")
			}
			return string(b), nil
		}
		b = append(b, v)
	}
	return "", errors.New("OCR 文本超出限制")
}
func recognizeAt(root string, r pluginapi.Request) (string, error) {
	if r.Operation != "recognize" {
		return "", errors.New("插件仅支持 recognize")
	}
	threads, err := intSetting(r.Settings, "threads", 4, 1, 16)
	if err != nil {
		return "", err
	}
	side, err := intSetting(r.Settings, "max_side", 960, 320, 2048)
	if err != nil {
		return "", err
	}
	info, err := os.Stat(r.ImagePath)
	if err != nil {
		return "", err
	}
	if !info.Mode().IsRegular() || info.Size() > 20<<20 {
		return "", errors.New("图片须为不超过 20 MiB 的普通文件")
	}
	img, err := capture.ReadImage(r.ImagePath)
	if err != nil {
		return "", err
	}
	for _, name := range []string{"detection.onnx", "recognition.onnx", "dictionary.txt"} {
		if _, err := os.Stat(filepath.Join(root, "models", name)); err != nil {
			return "", fmt.Errorf("插件模型缺失：%s", name)
		}
	}
	// Environment only affects this plugin process. No global host configuration.
	if err := os.Setenv("PPOCR_BENCH_THREADS", strconv.Itoa(threads)); err != nil {
		return "", err
	}
	runtime.LockOSThread()
	defer runtime.UnlockOSThread() // last_error is thread-local.
	resolve, closeLib, err := openLibrary(filepath.Join(root, "lib", libraryName))
	if err != nil {
		return "", fmt.Errorf("无法加载随插件提供的 OCR 运行库：%w", err)
	}
	defer closeLib()
	api, err := bind(resolve)
	if err != nil {
		return "", err
	}
	var opt options
	api.init(&opt, unsafe.Sizeof(opt))
	opt.Backend = 0
	opt.Side = int32(side)
	opt.LimitType = 0
	// Match RapidOCR's PP-OCRv6 preprocessing, while limiting the longest side.
	opt.DetMean = [3]float32{0.5, 0.5, 0.5}
	opt.DetStd = [3]float32{0.5, 0.5, 0.5}
	opt.Dilation = 1
	handle := api.create(filepath.Join(root, "models", "detection.onnx"), filepath.Join(root, "models", "recognition.onnx"), filepath.Join(root, "models", "dictionary.txt"), &opt)
	if handle == 0 {
		return "", fmt.Errorf("模型加载失败：%s", api.lastError())
	}
	defer api.destroy(handle)
	pixels := rgbPixels(img)
	input := nativeImage{int32(img.Bounds().Dx()), int32(img.Bounds().Dy()), &pixels[0]}
	// Native input is synchronous; pin Go-owned storage until inference returns.
	var pin runtime.Pinner
	pin.Pin(&pixels[0])
	pin.Pin(&input)
	defer pin.Unpin()
	var out result
	code := api.recognize(handle, &input, &out)
	runtime.KeepAlive(pixels)
	defer api.free(&out)
	if code != 0 {
		return "", fmt.Errorf("OCR 推理失败：%s", api.lastError())
	}
	if out.Count > 10000 || out.Count > 0 && out.Items == nil {
		return "", errors.New("OCR 返回异常结果数量")
	}
	lines := make([]string, 0, out.Count)
	size := 0
	for _, box := range unsafe.Slice(out.Items, int(out.Count)) {
		if box.Confidence < 0.5 {
			continue
		}
		text, err := nativeText(box.Text)
		if err != nil {
			return "", err
		}
		text = strings.TrimSpace(text)
		if text != "" {
			size += len(text) + 1
			if size > 1<<20 {
				return "", errors.New("OCR 结果超出限制")
			}
			lines = append(lines, text)
		}
	}
	if len(lines) == 0 {
		return "", errors.New("图片中未识别到文字")
	}
	return strings.Join(lines, "\n"), nil
}
func main() {
	exe, err := os.Executable()
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	root := filepath.Dir(filepath.Dir(exe))
	if err := pluginapi.Serve(func(_ context.Context, r pluginapi.Request) (string, error) { return recognizeAt(root, r) }); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
