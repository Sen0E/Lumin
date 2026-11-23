package desktop

import (
	"crypto/rand"

	"github.com/egoist/mygo/ui"

	"lumin/internal/config"
	"lumin/internal/domain"
)

var pages = []string{"翻译服务", "插件", "设置"}
var translationPages = []string{"翻译", "历史记录", "生词本"}

func (a *App) View(c *ui.Context) {
	t := c.Theme()
	ui.Column(c).Fill().Background(t.Background).Children(func() {
		ui.Row(c).Grow(1).MinHeight(0).AlignItems(ui.Stretch).Children(func() {
			ui.Column(c.Key("sidebar")).Width(180).Shrink(0).Padding(20, 14).Gap(8).Background(t.Surface).Children(func() {
				ui.Text(c, "Lumin").FontSize(28).Bold().Margin(0, 8, 0, 8)
				ui.Text(c, "偏好设置").FontSize(12).TextColor(t.TextMuted).Margin(0, 8, 28, 8)
				for _, page := range pages {
					b := ui.Button(c.Key(page), page).Height(42).OnClick(func() { a.page = page; a.confirmClear = false })
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
				ui.Text(c, "v0.4.0 · Go + MyGo").FontSize(11).TextColor(t.TextMuted)
			})
			ui.Column(c.Key("workspace")).Grow(1).MinWidth(0).MinHeight(0).Padding(28).Gap(18).Children(func() {
				ui.Text(c, a.page).FontSize(26).Bold()
				switch a.page {
				case "翻译服务":
					a.providersView(c)
				case "插件":
					a.pluginsView(c)
				case "设置":
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

func (a *App) settingsView(c *ui.Context) {
	t := c.Theme()
	ui.Scroll(c.Key("settings")).Grow(1).Disabled(a.saving).Children(func() {
		ui.Column(c).Gap(16).Children(func() {
			ui.Text(c, "后台运行").FontSize(18).Bold()
			ui.Text(c, "关闭窗口会隐藏到后台，快捷键和外部调用继续可用。通过托盘或重新运行 Lumin 打开设置；点击左下角“退出 Lumin”结束程序。").TextColor(t.TextMuted)
			ui.Checkbox(c, &a.watchClipboard, "监听剪贴板（立即生效，本次运行有效）")
			a.preferencesView(c)
			ui.Text(c, "数据与隐私").FontSize(18).Bold()
			ui.Checkbox(c, &a.draft.SaveHistory, "保存翻译历史")
			ui.Text(c, "配置与历史保存在："+a.store.Dir()).FontSize(12).Selectable().TextColor(t.TextMuted)
			ui.Text(c, "密钥可通过环境变量提供；直接填写的密钥以明文保存在本机配置文件中。剪贴板监听每次启动默认关闭。").FontSize(12).TextColor(t.TextMuted)
			field(c, "打开翻译窗口快捷键", &a.draft.InputShortcut, false)
			field(c, "翻译剪贴板快捷键", &a.draft.ClipboardShortcut, false)
			field(c, "截图翻译快捷键", &a.draft.ScreenshotShortcut, false)
			ui.Text(c, "留空可禁用快捷键。Wayland 由桌面门户授权；窗口内 Ctrl / ⌘ + Enter 翻译，Esc 取消当前任务，再按一次隐藏翻译窗口。").FontSize(12).TextColor(t.TextMuted)
			ui.Text(c, "OCR 引擎").FontSize(18).Bold()
			if a.draft.OCRPlugin != "" {
				ui.Text(c, a.draft.OCRPlugin).Selectable()
				a.pluginFields(c, "ocr", a.draft.OCRPlugin, &a.draft.OCRSettings)
				ui.Button(c, "恢复内置 Tesseract").OnClick(func() { a.draft.OCRPlugin = ""; a.draft.OCRSettings = nil; a.pluginEditors = nil })
			} else {
				ui.Text(c, "内置 Tesseract（可在插件页面更换）")
			}
			field(c, "OCR 语言（Tesseract）", &a.draft.OCRLanguages, false)
			ui.Text(c, "图片识字需安装 Tesseract 和语言包。默认 eng+chi_sim 表示英文与简体中文。").FontSize(12).TextColor(t.TextMuted)
			ui.Text(c, "外部调用").FontSize(18).Bold()
			ui.Checkbox(c, &a.draft.ServerEnabled, "启用本地 HTTP 服务（保存后重启生效）")
			field(c, "回环监听地址", &a.draft.ServerAddress, false)
			field(c, "外部调用令牌", &a.draft.ServerToken, true)
			ui.Button(c, "生成新令牌").OnClick(func() { a.draft.ServerToken = rand.Text() })
			ui.Text(c, "POST /translate 接收纯文本，使用 Authorization: Bearer <令牌>。详情见 docs/development.md。").FontSize(12).TextColor(t.TextMuted)
		})
	})
	ui.Row(c).Gap(8).Children(func() {
		ui.Button(c, "放弃未保存修改").Disabled(a.saving).OnClick(a.discardSettings)
		ui.Spacer(c)
		ui.PrimaryButton(c, "保存设置").Disabled(a.saving).OnClick(a.saveSettings)
	})
}
