package desktop

import (
	"context"
	"testing"
	"time"

	"github.com/egoist/mygo/ui"
)

func TestTrayDiagnosisPersistsAcrossTranslationMessages(t *testing.T) {
	a, q := testApp(t)
	a.ports.CheckTray = func(context.Context) string { return "托盘已注册到桌面" }
	tt := ui.NewTester(a.View, 920, 680)
	click(t, tt, "检查托盘")
	drainUntil(t, q, func() bool { return !a.trayChecking })
	a.Notice("翻译完成")
	tt.Frame()
	tt.Frame()
	if !tt.HasText("托盘已注册到桌面") || a.status != "翻译完成" {
		t.Fatal("tray diagnostic was overwritten")
	}
}

func TestTrayCheckSingleFlightAndShutdown(t *testing.T) {
	a, q := testApp(t)
	started, stopped := make(chan struct{}), make(chan struct{})
	a.ports.CheckTray = func(ctx context.Context) string {
		close(started)
		<-ctx.Done()
		close(stopped)
		return "late result"
	}
	a.CheckTray()
	<-started
	a.CheckTray() // A second worker would close started twice.
	a.Close()
	select {
	case <-stopped:
	case <-time.After(time.Second):
		t.Fatal("quit did not cancel diagnostic")
	}
	select {
	case fn := <-q:
		fn()
	case <-time.After(time.Second):
		t.Fatal("missing completion")
	}
	if a.trayStatus == "late result" {
		t.Fatal("closed app accepted diagnostic")
	}
}
