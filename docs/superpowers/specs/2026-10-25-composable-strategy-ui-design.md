# 主板与红策可组合列表 UI

用户选择完整升级（同款日K、均线、长按、T+1分时与切日联动），并要求UI层可组合。沿用Flutter与现有行情组件。

## 方案与范围

仅统一样式改动最小，但现有基础卡片已共用，无法获得完整红策体验。采用用户选择的完整展示升级：抽取可组合卡片容器、响应式信息头、信号/评级/形态标签、图表与可见加载；用插槽传入不同策略信息，保留主板自身的筛选、排序和统计口径。复用现有图表算法，不引入新的图表依赖。

## 组件边界

- StrategyCard / StrategyCardHeader：只处理边框、间距、响应式布局、点击和任意内容区，不依赖策略数据或网络。
- StrategyStockCard：将股票信息和各类标签组合成卡片，按策略类型选择内容；图表作为独立插槽。
- StrategyPatternChip / StrategyRatingChip / StrategyReferenceChips / StrategyRedRuleChips：可单独使用的信号与统计标签，说明口径保持不变。
- StrategyCharts：复用原红策日K与分时切日联动，原类名保留兼容入口。
- StrategyChartExposure<T> / StrategyStockCharts：可见后加载、错误提示、有限重试和主板图表数据装配；模块、股票、日期和刷新版本共同隔离状态。
- StrategyChartSettings：红策、主板独立持久化K线和分时开关，默认开启；兼容原红策配置与Provider类型。

## 数据与边界

主板图表请求使用radar.main_strategy。后端仅扩展两个现有行情接口的合法模块范围，沿用对应日期归档日线与完整序列回退、分时昨收和交易日解析。主板不依赖共享GOB红策的数据加载。原行情接口访问策略保持现状。

主板只为已曝光卡片请求图表，复用已有缓存；关闭面板不发对应请求。日K和分时一项失败不阻塞另一项或股票信息；切日与换股后丢弃旧请求结果；缺数据不伪造价格或借相邻交易日。原红策列表和策略回归继续使用现有批量日K加载行为。

## 验收

主板与红策共享组件，主板可显示完整图表并切日联动；主板独立开关保存后恢复且不影响红策；窄屏、长名称、多标签无溢出；长列表只请求可见主板股票；切日旧结果不能串入新卡片；原有红策、主板标签、评级、复制代码与跳转测试通过。

## 组合示例

```dart
StrategyCard(
  header: StrategyCardHeader(
    information: [Text(stock.stockName), StrategyPatternChip(stock: stock)],
    actions: [StockCodeCopyButton(code: stock.rawCode)],
  ),
  sections: [
    StrategyStockCharts(
      moduleCode: 'radar.main_strategy', date: date, code: stock.stockCode,
      showKline: true, showIntraday: true,
      klineRepository: klineRepository, intradayRepository: intradayRepository,
    ),
    customNotes,
  ],
)
```

标准股票列表使用StrategyStockCard；需要自由排列时直接组合StrategyCard、StrategyCardHeader及各个标签和图表。组合能力是代码层的插槽组件，不增加运行时拖拽编辑器。
