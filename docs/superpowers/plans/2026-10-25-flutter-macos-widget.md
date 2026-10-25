# Flutter Mac 与空白系统组件实施计划

> **For agentic workers:** Use superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [x]`) syntax for tracking.

**Goal:** 在现有 Flutter 项目中跑通 Mac 主应用，并随同一个 App 安装一个可添加的空白系统小组件。

**Architecture:** 保留 Flutter 主应用，在现有 macOS Xcode 工程增加 WidgetKit/SwiftUI extension target；本轮仅占位，不增加数据桥接。

**Tech Stack:** Flutter 3.44.4 / Dart 3.12.2、Xcode 15.4、SwiftUI、WidgetKit、CocoaPods。

**Spec:** `docs/superpowers/specs/2026-10-25-flutter-macos-widget-design.md`

## Global Constraints

- 在现有 `trading_app` 项目实施，保留其他未提交改动。
- 顺序：先 Mac 主应用，再空白系统小组件；不填充行情或虚构数据。
- 不改变 Android/Web 行为，不启动或重启生产后端，不访问账号凭据。
- 本轮直接在用户指定的当前 checkout 实施；只修改 Mac 支持及必要构建兼容项。
- 不升级 Flutter、Xcode 或全部插件来规避局部问题；优先使用当前工具链。

## Review Focus

- 正式构建必须包含网络客户端权限，不能只有 Debug 可联网。
- 空白组件必须嵌入主应用，不能只是单独编译成功。
- 扩展包标识、最低系统版本、签名及版本号须与主应用兼容。
- App 启动不能被仅支持手机的后台插件阻断。
- 用户添加的空白组件不得显示随机样例行情或产生网络/通知任务。

### Task 1: Flutter Mac 构建与启动

**Files:** `trading_app/macos/Runner/*.entitlements`，必要时 `trading_app/macos/Podfile` 及原生构建配置；按实际失败定位其他最小兼容改动。

**Interfaces:** 消费现有 Flutter/Dart 入口和 Go API；产出能启动到现有登录或主界面的 Mac `.app`。

- [x] 重现原始 Mac 构建失败并记录原因。
- [x] 安装缺失构建依赖，补齐 Debug/Release 出站网络权限。
- [x] 编译 Mac、实际启动并检查窗口与 API 连通。
- [x] 针对实际修改运行适当检查；记录既有测试/分析问题，不做无关重构。

### Task 2: 空白 WidgetKit 扩展

**Files:** 新增 `trading_app/macos/StockWidgetExtension/` 原生 Swift、Info.plist、entitlements；修改 `Runner.xcodeproj/project.pbxproj` 及必要 scheme/说明。

**Interfaces:** 消费 Task 1 的 Mac 主应用构建；产出嵌入 `Contents/PlugIns/*.appex` 的静态空白系统小组件。

- [x] 使用小、中尺寸的静态 SwiftUI 占位视图，不接数据。
- [x] 配置扩展 target、主应用依赖和 Embed App Extensions 阶段。
- [x] 编译并检查扩展 plist、签名、嵌入路径。
- [x] 安装本地测试 App并确认系统注册。
- [x] 系统组件库中找到并添加空白小组件（用户确认“能找到并添加”）。
- [x] 回归主应用启动与相关测试，进行一次完整变更审阅。
- [x] 将实际命令、结果、产物和未验证项写入实施记录与对话日志。

## 验证取舍

本轮主要是开发环境、构建配置和静态原生占位视图；以实际 Xcode/Flutter 构建和系统展示为验收，不编写重复 plist/工程文本的单元测试。若必须修改 Dart 行为，先为相应运行行为补充失败测试再修复。
