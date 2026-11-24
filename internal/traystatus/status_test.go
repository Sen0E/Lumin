package traystatus

import (
	"errors"
	"strings"
	"testing"
)

func TestCreationFailureExplainsFullRestart(t *testing.T) {
	missing := CreationFailure(errors.New("mygo: tray icons need libayatana-appindicator3: unsupported"))
	if !strings.Contains(missing, "退出 Lumin") || !strings.Contains(missing, "关闭窗口不会") {
		t.Fatal(missing)
	}
	other := CreationFailure(errors.New("invalid PNG"))
	if !strings.Contains(other, "invalid PNG") || strings.Contains(other, "缺少 AppIndicator") {
		t.Fatal(other)
	}
}
