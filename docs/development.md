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
  "ocr_disabled": false
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

0.9.0 起全部生产 OCR 通过已安装 `.luminext` 插件执行。发布资源 `bundled-ocr.luminext` 提供默认离线引擎，由 `ocrsetup.Ensure` 在未配置且未主动停用时安装并保存选择。宿主无系统 OCR 回退。`ocr.Recognizer` 只定义接口及固定测试图片，参数由插件声明。测试和默认资源构建契约见 [ocr.md](ocr.md)。

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

外部脚本需要添加鉴权头并处理 `202`；截图端点仍需用户授权和框选，不返回图片或识别结果；截图请求统一打开框选。自动划词端点尚未提供。不允许绑定局域网或公网地址，也不提供 CORS。

## 数据迁移与恢复

修改配置 schema 时，先为旧版本编写显式迁移函数和测试，再提升 `config.Version`。加载后校验，成功后才写新文件；迁移失败保留原文件。备份只接受 Lumin 格式。

0.7.0 起以 `state.json` 为权威数据；没有快照时读取旧 `config.json` / `history.json`。损坏快照或旧数据阻止加载与覆盖，不静默回退或清空。先退出并复制整个数据目录再修复，详见 backups.md。

常规备份可在主窗口「备份设置」在线完成；整目录手工复制仍须退出应用。自动恢复备份包含原文、译文及明文密钥。

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

MyGo `Tester` 检查输入、页面切换、异步回调、窗口关闭、取词顺序、显式复制/粘贴和 OCR 结果，同时渲染普通尺寸与最小尺寸。真机还需要检查输入法、系统主题、缩放、多显示器、托盘、快捷键门户、真实剪贴板与文件选择器。

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

保留 `version: 1`，新增 `screenshot_shortcut`、`ocr_plugin`、`ocr_settings` 和 provider 的 `plugin`、`settings` 字段。旧配置缺失这些可选字段仍能加载，不会自动开启截图快捷键或插件；新建配置默认使用 `CmdOrCtrl+Alt+S`。0.9.0 起 OCR 插件引用为空时引导配置，不使用系统回退。保存前验证已启用插件是否存在、支持当前平台和操作，以及必填参数。

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
| 选中文字快捷键、`/translate` | 翻译窗口 |
| 截图快捷键/托盘、`/ocr_recognize`、`/ocr_translate` | 框选；成功后翻译窗口；取消不额外开窗 |
| 托盘偏好设置、小窗口设置、`/config` | 配置窗口的设置页 |
| 小窗口配置翻译服务 | 配置窗口的翻译服务页 |

关闭任意窗口仅隐藏，不销毁视图，也不卸载系统集成。关闭翻译窗口取消当前翻译/OCR，代次校验防止迟到结果弹窗或覆盖文本；设置窗口隐藏保留草稿与服务测试。Esc 在小窗口忙碌时取消任务，空闲时隐藏。后台剪贴板监听仍需用户主动开启，每次进程启动默认关闭。开机启动属于 OS-02b，未实现。

真正退出必须经过 App.Quit：OnBeforeQuit 设置 quitting，避免 OnClose 拦截应用退出；OnQuitCanceled 恢复标记；OnQuit 统一释放资源。截图恢复不得在 quitting 时显示窗口。托盘不可用时通过单实例再次启动找回设置，左下角退出按钮始终可用。

回归入口：`go test ./internal/bootstrap ./internal/desktop`。`windows_test.go` 验证隐藏/唤起/退出和四种截图可见组合；`translation_window_test.go` 验证翻译路由、取消后可再次使用、输入聚焦、关闭后的旧结果、最小布局和 Esc。离屏截图增加 `TestCompactWindowLayouts`；原生实机清单见 verification.md。窗口逻辑不可用纯渲染测试替代目标桌面的托盘/窗口管理器验收。

## 0.5.0 窗口职责与托盘诊断

0.4.0 章节的历史/收藏、小窗口设置按钮等位置说明已由本节和 [window-map.md](window-map.md) 替代。窗口生命周期、配置 schema 不变。

主窗口 page 与服务子页 servicePage 独立保存；翻译窗口不再有 translationPage。历史载入写入原文/语言/结果并调用 ShowTranslation，不更改主窗口 page。新功能先在 window-map.md 确认归属；不要再把管理页加入翻译窗口。

- `settings_window.go`：常规、翻译、识别、热键设置以及服务子页，共享配置草稿；保存/放弃修改仍处理完整草稿。
- `history_view.go`：历史、搜索、清理与收藏过滤；过滤只影响显示。
- `translation_window.go`：原文操作、语言、结果、临时置顶；置顶通过 Ports 调用 Window.SetAlwaysOnTop，不写配置，不改其他窗口。
- `traystatus`：通过会话总线读取 StatusNotifierWatcher，查询已注册项目所属 Unix PID；注册成功不等于图标未被面板折叠。不修改桌面设置、不自动安装系统库。
- `desktop/tray.go`：单个检查最多三秒，关闭应用取消任务，隐藏设置窗口保留检查结果；结果与普通 status 分离。安装 GTK3 AppIndicator 库后必须完整退出再启动，MyGo 不在运行中重新加载它。

验证：window_map_test.go 覆盖功能位置、历史载入、收藏筛选、原文复制和置顶；原有服务测试、插件字段与配置保存回归仍需通过。源码和未迁移能力边界在窗口对照表中维护。

## 0.6.0 识别窗口维护

识别入口：RecognizeImage / Screenshot(false) → startRecognition；截图翻译：Screenshot(true) → recognizeTranslation。不能重新将两个流程的 busy/cancel/generation 合并。OCR 视图状态只在主线程写入，图片发布后不再修改；临时文件必须在工作线程 defer 清理。关闭和取消递增 generation，迟到完成不打开窗口。新增业务窗口时同时修改 windowSet 的截图隐藏/恢复、退出、派发刷新和测试。

预览图片沿用 capture.ReadImage 的 64 MiB/6400 万像素上限，支持 PNG/JPEG/BMP/TIFF/WebP；OCR 引擎仍有自己的输入限制。识别窗口内存图片保留到下一次识别或退出，不写入历史。配置 schema 和插件协议未变。

## 0.7.0 备份维护

格式、事务边界、旧文件迁移与恢复操作见 [backups.md](backups.md)。backupView 只在主窗口；文件对话框通过 Ports 注入，I/O 在工作线程运行，通过 post 更新视图。恢复时阻止新翻译/OCR/配置保存/历史写操作，取消旧任务与剪贴板监听；原生托盘退出仍有效。恢复代次校验在存储锁内，不能只靠 UI generation 防止旧翻译记录写回。

## 0.8.0 历史维护

DATA-03 的字段、格式及交互契约见 [history.md](history.md)。历史筛选只维护 filteredHistory 一处，列表和导出必须一致。确认目标绑定 ID，不能用行号删除；收藏保护必须在存储锁内重新检查。异步导出复制 Results 后再进入 goroutine，文件对话框通过 Ports.SaveHistoryExport 注入。不得在结果导出中序列化 settings/draft 或完整 storage.Snapshot。

## 0.10.0 框选输入维护

截图交互以 mouse-up 为提交边界，无顶部按钮。SelectionView 只保存模型和 ui.Handle；画布填满视口，浮动提示属于 Painter 绘制。完整状态契约与平台限制见 screenshots.md。不得恢复用顶部工具栏挤压截图布局，也不要在 native overlay 关闭前重复触发 finish。坐标换算在除法前转 float64，再用 capture.Selection.Pixels 的 floor/ceil 映射原图。

## 0.11.0 运行时工具依赖约束

运行依赖的归属和平台验收见 [runtime-dependencies.md](runtime-dependencies.md)。不要在宿主增加 exec.Command/LookPath 调用系统 OCR、截图或进程管理工具；宿主仅通过插件管理器启动已安装的包内入口。平台功能优先直接调用 OS API，专用引擎和模型随插件提供。构建工具与运行依赖分开记录。

修改插件启动逻辑后运行进程取消、私有 PATH 和真实模型测试；Windows Job Object 的挂起启动/分配/恢复顺序保证插件运行前纳入管理，不能改回先启动再分配。macOS 异步回调要遵守 Block 与 Objective-C 对象生命周期，取消后不能写回业务窗口。

## 0.11.1 热键显示与截图性能

热键原始值留在配置；`desktop/shortcuts.go` 在持久 preferencesEditor 中保存当前平台的可读值。仅实际编辑后写回紧凑格式，未编辑的 CmdOrCtrl 保持原样；不要在每帧用局部格式化字符串绑定输入框。同步入口统一为 syncPreferences，覆盖启动、放弃修改和恢复备份。

截图计时的开关、阶段定义和限制见 [截图耗时排查](screenshots.md#0111-截图耗时排查)。比较 PNG 编码可运行 `go test ./internal/capture -run '^$' -bench BenchmarkTempPNGFullHD -benchtime=3x`；该指标不代表系统截图或 OCR 总耗时。

## 0.12.0 选中文字翻译

取词接口、v1 配置兼容、取消语义和真机矩阵见 [selection-translation.md](selection-translation.md)。`ReadSelection` 必须在显示或聚焦翻译窗口之前调用；禁止以普通剪贴板或旧编辑器文本作为失败回退。剪贴板监听已移除，Paste 仅填入文本，不自动翻译。

## 0.12.1 Wayland 选区协议

读取路径与 VS Code 设置见 [取词排错](selection-translation.md#vs-code-与-wayland-排错0121)。原生实现只消费 data-control 的 primary_selection 事件，不能把普通 selection 事件当回退，也不能在每次失败时静默翻译旧输入。Unix 连接、FD、context 取消与消息边界分别有回归；协议全局缺失时保留 AT-SPI 路径。

## 0.13.0 翻译交互与外观

设计和维护契约见 [ux.md](ux.md)。失焦计时在主线程创建/取消，后台计时仅通过 Window.Update 发布；窗口重新聚焦和新取词交互递增代次，截图/隐藏/退出通过 windowSet 过滤。不要把临时隐藏误当作永久关闭。

save/restore 共用 desktop.applyDesktopSettings。配置字段只描述默认尺寸；原生 StateKey 记录手动拖动。preferences.WindowSizeEdited 记录显式尺寸/预设操作，使选择相同默认预设也能恢复窗口；仅主题/字号保存不能重置手动尺寸。快捷键有独立差异检查，避免外观保存触发重复门户授权。

主题通过 MyGo Theme.SetSource 应用到原生外观，三个业务视图共享 applyAppearance 的颜色适配；保留系统字体/布局度量，不保存 ui.Context 或已构建元素。0 值兼容旧配置；修改 schema、备份或显示设置时复验保存/恢复和退出记忆。

## 0.13.1 紧凑阅读布局

当前默认 350×420，最小 320×360。有效 Translate 请求校验后设 sourceCollapsed=true；ShowTranslation 用于手动输入时展开，TranslateText/取词/截图在其后启动 Translate 并进入阅读模式。不要在异步完成回调重新折叠，以免打断用户已展开的编辑动作。相同输入 Handle 与稳定 Key 在模式切换额外帧中保持，UI 构建值不能存入 App。已有尺寸不自动迁移，紧凑预设显式保存后通过原生 SetSize 应用。
