package desktop

import (
	"context"
	"errors"
	"fmt"
	"os"
	"reflect"
	"runtime"
	"strings"
	"time"

	"github.com/egoist/mygo/ui"
	"lumin/internal/config"
	"lumin/internal/ocr"
	"lumin/internal/plugin"
)

type ocrProbeState struct {
	Busy    bool
	Message string
	Config  config.Config
}

func (a *App) ocrEngine(cfg config.Config) (ocr.Recognizer, error) {
	if a.ocr != nil {
		return a.ocr, nil
	}
	if cfg.OCRDisabled || cfg.OCRPlugin == "" {
		return nil, errors.New("尚未启用 OCR 引擎，请在服务设置 → OCR 服务中选择插件并保存")
	}
	if a.ports.Plugins == nil {
		return nil, errors.New("OCR 插件管理器不可用")
	}
	return plugin.Recognizer{Manager: a.ports.Plugins, Ref: cfg.OCRPlugin, Settings: cfg.OCRSettings}, nil
}
func (a *App) ocrName(cfg config.Config) string {
	if cfg.OCRDisabled || cfg.OCRPlugin == "" {
		return "未启用 OCR 引擎"
	}
	for _, m := range a.installed {
		if m.Ref() == cfg.OCRPlugin {
			return m.Name + " · " + m.Version
		}
	}
	return cfg.OCRPlugin + "（插件未安装或尚未加载）"
}
func (a *App) selectOCR(ref string) {
	a.cancelOCRProbe()
	a.draft.OCRPlugin = ref
	a.draft.OCRDisabled = ref == ""
	a.draft.OCRSettings = nil
	a.pluginEditors = nil
	for _, m := range a.installed {
		if m.Ref() == ref {
			a.draft.OCRSettings = defaults(m)
			break
		}
	}
}
func (a *App) ocrProbeMatches() bool {
	p := a.ocrProbe.Config
	d := a.draft
	return p.OCRPlugin == d.OCRPlugin && p.OCRDisabled == d.OCRDisabled && p.OCRLanguages == d.OCRLanguages && reflect.DeepEqual(p.OCRSettings, d.OCRSettings)
}
func (a *App) cancelOCRProbe() {
	a.ocrProbeGeneration++
	if a.ocrProbeCancel != nil {
		a.ocrProbeCancel()
		a.ocrProbeCancel = nil
	}
	if a.ocrProbe.Busy {
		a.ocrProbe.Busy = false
		a.ocrProbe.Message = "已取消 OCR 测试"
	}
}
func (a *App) testOCR() {
	if a.saving || a.pluginBusy || a.backup.restoring {
		return
	}
	a.cancelOCRProbe()
	cfg := a.draft.Clone()
	engine, err := a.ocrEngine(cfg)
	a.ocrProbe = ocrProbeState{Config: cfg}
	if err != nil {
		a.ocrProbe.Message = err.Error()
		return
	}
	a.ocrProbe.Busy = true
	a.ocrProbe.Message = "正在使用当前草稿测试 OCR…"
	ctx, cancel := context.WithTimeout(a.root, 30*time.Second)
	a.ocrProbeCancel = cancel
	gen := a.ocrProbeGeneration
	go func() {
		defer cancel()
		start := time.Now()
		f, err := os.CreateTemp("", "lumin-ocr-probe-*.png")
		var text string
		if err == nil {
			path := f.Name()
			defer os.Remove(path)
			_, err = f.Write(ocr.ProbeImage)
			closeErr := f.Close()
			if err == nil {
				err = closeErr
			}
			if err == nil {
				text, err = engine.Recognize(ctx, path, cfg.OCRLanguages)
			}
		}
		if ctx.Err() != nil {
			err = ctx.Err()
		}
		elapsed := time.Since(start)
		a.post(func() {
			if a.ocrProbeGeneration != gen {
				return
			}
			a.ocrProbe.Busy = false
			a.ocrProbeCancel = nil
			switch {
			case !a.ocrProbeMatches():
				a.ocrProbe.Message = "OCR 配置已修改，请重新测试"
			case err != nil:
				a.ocrProbe.Message = "OCR 测试失败：" + err.Error()
			case strings.TrimSpace(text) == "":
				a.ocrProbe.Message = "OCR 测试失败：未返回文字"
			default:
				a.ocrProbe.Message = fmt.Sprintf("OCR 测试完成 · %.2f 秒\n识别结果：%s", elapsed.Seconds(), text)
			}
		})
	}()
}
func (a *App) ocrServicesView(c *ui.Context) {
	t := c.Theme()
	ui.Scroll(c.Key("ocr-service")).Grow(1).Disabled(a.saving).Children(func() {
		ui.Column(c).Gap(14).Children(func() {
			ui.Text(c, "OCR 引擎").FontSize(18).Bold()
			ui.Text(c, "已保存："+a.ocrName(a.settings)).FontSize(12).TextColor(t.TextMuted)
			ui.Text(c, "当前草稿："+a.ocrName(a.draft)).Selectable()
			ui.Row(c).Gap(8).Children(func() {
				ui.Button(c, "管理插件").OnClick(func() { a.servicePage = "插件" })
				ui.Button(c, "停用 OCR").Disabled(a.pluginBusy).OnClick(func() { a.selectOCR("") })
			})
			count := 0
			for _, m := range a.installed {
				if !m.Has("recognize") {
					continue
				}
				count++
				ui.Button(c.Key(m.Ref()), "选择 "+m.Name+" · "+m.Version).Disabled(a.pluginBusy || m.Entrypoints[runtime.GOOS+"/"+runtime.GOARCH] == "").OnClick(func() { a.selectOCR(m.Ref()) })
			}
			if count == 0 {
				ui.Text(c, "暂无 OCR 插件，请安装适用于当前平台的 .luminext 插件包。").TextColor(t.TextMuted)
			}
			if a.draft.OCRPlugin != "" && !a.draft.OCRDisabled {
				a.pluginFields(c, "ocr", a.draft.OCRPlugin, &a.draft.OCRSettings)
			}
			ui.Text(c, "测试使用固定英文图片“Lumin translation test.”；若选择云端插件，会将该样例发送给对应服务。不发送当前截图或剪贴板内容。").FontSize(12).TextColor(t.TextMuted)
			ui.Row(c).Gap(8).Children(func() {
				ui.Button(c, "测试 OCR 引擎").Disabled(a.ocrProbe.Busy || a.pluginBusy || a.draft.OCRDisabled || a.draft.OCRPlugin == "").OnClick(a.testOCR)
				if a.ocrProbe.Busy {
					ui.Button(c, "取消 OCR 测试").OnClick(a.cancelOCRProbe)
				}
			})
			if a.ocrProbe.Message != "" {
				message := a.ocrProbe.Message
				if !a.ocrProbeMatches() {
					message = "OCR 配置已修改，请重新测试"
				}
				ui.Text(c.Key("ocr-probe-status"), message).Role(ui.RoleStatus).Selectable().FontSize(12)
			}
		})
	})
}
