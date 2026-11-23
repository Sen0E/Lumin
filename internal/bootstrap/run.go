// Package bootstrap is the composition root for Lumin's desktop application.
package bootstrap

import (
	"context"
	"errors"
	"fmt"
	"image"
	"log"
	"net"
	"net/http"
	"path/filepath"
	"strings"
	"sync/atomic"
	"time"

	"github.com/egoist/mygo"
	"github.com/egoist/mygo/ui"

	"lumin/internal/config"
	"lumin/internal/desktop"
	"lumin/internal/localapi"
	"lumin/internal/plugin"
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
	plugins, err := plugin.New(filepath.Join(dir, "plugins"))
	if err != nil {
		return err
	}
	history, historyErr := store.History()
	mygo.App.SetName("Lumin")
	if !mygo.App.RequestSingleInstanceLock() {
		return nil
	}
	var window, translationWindow *mygo.Window
	var windows windowSet
	var state *desktop.App
	var tray *mygo.Tray
	var server *http.Server
	var offClipboard func()
	var closed atomic.Bool
	var capturing atomic.Bool
	var pending atomic.Int32
	// Keep both retained views up to date even when the dispatch window is hidden.
	dispatch := func(fn func()) {
		window.Update(func() {
			if closed.Load() || windows.quitting {
				return
			}
			fn()
			translationWindow.Invalidate()
		})
	}
	showPreferences := func() {
		if !closed.Load() && window != nil {
			windows.show(window)
			window.Invalidate()
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
		register(next.InputShortcut, func() { dispatch(state.ShowTranslation) })
		register(next.ClipboardShortcut, func() { dispatch(func() { state.Paste(true) }) })
		register(next.ScreenshotShortcut, func() { dispatch(func() { state.Screenshot(true) }) })
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
			Dispatch:        dispatch,
			ShowPreferences: showPreferences,
			ShowTranslator: func() {
				windows.show(translationWindow)
				translationWindow.Invalidate()
			},
			HideTranslator: func() { windows.hideOnClose(translationWindow) },
			Quit:           mygo.App.Quit,
			Plugins:        plugins,
			CaptureArea: func(ctx context.Context) (image.Image, error) {
				if !capturing.CompareAndSwap(false, true) {
					return nil, errors.New("正在进行截图")
				}
				defer capturing.Store(false)
				return captureArea(ctx, &windows, &closed)
			},
			ChoosePlugin: func() (string, error) {
				paths, err := mygo.Dialog.Open(mygo.OpenDialogOptions{Parent: window, Title: "安装 Lumin 插件", Filters: []mygo.FileFilter{{Name: "Lumin 插件", Extensions: []string{"luminext", "potext"}}}})
				if err != nil || len(paths) == 0 {
					return "", err
				}
				return paths[0], nil
			},
			ReadClipboard: mygo.Clipboard.ReadText, WriteClipboard: mygo.Clipboard.WriteText,
			ChooseImage: func() (string, error) {
				paths, err := mygo.Dialog.Open(mygo.OpenDialogOptions{Parent: translationWindow, Title: "选择需要识别的图片", Filters: []mygo.FileFilter{{Name: "图片", Extensions: []string{"png", "jpg", "jpeg", "bmp", "tif", "tiff", "webp"}}}})
				if err != nil || len(paths) == 0 {
					return "", err
				}
				return paths[0], nil
			}, ApplyConfig: apply,
		})
		window = mygo.NewWindow(mygo.WindowOptions{Title: "Lumin · 设置", Width: 1100, Height: 800, MinWidth: 920, MinHeight: 680, StateKey: "preferences", Content: ui.View(state.View)})
		translationWindow = mygo.NewWindow(mygo.WindowOptions{Title: "Lumin · 翻译", Width: 520, Height: 620, MinWidth: 460, MinHeight: 480, StateKey: "translation", Hidden: true, Content: ui.View(state.TranslationView)})
		windows.preferences, windows.translator = window, translationWindow
		state.RefreshPlugins()
		window.OnClose(func(e *mygo.CloseEvent) {
			if windows.hideOnClose(window) {
				e.PreventDefault()
			}
		})
		translationWindow.OnClose(func(e *mygo.CloseEvent) {
			if windows.hideOnClose(translationWindow) {
				e.PreventDefault()
				state.Cancel()
				window.Invalidate()
			}
		})
		offClipboard = mygo.Clipboard.OnChanged(func() { dispatch(state.ClipboardChanged) })
		tray, err = mygo.NewTray(mygo.TrayOptions{Icon: icon, ToolTip: "Lumin 翻译", Menu: mygo.NewMenu([]*mygo.MenuItem{
			{Label: "打开翻译", Click: func(*mygo.MenuItem, *mygo.Window) { dispatch(state.ShowTranslation) }},
			{Label: "翻译剪贴板", Click: func(*mygo.MenuItem, *mygo.Window) { dispatch(func() { state.Paste(true) }) }},
			{Label: "截图识字", Click: func(*mygo.MenuItem, *mygo.Window) { dispatch(func() { state.Screenshot(false) }) }},
			{Label: "截图翻译", Click: func(*mygo.MenuItem, *mygo.Window) { dispatch(func() { state.Screenshot(true) }) }},
			mygo.Separator(),
			{Label: "偏好设置", Click: func(*mygo.MenuItem, *mygo.Window) { dispatch(state.ShowSettings) }},
			{Label: "退出 Lumin", Role: mygo.RoleQuit},
		})})
		var notices []string
		if err != nil {
			notices = append(notices, "托盘不可用；关闭后可重新运行 Lumin 找回设置，或使用快捷键打开翻译："+err.Error())
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
						// Always decrement even if a quit started after accepting the request.
						window.Update(func() {
							defer pending.Add(-1)
							if closed.Load() || windows.quitting {
								return
							}
							switch action.Kind {
							case "translate":
								state.TranslateText(action.Text)
							case "input":
								state.ShowTranslation()
							case "ocr_recognize", "ocr_translate":
								state.Screenshot(action.Kind == "ocr_translate")
								return
							case "config":
								state.ShowSettings()
							}
							translationWindow.Invalidate()
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
	mygo.App.OnSecondInstance(func([]string, string) { showPreferences() })
	mygo.App.OnActivate(func(visible bool) {
		if !visible && !capturing.Load() {
			showPreferences()
		}
	})
	mygo.App.OnBeforeQuit(func(*mygo.QuitEvent) { windows.quitting = true })
	mygo.App.OnQuitCanceled(func() { windows.quitting = false })
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
