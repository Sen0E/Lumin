# 运行依赖与插件自包含约定

适用 Lumin 0.11.0。应用功能由宿主和 `.luminext` 插件提供，用户无需安装 OCR、截图或进程清理命令行工具。操作系统原生服务、基础共享库和权限仍然存在。

## 功能边界

| 功能 | 提供方 | 运行要求 |
| --- | --- | --- |
| 原生窗口、剪贴板、快捷键、托盘 | MyGo 与系统接口 | Linux 桌面运行库及桌面集成；托盘使用现有 AppIndicator 支持 |
| Linux 截图 | 宿主通过会话 D-Bus 调用 XDG Screenshot 门户 | 可用的 xdg-desktop-portal 和匹配后端、用户授权；无 grim/maim 回退 |
| Windows 截图 | 宿主调用系统 user32/gdi32 | 交互式桌面，固定显示器；不启动 PowerShell |
| macOS 截图 | 宿主调用 ScreenCaptureKit/AppKit/CoreGraphics | macOS 14+、屏幕录制权限；不启动 screencapture |
| 默认离线 OCR | PP-OCRv6 tiny 插件 | Linux/Windows amd64 发布包自带模型、字典、CPU 运行库，无 Python/Tesseract 或运行时下载 |
| 选中文字翻译 | Wayland 原生 PRIMARY / AT-SPI / Windows UIA / macOS AX | Linux Wayland 数据控制协议或辅助功能；目标应用发布选区；macOS 辅助功能授权；无复制模拟或工具调用 |
| 翻译、云端 OCR | 宿主服务或插件 HTTP 实现 | 用户配置服务地址/凭据及相应网络；离线 OCR 不需要网络 |
| 插件取消与退出清理 | Unix 进程组 / Windows Job Object | 直接调用系统接口，不启动 taskkill |

“无需额外工具”不等于静态链接所有操作系统组件。例如 Linux 离线 OCR 运行库仍依赖平台的 libc/libstdc++ 等基础库，原生窗口仍依赖桌面图形系统。模型/专用引擎应随插件分发，桌面接口应由宿主直接调用，不要求用户安装额外 CLI 作为功能前提。

## 插件打包要求

1. 通过清单声明平台入口；入口为包内可执行文件。需要的专用动态库、模型、字典及许可证一并打包，遵守插件包大小限制。
2. 宿主启动入口时使用绝对路径；PATH 依次为入口目录、插件根目录下的 `bin`、插件根目录。不继承本机工具搜索路径，也不继承 HOME、API Key 或代理环境变量。
3. 插件自行按包目录定位模型/动态库，避免依赖当前工作目录或系统安装的 Python/OCR 引擎。可选的辅助程序也必须随包提供并说明平台要求。
4. 本地离线插件不应在运行时下载必要资产。云服务插件允许按声明用途访问配置的 API。

私有 PATH 是依赖约定，不是权限沙箱。第三方可执行文件仍具备当前用户权限，可以使用绝对路径、网络和文件系统；安装时仍应确认来源。宿主不会把任意导入的第三方程序自动改造成自包含插件。

## 旧版迁移

删除 `example.tesseract` 开发示例及构建入口。配置仍指向 `example.tesseract@1.0.0` 时，有随包默认 OCR 的版本会自动改用 PP-OCRv6 tiny 并重置参数。主动停用及其他引擎选择保持；无随包资源时不修改配置、不启用系统回退，可在服务设置中手动换引擎。已经安装的旧插件文件不自动删除。

旧 CHANGELOG 和验证记录保留当时行为；以本说明及当前截图/插件文档为准。旧版本生成的压缩包不会自动更新，请使用 0.11.0 发布包。

## 开发与验证

构建时仍需要 Go/MyGo，组装 OCR 插件使用 Python 和资产获取流程；这些属于开发环境，不是分发后的运行依赖。发布脚本与固定资产哈希见 `scripts/build-release.sh`、`plugins/ppocrv6-tiny/assets.lock.json`。

```bash
CGO_ENABLED=0 go test ./...
go test -race ./internal/capture ./internal/plugin ./internal/ocrsetup
go tool mygo vet .
LUMIN_PPOCR_PACKAGE="$PWD/build/plugins/lumin.ppocrv6-tiny-1.0.0-linux-amd64.luminext" \
  CGO_ENABLED=0 go test ./plugins/ppocrv6-tiny ./internal/ocrsetup ./internal/desktop -count=1
```

`capture/dependencies_test.go` 检查所有平台的截图源文件、维护中的插件及宿主进程启动入口，防止引入命令行工具依赖；真实 OCR 测试在清空 PATH 后执行推理。Windows Job Object 有进程树取消测试，需 Windows 运行。交叉编译不能替代桌面验收。

可在真实桌面运行以下显式测试，会触发系统截图权限交互：

```bash
LUMIN_TEST_NATIVE_CAPTURE=1 go test ./internal/capture -run TestNativeCaptureOnDesktop -v -count=1
```

Windows/macOS 本轮只完成交叉编译；多屏、DPI、系统权限、Windows 进程树取消仍需相应系统实测。用户已确认此前 Linux 截图 OCR 与 0.10.0 全屏框选可用，本轮 Linux 门户主流程保持。
