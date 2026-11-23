package desktop

import (
	"context"
	"errors"
	"fmt"
	"reflect"
	"strings"
	"time"

	"lumin/internal/config"
	"lumin/internal/domain"
	"lumin/internal/plugin"
	"lumin/internal/provider"
)

// A probe is separate from translation state: it cannot change input or history.
type serviceProbe struct {
	Provider       config.Provider
	TimeoutSeconds int
	Busy, Failed   bool
	Message        string
}

func (a *App) translators(cfg config.Config) []domain.Translator {
	if a.ports.Translators != nil {
		return a.ports.Translators(cfg)
	}
	var list []domain.Translator
	for _, p := range cfg.Providers {
		if !p.Enabled {
			continue
		}
		if p.Kind == "plugin" {
			list = append(list, plugin.Translator{Manager: a.ports.Plugins, Ref: p.Plugin, ProviderID: p.ID, ProviderName: p.Name, Settings: p.Settings})
		} else {
			list = append(list, provider.New(p, time.Duration(cfg.TimeoutSeconds)*time.Second))
		}
	}
	return list
}

func (a *App) cancelProbe() {
	a.probeGeneration++
	if a.probeCancel != nil {
		a.probeCancel()
		a.probeCancel = nil
	}
	if a.probe.Busy {
		a.probe.Busy = false
		a.probe.Message = "已取消测试"
		a.probe.Failed = false
	}
}

func (a *App) probeMatchesDraft() bool {
	if a.probe.TimeoutSeconds != a.draft.TimeoutSeconds {
		return false
	}
	for _, p := range a.draft.Providers {
		if p.ID == a.probe.Provider.ID {
			return reflect.DeepEqual(p, a.probe.Provider)
		}
	}
	return false
}

func (a *App) testProvider(id string) {
	if a.saving || a.pluginBusy {
		return
	}
	cfg := config.Default()
	cfg.TimeoutSeconds = a.draft.TimeoutSeconds
	found := false
	// Clone maps too: plugin edits must not change the in-flight request.
	for _, p := range a.draft.Clone().Providers {
		if p.ID == id {
			cfg.Providers = []config.Provider{p}
			found = true
			break
		}
	}
	if !found {
		return
	}
	a.cancelProbe()
	a.probe = serviceProbe{Provider: cfg.Providers[0], TimeoutSeconds: cfg.TimeoutSeconds}
	cfg.Providers[0].Enabled = true // A disabled draft can be tested without enabling it.
	if err := cfg.Validate(); err != nil {
		a.probe.Failed = true
		a.probe.Message = err.Error()
		return
	}
	providers := a.translators(cfg)
	if len(providers) != 1 {
		a.probe.Failed = true
		a.probe.Message = "无法创建待测试服务"
		return
	}
	ctx, cancel := context.WithTimeout(a.root, time.Duration(cfg.TimeoutSeconds)*time.Second)
	a.probeCancel = cancel
	a.probe.Busy = true
	a.probe.Message = "正在测试…"
	gen := a.probeGeneration
	go func() {
		defer cancel()
		start := time.Now()
		var text string
		err := a.validatePlugins(cfg)
		if err == nil {
			text, err = providers[0].Translate(ctx, domain.Request{Text: "Hello, world!", Source: "en", Target: "zh"})
		}
		if ctx.Err() != nil {
			err = ctx.Err()
		}
		if err == nil && strings.TrimSpace(text) == "" {
			err = errors.New("服务未返回译文")
		}
		duration := time.Since(start)
		a.post(func() {
			if a.probeGeneration != gen {
				return
			}
			a.probe.Busy = false
			a.probeCancel = nil
			if !a.probeMatchesDraft() {
				a.probe.Message = "配置已修改，请重新测试"
				a.probe.Failed = false
				return
			}
			a.probe.Failed = err != nil
			if err != nil {
				switch {
				case errors.Is(err, context.DeadlineExceeded):
					a.probe.Message = "测试超时，请检查网络、接口或请求超时设置"
				case errors.Is(err, context.Canceled):
					a.probe.Message = "已取消测试"
				default:
					a.probe.Message = "测试失败：" + err.Error()
				}
				return
			}
			a.probe.Message = fmt.Sprintf("测试成功 · %.1f 秒\n%s", duration.Seconds(), preview(text, 240))
		})
	}()
}

func (a *App) moveProvider(id string, delta int) {
	if a.saving || delta != 1 && delta != -1 {
		return
	}
	for i, p := range a.draft.Providers {
		if p.ID != id {
			continue
		}
		j := i + delta
		if j >= 0 && j < len(a.draft.Providers) {
			a.draft.Providers[i], a.draft.Providers[j] = a.draft.Providers[j], a.draft.Providers[i]
			a.status = "服务顺序已调整，保存后生效"
		}
		return
	}
}

func (a *App) removeProvider(id string) {
	if a.saving {
		return
	}
	if a.probe.Provider.ID == id {
		a.cancelProbe()
		a.probe = serviceProbe{}
	}
	for i, p := range a.draft.Providers {
		if p.ID == id {
			a.draft.Providers = append(a.draft.Providers[:i], a.draft.Providers[i+1:]...)
			break
		}
	}
}
