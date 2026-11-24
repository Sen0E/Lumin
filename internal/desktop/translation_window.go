package desktop

import (
	"github.com/egoist/mygo/ui"
	"lumin/internal/domain"
	"strings"
	"unicode/utf8"
)

// TranslationView focuses on source editing and reading service results.
func (a *App) TranslationView(c *ui.Context) {
	a.applyAppearance(c)
	t := c.Theme()
	c.OnShortcut(0, ui.KeyEscape, func() {
		if a.busy {
			a.Cancel()
		} else {
			a.DismissTranslation()
		}
	})
	ui.Column(c).Fill().Background(t.Background).Children(func() {
		ui.Column(c).Grow(1).MinHeight(0).Padding(8).Gap(6).Children(func() {
			ui.Row(c.Key("translation-toolbar")).Gap(8).AlignItems(ui.Center).Children(func() {
				if a.sourceCollapsed {
					ui.Text(c, a.source+" → "+a.target).FontSize(12).TextColor(t.TextMuted).Grow(1).MinWidth(0).MaxLines(1)
				} else {
					ui.Text(c, "翻译").FontSize(13).TextColor(t.TextMuted)
					ui.Spacer(c)
				}
				if a.busy {
					ui.Button(c, "取消").FontSize(12).Padding(4, 8).OnClick(a.Cancel)
				}
				label := "置顶"
				if a.pinned {
					label = "取消置顶"
				}
				pin := ui.Button(c, label).FontSize(12).Padding(4, 8).OnClick(a.togglePin)
				if a.pinned {
					pin.Background(t.Accent).TextColor(t.AccentText)
				}
				ui.Button(c, "关闭").FontSize(12).Padding(4, 8).OnClick(a.DismissTranslation)
			})
			a.translationView(c)
		})
		ui.Text(c.Key("translation-status"), a.status).Role(ui.RoleStatus).
			FontSize(11).TextColor(t.TextMuted).Padding(3, 10).MaxLines(1).MaxHeight(24)
	})
}

func (a *App) translationView(c *ui.Context) {
	t := c.Theme()
	c.OnShortcut(ui.Cmd, ui.KeyEnter, a.Translate)

	if a.sourceCollapsed {
		ui.Row(c.Key("source-preview")).Padding(8).Background(t.Surface).Radius(8).Gap(6).AlignItems(ui.Center).Children(func() {
			preview := strings.Join(strings.Fields(a.text), " ")
			ui.Text(c, preview).FontSize(13).TextColor(t.TextMuted).Grow(1).MinWidth(0).MaxLines(2)
			ui.Button(c, "展开原文").FontSize(12).Padding(4, 8).OnClick(func() {
				a.sourceCollapsed = false
				a.focusInput = true
			})
		})
	} else {
		ui.Row(c.Key("languages")).Gap(8).AlignItems(ui.Center).Children(func() {
			ui.Select(c.Key("source"), &a.source, languageNames(false)).Label("源语言").Grow(1).MinWidth(0).OnChange(a.sourceChanged)
			ui.Button(c, "交换").FontSize(12).Padding(4, 8).Disabled(languageCode(a.source) == "auto").OnClick(func() { a.source, a.target = a.target, a.source; a.sourceChanged() })
			ui.Select(c.Key("target"), &a.target, languageNames(true)).Label("目标语言").Grow(1).MinWidth(0).OnChange(a.sourceChanged)
		})
		ui.Row(c.Key("source-heading")).Gap(8).AlignItems(ui.Center).Children(func() {
			ui.Textf(c, "原文 · %d 字", utf8.RuneCountInString(a.text)).FontSize(12).TextColor(t.TextMuted)
			ui.Spacer(c)
			ui.Button(c, "收起原文").FontSize(12).Padding(4, 8).OnClick(func() { a.sourceCollapsed = true; a.focusInput = false; a.input.CancelFocus(c) })
		})
		ui.TextArea(c.Key("source-text"), &a.text).Bind(&a.input).Label("待翻译文本").Placeholder("输入或粘贴文本，按 " + shortcutDisplay("CmdOrCtrl+Enter") + " 翻译…").
			Height(76).Shrink(0).MaxLength(domain.MaxTextRunes).OnChange(a.sourceChanged)
		if a.focusInput {
			a.input.Focus(c)
			a.focusInput = false
		}
		ui.Row(c.Key("actions")).Gap(6).AlignItems(ui.Center).Children(func() {
			ui.Button(c, "粘贴").FontSize(12).Padding(5, 8).OnClick(func() { a.Paste() })
			ui.Button(c, "复制原文").FontSize(12).Padding(5, 8).Disabled(strings.TrimSpace(a.text) == "").OnClick(func() { a.copy(a.text) })
			ui.Button(c, "清空").FontSize(12).Padding(5, 8).OnClick(func() { a.sourceChanged(); a.text = "" })
			ui.Spacer(c)
			ui.PrimaryButton(c, "翻译文本").FontSize(12).Padding(5, 10).Disabled(strings.TrimSpace(a.text) == "").OnClick(a.Translate)
		})
	}

	ui.Scroll(c.Key("results")).Grow(1).MinHeight(0).Children(func() {
		ui.Column(c).Gap(8).Children(func() {
			if len(a.results) == 0 {
				configured := false
				for _, provider := range a.settings.Providers {
					configured = configured || provider.Enabled
				}
				ui.Column(c.Key("translation-empty")).Padding(16).Gap(12).Children(func() {
					switch {
					case a.busy:
						ui.Text(c, "正在准备翻译…").TextColor(t.TextMuted)
					case configured:
						ui.Text(c, "译文将在这里显示").FontSize(18).Bold()
						ui.Text(c, "选中文字后按取词快捷键，或输入原文后点击翻译。每个已启用服务会显示独立结果。").TextColor(t.TextMuted)
					default:
						ui.Text(c, "先添加一个翻译服务").FontSize(18).Bold()
						ui.Text(c, "配置后即可翻译选中文字、截图或手动输入的内容。").TextColor(t.TextMuted)
						ui.Button(c, "配置翻译服务").OnClick(func() { a.openServices("翻译服务") })
					}
				})
			}
			for _, result := range a.results {
				ui.Column(c.Key(result.ProviderID)).Padding(10).Gap(6).Background(t.Surface).Radius(8).Children(func() {
					switch {
					case result.Error != "":
						ui.Text(c, result.Error).FontSize(13).TextColor(t.Danger).Selectable()
					case result.Text != "":
						ui.Text(c, result.Text).FontSize(float32(a.settings.ResultFontSize())).Selectable()
					case a.busy:
						ui.Text(c, "正在等待服务响应…").FontSize(13).TextColor(t.TextMuted)
					default:
						ui.Text(c, "已取消").FontSize(13).TextColor(t.TextMuted)
					}
					ui.Row(c).Gap(8).AlignItems(ui.Center).Children(func() {
						ui.Text(c, result.ProviderName).FontSize(11).TextColor(t.TextMuted).Grow(1).MinWidth(0).MaxLines(1)
						if result.Text != "" || result.Error != "" {
							ui.Textf(c, "%.1f s", float64(result.DurationMS)/1000).FontSize(11).TextColor(t.TextMuted)
						}
						ui.Button(c, "复制译文").FontSize(12).Padding(4, 8).Disabled(result.Text == "").OnClick(func() { a.copy(result.Text) })
					})
				})
			}
		})
	})
}

func (a *App) togglePin() {
	a.pinned = !a.pinned
	if a.ports.SetTranslationPinned != nil {
		a.ports.SetTranslationPinned(a.pinned)
	}
}
