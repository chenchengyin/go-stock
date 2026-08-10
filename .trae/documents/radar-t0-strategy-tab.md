# 盘达页面新增「主板策略」Tab

## 摘要

在盘达（雷达）页面新增第 4 个 Tab「主板策略」，请求 `/api/t0-selection` 接口展示 T0 选股结果，样式参考监控列表但移除异动相关元素，保留名称、涨幅、代码、成交额、同花顺跳转。

## 当前状态分析

### 现有 Tab 结构（radar_page.dart 第 197-238 行）

```
TabBar(tabs: [监控股票(自选), 自选异动, 全市场], controller: _tabController(length: 3))
TabBarView(children: [监控股票列表, 自选异动列表, 全市场异动列表])
```

- `_tabController = TabController(length: 3, ...)` — 第 34 行
- `_lazyLoadIfNeeded` — 第 47-56 行，Tab 首次切换时懒加载
- `_watchLoaded` / `_allLoaded` — 懒加载标记
- `DefaultTabController(length: 3, ...)` — 第 172 行（包裹 Scaffold）

### 参考组件

| 组件 | 文件 | 行号 |
|------|------|------|
| `_buildStockCard` | `radar_page.dart` | 378-525 |
| `formatAmount` | `stock_change_card.dart` | 251 |
| `StockLauncher.openTongHuaShun` | `stock_launcher.dart` | 7-33 |

### API 端点

`GET /api/t0-selection?date=YYYY-MM-DD` 返回：

```json
{
  "date": "2026-08-01",
  "count": 36,
  "results": [
    {
      "时间": "2026-08-01",
      "T0开盘涨幅(%)": 1.31,
      "T0收盘涨幅(%)": -0.52,
      "涨停日期": "2026-07-29",
      "MA20": 12.35,
      "成交额(亿)": 43.21,
      "股票代码": "002558.XSHE",
      "股票名称": "巨人网络",
      "前一交易日收盘": 11.45,
      "前一交易日收盘涨幅(%)": 2.14
    }
  ]
}
```

### 现有 API 调用模式（通过 Dio）

```dart
final dio = createApiClient();
final response = await dio.get('/api/t0-selection');
```

## 变更方案

### 1. 新增文件：`trading_app/lib/features/radar/presentation/radar_list/t0_strategy_view_model.dart`

**为什么新建**：T0 策略数据与新 API 耦合，与 RadarViewModel 的业务逻辑（异动监控、自选股票）完全独立，独立 ViewModel 避免污染现有代码。

**内容包括**：
- `T0StrategyViewModel extends ChangeNotifier`
- 状态字段：`List<T0StockItem> results`、`bool loading`、`String? error`、`String selectedDate`
- 方法：`Future<void> loadResults({String? date})` — 调用 `/api/t0-selection`
- 数据模型：`T0StockItem`（映射 JSON 字段）

### 2. 修改文件：`radar_page.dart`

**改动点**：

| 位置 | 变更 |
|------|------|
| 第 34 行 | `TabController(length: 3)` → `length: 4` |
| 第 52 行 | `_lazyLoadIfNeeded` 新增 `index == 3` 分支，触发策略加载 |
| 第 172 行 | `DefaultTabController(length: 3)` → `length: 4` |
| 第 197-201 行 | tabs 数组末尾新增 `Tab(text: '主板策略')` |
| 第 206-238 行 | TabBarView children 末尾新增策略 Tab |
| 新增方法 | `_buildStrategyCard` — 策略股票卡片 |
| 新增方法 | `_buildStrategyTab` — 策略 Tab 整体布局 |
| 新增懒加载标记 | `_strategyLoaded` |

**卡片样式（`_buildStrategyCard`）**：

```
┌──────────────────────────────────────────────┐
│ [股票名称] [代码]         [T0开盘涨幅%] [方向]  │
│ ¥前收盘价   成交额 XX 亿   [涨停日期]           │
│                            [同花顺按钮]        │
└──────────────────────────────────────────────┘
```

- 名称 `fontWeight: w600, fontSize: 14`
- 代码 `fontSize: 12, color: grey`
- T0 开盘涨幅用带颜色的大号字体（红涨绿跌），`fontSize: 16, fontWeight: bold`
- 前收盘价、成交额、涨停日期用次要文字
- 同花顺跳转按钮复用现有 `_openInTongHuaShun` 逻辑
- 无 异动 标签、无 盘口标签、无 X 移除按钮、无 点击跳转详情

### 3. Provider 注册（如有）

检查 `main.dart` 或路由中是否需注册 `ChangeNotifierProvider<T0StrategyViewModel>`。若需注册，添加一行。

## 假设与决策

1. **独立 ViewModel**：不复用 RadarViewModel，T0 策略逻辑独立
2. **懒加载**：与现有 Tab 行为一致，首次切换到 Tab 时才请求 API
3. **无日期选择器**：默认使用当天日期，后续可按需添加日期选择
4. **股票代码格式**：后端返回 `002558.XSHE`，跳转同花顺时 `StockLauncher._normalizeCode` 会去掉 `.XSHE` 后缀，正常兼容

## 验证步骤

1. 编译检查：`cd trading_app && flutter analyze`
2. 确认 Flutter Web 启动后盘达页面有 4 个 Tab
3. 切换到「主板策略」Tab 触发 API 请求
4. 验证列表正常渲染（名称、涨幅、代码、成交额、涨停日期）
5. 点击同花顺按钮跳转正常
6. 验证错误状态（空数据、网络错误）
