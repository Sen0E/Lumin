package desktop

import (
	"crypto/rand"
	"fmt"
	"runtime"
	"strings"

	"github.com/egoist/mygo/ui"
	"lumin/internal/config"
	"lumin/internal/plugin"
)

func (a *App) validatePlugins(cfg config.Config) error {
	for _, p := range cfg.Providers {
		if p.Enabled && p.Kind == "plugin" {
			if err := a.ports.Plugins.ValidateUse(p.Plugin, "translate", p.Settings); err != nil {
				return fmt.Errorf("%s：%w", p.Name, err)
			}
		}
	}
	if cfg.OCRPlugin != "" && !cfg.OCRDisabled {
		return a.ports.Plugins.ValidateUse(cfg.OCRPlugin, "recognize", cfg.OCRSettings)
	}
	return nil
}

func (a *App) RefreshPlugins() {
	if a.ports.Plugins == nil || a.pluginBusy {
		return
	}
	a.pluginBusy = true
	go func() {
		list, err := a.ports.Plugins.List()
		a.post(func() {
			a.pluginBusy = false
			a.installed = list
			if err != nil {
				a.status = err.Error()
			}
		})
	}()
}

// Installation never starts the executable. Enabling a profile is a separate action.
func (a *App) installPlugin() {
	if a.pluginBusy || a.ports.ChoosePlugin == nil || a.ports.Plugins == nil {
		return
	}
	a.pluginBusy = true
	go func() {
		path, err := a.ports.ChoosePlugin()
		var manifest plugin.Manifest
		if err == nil && path != "" {
			manifest, err = a.ports.Plugins.Install(path)
		}
		a.post(func() {
			a.pluginBusy = false
			if err != nil {
				a.status = err.Error()
				return
			}
			if path == "" {
				return
			}
			a.status = "已安装 " + manifest.Name + "；添加服务或选择 OCR 后保存设置即可使用"
			a.RefreshPlugins()
		})
	}()
}

func (a *App) pluginUsed(ref string) bool {
	for _, cfg := range []config.Config{a.settings, a.draft} {
		if cfg.OCRPlugin == ref {
			return true
		}
		for _, p := range cfg.Providers {
			if p.Plugin == ref {
				return true
			}
		}
	}
	return false
}

func (a *App) uninstallPlugin(ref string) {
	if a.pluginBusy || a.busy || a.recognition.busy || a.ocrProbe.Busy || a.probe.Busy || a.saving || a.pluginUsed(ref) {
		a.status = "请先移除使用此插件的服务或 OCR 设置，并保存"
		return
	}
	a.pluginBusy = true
	a.confirmUninstall = ""
	go func() {
		err := a.ports.Plugins.Remove(ref)
		a.post(func() {
			a.pluginBusy = false
			if err != nil {
				a.status = err.Error()
				return
			}
			a.status = "插件已卸载"
			a.RefreshPlugins()
		})
	}()
}

func defaults(m plugin.Manifest) map[string]string {
	values := map[string]string{}
	for _, f := range m.Fields {
		values[f.Key] = f.Default
	}
	return values
}

// Values live outside the build pass; MyGo observes bound edits on the next pass.
func (a *App) pluginFields(c *ui.Context, key, ref string, values *map[string]string) {
	var manifest *plugin.Manifest
	for i := range a.installed {
		if a.installed[i].Ref() == ref {
			manifest = &a.installed[i]
			break
		}
	}
	if manifest == nil {
		ui.Text(c, "插件未安装或尚未加载："+ref).TextColor(c.Theme().Danger)
		return
	}
	if *values == nil {
		*values = map[string]string{}
	}
	if a.pluginEditors == nil {
		a.pluginEditors = map[string]*string{}
	}
	for _, f := range manifest.Fields {
		editorKey := key + "/" + ref + "/" + f.Key
		value := a.pluginEditors[editorKey]
		if value == nil {
			v, ok := (*values)[f.Key]
			if !ok {
				v = f.Default
			}
			value = &v
			a.pluginEditors[editorKey] = value
		}
		ui.Column(c.Key(f.Key)).Gap(6).Children(func() {
			label := f.Label
			if f.Required {
				label += "（必填）"
			}
			ui.Text(c, label).FontSize(12).TextColor(c.Theme().TextMuted)
			input := ui.TextInput(c, value).Label("插件设置：" + label).OnChange(func() { (*values)[f.Key] = *value })
			if f.Secret {
				input.Password()
			}
		})
	}
}

func (a *App) pluginsView(c *ui.Context) {
	t := c.Theme()
	ui.Text(c, "安装 .luminext 翻译或 OCR 插件。原生插件拥有当前用户权限，请仅启用可信来源。安装本身不会执行插件。").TextColor(t.TextMuted)
	ui.Row(c).Gap(8).Children(func() {
		ui.PrimaryButton(c, "安装插件包").Disabled(a.pluginBusy || a.saving).OnClick(a.installPlugin)
		ui.Button(c, "刷新插件").Disabled(a.pluginBusy).OnClick(a.RefreshPlugins)
	})
	ui.Scroll(c.Key("plugins-list")).Grow(1).Children(func() {
		ui.Column(c).Gap(16).Children(func() {
			if len(a.installed) == 0 {
				ui.Text(c, "暂无插件。开发协议与示例见 docs/plugins.md。").TextColor(t.TextMuted)
			}
			for _, m := range a.installed {
				supported := m.Entrypoints[runtime.GOOS+"/"+runtime.GOARCH] != ""
				ui.Column(c.Key(m.Ref())).Padding(16).Gap(10).Border(1, t.Border).Radius(12).Children(func() {
					ui.Text(c, m.Name).Bold().FontSize(18)
					ui.Text(c, m.Ref()+" · "+strings.Join(m.Capabilities, " / ")).FontSize(12).Selectable()
					if m.Description != "" {
						ui.Text(c, m.Description).TextColor(t.TextMuted)
					}
					if !supported {
						ui.Text(c, "此插件没有当前平台的可执行文件").TextColor(t.Danger)
					}
					ui.Row(c).Gap(8).Children(func() {
						if m.Has("translate") {
							ui.Button(c, "添加翻译服务").Disabled(!supported || a.saving || a.pluginBusy || len(a.draft.Providers) >= 16).OnClick(func() {
								a.draft.Providers = append(a.draft.Providers, config.Provider{ID: rand.Text(), Name: m.Name, Kind: "plugin", Plugin: m.Ref(), Settings: defaults(m)})
								a.page, a.servicePage = "服务设置", "翻译服务"
								a.status = "请填写插件设置，勾选启用后保存服务"
							})
						}
						if m.Has("recognize") {
							ui.Button(c, "用于 OCR").Disabled(!supported || a.saving || a.pluginBusy).OnClick(func() {
								a.selectOCR(m.Ref())
								a.page, a.servicePage = "服务设置", "OCR 服务"
								a.status = "请填写 OCR 插件设置并保存"
							})
						}
						ui.Spacer(c)
						ui.Button(c, "卸载").Disabled(a.pluginBusy || a.busy || a.recognition.busy || a.ocrProbe.Busy || a.probe.Busy || a.saving || a.pluginUsed(m.Ref())).OnClick(func() { a.confirmUninstall = m.Ref() })
					})
					if a.pluginUsed(m.Ref()) {
						ui.Text(c, "已被设置引用；移除引用并保存后可卸载。").FontSize(12).TextColor(t.TextMuted)
					}
					if a.confirmUninstall == m.Ref() {
						ui.Row(c).Gap(8).Children(func() {
							ui.Button(c, "保留插件").OnClick(func() { a.confirmUninstall = "" })
							ui.Button(c, "确认卸载").Disabled(a.pluginBusy || a.busy || a.recognition.busy || a.ocrProbe.Busy || a.probe.Busy || a.saving).OnClick(func() { a.uninstallPlugin(m.Ref()) })
						})
					}
				})
			}
		})
	})
}
