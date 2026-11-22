// Package bootstrap is the composition root for Lumin's desktop application.
package bootstrap

import (
	"context"
	"errors"
	"fmt"
	"log"
	"net"
	"net/http"
	"strings"
	"sync/atomic"
	"time"

	"github.com/egoist/mygo"
	"github.com/egoist/mygo/ui"

	"lumin/internal/config"
	"lumin/internal/desktop"
	"lumin/internal/localapi"
	"lumin/internal/storage"
)

func Run(icon []byte) error {
	dir, err := storage.DefaultDir()
	if err != nil {
		return err
	}
	store, err := storage.New(dir)
	if err != nil {
		return err
	}
	cfg, err := store.LoadConfig()
	if err != nil {
		return fmt.Errorf("加载 %s/config.json：%w", dir, err)
	}
	history, historyErr := store.History()
	mygo.App.SetName("Lumin")
	if !mygo.App.RequestSingleInstanceLock() {
		return nil
	}
	var window *mygo.Window
	var state *desktop.App
	var tray *mygo.Tray
	var server *http.Server
	var offClipboard func()
	var closed atomic.Bool
	var pending atomic.Int32
	show := func() {
		if window != nil && !closed.Load() {
			window.Show()
			window.Focus()
		}
	}
	apply := func(next config.Config) error {
		mygo.GlobalShortcut.UnregisterAll()
		var failures []string
		register := func(keys string, fn func()) {
			if keys == "" {
				return
			}
			if err := mygo.GlobalShortcut.Register(keys, fn); err != nil {
				failures = append(failures, keys+"："+err.Error())
			}
		}
		register(next.InputShortcut, func() { window.Update(func() { state.ShowTranslation() }); show() })
		register(next.ClipboardShortcut, func() { window.Update(func() { state.Paste(true) }); show() })
		if next.ServerEnabled != cfg.ServerEnabled || next.ServerAddress != cfg.ServerAddress || next.ServerToken != cfg.ServerToken {
			failures = append(failures, "外部调用设置将在重启后生效")
		}
		if len(failures) != 0 {
			return errors.New(strings.Join(failures, "；"))
		}
		return nil
	}
	mygo.App.WhenReady(func() {
		state = desktop.New(store, cfg, history, desktop.Ports{
			Dispatch:      func(fn func()) { window.Update(fn) },
			ReadClipboard: mygo.Clipboard.ReadText, WriteClipboard: mygo.Clipboard.WriteText,
			ChooseImage: func() (string, error) {
				paths, err := mygo.Dialog.Open(mygo.OpenDialogOptions{Parent: window, Title: "选择需要识别的图片", Filters: []mygo.FileFilter{{Name: "图片", Extensions: []string{"png", "jpg", "jpeg", "bmp", "tif", "tiff", "webp"}}}})
				if err != nil || len(paths) == 0 {
					return "", err
				}
				return paths[0], nil
			}, ApplyConfig: apply,
		})
		window = mygo.NewWindow(mygo.WindowOptions{Title: "Lumin · 原生翻译", Width: 1100, Height: 800, MinWidth: 920, MinHeight: 680, StateKey: "main", Content: ui.View(state.View)})
		window.OnClosed(func() { closed.Store(true); state.Close(); mygo.App.Quit() })
		offClipboard = mygo.Clipboard.OnChanged(func() { window.Update(state.ClipboardChanged) })
		tray, err = mygo.NewTray(mygo.TrayOptions{Icon: icon, ToolTip: "Lumin 翻译", Menu: mygo.NewMenu([]*mygo.MenuItem{
			{Label: "打开翻译", Click: func(*mygo.MenuItem, *mygo.Window) { window.Update(state.ShowTranslation); show() }},
			{Label: "翻译剪贴板", Click: func(*mygo.MenuItem, *mygo.Window) { window.Update(func() { state.Paste(true) }); show() }},
			mygo.Separator(), {Role: mygo.RoleQuit},
		})})
		var notices []string
		if err != nil {
			notices = append(notices, "托盘不可用："+err.Error())
		}
		if err := apply(cfg); err != nil {
			notices = append(notices, "快捷键："+err.Error())
		}
		if historyErr != nil {
			notices = append(notices, historyErr.Error())
		}
		if cfg.ServerEnabled {
			listener, err := net.Listen("tcp", cfg.ServerAddress)
			if err != nil {
				notices = append(notices, "外部调用启动失败："+err.Error())
			} else {
				server = &http.Server{ReadHeaderTimeout: 3 * time.Second, ReadTimeout: 5 * time.Second, WriteTimeout: 5 * time.Second, IdleTimeout: 15 * time.Second, MaxHeaderBytes: 8192,
					Handler: localapi.Handler(cfg.ServerToken, func(action localapi.Action) error {
						if closed.Load() {
							return errors.New("closed")
						}
						if pending.Add(1) > 16 {
							pending.Add(-1)
							return errors.New("busy")
						}
						window.Update(func() {
							defer pending.Add(-1)
							if closed.Load() {
								return
							}
							switch action.Kind {
							case "translate":
								state.TranslateText(action.Text)
							case "input":
								state.ShowTranslation()
							case "config":
								state.ShowSettings()
							}
							show()
						})
						return nil
					}),
				}
				go func() {
					if err := server.Serve(listener); err != nil && !errors.Is(err, http.ErrServerClosed) {
						log.Printf("local API: %v", err)
					}
				}()
			}
		}
		if len(notices) > 0 {
			window.Update(func() { state.Notice(strings.Join(notices, "；")) })
		}
	})
	mygo.App.OnSecondInstance(func([]string, string) { show() })
	mygo.App.OnActivate(func(bool) { show() })
	mygo.App.OnQuit(func() {
		closed.Store(true)
		if state != nil {
			state.Close()
		}
		if offClipboard != nil {
			offClipboard()
		}
		mygo.GlobalShortcut.UnregisterAll()
		if tray != nil {
			tray.Destroy()
		}
		if server != nil {
			ctx, cancel := context.WithTimeout(context.Background(), time.Second)
			defer cancel()
			_ = server.Shutdown(ctx)
		}
	})
	return mygo.App.Run()
}
