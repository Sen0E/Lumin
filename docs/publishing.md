# 源码维护与 GitHub 发布

## 仓库范围

本仓库维护 Lumin 宿主、JSON-stdio 插件 SDK、打包工具、插件源码、测试及开发文档。旧应用参考源码与功能迁移对照已移除，运行和构建不依赖它们。

- `main.go` / `internal/`：桌面入口、业务模块、平台原生能力。
- `pkg/pluginapi/`：插件协议与 Go SDK。
- `cmd/lumin-plugin-pack/`：`.luminext` 打包命令。
- `plugins/`：彩云翻译与离线 OCR 插件，分别保留来源和许可。
- `examples/plugins/`：供第三方开发者参考的 LibreTranslate 示例。
- `resources/icon.png`：构建所需图标；不要删除。
- `scripts/`：可重建插件和平台发布包的脚本。
- `docs/`：架构、开发、交互、插件协议、验收与数据维护说明。

不将构建产物、模型/运行库压缩包、开发缓存、真实配置、备份、日志、编辑器配置或凭据放入源码提交。`.gitignore` 已覆盖这些本地文件。`plugins/ppocrv6-tiny/assets.lock.json` 是资源下载的固定版本和校验和，必须提交；模型和运行库由脚本下载与校验。

## 本地检查

```bash
go mod download
CGO_ENABLED=0 go test ./...
go vet ./...
go tool mygo vet .
git diff --check
git status --short
```

测试默认离线运行；真实 OCR、网络服务和桌面集成验收按 [验证记录](verification.md) 及对应模块文档执行。首次构建需要可用网络下载 Go 模块及 OCR 资源。

## 构建发布包

```bash
./scripts/build-release.sh linux/amd64
./scripts/build-release.sh windows/amd64
```

脚本先准备对应平台的 OCR 插件，然后复制为 `resources/<platform>/bundled-ocr.luminext` 再构建，确保发布包包含默认 OCR。直接 `go tool mygo build` 不会自动下载缺失的 OCR 资源。构建和下载结果均在忽略范围内，可放入 GitHub Releases，而不是源码目录。

版本使用根目录 `mygo.json`，同时更新界面版本、README、CHANGELOG 和功能状态。打包后检查实际平台架构、随包 OCR、插件许可证及源码附件。

## 提交前检查

`.gitignore` 只影响未跟踪文件。若本地开发工具目录已被跟踪，可用 `git rm -r --cached .agents` 从索引移除，文件仍保留在本机。随后检查：

```bash
git add -A
git diff --cached --stat
git diff --cached --name-only
git ls-files -ci --exclude-standard
```

最后一条列出“已跟踪但被忽略”的文件，应确认没有缓存、配置或构建产物。源码提交应包含当前全部实现、测试、图标、清单、构建脚本与文档；不要仅提交清理动作而漏掉此前未跟踪的新模块。

彩云插件的上游来源、GPL 许可证和 NOTICE，以及 OCR 模型、推理库的许可证均属于需要保留的分发材料。根项目的发布许可证应由维护者明确选择；仓库清理不替维护者重新授权代码，也不改变各插件现有许可。

如果之前已将配置、凭据或大体积资源提交到历史，新提交中的删除或忽略不会清除旧提交。发布前可用 `git log --all --stat` 查看历史；是否重建初始历史由维护者决定，本轮不改写提交历史，也不自动上传。
