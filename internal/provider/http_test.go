package provider

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"strings"
	"testing"
	"time"

	"lumin/internal/config"
	"lumin/internal/domain"
)

type transportFunc func(*http.Request) (*http.Response, error)

func (f transportFunc) RoundTrip(r *http.Request) (*http.Response, error) { return f(r) }
func response(status int, body string) *http.Response {
	return &http.Response{StatusCode: status, Body: io.NopCloser(strings.NewReader(body)), Header: make(http.Header)}
}

var request = domain.Request{Text: "hello", Source: "auto", Target: "zh"}

func TestProtocols(t *testing.T) {
	for _, tc := range []struct{ kind, body, auth string }{
		{"compatible", `{"choices":[{"message":{"content":"你好"}}]}`, "Bearer secret"},
		{"ollama", `{"message":{"content":"你好"}}`, "Bearer secret"},
		{"deepl", `{"translations":[{"text":"你好"}]}`, "DeepL-Auth-Key secret"},
		{"libretranslate", `{"translatedText":"你好"}`, ""},
	} {
		t.Run(tc.kind, func(t *testing.T) {
			p := New(config.Provider{Kind: tc.kind, Endpoint: "https://example.test/translate", Model: "local-model", APIKey: "secret"}, time.Second)
			p.client.Transport = transportFunc(func(r *http.Request) (*http.Response, error) {
				if r.Method != "POST" || r.Header.Get("Authorization") != tc.auth {
					t.Errorf("invalid HTTP request")
				}
				var payload map[string]any
				if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
					t.Fatal(err)
				}
				switch tc.kind {
				case "compatible", "ollama":
					if payload["stream"] != false || payload["model"] != "local-model" {
						t.Errorf("payload: %v", payload)
					}
				case "deepl":
					if payload["source_lang"] != nil || payload["target_lang"] != "ZH-HANS" {
						t.Errorf("payload: %v", payload)
					}
				case "libretranslate":
					if payload["q"] != "hello" || payload["api_key"] != "secret" {
						t.Errorf("payload: %v", payload)
					}
				}
				return response(200, tc.body), nil
			})
			text, err := p.Translate(context.Background(), request)
			if err != nil || text != "你好" {
				t.Fatalf("%q %v", text, err)
			}
		})
	}
}

func TestFailuresDoNotLeakResponseOrCredentials(t *testing.T) {
	for _, tc := range []struct {
		name, body string
		status     int
	}{
		{"auth", "sensitive-secret", 401}, {"invalid json", "sensitive-secret", 200}, {"empty", `{"choices":[]}`, 200}, {"oversized", strings.Repeat("x", maxResponse+1), 200},
	} {
		t.Run(tc.name, func(t *testing.T) {
			p := New(config.Provider{Kind: "compatible", Endpoint: "https://example.test"}, time.Second)
			p.client.Transport = transportFunc(func(*http.Request) (*http.Response, error) { return response(tc.status, tc.body), nil })
			_, err := p.Translate(context.Background(), request)
			if err == nil || strings.Contains(err.Error(), "sensitive-secret") {
				t.Fatalf("unsafe error %v", err)
			}
		})
	}
}

func TestCancellationAndEnvironmentKey(t *testing.T) {
	t.Setenv("LUMIN_TEST_KEY", "environment-key")
	p := New(config.Provider{Kind: "ollama", Endpoint: "http://local.test", APIKey: "stored-key", APIKeyEnv: "LUMIN_TEST_KEY"}, time.Second)
	p.client.Transport = transportFunc(func(r *http.Request) (*http.Response, error) {
		if r.Header.Get("Authorization") != "Bearer environment-key" {
			t.Error("environment key did not win")
		}
		<-r.Context().Done()
		return nil, r.Context().Err()
	})
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	_, err := p.Translate(ctx, request)
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("%v", err)
	}
}

func TestRedirectsRejected(t *testing.T) {
	p := New(config.Provider{Kind: "ollama", Endpoint: "https://first.test"}, time.Second)
	calls := 0
	p.client.Transport = transportFunc(func(*http.Request) (*http.Response, error) {
		calls++
		r := response(302, "")
		r.Header.Set("Location", "https://other.test")
		return r, nil
	})
	if _, err := p.Translate(context.Background(), request); err == nil || calls != 1 {
		t.Fatalf("redirect followed: %d %v", calls, err)
	}
}
