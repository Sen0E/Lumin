// Package historyexport serializes history only; configuration is never input.
package historyexport

import (
	"bytes"
	"encoding/csv"
	"encoding/json"
	"errors"
	"strconv"
	"time"

	"lumin/internal/domain"
)

const MaxBytes = 64 << 20

type Document struct {
	Format     string         `json:"format"`
	Version    int            `json:"version"`
	ExportedAt time.Time      `json:"exported_at"`
	Entries    []domain.Entry `json:"entries"`
}

// Encode emits one CSV row per provider result (one empty result for entries
// without results). encoding/csv quotes commas, quotes and embedded newlines.
func Encode(entries []domain.Entry, format string) ([]byte, error) {
	var data []byte
	var err error
	switch format {
	case "JSON":
		if entries == nil {
			entries = []domain.Entry{}
		}
		data, err = json.MarshalIndent(Document{"lumin-history", 1, time.Now().UTC(), entries}, "", "  ")
		data = append(data, '\n')
	case "CSV":
		var b bytes.Buffer
		w := csv.NewWriter(&b)
		err = w.Write([]string{"id", "created_at", "favorite", "source", "target", "text", "provider_id", "provider_name", "translation", "error", "duration_ms"})
		for _, entry := range entries {
			results := entry.Results
			if len(results) == 0 {
				results = []domain.Result{{}}
			}
			for _, r := range results {
				if err != nil {
					return nil, err
				}
				err = w.Write([]string{entry.ID, entry.CreatedAt.Format(time.RFC3339Nano), strconv.FormatBool(entry.Favorite), entry.Request.Source, entry.Request.Target, entry.Request.Text, r.ProviderID, r.ProviderName, r.Text, r.Error, strconv.FormatInt(r.DurationMS, 10)})
				if b.Len() > MaxBytes {
					return nil, errors.New("导出超过 64 MiB，请缩小搜索范围")
				}
			}
		}
		w.Flush()
		if err == nil {
			err = w.Error()
		}
		data = b.Bytes()
	default:
		return nil, errors.New("不支持的历史导出格式")
	}
	if err != nil {
		return nil, err
	}
	if len(data) > MaxBytes {
		return nil, errors.New("导出超过 64 MiB，请缩小搜索范围")
	}
	return data, nil
}
