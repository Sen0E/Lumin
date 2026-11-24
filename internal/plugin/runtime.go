package plugin

import (
	"bytes"
	"context"
	"crypto/rand"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	"lumin/internal/domain"
	"lumin/pkg/pluginapi"
)

type Request = pluginapi.Request
type Response = pluginapi.Response

func (m *Manager) Call(ctx context.Context, ref string, request Request) (string, error) {
	if m == nil {
		return "", errors.New("插件管理器不可用")
	}
	m.mu.RLock()
	defer m.mu.RUnlock()
	manifest, err := m.load(ref)
	if err != nil {
		return "", err
	}
	if !manifest.Has(request.Operation) {
		return "", errors.New("插件不支持此操作")
	}
	entry, ok := manifest.Entrypoints[runtime.GOOS+"/"+runtime.GOARCH]
	if !ok {
		return "", errors.New("插件未提供当前系统和架构的可执行文件")
	}
	settings, err := manifest.Settings(request.Settings)
	if err != nil {
		return "", err
	}
	request.Protocol, request.ID, request.Settings = Protocol, rand.Text(), settings
	if request.Operation == "translate" {
		if err := (domain.Request{Text: request.Text, Source: request.Source, Target: request.Target}).Validate(); err != nil {
			return "", err
		}
	}
	if request.Operation == "recognize" {
		request.ImagePath, err = filepath.Abs(request.ImagePath)
		if err != nil {
			return "", err
		}
		info, err := os.Stat(request.ImagePath)
		if err != nil {
			return "", err
		}
		if !info.Mode().IsRegular() || info.Size() > 20<<20 {
			return "", errors.New("OCR 插件图片须为不超过 20 MiB 的普通文件")
		}
	}
	dir := filepath.Join(m.root, ref)
	exe := filepath.Join(dir, filepath.FromSlash(entry))
	// Reject symlinks even if the user modified an installed package afterwards.
	resolved, err := filepath.EvalSymlinks(exe)
	if err != nil {
		return "", err
	}
	absolute, err := filepath.Abs(exe)
	if err != nil {
		return "", err
	}
	if resolved != absolute {
		return "", errors.New("插件入口不能包含符号链接")
	}
	info, err := os.Stat(exe)
	if err != nil || !info.Mode().IsRegular() {
		return "", errors.New("插件入口不是普通可执行文件")
	}
	ctx, cancel := context.WithTimeout(ctx, 90*time.Second)
	defer cancel()
	data, err := json.Marshal(request)
	if err != nil {
		return "", err
	}
	cmd := exec.CommandContext(ctx, exe)
	cmd.Dir = dir
	cmd.Stdin = bytes.NewReader(append(data, '\n'))
	cmd.WaitDelay = time.Second
	cmd.Env = pluginEnv(dir, filepath.Dir(exe))
	out, stderr := &boundedOutput{limit: 2 << 20, cancel: cancel}, &boundedOutput{limit: 64 << 10, cancel: cancel}
	cmd.Stdout, cmd.Stderr = out, stderr
	err = runProcess(cmd)
	if out.exceeded || stderr.exceeded {
		return "", errors.New("插件输出超出限制")
	}
	if ctx.Err() != nil {
		return "", ctx.Err()
	}
	if err != nil {
		return "", errors.New("插件进程执行失败，请检查平台入口及运行依赖")
	}
	var reply Response
	if err := json.Unmarshal(out.Bytes(), &reply); err != nil {
		return "", errors.New("插件必须向 stdout 输出一个 JSON 响应，日志应写入 stderr")
	}
	if reply.Protocol != Protocol || reply.ID != request.ID {
		return "", errors.New("插件响应协议或请求 ID 不匹配")
	}
	if reply.Error != "" {
		return "", fmt.Errorf("插件错误：%s", truncate(reply.Error, 600))
	}
	if strings.TrimSpace(reply.Text) == "" {
		return "", errors.New("插件返回了空文本")
	}
	return strings.TrimSpace(reply.Text), nil
}

func (m *Manager) ValidateUse(ref, operation string, settings map[string]string) error {
	if m == nil {
		return errors.New("插件管理器不可用")
	}
	m.mu.RLock()
	defer m.mu.RUnlock()
	manifest, err := m.load(ref)
	if err != nil {
		return err
	}
	if !manifest.Has(operation) {
		return errors.New("插件不支持此操作")
	}
	if manifest.Entrypoints[runtime.GOOS+"/"+runtime.GOARCH] == "" {
		return errors.New("插件缺少当前平台的入口")
	}
	_, err = manifest.Settings(settings)
	return err
}

func truncate(s string, n int) string {
	r := []rune(s)
	if len(r) > n {
		return string(r[:n]) + "…"
	}
	return s
}

func pluginEnv(root, entryDir string) []string {
	// Helpers may be supplied by the plugin, never resolved from the host PATH.
	env := []string{"PATH=" + strings.Join([]string{entryDir, filepath.Join(root, "bin"), root}, string(os.PathListSeparator))}
	for _, key := range []string{"SystemRoot", "SYSTEMROOT", "WINDIR", "TEMP", "TMP", "TMPDIR", "LANG", "LC_ALL"} {
		if value, ok := os.LookupEnv(key); ok {
			env = append(env, key+"="+value)
		}
	}
	return env
}

type boundedOutput struct {
	bytes.Buffer
	limit    int
	exceeded bool
	cancel   context.CancelFunc
}

func (b *boundedOutput) Write(p []byte) (int, error) {
	if b.Len()+len(p) > b.limit {
		b.exceeded = true
		b.cancel()
		return 0, errors.New("output limit")
	}
	return b.Buffer.Write(p)
}

type Translator struct {
	Manager                       *Manager
	Ref, ProviderID, ProviderName string
	Settings                      map[string]string
}

func (t Translator) ID() string   { return t.ProviderID }
func (t Translator) Name() string { return t.ProviderName }
func (t Translator) Translate(ctx context.Context, r domain.Request) (string, error) {
	return t.Manager.Call(ctx, t.Ref, Request{Operation: "translate", Text: r.Text, Source: r.Source, Target: r.Target, Settings: t.Settings})
}

type Recognizer struct {
	Manager  *Manager
	Ref      string
	Settings map[string]string
}

func (r Recognizer) Recognize(ctx context.Context, path, languages string) (string, error) {
	return r.Manager.Call(ctx, r.Ref, Request{Operation: "recognize", ImagePath: path, Languages: languages, Settings: r.Settings})
}
