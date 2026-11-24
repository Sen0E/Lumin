//go:build !linux

package traystatus

import "context"

func Check(context.Context) string {
	return "托盘图标已创建。若未显示，请检查系统通知区域或菜单栏的隐藏图标设置。"
}
