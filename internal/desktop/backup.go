package desktop

import (
	"fmt"
	"path/filepath"

	"github.com/egoist/mygo/ui"
	"lumin/internal/backup"
)

type backupState struct {
	busy, restoring, secrets, confirmed bool
	preview                             *backup.Document
	filename, status                    string
}

func (a *App) exportBackup() {
	if a.backup.busy || a.ports.SaveBackup == nil {
		return
	}
	a.backup.busy, a.backup.status = true, "正在导出已保存的数据…"
	secrets := a.backup.secrets
	go func() {
		path, err := a.ports.SaveBackup()
		if err == nil && path != "" {
			err = a.store.Export(path, secrets)
		}
		a.post(func() {
			a.backup.busy = false
			switch {
			case err != nil:
				a.backup.status = "导出失败：" + err.Error()
			case path == "":
				a.backup.status = "已取消导出"
			default:
				a.backup.status = "备份已保存：" + path
			}
		})
	}()
}
func (a *App) previewBackup() {
	if a.backup.busy || a.ports.OpenBackup == nil {
		return
	}
	a.backup.busy, a.backup.confirmed, a.backup.preview = true, false, nil
	a.backup.status = "正在检查备份…"
	go func() {
		path, err := a.ports.OpenBackup()
		var d *backup.Document
		if err == nil && path != "" {
			d, err = backup.Read(path)
		}
		a.post(func() {
			a.backup.busy = false
			switch {
			case err != nil:
				a.backup.status = "无法导入：" + err.Error()
			case d == nil:
				a.backup.status = "已取消选择"
			default:
				a.backup.preview, a.backup.filename = d, filepath.Base(path)
				a.backup.status = "校验通过，请查看预览后确认恢复"
			}
		})
	}()
}
func (a *App) restoreBackup() {
	b := &a.backup
	if b.busy || b.preview == nil || !b.confirmed || a.saving || a.historyBusy || a.pluginBusy || a.historyExportBusy {
		return
	}
	d := *b.preview
	b.busy, b.restoring, b.status = true, true, "正在保存恢复前备份并恢复数据…"
	a.Cancel()
	a.CancelRecognition()
	a.cancelProbe()
	a.cancelOCRProbe()
	a.historyGeneration++
	a.confirmDelete = ""
	a.confirmClear = false
	go func() {
		recovery, err := a.store.Restore(d)
		a.post(func() {
			b := &a.backup
			b.busy, b.restoring, b.confirmed = false, false, false
			if err != nil {
				b.status = "恢复失败，原数据保留：" + err.Error()
				return
			}
			previous := a.settings.Clone()
			a.settings, a.draft = d.Config.Clone(), d.Config.Clone()
			a.history = d.History
			a.results = nil
			a.probe = serviceProbe{}
			a.pluginEditors = nil
			a.syncPreferences()
			b.preview = nil
			b.status = "恢复成功；恢复前备份：" + recovery
			a.status = "配置和历史已恢复，未保存草稿已替换；默认语言及外部调用设置重启后生效"
			if err := a.applyDesktopSettings(previous, false); err != nil {
				a.status += "；" + err.Error()
			}
		})
	}()
}
func (a *App) backupView(c *ui.Context) {
	b, t := &a.backup, c.Theme()
	ui.Scroll(c.Key("backup-scroll")).Grow(1).Children(func() {
		ui.Column(c).Gap(14).Children(func() {
			ui.Text(c, "导出备份").FontSize(18).Bold()
			ui.Text(c, "备份已保存的配置、历史及收藏；不包含未保存草稿、插件程序和环境变量值。").TextColor(t.TextMuted)
			ui.Checkbox(c, &b.secrets, "包含密钥和插件参数（明文）").Disabled(b.busy)
			ui.Text(c, "默认移除 API Key、外部调用令牌及全部插件参数，并禁用翻译服务与外部调用；恢复后请补全配置再启用。").FontSize(12).TextColor(t.TextMuted)
			ui.Button(c, "导出备份文件").Disabled(b.busy || a.ports.SaveBackup == nil).OnClick(a.exportBackup)
			ui.Text(c, "从备份恢复").FontSize(18).Bold()
			ui.Text(c, "恢复采用覆盖方式，不合并记录。插件需单独安装；恢复前会在本机 backups 目录保存包含密钥的完整备份。").TextColor(t.TextMuted)
			ui.Button(c, "选择备份并预览").Disabled(b.busy || a.ports.OpenBackup == nil).OnClick(a.previewBackup)
			if d := b.preview; d != nil {
				favorites, plugins := 0, 0
				for _, e := range d.History {
					if e.Favorite {
						favorites++
					}
				}
				for _, p := range d.Config.Providers {
					if p.Kind == "plugin" {
						plugins++
					}
				}
				ui.Column(c.Key("backup-preview")).Padding(14).Gap(8).Border(1, t.Border).Radius(10).Children(func() {
					ui.Text(c, b.filename).Bold()
					ui.Text(c, fmt.Sprintf("格式 v%d · %s", d.Version, d.CreatedAt.Local().Format("2006-01-02 15:04")))
					ui.Text(c, fmt.Sprintf("%d 条历史（%d 条收藏）· %d 个翻译服务（%d 个插件引用）", len(d.History), favorites, len(d.Config.Providers), plugins))
					label := "未包含密钥和插件参数"
					if d.IncludesSecrets {
						label = "包含密钥和插件参数"
					}
					ui.Text(c, label).TextColor(t.TextMuted)
					ui.Checkbox(c, &b.confirmed, "确认覆盖当前配置、全部历史和未保存草稿").Disabled(b.busy)
					ui.Row(c).Gap(8).Children(func() {
						ui.Button(c, "取消恢复").Disabled(b.busy).OnClick(func() { b.preview = nil; b.confirmed = false })
						ui.PrimaryButton(c, "覆盖并恢复").Disabled(b.busy || !b.confirmed || a.saving || a.historyBusy || a.pluginBusy || a.historyExportBusy).OnClick(a.restoreBackup)
					})
				})
			}
			ui.Text(c.Key("backup-status"), b.status).Role(ui.RoleStatus).Selectable().FontSize(12).TextColor(t.TextMuted)
		})
	})
}
