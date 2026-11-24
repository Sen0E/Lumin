# 选中文字翻译与维护

适用 Lumin 0.12.1。先在原应用里选中文字，再按「选中文字翻译快捷键」（默认 Linux/Windows Ctrl + Alt + T，macOS Cmd + Alt + T）。不需要复制；成功后才聚焦翻译小窗并自动调用已启用的翻译服务。托盘同名入口走相同流程，但部分桌面菜单会改变焦点，优先使用快捷键。

没有有效选区、接口不支持或权限不足时清空旧原文并显示提示，不翻译旧输入和旧剪贴板。当前只支持一段连续选区，上限 20000 个 Unicode 字符。图片、扫描 PDF 或未开放文本接口的应用可用截图翻译。编辑器中的普通粘贴/复制保留；自动监听剪贴板已移除。

## 平台接口与限制

| 平台 | 原生读取方式 | 使用条件与边界 |
| --- | --- | --- |
| Linux Wayland（优先） | 直接连接 compositor 的 Unix socket，通过 ext-data-control-v1 或 wlr-data-control v2 读取 PRIMARY 选区、以 FD 接收 UTF-8 文本 | 桌面需开放数据控制协议且目标应用发布原生选区；适用于 VS Code/Monaco 等不提供可靠 AT-SPI 文本选区的应用，不需要 wl-paste |
| Linux X11 / Wayland（辅助功能路径） | 通过会话总线取得 AT-SPI 总线，定位活跃窗口的焦点对象，调用 Text.GetSelection/GetText；焦点对象无 Text 接口时向其祖先查找 | 桌面及目标应用必须提供辅助功能树和文本选区；关闭辅助功能、禁用可访问性的浏览器/Electron 或不支持此接口的应用会取词失败。不会自动修改系统辅助功能开关 |
| Windows | UI Automation GetFocusedElement → TextPattern.GetSelection → TextRange.GetText | 目标控件需支持 TextPattern；某些自绘控件、终端和权限更高的程序可能不可读。没有模拟 Ctrl+C 回退 |
| macOS | AXFocusedUIElement → AXSelectedText | 需在系统设置的隐私与安全性 → 辅助功能中授权实际运行的 Lumin；目标应用需开放该属性 |

Linux 不读取普通 CLIPBOARD。Wayland 的 PRIMARY 是鼠标选中后发布的原生选区通道，无需 Ctrl+C；0.12.0 仅使用 AT-SPI 的方案无法覆盖部分 VS Code/Monaco 场景，0.12.1 改为优先读取该通道。每次创建独立连接并读取当前 offer，不缓存旧文字；收到空选区直接报空，不用 AT-SPI 的另一份文本代替。协议不可用或连接失败时才尝试 AT-SPI。

PRIMARY 的清除和生命周期由源应用/桌面决定。应用未清除已发布的选区时，接收方无法判断用户是否已经在其他位置取消高亮；不能承诺跨任意应用严格判断“当前视觉高亮”。宿主不会用普通剪贴板历史替代该选区，也不向桌面写入数据。AT-SPI 路径仍只读活跃窗口的焦点或祖先，密码控件拒绝读取。

本实现不依赖 xclip、wl-paste、xdotool、PowerShell、AppleScript 或其他本机工具。不会改写剪贴板，不会模拟复制或按键，不需要恢复剪贴板格式。平台接口来自系统服务，并非 `.luminext` 模型能力。

## 架构与兼容

- `internal/selection`：平台读取和文本校验。Wayland 原生通道最多 1.2 秒，FD 读取最多 1 秒且随 context 取消，普通剪贴板 offer 永不请求；接口不可用后尝试辅助功能。Linux 辅助功能读取有 3 秒 context 上限、单次 D-Bus 调用 250ms、节点/队列限制和循环去重；macOS 设置 AX 消息超时；Windows UIA 原生调用由 provider 响应，context 不会强行中断正在执行的 COM 调用。
- `desktop.Ports.ReadSelection`：后台获取能力；测试可替换成固定文本或阻塞读取器。
- `desktop.TranslateSelection`：先获取，再弹窗，复用 TranslateText/Translate。取词期间忽略重复触发；取消、编辑、关闭、保存或退出使旧代次失效。原生读取晚到也不能覆盖新输入或弹回窗口。
- `bootstrap`：快捷键和托盘绑定；不在读取之前调用 ShowTranslator，不监听剪贴板变化。

配置版本仍为 1，Go 字段已改为 `SelectionShortcut`。持久化键保留 `clipboard_shortcut` 以兼容现有配置和备份，仅其动作语义改为选区翻译。旧按键字符串和显式空值保持，不自动重置；未来升级配置 schema 时再统一迁移键名。

## 验证

```bash
CGO_ENABLED=0 go test ./internal/selection ./internal/desktop ./internal/config ./internal/capture
go test -race ./internal/selection ./internal/desktop
go tool mygo vet .
```

自动化覆盖活跃窗口/焦点、祖先委托、旧兄弟选区隔离、空选区、Unicode 上限、循环、读取前不弹窗、重复触发、翻译目的窗口、无剪贴板回退、取消与迟到结果、错误提示及旧配置键兼容。Windows/macOS 只完成交叉编译，Linux 当前执行环境不能访问真实桌面会话总线/Wayland 合成器；自动化不代表任何真实应用已验收。

真机检查：

1. 退出旧实例并启动新版；在浏览器和文本编辑器分别选中不同的中英文文字，直接按快捷键，确认小窗自动翻译且设置窗不弹出。
2. 先复制一段不同的文字，再选中文字触发；结果必须来自选区，普通粘贴仍是原来复制的文字。
3. 清除选区后触发，确认不自动翻译此前的选区或剪贴板；非活跃窗口和另一输入框的旧选区也不能使用。
4. 不支持的应用显示接口提示；macOS 拒绝/允许辅助功能授权，分别验证。
5. 取词中关闭翻译窗、取消或修改输入，确认迟到结果不覆盖、不重开；按键冲突或桌面未绑定时先处理全局快捷键授权。

接口参考：[AT-SPI Text](https://gnome.pages.gitlab.gnome.org/at-spi2-core/devel-docs/doc-org.a11y.atspi.Text.html)、[AT-SPI Accessible](https://gnome.pages.gitlab.gnome.org/at-spi2-core/devel-docs/doc-org.a11y.atspi.Accessible.html)、[Windows TextPattern](https://learn.microsoft.com/en-us/windows/win32/api/uiautomationclient/nn-uiautomationclient-iuiautomationtextpattern)、[macOS AXSelectedText](https://developer.apple.com/documentation/applicationservices/kaxselectedtextattribute)。

## VS Code 与 Wayland 排错（0.12.1）

0.12.0 的单一 AT-SPI 取词路径兼容性不足。0.12.1 优先使用原生 PRIMARY，VS Code 的 `editor.selectionClipboard` 应保持启用（设置中搜索 Selection Clipboard）；这是选中即发布选区，不是要求手动复制。若桌面没有开放数据控制协议，仍需依赖辅助功能；不能保证任意 Wayland 桌面均可后台读取选区。

先从托盘退出旧实例，启动新版并记录日志，再在 VS Code 编辑器选中一段新的文字按取词快捷键：

```bash
LUMIN_CAPTURE_TRACE=1 ./build/linux-amd64/lumin 2>&1 | tee /tmp/lumin-capture.log
```

也可用独立开关 `LUMIN_SELECTION_TRACE=1`。日志出现 `[selection] trigger` 表示入口已触发；`wayland-primary` 给出原生通道结果及耗时，`read` 给出最终读取结果。不记录选中文字本身。没有 trigger 时先排查快捷键绑定；有 trigger 而无有效内容时检查后端结果及应用选区设置。

`primary_wayland_linux_test.go` 通过模拟合成器和真实 Unix socket / SCM_RIGHTS 传递数据，专门将普通剪贴板和 PRIMARY 设置为不同的内容。覆盖 ext/wlr 两种协议、分帧、空/缺失/非文本选区、取消及回退策略；这不是 VS Code 真机验收。

协议参考：[ext-data-control-v1](https://wayland.app/protocols/ext-data-control-v1)、[wlr-data-control-v1](https://raw.githubusercontent.com/swaywm/wlr-protocols/master/unstable/wlr-data-control-unstable-v1.xml)。

VS Code 的选区发布有约 100ms 的调度延迟（[原生选区实现](https://github.com/microsoft/vscode/blob/main/src/vs/workbench/contrib/codeEditor/electron-browser/selectionClipboard.ts)）。Wayland 读取前保留源焦点并等待 120ms，使快速选中后立即按键也能读到新选区；此等待仅用于取词，不影响截图。源应用未发布或未清除选区的边界仍由源应用决定。
