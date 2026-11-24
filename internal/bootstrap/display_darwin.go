//go:build darwin

package bootstrap

import (
	"github.com/egoist/mygo"
	"lumin/internal/capture"
)

func fullscreenOnDisplay(*mygo.Window, capture.DisplayTarget) bool { return false }
