package desktop

import (
	"context"
	"time"
)

// Tray status is kept apart from transient translation/save messages.
func (a *App) CheckTray() {
	if a.closed || a.trayChecking || a.ports.CheckTray == nil {
		return
	}
	a.trayChecking = true
	a.trayStatus = "正在检查系统托盘…"
	ctx, cancel := context.WithTimeout(a.root, 3*time.Second)
	go func() {
		defer cancel()
		status := a.ports.CheckTray(ctx)
		a.post(func() { a.trayChecking = false; a.trayStatus = status })
	}()
}
