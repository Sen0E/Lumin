package capture

import (
	"context"
	"crypto/rand"
	"errors"
	"fmt"
	"image"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/godbus/dbus/v5"
)

const portalName = "org.freedesktop.portal.Desktop"
const requestInterface = "org.freedesktop.portal.Request"

// Screen uses the desktop permission portal on both Wayland and X11. A portal
// denial is final; helpers are only tried when the portal is unavailable.
func Screen(ctx context.Context) (image.Image, error) {
	img, err := portalScreen(ctx)
	if !errors.Is(err, ErrUnavailable) {
		return img, err
	}
	return helperScreen(ctx)
}

func portalScreen(ctx context.Context) (image.Image, error) {
	conn, err := dbus.ConnectSessionBus(dbus.WithContext(ctx))
	if err != nil {
		return nil, fmt.Errorf("%w：无法连接会话总线", ErrUnavailable)
	}
	defer conn.Close()
	options := []dbus.MatchOption{dbus.WithMatchSender(portalName), dbus.WithMatchInterface(requestInterface), dbus.WithMatchMember("Response")}
	if err := conn.AddMatchSignal(options...); err != nil {
		return nil, errors.New("无法订阅截图门户响应，请检查会话总线权限")
	}
	signals := make(chan *dbus.Signal, 16)
	conn.Signal(signals)
	defer conn.RemoveSignal(signals)
	var handle dbus.ObjectPath
	err = conn.Object(portalName, "/org/freedesktop/portal/desktop").CallWithContext(ctx, "org.freedesktop.portal.Screenshot.Screenshot", 0, "", map[string]dbus.Variant{
		"handle_token": dbus.MakeVariant("lumin_" + strings.ToLower(rand.Text())), "interactive": dbus.MakeVariant(false), "modal": dbus.MakeVariant(false),
	}).Store(&handle)
	if err != nil {
		if ctx.Err() != nil {
			return nil, ctx.Err()
		}
		var busErr dbus.Error
		if errors.As(err, &busErr) {
			switch busErr.Name {
			case "org.freedesktop.DBus.Error.ServiceUnknown", "org.freedesktop.DBus.Error.UnknownMethod", "org.freedesktop.DBus.Error.UnknownInterface", "org.freedesktop.DBus.Error.UnknownObject":
				return nil, ErrUnavailable
			}
		}
		return nil, errors.New("截图门户拒绝请求，请检查桌面截图权限")
	}
	defer func() {
		closeCtx, cancel := context.WithTimeout(context.Background(), time.Second)
		defer cancel()
		_ = conn.Object(portalName, handle).CallWithContext(closeCtx, requestInterface+".Close", 0).Err
	}()
	for {
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		case sig, ok := <-signals:
			if !ok {
				return nil, errors.New("截图门户连接已断开")
			}
			if sig.Path != handle {
				continue
			}
			var code uint32
			var result map[string]dbus.Variant
			if err := dbus.Store(sig.Body, &code, &result); err != nil {
				return nil, errors.New("截图门户返回了无效响应")
			}
			if code == 1 {
				return nil, ErrCanceled
			}
			if code != 0 {
				return nil, errors.New("截图失败或未获得桌面授权")
			}
			uri, ok := result["uri"].Value().(string)
			if !ok {
				return nil, errors.New("截图门户未返回图片")
			}
			path, err := fileURI(uri)
			if err != nil {
				return nil, err
			}
			return ReadImage(path)
		}
	}
}

func fileURI(raw string) (string, error) {
	u, err := url.Parse(raw)
	if err != nil || u.Scheme != "file" || (u.Host != "" && u.Host != "localhost") || u.RawQuery != "" || u.Fragment != "" || !filepath.IsAbs(u.Path) {
		return "", errors.New("截图门户返回的文件地址无效")
	}
	return u.Path, nil
}

func helperScreen(ctx context.Context) (image.Image, error) {
	dir, err := os.MkdirTemp("", "lumin-screen-*")
	if err != nil {
		return nil, err
	}
	defer os.RemoveAll(dir)
	path := filepath.Join(dir, "screen.png")
	var name string
	var args []string
	if os.Getenv("WAYLAND_DISPLAY") != "" {
		name = "grim"
		args = []string{path}
	} else {
		name = "maim"
		args = []string{path}
	}
	bin, err := exec.LookPath(name)
	if err != nil {
		return nil, fmt.Errorf("%w：请安装并启用桌面 Screenshot 门户；备用截图工具 %s 未安装", ErrUnavailable, name)
	}
	if err := exec.CommandContext(ctx, bin, args...).Run(); err != nil {
		if ctx.Err() != nil {
			return nil, ctx.Err()
		}
		return nil, fmt.Errorf("%s 截图失败，请检查桌面支持与权限", name)
	}
	return ReadImage(path)
}
