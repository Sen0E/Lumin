package localapi

import (
	"net/http/httptest"
	"strings"
	"testing"
)

func TestRequestBoundary(t *testing.T) {
	for _, tc := range []struct {
		name, method, path, body, auth, origin string
		status                                 int
	}{
		{"valid", "POST", "/translate", "你好", "Bearer secret", "", 202},
		{"missing auth", "POST", "/translate", "hello", "", "", 401},
		{"browser", "POST", "/translate", "hello", "Bearer secret", "https://evil.test", 403},
		{"empty", "POST", "/translate", " ", "Bearer secret", "", 400},
		{"too large", "POST", "/translate", strings.Repeat("x", 80001), "Bearer secret", "", 413},
		{"invalid utf8", "POST", "/translate", string([]byte{255}), "Bearer secret", "", 400},
		{"method", "GET", "/translate", "hello", "Bearer secret", "", 405},
		{"unknown", "POST", "/unknown", "", "Bearer secret", "", 404},
		{"open settings", "POST", "/config", "", "Bearer secret", "", 202},
	} {
		t.Run(tc.name, func(t *testing.T) {
			calls := 0
			h := Handler("secret", func(a Action) error {
				calls++
				if tc.path == "/translate" && a.Text != tc.body {
					t.Error("body changed")
				}
				return nil
			})
			r := httptest.NewRequest(tc.method, tc.path, strings.NewReader(tc.body))
			r.Header.Set("Authorization", tc.auth)
			r.Header.Set("Origin", tc.origin)
			w := httptest.NewRecorder()
			h.ServeHTTP(w, r)
			if w.Code != tc.status {
				t.Fatalf("status %d body %s", w.Code, w.Body.String())
			}
			if (calls == 1) != (tc.status == 202) {
				t.Fatalf("unexpected enqueue count %d", calls)
			}
		})
	}
}
