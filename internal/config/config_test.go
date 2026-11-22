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
