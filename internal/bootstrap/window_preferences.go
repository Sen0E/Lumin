package bootstrap

import (
	"github.com/egoist/mygo"
	"lumin/internal/config"
)

func translationWindowOptions(cfg config.Config) mygo.WindowOptions {
	width, height := cfg.TranslationSize()
	return mygo.WindowOptions{Title: "Lumin · 翻译", Width: width, Height: height, MinWidth: 320, MinHeight: 360, StateKey: "translation", Hidden: true}
}
func shortcutsChanged(old, next config.Config) bool {
	return old.InputShortcut != next.InputShortcut || old.SelectionShortcut != next.SelectionShortcut || old.ScreenshotShortcut != next.ScreenshotShortcut
}
