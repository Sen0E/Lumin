// Package config owns the versioned user configuration and its validation.
package config

import (
	"errors"
	"fmt"
	"net"
	"net/url"
	"strconv"
	"strings"

	"lumin/internal/domain"
)

const Version = 1

type Provider struct {
	ID        string `json:"id"`
	Name      string `json:"name"`
	Kind      string `json:"kind"`
	Enabled   bool   `json:"enabled"`
	Endpoint  string `json:"endpoint"`
	Model     string `json:"model,omitempty"`
	APIKey    string `json:"api_key,omitempty"`
	APIKeyEnv string `json:"api_key_env,omitempty"`
}

type Config struct {
	Version           int        `json:"version"`
	Source            string     `json:"source"`
	Target            string     `json:"target"`
	TimeoutSeconds    int        `json:"timeout_seconds"`
	HistoryLimit      int        `json:"history_limit"`
	SaveHistory       bool       `json:"save_history"`
	Providers         []Provider `json:"providers"`
	InputShortcut     string     `json:"input_shortcut"`
	ClipboardShortcut string     `json:"clipboard_shortcut"`
	ServerEnabled     bool       `json:"server_enabled"`
	ServerAddress     string     `json:"server_address"`
	ServerToken       string     `json:"server_token,omitempty"`
	OCRLanguages      string     `json:"ocr_languages"`
}

func Default() Config {
	return Config{Version: Version, Source: "auto", Target: "zh", TimeoutSeconds: 60,
		HistoryLimit: 500, SaveHistory: true, InputShortcut: "CmdOrCtrl+Alt+Space",
		ClipboardShortcut: "CmdOrCtrl+Alt+T", ServerAddress: "127.0.0.1:60828", OCRLanguages: "eng+chi_sim",
		Providers: []Provider{
			{ID: "compatible", Name: "兼容 Chat Completions", Kind: "compatible", Endpoint: "https://api.openai.com/v1/chat/completions", APIKeyEnv: "OPENAI_API_KEY"},
			{ID: "ollama", Name: "Ollama 本地模型", Kind: "ollama", Endpoint: "http://127.0.0.1:11434/api/chat", Model: "qwen2.5:7b"},
			{ID: "deepl", Name: "DeepL API", Kind: "deepl", Endpoint: "https://api-free.deepl.com/v2/translate", APIKeyEnv: "DEEPL_API_KEY"},
			{ID: "libre", Name: "LibreTranslate", Kind: "libretranslate", Endpoint: "http://127.0.0.1:5000/translate"},
		},
	}
}

func (c Config) Clone() Config { c.Providers = append([]Provider(nil), c.Providers...); return c }

func (c Config) Validate() error {
	if c.Version != Version {
		return fmt.Errorf("不支持的配置版本 %d（当前支持 %d）", c.Version, Version)
	}
	if !domain.ValidLanguage(c.Source, false) || !domain.ValidLanguage(c.Target, true) {
		return errors.New("默认语言无效")
	}
	if c.TimeoutSeconds < 1 || c.TimeoutSeconds > 300 {
		return errors.New("请求超时须为 1–300 秒")
	}
	if c.HistoryLimit < 1 || c.HistoryLimit > 5000 {
		return errors.New("历史记录上限须为 1–5000")
	}
	if len(c.Providers) > 16 {
		return errors.New("最多配置 16 个翻译服务")
	}
	ids := map[string]bool{}
	for _, p := range c.Providers {
		if strings.TrimSpace(p.ID) == "" || ids[p.ID] {
			return errors.New("翻译服务 ID 为空或重复")
		}
		ids[p.ID] = true
		if strings.TrimSpace(p.Name) == "" {
			return errors.New("翻译服务名称不能为空")
		}
		switch p.Kind {
		case "compatible", "ollama", "deepl", "libretranslate":
		default:
			return fmt.Errorf("未知服务类型 %q", p.Kind)
		}
		if !p.Enabled {
			continue
		}
		u, err := url.Parse(p.Endpoint)
		if err != nil || (u.Scheme != "http" && u.Scheme != "https") || u.Hostname() == "" || u.User != nil || u.RawQuery != "" || u.Fragment != "" {
			return fmt.Errorf("%s：请填写完整 HTTP(S) 接口地址，不含认证信息、查询参数或片段", p.Name)
		}
		if (p.Kind == "compatible" || p.Kind == "ollama") && strings.TrimSpace(p.Model) == "" {
			return fmt.Errorf("%s：模型名称不能为空", p.Name)
		}
	}
	if c.ServerEnabled {
		host, port, err := net.SplitHostPort(c.ServerAddress)
		if err != nil {
			return errors.New("本地服务地址须为 127.0.0.1:端口 或 [::1]:端口")
		}
		ip := net.ParseIP(host)
		n, err := strconv.Atoi(port)
		if ip == nil || !ip.IsLoopback() || err != nil || n < 1 || n > 65535 {
			return errors.New("本地服务仅允许回环地址和有效端口")
		}
		if len(c.ServerToken) < 24 {
			return errors.New("外部调用令牌至少需要 24 个字符")
		}
	}
	if c.InputShortcut != "" && c.InputShortcut == c.ClipboardShortcut {
		return errors.New("两个全局快捷键不能相同")
	}
	if strings.TrimSpace(c.OCRLanguages) == "" {
		return errors.New("OCR 语言不能为空")
	}
	return nil
}
