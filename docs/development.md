# 开发与维护手册

## 环境与开发启动

固定依赖见 `go.mod`，当前 Go 1.27.4 / MyGo 0.10.0。运行 `go mod download` 后，使用 `go tool mygo doctor` 检查平台依赖。原生 UI 不需要 Node/Bun、Rust 或 WebView。

```bash
go tool mygo dev
# 或者不使用热重载
go run .
```

使用 `LUMIN_CONFIG_DIR=/tmp/lumin-dev` 隔离开发数据。MyGo 窗口位置等框架状态由框架独立保存，此变量只控制 Lumin 的配置和历史。单实例按应用名称/标识约束，改变数据目录不代表可以同时运行多个实例。

受限执行环境可以指定可写的 `GOCACHE`、`GOMODCACHE`。这属于环境设置，不要把某位开发者的缓存绝对路径或 `replace` 写入 `go.mod`。

## 翻译服务配置

服务字段：

| 字段            | 含义                                                      |
| --------------- | --------------------------------------------------------- |
| `id`          | 稳定且唯一的服务实例 ID，也用作 UI identity               |
| `name`        | 用户可见名称                                              |
| `kind`        | `compatible`、`ollama`、`deepl`、`libretranslate` |
| `enabled`     | 是否参与每次并行翻译                                      |
| `endpoint`    | 完整请求 URL，不带 URL 认证信息、查询参数或 fragment      |
| `model`       | LLM 模型名，启用 LLM 服务时必填                           |
| `api_key`     | 可选，本机明文保存，UI 隐藏显示                           |
| `api_key_env` | 可选环境变量名；变量非空时优先于 `api_key`              |

兼容接口及 Ollama 均发送非流式消息请求，服务之间并行，单个服务的译文完整返回后显示。不会伪造无密钥的翻译结果。不要把 HTML 网页 URL 当作 API 地址。

示例：只使用本机 Ollama，可以在界面启用对应模板并修改模型，也可在应用退出后写入以下配置：

```json
{
  "version": 1,
  "source": "auto",
  "target": "zh",
  "timeout_seconds": 60,
  "history_limit": 500,
  "save_history": true,
  "providers": [{
    "id": "ollama-local",
    "name": "我的本地模型",
    "kind": "ollama",
    "enabled": true,
    "endpoint": "http://127.0.0.1:11434/api/chat",
    "model": "qwen2.5:7b"
  }],
  "input_shortcut": "CmdOrCtrl+Alt+Space",
  "clipboard_shortcut": "CmdOrCtrl+Alt+T",
  "server_enabled": false,
  "server_address": "127.0.0.1:60828",
  "ocr_languages": "eng+chi_sim"
}
```

`source`/`target` 为启动默认语言；临时选择不自动修改磁盘。快捷键保存后重新注册；本地 HTTP 地址、令牌和启用状态需要重启。设置页和服务页共享同一份未保存草稿，任一保存按钮会保存整份配置。

## 新增翻译协议

1. 在 `provider` 中实现 `domain.Translator`。简单 JSON 协议可扩展 `HTTP`，复杂签名或流式协议应分文件/类型实现。
2. 在 `config.Validate` 增加类型、参数校验；按需扩展配置结构及默认模板。
3. 在 `desktop.Translate` 的构造处接入；协议增多时再抽出注册工厂，避免持续增加 switch。
4. 为服务专用字段增加 UI；保持服务实例 `id` 不变，不使用数组索引作持久 identity。
5. 使用 fake `http.RoundTripper` 或 `httptest.Server` 测试请求格式、授权、错误、取消、空结果与大小上限。测试不调用真实收费接口。
6. 更新迁移清单和文档中的接口状态。

模型请求将翻译规则放在 system 消息，原文放在 user 消息。语言映射属于 provider；不要把供应商语言代码泄漏到领域模型。用户输入不应直接拼入 URL 或 shell。

参考：[Ollama Chat API](https://docs.ollama.com/api/chat)、[DeepL Translate API](https://developers.deepl.com/api-reference/translate/request-translation)、[LibreTranslate API](https://libretranslate.com/docs/)。MyGo API 以固定版本的模块源码和 `docs/` 为准：

```bash
go list -m -f '{{.Dir}}' github.com/egoist/mygo
```

## UI 与异步任务

`App` 的可变展示字段只在 UI 线程访问。后台 I/O 必须通过 `post` 返回；它使用 `Window.Update` 并检查窗口关闭状态。异步任务还必须检查 `generation`，防止旧任务结果影响新输入。取消 context 后仍可能有已入队的回调，不能只靠取消 context。

稳定 identity 用 `c.Key`，跨帧焦点用 `ui.Handle`。不要跨帧保存 `ui.Element`；不要让 worker 捕获 `*ui.Context`。render 函数可能在一帧内重复执行，网络、落盘、进程等副作用只能从动作或受控任务触发。

## OCR

通过「图片识字」选文件，`ocr.Tesseract` 调用：

```bash
tesseract /absolute/path/image.png stdout -l eng+chi_sim
```

`tesseract --list-langs` 确认语言包。仅安装英文时将应用 OCR 语言改为 `eng`。本版本没有自动安装依赖、自动下载模型、截图框选或后台屏幕读取。新的 OCR 实现遵守 `Recognizer` 的取消和错误约定，UI 无需感知系统 API。

## 外部调用

在设置中生成令牌、启用本地服务、保存并重启。默认监听 `127.0.0.1:60828`。端口冲突会显示启动提示，翻译窗口仍然可用。

```bash
export LUMIN_API_TOKEN='填写设置中的令牌'
curl -X POST http://127.0.0.1:60828/translate \
  -H "Authorization: Bearer $LUMIN_API_TOKEN" \
  -H 'Content-Type: text/plain; charset=utf-8' \
  --data-binary 'Hello world'
```

| 路径                            | 行为                       |
| ------------------------------- | -------------------------- |
| `POST /translate`、`POST /` | 读取纯文本并排队在窗口翻译 |
| `POST /input_translate`       | 显示翻译页                 |
| `POST /config`                | 显示设置页                 |

成功返回 `202 queued`，表示已入队，不是译文或远端成功结果。其他状态：`400` 无效文本、`401` 鉴权失败、`403` 浏览器 Origin、`404` 未支持路径、`405` 方法错误、`413` 请求过大、`503` 正在退出或队列已满。最多 20000 字符，最大 80000 字节。

旧 Pot 脚本需要增加鉴权头并处理 `202`；划词与截图端点尚未提供。不允许绑定局域网或公网地址，也不提供 CORS。

## 数据迁移与恢复

修改配置 schema 时，先为旧版本编写显式迁移函数和测试，再提升 `config.Version`。加载后校验，成功后才写新文件；迁移失败保留原文件。当前没有 Pot 配置/历史导入器。

损坏的 `config.json` 会阻止启动并在错误中给出目录。先备份文件，修正 JSON 或移走该文件，再重启。损坏历史会在界面提示且拒绝覆盖，可备份后修复或移走 `history.json`。不要用静默清空处理反序列化错误。

备份时退出应用，复制整个 Lumin 数据目录。恢复时同样退出应用再替换。备份包含原文、译文以及可能直接填写的密钥。

## 检查

```bash
gofmt -w main.go internal
CGO_ENABLED=0 go test ./...
go test -race ./internal/...
go vet ./...
go tool mygo vet .
```

race 检查需要平台支持的 Go race 工具链及 C 编译器；正常应用构建不需要 CGO。截图检查：

```bash
LUMIN_TEST_SCREENSHOTS=/tmp/lumin-screenshots go test ./internal/desktop -run TestRenderPages -count=1
```

MyGo `Tester` 检查输入、页面切换、异步回调、窗口关闭、剪贴板反馈和 OCR 结果，同时渲染普通尺寸与最小尺寸。真机还需要检查输入法、系统主题、缩放、多显示器、托盘、快捷键门户、真实剪贴板与文件选择器。

## 构建与发布

```bash
go tool mygo build -platform linux/amd64
go tool mygo build -platform windows/amd64
go tool mygo build -platform darwin/arm64
```

产物位于 `build/<os>-<arch>/`。`mygo.json` 的 `webview: false` 表明应用不使用 WebView。仅检查跨平台源码编译可用：

```bash
CGO_ENABLED=0 GOOS=windows GOARCH=amd64 go build -o build/lumin.exe .
CGO_ENABLED=0 GOOS=darwin GOARCH=arm64 go build -o build/lumin-macos .
```

交叉编译通过不等于真机验证通过。正式发布前在对应平台验证系统集成、配置原子替换与权限、中文输入及窗口退出；安装包签名、公证按维护者的发布身份配置。

发布步骤：更新 `mygo.json` 和界面版本显示 → 更新 CHANGELOG → 执行检查 → 打包 → 在目标机器完成手动验收 → 发布。当前未配置自动更新或远程发布凭据。

## 维护规则

- 不修改参考目录来实现新功能；新代码只在 Go 工程内维护。
- 小改动优先在现有责任包中完成，不为单个调用制造多层包装。
- 涉及数据格式、后台任务、网络或 UI identity 的更改补充行为测试。
- 不在日志中输出密钥、完整 API 响应或原始剪贴板文本。
- MyGo 升级时先阅读目标版本源码与迁移指南，再运行 `go tool mygo migrate-ui .` 预览、审查并验证。
