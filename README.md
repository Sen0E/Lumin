# Lumin

基于 Go + MyGo 的原生桌面翻译工具，以本仓库中的 Pot Desktop 3.0.7 为功能迁移参考，优先维护 Linux，并保持 Windows/macOS 编译兼容。窗口使用 MyGo `ui`，无需 React、Node、Tauri 或 WebView。

当前版本是 **0.1.0，可运行的核心重写版**，不是 Pot 全功能的等价替换。详细状态见 [迁移清单](docs/migration.md)。

## 已实现

- 原生中文界面：翻译、历史记录、生词本、翻译服务、设置；跟随系统明暗主题。
- 多服务并行翻译、逐个显示结果、取消与超时、旧结果隔离、复制译文。
- Chat Completions 兼容协议、Ollama、DeepL 官方 API、LibreTranslate；可添加多个同类服务。
- 原生剪贴板粘贴、主动开启的剪贴板监听、全局快捷键、系统托盘。
- 导入图片并通过本机 Tesseract OCR 识字，编辑后翻译。
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
2. 勾选启用，点击「保存服务」。
3. 回到「翻译」，输入文本，点击「翻译文本」或按 `Ctrl+Enter`（macOS 为 `⌘+Enter`）。

使用本地 Ollama 时，先自行启动 Ollama 并安装目标模型。模板中的 `qwen2.5:7b` 只是可修改的模型名，Lumin 不下载模型。接口地址示例为 `http://127.0.0.1:11434/api/chat`。

DeepL 免费 API 使用 `https://api-free.deepl.com/v2/translate`；其他账户按服务商地址配置。兼容服务填写完整的 `/chat/completions` 地址，不自动追加路径。密钥环境变量优先于配置中直接填写的密钥。各服务支持的语言和模型以其实际接口为准。

## 常用操作

| 操作 | 默认方式 |
| --- | --- |
| 打开窗口 | `Ctrl+Alt+Space`；macOS 用 `Cmd` 替代 `Ctrl` |
| 翻译剪贴板 | `Ctrl+Alt+T`；macOS 用 `Cmd` 替代 `Ctrl` |
| 窗口内翻译 | `Ctrl+Enter` / `⌘+Enter` |
| 取消任务 | `Esc` 或「取消」 |
| 图片 OCR | 「图片识字」选择图片 |
| 生词本 | 历史记录中点击「收藏」 |
| 退出 | 关闭窗口或托盘菜单退出；当前关闭窗口即退出 |

Wayland 的全局快捷键依赖桌面门户及应用桌面条目，通常需要安装打包后的应用并完成桌面授权。剪贴板监听也受桌面支持限制；窗口内粘贴始终可以作为备用入口。

OCR 需要安装 `tesseract` 以及 `eng`、`chi_sim` 语言数据。可以在设置中将 OCR 语言改为已经安装的语言，使用 `tesseract --list-langs` 查看。本版本支持图片导入，不包含交互式截图框选。

## 数据

默认保存到 `os.UserConfigDir()/lumin`，Linux 通常为 `~/.config/lumin`：

- `config.json`：翻译服务及应用配置。
- `history.json`：历史和收藏。

开发时可以使用独立目录：

```bash
LUMIN_CONFIG_DIR=/tmp/lumin-dev go run .
```

配置中的 API Key 是本地明文，Unix 上新文件权限为 `0600`、新目录为 `0700`；可改用环境变量以免把密钥写入配置。历史含原文和译文，可以关闭保存。不要提交真实配置文件。

## 开发与打包

```bash
CGO_ENABLED=0 go test ./...
go test -race ./internal/...
go vet ./...
go tool mygo vet .
go tool mygo build -platform linux/amd64
```

Linux 产物在 `build/linux-amd64/`。交叉编译及发布验收见 [开发手册](docs/development.md)。

- [架构与目录职责](docs/architecture.md)
- [开发、扩展、调试和发布](docs/development.md)
- [Pot 迁移清单与后续路线](docs/migration.md)
- [验证记录与平台限制](docs/verification.md)
- [变更记录](CHANGELOG.md)

`pot-desktop-3.0.7/` 是保留的上游参考目录，不参与 Go 构建；其中内容继续保留原有 [GPL-3.0 许可证](pot-desktop-3.0.7/LICENSE) 与归属信息。
