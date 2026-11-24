# 验证记录

日期：2026-11-22。版本：Lumin 0.2.0。环境：Linux amd64，Go 1.27.4，MyGo 0.10.0。

## 已完成

- `CGO_ENABLED=0 go test ./...`：通过。
- `go test -race ./internal/... ./pkg/...`：通过；涵盖存储并发写入、任务回调和 UI 主线程状态边界。
- `go tool mygo vet .`：通过。
- `go vet ./...`：通过。
- `go mod tidy`：完成，依赖版本未漂移；`go mod verify` 校验下载依赖的完整性。
- 插件：路径穿越、符号链接、重复包项、缺失入口、版本固定、参数过滤与环境隔离测试；真实子进程验证协议、错误输出、超限输出及超时取消。
- 截图：原图坐标、反向拖动、缩放保持、指针取消、Enter/Esc、临时图片清理、截图 OCR → 翻译及过期回调隔离。
- MyGo Tester：中文输入、页面隐藏/恢复、剪贴板粘贴、无服务时配置引导、翻译保存、旧结果丢弃、关闭后的回调隔离、OCR 结果接收、复制译文防反馈循环、插件字段跨帧输入和放弃修改、缺失插件不能保存。
- UI 离屏渲染：1100×800 和最小 920×680 尺寸；翻译、历史、生词本、服务、插件、设置、框选窗口、结果与错误卡片，以及深色主题。已人工查看渲染图并修复布局溢出。
- Tesseract 真进程：使用仓库内英文测试图片和 `eng` 语言包成功识别 `Lumin translation test.`。
- 示例插件：生成 LibreTranslate / Tesseract 的 Linux amd64 `.luminext`；安装已打包 OCR 示例，调用真实子进程，成功识别英文测试图。
- Linux MyGo 发布构建：生成原生可执行文件、`.deb`、`.rpm`、`.tar.gz`、桌面条目和安装脚本。
- Windows amd64、macOS arm64：`CGO_ENABLED=0 go build` 交叉编译通过。

网络协议测试使用模拟传输验证请求与响应，包括错误、取消、超大响应、环境变量密钥优先级和禁止重定向；没有调用真实收费翻译服务。

## 本环境无法完成

- 原生窗口启动验证在本环境返回 `mygo: cannot open display`。当前沙箱无法访问宿主显示服务，因此没有把离屏渲染当成真机启动成功。
- Linux Screenshot 门户授权、grim/maim 回退和真实多显示器框选未完成真机验收；Windows/macOS 截图仅编译验证。macOS 目标屏幕的 `-R`、负坐标和 Retina 缩放仍需真机确认。
- 托盘、Wayland 全局快捷键授权、系统剪贴板事件、文件选择器和中文输入法需要在实际桌面验证。
- 当前 Tesseract 只有 `eng` 和 `osd`，未验证中文 OCR。安装 `chi_sim` 后再验证默认中英组合。
- 未配置实际翻译服务密钥或运行中的 Ollama，未做生产接口连通性和翻译质量测试。
- Windows/macOS 未进行真机运行、签名或安装包验收。

## 手动验收步骤

1. `go tool mygo dev` 打开主界面；检查中文输入法、缩放、系统深浅主题和最小窗口尺寸。
2. 配置可用服务，启用并保存；输入中英文各一段，检查译文、复制与历史保存。
3. 同时启用两个服务；让其中一个使用无效地址，确认另一个可独立成功。
4. 翻译期间改原文或取消，确认旧结果不覆盖新输入；关闭翻译窗口，检查任务取消而进程常驻；用快捷键重新打开。最后从托盘/设置明确退出，检查进程结束。
5. 重启检查配置与历史；收藏一条后清除普通历史，确认收藏仍可载入。
6. 检查托盘、全局快捷键与剪贴板监听；复制译文不能反复触发翻译。
7. 配置已安装 OCR 语言，导入图片并编辑识别结果后翻译。
8. 安装示例插件，添加服务或选择 OCR 引擎、填写参数并保存；验证切换、卸载引用保护与重启恢复。
9. 按 [截图验收](screenshots.md) 检查权限、两个窗口可见性恢复、鼠标所在显示器截图/框选、取消与临时文件。
10. 启用外部接口并重启，用带令牌的 curl 请求验证，包括截图端点；无令牌请求应返回 401。

复现命令及故障恢复见 [开发手册](development.md)。

## 彩云插件增量验证（2026-11-23）

- `CGO_ENABLED=0 go test ./...`、`go vet ./...` 和 `go test -race ./plugins/caiyun ./internal/plugin` 通过。
- 彩云 Go 移植的离线测试：请求字段/头与上游一致、语言及设置、错误不泄露正文/密钥、大小限制、取消、禁止重定向与 JSON-stdio。
- Linux 插件包由 Manager 安装、ValidateUse 校验并启动真实子进程；无网络输入校验响应通过协议回传。
- 获准联网的 curl 使用相同请求参数调用服务，HTTP 200，固定 `Hello world` 返回 `你好，世界`。
- 当前沙箱 Go 进程无法解析外网域名（Temporary failure in name resolution）；实际宿主 → 插件 → 彩云的线上调用测试未通过，因此不宣称端到端联网已验证。
- 插件提供 Linux amd64、Windows amd64、macOS arm64 包；Windows/macOS 仅交叉构建。
- 包内附 LICENSE / NOTICE / README 及 src 对应源码；从解压后的独立 src 模块运行测试和构建通过。复现命令见 [彩云插件文档](../plugins/caiyun/README.md)。

## 0.3.0 功能清单与服务配置（2026-11-23）

对应 [功能清单](features.md) 的 SVC-02 / SVC-03 / CFG-02。

- 全项目 `CGO_ENABLED=0 go test ./...` 通过；`go test -race ./internal/... ./pkg/... ./plugins/...` 通过。
- `go vet ./...`、`go tool mygo vet .` 通过。
- 服务测试覆盖：未启用草稿、固定文本、成功/失败/超时、取消、编辑/移除/放弃/关闭后的过期结果、插件参数快照、不覆盖翻译内容且不落配置/历史；测试按钮与 Esc 使用 Tester 验证。
- 服务排序通过 UI 动作验证；实例字段保持、边界不越界、保存加载顺序及 translator 创建顺序均通过。
- 设置通过真实控件输入验证数字范围、语言选择、保存、重启恢复和放弃修改；当前原文和临时语言选择保持。
- 离屏渲染检查设置页、服务页及测试结果卡片，1100×800 和 920×680 下保存按钮可见，内容区滚动正常。
- 真机限制保持：离屏 UI 不等于系统桌面验收；测试按钮调用真实服务仍依赖用户网络/服务账号，默认自动化测试不调用供应商接口。

产物验证：Linux 0.3.0 的可执行文件、deb、rpm 和 tar.gz 已生成；Windows amd64、macOS arm64 交叉编译通过。原生桌面与真实服务连通性仍按上述待实机条件验收。

## 0.4.0 后台常驻与独立窗口（2026-11-23）

用户已确认上一版本在当前 Linux 环境能够截图 OCR、能够联网翻译。这是用户实测反馈；尚未记录具体服务、桌面/门户版本和多屏条件，不据此关闭所有平台验收项。

本轮自动化：

- Linux MyGo 0.4.0 发布构建通过，生成可执行文件、deb、rpm、tar.gz；Windows amd64 与 macOS arm64 交叉编译通过。

- `CGO_ENABLED=0 go test ./...`、`go test -race ./internal/... ./pkg/... ./plugins/...`、`go vet ./...`、`go tool mygo vet .` 均通过。
- 窗口策略测试覆盖：关闭只隐藏、翻译与配置独立唤起、明确退出不被拦截、截图前四种可见组合、截图期间延迟显示/关闭请求、退出禁止恢复。
- 双视图 Tester 覆盖：配置主窗无翻译操作、导航与未保存草稿隔离、翻译入口与空剪贴板不误开配置、截图取消与失败提示、关闭翻译取消任务后还能继续使用、输入焦点、退出按钮、Esc 两阶段行为。
- 已检查配置 1100×800 / 920×680、翻译 520×620 / 460×480，以及历史/收藏、忙碌按钮、译文/错误和深色主题的离屏渲染；产图位于本次环境 `/tmp/lumin-v040/`（临时文件不随仓库分发）。

目标桌面复验清单：

1. 启动只显示配置主窗口，左侧只有翻译服务、插件、设置；设置页修改但不保存，关闭后用托盘或再次启动找回，草稿仍在。
2. 关闭全部窗口，确认进程继续运行；输入快捷键只显示小窗口并能直接输入，设置窗口保持隐藏；重复关闭/唤起至少三次。
3. 使用剪贴板快捷键翻译，检查结果和历史。点击小窗口设置，两个窗口可以同时存在且导航互不干扰。
4. 设置隐藏/可见与翻译隐藏/可见四种组合下分别截图确认、取消；快照不含 Lumin 窗口。后台截图成功只开小窗口，取消不显示设置。拒绝授权时小窗口显示错误。
5. 翻译中关闭小窗口或按 Esc 取消，确认旧结果不覆盖下次输入；空闲 Esc 隐藏。关闭设置窗口后服务测试仍可完成。
6. 主动打开剪贴板监听后，隐藏两个窗口，复制外部文本唤起小窗口；复制译文不重复触发。重启后监听默认关闭。
7. 启用 HTTP 并重启：所有窗口隐藏时 `/translate`、`/input_translate` 打开小窗口，`/config` 打开设置，OCR 端点执行相同截图流程。
8. 托盘不存在的桌面通过重新运行唤起设置并退出；正常桌面分别从托盘和设置点击退出，确认进程与快捷键、监听端口消失。截图/请求进行中退出，无遗留窗口。
9. 重新启动检查两个窗口分别恢复尺寸/位置；Windows/macOS 仍需对应真机检查关闭、Dock 激活和系统退出。

本轮也使用独立 `LUMIN_CONFIG_DIR=/tmp/lumin-v040/startup` 启动 Linux 产物，仍返回 `mygo: cannot open display`，因此无法在工具环境完成原生关闭与托盘实测。测试新版前先退出旧版进程，避免单实例机制唤起旧版。

此处的离屏和策略测试不代表窗口管理器、托盘或全局快捷键已完成实机验收。

## 0.5.0 窗口功能归属（2026-11-23）

用户确认：安装 libayatana-appindicator 后，当前 KDE Wayland 环境托盘可以正常使用。当前系统中的 `libayatana-appindicator3.so.1` 已通过动态加载检查；工具环境仍无法访问宿主显示和会话总线，未声称已在工具中实测桌面图标。

- 全项目 `CGO_ENABLED=0 go test ./...`、`go test -race ./internal/... ./pkg/... ./plugins/...`、Go vet、MyGo vet 通过。
- Linux 0.5.0 可执行文件、deb/rpm/tar.gz 已生成；Windows amd64、macOS arm64 交叉编译通过。
- 原有服务测试、排序、插件字段、草稿保存、截图/OCR/翻译链路继续通过。
- 新增窗口位置回归：主窗口配置分类、服务子页和历史；翻译窗不存在历史/收藏/设置导航及截图工具栏；历史收藏过滤及载入打开小窗口而不改主窗口页或草稿；置顶指令、原文复制和剪贴板反馈保护。
- 新增托盘诊断测试：库缺失的完整退出提示、无桌面服务/无面板、非法返回、其他应用图标、当前进程注册、项目消失；检查任务的单实例执行和退出取消。
- 已检查 520×620 / 460×480 翻译窗口和 1100×800 / 920×680 主窗口渲染；页面截图在 `/tmp/lumin-v050/`。

本轮真机复验重点：重新启动新版，托盘菜单包括图片识字；主窗口“服务设置”可切换翻译/OCR/插件；历史载入唤起小窗口；小窗口置顶切换生效且关闭后仍可快捷键唤起。完整功能位置以 [window-map.md](window-map.md) 为准。独立 Recognize 结果窗口仍是 UI-04 待迁移项，当前 OCR 文本继续进入翻译编辑器。

## 0.6.0 独立识别窗口

- UI-04：新增 recognition_window_test.go，覆盖识字路由、保留图片重试、编辑结果转翻译、复制防剪贴板循环、关闭拒绝迟到结果且不影响翻译、取消截图不弹窗。
- windows_test.go 增加第三业务窗口的截图隐藏/恢复、延迟打开、截图期间关闭与真实退出验证。
- 检查 480×560 原生离屏渲染（`LUMIN_TEST_SCREENSHOTS` 输出 recognition.png）；按钮和编辑器边界测试覆盖忙碌/空闲切换。
- `CGO_ENABLED=0 go test ./...`、`go test -race ./internal/...`、`go vet ./...`、`go tool mygo vet .` 均通过。
- 实机仍需复验：托盘图片/截图识字打开识别窗；截图翻译直达翻译窗；三个窗口可见时截图无自身窗口；识别运行中关闭不再弹回；修改结果后转翻译。自动化测试使用模拟 OCR，不代表真实 Tesseract/插件服务或 Windows 桌面已验收。
- 构建：MyGo Linux amd64 0.6.0（deb/rpm/tar.gz）及 Windows amd64 0.6.0 均成功；Windows ZIP 包含 EXE 和图标，PE 架构与 ZIP 完整性验证通过。未在 Windows 实机运行。

## 0.7.0 备份恢复

- `internal/backup`：完整/默认去凭据往返、不修改源配置、未知版本/字段、损坏 JSON、尾随内容、重复历史 ID、超大输入与失败导出不覆盖。
- `internal/storage`：旧双文件迁移、快照重新打开、恢复前备份往返、模拟原子提交失败、恢复备份写入失败、revision 拒绝迟到历史、导出不能覆盖数据目录文件、坏快照不回退。
- `internal/desktop`：默认导出无密钥、坏预览清除旧确认、预览不写入、必须确认覆盖、真实复选框/恢复按钮、使用已校验内存内容、恢复后设置/草稿/历史与快捷键刷新；检查 920×680 预览页离屏图。
- 全项目 CGO_ENABLED=0 测试、internal 竞态测试、Go vet、MyGo vet 均通过。最终按钮测试与版本变更后补跑相关 desktop 测试通过。
- MyGo Linux amd64（deb/rpm/tar.gz）与 Windows amd64 0.7.0 构建成功；Windows ZIP 完整性和 PE AMD64 检查通过。
- 未验证真实原生文件对话框、Windows 实机、断电或文件占用。实机建议先导出完整备份到数据目录之外，在独立 LUMIN_CONFIG_DIR 测试恢复及恢复前备份再导入。

## PP-OCRv6 tiny 插件 1.0.0（2026-11-24）

插件 `lumin.ppocrv6-tiny` 使用宿主 0.7.0 的协议 v1，不修改宿主协议或配置 schema。复现命令：

```bash
GOCACHE=/tmp/lumin-gocache GOTMPDIR=/tmp CGO_ENABLED=0 \
LUMIN_PPOCR_PACKAGE="$PWD/build/plugins/lumin.ppocrv6-tiny-1.0.0-linux-amd64.luminext" \
go test ./plugins/ppocrv6-tiny -v -count=1
```

结果：`PASS`（本次环境约 1.25 秒）。`TestInstalledPluginRealModels` 通过 `internal/plugin.Manager` 将真实包安装到含空格的临时目录，清空 `PATH` 后启动真实子进程并完成推理；英文样例包含 `Lumin translation test.`，中英文混排样例包含 `Lumin 离线文字识别测试`、`你好世界`、`Hello world` 和 `12345`。同一测试还覆盖取消、无效 `threads` 设置和无字图片错误。

- Linux amd64 包：`build/plugins/lumin.ppocrv6-tiny-1.0.0-linux-amd64.luminext`，10,474,412 字节，SHA-256 `a1fb68ba56bf835eec075bf97a94d2f58e6c5f4bb3459f50e157854ced63cf65`；包含 `bin/plugin`、`lib/librapidocrlite.so`、检测/识别 ONNX、字典、ABI 头文件和许可证，共 18 个文件。
- Windows amd64 包：`build/plugins/lumin.ppocrv6-tiny-1.0.0-windows-amd64.luminext`，9,312,658 字节，SHA-256 `12b0307008d95d70ef333cf6442d00e2c287816f5b15f7b0480dd3dd89ed33ad`；包含 `bin/plugin.exe`、`lib/rapidocrlite.dll` 和相同模型资产，共 18 个文件。仅完成交叉构建和包内容检查，未在 Windows 实机运行。
- 资产由 `plugins/ppocrv6-tiny/assets.lock.json` 固定 SHA-256；模型字典从识别 ONNX 的 `character` 元数据提取并校验。运行时不需要 Python、Tesseract 或网络。
- 两个包均通过 `unzip -t`；包内 `detection.onnx`、`recognition.onnx`、`dictionary.txt` 的 SHA-256 分别为 `f42c0fbd294d95eac1a550e131b277dac97462c8025fa4b6c3cec1b7894bd3d5`、`e16e242de5937ad92609223f19bc2aff3727ee40b095f996907c24749bad251b`、`34d139222b6e8d84830f57476e39f01e76be3baffde3e9df75a079e251140598`，与锁定文件一致。
- 已知限制：RapidOCRLite 5.0.0a16 是预发布专用解释器，不支持任意 ONNX；没有 macOS/ARM 包；没有 180° 方向分类；每次识别启动新插件进程，没有常驻模型缓存；只向宿主返回文本，不返回框或置信度元数据。

## 鼠标所在显示器截图（2026-11-24）

- `internal/capture/display_test.go` 覆盖横排右侧、竖排上方负坐标、负 X、缩放桌面、已裁剪为单屏和单显示器；另验证裁剪后的像素来源与空目标回退。
- `internal/bootstrap/capture.go` 在隐藏 Lumin 窗口前读取鼠标与显示器，后续截图和框选使用同一 `capture.DisplayTarget`；Linux 通过 GTK3 `gtk_window_fullscreen_on_monitor` 将覆盖层放到目标显示器。
- `GOCACHE=/tmp/lumin-gocache GOTMPDIR=/tmp CGO_ENABLED=0 go test ./...`：通过。
- `GOCACHE=/tmp/lumin-gocache GOTMPDIR=/tmp go test ./...`：通过。
- `GOCACHE=/tmp/lumin-gocache GOTMPDIR=/tmp go test -race ./internal/... ./pkg/... ./plugins/...`：通过。
- `GOCACHE=/tmp/lumin-gocache GOTMPDIR=/tmp go vet ./...`、`go tool mygo vet .`：通过。
- Windows amd64 与 macOS arm64 的 `internal/capture`、`internal/bootstrap` 测试包交叉编译通过；Windows PowerShell 使用目标 HMONITOR 的 `GetMonitorInfo` 物理边界，失败时才回退鼠标位置。
- 未在当前沙箱完成真实多屏合成器验收：仍需在 Linux KDE Wayland 依次把鼠标移到各屏，确认只出现目标屏快照且覆盖层不跨屏；混合 DPI、Windows/macOS 多屏和 macOS 录屏授权均待对应真机验证。

## 0.8.0 历史单条删除与导出（DATA-03）

- `internal/storage/history_actions_test.go`：按 ID 删除持久化、收藏保护、缺失 ID、提交失败保留原数据；导出不包含配置，保护活动数据路径，未知格式失败保留旧导出文件。
- `internal/historyexport/export_test.go`：JSON Unicode/多行完整往返，CSV 逗号/引号/换行、多服务及失败结果、一条历史无服务结果时仍保留；空导出与未知格式。
- `internal/desktop/history_actions_test.go`：真实删除按钮和收藏二次确认、取消、删除后载入列表、草稿与翻译输入隔离；点击导出时固定搜索/收藏筛选及深复制结果；原生保存取消/错误，恢复期间互斥。
- 已检查 920×680 历史页离屏图 `history-actions.png`（使用 LUMIN_TEST_SCREENSHOTS 输出），确认控件边界与文字完整；覆盖额外重建帧及删除后列表变化。
- `CGO_ENABLED=0 go test ./...`、`go test -race ./internal/...`、`go vet ./...`、`go tool mygo vet .` 全部通过，包含用户新增的多屏坐标测试。真实桌面文件保存和 Windows 运行仍待实机验证，多屏平台限制沿用 CAP-05 记录。
- MyGo Linux amd64 0.8.0 deb/rpm/tar.gz 与 Windows amd64 构建成功；Windows 便携 ZIP 的完整性和 PE AMD64 架构校验通过。

## 0.9.0 插件 OCR 默认可用与设置合并

- `ocrsetup/setup_test.go`：首次安装并持久化默认插件、安装时不执行入口、既有引擎和主动停用保留、缺少资源不回退系统；真实 PP-OCRv6 tiny 包安装后清空 PATH 仍可识别英文固定样例。
- `desktop/ocr_settings_test.go`：宿主无系统 OCR 回退、无引擎时不截图并直达服务设置/OCR 服务；重复识别导航移除；草稿选择/字段编辑/停用/放弃；独立测试不修改原文/识别结果/历史/配置，临时文件清理，取消拒绝迟到结果。
- `config/config_test.go`：新配置无需系统语言配置，旧 ocr_languages 与新增可选 ocr_disabled 兼容。
- 全项目 `CGO_ENABLED=0 go test ./...`、`go test -race ./internal/...`、Go vet、MyGo vet 通过。移除独立文字识别导航后补跑 desktop 全测试通过，并重新检查 MyGo vet。
- 使用 LUMIN_TEST_SCREENSHOTS 输出并检查 920×680 最小服务设置布局。真实 OCR 通过 LUMIN_PPOCR_PACKAGE 指定包运行；测试未发送当前截图或调用系统工具。
- 运行库当前仍按请求加载；未引入常驻模型缓存。桌面托盘入口、原生窗口/文件对话框与 Windows 模型运行仍待实机；用户已有多屏改动与相应平台限制保留。
- 最终 Linux/Windows amd64 0.9.0 构建成功；Linux tar.gz 与 Windows 便携 ZIP 均包含对应平台默认 OCR 插件，嵌套包完整性和清单平台检查通过，Windows PE AMD64 校验通过。

## 0.10.0 全屏框选体验（CAP-06）

- 采用 mouse-up 提交方式，取消顶部工具栏；画布在 1280×720 下精确覆盖 (0,0)-(1280,720)，检查输出 selection.png，拖动时提示直接绘制在画布。
- `selection_test.go`：松开自动提交、反向拖动、原图精确坐标、重复 Enter/Esc/右键只完成一次、误点重选、CancelPress 清除且可重选、拖动中右键/Esc 取消、整屏边缘/留白比例映射、尺寸变化不改变已选原图坐标。
- `capture_options_test.go`：覆盖层 Frameless + Hidden + AlwaysOnTop，保留目标负坐标与尺寸，几何信息不可用时仍有有效兜底。
- 全项目 CGO_ENABLED=0 测试、desktop/bootstrap/capture 竞态测试、Go vet、MyGo vet 通过。保存截图前修正了 float32 归一化导致部分右/下边缘多 1 像素的问题。
- 多屏、混合 DPI、各桌面合成器、Windows/macOS 全屏与标题栏实际表现仍需真机验收。保留既有按鼠标显示器抓取与 GTK 指定显示器全屏逻辑，未宣称离屏图等同实机全屏验证。
- Linux/Windows amd64 0.10.0 构建成功，Linux tar 与 Windows 便携包均保留默认 OCR 插件；Windows ZIP 完整性及 PE AMD64 验证通过。

## 0.11.0 运行时工具独立化（DEP-01）

日期：2026-11-24。以下为当前实现证据；前面 Tesseract、grim/maim、PowerShell、screencapture 的记录只描述历史版本，相关实现已移除。

- Linux 全量 Go 测试通过，指定真实 PP-OCR 包的清空 PATH 模型推理与默认引擎安装测试通过。
- capture/plugin/ocrsetup race 检查、Go vet、MyGo vet 通过。
- Windows amd64、macOS arm64 应用交叉构建通过；Windows capture/plugin 测试包及 macOS capture 测试包交叉编译通过。
- 新增源码依赖守卫、插件私有 PATH、历史 Tesseract 引用迁移及截图提前取消回归。
- Windows 原生 Job Object 进程树取消测试已实现并编译，未在 Windows 执行。
- 用户已确认 0.10.0 Linux 全屏框选交互正常。本轮保留 Linux 门户流程，移除工具回退；Windows GDI、macOS 14+ ScreenCaptureKit 的真实截图、权限与混合 DPI 尚待真机。
- macOS/ARM 暂无随包离线 OCR 插件，原生截图实现不代表默认 OCR 已覆盖这些平台。
- 发布构建：0.11.0 Linux tar/deb/rpm、Windows amd64 便携 ZIP 已生成；核验 tar/ZIP 含默认 OCR、ZIP 完整性与 EXE 的 PE AMD64 标记通过。

## 0.11.1 截图延迟与热键显示（PERF-01 / UI-04）

- 全量 Go 测试含真实 PP-OCR 包通过；bootstrap/capture/desktop race、Go vet、MyGo vet 通过。
- 热键 UI 回归覆盖平台显示、多个构建帧、草稿编辑、保存、页面切换、放弃与禁用；1100×800 热键页渲染已检查。
- 所有三个业务窗口显隐组合覆盖：仅实际隐藏可见窗口时需要截图前等待；背景截图跳过 200ms，仍不改变显示器选择和恢复规则。
- Full HD 固定样例 BenchmarkTempPNGFullHD，同机单轮各 3 次：默认压缩约 58.2ms/op，BestSpeed 约 36.3ms/op。只代表选区编码，不代表真实截图/识别总耗时。
- 显式原生截图测试已尝试，执行环境无法连接桌面会话总线，未获得真实门户耗时；不能据此宣称用户桌面延迟已解决。通过 LUMIN_CAPTURE_TRACE=1 可在用户桌面区分系统取屏、解码、窗口准备、编码与 OCR。
- 0.11.1 Linux tar/deb/rpm 与 Windows 便携 ZIP 已生成，tar/ZIP 随包 OCR、ZIP 完整性、Windows PE AMD64 标记通过。

## 0.12.0 选中文字翻译（SEL-01）

- 全量 Go 测试（含真实 PP-OCR 包）通过；selection/desktop/config race、Go vet、MyGo vet 通过。
- Linux AT-SPI 活跃窗口/焦点与祖先文本委托回归通过；不读取非活跃窗口和无关兄弟选区，空选区不回退旧文本；循环和 Unicode 20000 字符边界有测试。
- 窗口测试覆盖先获取选区再弹窗、直接翻译到小窗、无剪贴板读取/写入、失败不翻译旧内容、重复触发、取消和迟到结果；旧 v1 快捷键 JSON 键/禁用状态兼容测试通过。
- 已检查 1100×800 热键页面渲染，标签为选中文字翻译，显示平台实际按键和无需复制的提示；剪贴板监听控件及订阅移除。
- Windows amd64、macOS arm64 原生选区代码应用交叉编译通过；真实系统和应用覆盖尚未验证。当前执行环境无法访问桌面会话总线，不能宣称 Linux 取词已实机通过。
- Linux 0.12.0 tar/deb/rpm、Windows amd64 便携 ZIP 已生成；随包 OCR、ZIP 完整性和 PE AMD64 检查通过。原生接口、权限和验收步骤见 selection-translation.md。

### 0.11.1 用户截图日志补充

用户反馈速度改善，并提供 /tmp/lumin-capture.log 的 5 次记录：screen 467–734ms，平均 625.8ms，其中 decode-screen 平均 95.8ms；show-overlay 平均 6.2ms、选区编码平均 1.2ms、OCR 平均 130ms。约 530ms 位于解码之外的取屏链路，尚无更细阶段归因；不把框选等待或系统首帧呈现算作这些计时覆盖范围。

## 0.12.1 VS Code / Wayland 取词修复

用户确认 0.12.0 在 VS Code 中弹窗但选区为空或不对；先前“只用 AT-SPI”方案不能覆盖此应用。新增原生 Wayland PRIMARY 路径，支持 ext-data-control-v1 与 wlr-data-control v2；不可用时再尝试 AT-SPI，明确空选区不回退。对 VS Code 选区发布的约 100ms 调度延迟，保留源焦点等待 120ms 再读取。失败清空旧原文，避免误以为获取成功。

- 全量 Go 测试（含真实 OCR 包）、Go vet、MyGo vet 通过；desktop race 通过。
- 模拟合成器使用真实 Unix socket / SCM_RIGHTS 文件描述符传输，ext/wlr 两协议成功取词、普通 CLIPBOARD 与 PRIMARY 隔离、空/缺失/非文本选区、分帧、取消和回退策略通过。
- race 检查发现 FD deadline 与 context timer 同时到期时错误类型不稳定；已统一到期语义，selection race 连续 10 轮通过。
- 0.12.1 Linux tar/deb/rpm 与 Windows amd64 便携包已生成，检查随包 OCR、ZIP 完整性、PE AMD64 通过。
- 当前环境不能访问真实 Wayland 合成器；以上是协议模拟与构建证据，不能宣称 VS Code 真机验收通过。新版本启用 LUMIN_CAPTURE_TRACE 或 LUMIN_SELECTION_TRACE 后记录触发/后端/耗时与错误，不记录选中文字，便于后续复验。

## 0.13.0 翻译窗口交互与外观（UI-05 / UI-06 / UX-01）

- 用户已确认 0.12.1 在 VS Code 取词成功；本轮保留该读取路径，并增加读取新选区期间的失焦保护。
- 全量 Go 测试含真实 OCR 通过；desktop/bootstrap/config race、Go vet、MyGo vet 通过。
- 窗口策略覆盖默认自动隐藏/关闭开关/置顶、取消任务但保留内容、重新聚焦/截图/隐藏/退出过滤；真实 timer + 主线程派发队列验证旧回调不能隐藏新的交互。
- 配置测试覆盖 v1 缺省与范围、保存/重启/放弃、预设恢复相同默认尺寸、主题保存保留手动大小、字体与外观应用；快捷键差异判断防止外观保存重新授权。
- 已检查浅色/深色翻译窗与设置页渲染。460×480、520×620、640×720 下核心控件边界与原文折叠额外帧通过；正文/次要文字/错误文字/主按钮的双主题配色对比至少 4.5:1。
- Linux/Windows amd64 0.13.0 发布构建及 macOS arm64 应用交叉编译通过；Linux tar 与 Windows ZIP 含默认 OCR，ZIP 完整性与 PE AMD64 校验通过。
- 当前执行环境不能连接用户真实桌面；OnBlur/AlwaysOnTop、系统主题动态变化、原生菜单/标题栏与退出后尺寸记忆仍待各系统真机验收。ui.Tester 图像和交叉编译不能替代这些桌面集成验证。规则与验收矩阵见 ux.md。

## 0.13.1 紧凑阅读布局

- 使用 MyGo 原生控件实现默认 350×420 的紧凑阅读布局，原文摘要与译文卡片分区。
- 全量 CGO_ENABLED=0 go test ./...、Go vet、MyGo vet、git diff --check 通过。本轮未改变 OCR 执行链；未重复运行需显式开启的真实模型集成测试。
- 新增 320×360、350×420、420×520 双主题回归：首个译文位于首屏上部、主要按钮可见、正确复制、编辑与阅读模式额外构建帧不丢失内容；有效请求进入阅读模式，缺少服务仍留在编辑模式，手动输入入口重新聚焦。
- 已人工查看 浅色与深色紧凑窗口渲染。Linux amd64 tar/deb/rpm 与 Windows amd64 发布构建成功；Windows ZIP 完整性、PE AMD64 和 Linux tar 随包 OCR 检查通过。
- 保留旧版配置尺寸与原生窗口大小记忆；已有用户需在「翻译设置 → 紧凑 → 保存设置」应用 350×420。实际桌面标题栏、焦点/置顶等平台行为仍需真机确认。

## GitHub 源码整理（2026-11-24）

- 移除旧应用参考源码目录、过时迁移清单、编辑器工作区文件和旧版本发布归档；保留当前 Lumin 代码、测试、图标、插件源码、来源与许可证。旧归档共 11 个，约 124.2 MiB；最新版产物与资源下载缓存保留在忽略目录。
- 插件导入只使用统一 .luminext 校验，移除旧扩展名特判；相应回归通过。移除界面和当前维护文档中的旧应用品牌与失效路径，窗口文档改为 Lumin 自身职责说明。
- 扩充 .gitignore，覆盖构建、开发缓存、生成 OCR 包、本地配置、备份、凭据目录、编辑器文件和日志。新增 publishing.md 记录源码范围、构建和提交检查。
- 154 个当前可发布文件检查中无超过 10 MiB 的文件；常见私钥/GitHub/AWS/OpenAI 凭据格式扫描无命中。此检查针对工作树，不涵盖历史提交，也不代表完整密钥审计。彩云源码中的公共客户端默认参数按原有来源说明保留。
- 本地 Markdown 链接检查、全量 CGO_ENABLED=0 go test ./...、Go vet、MyGo vet、git diff --check 通过。
- go mod tidy 因模块缓存只读且缺少上游测试依赖未完成；未删除或升级依赖，仅将直接导入的 purego、x/sys 归入直接依赖组。现有模块可完成以上全部测试和静态检查。
- 当前环境禁止写入 Git 索引；原已跟踪的 .agents 仍需维护者运行 git rm -r --cached .agents，再 git add -A 统一暂存新增实现与删除文件。未创建提交、改写历史或上传远程。

## GitHub 三平台构建与发布（2026-11-24）

- 新增按版本标签触发、也可手动指定已有标签的工作流；目标为 Linux amd64、Windows amd64、macOS arm64。安装包携带彩云和 PP-OCRv6 tiny 两个插件，Release 同时提供每个平台的两个独立插件包与 SHA256SUMS。
- 发布工具的 6 项单元测试通过，覆盖标签/提交校验、完整资源集合、预发布包命名、缺少随包插件、平台入口/插件身份/许可源码检查，以及 macOS SDK 安全读取。工作流 YAML 和全部 Bash 步骤语法检查通过。
- 全量 CGO_ENABLED=0 Go 测试、Go vet 和 MyGo vet 通过；使用现有 OCR 包运行 internal/ocrsetup、plugins/ppocrv6-tiny 与 internal/plugin 的真实包测试通过。彩云协议测试不发送联网翻译请求。
- 本地重新编译彩云插件，并使用已有 OCR 资源生成 Linux 0.13.1 tar.gz、DEB、RPM；Linux 资源收集通过，检查了两个随包插件，并生成两个独立 .luminext 发布资源。
- 当前环境外部下载地址解析失败，完整下载加打包脚本未能从零执行。Windows NSIS 安装程序、macOS 原生 DMG/签名与三平台 GitHub runner 的完整流程，仍需首次推送标签后验证；未创建远程 Release。macOS SDK 固定上游下载地址与 SHA-256，本地仅完成模拟 SDK 归档测试，不能代替原生运行验收。


## CI 首次构建故障修复（2026-11-24）

- 修复 OCR 构建脚本通过 go list 读取未下载模块时得到空目录的问题；改为显式下载 go.mod 固定版本模块并读取 JSON 中的 Dir，检查目录及 LICENSE，保留全部依赖许可。使用当前真实模块缓存验证三份许可证读取与复制成功。
- 修复 Windows C 盘临时目录到 D 盘工作区打包失败的问题：不同卷直接判定为外部输出，相同卷保留目录包含关系检查。打包临时文件仍建在输出目录，最终重命名不会跨盘。
- 8 项 Python 发布脚本测试、插件打包/重复输出/嵌套输出回归、全量 CGO_ENABLED=0 Go 测试、Go vet、MyGo vet 与 git diff --check 通过。许可证回归模拟空缓存并验证显式下载后复制，覆盖缺少模块目录、下载错误与缺少许可证拒绝打包。
- Windows 专属回归覆盖 C→D、同卷外部路径、UNC 共享输出及大小写不同的嵌套路径；Windows amd64 测试二进制交叉编译通过，工作流已安排每个平台执行打包器测试。本机没有 Windows 运行环境，跨磁盘回归的实际执行和完整安装包构建仍需 GitHub runner 验证。
