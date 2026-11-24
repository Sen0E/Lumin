//go:build !linux && !darwin && !windows

package capture

import (
	"context"
	"image"
)

func Screen(context.Context, DisplayTarget) (image.Image, error) { return nil, ErrUnavailable }
