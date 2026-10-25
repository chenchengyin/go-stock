# Flutter Mac 与系统小组件

Mac 主程序使用本项目的 Flutter 界面；`StockWidgetExtension` 是同一个 Xcode 工程中的 SwiftUI/WidgetKit 扩展，构建时嵌入 App 的 `Contents/PlugIns/`。

当前小组件为静态空白卡片，支持小、中、大尺寸。不请求行情、不发送通知，不需要 App Group；后续接入数据时再加入共享存储和刷新逻辑。

## 右侧可操作面板

Mac 主 App 在顶部菜单栏增加“盘”字快捷按钮（提示“交易雷达”）。启动后先收起主窗口，点击图标展开右侧完整 Flutter App，再点收起；右键图标可显示或退出。窗口红色关闭按钮只收起，Dock 图标也能重新打开。收起不退出进程，不重新创建页面或引擎。

完整 Flutter 页面仍可点击、滚动、输入和导航。首次默认宽 400 点，贴当前屏幕可用区域的右侧，占可用高度，并显示在普通窗口上方。

- 标题栏“普通窗口／右侧面板”按钮切换模式。
- App 内快捷键为 `Control + Command + P`；也可在 `Window／窗口 → 右侧面板模式` 切换。
- `Window／窗口 → 面板宽度` 可选择 360、400、480、580 点；也可拖动窗口边缘调宽，范围为 320–640 点（窄屏以内实际可用宽度为限）。
- 切回普通窗口会恢复进入面板前的位置、尺寸及窗口层级；模式和面板宽度会记住。切换复用同一个 FlutterViewController，不重新登录或创建第二个 Flutter 引擎。

这是 App 自己的菜单栏入口与窗口模式，与系统 WidgetKit 小组件分开。当前不包含点击外部自动隐藏、边缘滑出、全局快捷键或复杂全屏 Spaces 行为。

原生窗口逻辑测试不启动 Flutter 测试宿主，直接将同一生产控制器源码编译到 `RunnerTests`，避免依赖 Flutter 插件运行时：

```bash
cd trading_app
xcodebuild -workspace macos/Runner.xcworkspace -scheme Runner \
  -configuration Debug -destination 'platform=macOS,arch=arm64' test
```

## 构建

需要 Mac、Xcode、Flutter 和 CocoaPods。部分现有插件尚未支持 Swift Package Manager，因此仍需 CocoaPods。请保留 `Podfile.lock`。

在仓库根目录运行：

```bash
scripts/build-flutter-macos.sh --release
```

脚本优先使用 PATH 中的 CocoaPods，也能使用本机已安装的、被 Git 忽略的 `.tools/bin/pod`。该本地工具目录不会随源码提交，其他 Mac 需自行安装 CocoaPods。

默认使用 Flutter 现有后端配置。本机开发可显式指定已经运行的 Go 服务：

```bash
scripts/build-flutter-macos.sh --debug \
  --dart-define=API_BASE_URL=http://127.0.0.1:8080
```

对应产物是 `trading_app/build/macos/Build/Products/Release/trading_app.app` 或 `Debug/trading_app.app`。Mac App 不包含 Go 后端进程，指定本地地址时需要本地服务持续运行。

## 安装并添加组件

1. 将构建的整个 App 放入“应用程序”目录，打开一次主程序。
2. 在桌面右键选择“编辑小组件”，或在通知中心选择“编辑小组件”。
3. 搜索“交易雷达”，选择尺寸并添加。组件内部留空是此阶段的预期行为。

本地构建当前使用 ad hoc 签名，只用于本机开发验收。正式分发需要在 Xcode 配置开发者签名和相应发布流程。不要通过关闭系统安全保护来解决组件发现问题。

## 验证

```bash
codesign --verify --deep --strict \
  trading_app/build/macos/Build/Products/Release/trading_app.app
pluginkit -m -A -D -v -i com.colin.tradingApp.StockWidget
```

签名及注册检查通过不等于已经验证系统组件库展示；实际添加与空白卡片显示仍需单独检查。此次详细验收记录在仓库的 `docs/superpowers/` 中。
