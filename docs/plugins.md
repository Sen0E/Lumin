# 插件开发与 Pot 迁移

Lumin 0.2.0 提供 `.luminext` 插件。插件是 ZIP 包，内含声明文件与各平台的可执行程序；宿主通过 **JSON 标准输入/输出协议 v1** 调用，每个请求启动一个进程。实现语言不限，Go SDK 位于 `pkg/pluginapi`，不依赖 MyGo。

## 安装与使用

1. 在「插件」页面选择 `.luminext`。安装只校验并解压，不运行程序。
2. 翻译插件：点击「添加翻译服务」，填写字段，勾选启用，点击「保存服务」。同一插件可建立多个服务配置。
3. OCR 插件：点击「用于 OCR」，在设置中填写参数并保存。之后图片识字、截图识字和截图翻译都使用该引擎。可以点击「恢复内置 Tesseract」并保存。
4. 升级：安装新版本包，创建或切换到新版本配置，保存并验证，再移除旧服务、卸载旧版。配置固定引用 `id@version`，不会自动升级。
5. 卸载前须移除已保存配置和草稿中的全部引用；正在执行任务或保存时不能卸载。禁用服务仍然算引用。

插件以当前用户权限执行，**不是沙箱**。只启用自己维护或信任的程序。`secret` 仅控制输入框隐藏显示，字段值仍在本地 `config.json` 明文存储并通过 stdin 传给插件。安装目录没有签名验证和自动更新。首次启用联网插件前应了解它会向何处发送原文或图片。

`.potext` 不能直接安装执行：其 JavaScript 调用依赖 Pot 注入的 Tauri API、HTTP、CryptoJS、数据库或 native 辅助程序。Lumin 采用独立协议，没有嵌入 JavaScript/Tauri 运行时，也没有声称兼容全部 Pot 生态。格式迁移见下文。

## 包结构

```text
example.translator-1.0.0.luminext  # ZIP，不要再套一层目录
├── plugin.json
└── bin/
    ├── linux-amd64
    ├── windows-amd64.exe
    └── darwin-arm64
```

```json
{
  "schema": 1,
  "id": "example.translator",
  "name": "我的翻译插件",
  "version": "1.0.0",
  "author": "维护者",
  "description": "通过指定 API 翻译文本",
  "capabilities": ["translate"],
  "entrypoints": {
    "linux/amd64": "bin/linux-amd64",
    "windows/amd64": "bin/windows-amd64.exe",
    "darwin/arm64": "bin/darwin-arm64"
  },
  "settings": [
    {"key": "endpoint", "label": "接口地址", "required": true},
    {"key": "api_key", "label": "API Key", "secret": true}
  ]
}
```

`id` 为小写字母开头的字母、数字、点和连字符，最长 80 字符，不能含连续两个点。`version` 使用 `1.2.3` 或 `1.2.3-beta.1`，最长 64 字符。`capabilities` 可选 `translate`、`recognize`，也可同时声明。平台支持 Linux、Windows、macOS（`darwin`）各自的 `amd64` / `arm64`。只需提供实际支持的平台；宿主不会启用缺少当前平台入口的插件。

设置最多 32 项，键为字母开头的字母、数字或下划线。可声明 `default`；`required` 的空值会阻止保存和执行。宿主只发送已声明字段，未知字段会被忽略。语言差异和供应商的语言代码映射由插件处理。

## 请求与响应

进程无命令行参数；工作目录是插件版本目录。stdin 是一个 UTF-8 JSON 对象加换行，随后 EOF。stdout 必须只有一个 JSON 对象；日志写 stderr。程序完成一个请求后退出 0。

翻译请求：

```json
{"protocol":1,"id":"request-id","operation":"translate","text":"Hello","source":"auto","target":"zh","settings":{"endpoint":"https://example.com/translate","api_key":"..."}}
```

OCR 请求：

```json
{"protocol":1,"id":"request-id","operation":"recognize","image_path":"/tmp/lumin-capture-123.png","languages":"eng+chi_sim","settings":{}}
```

图片路径为绝对路径，大小上限 20 MiB。截图生成私有临时 PNG，识别完成或失败后删除；插件必须在返回前读完图片，不能启动后台任务继续使用该路径。`languages` 是用户 OCR 语言配置；使用其他 OCR API 时自行映射。

成功与失败：

```json
{"protocol":1,"id":"request-id","text":"你好"}
{"protocol":1,"id":"request-id","text":"","error":"请检查 API Key"}
```

响应必须回传同一个 `id` 和协议版本；成功文本不能为空。不要将密钥、完整服务响应或原文放进错误消息。宿主只显示最多 600 字符的插件错误；非零退出码给出通用进程错误，不展示 stderr。

## Go SDK 与示例

在本仓库内新增示例可直接导入 `lumin/pkg/pluginapi`。独立插件仓库可以复制这个仅依赖标准库的协议包，或实现相同 JSON 协议；当前应用模块名是本地 `lumin`，尚未发布可直接 `go get` 的远程 SDK 模块。

```go
func main() {
    err := pluginapi.Serve(func(ctx context.Context, req pluginapi.Request) (string, error) {
        // 校验 req.Operation，使用 req.Settings 调用服务。
        // 为网络请求设置超时；错误中不要包含密钥。
        return translate(ctx, req)
    })
    if err != nil {
        fmt.Fprintln(os.Stderr, err)
        os.Exit(1)
    }
}
```

维护中的插件与两个示例：

- `plugins/caiyun`：[彩云小译 Pot 插件的 Go 移植](../plugins/caiyun/README.md)，默认沿用上游客户端参数；包内附 GPL 许可、出处与对应构建源码。

- `examples/plugins/libretranslate`：实际 HTTP 翻译适配，需提供可用的 LibreTranslate 服务；不是离线模型。
- `examples/plugins/tesseract`：调用已安装的 Tesseract，使用应用 OCR 语言设置；用于展示完整 OCR 协议。

构建插件包（脚本需要 Bash 和 Python 3）：

```bash
make plugins
# 只构建彩云插件
./scripts/build-plugins.sh caiyun
# 可交叉编译单平台包
GOOS=windows GOARCH=amd64 ./scripts/build-plugins.sh
```

输出到 `build/plugins/`，文件名含平台。脚本可重复运行，重新生成 build 目录下的同名产物；已安装版本仍不允许覆盖。正式修改插件后应提升版本，构建脚本读取清单版本生成文件名。

自行打包已准备好的目录：

```bash
go run ./cmd/lumin-plugin-pack -dir /path/to/prepared-plugin \
  -out /path/to/example.translator-1.0.0.luminext
```

`plugin.json` 必须位于源目录根部，声明入口必须存在。打包工具校验产物且拒绝覆盖；输出不能在源目录里面。用 `CGO_ENABLED=0 GOOS=... GOARCH=... go build` 生成对应入口；需要 CGO 或外部工具的插件自行记录运行依赖。

## 生命周期与限制

- 请求启动独立进程，无持久进程协议、流式输出或宿主数据库访问。
- 翻译受应用请求超时约束；插件调用自身上限 90 秒，OCR 整体包括截图和框选共 90 秒。SDK 的 context 不接收宿主取消通知，宿主通过终止进程取消。
- Linux/macOS 创建进程组，取消时终止整组；Windows 尝试 `taskkill /T /F` 并终止主进程。插件不得脱离进程组驻留后台。
- stdout 最多 2 MiB、stderr 最多 64 KiB，超出即取消；子进程退出后管道等待上限 1 秒。
- 继承环境仅含 PATH、Windows 系统路径、临时目录和语言区域变量；不自动继承宿主 API Key、HOME 或代理环境变量。插件可通过声明字段接收所需设置。这不是权限隔离，程序仍能访问当前用户可访问的系统资源。
- 包最多 128 MiB、128 个条目、单文件 64 MiB、总解压量 256 MiB；清单最多 64 KiB。拒绝路径穿越、符号链接、大小写重复路径和 Windows 保留设备名。
- 使用私有暂存目录解压后原子重命名。相同版本拒绝覆盖；安装失败清理暂存目录。文件位于数据目录 `plugins/<id>@<version>/`。
- 进程崩溃、协议错误、能力不匹配只影响对应服务，其他翻译服务仍可完成。

## 从 Pot 插件迁移

迁移依据是本仓库 `pot-desktop-3.0.7/src/utils/invoke_plugin.js` 与 `src-tauri/src/cmd.rs`，不依赖上游未来行为。

| Pot                                | Lumin                                                    |
| ---------------------------------- | -------------------------------------------------------- |
| ZIP `.potext`、`info.json`     | ZIP `.luminext`、`plugin.json`                       |
| `main.js` 导出 `translate`     | 可执行程序处理 `operation: translate`                  |
| `recognize(base64, ...)`         | `operation: recognize`，按 `image_path` 读取原始图片 |
| `needs` 等配置声明               | 清单 `settings`，值在请求 `settings` 中              |
| 注入的 Tauri `http` / CryptoJS   | 插件自身 HTTP、签名与加密库                              |
| Pot 的 `run` / 文件 / 数据库 API | 程序自身依赖；当前没有对应宿主 RPC                       |
| 界面元素、收集到生词本、语音等扩展 | v1 尚未定义能力，需要单独设计协议                        |

建议先迁移纯 HTTP 翻译/OCR 插件：保留请求签名、语言映射与结果解析逻辑，使用标准库重写 I/O，再增加协议入口和清单。不要只重命名包扩展名。对携带二进制的旧插件须核实许可证、平台架构和分发权利。

## 回归与排错

```bash
go test ./internal/plugin ./pkg/pluginapi ./internal/desktop
LUMIN_TEST_OCR_PLUGIN="$PWD/build/plugins/example.tesseract-1.0.0-linux-amd64.luminext" \
  go test ./internal/plugin -run TestBuiltOCRExample -count=1
```

第二条需要先构建示例并安装 Tesseract `eng`。协议测试还使用真正的测试子进程验证错误输出、超大响应与取消，无需联网。服务无法启用先检查清单平台、字段、版本引用；执行失败检查入口架构和依赖。若手动修改导致清单损坏，退出应用，备份后移走对应插件版本目录，再安装原包。
