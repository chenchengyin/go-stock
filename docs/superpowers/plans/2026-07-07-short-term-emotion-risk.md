# Flutter App 超短情绪短线避坑分阶段实现计划

> **给后续执行 Agent 的要求：** 实现本计划时必须使用 `superpowers:subagent-driven-development`（推荐）或 `superpowers:executing-plans`，并按阶段逐项执行。每个阶段完成、验证通过、向用户汇报后必须暂停，等待用户明确回复“OK/继续/下一步”再进入下一个阶段。

**目标：** 在 Flutter App 主页底部导航中新增 `超短情绪` Tab，位置在 `盘达` 后面、`市场快讯` 前面，用于展示 `超短情绪:短线避坑` 仪表盘。

**架构：** 后端能力尽量放在 `backend/flutter_api` 包内，新增 REST 接口 `/api/short-term-emotion`，避免把可变业务逻辑散到主后端目录里。Flutter 端新增独立 feature：domain model、repository、view model、page。最后再接入首页底部 Tab。

**技术栈：** Go `backend/flutter_api` REST 服务、现有 `backend/data` 数据源、Flutter、Provider、Dio、现有 `AppColors` 主题系统。

## 全局约束

- 新增的是 Flutter App 首页底部 Tab，不是 Wails/Vue 页面。
- 不新增或修改 Wails 前端页面，不改 `frontend/src`。
- 底部 Tab 最终顺序：`盘达`、`超短情绪`、`市场快讯`、`我的`。
- 暂时隐藏 `策略吧` Tab：只注释入口和页面映射，不删除策略吧代码、接口、Provider、数据模型。
- 后端新增业务代码尽量放在 `backend/flutter_api` 文件夹下；可复用 `backend/data` 现有数据读取能力，但不要新增核心业务文件到 `backend/data`。
- 不做单独的 `交易前检查清单`；避坑内容统一显示为 `风险信号`。
- 评分必须确定性、可解释、可测试；v1 不使用 AI 计算分数。
- 页面风格必须融入现有 App：白底、轻卡片、细分割线、蓝色品牌色、红涨绿跌、紧凑信息密度。

---

## 阶段执行规则

每个阶段都是一个单独可编码、可验证、可回滚的小交付：

1. 只做当前阶段列出的文件。
2. 当前阶段验证通过后，汇报改动、测试结果、风险。
3. 等用户确认后再进入下一阶段。
4. 如果当前阶段发现范围不够，先停下来问用户，不顺手扩大范围。

推荐阶段顺序：

1. 后端纯评分器
2. 后端实时聚合和 REST 接口
3. Flutter 数据层和 ViewModel
4. Flutter 页面，但先不接入底部 Tab
5. 首页底部 Tab 接入并隐藏策略吧
6. 端到端验证和说明文档

---

## 当前可复用代码

- `backend/flutter_api/server.go`
  - Flutter REST 服务注册入口，目前已有 `/api/news`、`/api/stock-changes`、`/api/strategy` 等。
- `backend/data/market_statistic_api.go`
  - 已有盘中市场宽度快照：上涨家数、下跌家数、红盘率、涨停、跌停、涨跌停比。
- `backend/data/stock_changes_api.go`
  - 已有盘中异动：火箭发射、大笔买入、封涨停板、高台跳水、打开涨停板等。
- `backend/data/market_news_api.go`
  - `GetUplimitHot(date, limit)` 已有涨停梯队、热门板块、最高连板等数据。
- `trading_app/lib/app/app_shell.dart`
  - 当前底部导航：`盘达`、`市场快讯`、`策略吧`、`我的`。
- `trading_app/lib/app/app_config.dart`
  - Provider 注册入口。
- `trading_app/lib/core/network/api_client.dart`
  - Flutter 端统一 Dio client。
- `trading_app/lib/core/theme/app_colors.dart`
  - App 主题色，页面必须复用。

---

## 目标接口

```http
GET /api/short-term-emotion
```

返回 JSON：

```json
{
  "score": 67,
  "phase": "活跃偏分歧",
  "action": "轻仓参与",
  "riskLevel": "中等偏高",
  "suggestedWeight": "2-4成",
  "mainTheme": "机器人",
  "updateTime": "14:32:18",
  "isTrading": true,
  "explanation": "当前短线情绪 67 分，处于活跃偏分歧...",
  "dashboard": [],
  "components": [],
  "riskSignals": [],
  "intradayEvents": []
}
```

---

### 阶段 1：后端纯评分器

**本阶段目标：** 只完成可单测的评分模型和评分函数，不接真实数据、不加 HTTP 路由、不碰 Flutter。

**文件：**
- 新建：`backend/flutter_api/short_term_emotion.go`
- 新建/修改测试：`backend/flutter_api/short_term_emotion_test.go`

**接口：**
- 产出：`func CalculateShortTermEmotion(input ShortTermEmotionInput) ShortTermEmotion`
- 产出：`func BuildShortTermEmotionEmpty(now time.Time, reason string) ShortTermEmotion`

- [ ] **步骤 1：写失败测试**

在 `backend/flutter_api/short_term_emotion_test.go` 写三组测试：

- `活跃但有风险`：红盘率较好、涨停较多、炸板率 >= 30%，期望 `Phase == "活跃偏分歧"`、`Action == "轻仓参与"`、`RiskLevel == "中等偏高"`。
- `冰点退潮`：下跌家数明显大于上涨家数、跌停多、空头异动多，期望分数 <= 35、`Action == "谨慎观察"`。
- `数据不足`：返回 `Phase == "数据不足"`、`Score == 0`。

运行：

```bash
go test ./backend/flutter_api -run 'TestCalculateShortTermEmotion|TestBuildShortTermEmotionEmpty' -v
```

预期：实现前编译失败，提示类型或函数不存在。

- [ ] **步骤 2：实现模型和评分函数**

在 `backend/flutter_api/short_term_emotion.go` 中实现：

- `ShortTermEmotion`
- `ShortTermEmotionMetric`
- `ShortTermEmotionComponent`
- `ShortTermEmotionSignal`
- `ShortTermEmotionEvent`
- `ShortTermEmotionInput`
- `CalculateShortTermEmotion`
- `BuildShortTermEmotionEmpty`

默认权重：

- `宽度情绪`：25
- `涨跌停质量`：25
- `连板生态`：20
- `异动强弱`：15
- `板块主线`：10
- `指数量能`：5

避坑惩罚：

- `炸板率 >= 30%`：扣 8 分，最高建议不超过 `轻仓参与`。
- `跌停 >= 20`：扣 12 分，风险等级至少为 `高`。
- `空头异动 > 多头异动`：扣 10 分，降低动作建议。

- [ ] **步骤 3：验证阶段 1**

运行：

```bash
go test ./backend/flutter_api -run 'TestCalculateShortTermEmotion|TestBuildShortTermEmotionEmpty' -v
```

预期：PASS。

- [ ] **阶段 1 暂停点**

向用户汇报：

- 新增了哪些模型和函数。
- 三组评分测试是否通过。
- 当前尚未接入真实数据和 HTTP 接口。

然后停止，等待用户确认再进入阶段 2。

---

### 阶段 2：后端实时聚合和 REST 接口

**本阶段目标：** 接入真实现有数据源，暴露 `/api/short-term-emotion`，但仍不改 Flutter App。

**文件：**
- 修改：`backend/flutter_api/short_term_emotion.go`
- 修改：`backend/flutter_api/server.go`
- 修改测试：`backend/flutter_api/short_term_emotion_test.go`

**接口：**
- 产出：`func GetShortTermEmotion(isTrading bool) ShortTermEmotion`
- 产出：`func handleShortTermEmotion(w http.ResponseWriter, r *http.Request)`
- 注册：`mux.HandleFunc("/api/short-term-emotion", handleShortTermEmotion)`

- [ ] **步骤 1：写归一化测试**

测试：

```go
func normalizeUplimitHot(raw map[string]any) uplimitHotSummary
func summarizeStockChanges(resp *data.StockChangesResponse) stockChangeSummary
```

覆盖：

- 从 `GetUplimitHot` 原始 map 中提取主线名称、主线分、最高连板、3 板以上数量、封板数量。
- 从异动列表中统计多头异动、空头异动、炸板数量。

运行：

```bash
go test ./backend/flutter_api -run 'TestNormalizeUplimitHot|TestSummarizeStockChanges' -v
```

预期：实现前编译失败。

- [ ] **步骤 2：实现实时聚合**

`GetShortTermEmotion(isTrading bool)` 流程：

1. 使用 `Asia/Shanghai` 当前时间。
2. 读取 `data.NewMarketStatisticApi().GetTodayData()`。
3. 无市场统计时返回 `BuildShortTermEmotionEmpty(now, "暂无市场统计数据")`。
4. 取最新一条市场统计作为市场宽度来源。
5. 调用 `data.NewStockChangesApi().GetStockChanges(changeTypes, 0, 500)` 获取盘中异动。
6. 调用 `data.NewMarketNewsApi().GetUplimitHot(today, 20)` 获取涨停梯队。
7. 归一化为 `ShortTermEmotionInput`。
8. 调用 `CalculateShortTermEmotion`。

v1 中实时两市成交额使用：

```go
TurnoverText: "实时成交额待接入"
VolumeScore: 50
```

- [ ] **步骤 3：实现 HTTP handler 并注册路由**

`handleShortTermEmotion` 要求：

- 只允许 GET。
- 调用现有 `isTradingTime()`。
- 返回 `WriteJSON(w, GetShortTermEmotion(isTradingTime()))`。
- 失败时返回一个可展示的 `ShortTermEmotion`，不让 Flutter 页面白屏。

在 `backend/flutter_api/server.go` 的 `Start()` 路由注册区添加：

```go
mux.HandleFunc("/api/short-term-emotion", handleShortTermEmotion)
```

- [ ] **步骤 4：验证阶段 2**

运行：

```bash
go test ./backend/flutter_api -run 'Test.*ShortTermEmotion|TestNormalizeUplimitHot|TestSummarizeStockChanges' -v
```

可选手测：

```bash
curl http://localhost:8080/api/short-term-emotion
```

预期：测试 PASS；接口返回包含 `score`、`phase`、`dashboard`、`riskSignals` 的 JSON。

- [ ] **阶段 2 暂停点**

向用户汇报：

- REST 接口是否可用。
- 真实数据源接入了哪些。
- 成交额仍是中性占位。

然后停止，等待用户确认再进入阶段 3。

---

### 阶段 3：Flutter 数据层和 ViewModel

**本阶段目标：** Flutter 能请求并解析 `/api/short-term-emotion`，但暂不做页面、不接底部 Tab。

**文件：**
- 新建：`trading_app/lib/features/short_term_emotion/domain/short_term_emotion_models.dart`
- 新建：`trading_app/lib/features/short_term_emotion/data/short_term_emotion_repository.dart`
- 新建：`trading_app/lib/features/short_term_emotion/presentation/short_term_emotion_view_model.dart`
- 修改：`trading_app/lib/app/app_config.dart`

**接口：**
- 产出：`ShortTermEmotion.fromJson(Map<String, dynamic> json)`
- 产出：`ShortTermEmotionRepository.fetch()`
- 产出：`ShortTermEmotionViewModel.load()` / `refresh()`

- [ ] **步骤 1：定义 Dart 模型**

`short_term_emotion_models.dart` 包含：

- `ShortTermEmotion`
- `ShortTermEmotionMetric`
- `ShortTermEmotionComponent`
- `ShortTermEmotionSignal`
- `ShortTermEmotionEvent`

字段与后端 JSON 保持一致：

- `score`
- `phase`
- `action`
- `riskLevel`
- `suggestedWeight`
- `mainTheme`
- `updateTime`
- `isTrading`
- `explanation`
- `dashboard`
- `components`
- `riskSignals`
- `intradayEvents`

- [ ] **步骤 2：实现 Repository**

`short_term_emotion_repository.dart` 使用 `createApiClient()`：

```dart
final resp = await _dio.get('/api/short-term-emotion');
return ShortTermEmotion.fromJson(resp.data as Map<String, dynamic>);
```

- [ ] **步骤 3：实现 ViewModel 并注册 Provider**

`ShortTermEmotionViewModel` 继承 `ChangeNotifier`，包含：

- `ViewState state`
- `ShortTermEmotion? emotion`
- `Future<void> load()`
- `Future<void> refresh()`

在 `trading_app/lib/app/app_config.dart` 添加：

```dart
ChangeNotifierProvider(
  create: (_) => ShortTermEmotionViewModel(ShortTermEmotionRepository())..load(),
),
```

- [ ] **步骤 4：验证阶段 3**

运行：

```bash
cd trading_app && flutter analyze
```

预期：无新增 error。若项目已有历史 warning，记录但不扩大范围处理。

- [ ] **阶段 3 暂停点**

向用户汇报：

- Flutter 数据模型、Repository、ViewModel 已完成。
- 目前还没有 UI 页面和底部 Tab。
- `flutter analyze` 结果。

然后停止，等待用户确认再进入阶段 4。

---

### 阶段 4：Flutter 超短情绪页面

**本阶段目标：** 实现 `ShortTermEmotionPage`，可由代码直接引用，但暂不接入底部 Tab。

**文件：**
- 新建：`trading_app/lib/features/short_term_emotion/presentation/short_term_emotion_page.dart`

**接口：**
- 消费：`ShortTermEmotionViewModel`
- 产出：首页可用页面 `ShortTermEmotionPage`

- [ ] **步骤 1：实现页面结构**

页面必须包含：

- 顶部标题：`超短情绪`
- 副标题或说明：`短线避坑`
- 右上角刷新按钮
- 顶部结论卡
- `盯盘仪表盘`
- `评分拆解`
- `短线避坑结论`
- `风险信号`

页面不包含：

- 单独的 `交易前检查清单`
- Wails/Vue 相关内容

- [ ] **步骤 2：贴合现有 App 审美**

视觉约束：

- 背景使用 `AppColors.scaffoldBg` 或白底。
- 卡片使用 `AppColors.cardBg`，圆角 10-12。
- 分割线使用 `AppColors.divider`。
- 涨/积极使用 `AppColors.textPriceUp` 或 `AppColors.success`。
- 跌/风险使用 `AppColors.textPriceDown`、`AppColors.warning`、`AppColors.error`。
- 字号接近 `NewsPage` 和 `RadarPage`。
- 信息密度偏高，适合盯盘。

- [ ] **步骤 3：实现刷新和状态**

页面要求：

- 首次进入自动加载。
- 下拉刷新或右上刷新按钮触发 `refresh()`。
- 可选 10 秒自动刷新；如果加入自动刷新，页面 dispose 时必须取消 timer。
- 错误态显示重试按钮。
- 空数据态显示 `暂无市场统计数据`。

- [ ] **步骤 4：验证阶段 4**

运行：

```bash
cd trading_app && flutter analyze
```

预期：无新增 error。

- [ ] **阶段 4 暂停点**

向用户汇报：

- 页面文件已完成。
- 页面尚未进入底部 Tab，避免影响主导航。
- `flutter analyze` 结果。

然后停止，等待用户确认再进入阶段 5。

---

### 阶段 5：首页底部 Tab 接入，并隐藏策略吧入口

**本阶段目标：** 把已经完成的 Flutter 页面接入首页底部导航，并隐藏策略吧入口。

**文件：**
- 修改：`trading_app/lib/app/app_shell.dart`

**接口：**
- 新 Tab 顺序：`盘达`、`超短情绪`、`市场快讯`、`我的`
- `策略吧` 代码保留，但入口注释隐藏。

- [ ] **步骤 1：新增页面 import，注释策略吧入口 import**

添加：

```dart
import '../features/short_term_emotion/presentation/short_term_emotion_page.dart';
```

策略吧 import 注释保留：

```dart
// 策略吧暂时隐藏入口，页面逻辑保留，后续需要时可恢复。
// import '../features/strategy/presentation/strategy_page.dart';
```

- [ ] **步骤 2：调整 `_buildPage`**

目标映射：

```dart
0 => const RadarPage(),
1 => const ShortTermEmotionPage(),
2 => NewsPage(key: newsKey),
3 => const ProfilePage(),
```

保留策略吧映射注释：

```dart
// 策略吧暂时隐藏入口，原映射保留参考：
// 3 => const StrategyPage(),
```

- [ ] **步骤 3：调整 `_items`**

目标底部导航：

```dart
BottomNavigationBarItem(
  icon: Icon(Icons.radar_outlined),
  activeIcon: Icon(Icons.radar),
  label: '盘达',
),
BottomNavigationBarItem(
  icon: Icon(Icons.speed_outlined),
  activeIcon: Icon(Icons.speed),
  label: '超短情绪',
),
BottomNavigationBarItem(
  icon: Icon(Icons.article_outlined),
  activeIcon: Icon(Icons.article),
  label: '市场快讯',
),
BottomNavigationBarItem(
  icon: Icon(Icons.person_outline),
  activeIcon: Icon(Icons.person),
  label: '我的',
),
```

把策略吧 item 注释保留：

```dart
// 策略吧暂时隐藏入口，后续恢复时插回底部导航。
// BottomNavigationBarItem(
//   icon: Icon(Icons.local_fire_department_outlined),
//   activeIcon: Icon(Icons.local_fire_department),
//   label: '策略吧',
// ),
```

- [ ] **步骤 4：修正市场快讯刷新索引**

市场快讯 index 从 1 变为 2。

把：

```dart
if (index == 1 && index == _currentIndex) {
  _newsKey++;
}
```

改成：

```dart
if (index == 2 && index == _currentIndex) {
  _newsKey++;
}
```

把：

```dart
_buildPage(index, newsKey: index == 1 ? ValueKey(_newsKey) : null)
```

改成：

```dart
_buildPage(index, newsKey: index == 2 ? ValueKey(_newsKey) : null)
```

- [ ] **步骤 5：验证阶段 5**

运行：

```bash
cd trading_app && flutter analyze
```

手动验证：

- 底部 Tab 顺序为：`盘达`、`超短情绪`、`市场快讯`、`我的`。
- `策略吧` 底部入口不可见。
- 重复点击 `市场快讯` 仍能刷新新闻。

- [ ] **阶段 5 暂停点**

向用户汇报：

- 底部 Tab 是否已按顺序接入。
- 策略吧是否只隐藏入口、未删除逻辑。
- 静态检查和手动验证结果。

然后停止，等待用户确认再进入阶段 6。

---

### 阶段 6：端到端验证和说明文档

**本阶段目标：** 做最终联调验证，补充用户说明文档。

**文件：**
- 新建或修改：`docs/超短情绪短线避坑说明.md`

**接口：**
- 确认后端接口、Flutter 页面、Tab 顺序都可用。

- [x] **步骤 1：补充说明文档**

文档说明：

- 这是市场状态和风险控制工具，不是确定性买卖信号。
- 默认权重。
- `中等偏避坑` 的含义。
- v1 局限：实时两市成交额暂为中性占位。

- [x] **步骤 2：完整验证**

运行：

```bash
go test ./backend/flutter_api -run 'Test.*ShortTermEmotion|TestNormalizeUplimitHot|TestSummarizeStockChanges' -v
cd trading_app && flutter analyze
```

可选接口手测：

```bash
curl http://localhost:8080/api/short-term-emotion
```

- [x] **步骤 3：最终手动验收**

验证：

- `盘达` 可正常打开。
- `超短情绪` 可正常打开并加载数据。
- `市场快讯` 可正常打开并重复点击刷新。
- `我的` 可正常打开。
- `策略吧` 入口不可见。
- 接口失败时 `超短情绪` 页面显示错误态和重试按钮，不白屏。

- [x] **阶段 6 完成点**

向用户汇报最终结果、测试结果、已知限制和后续可选增强项。

---

## 自检记录

- 范围覆盖：已覆盖 Flutter App 首页 Tab、策略吧隐藏、Flutter REST API、Flutter 页面、Provider、后端评分与聚合。
- 已移除 Wails/Vue 页面任务，不修改 `frontend/src`。
- 后端业务新增文件放在 `backend/flutter_api`，只调用 `backend/data` 现有能力。
- 每个阶段都有独立验证命令和明确暂停点。
- Tab 索引变化已覆盖市场快讯重复点击刷新逻辑。
- 后续增强项：实时两市成交额、历史情绪回放、权重配置、个性化交易反馈校准，应拆成独立计划。
