package desktop

import (
	"crypto/rand"

	"github.com/egoist/mygo/ui"

	"lumin/internal/config"
	"lumin/internal/domain"
)

var pages = []string{"常规设置", "翻译设置", "热键设置", "服务设置", "历史记录", "备份设置", "关于应用"}

func (a *App) View(c *ui.Context) {
	a.applyAppearance(c)
	t := c.Theme()
	ui.Column(c).Fill().Background(t.Background).Children(func() {
		ui.Row(c).Grow(1).MinHeight(0).Disabled(a.backup.restoring).AlignItems(ui.Stretch).Children(func() {
			ui.Column(c.Key("sidebar")).Width(180).Shrink(0).Padding(20, 14).Gap(8).Background(t.Surface).Children(func() {
				ui.Text(c, "Lumin").FontSize(28).Bold().Margin(0, 8, 0, 8)
				ui.Text(c, "偏好设置").FontSize(12).TextColor(t.TextMuted).Margin(0, 8, 28, 8)
				for _, page := range pages {
					b := ui.Button(c.Key(page), page).Height(42).OnClick(func() { a.page = page; a.confirmClear = false; a.confirmDelete = "" })
					if a.page == page {
						b.Background(t.Accent).TextColor(t.AccentText)
					}
				}
				ui.Spacer(c)
				ui.Button(c, "退出 Lumin").OnClick(func() {
					if a.ports.Quit != nil {
						a.ports.Quit()
					}
				})
				ui.Text(c, "关闭窗口后继续后台运行").FontSize(11).TextColor(t.TextMuted)
				ui.Text(c, "v0.13.3 · Go + MyGo").FontSize(11).TextColor(t.TextMuted)
			})
			ui.Column(c.Key("workspace")).Grow(1).MinWidth(0).MinHeight(0).Padding(28).Gap(18).Children(func() {
				ui.Text(c, a.page).FontSize(26).Bold()
				switch a.page {
				case "服务设置":
					a.servicesView(c)
				case "备份设置":
					a.backupView(c)
				case "历史记录":
					a.historyView(c)
				case "关于应用":
					ui.Text(c, "Lumin 0.13.3").FontSize(20).Bold()
					ui.Text(c, "Go + MyGo 原生桌面翻译工具").TextColor(t.TextMuted)
					ui.Text(c, "支持离线文字识别、可配置翻译服务与插件扩展。").TextColor(t.TextMuted)
				default:
					a.settingsView(c)
				}
			})
		})
		ui.Text(c.Key("status"), a.status).Role(ui.RoleStatus).FontSize(12).TextColor(t.TextMuted).
			Padding(10, 18).Background(t.Surface).MaxHeight(60)
	})
}

func languageNames(target bool) []string {
	var names []string
	for _, l := range domain.Languages {
		if !target || l.Code != "auto" {
			names = append(names, l.Name)
		}
	}
	return names
}

func preview(s string, n int) string {
	r := []rune(s)
	if len(r) > n {
		return string(r[:n]) + "…"
	}
	return s
}

func field(c *ui.Context, label string, value *string, secret bool) {
	ui.Column(c.Key(label)).Gap(6).Children(func() {
		ui.Text(c, label).FontSize(12).TextColor(c.Theme().TextMuted)
		input := ui.TextInput(c, value).Label(label)
		if secret {
			input.Password()
		}
	})
}

func (a *App) providersView(c *ui.Context) {
	t := c.Theme()
	c.OnShortcut(0, ui.KeyEscape, a.cancelProbe)
	ui.Text(c, "按列表顺序显示译文。测试使用当前草稿，将 “Hello, world!” 译成简体中文，可能产生接口费用。").TextColor(t.TextMuted)
	ui.Scroll(c.Key("provider-list")).Grow(1).Disabled(a.saving).Children(func() {
		ui.Column(c).Gap(16).Children(func() {
			for i := range a.draft.Providers {
				p := &a.draft.Providers[i]
				ui.Column(c.Key(p.ID)).Padding(18).Gap(12).Border(1, t.Border).Radius(12).Children(func() {
					ui.Row(c).Gap(10).Children(func() {
						ui.Checkbox(c, &p.Enabled, p.Name).Grow(1)
						ui.Text(c, p.Kind).TextColor(t.TextMuted).FontSize(12)
						ui.Button(c, "上移").Label("上移服务：" + p.ID).Disabled(i == 0).OnClick(func() { a.moveProvider(p.ID, -1) })
						ui.Button(c, "下移").Label("下移服务：" + p.ID).Disabled(i == len(a.draft.Providers)-1).OnClick(func() { a.moveProvider(p.ID, 1) })
						ui.Button(c, "移除").OnClick(func() { a.removeProvider(p.ID) })
					})
					ui.Row(c.Key("probe-actions")).Gap(8).AlignItems(ui.Center).Children(func() {
						ui.Button(c, "测试此服务").Label("测试服务：" + p.ID).Disabled(a.pluginBusy || a.probe.Busy && a.probe.Provider.ID == p.ID).OnClick(func() { a.testProvider(p.ID) })
						if a.probe.Provider.ID == p.ID && a.probe.Busy {
							ui.Button(c, "取消测试").OnClick(a.cancelProbe)
						}
					})
					if a.probe.Provider.ID == p.ID {
						message := a.probe.Message
						if !a.probeMatchesDraft() {
							message = "配置已修改，请重新测试"
						}
						text := ui.Text(c.Key("probe-message"), message).Selectable().FontSize(12)
						if a.probe.Failed && a.probeMatchesDraft() {
							text.TextColor(t.Danger)
						} else {
							text.TextColor(t.TextMuted)
						}
					}
					field(c, "服务名称", &p.Name, false)
					if p.Kind == "plugin" {
						ui.Text(c, p.Plugin).FontSize(12).Selectable()
						a.pluginFields(c, p.ID, p.Plugin, &p.Settings)
					} else {
						field(c, "完整接口地址", &p.Endpoint, false)
						if p.Kind == "compatible" || p.Kind == "ollama" {
							field(c, "模型名称", &p.Model, false)
						}
						field(c, "API Key（可选，本地保存）", &p.APIKey, true)
						field(c, "密钥环境变量（设置后优先使用）", &p.APIKeyEnv, false)
					}
				})
			}
		})
	})
	ui.Row(c).Gap(8).AlignItems(ui.Center).Children(func() {
		ui.Select(c, &a.selectedProvider, []string{"compatible", "ollama", "deepl", "libretranslate"}).Label("新增服务类型").Width(190).Disabled(a.saving)
		ui.Button(c, "添加服务").Disabled(a.saving || a.selectedProvider == "" || len(a.draft.Providers) >= 16).OnClick(func() {
			for _, p := range config.Default().Providers {
				if p.Kind == a.selectedProvider {
					p.ID = rand.Text()
					a.draft.Providers = append(a.draft.Providers, p)
					break
				}
			}
		})
		ui.Spacer(c)
		ui.PrimaryButton(c, "保存服务").Disabled(a.saving).OnClick(a.saveSettings)
	})
}
