package config

import "testing"

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
