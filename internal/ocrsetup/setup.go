// Package ocrsetup provisions the official OCR plugin shipped with a release.
package ocrsetup

import (
	"errors"
	"fmt"
	"os"
	"runtime"

	"lumin/internal/config"
	"lumin/internal/plugin"
	"lumin/internal/storage"
)

const DefaultRef = "lumin.ppocrv6-tiny@1.0.0"
const LegacyToolRef = "example.tesseract@1.0.0"

// Ensure preserves selected engines and explicit disable choices, except the
// retired tool-based example. It installs the pinned bundle without executing
// it, then persists its selection.
func Ensure(store *storage.Store, manager *plugin.Manager, cfg config.Config, archive string) (config.Config, error) {
	if cfg.OCRDisabled || cfg.OCRPlugin != "" && cfg.OCRPlugin != LegacyToolRef {
		return cfg, nil
	}
	list, err := manager.List()
	if err != nil {
		return cfg, err
	}
	found := false
	for _, m := range list {
		if m.Ref() == DefaultRef {
			found = true
			break
		}
	}
	if !found {
		if _, err := os.Stat(archive); errors.Is(err, os.ErrNotExist) {
			return cfg, nil
		} else if err != nil {
			return cfg, err
		}
		m, err := plugin.Inspect(archive)
		if err != nil {
			return cfg, err
		}
		if m.Ref() != DefaultRef || !m.Has("recognize") || m.Entrypoints[runtime.GOOS+"/"+runtime.GOARCH] == "" {
			return cfg, errors.New("随包 OCR 插件标识或平台不匹配")
		}
		if _, err = manager.Install(archive); err != nil {
			return cfg, err
		}
	}
	if err = manager.ValidateUse(DefaultRef, "recognize", nil); err != nil {
		return cfg, err
	}
	next := cfg.Clone()
	next.OCRPlugin = DefaultRef
	next.OCRSettings = map[string]string{"threads": "4", "max_side": "960"}
	if err = store.SaveConfig(next); err != nil {
		return cfg, fmt.Errorf("保存默认 OCR 引擎：%w", err)
	}
	return next, nil
}
