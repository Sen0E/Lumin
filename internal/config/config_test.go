package config

import (
	"encoding/json"
	"testing"
)

func TestValidation(t *testing.T) {
	for _, tc := range []struct {
		name   string
		change func(*Config)
	}{
		{"future schema", func(c *Config) { c.Version++ }},
		{"target auto", func(c *Config) { c.Target = "auto" }},
		{"timeout", func(c *Config) { c.TimeoutSeconds = 0 }},
		{"duplicate provider", func(c *Config) { c.Providers[1].ID = c.Providers[0].ID }},
		{"remote server", func(c *Config) {
			c.ServerEnabled = true
			c.ServerToken = "123456789012345678901234"
			c.ServerAddress = "0.0.0.0:60828"
		}},
		{"weak token", func(c *Config) { c.ServerEnabled = true }},
		{"URL credentials", func(c *Config) {
			c.Providers[1].Enabled = true
			c.Providers[1].Endpoint = "https://user:secret@host/api/chat"
		}},
		{"missing model", func(c *Config) { c.Providers[0].Enabled = true }},
	} {
		t.Run(tc.name, func(t *testing.T) {
			c := Default()
			tc.change(&c)
			if c.Validate() == nil {
				t.Fatal("expected validation error")
			}
		})
	}
	if err := Default().Validate(); err != nil {
		t.Fatal(err)
	}
}

func TestCloneIsIndependent(t *testing.T) {
	c := Default()
	draft := c.Clone()
	draft.Providers[0].Name = "changed"
	if c.Providers[0].Name == "changed" {
		t.Fatal("draft modified live config")
	}
}

func TestPluginConfigAndNestedClone(t *testing.T) {
	cfg := Default()
	cfg.Providers = []Provider{{ID: "one", Name: "Plugin", Kind: "plugin", Enabled: true, Plugin: "test.translate@1.0.0", Settings: map[string]string{"token": "saved"}}}
	cfg.OCRPlugin = "test.ocr@1.0.0"
	cfg.OCRSettings = map[string]string{"model": "saved"}
	if err := cfg.Validate(); err != nil {
		t.Fatal(err)
	}
	draft := cfg.Clone()
	draft.Providers[0].Settings["token"] = "draft"
	draft.OCRSettings["model"] = "draft"
	if cfg.Providers[0].Settings["token"] != "saved" || cfg.OCRSettings["model"] != "saved" {
		t.Fatal("draft aliases saved settings")
	}
	draft.ScreenshotShortcut = draft.InputShortcut
	if err := draft.Validate(); err == nil {
		t.Fatal("duplicate capture shortcut accepted")
	}
	draft = cfg.Clone()
	draft.Providers[0].Plugin = "../unsafe@1.0.0"
	if err := draft.Validate(); err == nil {
		t.Fatal("invalid plugin ref accepted")
	}
}

func TestPluginOCRNeedsNoSystemLanguageConfiguration(t *testing.T) {
	c := Default()
	if c.OCRLanguages != "" {
		t.Fatal("new config contains system OCR languages")
	}
	if err := c.Validate(); err != nil {
		t.Fatal(err)
	}
	c.OCRLanguages = "eng+chi_sim" // Legacy field still loads; it is not a fallback switch.
	c.OCRDisabled = true
	if err := c.Validate(); err != nil {
		t.Fatal("legacy OCR compatibility lost", err)
	}
}

func TestSelectionShortcutPreservesV1SavedKey(t *testing.T) {
	c := Default()
	if err := json.Unmarshal([]byte(`{"clipboard_shortcut":"Ctrl+Alt+Y"}`), &c); err != nil {
		t.Fatal(err)
	}
	if c.SelectionShortcut != "Ctrl+Alt+Y" {
		t.Fatal("legacy binding lost")
	}
	data, err := json.Marshal(c)
	if err != nil {
		t.Fatal(err)
	}
	var raw map[string]any
	if err = json.Unmarshal(data, &raw); err != nil {
		t.Fatal(err)
	}
	if raw["clipboard_shortcut"] != "Ctrl+Alt+Y" {
		t.Fatal("v1 backup compatibility changed")
	}
	if err = json.Unmarshal([]byte(`{"clipboard_shortcut":""}`), &c); err != nil || c.SelectionShortcut != "" {
		t.Fatal("disabled shortcut re-enabled")
	}
}

func TestDisplayPreferencesLegacyDefaultsAndBounds(t *testing.T) {
	c := Default()
	c.Appearance = ""
	c.TranslationWidth = 0
	c.TranslationHeight = 0
	c.TranslationFontSize = 0
	if err := c.Validate(); err != nil {
		t.Fatal(err)
	}
	w, h := c.TranslationSize()
	if w != 350 || h != 420 || c.ResultFontSize() != 16 || c.KeepTranslationOnBlur {
		t.Fatal("legacy defaults changed")
	}
	for _, change := range []func(*Config){func(c *Config) { c.Appearance = "invalid" }, func(c *Config) { c.TranslationWidth = 319 }, func(c *Config) { c.TranslationHeight = 359 }, func(c *Config) { c.TranslationFontSize = 25 }} {
		bad := c.Clone()
		change(&bad)
		if bad.Validate() == nil {
			t.Fatal("invalid display preference accepted")
		}
	}
}
