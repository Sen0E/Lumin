// SPDX-License-Identifier: GPL-3.0-only
// Go port of TechDecryptor/pot-app-translate-plugin-caiyun; see NOTICE.
package main

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
	"unicode/utf8"

	"lumin/pkg/pluginapi"
)

const (
	endpoint = "https://interpreter.cyapi.cn/v1/translator"
	// These are the public upstream client's defaults, not a user's API credential.
	defaultToken    = "ssdj273ksdiwi923bsd9"
	defaultDeviceID = "F1F902F7-1780-4C88-848D-71F35D88A602"
	userAgent       = "caiyunInterpreter/5 CFNetwork/1404.0.5 Darwin/22.3.0"
	maxResponse     = 2 << 20
)

type translator struct {
	client *http.Client
	now    func() time.Time
}

func newTranslator() translator {
	return translator{client: &http.Client{Timeout: 45 * time.Second, CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }}, now: time.Now}
}

type translationRequest struct {
	Source    string `json:"source"`
	Detect    bool   `json:"detect"`
	OSType    string `json:"os_type"`
	DeviceID  string `json:"device_id"`
	TransType string `json:"trans_type"`
	Media     string `json:"media"`
	RequestID int64  `json:"request_id"`
	UserID    string `json:"user_id"`
	Dict      bool   `json:"dict"`
}

// Match the upstream language table, including its zh_cn/zh_tw aliases.
// Traditional Chinese maps to zh, as in Pot; this does not promise zh-TW output.
func language(code string, target bool) (string, error) {
	switch code {
	case "zh", "zh-TW", "zh_cn", "zh_tw":
		return "zh", nil
	case "en", "es", "fr", "ja", "ko", "ru":
		return code, nil
	case "auto":
		if !target {
			return "auto", nil
		}
	}
	return "", errors.New("彩云插件支持自动检测源语言，以及中、英、日、韩、法、西、俄语；目标语言不能是自动检测")
}

func (t translator) translate(ctx context.Context, r pluginapi.Request) (string, error) {
	if r.Operation != "translate" {
		return "", errors.New("彩云插件只支持文本翻译")
	}
	if strings.TrimSpace(r.Text) == "" || !utf8.ValidString(r.Text) || utf8.RuneCountInString(r.Text) > 20000 {
		return "", errors.New("请输入 1–20000 字符的有效文本")
	}
	from, err := language(r.Source, false)
	if err != nil {
		return "", err
	}
	to, err := language(r.Target, true)
	if err != nil {
		return "", err
	}
	token := strings.TrimSpace(r.Settings["token"])
	if token == "" {
		token = defaultToken
	}
	device := strings.TrimSpace(r.Settings["device_id"])
	if device == "" {
		device = defaultDeviceID
	}
	if len(token) > 4096 || strings.ContainsAny(token, "\r\n") || len(device) > 256 {
		return "", errors.New("彩云插件 Token 或设备 ID 格式无效")
	}
	body, err := json.Marshal(translationRequest{Source: r.Text, Detect: true, OSType: "ios", DeviceID: device, TransType: from + "2" + to, Media: "text", RequestID: t.now().UnixMilli() % 1000000000, UserID: "", Dict: true})
	if err != nil {
		return "", err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, bytes.NewReader(body))
	if err != nil {
		return "", err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("x-authorization", "token "+token)
	req.Header.Set("User-Agent", userAgent)
	resp, err := t.client.Do(req)
	if err != nil {
		if ctx.Err() != nil {
			return "", ctx.Err()
		}
		var timeout interface{ Timeout() bool }
		if errors.As(err, &timeout) && timeout.Timeout() {
			return "", errors.New("彩云请求超时，请稍后重试")
		}
		return "", errors.New("无法连接彩云翻译服务，请检查网络")
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		switch resp.StatusCode {
		case 401, 403:
			return "", fmt.Errorf("彩云拒绝授权（HTTP %d）；上游默认 Token 可能已失效，可在插件设置中更换", resp.StatusCode)
		case 429:
			return "", errors.New("彩云请求过于频繁或额度不足（HTTP 429），请稍后重试")
		default:
			return "", fmt.Errorf("彩云服务返回 HTTP %d", resp.StatusCode)
		}
	}
	data, err := io.ReadAll(io.LimitReader(resp.Body, maxResponse+1))
	if err != nil {
		if ctx.Err() != nil {
			return "", ctx.Err()
		}
		return "", errors.New("读取彩云响应失败")
	}
	if len(data) > maxResponse {
		return "", errors.New("彩云响应超出 2 MiB 限制")
	}
	var result struct {
		Target json.RawMessage `json:"target"`
	}
	if err := json.Unmarshal(data, &result); err != nil {
		return "", errors.New("彩云返回了无效 JSON 响应")
	}
	// Upstream returns a string. Also accept the service's segmented string array.
	var text string
	if err := json.Unmarshal(result.Target, &text); err != nil {
		var parts []string
		if err := json.Unmarshal(result.Target, &parts); err != nil {
			return "", errors.New("彩云没有返回有效译文，请检查接口或授权状态")
		}
		text = strings.Join(parts, "\n")
	}
	if strings.TrimSpace(text) == "" {
		return "", errors.New("彩云没有返回译文，请检查接口或授权状态")
	}
	return text, nil
}
