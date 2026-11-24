package desktop

import (
	"encoding/json"
	"errors"
	"image/png"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/egoist/mygo/ui"
	"lumin/internal/domain"
	"lumin/internal/historyexport"
)

func setupHistoryActions(t *testing.T) (*App, chan func()) {
	t.Helper()
	a, q := testApp(t)
	a.page = "历史记录"
	a.history = []domain.Entry{{ID: "saved", CreatedAt: time.Now(), Favorite: true, Request: domain.Request{Text: "Favorite 中文", Source: "en", Target: "zh"}, Results: []domain.Result{{ProviderName: "test", Text: "结果\n多行"}}}, {ID: "ordinary", CreatedAt: time.Now(), Request: domain.Request{Text: "Ordinary", Source: "en", Target: "zh"}}}
	for i := len(a.history) - 1; i >= 0; i-- {
		if err := a.store.Add(a.history[i], 100); err != nil {
			t.Fatal(err)
		}
	}
	return a, q
}
func TestHistoryDeleteConfirmationAndDraftIsolation(t *testing.T) {
	a, q := setupHistoryActions(t)
	a.draft.TimeoutSeconds = 99
	a.text = "current input"
	tt := ui.NewTester(a.View, 920, 680)
	click(t, tt, "删除记录：saved")
	if !tt.HasText("确认删除收藏") {
		t.Fatal("favorite warning absent")
	}
	h, _ := a.store.History()
	if len(h) != 2 {
		t.Fatal("delete before confirmation")
	}
	click(t, tt, "保留记录")
	tt.Frame()
	tt.Frame()
	if a.confirmDelete != "" {
		t.Fatal("cancel failed")
	}
	click(t, tt, "删除记录：ordinary")
	click(t, tt, "确认删除")
	drainUntil(t, q, func() bool { return !a.historyBusy && len(a.history) == 1 })
	tt.Frame()
	tt.Frame()
	if a.history[0].ID != "saved" || a.text != "current input" || a.draft.TimeoutSeconds != 99 {
		t.Fatal("delete touched unrelated state")
	}
	click(t, tt, "删除记录：saved")
	click(t, tt, "确认删除收藏")
	drainUntil(t, q, func() bool { return !a.historyBusy && len(a.history) == 0 })
	tt.Frame()
	tt.Frame()
	if !tt.HasText("暂无记录") {
		t.Fatal("deleted row stayed in view")
	}
}
func TestHistoryExportFreezesCurrentFilter(t *testing.T) {
	a, q := setupHistoryActions(t)
	a.favoritesOnly = true
	a.search = "多行"
	path := filepath.Join(t.TempDir(), "history.json")
	started, release := make(chan struct{}), make(chan struct{})
	a.ports.SaveHistoryExport = func(format string) (string, error) {
		if format != "JSON" {
			return "", errors.New("wrong format")
		}
		close(started)
		<-release
		return path, nil
	}
	tt := ui.NewTester(a.View, 920, 680)
	click(t, tt, "导出当前结果")
	<-started
	// Change the filter and replace result data while the native dialog is open.
	a.search = "ordinary"
	a.favoritesOnly = false
	a.history[0].Results[0].Text = "changed"
	close(release)
	drainUntil(t, q, func() bool { return !a.historyExportBusy })
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	var d historyexport.Document
	if err = json.Unmarshal(data, &d); err != nil {
		t.Fatal(err)
	}
	if len(d.Entries) != 1 || d.Entries[0].ID != "saved" || d.Entries[0].Results[0].Text != "结果\n多行" {
		t.Fatal("export changed with UI filter", d)
	}
	a.backup.restoring = true
	a.confirmDelete = "saved"
	a.deleteHistory("saved", true)
	a.exportHistory()
	if a.historyBusy || a.historyExportBusy {
		t.Fatal("history work started during restore")
	}
}
func TestHistoryExportFailureAndCancellation(t *testing.T) {
	for _, err := range []error{nil, errors.New("permission denied")} {
		a, q := setupHistoryActions(t)
		a.ports.SaveHistoryExport = func(string) (string, error) { return "", err }
		a.exportHistory()
		drainUntil(t, q, func() bool { return !a.historyExportBusy })
		want := "已取消历史导出"
		if err != nil {
			want = "历史导出失败"
		}
		if !strings.Contains(a.status, want) {
			t.Fatal(a.status)
		}
	}
}
func TestHistoryActionsLayout(t *testing.T) {
	a, _ := setupHistoryActions(t)
	a.confirmDelete = "saved"
	a.ports.SaveHistoryExport = func(string) (string, error) { return "", nil }
	for _, size := range []struct{ w, h int }{{920, 680}, {1100, 800}} {
		tt := ui.NewTester(a.View, size.w, size.h)
		tt.Frame()
		tt.Frame()
		for _, label := range []string{"历史导出格式", "导出当前结果", "删除记录：saved", "确认删除收藏", "保留记录"} {
			b, ok := tt.Find(label)
			if !ok || b.W <= 0 || b.H <= 0 || b.X < 0 || b.Y < 0 || b.X+b.W > float32(size.w) || b.Y+b.H > float32(size.h) {
				t.Fatalf("%s out of bounds %+v", label, b)
			}
		}
		if dir := os.Getenv("LUMIN_TEST_SCREENSHOTS"); dir != "" && size.w == 920 {
			if err := os.MkdirAll(dir, 0755); err != nil {
				t.Fatal(err)
			}
			f, err := os.Create(filepath.Join(dir, "history-actions.png"))
			if err != nil {
				t.Fatal(err)
			}
			err = png.Encode(f, tt.Image())
			f.Close()
			if err != nil {
				t.Fatal(err)
			}
		}
	}
}
