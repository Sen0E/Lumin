package desktop

import (
	"math"

	"github.com/egoist/mygo/ui"
	"lumin/internal/config"
)

func appearanceLabel(value string) string {
	switch value {
	case "light":
		return "浅色"
	case "dark":
		return "深色"
	default:
		return "跟随系统"
	}
}
func appearanceValue(label string) string {
	switch label {
	case "浅色":
		return "light"
	case "深色":
		return "dark"
	default:
		return "system"
	}
}
func (a *App) appearanceView(c *ui.Context) {
	ui.Text(c, "外观").FontSize(18).Bold()
	ui.Select(c, &a.preferences.Appearance, []string{"跟随系统", "浅色", "深色"}).Label("外观主题选择").OnChange(func() { a.draft.Appearance = appearanceValue(a.preferences.Appearance) })
	ui.Text(c, "保存后应用于所有窗口；跟随系统时会自动响应系统外观变化。").FontSize(12).TextColor(c.Theme().TextMuted)
}
func (a *App) translationPreferencesView(c *ui.Context) {
	ui.Text(c, "翻译窗口与阅读").FontSize(18).Bold()
	ui.Checkbox(c, &a.preferences.AutoHide, "点击其他窗口后自动隐藏（未置顶）").OnClick(func() { a.draft.KeepTranslationOnBlur = !a.preferences.AutoHide })
	ui.Text(c, "置顶后始终保持显示；手动关闭或 Esc 仍可隐藏。自动隐藏会取消当前任务，保留已有原文和译文。").FontSize(12).TextColor(c.Theme().TextMuted)
	ui.Row(c.Key("translation-size")).Gap(12).Children(func() {
		ui.Column(c.Key("window-width")).Grow(1).MinWidth(0).Gap(6).Children(func() {
			ui.Text(c, "默认宽度").FontSize(12).TextColor(c.Theme().TextMuted)
			ui.NumberInput(c, &a.preferences.WindowWidth, 320, 1600, 10).Label("翻译窗口宽度输入").OnChange(func() {
				a.draft.TranslationWidth = int(math.Round(a.preferences.WindowWidth))
				a.preferences.WindowSizeEdited = true
			})
		})
		ui.Column(c.Key("window-height")).Grow(1).MinWidth(0).Gap(6).Children(func() {
			ui.Text(c, "默认高度").FontSize(12).TextColor(c.Theme().TextMuted)
			ui.NumberInput(c, &a.preferences.WindowHeight, 360, 1200, 10).Label("翻译窗口高度输入").OnChange(func() {
				a.draft.TranslationHeight = int(math.Round(a.preferences.WindowHeight))
				a.preferences.WindowSizeEdited = true
			})
		})
		ui.Column(c.Key("result-font")).Grow(1).MinWidth(0).Gap(6).Children(func() {
			ui.Text(c, "译文字号").FontSize(12).TextColor(c.Theme().TextMuted)
			ui.NumberInput(c, &a.preferences.ResultFont, 12, 24, 1).Label("译文字号输入").OnChange(func() { a.draft.TranslationFontSize = int(math.Round(a.preferences.ResultFont)) })
		})
	})
	ui.Row(c.Key("size-presets")).Gap(8).Children(func() {
		for _, preset := range []struct {
			name string
			w, h int
		}{{"紧凑", 350, 420}, {"标准", 420, 520}, {"宽屏", 520, 620}} {
			ui.Button(c.Key(preset.name), preset.name).OnClick(func() {
				a.preferences.WindowSizeEdited = true
				a.draft.TranslationWidth, a.draft.TranslationHeight = preset.w, preset.h
				a.preferences.WindowWidth, a.preferences.WindowHeight = float64(preset.w), float64(preset.h)
			})
		}
	})
	ui.Text(c, "尺寸按逻辑像素计算，保存后立即应用。窗口边缘可直接拖动调整，位置与大小会在退出后记忆。").FontSize(12).TextColor(c.Theme().TextMuted)
}

func (a *App) cancelTranslationBlur() {
	if a.ports.CancelTranslationBlur != nil {
		a.ports.CancelTranslationBlur()
	}
}

// Only native blur calls this. Capture, visibility and focus are checked by
// bootstrap after a short grace period; pinning is a session-level choice.
func (a *App) TranslationBlurred() {
	if a.closed || a.pinned || a.readingSelection || a.settings.KeepTranslationOnBlur {
		return
	}
	a.DismissTranslation()
}

// Keep native/system metrics, while giving small status/error labels and white
// action text enough contrast on the surfaces used by all three business views.
func readableTheme(base *ui.Theme) *ui.Theme {
	t := *base
	if t.Dark {
		t.Accent = ui.Hex("#2563eb")
		t.AccentHover = ui.Hex("#1d4ed8")
		t.AccentPressed = ui.Hex("#1e40af")
		t.Danger = ui.Hex("#f87171")
	} else {
		t.TextMuted = ui.Hex("#666670")
		t.Danger = ui.Hex("#b91c1c")
	}
	return &t
}
func (a *App) applyAppearance(c *ui.Context) {
	base := c.Theme()
	if a.settings.Appearance == "dark" && !base.Dark {
		base = ui.DarkTheme()
	}
	if a.settings.Appearance == "light" && base.Dark {
		base = ui.LightTheme()
	}
	c.SetTheme(readableTheme(base))
}

// Apply native appearance/hotkeys and size from both save and backup restore.
// An explicit preset can reapply the same default after a manual native resize;
// saving an unrelated preference keeps that manually adjusted size.
func (a *App) applyDesktopSettings(previous config.Config, forceSize bool) error {
	var err error
	if a.ports.ApplyConfig != nil {
		err = a.ports.ApplyConfig(a.settings.Clone())
	}
	ow, oh := previous.TranslationSize()
	nw, nh := a.settings.TranslationSize()
	if (forceSize || ow != nw || oh != nh) && a.ports.SetTranslationSize != nil {
		a.ports.SetTranslationSize(nw, nh)
	}
	return err
}
