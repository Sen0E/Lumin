# Lumin

基于 Go + MyGo 的原生桌面翻译工具，以本仓库中的 Pot Desktop 3.0.7 为功能迁移参考，优先维护 Linux，并保持 Windows/macOS 编译兼容。窗口使用 MyGo `ui`，无需 React、Node、Tauri 或 WebView。

当前版本是 **0.4.0，支持后台常驻与独立翻译小窗口**，不是 Pot 全功能的等价替换。详细状态见 [迁移清单](docs/migration.md)。

## 已实现

- 原生中文双窗口：主窗口只放翻译服务、插件与设置；独立小窗口提供翻译、历史与生词本，跟随系统明暗主题。
- 关闭窗口隐藏到后台，托盘、快捷键和 HTTP 继续可用；从托盘或设置窗口明确退出。
- 单个服务/插件使用草稿测试、可取消测试、服务上移下移排序；默认语言、超时和历史上限可在界面设置。
- 多服务并行翻译、逐个显示结果、取消与超时、旧结果隔离、复制译文。
- Chat Completions 兼容协议、Ollama、DeepL 官方 API、LibreTranslate；可添加多个同类服务。
- 原生剪贴板粘贴、主动开启的剪贴板监听、全局快捷键、系统托盘。
- 导入图片或截图框选识字，支持识别后自动翻译；默认使用本机 Tesseract。
- `.luminext` 插件安装、配置、卸载，支持翻译和 OCR；提供 Go SDK、打包工具和示例；已移植彩云小译插件。
- 本地历史记录、搜索、收藏生词本、清理普通历史。
- 可选的带令牌鉴权的回环 HTTP 外部调用。
- 配置与历史原子保存，配置版本检查，自动化测试与维护文档。

## 启动

需要 **Go 1.27.4 或以上**，MyGo 固定为 **v0.10.0**。Go 版本沿用原工程设置。

```bash
go mod download
go tool mygo doctor
go tool mygo dev
```

也可以使用 `go run .`。Linux 运行时需要 GTK 3 和系统字体；托盘按桌面环境需要 AppIndicator。无需安装 WebKitGTK。具体检查以 `mygo doctor` 输出为准。

首次启动所有翻译服务均未启用，应用不会自动上传文本：

1. 进入「翻译服务」。选择服务，填写**完整接口地址**及模型名称或密钥。
2. 可先点击「测试此服务」，会发送固定测试文本到该接口或插件；测试不保存草稿或历史。
3. 勾选启用，点击「保存服务」。
4. 从托盘「打开翻译」或按 `Ctrl+Alt+Space` 打开小窗口，输入文本，点击「翻译文本」或按 `Ctrl+Enter`（macOS 为 `⌘+Enter`）。

使用本地 Ollama 时，先自行启动 Ollama 并安装目标模型。模板中的 `qwen2.5:7b` 只是可修改的模型名，Lumin 不下载模型。接口地址示例为 `http://127.0.0.1:11434/api/chat`。

DeepL 免费 API 使用 `https://api-free.deepl.com/v2/translate`；其他账户按服务商地址配置。兼容服务填写完整的 `/chat/completions` 地址，不自动追加路径。密钥环境变量优先于配置中直接填写的密钥。各服务支持的语言和模型以其实际接口为准。

## 常用操作

| 操作 | 默认方式 |
| --- | --- |
| 打开翻译小窗口 | `Ctrl+Alt+Space`；macOS 用 `Cmd` 替代 `Ctrl` |
| 翻译剪贴板 | `Ctrl+Alt+T`；macOS 用 `Cmd` 替代 `Ctrl` |
| 窗口内翻译 | `Ctrl+Enter` / `⌘+Enter` |
| 取消任务 | `Esc` 或「取消」 |
| 截图翻译 | `Ctrl+Alt+S` / `Cmd+Alt+S`，或按钮、托盘 |
| 图片 OCR | 「图片识字」选择图片 |
| 生词本 | 历史记录中点击「收藏」 |
| 后台运行 | 关闭任意窗口；翻译小窗口空闲时也可按 `Esc` 隐藏 |
| 打开设置 | 托盘「偏好设置」、小窗口「设置」，或再次启动 Lumin |
| 退出 | 托盘「退出 Lumin」或设置窗口左下角「退出 Lumin」 |

翻译小窗口默认 520×620，最小 460×480；两个窗口分别记住尺寸和位置。关闭翻译窗口会取消本次翻译/OCR；关闭设置窗口保留未保存草稿和正在运行的服务测试。剪贴板监听移到设置页，开启后本次运行有效，关闭窗口后仍可触发翻译。托盘不可用时，重新运行同一应用会唤起已有实例的设置窗口，可从那里退出。后台常驻不包含开机启动。

Wayland 的全局快捷键依赖桌面门户及应用桌面条目，通常需要安装打包后的应用并完成桌面授权。剪贴板监听也受桌面支持限制；窗口内粘贴始终可以作为备用入口。

OCR 需要安装 `tesseract` 以及 `eng`、`chi_sim` 语言数据。可以在设置中将 OCR 语言改为已经安装的语言，使用 `tesseract --list-langs` 查看。也可安装 OCR 插件替换内置引擎。截图框选的系统依赖、权限和多显示器行为见 [截图说明](docs/screenshots.md)。Linux 优先使用 XDG Screenshot 门户。

「插件」页面安装 `.luminext` 后，添加翻译服务或选择用于 OCR，再配置并保存。新格式不直接运行 Pot `.potext`；迁移依据、协议与示例见 [插件开发文档](docs/plugins.md)。原生插件以当前用户权限运行，仅启用可信来源。

彩云小译的安装、默认参数和开发说明见 [彩云插件](plugins/caiyun/README.md)。在「插件」中安装 `build/plugins/lumin.caiyun-1.0.0-linux-amd64.luminext`，添加翻译服务、启用并保存即可；默认参数沿用原 Pot 插件。

## 数据

默认保存到 `os.UserConfigDir()/lumin`，Linux 通常为 `~/.config/lumin`：

- `config.json`：翻译服务及应用配置。
- `history.json`：历史和收藏。
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
- [插件协议、SDK、示例及 Pot 迁移](docs/plugins.md)
- [功能清单与后续开发计划](docs/features.md)
- [架构与目录职责](docs/architecture.md)
- [开发、扩展、调试和发布](docs/development.md)
- [Pot 迁移清单与后续路线](docs/migration.md)
- [验证记录与平台限制](docs/verification.md)
- [变更记录](CHANGELOG.md)

`pot-desktop-3.0.7/` 是保留的上游参考目录，不参与 Go 构建；其中内容继续保留原有 [GPL-3.0 许可证](pot-desktop-3.0.7/LICENSE) 与归属信息。
