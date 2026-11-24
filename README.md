# Lumin

基于 Go + MyGo 的独立原生桌面翻译工具。窗口使用 MyGo `ui`，无需 React、Node、Tauri 或 WebView。

当前版本是 **0.13.1**。后续以现有体验、性能和稳定性为重点；详细状态见 [功能清单](docs/features.md)。

## 已实现

- 原生中文窗口：主窗口放配置与历史管理；翻译小窗口处理当前翻译；识别窗口提供图片预览、结果编辑、复制、重新识别与转翻译，跟随系统明暗主题。
- 关闭窗口隐藏到后台，托盘、快捷键和 HTTP 继续可用；从托盘或设置窗口明确退出。
- 单个服务/插件使用草稿测试、可取消测试、服务上移下移排序；默认语言、超时和历史上限可在界面设置。
- 多服务并行翻译、逐个显示结果、取消与超时、旧结果隔离、复制译文。
- Chat Completions 兼容协议、Ollama、DeepL 官方 API、LibreTranslate；可添加多个同类服务。
- 选中文字快捷翻译、原生复制/粘贴、全局快捷键、系统托盘。
- 导入图片或截图框选识字，支持识别后自动翻译；截图只取鼠标所在显示器并在该屏框选，发布包随附 PP-OCRv6 tiny 离线插件，首次启动自动配置。
- `.luminext` 插件安装、配置、卸载，支持翻译和 OCR；提供 Go SDK、打包工具和示例；已移植彩云小译和 PP-OCRv6 tiny 离线识别插件。
- 本地历史记录、搜索、收藏生词本、单条确认删除、清理普通历史和 CSV/JSON 筛选导出。
- 可选的带令牌鉴权的回环 HTTP 外部调用。
- 配置与历史原子保存，配置版本检查，自动化测试与维护文档。

## 启动

需要 **Go 1.27.4 或以上**，MyGo 固定为 **v0.10.0**。Go 版本沿用原工程设置。

```bash
go mod download
go tool mygo doctor
go tool mygo dev
```

也可以使用 `go run .`。Linux 运行时需要 GTK 3 和系统字体；托盘需要 GTK3 版本的 libayatana-appindicator 或 libappindicator，安装后须完整退出旧进程并重启。无需安装 WebKitGTK。具体检查以 `mygo doctor` 输出为准。

首次启动所有翻译服务均未启用，应用不会自动上传文本：

1. 进入「服务设置 → 翻译服务」。选择服务，填写**完整接口地址**及模型名称或密钥。
2. 可先点击「测试此服务」，会发送固定测试文本到该接口或插件；测试不保存草稿或历史。
3. 勾选启用，点击「保存服务」。
4. 从托盘「打开翻译」或按 `Ctrl+Alt+Space` 打开小窗口，输入文本，点击「翻译文本」或按 `Ctrl+Enter`（macOS 为 `⌘+Enter`）。

使用本地 Ollama 时，先自行启动 Ollama 并安装目标模型。模板中的 `qwen2.5:7b` 只是可修改的模型名，Lumin 不下载模型。接口地址示例为 `http://127.0.0.1:11434/api/chat`。

DeepL 免费 API 使用 `https://api-free.deepl.com/v2/translate`；其他账户按服务商地址配置。兼容服务填写完整的 `/chat/completions` 地址，不自动追加路径。密钥环境变量优先于配置中直接填写的密钥。各服务支持的语言和模型以其实际接口为准。

## 常用操作

| 操作           | 默认方式                                           |
| -------------- | -------------------------------------------------- |
| 打开翻译小窗口 | `Ctrl+Alt+Space`；macOS 用 `Cmd` 替代 `Ctrl` |
| 选中文字翻译   | `Ctrl+Alt+T`；macOS 用 `Cmd` 替代 `Ctrl`     |
| 窗口内翻译     | `Ctrl+Enter` / `⌘+Enter`                      |
| 取消任务       | `Esc` 或「取消」                                 |
| 截图翻译       | `Ctrl+Alt+S` / `Cmd+Alt+S`，或托盘             |
| 图片 OCR       | 托盘「图片识字」选择图片                           |
| 生词本         | 主窗口「历史记录」中收藏，勾选「仅显示收藏」       |
| 后台运行       | 关闭任意窗口；翻译小窗口空闲时也可按 `Esc` 隐藏  |
| 打开设置       | 托盘「偏好设置」或再次启动 Lumin                   |
| 退出           | 托盘「退出 Lumin」或设置窗口左下角「退出 Lumin」   |

翻译小窗口默认 350×420，最小 320×360，可在翻译设置调整默认宽高和译文字号；识别窗口默认 600×700，最小 480×560；三个窗口分别记住尺寸和位置。未置顶时点击外部自动隐藏翻译窗，可在翻译设置关闭该行为；置顶后保持显示。关闭或自动隐藏翻译窗口会取消本次翻译/截图翻译，保留已有内容；关闭识别窗口只取消独立识别任务；关闭设置窗口保留未保存草稿和正在运行的服务测试。取词快捷键在后台有效，选中原应用文字即可触发。托盘不可用时，重新运行同一应用会唤起已有实例的设置窗口，可从那里退出。后台常驻不包含开机启动。

Wayland 的全局快捷键依赖桌面门户及应用桌面条目，通常需要安装打包后的应用并完成桌面授权。取词需要目标应用提供辅助功能接口；窗口内仍可手动粘贴并点击翻译。

Linux/Windows x64 发布包自带 PP-OCRv6 tiny 离线 OCR 插件，首次启动安装到用户数据目录并设为默认引擎，无需 Python、Tesseract 或额外模型下载。已有引擎选择或主动停用会保留；历史 Tesseract 示例引用会迁移到随包引擎。「服务设置 → OCR 服务」统一展示当前引擎，支持切换插件、编辑参数、停用及测试当前草稿。源码运行若没有随包资源，请先手动安装 `.luminext`，应用不会调用系统 OCR 回退。截图权限与多屏行为见 [截图说明](docs/screenshots.md)。

「服务设置 → 插件」页面安装 `.luminext` 后，添加翻译服务或选择用于 OCR，再配置并保存。插件协议与示例见 [插件开发文档](docs/plugins.md)。原生插件以当前用户权限运行，仅启用可信来源。

彩云小译的安装、默认参数和开发说明见 [彩云插件](plugins/caiyun/README.md)。在「服务设置 → 插件」中安装 `build/plugins/lumin.caiyun-1.0.0-linux-amd64.luminext`，添加翻译服务、启用并保存即可；默认参数、来源与许可说明见插件文档。

窗口职责及功能位置见 [窗口功能说明](docs/window-map.md)。常规设置页可点击「检查托盘」，查看创建/桌面注册状态；KDE 将图标折叠时需展开隐藏图标。

## 数据

默认保存到 `os.UserConfigDir()/lumin`，Linux 通常为 `~/.config/lumin`：

- `state.json`：配置、历史和收藏的单文件原子快照。
- `backups/`：恢复前自动生成的完整备份（包含明文密钥）。
- 旧版 `config.json` / `history.json` 在首次成功写入时迁移，原文件保留；此后仅使用 `state.json`。
- `plugins/<id>@<version>/`：已安装插件，版本独立保存。

开发时可以使用独立目录：

```bash
LUMIN_CONFIG_DIR=/tmp/lumin-dev go run .
```

配置中的 API Key 是本地明文，Unix 上新文件权限为 `0600`、新目录为 `0700`；可改用环境变量以免把密钥写入配置。历史含原文和译文，可以关闭保存。不要提交真实配置文件。

## 开发与打包

```bash
CGO_ENABLED=0 go test ./...
go test -race ./internal/... ./pkg/... ./plugins/...
go vet ./...
go tool mygo vet .
go tool mygo build -platform linux/amd64
```

Linux 产物在 `build/linux-amd64/`。交叉编译及发布验收见 [开发手册](docs/development.md)。

- [截图框选、平台依赖与验收](docs/screenshots.md)
- [插件协议、SDK 与示例](docs/plugins.md)
- [功能清单与后续开发计划](docs/features.md)
- [架构与目录职责](docs/architecture.md)
- [开发、扩展、调试和发布](docs/development.md)
- [验证记录与平台限制](docs/verification.md)
- [变更记录](CHANGELOG.md)
- [源码整理与 GitHub 发布](docs/publishing.md)

截图识字、图片识字和 HTTP `/ocr_recognize` 进入独立识别窗口；截图翻译和 `/ocr_translate` 仍直接进入翻译窗口。识别窗口可编辑结果后点击「翻译文本」，或用「重新识别」对保留的图片再次调用当前已保存的 OCR 引擎。

### 备份与恢复

主窗口「备份设置」可导出 `.luminbackup`，或选择文件预览后覆盖恢复。默认不包含 API Key、外部调用令牌及插件参数；勾选「包含密钥和插件参数」才导出完整配置。恢复前会自动保存旧数据，可再次导入该文件撤回恢复。插件程序与环境变量值不包含在备份中。详见 [备份维护文档](docs/backups.md)。

### 历史删除与导出

主窗口「历史记录」中可逐条删除；收藏记录需明确确认删除收藏。选择 JSON 或 CSV 后点击「导出当前结果」，导出点击时的搜索和收藏筛选结果；只导出收藏可先勾选「仅显示收藏」。JSON 保留完整历史结构，CSV 每个服务结果一行，保留原文、译文中的换行与 Unicode。导出不包含应用配置和密钥字段；这是供阅读和其他工具使用的数据导出，恢复数据请使用备份设置。详见 [历史维护文档](docs/history.md)。

### 发布包与默认 OCR

使用 `make build` 或 `scripts/build-release.sh linux/amd64`；Windows 使用 `scripts/build-release.sh windows/amd64`。脚本构建固定版本的 OCR 插件，放入 MyGo 平台资源目录后打包。首次构建需下载已锁定的模型和运行库，运行安装包不联网下载模型。直接执行 `go tool mygo build` 不负责准备缺失的 OCR 资源；macOS/ARM 默认模型包暂未提供。详见 [OCR 维护说明](docs/ocr.md)。

截图框选采用无边框全屏画布，左键拖拽后松开即可识别，右键或 Esc 取消；无需点击确认按钮。尺寸提示浮在截图上，不占用顶部空间。误点或小于 2×2 原图像素的区域不提交，可直接重选。

运行时不调用本机 Tesseract、grim、maim、PowerShell、screencapture 或 taskkill。截图使用系统原生接口，插件工具和模型随包提供；操作系统桌面服务、基础运行库和权限要求见 [运行依赖说明](docs/runtime-dependencies.md)。

选中其他应用中的文字后按取词快捷键，读取成功再弹出翻译小窗并自动翻译，无需复制；取词不读写普通剪贴板。Linux Wayland 优先读取原生选区，兼容发布选区的 VS Code/Monaco；该通道不可用时再尝试辅助功能接口。两种接口都不可用时提示取词失败，可使用截图翻译。平台限制见 [选中文字翻译](docs/selection-translation.md)。

常规设置支持跟随系统、浅色、深色，保存后统一应用到所有业务窗口。翻译窗可收起原文、阅读和复制多服务译文；窗口模式、尺寸与颜色规则见 [交互与外观设计](docs/ux.md)。
