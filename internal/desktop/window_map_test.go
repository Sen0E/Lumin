package desktop

import (
	"github.com/egoist/mygo/ui"
	"lumin/internal/domain"
	"testing"
	"time"
)

func TestMainHistoryLoadsTranslatorWithoutChangingMainPage(t *testing.T) {
	a, _ := testApp(t)
	a.page = "历史记录"
	a.history = []domain.Entry{
		{ID: "ordinary", CreatedAt: time.Now(), Request: domain.Request{Text: "ordinary input", Source: "en", Target: "zh"}},
		{ID: "favorite", CreatedAt: time.Now(), Favorite: true, Request: domain.Request{Text: "saved input", Source: "en", Target: "zh"}, Results: []domain.Result{{ProviderID: "test", ProviderName: "Test", Text: "已保存译文"}}},
	}
	a.draft.Providers[0].Name = "unsaved draft"
	shown := 0
	a.ports.ShowTranslator = func() { shown++ }
	main := ui.NewTester(a.View, 920, 680)
	click(t, main, "仅显示收藏（本地生词本）")
	main.Frame()
	main.Frame()
	if main.HasText("ordinary input") || !main.HasText("saved input") {
		t.Fatal("favorites filter failed")
	}
	click(t, main, "载入")
	if shown != 1 || a.page != "历史记录" || a.text != "saved input" || a.results[0].Text != "已保存译文" {
		t.Fatal("history load routed incorrectly")
	}
	if a.draft.Providers[0].Name != "unsaved draft" {
		t.Fatal("history load overwrote preferences draft")
	}
	popup := ui.NewTester(a.TranslationView, 460, 480)
	if !popup.HasText("已保存译文") {
		t.Fatal("loaded result absent from popup")
	}
}

func TestWindowControlPlacement(t *testing.T) {
	a, _ := testApp(t)
	cases := []struct{ page, contains string }{
		{"常规设置", "启用本地 HTTP 服务（保存后重启生效）"},
		{"翻译设置", "启动源语言"},
		{"热键设置", "打开翻译窗口快捷键"},
		{"服务设置", "新增服务类型"},
		{"历史记录", "搜索记录"},
	}
	for _, tc := range cases {
		a.page = tc.page
		tt := ui.NewTester(a.View, 920, 680)
		if _, ok := tt.Find(tc.contains); !ok {
			t.Fatalf("%s missing %s", tc.page, tc.contains)
		}
		for _, label := range []string{"待翻译文本", "复制原文", "翻译文本", "图片识字", "截图识字"} {
			for _, text := range tt.Texts() {
				if text == label {
					t.Fatalf("translation action %q in main %s", label, tc.page)
				}
			}
		}
	}
	a.page = "服务设置"
	tt := ui.NewTester(a.View, 920, 680)
	click(t, tt, "OCR 服务")
	if !tt.HasText("OCR 引擎") {
		t.Fatal("OCR service missing")
	}
	click(t, tt, "插件")
	if !tt.HasText("安装插件") {
		t.Fatal("plugins not under service settings")
	}
}

func TestPopupPinAndSourceCopy(t *testing.T) {
	a, _ := testApp(t)
	a.text = "source text"
	var values []bool
	clip := ""
	a.ports.SetTranslationPinned = func(v bool) { values = append(values, v) }
	a.ports.WriteClipboard = func(v string) { clip = v }
	tt := ui.NewTester(a.TranslationView, 460, 480)
	click(t, tt, "置顶")
	tt.Frame()
	tt.Frame()
	click(t, tt, "取消置顶")
	click(t, tt, "复制原文")
	if len(values) != 2 || !values[0] || values[1] || a.pinned {
		t.Fatal("pin command not delivered")
	}
	if clip != "source text" {
		t.Fatal("source copy failed")
	}
}
