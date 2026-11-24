package traystatus

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/godbus/dbus/v5"
)

type fakeRegistry struct {
	watcher bool
	host    any
	items   any
	pids    map[string]uint32
	err     error
}

func (r fakeRegistry) hasWatcher(context.Context) (bool, error) { return r.watcher, r.err }
func (r fakeRegistry) property(_ context.Context, name string) (dbus.Variant, error) {
	if name == "IsStatusNotifierHostRegistered" {
		return dbus.MakeVariant(r.host), nil
	}
	return dbus.MakeVariant(r.items), nil
}
func (r fakeRegistry) processID(_ context.Context, name string) (uint32, error) {
	if pid, ok := r.pids[name]; ok {
		return pid, nil
	}
	return 0, errors.New("service disappeared")
}
func TestPanelRegistrationDiagnosis(t *testing.T) {
	cases := []struct {
		name     string
		registry fakeRegistry
		want     string
	}{
		{"missing watcher", fakeRegistry{}, "未检测到桌面托盘服务"},
		{"missing panel", fakeRegistry{watcher: true, host: false}, "没有面板接收图标"},
		{"invalid host", fakeRegistry{watcher: true, host: "true"}, "无效的面板状态"},
		{"invalid items", fakeRegistry{watcher: true, host: true, items: uint32(1)}, "无效的图标列表"},
		{"empty", fakeRegistry{watcher: true, host: true, items: []string{}}, "尚未找到当前 Lumin"},
		{"other app named lumin", fakeRegistry{watcher: true, host: true, items: []string{"org.lumin/StatusNotifierItem"}, pids: map[string]uint32{"org.lumin": 9}}, "尚未找到当前 Lumin"},
		{"own unique name", fakeRegistry{watcher: true, host: true, items: []string{":1.42/StatusNotifierItem"}, pids: map[string]uint32{":1.42": 42}}, "已注册到桌面"},
		{"own service", fakeRegistry{watcher: true, host: true, items: []string{"org.kde.StatusNotifierItem-42-1"}, pids: map[string]uint32{"org.kde.StatusNotifierItem-42-1": 42}}, "已注册到桌面"},
		{"stale item before own", fakeRegistry{watcher: true, host: true, items: []string{":1.10/old", ":1.42/new"}, pids: map[string]uint32{":1.42": 42}}, "已注册到桌面"},
		{"owner disappeared", fakeRegistry{watcher: true, host: true, items: []string{":1.10/old"}}, "暂时无法确认"},
		{"bus denied", fakeRegistry{err: errors.New("denied")}, "无法检查桌面托盘服务"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := inspect(context.Background(), tc.registry, 42); !strings.Contains(got, tc.want) {
				t.Fatalf("%q want %q", got, tc.want)
			}
		})
	}
}
