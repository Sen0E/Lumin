// Package traystatus diagnoses tray registration without creating native UI.
package traystatus

import (
	"fmt"
	"strings"
)

// CreationFailure distinguishes a library loaded at process startup from panel
// availability. Installing that library cannot repair an already running MyGo.
func CreationFailure(err error) string {
	if strings.Contains(err.Error(), "libayatana-appindicator3") {
		return "托盘创建失败：缺少 AppIndicator 库。安装 GTK3 版本的 libayatana-appindicator 后，请点击“退出 Lumin”再重新启动；关闭窗口不会重载依赖。"
	}
	return fmt.Sprintf("托盘创建失败：%v。可重新运行 Lumin 找回设置并退出。", err)
}
