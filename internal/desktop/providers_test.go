package desktop

import (
	"context"
	"errors"
	"image/png"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/egoist/mygo/ui"
	"lumin/internal/config"
	"lumin/internal/domain"
)

func probeApp(t *testing.T) (*App, chan func()) {
	a, q := testApp(t)
	a.draft.Providers = []config.Provider{{ID: "probe", Name: "测试服务", Kind: "ollama", Endpoint: "http://localhost:11434/api/chat", Model: "test-model", Settings: map[string]string{"sample": "before"}}}
	return a, q
}
func nextUpdate(t *testing.T, q chan func()) func() {
	t.Helper()
	select {
	case fn := <-q:
		return fn
	case <-time.After(3 * time.Second):
		t.Fatal("missing UI completion")
		return nil
	}
}

func TestProbeUsesDisabledDraftWithoutSavingOrChangingTranslation(t *testing.T) {
	a, q := probeApp(t)
	a.text = "private user input"
	a.results = []domain.Result{{Text: "previous result"}}
	calls := make(chan domain.Request, 1)
	a.ports.Translators = func(cfg config.Config) []domain.Translator {
		if len(cfg.Providers) != 1 || cfg.Providers[0].Model != "test-model" || !cfg.Providers[0].Enabled {
			t.Fatal("probe did not use just the selected draft")
		}
		return []domain.Translator{testTranslator{func(_ context.Context, r domain.Request) (string, error) {
			calls <- r
			return "你好，世界！", nil
		}}}
	}
	a.testProvider("probe")
	drainUntil(t, q, func() bool { return !a.probe.Busy })
	req := <-calls
	if req.Text != "Hello, world!" || req.Source != "en" || req.Target != "zh" {
		t.Fatal(req)
	}
	if a.probe.Failed || !strings.Contains(a.probe.Message, "你好，世界") {
		t.Fatal(a.probe.Message)
	}
	if a.draft.Providers[0].Enabled || a.settings.Providers[0].ID == "probe" || a.text != "private user input" || a.results[0].Text != "previous result" {
		t.Fatal("probe changed live configuration or translation")
	}
	history, err := a.store.History()
	if err != nil || len(history) != 0 {
		t.Fatal("probe persisted history")
	}
	if _, err := os.Stat(filepath.Join(a.store.Dir(), "state.json")); !os.IsNotExist(err) {
		t.Fatal("probe saved configuration")
	}
}

func TestProbeValidationAndMissingPlugin(t *testing.T) {
	a, q := probeApp(t)
	a.draft.Providers[0].Model = ""
	a.ports.Translators = func(config.Config) []domain.Translator { t.Fatal("invalid config reached factory"); return nil }
	a.testProvider("probe")
	if !a.probe.Failed || a.probe.Busy {
		t.Fatal("invalid model accepted")
	}
	a.ports.Translators = nil
	a.draft.Providers[0] = config.Provider{ID: "plugin", Name: "Missing plugin", Kind: "plugin", Plugin: "missing.plugin@1.0.0"}
	a.testProvider("plugin")
	drainUntil(t, q, func() bool { return !a.probe.Busy })
	if !a.probe.Failed || !strings.Contains(a.probe.Message, "插件管理器不可用") {
		t.Fatal(a.probe.Message)
	}
}

func TestProbeQueuedResultsAreInvalidated(t *testing.T) {
	for _, action := range []string{"cancel", "edit", "remove", "discard", "close"} {
		t.Run(action, func(t *testing.T) {
			a, q := probeApp(t)
			a.ports.Translators = func(config.Config) []domain.Translator {
				return []domain.Translator{testTranslator{func(context.Context, domain.Request) (string, error) { return "stale success", nil }}}
			}
			a.testProvider("probe")
			complete := nextUpdate(t, q)
			switch action {
			case "cancel":
				a.cancelProbe()
			case "edit":
				a.draft.Providers[0].Endpoint = "http://new.test/translate"
			case "remove":
				a.removeProvider("probe")
			case "discard":
				a.discardSettings()
			case "close":
				a.Close()
			}
			complete()
			if strings.Contains(a.probe.Message, "stale success") || a.probe.Busy {
				t.Fatalf("%s accepted stale result: %+v", action, a.probe)
			}
		})
	}
}

func TestProbeSnapshotsPluginSettingsAndReplacesOldTask(t *testing.T) {
	a, q := probeApp(t)
	release := make(chan struct{})
	seen := make(chan string, 1)
	a.ports.Translators = func(cfg config.Config) []domain.Translator {
		values := cfg.Providers[0].Settings
		return []domain.Translator{testTranslator{func(context.Context, domain.Request) (string, error) {
			<-release
			seen <- values["sample"]
			return "old", nil
		}}}
	}
	a.testProvider("probe")
	a.draft.Providers[0].Settings["sample"] = "after"
	close(release)
	old := nextUpdate(t, q)
	if <-seen != "before" {
		t.Fatal("worker observed mutable draft map")
	}
	a.ports.Translators = func(config.Config) []domain.Translator {
		return []domain.Translator{testTranslator{func(context.Context, domain.Request) (string, error) { return "new result", nil }}}
	}
	a.testProvider("probe")
	newResult := nextUpdate(t, q)
	newResult()
	old()
	if !strings.Contains(a.probe.Message, "new result") {
		t.Fatal("old task overwrote new probe")
	}
}

func TestProbeTimeoutAndFailure(t *testing.T) {
	for _, timeout := range []bool{false, true} {
		t.Run(map[bool]string{false: "error", true: "timeout"}[timeout], func(t *testing.T) {
			a, q := probeApp(t)
			a.draft.TimeoutSeconds = 1
			a.ports.Translators = func(config.Config) []domain.Translator {
				return []domain.Translator{testTranslator{func(ctx context.Context, _ domain.Request) (string, error) {
					if timeout {
						<-ctx.Done()
						return "", ctx.Err()
					}
					return "", errors.New("service unavailable")
				}}}
			}
			a.testProvider("probe")
			drainUntil(t, q, func() bool { return !a.probe.Busy })
			if !a.probe.Failed {
				t.Fatal("failure reported as success")
			}
			if timeout && !strings.Contains(a.probe.Message, "超时") {
				t.Fatal(a.probe.Message)
			}
		})
	}
}

func TestProviderReorderKeepsIdentityAndPersists(t *testing.T) {
	a, q := testApp(t)
	a.page = "服务设置"
	ids := []string{a.draft.Providers[0].ID, a.draft.Providers[1].ID}
	tt := ui.NewTester(a.View, 1100, 800)
	click(t, tt, "下移服务："+ids[0])
	tt.Frame()
	tt.Frame()
	if a.draft.Providers[0].ID != ids[1] || a.draft.Providers[1].ID != ids[0] {
		t.Fatal("UI reorder failed")
	}
	if a.draft.Providers[0].Model != "qwen2.5:7b" || a.settings.Providers[0].ID != ids[0] {
		t.Fatal("reorder mixed values or changed saved state")
	}
	a.moveProvider(ids[1], -1)
	if a.draft.Providers[0].ID != ids[1] {
		t.Fatal("moved first item past boundary")
	}
	click(t, tt, "保存服务")
	drainUntil(t, q, func() bool { return !a.saving })
	cfg, err := a.store.LoadConfig()
	if err != nil || cfg.Providers[0].ID != ids[1] {
		t.Fatalf("order not persisted: %v", err)
	}
	cfg.Providers = cfg.Providers[:2]
	for i := range cfg.Providers {
		cfg.Providers[i].Enabled = true
	}
	list := a.translators(cfg)
	if list[0].ID() != ids[1] || list[1].ID() != ids[0] {
		t.Fatal("translator construction ignored saved order")
	}
}

func TestRenderServiceProbe(t *testing.T) {
	a, _ := probeApp(t)
	a.page = "服务设置"
	a.probe = serviceProbe{Provider: a.draft.Clone().Providers[0], TimeoutSeconds: a.draft.TimeoutSeconds, Message: "测试成功 · 0.3 秒\n你好，世界！"}
	for _, width := range []int{1100, 920} {
		tt := ui.NewTester(a.View, width, 680)
		tt.Frame()
		if !tt.HasText(a.probe.Message) {
			t.Fatal("probe result not rendered")
		}
		if r, ok := tt.Find("保存服务"); !ok || r.Y+r.H > 680 {
			t.Fatal("save outside window")
		}
		if dir := os.Getenv("LUMIN_TEST_SCREENSHOTS"); dir != "" && width == 920 {
			if err := os.MkdirAll(dir, 0755); err != nil {
				t.Fatal(err)
			}
			f, err := os.Create(filepath.Join(dir, "service-probe.png"))
			if err != nil {
				t.Fatal(err)
			}
			if err = png.Encode(f, tt.Image()); err != nil {
				t.Fatal(err)
			}
			_ = f.Close()
		}
	}
}

func TestProbeButtonAndEscape(t *testing.T) {
	a, q := probeApp(t)
	a.page = "服务设置"
	started := make(chan struct{})
	a.ports.Translators = func(config.Config) []domain.Translator {
		return []domain.Translator{testTranslator{func(ctx context.Context, _ domain.Request) (string, error) {
			close(started)
			<-ctx.Done()
			return "", ctx.Err()
		}}}
	}
	tt := ui.NewTester(a.View, 920, 680)
	click(t, tt, "测试服务：probe")
	select {
	case <-started:
	case <-time.After(time.Second):
		t.Fatal("probe button did not start request")
	}
	tt.Frame()
	if !a.probe.Busy {
		t.Fatal("probe did not render busy state")
	}
	tt.Key(0, ui.KeyEscape)
	complete := nextUpdate(t, q)
	complete()
	tt.Frame()
	if a.probe.Busy || a.probe.Message != "已取消测试" {
		t.Fatal("Escape did not cancel probe")
	}
}
