package desktop

import (
	"runtime"
	"strings"

	"github.com/egoist/mygo/ui"
)

func shortcutDisplay(value string) string { return shortcutDisplayFor(value, runtime.GOOS) }

func shortcutDisplayFor(value, platform string) string {
	if strings.TrimSpace(value) == "" {
		return ""
	}
	parts := strings.Split(value, "+")
	for i, part := range parts {
		part = strings.TrimSpace(part)
		if strings.EqualFold(part, "CmdOrCtrl") || strings.EqualFold(part, "CommandOrControl") {
			part = "Ctrl"
			if platform == "darwin" {
				part = "Cmd"
			}
		}
		parts[i] = part
	}
	return strings.Join(parts, " + ")
}

func compactShortcut(value string) string {
	if strings.TrimSpace(value) == "" {
		return ""
	}
	parts := strings.Split(value, "+")
	for i := range parts {
		parts[i] = strings.TrimSpace(parts[i])
	}
	return strings.Join(parts, "+")
}

// Bind the readable value persistently; never derive a local editor each frame.
// Unedited portable accelerators remain byte-for-byte unchanged in the config.
func shortcutField(c *ui.Context, label string, editor, value *string) {
	ui.Column(c.Key(label)).Gap(6).Children(func() {
		ui.Text(c, label).FontSize(12).TextColor(c.Theme().TextMuted)
		ui.TextInput(c, editor).Label(label + "输入").OnChange(func() {
			compact := compactShortcut(*editor)
			if compact != compactShortcut(shortcutDisplay(*value)) {
				*value = compact
			}
		})
	})
}
