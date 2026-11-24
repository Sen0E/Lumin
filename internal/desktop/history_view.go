package desktop

import (
	"github.com/egoist/mygo/ui"
	"lumin/internal/domain"
)

func (a *App) historyView(c *ui.Context) {
	t := c.Theme()
	favorites := a.favoritesOnly
	ui.Checkbox(c, &a.favoritesOnly, "仅显示收藏（本地生词本）").OnClick(func() { a.confirmClear = false; a.confirmDelete = "" })
	ui.Row(c).Gap(8).Children(func() {
		ui.TextInput(c.Key("history-search"), &a.search).Placeholder("搜索原文或译文").Label("搜索记录").Grow(1).OnChange(func() { a.confirmDelete = ""; a.confirmClear = false })
		if !favorites {
			ui.Button(c, "清除历史").Disabled(a.historyBusy || a.busy).OnClick(func() { a.confirmClear = true; a.confirmDelete = "" })
		}
	})
	entries := a.filteredHistory()
	ui.Row(c.Key("history-export")).Gap(8).AlignItems(ui.Center).Children(func() {
		ui.Textf(c, "当前 %d 条记录", len(entries)).Grow(1)
		ui.Select(c, &a.historyExportFormat, []string{"JSON", "CSV"}).Label("历史导出格式").Width(100).Disabled(a.historyExportBusy)
		ui.Button(c, "导出当前结果").Disabled(a.historyExportBusy || a.historyBusy || len(entries) == 0 || a.ports.SaveHistoryExport == nil).OnClick(a.exportHistory)
	})
	if a.confirmClear && !favorites {
		ui.Row(c.Key("clear-confirm")).Gap(8).AlignItems(ui.Center).Children(func() {
			ui.Text(c, "清除普通历史，保留生词本？").Grow(1)
			ui.Button(c, "返回").OnClick(func() { a.confirmClear = false; a.confirmDelete = "" })
			ui.Button(c, "确认清除").Disabled(a.historyBusy).OnClick(func() { a.confirmClear = false; a.changeHistory(a.store.Clear) })
		})
	}
	ui.Scroll(c.Key("history-list")).Grow(1).Children(func() {
		ui.Column(c).Gap(12).Children(func() {
			for _, entry := range entries {
				ui.Column(c.Key(entry.ID)).Padding(16).Gap(10).Border(1, t.Border).Radius(10).Children(func() {
					ui.Row(c).AlignItems(ui.Center).Gap(8).Children(func() {
						ui.Text(c, entry.CreatedAt.Local().Format("2006-01-02 15:04")+" · "+domain.LanguageName(entry.Request.Target)).FontSize(12).TextColor(t.TextMuted)
						ui.Spacer(c)
						label := "收藏"
						if entry.Favorite {
							label = "取消收藏"
						}
						ui.Button(c, label).Disabled(a.historyBusy).OnClick(func() { a.changeHistory(func() error { return a.store.Favorite(entry.ID, !entry.Favorite) }) })
						ui.Button(c, "删除").Label("删除记录：" + entry.ID).Disabled(a.historyBusy).OnClick(func() { a.confirmDelete = entry.ID; a.confirmClear = false })
						ui.Button(c, "载入").OnClick(func() {
							a.sourceChanged()
							a.text = entry.Request.Text
							a.source = domain.LanguageName(entry.Request.Source)
							a.target = domain.LanguageName(entry.Request.Target)
							a.results = append([]domain.Result(nil), entry.Results...)
							a.ShowTranslation()
						})
					})
					if a.confirmDelete == entry.ID {
						ui.Column(c.Key("delete-confirm")).Gap(8).Children(func() {
							message := "删除这条历史记录？此操作不可撤销。"
							label := "确认删除"
							if entry.Favorite {
								message = "这条记录已收藏，删除会同时移出生词本。此操作不可撤销。"
								label = "确认删除收藏"
							}
							ui.Text(c, message).TextColor(t.Danger)
							ui.Row(c).Gap(8).Children(func() {
								ui.Button(c, "保留记录").Disabled(a.historyBusy).OnClick(func() { a.confirmDelete = "" })
								ui.Button(c, label).Disabled(a.historyBusy).OnClick(func() { a.deleteHistory(entry.ID, entry.Favorite) })
							})
						})
					}
					ui.Text(c, preview(entry.Request.Text, 180)).Bold().Selectable()
					for _, result := range entry.Results {
						if result.Text != "" {
							ui.Text(c, result.ProviderName+" · "+preview(result.Text, 240)).TextColor(t.TextMuted).Selectable()
						}
					}
				})
			}
			if len(entries) == 0 {
				ui.Text(c, "暂无记录。完成翻译后，历史记录会出现在这里；收藏的记录也会显示在生词本。").TextColor(t.TextMuted).Padding(24)
			}
		})
	})
}
