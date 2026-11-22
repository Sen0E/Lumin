package desktop

import (
	"crypto/rand"
	"fmt"
	"strings"
	"unicode/utf8"

	"github.com/egoist/mygo/ui"

	"lumin/internal/config"
	"lumin/internal/domain"
)

var pages = []string{"翻译", "历史记录", "生词本", "翻译服务", "设置"}

func (a *App) View(c *ui.Context) {
	t := c.Theme()
	ui.Column(c).Fill().Background(t.Background).Children(func() {
		ui.Row(c).Grow(1).MinHeight(0).AlignItems(ui.Stretch).Children(func() {
			ui.Column(c.Key("sidebar")).Width(180).Shrink(0).Padding(20, 14).Gap(8).Background(t.Surface).Children(func() {
				ui.Text(c, "Lumin").FontSize(28).Bold().Margin(0, 8, 0, 8)
				ui.Text(c, "让语言更近一步").FontSize(12).TextColor(t.TextMuted).Margin(0, 8, 28, 8)
				for _, page := range pages {
					b := ui.Button(c.Key(page), page).Height(42).OnClick(func() { a.page = page; a.confirmClear = false })
					if a.page == page {
						b.Background(t.Accent).TextColor(t.AccentText)
					}
				}
				ui.Spacer(c)
				ui.Text(c, "原生桌面翻译").FontSize(12).TextColor(t.TextMuted)
				ui.Text(c, "v0.1.0 · Go + MyGo").FontSize(11).TextColor(t.TextMuted)
			})
			ui.Column(c.Key("workspace")).Grow(1).MinWidth(0).MinHeight(0).Padding(28).Gap(18).Children(func() {
				ui.Text(c, a.page).FontSize(26).Bold()
				switch a.page {
				case "翻译":
					a.translationView(c)
				case "历史记录", "生词本":
					a.historyView(c)
				case "翻译服务":
					a.providersView(c)
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

func (a *App) translationView(c *ui.Context) {
	t := c.Theme()
	c.OnShortcut(ui.Cmd, ui.KeyEnter, a.Translate)
	c.OnShortcut(0, ui.KeyEscape, a.Cancel)
	ui.Row(c.Key("languages")).Gap(10).AlignItems(ui.Center).Children(func() {
		ui.Select(c.Key("source"), &a.source, languageNames(false)).Label("源语言").Width(170).OnChange(a.sourceChanged)
		ui.Button(c, "交换").Disabled(languageCode(a.source) == "auto").OnClick(func() { a.source, a.target = a.target, a.source; a.sourceChanged() })
		ui.Select(c.Key("target"), &a.target, languageNames(true)).Label("目标语言").Width(170).OnChange(a.sourceChanged)
		ui.Spacer(c)
		ui.Textf(c, "%d 字", utf8.RuneCountInString(a.text)).FontSize(12).TextColor(t.TextMuted)
	})
	ui.TextArea(c.Key("source-text"), &a.text).Label("待翻译文本").Placeholder("输入或粘贴文本，按 Ctrl / ⌘ + Enter 翻译…").
		Height(170).MaxLength(domain.MaxTextRunes).OnChange(a.sourceChanged)
	ui.Row(c.Key("actions")).Gap(8).AlignItems(ui.Center).Children(func() {
		ui.Button(c, "粘贴").OnClick(func() { a.Paste(false) })
		ui.Button(c, "图片识字").Disabled(a.busy).OnClick(a.recognizeImage)
		ui.Button(c, "清空").OnClick(func() { a.sourceChanged(); a.text = "" })
		ui.Spacer(c)
		ui.Checkbox(c, &a.watchClipboard, "监听剪贴板")
		if a.busy {
			ui.Button(c, "取消").OnClick(a.Cancel)
		}
		ui.PrimaryButton(c, "翻译文本").Disabled(strings.TrimSpace(a.text) == "").OnClick(a.Translate)
	})
	ui.Scroll(c.Key("results")).Grow(1).Children(func() {
		ui.Column(c).Gap(12).Children(func() {
			if len(a.results) == 0 {
				ui.Column(c).Padding(30).Gap(12).Border(1, t.Border).Radius(12).Children(func() {
					ui.Text(c, "在这里读懂另一种语言").FontSize(20).Bold()
					ui.Text(c, "配置翻译服务后，可同时比较多个服务的译文。").TextColor(t.TextMuted)
					ui.Text(c, "本地模型可通过 Ollama 使用；图片识字使用本机 Tesseract。").TextColor(t.TextMuted)
					ui.Button(c, "配置翻译服务").OnClick(func() { a.page = "翻译服务" })
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
	favorites := a.page == "生词本"
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
							a.page = "翻译"
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
	ui.Text(c, "启用的服务会并行翻译。填写完整接口地址，修改后点击保存。").TextColor(t.TextMuted)
	ui.Scroll(c.Key("provider-list")).Grow(1).Disabled(a.saving).Children(func() {
		ui.Column(c).Gap(16).Children(func() {
			for i := range a.draft.Providers {
				p := &a.draft.Providers[i]
				ui.Column(c.Key(p.ID)).Padding(18).Gap(12).Border(1, t.Border).Radius(12).Children(func() {
					ui.Row(c).Gap(10).Children(func() {
						ui.Checkbox(c, &p.Enabled, p.Name).Grow(1)
						ui.Text(c, p.Kind).TextColor(t.TextMuted).FontSize(12)
						id := p.ID
						ui.Button(c, "移除").OnClick(func() {
							for j, item := range a.draft.Providers {
								if item.ID == id {
									a.draft.Providers = append(a.draft.Providers[:j], a.draft.Providers[j+1:]...)
									break
								}
							}
						})
					})
					field(c, "服务名称", &p.Name, false)
					field(c, "完整接口地址", &p.Endpoint, false)
					if p.Kind == "compatible" || p.Kind == "ollama" {
						field(c, "模型名称", &p.Model, false)
					}
					field(c, "API Key（可选，本地保存）", &p.APIKey, true)
					field(c, "密钥环境变量（设置后优先使用）", &p.APIKeyEnv, false)
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
			ui.Text(c, "数据与隐私").FontSize(18).Bold()
			ui.Checkbox(c, &a.draft.SaveHistory, "保存翻译历史")
			ui.Text(c, "配置与历史保存在："+a.store.Dir()).FontSize(12).Selectable().TextColor(t.TextMuted)
			ui.Text(c, "密钥可通过环境变量提供；直接填写的密钥以明文保存在本机配置文件中。剪贴板监听每次启动默认关闭。").FontSize(12).TextColor(t.TextMuted)
			field(c, "打开翻译窗口快捷键", &a.draft.InputShortcut, false)
			field(c, "翻译剪贴板快捷键", &a.draft.ClipboardShortcut, false)
			ui.Text(c, "留空可禁用快捷键。Wayland 由桌面门户授权；窗口内 Ctrl / ⌘ + Enter 翻译，Esc 取消。").FontSize(12).TextColor(t.TextMuted)
			field(c, "OCR 语言（Tesseract）", &a.draft.OCRLanguages, false)
			ui.Text(c, "图片识字需安装 Tesseract 和语言包。默认 eng+chi_sim 表示英文与简体中文。").FontSize(12).TextColor(t.TextMuted)
			ui.Text(c, "外部调用").FontSize(18).Bold()
			ui.Checkbox(c, &a.draft.ServerEnabled, "启用本地 HTTP 服务（保存后重启生效）")
			field(c, "回环监听地址", &a.draft.ServerAddress, false)
			field(c, "外部调用令牌", &a.draft.ServerToken, true)
			ui.Button(c, "生成新令牌").OnClick(func() { a.draft.ServerToken = rand.Text() })
			ui.Text(c, "POST /translate 接收纯文本，使用 Authorization: Bearer <令牌>。详情见 docs/development.md。").FontSize(12).TextColor(t.TextMuted)
			ui.Text(c, fmt.Sprintf("请求超时：%d 秒 · 普通历史上限：%d 条", a.draft.TimeoutSeconds, a.draft.HistoryLimit)).TextColor(t.TextMuted)
			ui.Text(c, "超时、历史上限和启动默认语言可在退出应用后修改 config.json。收藏不计入普通历史上限。").FontSize(12).TextColor(t.TextMuted)
		})
	})
	ui.Row(c).Gap(8).Children(func() {
		ui.Button(c, "放弃未保存修改").Disabled(a.saving).OnClick(func() { a.draft = a.settings.Clone(); a.status = "已恢复已保存设置" })
		ui.Spacer(c)
		ui.PrimaryButton(c, "保存设置").Disabled(a.saving).OnClick(a.saveSettings)
	})
}
