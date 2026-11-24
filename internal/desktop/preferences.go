package desktop

import (
	"math"

	"github.com/egoist/mygo/ui"
	"lumin/internal/domain"
)

// MyGo NumberInput binds float64, while persisted settings remain validated ints.
// These editor values live across frames and publish edits through OnChange.
type preferencesEditor struct {
	Timeout, History                                     float64
	WindowWidth, WindowHeight, ResultFont                float64
	Appearance                                           string
	AutoHide                                             bool
	WindowSizeEdited                                     bool
	Source, Target                                       string
	InputShortcut, SelectionShortcut, ScreenshotShortcut string
}

func (a *App) syncPreferences() {
	w, h := a.draft.TranslationSize()
	a.preferences = preferencesEditor{WindowWidth: float64(w), WindowHeight: float64(h), ResultFont: float64(a.draft.ResultFontSize()), Appearance: appearanceLabel(a.draft.Appearance), AutoHide: !a.draft.KeepTranslationOnBlur, Timeout: float64(a.draft.TimeoutSeconds), History: float64(a.draft.HistoryLimit), Source: domain.LanguageName(a.draft.Source), Target: domain.LanguageName(a.draft.Target),
		InputShortcut: shortcutDisplay(a.draft.InputShortcut), SelectionShortcut: shortcutDisplay(a.draft.SelectionShortcut), ScreenshotShortcut: shortcutDisplay(a.draft.ScreenshotShortcut)}
}

func (a *App) discardSettings() {
	a.cancelProbe()
	a.cancelOCRProbe()
	a.probe = serviceProbe{}
	a.draft = a.settings.Clone()
	a.pluginEditors = nil
	a.syncPreferences()
	a.status = "已恢复已保存设置"
}

func (a *App) preferencesView(c *ui.Context) {
	t := c.Theme()
	ui.Text(c, "翻译默认设置").FontSize(18).Bold()
	ui.Row(c.Key("default-languages")).Gap(16).Children(func() {
		ui.Column(c.Key("default-source")).Grow(1).MinWidth(0).Gap(6).Children(func() {
			ui.Text(c, "启动源语言").FontSize(12).TextColor(t.TextMuted)
			ui.Select(c, &a.preferences.Source, languageNames(false)).Label("默认源语言选择").OnChange(func() { a.draft.Source = languageCode(a.preferences.Source) })
		})
		ui.Column(c.Key("default-target")).Grow(1).MinWidth(0).Gap(6).Children(func() {
			ui.Text(c, "启动目标语言").FontSize(12).TextColor(t.TextMuted)
			ui.Select(c, &a.preferences.Target, languageNames(true)).Label("默认目标语言选择").OnChange(func() { a.draft.Target = languageCode(a.preferences.Target) })
		})
	})
	ui.Row(c.Key("limits")).Gap(16).Children(func() {
		ui.Column(c.Key("timeout")).Grow(1).MinWidth(0).Gap(6).Children(func() {
			ui.Text(c, "请求超时（1–300 秒）").FontSize(12).TextColor(t.TextMuted)
			ui.NumberInput(c, &a.preferences.Timeout, 1, 300, 1).Label("请求超时输入").OnChange(func() { a.draft.TimeoutSeconds = int(math.Round(a.preferences.Timeout)) })
		})
		ui.Column(c.Key("history-limit")).Grow(1).MinWidth(0).Gap(6).Children(func() {
			ui.Text(c, "普通历史上限（1–5000 条）").FontSize(12).TextColor(t.TextMuted)
			ui.NumberInput(c, &a.preferences.History, 1, 5000, 1).Label("历史上限输入").OnChange(func() { a.draft.HistoryLimit = int(math.Round(a.preferences.History)) })
		})
	})
	ui.Text(c, "默认语言下次启动生效；超时用于保存后的新请求。历史上限在下次记录翻译时执行，收藏不计入上限。").FontSize(12).TextColor(t.TextMuted)
}
