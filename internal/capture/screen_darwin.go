package capture

import (
	"context"
	"errors"
	"image"
	"os"
	"os/exec"
	"path/filepath"
)

// Screen captures the main display using the system's screen-recording permission.
func Screen(ctx context.Context) (image.Image, error) {
	dir, err := os.MkdirTemp("", "lumin-screen-*")
	if err != nil {
		return nil, err
	}
	defer os.RemoveAll(dir)
	path := filepath.Join(dir, "screen.png")
	if err := exec.CommandContext(ctx, "/usr/sbin/screencapture", "-x", "-m", "-t", "png", path).Run(); err != nil {
		if ctx.Err() != nil {
			return nil, ctx.Err()
		}
		return nil, errors.New("系统截图失败，请在系统设置中允许屏幕录制")
	}
	return ReadImage(path)
}
