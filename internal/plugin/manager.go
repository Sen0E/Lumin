package plugin

import (
	"archive/zip"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
)

const maxArchive = 128 << 20
const maxExpanded = 256 << 20
const maxFile = 64 << 20

type Manager struct {
	root string
	mu   sync.RWMutex
}

func New(root string) (*Manager, error) {
	var err error
	root, err = filepath.Abs(root)
	if err != nil {
		return nil, err
	}
	if err := os.MkdirAll(root, 0700); err != nil {
		return nil, err
	}
	// Normalize trusted storage ancestors (e.g. macOS /var -> /private/var
	// and Windows directory aliases). Runtime checks must reject links inside
	// an installed plugin, not aliases in the user's chosen storage location.
	root, err = filepath.EvalSymlinks(root)
	if err != nil {
		return nil, err
	}
	return &Manager{root: root}, nil
}

// Inspect validates every archive entry before any extraction or execution.
func Inspect(file string) (Manifest, error) {
	r, m, err := openArchive(file)
	if r != nil {
		_ = r.Close()
	}
	return m, err
}

func openArchive(file string) (*zip.ReadCloser, Manifest, error) {
	var m Manifest
	if !strings.EqualFold(filepath.Ext(file), ".luminext") {
		return nil, m, errors.New("请选择 .luminext 插件包")
	}
	info, err := os.Stat(file)
	if err != nil {
		return nil, m, err
	}
	if !info.Mode().IsRegular() || info.Size() > maxArchive {
		return nil, m, errors.New("插件包须为不超过 128 MiB 的普通文件")
	}
	r, err := zip.OpenReader(file)
	if err != nil {
		return nil, m, err
	}
	fail := func(err error) (*zip.ReadCloser, Manifest, error) { _ = r.Close(); return nil, m, err }
	if len(r.File) > 128 {
		return fail(errors.New("插件包文件数量超出限制"))
	}
	seen := map[string]bool{}
	files := map[string]*zip.File{}
	var total uint64
	for _, f := range r.File {
		name := strings.TrimSuffix(f.Name, "/")
		if !safePath(name) || seen[strings.ToLower(name)] || (!f.Mode().IsRegular() && !f.FileInfo().IsDir()) {
			return fail(errors.New("插件包包含不安全路径、重复文件或符号链接"))
		}
		seen[strings.ToLower(name)] = true
		if f.UncompressedSize64 > maxFile || total > maxExpanded-f.UncompressedSize64 {
			return fail(errors.New("插件解压大小超出限制"))
		}
		total += f.UncompressedSize64
		if !f.FileInfo().IsDir() {
			files[name] = f
		}
	}
	manifest := files["plugin.json"]
	if manifest == nil || manifest.UncompressedSize64 > 65536 {
		return fail(errors.New("缺少有效的 plugin.json"))
	}
	f, err := manifest.Open()
	if err != nil {
		return fail(err)
	}
	data, err := io.ReadAll(io.LimitReader(f, 65537))
	_ = f.Close()
	if err != nil || len(data) > 65536 {
		return fail(errors.New("读取插件清单失败"))
	}
	if err := json.Unmarshal(data, &m); err != nil {
		return fail(errors.New("plugin.json 不是有效 JSON"))
	}
	if err := m.Validate(); err != nil {
		return fail(err)
	}
	for _, name := range m.Entrypoints {
		if files[name] == nil {
			return fail(fmt.Errorf("缺少平台入口 %s", name))
		}
	}
	return r, m, nil
}

func (m *Manager) Install(file string) (Manifest, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	r, manifest, err := openArchive(file)
	if err != nil {
		return manifest, err
	}
	defer r.Close()
	dest := filepath.Join(m.root, manifest.Ref())
	if _, err := os.Lstat(dest); err == nil {
		return manifest, errors.New("此版本已安装；升级请使用新的版本号")
	} else if !errors.Is(err, os.ErrNotExist) {
		return manifest, err
	}
	stage, err := os.MkdirTemp(m.root, ".install-*")
	if err != nil {
		return manifest, err
	}
	defer os.RemoveAll(stage)
	for _, entry := range r.File {
		name := filepath.Join(stage, filepath.FromSlash(entry.Name))
		if entry.FileInfo().IsDir() {
			if err := os.MkdirAll(name, 0700); err != nil {
				return manifest, err
			}
			continue
		}
		if err := os.MkdirAll(filepath.Dir(name), 0700); err != nil {
			return manifest, err
		}
		in, err := entry.Open()
		if err != nil {
			return manifest, err
		}
		mode := os.FileMode(0600)
		for _, p := range manifest.Entrypoints {
			if p == entry.Name {
				mode = 0700
			}
		}
		out, err := os.OpenFile(name, os.O_WRONLY|os.O_CREATE|os.O_EXCL, mode)
		if err != nil {
			_ = in.Close()
			return manifest, err
		}
		n, copyErr := io.Copy(out, io.LimitReader(in, maxFile+1))
		closeErr := out.Close()
		_ = in.Close()
		if copyErr != nil {
			return manifest, copyErr
		}
		if closeErr != nil {
			return manifest, closeErr
		}
		if n > maxFile || uint64(n) != entry.UncompressedSize64 {
			return manifest, errors.New("插件文件大小不一致")
		}
	}
	if err := os.Rename(stage, dest); err != nil {
		return manifest, err
	}
	return manifest, nil
}

func (m *Manager) load(ref string) (Manifest, error) {
	var manifest Manifest
	if !ValidRef(ref) {
		return manifest, errors.New("无效插件标识")
	}
	dir := filepath.Join(m.root, ref)
	info, err := os.Lstat(dir)
	if err != nil {
		return manifest, err
	}
	if !info.IsDir() || info.Mode()&os.ModeSymlink != 0 {
		return manifest, errors.New("插件目录不是普通目录")
	}
	f, err := os.Open(filepath.Join(dir, "plugin.json"))
	if err != nil {
		return manifest, err
	}
	defer f.Close()
	data, err := io.ReadAll(io.LimitReader(f, 65537))
	if err != nil || len(data) > 65536 {
		return manifest, errors.New("读取插件清单失败")
	}
	if err := json.Unmarshal(data, &manifest); err != nil {
		return manifest, err
	}
	if err := manifest.Validate(); err != nil {
		return manifest, err
	}
	if manifest.Ref() != ref {
		return manifest, errors.New("插件目录与清单标识不匹配")
	}
	return manifest, nil
}

func (m *Manager) List() ([]Manifest, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	entries, err := os.ReadDir(m.root)
	if err != nil {
		return nil, err
	}
	var result []Manifest
	for _, e := range entries {
		if !ValidRef(e.Name()) {
			continue
		}
		manifest, err := m.load(e.Name())
		if err != nil {
			return result, fmt.Errorf("插件 %s 损坏：%w", e.Name(), err)
		}
		result = append(result, manifest)
	}
	sort.Slice(result, func(i, j int) bool { return result[i].Ref() < result[j].Ref() })
	return result, nil
}

// Remove waits for active calls to finish; callers check saved and draft usage.
func (m *Manager) Remove(ref string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if !ValidRef(ref) {
		return errors.New("无效插件标识")
	}
	return os.RemoveAll(filepath.Join(m.root, ref))
}
