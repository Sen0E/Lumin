// Package localapi exposes a small authenticated loopback automation endpoint.
package localapi

import (
	"crypto/subtle"
	"errors"
	"io"
	"net/http"
	"strings"
	"unicode/utf8"

	"lumin/internal/domain"
)

type Action struct{ Kind, Text string }

// Handler acknowledges queueing, not completion of an external translation.
func Handler(token string, enqueue func(Action) error) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/plain; charset=utf-8")
		w.Header().Set("Cache-Control", "no-store")
		if r.Method != http.MethodPost {
			w.Header().Set("Allow", "POST")
			http.Error(w, "POST required", http.StatusMethodNotAllowed)
			return
		}
		if token == "" || subtle.ConstantTimeCompare([]byte(r.Header.Get("Authorization")), []byte("Bearer "+token)) != 1 {
			http.Error(w, "unauthorized", http.StatusUnauthorized)
			return
		}
		if r.Header.Get("Origin") != "" {
			http.Error(w, "browser origins are not allowed", http.StatusForbidden)
			return
		}
		var a Action
		switch r.URL.Path {
		case "/", "/translate":
			a.Kind = "translate"
			b, err := io.ReadAll(http.MaxBytesReader(w, r.Body, domain.MaxTextRunes*4))
			if err != nil {
				var limit *http.MaxBytesError
				status := http.StatusBadRequest
				if errors.As(err, &limit) {
					status = http.StatusRequestEntityTooLarge
				}
				http.Error(w, "invalid body", status)
				return
			}
			a.Text = string(b)
			if !utf8.Valid(b) || utf8.RuneCount(b) > domain.MaxTextRunes || strings.TrimSpace(a.Text) == "" {
				http.Error(w, "invalid text", http.StatusBadRequest)
				return
			}
		case "/input_translate":
			a.Kind = "input"
		case "/config":
			a.Kind = "config"
		default:
			http.NotFound(w, r)
			return
		}
		if err := enqueue(a); err != nil {
			http.Error(w, "application is closing or busy", http.StatusServiceUnavailable)
			return
		}
		w.WriteHeader(http.StatusAccepted)
		_, _ = io.WriteString(w, "queued\n")
	})
}
