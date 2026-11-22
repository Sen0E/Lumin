// Package ocr provides cancellable image recognition without a web runtime.
package ocr

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

type Recognizer interface {
	Recognize(context.Context, string, string) (string, error)
}
type Tesseract struct{}

func (Tesseract) Recognize(ctx context.Context, path, languages string) (string, error) {
	bin, err := exec.LookPath("tesseract")
	if err != nil {
		return "", errors.New("未找到 tesseract，请安装 Tesseract OCR 和对应语言包")
	}
	path, err = filepath.Abs(path)
	if err != nil {
		return "", err
	}
	info, err := os.Stat(path)
	if err != nil {
		return "", err
	}
	if !info.Mode().IsRegular() || info.Size() > 20<<20 {
		return "", errors.New("请选择不超过 20 MiB 的普通图片文件")
	}
	switch strings.ToLower(filepath.Ext(path)) {
	case ".png", ".jpg", ".jpeg", ".tif", ".tiff", ".bmp", ".webp":
	default:
		return "", errors.New("不支持的图片格式")
	}
	cmd := exec.CommandContext(ctx, bin, path, "stdout", "-l", languages)
	out, stderr := &limitedBuffer{}, &limitedBuffer{}
	cmd.Stdout, cmd.Stderr = out, stderr
	if err := cmd.Run(); err != nil {
		if ctx.Err() != nil {
			return "", ctx.Err()
		}
		return "", fmt.Errorf("OCR 失败，请检查图片和语言包（%s）", languages)
	}
	text := strings.TrimSpace(out.String())
	if text == "" {
		return "", errors.New("图片中未识别到文字")
	}
	return text, nil
}

type limitedBuffer struct{ bytes.Buffer }

func (b *limitedBuffer) Write(p []byte) (int, error) {
	if b.Len()+len(p) > 1<<20 {
		return 0, errors.New("OCR 输出超出限制")
	}
	return b.Buffer.Write(p)
}
