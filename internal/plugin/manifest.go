// Package plugin manages immutable .luminext packages and the JSON stdio ABI.
package plugin

import (
	"errors"
	"fmt"
	"path"
	"regexp"
	"slices"
	"strings"

	"lumin/pkg/pluginapi"
)

const Protocol = 1

type Field struct {
	Key      string `json:"key"`
	Label    string `json:"label"`
	Default  string `json:"default,omitempty"`
	Required bool   `json:"required,omitempty"`
	Secret   bool   `json:"secret,omitempty"`
}

type Manifest struct {
	Schema       int               `json:"schema"`
	ID           string            `json:"id"`
	Name         string            `json:"name"`
	Version      string            `json:"version"`
	Author       string            `json:"author,omitempty"`
	Description  string            `json:"description,omitempty"`
	Capabilities []string          `json:"capabilities"`
	Entrypoints  map[string]string `json:"entrypoints"`
	Fields       []Field           `json:"settings,omitempty"`
}

var fieldKey = regexp.MustCompile(`^[a-zA-Z][a-zA-Z0-9_]{0,63}$`)

func ValidRef(ref string) bool {
	return pluginapi.ValidRef(ref)
}
func (m Manifest) Ref() string                { return m.ID + "@" + m.Version }
func (m Manifest) Has(capability string) bool { return slices.Contains(m.Capabilities, capability) }

func safePath(name string) bool {
	if name == "" || name == "." || name == ".." || path.Clean(name) != name || strings.HasPrefix(name, "/") || strings.HasPrefix(name, "../") || strings.ContainsAny(name, "\\:\x00") {
		return false
	}
	for _, part := range strings.Split(name, "/") {
		if strings.HasSuffix(part, ".") || strings.HasSuffix(part, " ") {
			return false
		}
		base, _, _ := strings.Cut(strings.ToUpper(part), ".")
		if base == "CON" || base == "PRN" || base == "AUX" || base == "NUL" || base == "CONIN$" || base == "CONOUT$" {
			return false
		}
		if len(base) == 4 && (strings.HasPrefix(base, "COM") || strings.HasPrefix(base, "LPT")) && base[3] >= '0' && base[3] <= '9' {
			return false
		}
	}
	return true
}

func (m Manifest) Validate() error {
	if m.Schema != Protocol {
		return errors.New("不支持的插件协议版本")
	}
	if !ValidRef(m.Ref()) || strings.TrimSpace(m.Name) == "" || len(m.Name) > 160 {
		return errors.New("插件 ID、版本或名称无效")
	}
	if len(m.Capabilities) == 0 || len(m.Capabilities) > 2 {
		return errors.New("插件需要声明 translate 或 recognize 能力")
	}
	seen := map[string]bool{}
	for _, c := range m.Capabilities {
		if (c != "translate" && c != "recognize") || seen[c] {
			return errors.New("未知或重复的插件能力")
		}
		seen[c] = true
	}
	if len(m.Entrypoints) == 0 || len(m.Entrypoints) > 6 {
		return errors.New("插件缺少平台入口")
	}
	for target, file := range m.Entrypoints {
		switch target {
		case "linux/amd64", "linux/arm64", "windows/amd64", "windows/arm64", "darwin/amd64", "darwin/arm64":
		default:
			return fmt.Errorf("不支持的插件平台 %s", target)
		}
		if !safePath(file) || file == "plugin.json" {
			return errors.New("插件入口必须是包内的普通文件路径")
		}
	}
	if len(m.Fields) > 32 {
		return errors.New("插件设置最多 32 项")
	}
	seen = map[string]bool{}
	for _, f := range m.Fields {
		if !fieldKey.MatchString(f.Key) || seen[f.Key] || strings.TrimSpace(f.Label) == "" || len(f.Label) > 160 || len(f.Default) > 65536 {
			return errors.New("插件设置定义无效")
		}
		seen[f.Key] = true
	}
	return nil
}

func (m Manifest) Settings(values map[string]string) (map[string]string, error) {
	out := map[string]string{}
	for _, f := range m.Fields {
		v, ok := values[f.Key]
		if !ok {
			v = f.Default
		}
		if len(v) > 65536 {
			return nil, fmt.Errorf("%s 超出长度限制", f.Label)
		}
		if f.Required && strings.TrimSpace(v) == "" {
			return nil, fmt.Errorf("请填写插件设置：%s", f.Label)
		}
		out[f.Key] = v
	}
	return out, nil
}
