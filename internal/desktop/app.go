// Package desktop owns native views and main-thread presentation state.
package desktop

import (
	"context"
	"crypto/rand"
	"errors"
	"fmt"
	"image"
	"strings"
	"time"

	"github.com/egoist/mygo/ui"

	"lumin/internal/capture"
	"lumin/internal/config"
	"lumin/internal/domain"
	"lumin/internal/ocr"
	"lumin/internal/plugin"
	"lumin/internal/storage"
	"lumin/internal/translation"
)

// Ports isolates OS operations and the event-loop dispatcher for deterministic tests.
type Ports struct {
	CaptureArea     func(context.Context) (image.Image, error)
	Plugins         *plugin.Manager
	ChoosePlugin    func() (string, error)
	Translators     func(config.Config) []domain.Translator
	Dispatch        func(func())
	ReadClipboard   func() string
	WriteClipboard  func(string)
	ChooseImage     func() (string, error)
	ApplyConfig     func(config.Config) error
	ShowPreferences func()
	ShowTranslator  func()
	HideTranslator  func()
	Quit            func()
}

type App struct {
	probe                             serviceProbe
	probeGeneration                   uint64
	probeCancel                       context.CancelFunc
	preferences                       preferencesEditor
	installed                         []plugin.Manifest
	pluginBusy                        bool
	confirmUninstall                  string
	pluginEditors                     map[string]*string
	store                             *storage.Store
	ports                             Ports
	ocr                               ocr.Recognizer
	settings                          config.Config
	draft                             config.Config
	page                              string
	translationPage                   string
	input                             ui.Handle
	focusInput                        bool
	text, source, target              string
	results                           []domain.Result
	history                           []domain.Entry
	search                            string
	status                            string
	busy, saving, historyBusy, closed bool
	watchClipboard                    bool
	lastClipboard                     string
	generation                        uint64
	historyGeneration                 uint64
	cancel                            context.CancelFunc
	root                              context.Context
	stop                              context.CancelFunc
	confirmClear                      bool
	selectedProvider                  string
}

func New(store *storage.Store, cfg config.Config, history []domain.Entry, ports Ports) *App {
	ctx, stop := context.WithCancel(context.Background())
	a := &App{store: store, ports: ports, ocr: ocr.Tesseract{}, settings: cfg.Clone(), draft: cfg.Clone(),
		page: "设置", translationPage: "翻译", source: domain.LanguageName(cfg.Source), target: domain.LanguageName(cfg.Target), selectedProvider: "compatible",
		history: history, root: ctx, stop: stop, status: "准备就绪"}
	a.syncPreferences()
	return a
}

func (a *App) Close()          { a.closed = true; a.cancelProbe(); a.Cancel(); a.stop() }
func (a *App) Notice(s string) { a.status = s }
func (a *App) ShowSettings()   { a.openPreferences("设置") }

func (a *App) openPreferences(page string) {
	a.page = page
	if a.ports.ShowPreferences != nil {
		a.ports.ShowPreferences()
	}
}

func (a *App) ShowTranslation() {
	a.translationPage = "翻译"
	a.focusInput = true
	if a.ports.ShowTranslator != nil {
		a.ports.ShowTranslator()
	}
}

// DismissTranslation ends the current interaction without shutting down the app.
func (a *App) DismissTranslation() {
	a.Cancel()
	if a.ports.HideTranslator != nil {
		a.ports.HideTranslator()
	}
}

// post guards callbacks that were queued just before the window closed.
func (a *App) post(fn func()) {
	a.ports.Dispatch(func() {
		if !a.closed {
			fn()
		}
	})
}

func (a *App) Cancel() {
	a.generation++
	if a.cancel != nil {
		a.cancel()
		a.cancel = nil
	}
	if a.busy {
		a.status = "已取消"
	}
	a.busy = false
}

func (a *App) sourceChanged() { a.Cancel(); a.results = nil }

func (a *App) TranslateText(text string) {
	a.sourceChanged()
	a.text = text
	a.ShowTranslation()
	a.Translate()
}

func (a *App) Paste(translate bool) {
	a.ShowTranslation()
	if a.ports.ReadClipboard == nil {
		return
	}
	text := a.ports.ReadClipboard()
	if strings.TrimSpace(text) == "" {
		a.status = "剪贴板中没有文本"
		return
	}
	a.sourceChanged()
	a.text = text
	if translate {
		a.Translate()
	}
}

func (a *App) ClipboardChanged() {
	if a.closed || !a.watchClipboard || a.ports.ReadClipboard == nil {
		return
	}
	text := a.ports.ReadClipboard()
	if text == a.lastClipboard || strings.TrimSpace(text) == "" {
		return
	}
	a.lastClipboard = text
	a.TranslateText(text)
}

func (a *App) copy(text string) {
	a.lastClipboard = text // Our own copies must not retrigger the watcher.
	if a.ports.WriteClipboard != nil {
		a.ports.WriteClipboard(text)
		a.status = "已复制"
	}
}

func languageCode(name string) string {
	for _, l := range domain.Languages {
		if l.Name == name {
			return l.Code
		}
	}
	return name
}

func (a *App) Translate() {
	r := domain.Request{Text: strings.TrimSpace(a.text), Source: languageCode(a.source), Target: languageCode(a.target)}
	if err := r.Validate(); err != nil {
		a.status = err.Error()
		return
	}
	cfg := a.settings.Clone()
	providers := a.translators(cfg)
	if len(providers) == 0 {
		a.status = "请先在「翻译服务」中配置并启用至少一个服务"
		return
	}
	a.Cancel()
	ctx, cancel := context.WithTimeout(a.root, time.Duration(cfg.TimeoutSeconds)*time.Second)
	a.cancel, a.busy, a.status = cancel, true, "正在翻译…"
	gen := a.generation
	a.results = make([]domain.Result, len(providers))
	for i, p := range providers {
		a.results[i] = domain.Result{ProviderID: p.ID(), ProviderName: p.Name()}
	}
	go func() {
		defer cancel()
		results, err := translation.Run(ctx, r, providers, func(result domain.Result) {
			a.post(func() {
				if a.generation != gen {
					return
				}
				for i := range a.results {
					if a.results[i].ProviderID == result.ProviderID {
						a.results[i] = result
					}
				}
			})
		})
		var saveErr error
		if err == nil && cfg.SaveHistory && ctx.Err() == nil {
			saveErr = a.store.Add(domain.Entry{ID: rand.Text(), CreatedAt: time.Now(), Request: r, Results: results}, cfg.HistoryLimit)
		}
		a.post(func() {
			if a.generation != gen {
				return
			}
			a.busy, a.cancel = false, nil
			a.results = results
			success := 0
			for _, result := range results {
				if result.Error == "" {
					success++
				}
			}
			a.status = fmt.Sprintf("翻译完成 · %d/%d 个服务成功", success, len(results))
			if err != nil {
				a.status = "翻译已取消或超时"
			}
			if saveErr != nil {
				a.status = "翻译完成，但历史记录保存失败：" + saveErr.Error()
			}
			a.loadHistory()
		})
	}()
}

func (a *App) loadHistory() {
	a.historyGeneration++
	gen := a.historyGeneration
	go func() {
		history, err := a.store.History()
		a.post(func() {
			if a.historyGeneration != gen {
				return
			}
			if err != nil {
				a.status = err.Error()
				return
			}
			a.history = history
		})
	}()
}

func (a *App) changeHistory(fn func() error) {
	if a.historyBusy {
		return
	}
	a.historyBusy = true
	go func() {
		err := fn()
		a.post(func() {
			a.historyBusy = false
			if err != nil {
				a.status = err.Error()
			}
			a.loadHistory()
		})
	}()
}

func (a *App) saveSettings() {
	if a.saving {
		return
	}
	cfg := a.draft.Clone()
	if err := cfg.Validate(); err != nil {
		a.status = err.Error()
		return
	}
	a.cancelProbe()
	a.saving = true
	go func() {
		err := a.validatePlugins(cfg)
		if err == nil {
			err = a.store.SaveConfig(cfg)
		}
		a.post(func() {
			a.saving = false
			if err != nil {
				a.status = "保存失败：" + err.Error()
				return
			}
			a.sourceChanged()
			a.settings = cfg.Clone()
			a.status = "设置已保存"
			if a.ports.ApplyConfig != nil {
				if err := a.ports.ApplyConfig(cfg); err != nil {
					a.status = "设置已保存，桌面集成提示：" + err.Error()
				}
			}
		})
	}()
}

func (a *App) recognizeImage() { a.recognize(false, false) }

func (a *App) Screenshot(translate bool) { a.recognize(true, translate) }

func (a *App) recognize(screenshot, translate bool) {
	if a.busy {
		return
	}
	if screenshot && a.ports.CaptureArea == nil || !screenshot && a.ports.ChooseImage == nil {
		a.status = "图片获取接口不可用"
		return
	}
	a.Cancel()
	a.busy = true
	a.status = "选择图片…"
	gen := a.generation
	cfg := a.settings.Clone()
	recognizer := a.ocr
	if cfg.OCRPlugin != "" {
		recognizer = plugin.Recognizer{Manager: a.ports.Plugins, Ref: cfg.OCRPlugin, Settings: cfg.OCRSettings}
	}
	if screenshot {
		a.status = "正在获取屏幕，请框选需要识别的区域…"
	}
	ctx, cancel := context.WithTimeout(a.root, 90*time.Second)
	a.cancel = cancel
	go func() {
		defer cancel()
		var path string
		var err error
		if screenshot {
			var img image.Image
			img, err = a.ports.CaptureArea(ctx)
			if err == nil {
				var cleanup func()
				path, cleanup, err = capture.TempPNG(img)
				if cleanup != nil {
					defer cleanup()
				}
			}
		} else {
			path, err = a.ports.ChooseImage()
		}
		var text string
		if err == nil && path != "" && ctx.Err() == nil {
			text, err = recognizer.Recognize(ctx, path, cfg.OCRLanguages)
		}
		if err == nil {
			err = ctx.Err()
		}
		a.post(func() {
			if a.generation != gen {
				return
			}
			a.busy, a.cancel = false, nil
			if err != nil {
				a.status = err.Error()
				if screenshot && !errors.Is(err, capture.ErrCanceled) && !errors.Is(err, context.Canceled) {
					a.ShowTranslation()
				}
				return
			}
			if path == "" {
				a.status = "已取消图片选择"
				return
			}
			a.text, a.results = text, nil
			a.ShowTranslation()
			a.status = "文字识别完成，可编辑后翻译"
			if translate {
				a.Translate()
			}
		})
	}()
}
