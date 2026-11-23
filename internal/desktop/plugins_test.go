package desktop

import (
	"strings"
	"testing"

	"github.com/egoist/mygo/ui"
	"lumin/internal/config"
	"lumin/internal/plugin"
)

func TestPluginFieldsSurviveFramesAndDiscard(t *testing.T) {
	a, _ := testApp(t)
	a.installed = []plugin.Manifest{{Schema: 1, ID: "test.translate", Version: "1.0.0", Name: "Test plugin", Fields: []plugin.Field{{Key: "token", Label: "插件令牌", Secret: true}}}}
	a.settings.Providers = []config.Provider{{ID: "plugin-one", Name: "Test plugin", Kind: "plugin", Plugin: "test.translate@1.0.0", Settings: map[string]string{"token": ""}}}
	a.draft = a.settings.Clone()
	a.page = "翻译服务"
	tt := ui.NewTester(a.View, 1100, 800)
	click(t, tt, "插件设置：插件令牌")
	tt.Type("test-key")
	tt.Frame()
	tt.Frame()
	if a.draft.Providers[0].Settings["token"] != "test-key" {
		t.Fatalf("plugin input was lost across frames: config=%v editors=%v texts=%v", a.draft.Providers[0].Settings, a.pluginEditors, tt.Texts())
	}
	if a.settings.Providers[0].Settings["token"] != "" {
		t.Fatal("unsaved edit changed runtime settings")
	}
	click(t, tt, "插件")
	click(t, tt, "翻译服务")
	tt.Frame()
	if a.draft.Providers[0].Settings["token"] != "test-key" {
		t.Fatal("navigation lost plugin values")
	}
	click(t, tt, "设置")
	click(t, tt, "放弃未保存修改")
	click(t, tt, "翻译服务")
	if a.draft.Providers[0].Settings["token"] != "" {
		t.Fatal("discard did not restore settings")
	}
	if !a.pluginUsed("test.translate@1.0.0") {
		t.Fatal("referenced plugin could be removed")
	}
	a.draft.Providers = nil
	if !a.pluginUsed("test.translate@1.0.0") {
		t.Fatal("saved references ignored")
	}
	a.settings.Providers = nil
	if a.pluginUsed("test.translate@1.0.0") {
		t.Fatal("unused plugin locked")
	}
}
func TestCannotSaveMissingPlugin(t *testing.T) {
	a, q := testApp(t)
	a.draft.Providers = []config.Provider{{ID: "plugin", Name: "Missing", Kind: "plugin", Plugin: "missing.plugin@1.0.0", Enabled: true}}
	a.saveSettings()
	drainUntil(t, q, func() bool { return !a.saving })
	if !strings.Contains(a.status, "保存失败") {
		t.Fatal(a.status)
	}
	if a.settings.Providers[0].Kind == "plugin" {
		t.Fatal("invalid runtime config committed")
	}
}
