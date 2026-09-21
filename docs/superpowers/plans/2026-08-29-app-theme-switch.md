# App 主题切换（浅色 / 耀夜 / 灰色）Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** 让全 App 壳、页面背景和正文跟随浅色 / 耀夜 / 灰色；耀夜即现有深色调色板；灰色仍是顶层灰度滤镜叠在浅色调色板上；主题写入本地。

**Architecture:** 继续用 `AppColors` 静态语义色 + `ThemeManager` 驱动 `MaterialApp.theme`。清掉页面里写死的白/黑/品牌蓝。`applyGrey()` 只套浅色板；`ColorFiltered` 仅在 `grey` 时包裹整树。不迁 `ThemeExtension`，不改涨跌/标签/图表描边白点。

**Tech Stack:** Flutter、Provider、`shared_preferences`、`flutter_test`

## Global Constraints

- 变体枚举保持 `light` / `dark` / `grey`；设置文案 `dark` →「耀夜」
- 灰色与耀夜互斥：灰色 = 浅色板 + 顶层滤镜，不叠在耀夜上
- 涨跌红绿、异动标签色、标签白字、主按钮白字、情绪图描边白点不改
- 不跟随系统暗色；不改后端；本轮不强制 rebuild Flutter web
- prefs key 必须是 `app_theme_variant`；非法/缺失 → `light`；写入失败只 `debugPrint`，不弹错
- 所有测试在 `trading_app/` 下跑：`flutter test test/core/theme/theme_manager_test.dart` 等

---

## File Structure

| 文件 | 职责 |
|---|---|
| `trading_app/lib/core/theme/app_colors.dart` | 调色板；`applyGrey` 走浅色；`AppColorsWidget` 比较 `variant` |
| `trading_app/lib/core/theme/app_theme.dart` | 按变体设 `Brightness` 并生成 `ThemeData` |
| `trading_app/lib/core/theme/theme_manager.dart` | 解析/持久化/切换；灰度滤镜判定 |
| `trading_app/lib/main.dart` | `MaterialApp.color` 用 token；滤镜用 `shouldApplyGreyFilter` |
| `trading_app/lib/app/app_config.dart` | `ThemeManager()..restore()` |
| `trading_app/lib/app/app_shell.dart` | 底栏语义色 |
| 各 feature / shared 页面 | 背景与正文改 token |
| `trading_app/test/core/theme/theme_manager_test.dart` | 解析、持久化、brightness、滤镜开关 |
| `trading_app/test/app/app_shell_theme_test.dart` | 耀夜底栏非白 |
| `trading_app/test/radar_theme_chrome_test.dart` | 耀夜顶栏/Tab 非白 |

不新增 ThemeExtension 文件。

---

### Task 1: 变体解析 + ThemeManager 持久化

**Files:**
- Modify: `trading_app/lib/core/theme/theme_manager.dart`
- Create: `trading_app/test/core/theme/theme_manager_test.dart`

**Interfaces:**
- Consumes: `AppThemeVariant`、`AppColors.applyVariant`、`SharedPreferences`
- Produces:
  - `static const String prefsKey = 'app_theme_variant'`
  - `static AppThemeVariant parseStored(String? raw)`
  - `static bool shouldApplyGreyFilter(AppThemeVariant variant)`
  - `Future<void> restore()`
  - `void setVariant(AppThemeVariant variant)` — 仍同步改内存，然后异步写 prefs

- [ ] **Step 1: 写失败单测**

创建 `trading_app/test/core/theme/theme_manager_test.dart`：

```dart
import 'package:flutter_test/flutter_test.dart';
import 'package:shared_preferences/shared_preferences.dart';
import 'package:trading_app/core/theme/app_colors.dart';
import 'package:trading_app/core/theme/theme_manager.dart';

void main() {
  TestWidgetsFlutterBinding.ensureInitialized();

  setUp(() {
    SharedPreferences.setMockInitialValues({});
    AppColors.applyLight();
  });

  group('parseStored', () {
    test('null and unknown fall back to light', () {
      expect(ThemeManager.parseStored(null), AppThemeVariant.light);
      expect(ThemeManager.parseStored(''), AppThemeVariant.light);
      expect(ThemeManager.parseStored('midnight'), AppThemeVariant.light);
    });

    test('parses dark and grey', () {
      expect(ThemeManager.parseStored('dark'), AppThemeVariant.dark);
      expect(ThemeManager.parseStored('grey'), AppThemeVariant.grey);
      expect(ThemeManager.parseStored('light'), AppThemeVariant.light);
    });
  });

  test('shouldApplyGreyFilter only for grey', () {
    expect(ThemeManager.shouldApplyGreyFilter(AppThemeVariant.grey), isTrue);
    expect(ThemeManager.shouldApplyGreyFilter(AppThemeVariant.dark), isFalse);
    expect(ThemeManager.shouldApplyGreyFilter(AppThemeVariant.light), isFalse);
  });

  test('setVariant persists and restore reads back', () async {
    final tm = ThemeManager();
    tm.setVariant(AppThemeVariant.dark);
    await Future<void>.delayed(Duration.zero);
    final prefs = await SharedPreferences.getInstance();
    expect(prefs.getString(ThemeManager.prefsKey), 'dark');

    final tm2 = ThemeManager();
    await tm2.restore();
    expect(tm2.variant, AppThemeVariant.dark);
  });

  test('restore unknown value stays light', () async {
    SharedPreferences.setMockInitialValues({ThemeManager.prefsKey: 'nope'});
    final tm = ThemeManager();
    await tm.restore();
    expect(tm.variant, AppThemeVariant.light);
  });
}
```

- [ ] **Step 2: 跑测试确认失败**

Run: `cd trading_app && flutter test test/core/theme/theme_manager_test.dart`

Expected: FAIL，`parseStored` / `prefsKey` / `restore` 未定义。

- [ ] **Step 3: 最小实现**

在 `theme_manager.dart` 增加 `shared_preferences` 与 `debugPrint` 导入，替换类体为：

```dart
class ThemeManager extends ChangeNotifier {
  static const String prefsKey = 'app_theme_variant';

  ThemeManager({AppThemeVariant variant = AppThemeVariant.light})
      : _variant = variant {
    AppColors.applyVariant(variant);
  }

  AppThemeVariant _variant;
  AppThemeVariant get variant => _variant;

  AppColors get colors => AppColors.instance;

  ThemeData get themeData => AppTheme.current();

  static AppThemeVariant parseStored(String? raw) {
    switch (raw) {
      case 'dark':
        return AppThemeVariant.dark;
      case 'grey':
        return AppThemeVariant.grey;
      default:
        return AppThemeVariant.light;
    }
  }

  static bool shouldApplyGreyFilter(AppThemeVariant variant) =>
      variant == AppThemeVariant.grey;

  Future<void> restore() async {
    try {
      final prefs = await SharedPreferences.getInstance();
      final next = parseStored(prefs.getString(prefsKey));
      if (next == _variant) {
        AppColors.applyVariant(next);
        return;
      }
      setVariant(next);
    } catch (e, st) {
      debugPrint('ThemeManager.restore failed: $e\n$st');
    }
  }

  void setVariant(AppThemeVariant variant) {
    if (_variant == variant) return;
    _variant = variant;
    AppColors.applyVariant(variant);
    notifyListeners();
    _persist(variant);
  }

  Future<void> _persist(AppThemeVariant variant) async {
    try {
      final prefs = await SharedPreferences.getInstance();
      await prefs.setString(prefsKey, variant.name);
    } catch (e, st) {
      debugPrint('ThemeManager persist failed: $e\n$st');
    }
  }

  void cycle() {
    switch (_variant) {
      case AppThemeVariant.light:
        setVariant(AppThemeVariant.dark);
      case AppThemeVariant.dark:
        setVariant(AppThemeVariant.grey);
      case AppThemeVariant.grey:
        setVariant(AppThemeVariant.light);
    }
  }
}
```

注意：`AppTheme.current(variant)` 在 Task 2 才改签名。本任务若编译失败，先临时保留 `AppTheme.current()` 无参，Task 2 再改；**优先同一会话做完 Task 1+2**，避免中间破编译。若必须拆开，Task 1 的 `themeData` 仍调用现有 `AppTheme.current()`，到 Task 2 再改成带 `variant`。

**本任务落点：** `themeData` 暂时继续 `AppTheme.current()`；测试不断言 brightness。

- [ ] **Step 4: 跑测试确认通过**

Run: `cd trading_app && flutter test test/core/theme/theme_manager_test.dart`

Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add trading_app/lib/core/theme/theme_manager.dart trading_app/test/core/theme/theme_manager_test.dart
git commit -m "feat(theme): persist AppThemeVariant and parse stored values"
```

---

### Task 2: AppTheme brightness、灰色走浅色板、InheritedWidget 通知

**Files:**
- Modify: `trading_app/lib/core/theme/app_theme.dart`
- Modify: `trading_app/lib/core/theme/app_colors.dart`
- Modify: `trading_app/lib/core/theme/theme_manager.dart`（`themeData => AppTheme.current(variant)`）
- Modify: `trading_app/test/core/theme/theme_manager_test.dart`（补 brightness 断言）

**Interfaces:**
- Consumes: `AppThemeVariant`、`AppColors` 静态字段
- Produces:
  - `AppTheme.current(AppThemeVariant variant)` → `ThemeData`
  - `AppColors.applyGrey()` → `_apply(_light)`
  - `AppColorsWidget({required AppThemeVariant variant, ...})`
  - `updateShouldNotify` → `old.variant != variant`

- [ ] **Step 1: 写失败测试（brightness + applyGrey）**

在 `theme_manager_test.dart` 追加：

```dart
  test('dark themeData uses Brightness.dark', () {
    final tm = ThemeManager();
    tm.setVariant(AppThemeVariant.dark);
    expect(tm.themeData.brightness, Brightness.dark);
    expect(tm.themeData.colorScheme.brightness, Brightness.dark);
  });

  test('grey themeData uses light palette and light brightness', () {
    final tm = ThemeManager();
    tm.setVariant(AppThemeVariant.grey);
    expect(tm.themeData.brightness, Brightness.light);
    expect(AppColors.cardBg, Colors.white);
    expect(AppColors.scaffoldBg, const Color(0xfff5f5f5));
  });
```

需要 `import 'package:flutter/material.dart';`。

- [ ] **Step 2: 跑测试确认失败**

Run: `cd trading_app && flutter test test/core/theme/theme_manager_test.dart`

Expected: FAIL，`current` 仍是 `Brightness.light`，或 `applyGrey` 仍把 `cardBg` 改成灰板。

- [ ] **Step 3: 实现**

`app_theme.dart`：

```dart
class AppTheme {
  static ThemeData current(AppThemeVariant variant) {
    final brightness = variant == AppThemeVariant.dark
        ? Brightness.dark
        : Brightness.light;
    final onPrimary = brightness == Brightness.dark
        ? const Color(0xffe0e0e0)
        : Colors.white;

    final colorScheme = ColorScheme(
      brightness: brightness,
      primary: AppColors.brand,
      onPrimary: onPrimary,
      secondary: AppColors.brandLight,
      onSecondary: AppColors.textPrimary,
      error: AppColors.error,
      onError: onPrimary,
      surface: AppColors.cardBg,
      onSurface: AppColors.textPrimary,
    );

    return ThemeData(
      useMaterial3: true,
      brightness: brightness,
      colorScheme: colorScheme,
      scaffoldBackgroundColor: AppColors.scaffoldBg,
      appBarTheme: AppBarTheme(
        centerTitle: false,
        elevation: 0.5,
        backgroundColor: AppColors.appBarBg,
        foregroundColor: AppColors.appBarFg,
      ),
      cardTheme: CardThemeData(
        color: AppColors.cardBg,
        elevation: 0,
        shape: RoundedRectangleBorder(borderRadius: BorderRadius.circular(8)),
      ),
      dividerTheme: DividerThemeData(
        color: AppColors.divider,
        thickness: 0.5,
      ),
      inputDecorationTheme: InputDecorationTheme(
        filled: true,
        fillColor: AppColors.inputFill,
        border: OutlineInputBorder(
          borderRadius: BorderRadius.circular(12),
          borderSide: BorderSide(color: AppColors.border),
        ),
      ),
    );
  }
}
```

`app_theme.dart` 需 `import` `AppThemeVariant`（已在 `app_colors.dart`）。

`applyGrey`：

```dart
static void applyGrey() => _apply(_light);
```

删除 `_grey` 调色板整块（从 `static final _Palette _grey =` 到该对象结束）。全仓库搜 `applyGrey` / `_grey`，不得再引用。

`AppColorsWidget`：

```dart
class AppColorsWidget extends InheritedWidget {
  const AppColorsWidget({
    super.key,
    required this.colors,
    required this.variant,
    required super.child,
  });

  final AppColors colors;
  final AppThemeVariant variant;

  @override
  bool updateShouldNotify(AppColorsWidget old) => old.variant != variant;
}
```

`theme_manager.dart`：`ThemeData get themeData => AppTheme.current(variant);`

`main.dart` 的 `AppColorsWidget` 增加 `variant: tm.variant`（可并入 Task 3；若本任务就要编译过，必须同时改 `main.dart`）。

- [ ] **Step 4: 跑测试确认通过**

Run: `cd trading_app && flutter test test/core/theme/theme_manager_test.dart`

Expected: PASS。再跑 `cd trading_app && flutter test` 确认无编译错误（`AppColorsWidget` 少参数会红）。

- [ ] **Step 5: Commit**

```bash
git add trading_app/lib/core/theme/app_theme.dart trading_app/lib/core/theme/app_colors.dart trading_app/lib/core/theme/theme_manager.dart trading_app/lib/main.dart trading_app/test/core/theme/theme_manager_test.dart
git commit -m "feat(theme): dark brightness, grey uses light palette"
```

---

### Task 3: 启动恢复、MaterialApp.color、灰度滤镜入口

**Files:**
- Modify: `trading_app/lib/app/app_config.dart`
- Modify: `trading_app/lib/main.dart`
- Modify: `trading_app/test/core/theme/theme_manager_test.dart`（可选：不测 widget 树）
- Create: `trading_app/test/main_grey_filter_test.dart`

**Interfaces:**
- Consumes: `ThemeManager.restore`、`ThemeManager.shouldApplyGreyFilter`
- Produces: 启动后读 prefs；`grey` 时根上有 `ColorFiltered`

- [ ] **Step 1: 写失败 widget 测试**

`trading_app/test/main_grey_filter_test.dart`：

```dart
import 'package:flutter/material.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:provider/provider.dart';
import 'package:shared_preferences/shared_preferences.dart';
import 'package:trading_app/core/theme/app_colors.dart';
import 'package:trading_app/core/theme/theme_manager.dart';
import 'package:trading_app/main.dart';

void main() {
  TestWidgetsFlutterBinding.ensureInitialized();

  testWidgets('grey variant wraps app in ColorFiltered', (tester) async {
    SharedPreferences.setMockInitialValues({});
    final tm = ThemeManager();
    tm.setVariant(AppThemeVariant.grey);

    await tester.pumpWidget(
      ChangeNotifierProvider.value(
        value: tm,
        child: Consumer<ThemeManager>(
          builder: (context, manager, _) {
            Widget app = const SizedBox(key: Key('inner'));
            if (ThemeManager.shouldApplyGreyFilter(manager.variant)) {
              app = ColorFiltered(
                colorFilter: const ColorFilter.matrix([
                  0.2126, 0.7152, 0.0722, 0, 0,
                  0.2126, 0.7152, 0.0722, 0, 0,
                  0.2126, 0.7152, 0.0722, 0, 0,
                  0, 0, 0, 1, 0,
                ]),
                child: app,
              );
            }
            return app;
          },
        ),
      ),
    );

    expect(find.byType(ColorFiltered), findsOneWidget);
  });
}
```

该测试直接锁 `shouldApplyGreyFilter` 行为。另外改 `main.dart` 里条件为 `ThemeManager.shouldApplyGreyFilter(tm.variant)`，矩阵仍用现有 `_grayScaleMatrix`。

- [ ] **Step 2: 跑测试**

Run: `cd trading_app && flutter test test/main_grey_filter_test.dart`

Expected: 若 Step 1 测试已用 helper，可能直接 PASS。接着改生产代码。

- [ ] **Step 3: 改生产代码**

`app_config.dart`：

```dart
ChangeNotifierProvider(create: (_) => ThemeManager()..restore()),
```

`main.dart`：

- `color: AppColors.scaffoldBg`（不要 `Colors.white`）
- `AppColorsWidget(colors: colors, variant: tm.variant, child: ...)`
- `if (ThemeManager.shouldApplyGreyFilter(tm.variant)) { app = ColorFiltered(...); }`

- [ ] **Step 4: 跑测试**

Run: `cd trading_app && flutter test test/core/theme/theme_manager_test.dart test/main_grey_filter_test.dart`

Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add trading_app/lib/main.dart trading_app/lib/app/app_config.dart trading_app/test/main_grey_filter_test.dart
git commit -m "feat(theme): restore variant on launch and gate grey filter"
```

---

### Task 4: AppShell 底栏跟主题

**Files:**
- Modify: `trading_app/lib/app/app_shell.dart`
- Create: `trading_app/test/app/app_shell_theme_test.dart`

**Interfaces:**
- Consumes: `AppColors.cardBg`、`AppColors.brand`、`AppColors.textTertiary`
- Produces: `BottomNavigationBar` 三色全部来自 token

- [ ] **Step 1: 写失败测试**

```dart
import 'package:flutter/material.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:provider/provider.dart';
import 'package:shared_preferences/shared_preferences.dart';
import 'package:trading_app/app/app_shell.dart';
import 'package:trading_app/core/theme/app_colors.dart';
import 'package:trading_app/core/theme/theme_manager.dart';
import 'package:trading_app/features/radar/data/radar_repository.dart';
import 'package:trading_app/features/radar/domain/voice_announcement_view_model.dart';
import 'package:trading_app/features/radar/presentation/radar_list/radar_view_model.dart';
import 'package:trading_app/features/radar/presentation/radar_list/t0_strategy_view_model.dart';

void main() {
  testWidgets('bottom bar is not white in dark variant', (tester) async {
    SharedPreferences.setMockInitialValues({'voice_announcement_asked': true});
    final tm = ThemeManager()..setVariant(AppThemeVariant.dark);
    final radarVm = RadarViewModel(RadarRepositoryImpl());
    final strategyVm = T0StrategyViewModel();
    final voiceVm = VoiceAnnouncementViewModel();

    await tester.pumpWidget(
      MultiProvider(
        providers: [
          ChangeNotifierProvider.value(value: tm),
          ChangeNotifierProvider.value(value: radarVm),
          ChangeNotifierProvider.value(value: strategyVm),
          ChangeNotifierProvider.value(value: voiceVm),
        ],
        child: MaterialApp(
          theme: tm.themeData,
          home: const AppShell(),
        ),
      ),
    );
    await tester.pump();

    final bar = tester.widget<BottomNavigationBar>(find.byType(BottomNavigationBar));
    expect(bar.backgroundColor, AppColors.cardBg);
    expect(bar.backgroundColor, isNot(Colors.white));
    expect(bar.selectedItemColor, AppColors.brand);
    expect(bar.unselectedItemColor, AppColors.textTertiary);

    radarVm.dispose();
    strategyVm.dispose();
    voiceVm.dispose();
  });
}
```

若 `AppShell` 还依赖 News/Emotion Provider，按 `app_config.dart` 补齐测试里缺的 Provider，直到 `pump` 不抛错。缺哪个就加哪个，不要改页面结构。

- [ ] **Step 2: 跑测试确认失败**

Run: `cd trading_app && flutter test test/app/app_shell_theme_test.dart`

Expected: FAIL，`backgroundColor` 仍是 `Colors.white`。

- [ ] **Step 3: 改 AppShell**

把底栏三行换成：

```dart
backgroundColor: AppColors.cardBg,
selectedItemColor: AppColors.brand,
unselectedItemColor: AppColors.textTertiary,
```

文件顶部增加：`import '../core/theme/app_colors.dart';`

- [ ] **Step 4: 跑测试确认通过**

Run: `cd trading_app && flutter test test/app/app_shell_theme_test.dart`

Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add trading_app/lib/app/app_shell.dart trading_app/test/app/app_shell_theme_test.dart
git commit -m "feat(theme): bottom navigation follows AppColors"
```

---

### Task 5: 盘达顶栏 / Tab / 搜索框

**Files:**
- Modify: `trading_app/lib/features/radar/presentation/radar_list/radar_page.dart`
- Create: `trading_app/test/radar_theme_chrome_test.dart`

**Interfaces:**
- Consumes: `AppColors.appBarBg`、`appBarFg`、`cardBg`、`brand`、`textTertiary`、`inputFill`
- Produces: 耀夜下 AppBar / Tab 容器不是 `Colors.white`

- [ ] **Step 1: 写失败测试**

```dart
import 'package:flutter/material.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:provider/provider.dart';
import 'package:shared_preferences/shared_preferences.dart';
import 'package:trading_app/core/theme/app_colors.dart';
import 'package:trading_app/core/theme/theme_manager.dart';
import 'package:trading_app/features/radar/data/radar_repository.dart';
import 'package:trading_app/features/radar/domain/voice_announcement_view_model.dart';
import 'package:trading_app/features/radar/presentation/radar_list/radar_page.dart';
import 'package:trading_app/features/radar/presentation/radar_list/radar_view_model.dart';
import 'package:trading_app/features/radar/presentation/radar_list/t0_strategy_view_model.dart';

void main() {
  testWidgets('radar app bar is not white in dark variant', (tester) async {
    SharedPreferences.setMockInitialValues({'voice_announcement_asked': true});
    final tm = ThemeManager()..setVariant(AppThemeVariant.dark);
    final radarVm = RadarViewModel(RadarRepositoryImpl());
    final strategyVm = T0StrategyViewModel();
    final voiceVm = VoiceAnnouncementViewModel();

    await tester.pumpWidget(
      MultiProvider(
        providers: [
          ChangeNotifierProvider.value(value: tm),
          ChangeNotifierProvider.value(value: radarVm),
          ChangeNotifierProvider.value(value: strategyVm),
          ChangeNotifierProvider.value(value: voiceVm),
        ],
        child: MaterialApp(theme: tm.themeData, home: const RadarPage()),
      ),
    );
    await tester.pump();

    final appBar = tester.widget<AppBar>(find.byType(AppBar));
    expect(appBar.backgroundColor, AppColors.appBarBg);
    expect(appBar.backgroundColor, isNot(Colors.white));

    radarVm.dispose();
    strategyVm.dispose();
    voiceVm.dispose();
  });
}
```

- [ ] **Step 2: 跑测试确认失败**

Run: `cd trading_app && flutter test test/radar_theme_chrome_test.dart`

Expected: FAIL，AppBar 仍 `Colors.white`。

- [ ] **Step 3: 替换 radar_page 写死色**

精确替换（保留按钮 `foregroundColor: Colors.white`、标签白字）：

```dart
// AppBar
backgroundColor: AppColors.appBarBg,
foregroundColor: AppColors.appBarFg,

// Tab 外层 Container
color: AppColors.appBarBg,

// TabBar
labelColor: AppColors.brand,
unselectedLabelColor: AppColors.textTertiary,
indicatorColor: AppColors.brand,

// 搜索框 fillColor: Colors.grey[100] →
fillColor: AppColors.inputFill,
```

文件中其余 `Color(0xff2364aa)`（约 1046、1053 行附近）改为 `AppColors.brand`。

- [ ] **Step 4: 跑测试确认通过**

Run: `cd trading_app && flutter test test/radar_theme_chrome_test.dart test/radar_page_test.dart`

Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add trading_app/lib/features/radar/presentation/radar_list/radar_page.dart trading_app/test/radar_theme_chrome_test.dart
git commit -m "feat(theme): radar chrome uses AppColors tokens"
```

---

### Task 6: 其余页面背景 / 正文 token 化

**Files:**
- Modify: `trading_app/lib/features/radar/presentation/radar_list/search_results_panel.dart`
- Modify: `trading_app/lib/features/radar/presentation/radar_list/monitor_settings_page.dart`
- Modify: `trading_app/lib/features/radar/presentation/radar_list/voice_manager_page.dart`
- Modify: `trading_app/lib/features/radar/presentation/stock_change_detail/stock_change_detail_page.dart`
- Modify: `trading_app/lib/features/profile/presentation/system_settings_page.dart`
- Modify: `trading_app/lib/features/profile/presentation/profile_page.dart`
- Modify: `trading_app/lib/features/profile/presentation/edit_profile_page.dart`
- Modify: `trading_app/lib/features/auth/presentation/login_page.dart`
- Modify: `trading_app/lib/features/auth/presentation/register_page.dart`
- Modify: `trading_app/lib/features/news/presentation/market_news/news_page.dart`
- Modify: `trading_app/lib/features/news/presentation/news_detail/news_detail_page.dart`
- Modify: `trading_app/lib/features/news/presentation/hot_topic/hot_topic_detail_page.dart`
- Modify: `trading_app/lib/features/news/presentation/hot_topic/hot_topic_card.dart`
- Modify: `trading_app/lib/features/hotlist/presentation/hotlist_list/hotlist_page.dart`
- Modify: `trading_app/lib/shared/widgets/news_card.dart`
- Modify: `trading_app/lib/shared/widgets/ios_widgets.dart`
- Modify: `trading_app/lib/features/strategy/presentation/strategy_page.dart`
- Modify: `trading_app/lib/features/strategy/presentation/post_detail_page.dart`
- Modify: `trading_app/lib/features/strategy/presentation/create_post_page.dart`

**Interfaces:**
- Consumes: 下表 token 映射
- Produces: 这些文件中不再用白/系统分组底/黑正文/`0xff2364aa` 当壳或正文（允许列表见下）

**映射（所有文件同一套）：**

| 旧值 | 新值 |
|---|---|
| `Colors.white` / `CupertinoColors.white` 作 Scaffold/Container 背景 | 页面底 `AppColors.scaffoldBg`，卡片/分组 `AppColors.cardBg` |
| `CupertinoColors.systemGroupedBackground` | `AppColors.scaffoldBg` |
| `CupertinoColors.black` 作标题 | `AppColors.textPrimary` |
| `Color(0xff2364aa)` / `Color(0xff1967d2)` | `AppColors.brand`（链接用 `AppColors.link`） |
| `Color(0xff5f6368)` 作次要字 | `AppColors.textSecondary` |
| `CupertinoColors.systemGrey5` 底边 | `AppColors.divider` |
| `CupertinoColors.systemGrey` / `systemGrey2` / `systemGrey6` 作次要 UI | `AppColors.textTertiary` / `AppColors.surfaceBg`（灰字用 tertiary，浅底用 surfaceBg） |

**禁止改：**

- `stock_change_card.dart` 标签 `Colors.white` 字
- `short_term_emotion_trend_chart.dart` 白描边
- 主按钮 / 标签上的白字
- `news_card.dart` 里涨跌红绿 `0xffe53935` / `0xff0d904f`
- `strategy_page` 里按钮上的 `CupertinoColors.white`（onPrimary）

**设置文案：**

```dart
case AppThemeVariant.dark:
  return '耀夜';
```

ActionSheet 里「深色」改为「耀夜」。

每个改动的文件若尚未 import `app_colors.dart`，补：

```dart
import '.../core/theme/app_colors.dart';
```

（按文件相对路径。）

- [ ] **Step 1: 先 grep 基线**

Run:

```bash
cd trading_app && rg -n "CupertinoColors.white|CupertinoColors.systemGroupedBackground|CupertinoColors.black|Colors.white|0xff2364aa|0xff1967d2" lib --glob '!**/pankou_analyzer.dart' --glob '!**/short_term_emotion_trend_chart.dart'
```

记下列表，作为本任务要清的范围。

- [ ] **Step 2: 按映射改文件（无单独失败测试；用 grep 验收）**

逐文件替换，不要「顺便重构」布局。`monitor_settings_page` / `voice_manager_page` / `stock_change_detail_page` 的 `backgroundColor: Colors.white` 改为 `AppColors.appBarBg` 或 `scaffoldBg`（AppBar 用 appBarBg，页面用 scaffoldBg）。详情页主按钮白字保留。

`news_card.dart`：

```dart
Color get _titleColor =>
    item.isRed ? const Color(0xffb71c1c) : AppColors.textPrimary;

color: AppColors.cardBg,
bottom: BorderSide(color: AppColors.divider, width: 0.5),
```

- [ ] **Step 3: 再 grep，只允许白名单命中**

允许残留：

- `Colors.white` 在 `onPrimary`、标签字、`app_theme.dart` 浅色 `onPrimary`、`app_colors.dart` 浅色 `cardBg: Colors.white`
- 图表文件
- `stock_change_card` 标签白字
- `pankou_analyzer` 语义色

Run: 同 Step 1 的 `rg`

Expected: 页面 Scaffold/Container 不再出现 `CupertinoColors.white` / `systemGroupedBackground`。

- [ ] **Step 4: 跑回归测试**

Run: `cd trading_app && flutter test`

Expected: PASS（`widget_test.dart` 若本来就找错 `NavigationBar`，本任务不修，除非你改 `AppShell` 时弄崩了 pump。）

- [ ] **Step 5: Commit**

```bash
git add trading_app/lib/features trading_app/lib/shared
git commit -m "feat(theme): replace hardcoded chrome colors with AppColors"
```

---

### Task 7: 全量核对 + 手工验收清单

**Files:** 无新文件（若 grep 仍有漏网，改回 Task 6 同类替换）

**Interfaces:** 无

- [ ] **Step 1: 仓库级 grep**

```bash
cd trading_app && rg -n "CupertinoColors.white|systemGroupedBackground" lib
```

Expected: 无页面背景命中；若有，立刻按 Task 6 映射改掉并补进同一 commit 或新 commit `fix(theme): remaining cupertino chrome colors`。

- [ ] **Step 2: 跑全量 Flutter 测试**

Run: `cd trading_app && flutter test`

Expected: PASS

- [ ] **Step 3: 手工验收（实现者在模拟器/真机）**

1. 设置 → 浅色：顶栏底栏接近白/浅灰，涨跌仍红绿。
2. 设置 → 耀夜：盘达顶栏、Tab、搜索、列表、底栏、我的、设置、快讯、详情均为深色底+浅字，无「壳白内容黑」。
3. 设置 → 灰色：整屏发灰。
4. 杀进程再开：仍是上次主题。
5. 涨跌与异动标签在浅色/耀夜下颜色不变。

- [ ] **Step 4: Commit（仅当 Step 1 还有代码改动）**

无改动则不空提交。

---

## Spec coverage

| Spec 项 | 任务 |
|---|---|
| 浅色 / 耀夜 / 灰色三选一，枚举不改名 | 1 |
| 耀夜 = 深色板，文案「耀夜」 | 2、6 |
| 灰色 = 浅色板 + ColorFiltered | 2、3 |
| prefs `app_theme_variant`，非法回退 light | 1、3 |
| ThemeData brightness | 2 |
| AppColorsWidget 按 variant 通知 | 2 |
| MaterialApp.color 不写死白 | 3 |
| AppShell 底栏 | 4 |
| 盘达壳 | 5 |
| 全 App 页面背景/正文 | 6 |
| 涨跌/标签/图白点不改 | 5–6 白名单 |
| 测试 ThemeManager / AppTheme / 盘达壳 / 滤镜 | 1–5 |
| 不迁 colorScheme 为主、不跟系统、不改后端 | 全局约束 |

无未覆盖 spec 项。
