# PP-OCRv6 tiny OCR 插件

版本 1.0.0；插件 ID `lumin.ppocrv6-tiny`，能力 `recognize`，使用现有 Lumin 协议 v1。包含检测/识别 ONNX 模型、字典及原生 CPU 推理库，运行时离线，不依赖 Python、Tesseract 或额外模型下载。

## 安装

主窗口「服务设置 → 插件 → 安装插件包」选择对应平台 `.luminext`，点击该插件「用于 OCR」，再在「OCR 服务」保存设置。随后托盘图片识字、截图识字和截图翻译都会使用它。适用于已有 Lumin 0.7.0，无需升级宿主协议。

`threads` 默认 4（1–16）；`max_side` 默认 960（320–2048），限制检测图片最长边。提高最长边可改善小字检测但增加耗时和内存。模型使用自身字典自动识别支持字符，不读取 Tesseract 的 eng+chi_sim 语言选项。支持中英文等 tiny 字典覆盖的字符，不承诺日文/韩文等完整语言覆盖。

## 架构

Go 插件入口 → 解码图片并转 RGB → 固定 RapidOCRLite 5.0.0a16 C ABI → PP-OCRv6 tiny 检测 → DB 后处理与透视裁剪 → tiny 识别与 CTC 字典解码 → 返回按引擎顺序连接的文本。图片透明部分合成白底，置信度低于 0.5 的结果不返回。图片文件最多 20 MiB，解码沿用宿主 6400 万像素上限；结果最多 1 MiB。

原生库由插件私有目录加载；Windows 限制 DLL 搜索目录。CPU 模式不需要 GPU 驱动。宿主取消或超时会终止整个插件进程，释放原生模型和线程；当前每次识别创建新进程，未使用常驻模型缓存。没有额外的 180° 方向分类模型，倒置图片需要先旋转。

本版本使用专用 ONNX 解释器 RapidOCRLite，**不是 ONNX Runtime**，不支持任意 ONNX 架构。上游运行库是预发布版本，已锁定下载哈希，升级必须重新跑真实样例。Linux 包仍依赖标准 glibc/libstdc++/libgcc 系统运行库；Windows x64 SDK 仅依赖 KERNEL32.dll。macOS/ARM 构建尚未发布。

## 构建与维护

```bash
python3 scripts/build-ppocr-plugin.py --target linux/amd64
python3 scripts/build-ppocr-plugin.py --target windows/amd64
```

构建机需要 Go、Python 3 标准库和 curl；首次构建下载约定资产，SHA-256 不符立即失败。缓存默认 `build/ppocr-cache`，可用 `--cache` 指定已有离线缓存。Python 不进入运行包。脚本只提取 SDK 中指定文件，从已校验 ONNX 的 metadata_props.character 提取对应字典，避免字典与模型不匹配。

更新模型时同时更新 assets.lock.json 和插件版本；不可只替换 recognition.onnx 而保留旧字典。更换运行库要核对 ABI：options 的前 128 字节、image/result/text_box 的大小和字段偏移，必须保持 C 指针/结果生命周期。错误信息依赖线程局部状态，调用期间固定 OS 线程。

真实集成测试：

```bash
LUMIN_PPOCR_PACKAGE="$PWD/build/plugins/lumin.ppocrv6-tiny-1.0.0-linux-amd64.luminext" go test ./plugins/ppocrv6-tiny -v
```

测试在临时目录安装真实插件，清空 PATH 后通过宿主 JSON 协议识别英文和中英文混排样例，并检查无字图片、无效参数及取消。普通单元测试不下载或加载模型。Windows 目前完成交叉编译与包校验，仍需 Windows 实机识别验收。
