# 彩云小译 · Lumin 插件

将 [TechDecryptor 的 Pot 彩云插件](https://github.com/TechDecryptor/pot-app-translate-plugin-caiyun) 移植为 Go + Lumin JSON-stdio 插件。上游参考提交固定为 `f4b77ce3a5db5c1ac269d742dc00dcfc822575cd`（2024-06-23），保留 GPL-3.0 许可和出处，见 LICENSE / NOTICE。

## 安装

1. 在 Lumin「插件」页面安装 `lumin.caiyun-1.0.0-<os>-<arch>.luminext`。
2. 点击「添加翻译服务」。默认两项设置均可留空，使用上游公开的客户端参数。
3. 勾选启用，点击「保存服务」，回到翻译页面输入文本。

插件 ID 为 `lumin.caiyun`，版本为 `1.0.0`。它是新的 `.luminext` 插件，不需要加载 Pot/Tauri 或 JavaScript。无需更换 Lumin 0.2.0 主程序。

## 行为与兼容范围

- 请求地址：`https://interpreter.cyapi.cn/v1/translator`。
- 保留上游的 JSON 请求字段、客户端 User-Agent、默认 Token、默认设备 ID 和毫秒时间戳 request_id 算法。
- 支持自动检测源语言，以及中、英、日、韩、法、西、俄语。德、葡、意等其他语言明确提示不支持。
- `zh` / `zh-TW` 和 Pot 的 `zh_cn` / `zh_tw` 都映射为 `zh`，与原插件一致；选择繁体中文不代表服务保证输出繁体。
- 上游字符串译文原样返回；同时兼容字符串数组响应，以换行连接。
- 默认无需配置；可选 `token` 为不带 `token ` 前缀的值，`device_id` 为设备标识。它们只覆盖同一个客户端接口的请求参数，不会切换到彩云开放平台 API。
- HTTP 超时 45 秒，受宿主整体超时和取消约束。拒绝重定向，响应上限 2 MiB，不自动重试。
- 授权失败、限流、网络异常、空译文与错误 JSON 均给出明确错误，不把上游原始响应或配置密钥写入错误消息。

这是对上游客户端接口的复现，不是彩云官方 SDK。接口、默认参数或额度可能变更；HTTP 401/403 时需要检查接口状态或更新可用参数，不能保证长期免配置可用。文本会发送到彩云服务。自定义 Token 以本地明文配置保存，输入框隐藏显示。

## 构建与维护

在 Lumin 仓库根目录：

```bash
./scripts/build-plugins.sh caiyun
# 也可 make plugins 一起构建全部插件
GOOS=windows GOARCH=amd64 ./scripts/build-plugins.sh caiyun
GOOS=darwin GOARCH=arm64 ./scripts/build-plugins.sh caiyun
```

产物在 `build/plugins/`。构建脚本把 LICENSE、NOTICE、本说明及完整构建源码一起放入插件包。修改实现后提升 plugin.json 的版本；安装器不覆盖已安装的同版本。

源码职责：

- `main.go`：JSON-stdio 协议入口。
- `caiyun.go`：语言映射、请求构建、HTTP 与响应校验。
- `caiyun_test.go`：模拟传输下的请求格式、语言、参数覆盖、错误、取消、重定向和协议测试。
- `plugin.json`：平台入口及可选参数声明。

```bash
go test ./plugins/caiyun
go test -race ./plugins/caiyun
go vet ./plugins/caiyun
```

离线测试不调用彩云接口，也不消耗配额。真实接口调用应使用非敏感测试文本，并区分“编译/模拟测试成功”和“线上服务成功”。

## 从插件包源码重新构建

把 `.luminext` 作为 ZIP 解压。包内 `src/` 为独立 Go 模块，仅依赖标准库，包含插件和协议 SDK 的对应版本；使用 src/go.mod 所列 Go 版本或更高版本。

```bash
cd src
go test ./...
CGO_ENABLED=0 go build -buildvcs=false -trimpath -ldflags='-s -w' -o ../bin/plugin ./plugins/caiyun
```

Windows 使用 `GOOS=windows GOARCH=amd64` 并把输出名改为 `../bin/plugin.exe`。修改平台时同步更新 plugin.json 的 entrypoints；分发修改版时提升版本、标注修改并保留 GPL 许可和对应源码。

## 验证记录（2026-11-23）

- 全项目测试与静态检查、插件/运行器竞态检查通过。包内独立源码模块的测试与重建通过。
- 离线测试验证请求与上游一致，覆盖语言映射、参数覆盖、字符串/数组译文、响应错误、取消和重定向拒绝。
- 使用获准联网的 curl，按相同客户端参数发送固定非敏感文本 `Hello world`，接口返回 HTTP 200，译文为 `你好，世界`。这只证明测试时该接口与默认参数可用。
- Linux 包可由真实插件管理器安装、校验，并通过真实子进程回传结构化的语言错误；该过程不联网。
- 当前沙箱 Go 进程的外网 DNS 返回 `Temporary failure in name resolution`，所以通过宿主执行插件的线上翻译尚未在本环境完成。目标桌面仍需验证正常联网翻译。
- Windows amd64 / macOS arm64 提供交叉构建包；未做相应系统真机执行或签名。

复现安装与子进程测试：

```bash
LUMIN_TEST_CAIYUN_PACKAGE="$PWD/build/plugins/lumin.caiyun-1.0.0-linux-amd64.luminext" \
  go test ./internal/plugin -run TestBuiltCaiyunPackage -count=1 -v
```

在可联网环境显式开启端到端测试（会发送固定文本到彩云）：

```bash
LUMIN_TEST_CAIYUN_PLUGIN="$PWD/build/plugins/lumin.caiyun-1.0.0-linux-amd64.luminext" \
  go test ./internal/plugin -run TestBuiltCaiyunPlugin -count=1 -v
```
