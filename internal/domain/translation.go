// Package domain defines framework-independent translation and history types.
package domain

import (
	"context"
	"errors"
	"strings"
	"time"
	"unicode/utf8"
)

const MaxTextRunes = 20000

type Language struct{ Code, Name string }

var Languages = []Language{
	{"auto", "自动检测"}, {"zh", "简体中文"}, {"zh-TW", "繁體中文"},
	{"en", "English"}, {"ja", "日本語"}, {"ko", "한국어"},
	{"fr", "Français"}, {"de", "Deutsch"}, {"es", "Español"},
	{"ru", "Русский"}, {"pt", "Português"}, {"it", "Italiano"},
}

func LanguageName(code string) string {
	for _, l := range Languages {
		if l.Code == code {
			return l.Name
		}
	}
	return code
}

func ValidLanguage(code string, target bool) bool {
	for _, l := range Languages {
		if l.Code == code {
			return !target || code != "auto"
		}
	}
	return false
}

type Request struct {
	Text   string `json:"text"`
	Source string `json:"source"`
	Target string `json:"target"`
}

func (r Request) Validate() error {
	if strings.TrimSpace(r.Text) == "" {
		return errors.New("请输入待翻译文本")
	}
	if !utf8.ValidString(r.Text) || utf8.RuneCountInString(r.Text) > MaxTextRunes {
		return errors.New("文本须为有效 UTF-8，且不超过 20000 字符")
	}
	if !ValidLanguage(r.Source, false) || !ValidLanguage(r.Target, true) {
		return errors.New("不支持的源语言或目标语言")
	}
	return nil
}

type Result struct {
	ProviderID   string `json:"provider_id"`
	ProviderName string `json:"provider_name"`
	Text         string `json:"text,omitempty"`
	Error        string `json:"error,omitempty"`
	DurationMS   int64  `json:"duration_ms"`
}

// Translator implementations must honor cancellation and support concurrent calls.
type Translator interface {
	ID() string
	Name() string
	Translate(context.Context, Request) (string, error)
}

type Entry struct {
	ID        string    `json:"id"`
	CreatedAt time.Time `json:"created_at"`
	Request   Request   `json:"request"`
	Results   []Result  `json:"results"`
	Favorite  bool      `json:"favorite"`
}
