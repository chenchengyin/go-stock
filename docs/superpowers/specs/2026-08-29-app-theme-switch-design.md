# App 主题切换（浅色 / 耀夜 / 灰色）设计

## 背景与目标

设置里已有浅色、深色、灰色三种变体，`AppColors` 也有对应调色板。实际页面大量写死 `Colors.white`、`CupertinoColors.white`、`CupertinoColors.systemGroupedBackground` 和品牌蓝 `0xff2364aa`。切换「深色」后，列表底会变深，顶栏、Tab、搜索框、底栏仍是浅色，截图即此状态。

灰色已经在 `main.dart` 用顶层 `ColorFiltered` 灰度矩阵实现，应保留。

**目标**：全 App 的壳、页面背景、正文色跟随主题；涨跌红绿与异动标签本轮不改。

## 已确认的产品决策

| 项 | 决策 |
| --- | --- |
| 范围 | 全 App 壳 + 页面背景 + 正文（盘达、超短情绪、快讯、我的、设置、详情、登录注册、热榜、策略吧页面） |
| 变体 | 浅色、耀夜、灰色，三选一 |
| 耀夜 | 即现有深色调色板，设置文案由「深色」改为「耀夜」 |
| 灰色 | 顶层灰度滤镜；调色板用浅色，不再维护一套独立灰板 |
| 涨跌 / 标签 / 图表描边白点 | 不改 |
| 实现方案 | 继续用 `AppColors` 语义色，清掉写死颜色；不迁移到 `colorScheme` 为主 |
| 跟随系统暗色 | 不做 |
| 持久化 | `SharedPreferences`，key `app_theme_variant`；非法/缺失回退浅色 |

## 主题模型

`AppThemeVariant` 仍为 `light` / `dark` / `grey`（枚举值不改，避免破坏已有引用）。

| 变体 | 调色板 | `ThemeData.brightness` | 顶层滤镜 |
| --- | --- | --- | --- |
| `light`（浅色） | `_light` | `Brightness.light` | 无 |
| `dark`（耀夜） | `_dark` | `Brightness.dark` | 无 |
| `grey`（灰色） | `_light` | `Brightness.light` | `ColorFiltered` 灰度矩阵（现有 `_grayScaleMatrix`） |

`applyGrey()` 改为调用浅色调色板，灰色观感只由滤镜产生。可删除或停用 `_grey` 调色板，避免两套灰色来源。

## 组件职责

### `AppColors`

语义 token 保持现有分类：`scaffoldBg`、`cardBg`、`backgroundColor`、`surfaceBg`、`textPrimary` / `textSecondary` / `textTertiary`、`appBarBg` / `appBarFg`、`inputFill`、`border` / `divider`、`brand`。

页面背景、卡片、顶栏、底栏、搜索填充、正文必须用这些字段，禁止再写死白/黑当背景或正文色。

`AppColorsWidget.updateShouldNotify` 不得只比较 `AppColors.instance` 引用（单例恒等，永远不通知）。应比较变体或代数，使 `AppColors.of(context)` 能跟随切换。

### `ThemeManager`

- `setVariant`：`AppColors.applyVariant` → 写 `app_theme_variant` → `notifyListeners`。
- 启动：从 prefs 读取；无法解析则 `light`。
- 写入失败只打日志，不阻断切换（内存主题仍生效）。
- `cycle()` 顺序保持：浅色 → 耀夜 → 灰色 → 浅色。

`ThemeManager` 创建改为异步恢复：`AppDependencies` 中 `create: (_) => ThemeManager()..restore()`（或等价 `Future`，保证首帧前尽量读完；若首帧来不及，允许先浅色再切到已存变体，避免启动卡死）。

### `AppTheme`

`AppTheme.current()` 用当前 `AppColors` 填 `ColorScheme` / `AppBarTheme` / `scaffoldBackgroundColor` / `cardTheme` / `inputDecorationTheme`。

- 耀夜：`brightness: Brightness.dark`，`onPrimary` 等对比色用浅色字。
- 浅色与灰色：`Brightness.light`。
- `MaterialApp.color` 不得写死 `Colors.white`，改用 `AppColors.scaffoldBg`。

### `AppShell`

底栏 `backgroundColor` / `selectedItemColor` / `unselectedItemColor` 改为 `AppColors.cardBg`（或 `surfaceBg`）、`brand`、`textTertiary`。不得写死 `Colors.white` / `Colors.blue` / `Colors.grey`。

## 页面改造规则

将下列用法替换为语义色（一对一约定）：

| 写死色 | 替换为 |
| --- | --- |
| `Colors.white` / `CupertinoColors.white` 作背景 | `cardBg` 或 `scaffoldBg`（卡片用前者，页面底用后者） |
| `CupertinoColors.systemGroupedBackground` | `scaffoldBg` 或 `backgroundColor` |
| `CupertinoColors.black` / 写死深色正文 | `textPrimary` |
| Tab/链接蓝 `0xff2364aa`、`0xff1967d2` | `brand` 或 `link` |
| 搜索框白底 | `inputFill` + `border` |

**允许保留的写死色：**

- 涨跌：`textPriceUp` / `textPriceDown`（已是 token）及盘口分析器里的红绿紫语义。
- 异动标签：`tagRed` 等及标签上的白字（对比色，不算页面正文）。
- 图表/情绪曲线上作为描边或中心点的白色。
- 主按钮上的白字（`onPrimary`）。

策略吧入口仍隐藏，但 `strategy_page.dart` / `post_detail_page.dart` / `create_post_page.dart` 同样替换，避免日后漏改。

涉及文件（实现时以仓库实际硬编码为准，至少覆盖）：

- `lib/main.dart`、`lib/app/app_shell.dart`、`lib/app/app_config.dart`
- `lib/core/theme/app_colors.dart`、`app_theme.dart`、`theme_manager.dart`
- 盘达：`radar_page.dart`、`search_results_panel.dart`、`monitor_settings_page.dart`、`voice_manager_page.dart`、`stock_change_detail_page.dart`
- 共享：`ios_widgets.dart`、`news_card.dart`、`stock_change_card.dart`（仅背景/标题，标签白字可留）
- 个人/设置/登录：`profile_page.dart`、`system_settings_page.dart`、`edit_profile_page.dart`、`login_page.dart`、`register_page.dart`
- 快讯/热榜/情绪：`news_page.dart`、`news_detail_page.dart`、`hot_topic_*`、`hotlist_page.dart`、`short_term_emotion_page.dart`（图表白点不改）
- 策略吧三页

## 数据流

```text
启动 → ThemeManager.restore(prefs)
     → applyVariant → MaterialApp.theme = AppTheme.current()
     → grey 则外包 ColorFiltered

设置页选择 → setVariant
           → applyVariant + prefs.setString
           → notifyListeners
           → Consumer 重建 MaterialApp
```

灰色与耀夜互斥：选灰色时滤镜开、调色板为浅色；选耀夜时滤镜关、调色板为 `_dark`。不把灰色滤镜叠在耀夜上。

## 错误处理

- prefs 读取失败或值为未知字符串 → `light`。
- prefs 写入失败 → 界面已切换，不弹错；下次启动可能回到旧值。
- 不引入主题相关的用户可见错误页。

## 测试

- `ThemeManager`：`setVariant` 后 `variant` 正确；restore 能读回；未知值回退 `light`。
- `AppTheme`：`dark` 时 `brightness == Brightness.dark`。
- 盘达 / `AppShell`：在 `dark` 下顶栏、Tab、搜索、底栏颜色取自 `AppColors`，断言不为 `Colors.white`（或等于对应 token）。
- 灰色：`variant == grey` 时 `TradingRadarApp` 仍包 `ColorFiltered`（可用 widget 测试查祖先或抽滤镜开关为可测函数）。

## 验收

1. 浅色：观感与现在浅色一致（允许底栏/顶栏从写死白改为 `cardBg`，浅色下仍是白/浅灰）。
2. 耀夜：盘达顶栏、Tab、搜索、列表、底栏、设置、个人、快讯、详情背景与正文均为深色体系，不再出现「壳白、内容黑」。
3. 灰色：整屏发灰（含涨跌红绿被滤成灰），与现在顶层滤镜一致。
4. 杀进程再开，主题仍是上次选择。
5. 涨跌颜色、异动标签色在浅色/耀夜下仍为红绿/现有标签色。

## 非目标

- 不为每个组件引入独立 ThemeExtension。
- 不重做视觉设计（新配色、新字体、新间距）。
- 不改后端、不改 Flutter web 构建产物（实现完成后再按现有流程决定是否 rebuild web）。
