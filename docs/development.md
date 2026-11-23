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
| `kind`        | `compatible`、`ollama`、`deepl`、`libretranslate`、`plugin` |
| `enabled`     | 是否参与每次并行翻译                                      |
| `endpoint`    | 完整请求 URL，不带 URL 认证信息、查询参数或 fragment      |
| `model`       | LLM 模型名，启用 LLM 服务时必填                           |
| `api_key`     | 可选，本机明文保存，UI 隐藏显示                           |
| `plugin` | 插件固定引用 `id@version`，仅 kind=plugin |
| `settings` | 插件声明的字符串参数 map，仅 kind=plugin |
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

`tesseract --list-langs` 确认语言包。仅安装英文时将应用 OCR 语言改为 `eng`。截图框选、截图翻译和替换 OCR 插件已实现；没有自动安装依赖或下载模型。详见 [截图说明](screenshots.md) 和 [插件协议](plugins.md)。新的 OCR 实现遵守 `Recognizer` 的取消和错误约定，UI 无需感知系统 API。

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
| `POST /ocr_recognize` | 截图框选并识字到编辑器 |
| `POST /ocr_translate` | 截图框选、识字后翻译 |
| `POST /config`                | 显示设置页                 |

成功返回 `202 queued`，表示已入队，不是译文或远端成功结果。其他状态：`400` 无效文本、`401` 鉴权失败、`403` 浏览器 Origin、`404` 未支持路径、`405` 方法错误、`413` 请求过大、`503` 正在退出或队列已满。最多 20000 字符，最大 80000 字节。

旧 Pot 脚本需要增加鉴权头并处理 `202`；截图端点仍需用户授权和框选，不返回图片或识别结果；Pot 的截图 query 参数不作兼容，统一打开框选。自动划词端点尚未提供。不允许绑定局域网或公网地址，也不提供 CORS。

## 数据迁移与恢复

修改配置 schema 时，先为旧版本编写显式迁移函数和测试，再提升 `config.Version`。加载后校验，成功后才写新文件；迁移失败保留原文件。当前没有 Pot 配置/历史导入器。

损坏的 `config.json` 会阻止启动并在错误中给出目录。先备份文件，修正 JSON 或移走该文件，再重启。损坏历史会在界面提示且拒绝覆盖，可备份后修复或移走 `history.json`。不要用静默清空处理反序列化错误。

备份时退出应用，复制整个 Lumin 数据目录。恢复时同样退出应用再替换。备份包含原文、译文以及可能直接填写的密钥。

## 检查

```bash
gofmt -w main.go internal pkg cmd examples plugins
CGO_ENABLED=0 go test ./...
go test -race ./internal/... ./pkg/... ./plugins/...
go vet ./...
go tool mygo vet .
```

race 检查需要平台支持的 Go race 工具链及 C 编译器；正常应用构建不需要 CGO。截图检查：

```bash
LUMIN_TEST_SCREENSHOTS=/tmp/lumin-screenshots go test ./internal/desktop -run 'TestRenderPages|TestSelectionDragResizeAndCancel' -count=1
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

## 0.2.0 配置扩展

保留 `version: 1`，新增 `screenshot_shortcut`、`ocr_plugin`、`ocr_settings` 和 provider 的 `plugin`、`settings` 字段。旧配置缺失这些可选字段仍能加载，不会自动开启截图快捷键或插件；新建配置默认使用 `CmdOrCtrl+Alt+S`。OCR 插件引用为空时使用内置 Tesseract。保存前验证已启用插件是否存在、支持当前平台和操作，以及必填参数。

示例插件通过 `make plugins` 构建，打包工具在 `cmd/lumin-plugin-pack`，完整维护流程见 [plugins.md](plugins.md)。插件及其配置应一起备份，跨平台恢复时还需安装目标平台入口。

彩云移植位于 `plugins/caiyun`，只依赖标准库与 `pkg/pluginapi`，不修改宿主 provider。维护新插件时在 `scripts/build-plugins.sh` 的 sources 列表登记源目录；包名来自清单 ID 和版本。GPL 插件随二进制包包含许可及完整对应源码；新增同类插件也应保留其分发材料。

## 0.3.0 服务测试、排序与常用设置

对应任务：功能清单的 SVC-02、SVC-03、CFG-02。配置仍为 version 1，没有新增持久化字段，0.1/0.2 的有效配置继续加载。

- 「测试此服务」使用该实例的当前草稿，包括尚未启用的服务。实际发送固定 `Hello, world!`（en → zh），使用草稿超时，可能产生服务调用费用；不读取输入框或剪贴板，不写历史或配置。成功显示预览和耗时，失败显示错误。
- 同一时间只有一个服务测试；测试其他服务会取消前一个，当前测试可点击取消或在服务页按 Esc。与正式翻译使用独立的 context、代次和展示字段。
- 测试快照深拷贝插件参数。完成时校验服务配置及超时是否仍匹配；编辑后的旧结果不作为当前配置成功的依据。移除、放弃修改、保存或退出程序会取消在途测试（0.4.0 起关闭设置窗口只是隐藏）。
- 服务上移/下移仅修改草稿顺序。实例 ID 保持不变；保存后用于下一次翻译与下次启动，已有译文不会被重新排列。
- 设置页原生数字输入限制超时 1–300 秒、普通历史 1–5000 条；输入不合法时不提交新数值，离开输入框恢复有效值。默认源/目标语言下次启动生效；保存超时对之后的新请求生效；历史上限在下一次写入翻译记录时执行。
- 翻译服务和设置页仍共享一份草稿，任一保存按钮保存全部；放弃修改会同步恢复数字、语言及插件字段编辑器。

实现职责：`desktop/providers.go` 管理服务构造、测试和顺序动作；`desktop/preferences.go` 管理设置编辑器。MyGo NumberInput 绑定持久 float64 编辑值，OnChange 写回配置 int；不要改为每帧重新创建局部值。

回归测试在 `internal/desktop/providers_test.go`、`preferences_test.go`。测试命令 `go test ./internal/desktop`；离屏渲染额外包含 `TestRenderServiceProbe`。

## 0.4.0 后台运行与窗口维护

配置仍为 version 1，无新增持久化字段。MyGo WindowOptions 使用独立 StateKey：`preferences`（1100×800，最小 920×680）、`translation`（520×620，最小 460×480，初始隐藏）。旧版 `main` 窗口布局不会套用到新小窗口。尺寸单位为 MyGo 逻辑像素；保留系统标题栏，支持移动、缩放和关闭。

| 入口 | 目标 |
| --- | --- |
| 首次启动、再次运行、无可见窗口的 Dock 激活 | 配置窗口 |
| 输入快捷键、托盘打开翻译、`/input_translate` | 翻译窗口并聚焦输入框 |
| 剪贴板快捷键/监听、`/translate` | 翻译窗口 |
| 截图快捷键/托盘、`/ocr_recognize`、`/ocr_translate` | 框选；成功后翻译窗口；取消不额外开窗 |
| 托盘偏好设置、小窗口设置、`/config` | 配置窗口的设置页 |
| 小窗口配置翻译服务 | 配置窗口的翻译服务页 |

关闭任意窗口仅隐藏，不销毁视图，也不卸载系统集成。关闭翻译窗口取消当前翻译/OCR，代次校验防止迟到结果弹窗或覆盖文本；设置窗口隐藏保留草稿与服务测试。Esc 在小窗口忙碌时取消任务，空闲时隐藏。后台剪贴板监听仍需用户主动开启，每次进程启动默认关闭。开机启动属于 OS-02b，未实现。

真正退出必须经过 App.Quit：OnBeforeQuit 设置 quitting，避免 OnClose 拦截应用退出；OnQuitCanceled 恢复标记；OnQuit 统一释放资源。截图恢复不得在 quitting 时显示窗口。托盘不可用时通过单实例再次启动找回设置，左下角退出按钮始终可用。

回归入口：`go test ./internal/bootstrap ./internal/desktop`。`windows_test.go` 验证隐藏/唤起/退出和四种截图可见组合；`translation_window_test.go` 验证翻译路由、取消后可再次使用、输入聚焦、关闭后的旧结果、最小布局和 Esc。离屏截图增加 `TestCompactWindowLayouts`；原生实机清单见 verification.md。窗口逻辑不可用纯渲染测试替代目标桌面的托盘/窗口管理器验收。
