package selection

import (
	"context"
	"errors"
	"strings"
	"testing"
)

func TestSelectionValidation(t *testing.T) {
	for _, tc := range []struct {
		text string
		want error
	}{
		{"  \n", ErrEmpty}, {strings.Repeat("中", MaxCharacters+1), ErrTooLarge},
		{strings.Repeat("界", MaxCharacters), nil}, {"  Hello 世界\n", nil},
	} {
		got, err := validate(tc.text)
		if !errors.Is(err, tc.want) || err == nil && got != tc.text {
			t.Fatalf("length %d: %q %v", len(tc.text), got, err)
		}
	}
}
func TestSelectionCanceledBeforeOSAccess(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := Read(ctx); !errors.Is(err, context.Canceled) {
		t.Fatal(err)
	}
}
