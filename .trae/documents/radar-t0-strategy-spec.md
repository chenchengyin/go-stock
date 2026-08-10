# Spec: 盘达「主板策略」Tab

## 需求描述

在盘达（雷达）页面新增第 4 个 Tab「主板策略」，请求后端 `/api/t0-selection` 接口获取 T0 开盘日线选股结果，以列表形式展示。

## 功能需求

### FR1: Tab 新增
- 盘达页面 TabBar 从 3 个变为 4 个：**监控股票(自选)** / **自选异动** / **全市场** / **主板策略**
- «主板策略» Tab 位于最右侧

### FR2: 数据加载
- 首次切换到«主板策略» Tab 时，请求 `GET /api/t0-selection?date=YYYY-MM-DD`（date 可选，默认当天）
- 加载中显示 `CircularProgressIndicator`
- 加载失败显示错误信息 Text
- 空结果显示"暂无符合条件的股票"

### FR3: 数据展示
每只股票展示以下字段：

| 字段 | 样式 | 数据源 |
|------|------|--------|
| 股票名称 | 14px, w600, 主文字色 | `股票名称` |
| 股票代码 | 12px, grey | `股票代码` (去除 .XSHE/.XSHG 后缀) |
| T0 开盘涨幅 | 16px, bold, 红涨绿跌 | `T0开盘涨幅(%)` |
| 前收盘价 | 12px, 次要文字色, ¥ 前缀 | `前一交易日收盘` |
| 成交额 | 12px, 次要文字色, XX 亿 | `成交额(亿)` |
| 涨停日期 | 12px, 次要文字色 | `涨停日期` |
| 同花顺按钮 | kline_button.png 图标, 24x24 | 代码去后缀后用 StockLauncher |

列表按 API 返回的顺序展示（后端已按成交额降序）。

### FR4: 同花顺跳转
- 点击 kline_button.png 图标调用 `StockLauncher.openTongHuaShun(code: xxx)`
- 代码需去除 `.XSHE` / `.XSHG` 后缀得到纯数字

### FR5: 不含异动元素
- 无 «异动» 红色标签
- 无 盘口语言标签
- 无 X 移除按钮
- 无 点击跳转详情页

## 技术设计

### 新增文件

| 文件 | 说明 |
|------|------|
| `trading_app/lib/features/radar/presentation/radar_list/t0_strategy_view_model.dart` | ViewModel + 数据模型 |

### 修改文件

| 文件 | 改动 |
|------|------|
| `radar_page.dart` | Tab 数 3→4, 新增 Tab, 新增卡片组件, 新增懒加载标记 |
| `app_config.dart` | 注册 `ChangeNotifierProvider<T0StrategyViewModel>` |

### 数据模型 (fromJson)

```
T0StrategyStock:
  - stockCode: String
  - stockName: String
  - openGap: double      // T0开盘涨幅(%)
  - closeRet: double     // T0收盘涨幅(%)
  - limitUpDates: String  // 涨停日期
  - ma20: double         // MA20
  - amountYi: double     // 成交额(亿)
  - prevClose: double    // 前一交易日收盘
  - prevCloseRet: double // 前一交易日收盘涨幅(%)
```

### ViewModel 状态

| 状态 | 类型 | 初始值 |
|------|------|--------|
| results | `List<T0StrategyStock>` | `[]` |
| loading | `bool` | `false` |
| error | `String?` | `null` |

### 卡片布局

```
┌───────────────────────────────────────────────────┐
│ 巨人网络  002558              +1.31% [kline_btn]  │  ← Row: 名称代码 + 涨幅 + 按钮
│ ¥11.45  成交额 43.21亿  涨停: 2026-07-29         │  ← Row: 前收盘 + 成交额 + 涨停日期
│ 收盘涨幅 -0.52%                                   │  ← Row: T0收盘涨幅
└───────────────────────────────────────────────────┘
```

### API 调用

```dart
final dio = createApiClient();
final resp = await dio.get('/api/t0-selection',
  queryParameters: {'date': selectedDate},
);
// resp.data = {"date":"...", "count":36, "results":[...]}
```
