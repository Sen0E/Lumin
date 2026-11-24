package traystatus

import (
	"context"
	"fmt"
	"os"
	"strings"

	"github.com/godbus/dbus/v5"
)

const watcher = "org.kde.StatusNotifierWatcher"
const watcherPath = dbus.ObjectPath("/StatusNotifierWatcher")

type registry interface {
	hasWatcher(context.Context) (bool, error)
	property(context.Context, string) (dbus.Variant, error)
	processID(context.Context, string) (uint32, error)
}

type sessionRegistry struct{ conn *dbus.Conn }

func (r sessionRegistry) hasWatcher(ctx context.Context) (bool, error) {
	var owned bool
	err := r.conn.BusObject().CallWithContext(ctx, "org.freedesktop.DBus.NameHasOwner", 0, watcher).Store(&owned)
	return owned, err
}
func (r sessionRegistry) property(ctx context.Context, name string) (dbus.Variant, error) {
	var value dbus.Variant
	err := r.conn.Object(watcher, watcherPath).CallWithContext(ctx, "org.freedesktop.DBus.Properties.Get", 0, watcher, name).Store(&value)
	return value, err
}
func (r sessionRegistry) processID(ctx context.Context, name string) (uint32, error) {
	var pid uint32
	err := r.conn.BusObject().CallWithContext(ctx, "org.freedesktop.DBus.GetConnectionUnixProcessID", 0, name).Store(&pid)
	return pid, err
}

// Check uses the panel registry rather than assuming NewTray success means an
// icon is visible. The registry cannot tell if Plasma moved it to the overflow.
func Check(ctx context.Context) string {
	conn, err := dbus.ConnectSessionBus(dbus.WithContext(ctx))
	if err != nil {
		return fmt.Sprintf("无法检查托盘：会话总线连接失败：%v", err)
	}
	defer conn.Close()
	return inspect(ctx, sessionRegistry{conn}, uint32(os.Getpid()))
}

func inspect(ctx context.Context, r registry, pid uint32) string {
	present, err := r.hasWatcher(ctx)
	if err != nil {
		return fmt.Sprintf("无法检查桌面托盘服务：%v", err)
	}
	if !present {
		return "未检测到桌面托盘服务。KDE 请确认面板有“系统托盘”；GNOME 需要启用 AppIndicator 扩展。恢复桌面托盘后再次检查。"
	}
	host, err := r.property(ctx, "IsStatusNotifierHostRegistered")
	if err != nil {
		return fmt.Sprintf("无法读取托盘面板状态：%v", err)
	}
	registered, ok := host.Value().(bool)
	if !ok {
		return "桌面托盘服务返回了无效的面板状态。"
	}
	if !registered {
		return "桌面托盘服务已运行，但没有面板接收图标。请启用面板的“系统托盘”组件。"
	}
	value, err := r.property(ctx, "RegisteredStatusNotifierItems")
	if err != nil {
		return fmt.Sprintf("无法读取已注册的托盘图标：%v", err)
	}
	items, ok := value.Value().([]string)
	if !ok {
		return "桌面托盘服务返回了无效的图标列表。"
	}
	var lookupErr error
	for _, item := range items {
		// Hosts return a service name or service + object path. Do not infer PID
		// from the text: other apps can use a similar service or icon name.
		service, _, _ := strings.Cut(item, "/")
		if service == "" {
			continue
		}
		owner, err := r.processID(ctx, service)
		if err != nil {
			lookupErr = err
			continue
		}
		if owner == pid {
			return "Lumin 托盘已注册到桌面。若未显示，请展开托盘隐藏图标，并在 KDE“系统托盘设置 → 条目”中设为“始终显示”。"
		}
	}
	if lookupErr != nil {
		return fmt.Sprintf("暂时无法确认 Lumin 托盘注册状态：%v。请稍后再次检查。", lookupErr)
	}
	return "桌面托盘可用，但尚未找到当前 Lumin 的图标。请稍后再次检查；若仍未出现，请点击“退出 Lumin”后重新启动。"
}
