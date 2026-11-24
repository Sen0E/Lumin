// Package selection reads the focused application's actual text selection.
// It never reads or writes the ordinary clipboard and never starts tools.
package selection

import (
	"context"
	"errors"
	"strings"
	"time"
	"unicode/utf8"
)

const MaxCharacters = 20_000

var ErrEmpty = errors.New("未获取到选中文字，请先在原应用中选中文字，再按取词快捷键")
var ErrUnavailable = errors.New("当前应用或桌面未开放选中文字接口，可使用截图翻译")
var ErrTooLarge = errors.New("选中文字超过 20000 字，请缩小选区")

func Read(ctx context.Context) (text string, err error) {
	start := time.Now()
	defer func() { Trace("read", time.Since(start), err) }()
	if err := ctx.Err(); err != nil {
		return "", err
	}
	text, err = readNative(ctx)
	if err != nil {
		return "", err
	}
	if err := ctx.Err(); err != nil {
		return "", err
	}
	return validate(text)
}

func validate(text string) (string, error) {
	if strings.TrimSpace(text) == "" {
		return "", ErrEmpty
	}
	if !utf8.ValidString(text) {
		return "", errors.New("选中文字的编码无效")
	}
	if utf8.RuneCountInString(text) > MaxCharacters {
		return "", ErrTooLarge
	}
	return text, nil
}
