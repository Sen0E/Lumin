package desktop

import (
	"fmt"
	"strings"

	"lumin/internal/domain"
)

func (a *App) filteredHistory() []domain.Entry {
	var entries []domain.Entry
	query := strings.ToLower(a.search)
	for _, entry := range a.history {
		if a.favoritesOnly && !entry.Favorite {
			continue
		}
		searchable := entry.Request.Text
		for _, r := range entry.Results {
			searchable += "\n" + r.Text
		}
		if !strings.Contains(strings.ToLower(searchable), query) {
			continue
		}
		entries = append(entries, entry)
	}
	return entries
}

func (a *App) deleteHistory(id string, favorite bool) {
	if a.confirmDelete != id || a.historyBusy || a.backup.restoring {
		return
	}
	a.confirmDelete = ""
	a.changeHistory(func() error { return a.store.Delete(id, favorite) })
}

func (a *App) exportHistory() {
	if a.historyExportBusy || a.historyBusy || a.backup.restoring || a.ports.SaveHistoryExport == nil {
		return
	}
	entries := a.filteredHistory()
	if len(entries) == 0 {
		a.status = "当前没有可导出的记录"
		return
	}
	// Deep copy mutable result slices before passing the displayed selection to I/O.
	for i := range entries {
		entries[i].Results = append([]domain.Result(nil), entries[i].Results...)
	}
	format := a.historyExportFormat
	a.historyExportBusy = true
	a.status = fmt.Sprintf("正在导出当前筛选的 %d 条记录…", len(entries))
	go func() {
		path, err := a.ports.SaveHistoryExport(format)
		if err == nil && path != "" {
			err = a.store.ExportHistory(path, entries, format)
		}
		a.post(func() {
			a.historyExportBusy = false
			switch {
			case err != nil:
				a.status = "历史导出失败：" + err.Error()
			case path == "":
				a.status = "已取消历史导出"
			default:
				a.status = fmt.Sprintf("已导出 %d 条记录：%s", len(entries), path)
			}
		})
	}()
}
