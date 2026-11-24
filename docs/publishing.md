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
./scripts/build-release.sh darwin/arm64
```

脚本先构建对应平台的 PP-OCRv6 tiny 和彩云两个插件，分别复制为 `resources/<platform>/bundled-ocr.luminext`、`bundled-caiyun.luminext` 后打包。OCR 首次启动自动配置，彩云随包提供且可从 Release 单独下载导入，需主动启用。Windows 在本机安装 NSIS 或由 MyGo 自动准备后生成安装程序；Linux 交叉编译 Windows 时需安装 NSIS，也可追加 `-skip-installer` 仅构建便携程序。macOS DMG 必须在 macOS 构建机生成。直接 `go tool mygo build` 不会自动下载缺失的 OCR 资源。构建和下载结果均在忽略范围内，可放入 GitHub Releases，而不是源码目录。

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

## GitHub Actions 自动发布

工作流：`.github/workflows/release.yml`。仅构建以下三个目标，不构建 Intel Mac：

| 系统 | Runner | 应用资源 | 独立插件资源 |
| --- | --- | --- | --- |
| Linux x64 | ubuntu-24.04 | DEB、RPM、tar.gz | 彩云、PP-OCRv6 tiny 各一个 .luminext |
| Windows x64 | windows-2025 | NSIS 安装 EXE、便携 ZIP | 彩云、PP-OCRv6 tiny 各一个 .luminext |
| macOS Apple Silicon | macos-15 arm64 | DMG，最低 macOS 14.0 | 彩云、PP-OCRv6 tiny 各一个 .luminext |

### 触发方式

1. 将当前所有实现、`.github` 工作流和脚本提交并推送到 GitHub。
2. 更新 `mygo.json`、界面版本及 CHANGELOG，提交版本变更。标签必须与 mygo.json 完全一致。当前版本示例（先提交版本变更，再创建标签）：

```bash
git tag -a v0.13.2 -m "Release v0.13.2"
git push origin v0.13.2
```

也可在 Actions 页面选择 Build and release → Run workflow，输入已存在的标签。手动运行只读取该标签源码，不创建或移动标签。不要给同一个发布版本改标签指向；修改源码后使用新版本。带 `-beta.1` / `-rc.1` 等版本后缀的标签自动发布为 Pre-release。

### 发布检查与权限

- 使用仓库自动提供的 GITHUB_TOKEN，无需个人访问令牌；只给最后的 release job `contents: write`，构建 job 只读源码。如果组织策略禁止 Actions 写 Release，需要仓库管理员允许此权限。
- 标签、mygo.json 版本与实际 checkout 提交必须一致；运行全量 Go 测试、静态检查、发布脚本单元测试。
- 三个平台各自编译两个插件和应用，再实际执行离线 OCR 样例与彩云本地协议测试；不会向彩云发送联网测试请求。macOS 同时验证 app 签名和 DMG。
- 校验每个插件平台入口、OCR 模型/运行库和翻译插件对应源码/许可。收集准确的 12 个资源（6 个应用包、6 个插件包），生成 SHA256SUMS。缺少或混入旧资源均拒绝发布。
- 全部检查通过后先创建草稿，上传完成才公开。上传失败会保留草稿；重新运行失败任务可继续上传同一草稿。已公开的 Release 不会被自动覆盖；修改请发布新版本。
- 同一个标签的工作流排队运行，不在发布中途自动取消。只有三平台及检查全部通过才运行发布任务；单个平台失败不创建不完整公开版本。

### 签名与本地维护

当前 macOS 使用 ad-hoc 签名，未配置 Developer ID 和 Apple 公证；下载到其他 Mac 时 Gatekeeper 可能阻止直接打开，不能当作已公证分发包。Windows EXE 同样未配置 Authenticode 证书。若后续提供开发者证书，应通过 GitHub Secrets 注入；不要把证书、私钥或口令提交到仓库。

`plugins/ppocrv6-tiny/assets.lock.json` 固定 macOS arm64 SDK 的 URL 与 SHA-256；构建时对 ZIP 内的插件入口与动态库进行本地签名。Python、curl 和 Go 都只用于构建，不进入插件运行依赖。

维护发布逻辑时运行：

```bash
python3 -m unittest discover -s scripts -p 'test_*.py' -v
bash -n scripts/build-release.sh scripts/build-plugins.sh
```

本地测试只验证当前宿主及可交叉编译的部分；首次 GitHub runner 完整运行记录仍是三平台安装包与系统签名验证的依据。

### 构建故障排查与修复后重试

- 报错找不到 `LICENSE`：OCR 脚本需要先 `go mod download -json <模块>` 获取当前 go.mod 中固定版本的模块目录，不能假设目标平台编译已下载全部跨平台依赖。当前脚本会显式获取 purego、x/image、x/sys 并检查许可证存在；不会在缺少许可时继续打包。
- Windows 报错 `Rel: can't make D: ... relative to C:`：GitHub runner 的临时目录和工作区位于不同磁盘属于正常布局。打包器允许不同卷上的输出，同时继续拒绝将输出放入插件源码目录；Windows 专属回归由 Windows runner 实际执行。
- 修改脚本后，先提交并推送修复。Actions 的已有运行和版本标签都指向原提交，直接重跑不会使用 main 上的新代码。推荐更新 mygo.json、界面版本及 CHANGELOG 后创建新版本标签（例如 v0.13.2）；同一提交因网络等临时故障失败才适合直接重跑。手动运行时同样需要填写包含修复的已有标签。

- 报错 `tag vX does not match mygo.json: vY`：标签指向的提交仍是旧版本。只更新 main 或只创建新标签都不够；必须先在源码中完成版本升级并提交，再创建标签。当前工作树版本为 0.13.2；已创建的旧标签不会跟随 main 移动。若标签在 prepare 阶段失败且从未发布 Release，可明确重新指向修复提交后再推送；已有公开 Release 的标签不应移动，应使用新版本。
