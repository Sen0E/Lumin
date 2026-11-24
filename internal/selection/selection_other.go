//go:build !linux && !windows && !darwin

package selection

import "context"

func readNative(context.Context) (string, error) { return "", ErrUnavailable }
