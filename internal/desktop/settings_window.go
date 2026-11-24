package desktop

import (
	"crypto/rand"
	"github.com/egoist/mygo/ui"
)

func (a *App) settingsView(c *ui.Context) {
	t := c.Theme()
	ui.Scroll(c.Key("settings-" + a.page)).Grow(1).Disabled(a.saving).Children(func() {
		ui.Column(c).Gap(16).Children(func() {
			switch a.page {
			case "常规设置":
				ui.Text(c, "后台运行").FontSize(18).Bold()
				ui.Text(c, "关闭窗口会隐藏到后台，快捷键和外部调用继续可用。通过托盘或重新运行 Lumin 打开设置；点击左下角“退出 Lumin”结束程序。").TextColor(t.TextMuted)
				ui.Row(c).Gap(10).AlignItems(ui.Center).Children(func() {
					ui.Text(c.Key("tray-status"), a.trayStatus).Grow(1).MinWidth(0).FontSize(12).TextColor(t.TextMuted).Selectable()
					ui.Button(c, "检查托盘").Disabled(a.trayChecking || a.ports.CheckTray == nil).OnClick(a.CheckTray)
				})
				a.appearanceView(c)
				ui.Text(c, "外部调用").FontSize(18).Bold()
				ui.Checkbox(c, &a.draft.ServerEnabled, "启用本地 HTTP 服务（保存后重启生效）")
				field(c, "回环监听地址", &a.draft.ServerAddress, false)
				field(c, "外部调用令牌", &a.draft.ServerToken, true)
				ui.Button(c, "生成新令牌").OnClick(func() { a.draft.ServerToken = rand.Text() })
				ui.Text(c, "POST /translate 接收纯文本，使用 Authorization: Bearer <令牌>。详情见 docs/development.md。").FontSize(12).TextColor(t.TextMuted)
			case "翻译设置":
				a.preferencesView(c)
				a.translationPreferencesView(c)
				ui.Text(c, "数据与隐私").FontSize(18).Bold()
				ui.Checkbox(c, &a.draft.SaveHistory, "保存翻译历史")
				ui.Text(c, "配置与历史保存在："+a.store.Dir()).FontSize(12).Selectable().TextColor(t.TextMuted)
				ui.Text(c, "密钥可通过环境变量提供；直接填写的密钥以明文保存在本机配置文件中。选中文字翻译不会读取或修改普通剪贴板。").FontSize(12).TextColor(t.TextMuted)

			case "热键设置":
				shortcutField(c, "打开翻译窗口快捷键", &a.preferences.InputShortcut, &a.draft.InputShortcut)
				shortcutField(c, "选中文字翻译快捷键", &a.preferences.SelectionShortcut, &a.draft.SelectionShortcut)
				shortcutField(c, "截图翻译快捷键", &a.preferences.ScreenshotShortcut, &a.draft.ScreenshotShortcut)
				ui.Text(c, "先在原应用中选中文字，再按取词快捷键；无需复制。取词失败时可使用截图翻译。").FontSize(12).TextColor(t.TextMuted)
				ui.Text(c, "留空可禁用快捷键。Wayland 由桌面门户授权；窗口内 "+shortcutDisplay("CmdOrCtrl+Enter")+" 翻译，Esc 取消当前任务，再按一次隐藏翻译窗口。").FontSize(12).TextColor(t.TextMuted)

			}
		})
	})
	a.settingsFooter(c)
}

func (a *App) settingsFooter(c *ui.Context) {
	ui.Row(c).Gap(8).Children(func() {
		ui.Button(c, "放弃未保存修改").Disabled(a.saving).OnClick(a.discardSettings)
		ui.Spacer(c)
		ui.PrimaryButton(c, "保存设置").Disabled(a.saving).OnClick(a.saveSettings)
	})
}

func (a *App) servicesView(c *ui.Context) {
	ui.Row(c).Gap(8).Children(func() {
		for _, page := range []string{"翻译服务", "OCR 服务", "插件"} {
			b := ui.Button(c.Key(page), page).OnClick(func() { a.servicePage = page })
			if a.servicePage == page {
				b.Background(c.Theme().Accent).TextColor(c.Theme().AccentText)
			}
		}
	})
	switch a.servicePage {
	case "插件":
		a.pluginsView(c)
	case "OCR 服务":
		a.ocrServicesView(c)
		a.settingsFooter(c)
	default:
		a.providersView(c)
	}
}

func (a *App) openServices(page string) {
	a.servicePage = page
	a.openPreferences("服务设置")
}
