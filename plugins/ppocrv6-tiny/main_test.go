package main

import (
	"context"
	"errors"
	"image"
	"image/color"
	"image/png"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
	"unsafe"

	"lumin/internal/plugin"
	"lumin/pkg/pluginapi"
)

func TestNativeLayouts(t *testing.T) {
	if unsafe.Sizeof(options{}) != 128 || unsafe.Sizeof(nativeImage{}) != 16 || unsafe.Sizeof(textBox{}) != 64 || unsafe.Sizeof(result{}) != 16 {
		t.Fatal("C ABI layout changed")
	}
	if unsafe.Offsetof(textBox{}.Confidence) != 8 || unsafe.Offsetof(textBox{}.Points) != 28 || unsafe.Offsetof(options{}.Fast) != 124 {
		t.Fatal("C ABI offsets changed")
	}
}
func TestParameterValidationAndAlpha(t *testing.T) {
	for _, s := range []string{"0", "17", "invalid"} {
		if _, err := intSetting(map[string]string{"threads": s}, "threads", 4, 1, 16); err == nil {
			t.Fatal("invalid threads accepted")
		}
	}
	if n, err := intSetting(nil, "threads", 4, 1, 16); err != nil || n != 4 {
		t.Fatal(n, err)
	}
	if _, err := recognizeAt(t.TempDir(), pluginapi.Request{Operation: "translate"}); err == nil {
		t.Fatal("accepted translate")
	}
	img := image.NewNRGBA(image.Rect(0, 0, 2, 1))
	img.SetNRGBA(0, 0, color.NRGBA{R: 255, A: 128})
	got := rgbPixels(img)
	if got[0] != 255 || got[1] != 127 || got[2] != 127 || got[3] != 255 || got[4] != 255 || got[5] != 255 {
		t.Fatal("transparent image not on white", got)
	}
	if _, err := nativeText(nil); err == nil {
		t.Fatal("accepted nil")
	}
	text := []byte("你好\x00")
	if got, err := nativeText(&text[0]); err != nil || got != "你好" {
		t.Fatal(got, err)
	}
}

func TestInstalledPluginRealModels(t *testing.T) {
	path := os.Getenv("LUMIN_PPOCR_PACKAGE")
	if path == "" {
		t.Skip("set LUMIN_PPOCR_PACKAGE to run real installed-plugin inference")
	}
	manager, err := plugin.New(filepath.Join(t.TempDir(), "plugins with spaces"))
	if err != nil {
		t.Fatal(err)
	}
	manifest, err := manager.Install(path)
	if err != nil {
		t.Fatal(err)
	}
	if manifest.ID != "lumin.ppocrv6-tiny" || !manifest.Has("recognize") {
		t.Fatal("wrong package")
	}
	// No Python, Tesseract, shell, or OCR executable can be found via PATH.
	t.Setenv("PATH", "")
	for _, sample := range []struct {
		path     string
		expected []string
	}{
		{"../../internal/ocr/testdata/english.png", []string{"Lumin", "translation test"}},
		{"testdata/chinese-english.png", []string{"离线文字识别测试", "你好世界", "Hello world", "12345"}},
	} {
		ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
		got, err := manager.Call(ctx, manifest.Ref(), plugin.Request{Operation: "recognize", ImagePath: sample.path})
		cancel()
		if err != nil {
			t.Fatal(err)
		}
		for _, part := range sample.expected {
			if !strings.Contains(got, part) {
				t.Fatalf("missing %q in %q", part, got)
			}
		}
		t.Log(got)
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := manager.Call(ctx, manifest.Ref(), plugin.Request{Operation: "recognize", ImagePath: "testdata/chinese-english.png"}); !errors.Is(err, context.Canceled) {
		t.Fatal("cancel ignored", err)
	}
	if _, err := manager.Call(context.Background(), manifest.Ref(), plugin.Request{Operation: "recognize", ImagePath: "testdata/chinese-english.png", Settings: map[string]string{"threads": "99"}}); err == nil {
		t.Fatal("bad setting accepted")
	}
	blank := filepath.Join(t.TempDir(), "blank.png")
	f, err := os.Create(blank)
	if err != nil {
		t.Fatal(err)
	}
	img := image.NewGray(image.Rect(0, 0, 128, 128))
	for i := range img.Pix {
		img.Pix[i] = 255
	}
	err = png.Encode(f, img)
	f.Close()
	if err != nil {
		t.Fatal(err)
	}
	if _, err := manager.Call(context.Background(), manifest.Ref(), plugin.Request{Operation: "recognize", ImagePath: blank}); err == nil || !strings.Contains(err.Error(), "未识别到文字") {
		t.Fatal("blank image not handled", err)
	}
}
