// SPDX-License-Identifier: GPL-3.0-only
package main

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"strings"
	"testing"
	"time"

	"lumin/pkg/pluginapi"
)

type roundTrip func(*http.Request) (*http.Response, error)

func (f roundTrip) RoundTrip(r *http.Request) (*http.Response, error) { return f(r) }
func response(status int, body string) *http.Response {
	return &http.Response{StatusCode: status, Header: make(http.Header), Body: io.NopCloser(strings.NewReader(body))}
}
func input() pluginapi.Request {
	return pluginapi.Request{Protocol: 1, ID: "test", Operation: "translate", Text: "Hello\nworld", Source: "auto", Target: "zh"}
}
func TestUpstreamRequest(t *testing.T) {
	tr := newTranslator()
	now := time.UnixMilli(1719155362123)
	tr.now = func() time.Time { return now }
	tr.client.Transport = roundTrip(func(r *http.Request) (*http.Response, error) {
		if r.Method != "POST" || r.URL.String() != endpoint || r.Header.Get("x-authorization") != "token "+defaultToken || r.Header.Get("User-Agent") != userAgent || r.Header.Get("Content-Type") != "application/json" {
			t.Fatalf("request headers/method/endpoint mismatch")
		}
		var body map[string]any
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			t.Fatal(err)
		}
		want := map[string]any{"source": "Hello\nworld", "detect": true, "os_type": "ios", "device_id": defaultDeviceID, "trans_type": "auto2zh", "media": "text", "request_id": float64(now.UnixMilli() % 1000000000), "user_id": "", "dict": true}
		got, _ := json.Marshal(body)
		expected, _ := json.Marshal(want)
		if string(got) != string(expected) {
			t.Fatalf("request body mismatch: %s", got)
		}
		return response(200, `{"target":"你好\n世界"}`), nil
	})
	got, err := tr.translate(context.Background(), input())
	if err != nil || got != "你好\n世界" {
		t.Fatalf("%q %v", got, err)
	}
}
func TestLanguagesAndOverrides(t *testing.T) {
	for _, code := range []string{"zh", "zh-TW", "zh_cn", "zh_tw", "en", "es", "fr", "ja", "ko", "ru"} {
		got, err := language(code, true)
		if err != nil {
			t.Fatal(err)
		}
		if strings.HasPrefix(code, "zh") && got != "zh" {
			t.Fatal(got)
		}
	}
	for _, code := range []string{"auto", "de", "pt", "it", "", "xx"} {
		if _, err := language(code, true); err == nil {
			t.Fatalf("unsupported target %q accepted", code)
		}
	}
	tr := newTranslator()
	tr.client.Transport = roundTrip(func(r *http.Request) (*http.Response, error) {
		if r.Header.Get("x-authorization") != "token custom" {
			t.Fatal("custom token ignored")
		}
		var body translationRequest
		_ = json.NewDecoder(r.Body).Decode(&body)
		if body.DeviceID != "custom-device" || body.TransType != "ja2zh" {
			t.Fatal("mapping/override ignored")
		}
		return response(200, `{"target":["第一行","第二行"]}`), nil
	})
	req := input()
	req.Source = "ja"
	req.Target = "zh-TW"
	req.Settings = map[string]string{"token": " custom ", "device_id": "custom-device"}
	got, err := tr.translate(context.Background(), req)
	if err != nil || got != "第一行\n第二行" {
		t.Fatalf("%q %v", got, err)
	}
}
func TestInvalidInputMakesNoRequest(t *testing.T) {
	tr := newTranslator()
	tr.client.Transport = roundTrip(func(*http.Request) (*http.Response, error) { t.Fatal("invalid input reached network"); return nil, nil })
	changes := []func(*pluginapi.Request){func(r *pluginapi.Request) { r.Operation = "recognize" }, func(r *pluginapi.Request) { r.Text = " " }, func(r *pluginapi.Request) { r.Text = strings.Repeat("字", 20001) }, func(r *pluginapi.Request) { r.Source = "de" }, func(r *pluginapi.Request) { r.Target = "auto" }, func(r *pluginapi.Request) { r.Settings = map[string]string{"token": "a\r\nb"} }}
	for _, change := range changes {
		req := input()
		change(&req)
		if _, err := tr.translate(context.Background(), req); err == nil {
			t.Fatal("expected input error")
		}
	}
}
func TestServiceFailuresDoNotExposeBodyOrCredentials(t *testing.T) {
	for _, tc := range []struct {
		status int
		body   string
	}{{401, `secret-body`}, {403, `secret-body`}, {429, `secret-body`}, {500, `secret-body`}, {200, `<html>secret-body</html>`}, {200, `{"message":"secret-body"}`}, {200, `{"target":null}`}, {200, `{"target":[]}`}, {200, `{"target":{"secret-body":true}}`}, {200, `{"target":"  "}`}, {200, strings.Repeat("x", maxResponse+1)}} {
		tr := newTranslator()
		tr.client.Transport = roundTrip(func(*http.Request) (*http.Response, error) { return response(tc.status, tc.body), nil })
		req := input()
		req.Settings = map[string]string{"token": "private-token"}
		_, err := tr.translate(context.Background(), req)
		if err == nil {
			t.Fatalf("accepted invalid response %d", tc.status)
		}
		for _, secret := range []string{"secret-body", "private-token"} {
			if strings.Contains(err.Error(), secret) {
				t.Fatal("error leaked service body or credential")
			}
		}
	}
}
func TestCancellationAndNoRedirect(t *testing.T) {
	tr := newTranslator()
	tr.client.Transport = roundTrip(func(r *http.Request) (*http.Response, error) { <-r.Context().Done(); return nil, r.Context().Err() })
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := tr.translate(ctx, input()); !errors.Is(err, context.Canceled) {
		t.Fatalf("lost cancellation: %v", err)
	}
	calls := 0
	tr.client.Transport = roundTrip(func(r *http.Request) (*http.Response, error) {
		calls++
		res := response(302, "")
		res.Header.Set("Location", "https://untrusted.example/collect")
		return res, nil
	})
	if _, err := tr.translate(context.Background(), input()); err == nil || calls != 1 {
		t.Fatalf("redirect followed: calls=%d err=%v", calls, err)
	}
}
func TestStdioResponse(t *testing.T) {
	tr := newTranslator()
	tr.client.Transport = roundTrip(func(*http.Request) (*http.Response, error) { return response(200, `{"target":"你好"}`), nil })
	req := input()
	data, _ := json.Marshal(req)
	var out bytes.Buffer
	if err := pluginapi.ServeIO(bytes.NewReader(data), &out, tr.translate); err != nil {
		t.Fatal(err)
	}
	var result pluginapi.Response
	if err := json.Unmarshal(out.Bytes(), &result); err != nil {
		t.Fatal(err)
	}
	if result.Protocol != 1 || result.ID != req.ID || result.Text != "你好" || result.Error != "" {
		t.Fatal(result)
	}
}
