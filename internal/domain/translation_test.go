package domain

import (
	"strings"
	"testing"
)

func TestRequestBoundaries(t *testing.T) {
	for _, tc := range []struct {
		text, source, target string
		valid                bool
	}{
		{"你好", "auto", "en", true},
		{strings.Repeat("中", MaxTextRunes), "zh", "en", true},
		{strings.Repeat("中", MaxTextRunes+1), "zh", "en", false},
		{" \n ", "auto", "en", false},
		{"hello", "en", "auto", false},
		{"hello", "unknown", "en", false},
		{string([]byte{255}), "en", "zh", false},
	} {
		err := (Request{Text: tc.text, Source: tc.source, Target: tc.target}).Validate()
		if (err == nil) != tc.valid {
			t.Errorf("source=%s target=%s bytes=%d: %v", tc.source, tc.target, len(tc.text), err)
		}
	}
}
