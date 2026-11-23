package desktop

import (
	"github.com/egoist/mygo/ui"
	"lumin/internal/domain"
	"strings"
	"unicode/utf8"
)

// TranslationView has its own navigation so translation requests never replace
// an in-progress settings edit in the preferences window.
func (a *App) TranslationView(c *ui.Context) {
	t := c.Theme()
	c.OnShortcut(0, ui.KeyEscape, func() {
		if a.busy {
			a.Cancel()
		} else {
			a.DismissTranslation()
		}
	})
	ui.Column(c).Fill().Background(t.Background).Children(func() {
		ui.Column(c).Grow(1).MinHeight(0).Padding(14).Gap(10).Children(func() {
			ui.Row(c).Gap(6).AlignItems(ui.Center).Children(func() {
				for _, page := range translationPages {
					b := ui.Button(c.Key(page), page).OnClick(func() {
						a.translationPage = page
						a.confirmClear = false
					})
					if a.translationPage == page {
						b.Background(t.Accent).TextColor(t.AccentText)
					}
				}
				ui.Spacer(c)
				ui.Button(c, "设置").OnClick(a.ShowSettings)
			})
			switch a.translationPage {
			case "历史记录", "生词本":
				a.historyView(c)
			default:
				a.translationView(c)
			}
		})
		ui.Text(c.Key("translation-status"), a.status).Role(ui.RoleStatus).
			FontSize(12).TextColor(t.TextMuted).Padding(8, 14).Background(t.Surface).MaxHeight(52)
	})
}

func (a *App) translationView(c *ui.Context) {
	t := c.Theme()
	c.OnShortcut(ui.Cmd, ui.KeyEnter, a.Translate)

	ui.Row(c.Key("languages")).Gap(10).AlignItems(ui.Center).Children(func() {
		ui.Select(c.Key("source"), &a.source, languageNames(false)).Label("源语言").Grow(1).MinWidth(0).OnChange(a.sourceChanged)
		ui.Button(c, "交换").Disabled(languageCode(a.source) == "auto").OnClick(func() { a.source, a.target = a.target, a.source; a.sourceChanged() })
		ui.Select(c.Key("target"), &a.target, languageNames(true)).Label("目标语言").Grow(1).MinWidth(0).OnChange(a.sourceChanged)
	})
	ui.TextArea(c.Key("source-text"), &a.text).Bind(&a.input).Label("待翻译文本").Placeholder("输入或粘贴文本，按 Ctrl / ⌘ + Enter 翻译…").
		Height(110).Shrink(0).MaxLength(domain.MaxTextRunes).OnChange(a.sourceChanged)
	if a.focusInput {
		a.input.Focus(c)
		a.focusInput = false
	}
	ui.Row(c.Key("actions")).Gap(8).AlignItems(ui.Center).Children(func() {
		ui.Button(c, "粘贴").OnClick(func() { a.Paste(false) })
		ui.Button(c, "图片识字").Disabled(a.busy).OnClick(a.recognizeImage)
		ui.Button(c, "清空").OnClick(func() { a.sourceChanged(); a.text = "" })
		ui.Spacer(c)
		if a.busy {
			ui.Button(c, "取消").OnClick(a.Cancel)
		}
		ui.PrimaryButton(c, "翻译文本").Disabled(strings.TrimSpace(a.text) == "").OnClick(a.Translate)
	})
	ui.Row(c.Key("capture-actions")).Gap(8).AlignItems(ui.Center).Children(func() {
		ui.Button(c, "截图识字").Disabled(a.busy).OnClick(func() { a.Screenshot(false) })
		ui.Button(c, "截图翻译").Disabled(a.busy).OnClick(func() { a.Screenshot(true) })
		ui.Spacer(c)
		ui.Textf(c, "%d 字", utf8.RuneCountInString(a.text)).FontSize(12).TextColor(t.TextMuted)
	})
	ui.Scroll(c.Key("results")).Grow(1).Children(func() {
		ui.Column(c).Gap(12).Children(func() {
			if len(a.results) == 0 {
				ui.Column(c).Padding(16).Gap(12).Border(1, t.Border).Radius(12).Children(func() {
					ui.Text(c, "在这里读懂另一种语言").FontSize(20).Bold()
					ui.Text(c, "配置翻译服务后，可同时比较多个服务的译文。").TextColor(t.TextMuted)
					ui.Button(c, "配置翻译服务").OnClick(func() { a.openPreferences("翻译服务") })
				})
			}
			for _, result := range a.results {
				ui.Column(c.Key(result.ProviderID)).Padding(18).Gap(12).Border(1, t.Border).Radius(12).Children(func() {
					ui.Row(c).Gap(8).AlignItems(ui.Center).Children(func() {
						ui.Text(c, result.ProviderName).Bold()
						ui.Spacer(c)
						if result.Text != "" || result.Error != "" {
							ui.Textf(c, "%.1f s", float64(result.DurationMS)/1000).FontSize(12).TextColor(t.TextMuted)
						}
						ui.Button(c, "复制译文").Disabled(result.Text == "").OnClick(func() { a.copy(result.Text) })
					})
					switch {
					case result.Error != "":
						ui.Text(c, result.Error).TextColor(t.Danger).Selectable()
					case result.Text != "":
						ui.Text(c, result.Text).FontSize(16).Selectable()
					case a.busy:
						ui.Text(c, "正在等待服务响应…").TextColor(t.TextMuted)
					default:
						ui.Text(c, "已取消").TextColor(t.TextMuted)
					}
				})
			}
		})
	})
}

func (a *App) historyView(c *ui.Context) {
	t := c.Theme()
	favorites := a.translationPage == "生词本"
	ui.Row(c).Gap(8).Children(func() {
		ui.TextInput(c.Key("history-search"), &a.search).Placeholder("搜索原文或译文").Label("搜索记录").Grow(1)
		if !favorites {
			ui.Button(c, "清除历史").Disabled(a.historyBusy || a.busy).OnClick(func() { a.confirmClear = true })
		}
	})
	if a.confirmClear && !favorites {
		ui.Row(c.Key("clear-confirm")).Gap(8).AlignItems(ui.Center).Children(func() {
			ui.Text(c, "清除普通历史，保留生词本？").Grow(1)
			ui.Button(c, "返回").OnClick(func() { a.confirmClear = false })
			ui.Button(c, "确认清除").Disabled(a.historyBusy).OnClick(func() { a.confirmClear = false; a.changeHistory(a.store.Clear) })
		})
	}
	ui.Scroll(c.Key("history-list")).Grow(1).Children(func() {
		ui.Column(c).Gap(12).Children(func() {
			count := 0
			for _, entry := range a.history {
				if favorites && !entry.Favorite {
					continue
				}
				searchable := entry.Request.Text
				for _, result := range entry.Results {
					searchable += "\n" + result.Text
				}
				if !strings.Contains(strings.ToLower(searchable), strings.ToLower(a.search)) {
					continue
				}
				count++
				ui.Column(c.Key(entry.ID)).Padding(16).Gap(10).Border(1, t.Border).Radius(10).Children(func() {
					ui.Row(c).AlignItems(ui.Center).Gap(8).Children(func() {
						ui.Text(c, entry.CreatedAt.Local().Format("2006-01-02 15:04")+" · "+domain.LanguageName(entry.Request.Target)).FontSize(12).TextColor(t.TextMuted)
						ui.Spacer(c)
						label := "收藏"
						if entry.Favorite {
							label = "取消收藏"
						}
						ui.Button(c, label).Disabled(a.historyBusy).OnClick(func() { a.changeHistory(func() error { return a.store.Favorite(entry.ID, !entry.Favorite) }) })
						ui.Button(c, "载入").OnClick(func() {
							a.sourceChanged()
							a.text = entry.Request.Text
							a.source = domain.LanguageName(entry.Request.Source)
							a.target = domain.LanguageName(entry.Request.Target)
							a.results = append([]domain.Result(nil), entry.Results...)
							a.translationPage = "翻译"
						})
					})
					ui.Text(c, preview(entry.Request.Text, 180)).Bold().Selectable()
					for _, result := range entry.Results {
						if result.Text != "" {
							ui.Text(c, result.ProviderName+" · "+preview(result.Text, 240)).TextColor(t.TextMuted).Selectable()
						}
					}
				})
			}
			if count == 0 {
				ui.Text(c, "暂无记录。完成翻译后，历史记录会出现在这里；收藏的记录也会显示在生词本。").TextColor(t.TextMuted).Padding(24)
			}
		})
	})
}
