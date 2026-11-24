// Package backup defines a bounded, versioned JSON exchange format. It never
// unpacks paths or executes plugins from an imported document.
package backup

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/url"
	"os"
	"path/filepath"
	"time"

	"lumin/internal/config"
	"lumin/internal/domain"
)

const Version = 1
const MaxBytes = 64 << 20
const MaxEntries = 50000

type Document struct {
	Format          string         `json:"format"`
	Version         int            `json:"version"`
	CreatedAt       time.Time      `json:"created_at"`
	IncludesSecrets bool           `json:"includes_secrets"`
	Config          config.Config  `json:"config"`
	History         []domain.Entry `json:"history"`
}

func New(c config.Config, h []domain.Entry, secrets bool) Document {
	c = c.Clone()
	if !secrets {
		c.ServerEnabled, c.ServerToken = false, ""
		c.OCRSettings = nil // Unknown plugin fields may contain credentials.
		for i := range c.Providers {
			p := &c.Providers[i]
			p.APIKey, p.Settings = "", nil
			if u, err := url.Parse(p.Endpoint); err == nil {
				u.User = nil
				u.RawQuery = ""
				u.Fragment = ""
				p.Endpoint = u.String()
			} else {
				p.Endpoint = ""
			}
			p.Enabled = false // Require reviewing credentials before calling a service.
		}
	}
	return Document{Format: "lumin-backup", Version: Version, CreatedAt: time.Now().UTC(), IncludesSecrets: secrets, Config: c, History: h}
}

func (d Document) Validate() error {
	if d.Format != "lumin-backup" || d.Version != Version || d.CreatedAt.IsZero() {
		return errors.New("备份格式或版本无效")
	}
	if err := d.Config.Validate(); err != nil {
		return err
	}
	if len(d.History) > MaxEntries {
		return errors.New("备份历史条数超出限制")
	}
	ids := make(map[string]bool, len(d.History))
	for _, e := range d.History {
		if e.ID == "" || ids[e.ID] || e.CreatedAt.IsZero() || len(e.Results) > 16 {
			return errors.New("备份历史 ID、时间或结果数量无效")
		}
		ids[e.ID] = true
		if err := e.Request.Validate(); err != nil {
			return fmt.Errorf("备份历史无效：%w", err)
		}
	}
	// A document cannot claim to be redacted while carrying known secret fields.
	if !d.IncludesSecrets {
		if d.Config.ServerToken != "" || len(d.Config.OCRSettings) != 0 {
			return errors.New("备份的密钥声明与内容不符")
		}
		for _, p := range d.Config.Providers {
			if p.APIKey != "" || len(p.Settings) != 0 {
				return errors.New("备份的密钥声明与内容不符")
			}
		}
	}
	return nil
}

func Read(path string) (*Document, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	info, err := f.Stat()
	if err != nil {
		return nil, err
	}
	if !info.Mode().IsRegular() || info.Size() > MaxBytes {
		return nil, errors.New("请选择不超过 64 MiB 的备份文件")
	}
	b, err := io.ReadAll(io.LimitReader(f, MaxBytes+1))
	if err != nil {
		return nil, err
	}
	if len(b) > MaxBytes {
		return nil, errors.New("备份文件过大")
	}
	var d Document
	dec := json.NewDecoder(bytes.NewReader(b))
	dec.DisallowUnknownFields()
	if err := dec.Decode(&d); err != nil {
		return nil, fmt.Errorf("备份解析失败：%w", err)
	}
	if dec.Decode(new(any)) != io.EOF {
		return nil, errors.New("备份包含多余内容")
	}
	if err := d.Validate(); err != nil {
		return nil, err
	}
	return &d, nil
}

func Write(path string, d Document) error {
	if err := d.Validate(); err != nil {
		return err
	}
	b, err := json.MarshalIndent(d, "", "  ")
	if err != nil {
		return err
	}
	if len(b)+1 > MaxBytes {
		return errors.New("备份超过 64 MiB，未写入文件")
	}
	return AtomicWrite(path, append(b, '\n'))
}

// AtomicWrite keeps the destination intact until the new private file is synced.
func AtomicWrite(path string, data []byte) (err error) {
	f, err := os.CreateTemp(filepath.Dir(path), ".lumin-*.tmp")
	if err != nil {
		return err
	}
	defer func() { _ = f.Close(); _ = os.Remove(f.Name()) }()
	if _, err = f.Write(data); err != nil {
		return err
	}
	if err = f.Sync(); err != nil {
		return err
	}
	if err = f.Close(); err != nil {
		return err
	}
	return os.Rename(f.Name(), path)
}
