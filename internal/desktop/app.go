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
	SetTranslationSize    func(int, int)
	SaveHistoryExport     func(format string) (string, error)
	OpenBackup            func() (string, error)
	SaveBackup            func() (string, error)
	SetRecognitionPinned  func(bool)
	ShowRecognizer        func()
	HideRecognizer        func()
	SetTranslationPinned  func(bool)
	CancelTranslationBlur func()
	CheckTray             func(context.Context) string
	CaptureArea           func(context.Context) (image.Image, error)
	Plugins               *plugin.Manager
	ChoosePlugin          func() (string, error)
	Translators           func(config.Config) []domain.Translator
	Dispatch              func(func())
	ReadSelection         func(context.Context) (string, error)
	ReadClipboard         func() string
	WriteClipboard        func(string)
	ChooseImage           func() (string, error)
	ApplyConfig           func(config.Config) error
	ShowPreferences       func()
	ShowTranslator        func()
	HideTranslator        func()
	Quit                  func()
}

type App struct {
	historyExportBusy                 bool
	historyExportFormat               string
	confirmDelete                     string
	backup                            backupState
	recognition                       recognitionState
	trayStatus                        string
	trayChecking                      bool
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
	ocr                               ocr.Recognizer // Optional test adapter; production always resolves plugins.
	ocrProbe                          ocrProbeState
	ocrProbeGeneration                uint64
	ocrProbeCancel                    context.CancelFunc
	settings                          config.Config
	draft                             config.Config
	page                              string
	servicePage                       string
	favoritesOnly                     bool
	pinned                            bool
	readingSelection                  bool
	sourceCollapsed                   bool
	input                             ui.Handle
	focusInput                        bool
	text, source, target              string
	results                           []domain.Result
	history                           []domain.Entry
	search                            string
	status                            string
	busy, saving, historyBusy, closed bool
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
	a := &App{store: store, ports: ports, settings: cfg.Clone(), draft: cfg.Clone(),
		historyExportFormat: "JSON", page: "常规设置", servicePage: "翻译服务", source: domain.LanguageName(cfg.Source), target: domain.LanguageName(cfg.Target), selectedProvider: "compatible",
		history: history, root: ctx, stop: stop, status: "准备就绪"}
	a.syncPreferences()
	return a
}

func (a *App) Close() {
	a.closed = true
	a.cancelOCRProbe()
	a.cancelProbe()
	a.Cancel()
	a.CancelRecognition()
	a.stop()
}
func (a *App) Notice(s string) { a.status = s }
func (a *App) ShowSettings()   { a.openPreferences("常规设置") }

func (a *App) openPreferences(page string) {
	a.page = page
	if a.ports.ShowPreferences != nil {
		a.ports.ShowPreferences()
	}
}

func (a *App) ShowTranslation() {
	a.cancelTranslationBlur()
	a.sourceCollapsed = false
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
	a.readingSelection = false
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

func (a *App) Paste() {
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
}

func (a *App) copy(text string) {
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
	if a.backup.restoring {
		return
	}
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
	// Enter reading mode only after a valid request has an enabled service.
	a.sourceCollapsed = true
	a.focusInput = false
	gen := a.generation
	revision := a.store.Revision()
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
			saveErr = a.store.AddIfRevision(domain.Entry{ID: rand.Text(), CreatedAt: time.Now(), Request: r, Results: results}, cfg.HistoryLimit, revision)
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
	if a.historyBusy || a.backup.restoring {
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
	if a.saving || a.backup.restoring {
		return
	}
	cfg := a.draft.Clone()
	forceSize := a.preferences.WindowSizeEdited
	if err := cfg.Validate(); err != nil {
		a.status = err.Error()
		return
	}
	a.cancelProbe()
	a.cancelOCRProbe()
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
			a.CancelRecognition()
			previous := a.settings.Clone()
			a.settings = cfg.Clone()
			a.preferences.WindowSizeEdited = false
			a.status = "设置已保存"
			if err := a.applyDesktopSettings(previous, forceSize); err != nil {
				a.status = "设置已保存，桌面集成提示：" + err.Error()
			}
		})
	}()
}

func (a *App) RecognizeImage() { a.showRecognition(); a.startRecognition(false, nil) }

func (a *App) Screenshot(translate bool) {
	if translate {
		a.recognizeTranslation()
	} else {
		a.startRecognition(true, nil)
	}
}

func (a *App) recognizeTranslation() {
	if a.backup.restoring {
		return
	}
	if a.busy {
		return
	}
	if a.ports.CaptureArea == nil {
		a.status = "图片获取接口不可用"
		return
	}
	a.cancelTranslationBlur()
	a.Cancel()
	a.busy = true
	a.status = "选择图片…"
	gen := a.generation
	cfg := a.settings.Clone()
	recognizer, err := a.ocrEngine(cfg)
	if err != nil {
		a.busy = false
		a.status = err.Error()
		a.openServices("OCR 服务")
		return
	}
	a.status = "正在获取屏幕，请框选需要识别的区域…"
	ctx, cancel := context.WithTimeout(a.root, 90*time.Second)
	a.cancel = cancel
	go func() {
		defer cancel()
		var path string
		var err error
		{
			var img image.Image
			img, err = a.ports.CaptureArea(ctx)
			if err == nil {
				var cleanup func()
				path, cleanup, err = capture.TempPNG(img)
				if cleanup != nil {
					defer cleanup()
				}
			}
		}
		var text string
		if err == nil && path != "" && ctx.Err() == nil {
			ocrDone := capture.TracePhase("ocr")
			text, err = recognizer.Recognize(ctx, path, cfg.OCRLanguages)
			ocrDone()
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
				if !errors.Is(err, capture.ErrCanceled) && !errors.Is(err, context.Canceled) {
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
			a.Translate()
		})
	}()
}
