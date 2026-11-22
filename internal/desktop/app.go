// Package desktop owns native views and main-thread presentation state.
package desktop

import (
	"context"
	"crypto/rand"
	"fmt"
	"strings"
	"time"

	"lumin/internal/config"
	"lumin/internal/domain"
	"lumin/internal/ocr"
	"lumin/internal/provider"
	"lumin/internal/storage"
	"lumin/internal/translation"
)

// Ports isolates OS operations and the event-loop dispatcher for deterministic tests.
type Ports struct {
	Translators    func(config.Config) []domain.Translator
	Dispatch       func(func())
	ReadClipboard  func() string
	WriteClipboard func(string)
	ChooseImage    func() (string, error)
	ApplyConfig    func(config.Config) error
}

type App struct {
	store                             *storage.Store
	ports                             Ports
	ocr                               ocr.Recognizer
	settings                          config.Config
	draft                             config.Config
	page                              string
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
	return &App{store: store, ports: ports, ocr: ocr.Tesseract{}, settings: cfg.Clone(), draft: cfg.Clone(),
		page: "翻译", source: domain.LanguageName(cfg.Source), target: domain.LanguageName(cfg.Target), selectedProvider: "compatible",
		history: history, root: ctx, stop: stop, status: "准备就绪"}
}

func (a *App) Close()           { a.closed = true; a.Cancel(); a.stop() }
func (a *App) Notice(s string)  { a.status = s }
func (a *App) ShowSettings()    { a.page = "设置" }
func (a *App) ShowTranslation() { a.page = "翻译" }

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
	a.page = "翻译"
	a.Translate()
}

func (a *App) Paste(translate bool) {
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
	a.page = "翻译"
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
	var providers []domain.Translator
	for _, p := range cfg.Providers {
		if p.Enabled {
			providers = append(providers, provider.New(p, time.Duration(cfg.TimeoutSeconds)*time.Second))
		}
	}
	if a.ports.Translators != nil {
		providers = a.ports.Translators(cfg)
	}
	if len(providers) == 0 {
		a.status = "请先在「翻译服务」中配置并启用至少一个服务"
		a.page = "翻译服务"
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
	a.saving = true
	go func() {
		err := a.store.SaveConfig(cfg)
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

func (a *App) recognizeImage() {
	if a.ports.ChooseImage == nil {
		return
	}
	a.Cancel()
	a.busy = true
	a.status = "选择图片…"
	gen := a.generation
	languages := a.settings.OCRLanguages
	ctx, cancel := context.WithTimeout(a.root, 90*time.Second)
	a.cancel = cancel
	go func() {
		defer cancel()
		path, err := a.ports.ChooseImage()
		var text string
		if err == nil && path != "" && ctx.Err() == nil {
			text, err = a.ocr.Recognize(ctx, path, languages)
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
				return
			}
			if path == "" {
				a.status = "已取消图片选择"
				return
			}
			a.text, a.page, a.results = text, "翻译", nil
			a.status = "文字识别完成，可编辑后翻译"
		})
	}()
}
