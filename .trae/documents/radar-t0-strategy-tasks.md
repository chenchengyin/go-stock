# Tasks: 盘达「主板策略」Tab

## Task 1: 创建 T0StrategyViewModel + 数据模型
**文件**: `trading_app/lib/features/radar/presentation/radar_list/t0_strategy_view_model.dart`
**优先级**: high
**预估**: 新文件 ~80行

- [ ] 定义 `T0StrategyStock` 数据类，含 `fromJson` 工厂方法
- [ ] 定义 `T0StrategyViewModel extends ChangeNotifier`
  - 状态: `results`, `loading`, `error`
  - 方法: `Future<void> loadResults({String? date})` — 调用 API 并解析
- [ ] API 调用: `createApiClient().get('/api/t0-selection')`, queryParameters 传 date
- [ ] 错误处理: try-catch 设置 error 字段，notifyListeners

## Task 2: 修改 app_config.dart 注册 Provider
**文件**: `trading_app/lib/app/app_config.dart`
**优先级**: high

- [ ] 在 `MultiProvider` 的 `providers` 列表中新增一行:
  `ChangeNotifierProvider(create: (_) => T0StrategyViewModel())`
- [ ] 添加正确的 import

## Task 3: 修改 radar_page.dart — Tab 结构
**文件**: `trading_app/lib/features/radar/presentation/radar_list/radar_page.dart`
**优先级**: high

- [ ] `TabController(length: 3)` → `length: 4`
- [ ] `DefaultTabController(length: 3)` → `length: 4`
- [ ] 新增懒加载标记 `_strategyLoaded = false`
- [ ] `_lazyLoadIfNeeded` 新增 `index == 3` 分支:
  ```dart
  if (index == 3 && !_strategyLoaded) {
    _strategyLoaded = true;
    context.read<T0StrategyViewModel>().loadResults();
  }
  ```
- [ ] tabs 数组新增 `const Tab(text: '主板策略')`
- [ ] TabBarView children 新增:
  ```dart
  Consumer<T0StrategyViewModel>(builder: (_, vm, __) => _buildStrategyTab(vm)),
  ```
- [ ] 添加 import: `import 't0_strategy_view_model.dart';`

## Task 4: 实现 _buildStrategyTab + _buildStrategyCard
**文件**: `trading_app/lib/features/radar/presentation/radar_list/radar_page.dart`
**优先级**: high

- [ ] `_buildStrategyTab`: SafeArea + Column
  - loading → CircularProgressIndicator
  - error → 错误文字
  - results.isEmpty → "暂无符合条件的股票"
  - 否则 → ListView.builder 调用 `_buildStrategyCard`
- [ ] `_buildStrategyCard`: Container(cardBg, 圆角12, 边框)
  - 第一行: Row [名称(14px w600), 代码(12px grey), Spacer, T0开盘涨幅(16px bold 红/绿), kline_button(24x24)]
  - 第二行: Row [¥前收盘价, 成交额 XX亿, 涨停日期] (12px 次要文字)
  - 第三行: T0收盘涨幅 (可选，如果!=-则显示)
  - 代码去后缀: `code.replaceAll(RegExp(r'\.\w+$'), '')`
  - kline_button 点击: `StockLauncher.openTongHuaShun(code: 纯数字代码)`

## Task 5: 验证
- [ ] 编译: `cd trading_app && flutter analyze` — 零错误
- [ ] 启动 Flutter Web + 后端，切换到主板策略 Tab
- [ ] 确认列表渲染、涨幅颜色正确、同花顺跳转正常
- [ ] 确认错误状态 (停掉后端测试空数据/网络错误)
