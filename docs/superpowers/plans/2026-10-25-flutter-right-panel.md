# Flutter Mac 右侧面板实施计划

> **For agentic workers:** Use superpowers:executing-plans to implement task-by-task.

**Goal:** 让现有完整 Flutter App 在屏幕右侧可操作，并能恢复普通窗口。

**Architecture:** 一个 Flutter 主窗口、一个原生窗口呈现控制器；只调整窗口几何、层级与本地偏好，不另建 Flutter 引擎。

**Tech Stack:** Swift/AppKit、Flutter Mac、Xcode 15.4。

**Spec:** `docs/superpowers/specs/2026-10-25-flutter-right-panel-design.md`

## Global Constraints

- 直接在现有 checkout 接续工作，保留上一阶段和其他未提交改动。
- 不改 Dart 业务、Go 服务、已有 WidgetKit 扩展或系统权限。
- 默认面板宽 400 点，允许 320–640 点（屏幕更窄时以可用屏幕为上限）；尊重屏幕非零/负坐标及 Dock/菜单占位。
- 普通窗口与面板使用同一 FlutterViewController，切换不重建业务状态。

## Review Focus

- 切回普通模式必须恢复原 frame、层级和可移动性。
- 重复进入面板不能覆盖原普通窗口状态。
- 面板不允许跑出当前可用屏幕；调整宽度后重新贴右。
- 所有 AppKit 操作在主线程。
- 本地模式与宽度偏好不能影响其他平台或登录设置。

### Task 1: 可测试的原生呈现控制器

**Files:** 新增 `Runner/RightPanelController.swift`；替换 `RunnerTests/RunnerTests.swift` 默认空测试；将新源加入现有 Xcode target。

**Interfaces:** `RightPanelController(window:defaults:visibleFrameProvider:)`、`setPanelMode(_:)`、`setPanelWidth(_:)`、只读 `isPanelMode`；控制真实 NSWindow，保留其 contentViewController。

- [x] 先写右侧定位/窄屏/模式恢复/保留内容/偏好恢复的原生测试，确认缺失实现而失败。
- [x] 实现控制器，运行整个 RunnerTests 套件通过。

### Task 2: 主窗口接入和实际运行

**Files:** `Runner/MainFlutterWindow.swift`、`Runner/AppDelegate.swift`、`Runner.xcodeproj/project.pbxproj`、Mac README、验收记录。

- [x] 主窗口保留原 Flutter 初始化，应用启动时恢复用户窗口模式，首启默认右侧。
- [x] 添加标题栏切换按钮及“窗口”菜单的模式/宽度入口；只提供 App 内快捷键。
- [x] 编译 Debug/Release、校验签名与已有扩展，做一次独立变更审阅。
- [x] 更新本轮安装的 App 并发起启动，记录沙盒系统确认阻塞。
- [x] 用户解锁后，修复启动回调的无效 super 调用；实际验证面板显示、普通窗口/快捷键切换、策略页切换及股票搜索输入。
- [x] 记录构建结果、原生测试与既有 Flutter 测试失败，不进行无关雷达重构。
