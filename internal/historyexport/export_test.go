package historyexport

import (
	"encoding/csv"
	"encoding/json"
	"reflect"
	"strings"
	"testing"
	"time"

	"lumin/internal/domain"
)

func TestUnicodeMultilineAndMultipleProviders(t *testing.T) {
	entries := []domain.Entry{{ID: "one", CreatedAt: time.Now().UTC(), Favorite: true, Request: domain.Request{Text: "你好,\"世界\"\n第二行", Source: "zh", Target: "en"}, Results: []domain.Result{{ProviderID: "p1", ProviderName: "服务一", Text: "Hello, \"world\"\nsecond line", DurationMS: 42}, {ProviderID: "p2", ProviderName: "服务二", Error: "超时\n请重试"}}}, {ID: "empty", Request: domain.Request{Text: "empty"}}}
	b, err := Encode(entries, "JSON")
	if err != nil {
		t.Fatal(err)
	}
	var doc Document
	if err = json.Unmarshal(b, &doc); err != nil {
		t.Fatal(err)
	}
	if doc.Format != "lumin-history" || doc.Version != 1 || !reflect.DeepEqual(doc.Entries, entries) {
		t.Fatal("JSON round trip changed data")
	}
	b, err = Encode(entries, "CSV")
	if err != nil {
		t.Fatal(err)
	}
	rows, err := csv.NewReader(strings.NewReader(string(b))).ReadAll()
	if err != nil {
		t.Fatal(err)
	}
	if len(rows) != 4 || rows[1][5] != entries[0].Request.Text || rows[1][8] != entries[0].Results[0].Text || rows[2][9] != entries[0].Results[1].Error || rows[3][0] != "empty" || rows[1][2] != "true" {
		t.Fatal("CSV lost multiline or provider fields", rows)
	}
}
func TestInvalidFormatAndEmptyExport(t *testing.T) {
	if _, err := Encode(nil, "XML"); err == nil {
		t.Fatal("unknown format accepted")
	}
	b, err := Encode(nil, "JSON")
	if err != nil || !strings.Contains(string(b), `"entries": []`) {
		t.Fatal("empty JSON must be an array")
	}
	b, err = Encode(nil, "CSV")
	if err != nil {
		t.Fatal(err)
	}
	rows, err := csv.NewReader(strings.NewReader(string(b))).ReadAll()
	if err != nil || len(rows) != 1 {
		t.Fatal("empty CSV header")
	}
}
