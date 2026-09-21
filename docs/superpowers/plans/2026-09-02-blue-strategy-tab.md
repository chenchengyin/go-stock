# 蓝策 Tab Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** 在「主板策略」右侧增加「蓝策」Tab，复用同一份 T0 结果和日期状态，只展示 `买入信号 == blue` 的股票。

**Architecture:** 保持后端 `/api/t0-selection`、归档和 T0 选股逻辑不变。在 `T0StrategyViewModel` 中从现有 `results` 派生只读的 `blueResults`，`RadarPage` 复用现有策略列表、日期栏、预热状态和竞价轮询，只替换待展示列表。

**Tech Stack:** Flutter / Dart 3.8.1、`provider`、`flutter_test`、现有 `T0StrategyViewModel` 和 `RadarPage`。

**Spec:** `docs/superpowers/specs/2026-09-02-blue-strategy-tab-design.md`

## Global Constraints

- Tab 名称为 `蓝策`；有结果时显示数量，如 `蓝策(3)`。
- `蓝策` 紧邻「主板策略」右侧。
- 过滤规则严格为 `买入信号 == blue`；其他灯色、空信号和缺失信号均不显示。
- 当日正式结果、09:15～09:25 竞价候选预览和历史归档都使用相同过滤规则。
- 过滤后保持主板策略当前顺序，不新增蓝策专属排序规则。
- 日期选择、前一天/后一天导航和时间窗口逻辑完全沿用现状。
- 主板策略和蓝策共用同一个 ViewModel、请求、轮询和缓存；首次进入策略区只初始化加载一次，日期切换仍按现有逻辑请求。
- 5 个 Tab 使用横向滚动，保持现有标签的可读性。
- 不修改后端 T0 选股条件、买入信号计算、API、归档 JSON、缓存目录或数据库。
- 本次只修改 Flutter 源码和测试，不重建 `trading_app/build/web`。
- 当前工作区已有数据库、缓存、`.pnpm-store` 和构建产物改动必须保留，不能加入本次提交。
- 实现阶段使用分支 `codex/t0-blue-strategy-tab`；提交信息沿用项目现有 `feat(t0): ...` 习惯。

## 执行前置（不计入功能 Task）

### 流程图

```text
当前 dev 工作区（含既有脏文件）
        |
        v
创建/复用隔离 worktree + codex/t0-blue-strategy-tab
        |
        v
记录基线 SHA、运行相关 Flutter 测试
        |
        v
仅在隔离 worktree 中按 Task 1 → Task 2 → Task 3 实现
```

### 伪代码

```text
检查当前分支、worktree 和 git status
不清理、不 stash、不 reset 当前工作区的数据库、缓存和构建产物
使用 using-git-worktrees skill 创建或复用目标分支和隔离 worktree
记录 BASE_SHA = worktree 初始 HEAD
在 worktree 中执行 flutter pub get 和相关测试基线
若基线失败，记录失败并停止，不把基线问题混入本功能
```

### 注意点

- 当前仓库有大量既有脏文件；实现代码、测试和提交都必须在隔离 worktree 中进行。
- 计划文档可以继续留在当前共享工作区，不要为了携带文档而暂存或提交既有脏文件。
- 后续提交范围检查使用前置记录的 `BASE_SHA` 或 `git merge-base HEAD dev`，不要固定假设一定是 `HEAD~2`。

---

## 文件职责与改动面

| 文件 | 职责与本次改动 |
| --- | --- |
| `trading_app/lib/features/radar/presentation/radar_list/t0_strategy_view_model.dart` | 保持 T0 网络、日期和状态机不变；新增 `blueResults` 只读派生列表 |
| `trading_app/lib/features/radar/presentation/radar_list/radar_page.dart` | 将 4 个 Tab 扩展为 5 个；加入蓝策标签和页面；复用策略列表构建逻辑；覆盖点击/滑动懒加载 |
| `trading_app/test/t0_strategy_view_model_test.dart` | 验证正式结果、候选预览和历史归档的蓝灯过滤及顺序保持 |
| `trading_app/test/radar_page_test.dart` | 验证 5 个 Tab、蓝策位置和主板/蓝策数量标签 |

不创建新的后端文件、Flutter 页面文件、API 参数、缓存文件或数据库结构。

## 任务之间的接口

任务 1 产出以下 ViewModel 接口，任务 2 直接消费它：

```dart
List<T0StrategyStock> get blueResults;
```

任务 2 将现有页面方法扩展为以下签名，主板和蓝策共用同一渲染入口：

```dart
Widget _buildStrategyTab(
  T0StrategyViewModel vm, {
  bool blueOnly = false,
});
```

当 `blueOnly == false` 使用 `vm.results`；当 `blueOnly == true` 使用 `vm.blueResults`。

---

### Task 1: 增加 ViewModel 蓝灯派生列表

#### 流程图

```text
现有 results
    |
    v
按 buySignal == "blue" 过滤
    |
    v
blueResults（只读、保持原顺序）
```

#### 伪代码

```text
blueResults:
  遍历当前 results
  只保留 buySignal == "blue"
  返回不可修改列表
```

#### 注意点

- 过滤必须基于当前 `_results`，不能重新请求、重新排序或复制一套状态。
- 正式结果、候选预览和历史归档只要最终进入 `_results`，自然共用同一个 getter。
- 测试需覆盖空/缺失信号，并确认原始 `results` 顺序和内容不变。

**Files:**

- Modify: `trading_app/lib/features/radar/presentation/radar_list/t0_strategy_view_model.dart:151-158`
- Test: `trading_app/test/t0_strategy_view_model_test.dart`

**Interfaces:**

- Consumes: 现有 `_results` 和 `T0StrategyStock.buySignal`。
- Produces: `List<T0StrategyStock> get blueResults`；返回当前 `results` 中 `buySignal == 'blue'` 的条目，并保持原顺序。

- [x] **Step 1: 写蓝灯过滤失败测试**

在 `trading_app/test/t0_strategy_view_model_test.dart` 增加正式结果过滤测试，使用现有 `applyResponseForTest`，同时断言派生列表不改变原始结果：

```dart
test('blueResults：只返回 blue 并保持主板策略顺序', () {
  final vm = T0StrategyViewModel();
  addTearDown(vm.dispose);

  vm.applyResponseForTest({
    'date': '2026-09-02',
    'results': [
      {'股票代码': '600001.XSHG', '股票名称': '绿股', '买入信号': 'green'},
      {'股票代码': '600002.XSHG', '股票名称': '蓝股一', '买入信号': 'blue'},
      {'股票代码': '600003.XSHG', '股票名称': '蓝股二', '买入信号': 'blue'},
      {'股票代码': '600004.XSHG', '股票名称': '无信号股'},
    ],
  });

  expect(
    vm.blueResults.map((s) => s.rawCode).toList(),
    ['600002', '600003'],
  );
  expect(
    vm.results.map((s) => s.rawCode).toList(),
    ['600001', '600002', '600003', '600004'],
  );
});
```

再增加状态覆盖测试，确认同一个 getter 对候选预览和历史归档也按同样规则工作：

```dart
test('blueResults：候选预览和历史归档均只保留 blue', () {
  final previewVm = T0StrategyViewModel(
    now: () => DateTime.utc(2026, 9, 2, 1, 20),
  );
  addTearDown(previewVm.dispose);
  previewVm.applyResponseForTest(_candidateReady(
    date: '2026-09-02',
    candidates: [
      {'股票代码': '600010.XSHG', '股票名称': '蓝色候选', '买入信号': 'blue'},
      {'股票代码': '600011.XSHG', '股票名称': '绿色候选', '买入信号': 'green'},
    ],
  ));

  final archiveVm = T0StrategyViewModel();
  addTearDown(archiveVm.dispose);
  archiveVm.applyResponseForTest({
    'archived': true,
    'date': '2026-09-01',
    'results': [
      {'股票代码': '600020.XSHG', '股票名称': '历史蓝股', '买入信号': 'blue'},
      {'股票代码': '600021.XSHG', '股票名称': '历史红股', '买入信号': 'red'},
    ],
  });

  expect(previewVm.blueResults.map((s) => s.rawCode).toList(), ['600010']);
  expect(archiveVm.blueResults.map((s) => s.rawCode).toList(), ['600020']);
});
```

- [x] **Step 2: 运行测试确认先失败**

Run:

```bash
cd /Users/vb/Projects/go-stock/trading_app
flutter test test/t0_strategy_view_model_test.dart --plain-name 'blueResults'
```

Expected: FAIL，Dart 报告 `T0StrategyViewModel` 尚未定义 `blueResults` getter。

- [x] **Step 3: 写最小 ViewModel 实现**

在现有 `results` getter 后添加只读派生 getter，不修改 `_results`、排序、网络请求或任何状态机：

```dart
List<T0StrategyStock> get results => _results;

List<T0StrategyStock> get blueResults => List.unmodifiable(
      _results.where((stock) => stock.buySignal == 'blue'),
    );
```

- [x] **Step 4: 运行 ViewModel 测试确认通过**

Run:

```bash
cd /Users/vb/Projects/go-stock/trading_app
flutter test test/t0_strategy_view_model_test.dart
```

Expected: 文件内现有测试和新增的两个 `blueResults` 测试全部 PASS。

- [x] **Step 5: 格式化并提交 ViewModel 切片**

Run:

```bash
cd /Users/vb/Projects/go-stock/trading_app
dart format lib/features/radar/presentation/radar_list/t0_strategy_view_model.dart test/t0_strategy_view_model_test.dart
flutter test test/t0_strategy_view_model_test.dart
```

确认测试仍通过后，只提交本任务的两个文件：

```bash
cd /Users/vb/Projects/go-stock
git add trading_app/lib/features/radar/presentation/radar_list/t0_strategy_view_model.dart trading_app/test/t0_strategy_view_model_test.dart
git commit -m "feat(t0): add blue strategy results"
```

---

### Task 2: 增加蓝策 Tab 并复用策略页面

#### 流程图

```text
Tab 点击或滑动稳定
        |
        v
_lazyLoadIfNeeded(index)
        |
        +-- index 1/2：共享 T0 初始化（一次）
        +-- index 3：自选异动初始化（一次）
        +-- index 4：全市场初始化（一次）
        |
        v
主板策略使用 results；蓝策使用 blueResults
```

#### 伪代码

```text
TabBar / TabBarView 触发 index
若正在滑动且尚未稳定：等待，不重复加载
若 index 属于主板策略或蓝策且策略未加载：执行现有两次请求
按 blueOnly 选择 results 或 blueResults
复用日期栏、状态提示、卡片和交互
```

#### 注意点

- `TabController`、`DefaultTabController`、`TabBar` 和 `TabBarView` 的长度及顺序必须同时更新。
- `_onTabChanged` 要覆盖点击动画结束和手势滑动稳定两个路径；幂等计数器不能因监听器多次回调而重复请求。
- 自动化测试至少覆盖：点击蓝策只初始化一次、从主板策略切回蓝策不重复请求、通过 `TabBarView` 滑动到蓝策仍能完成初始化。

**Files:**

- Modify: `trading_app/lib/features/radar/presentation/radar_list/radar_page.dart:58-105,226-315,730-865`
- Test: `trading_app/test/radar_page_test.dart`

**Interfaces:**

- Consumes: Task 1 的 `T0StrategyViewModel.blueResults`。
- Produces: 5 个 Tab 的页面顺序、共享 T0 初始化加载和 `_buildStrategyTab(vm, {bool blueOnly = false})` 渲染入口。

- [x] **Step 1: 写 Tab 结构和数量标签失败测试**

在 `trading_app/test/radar_page_test.dart` 增加使用预置 ViewModel 结果的 Widget 测试。测试通过无网络替身覆盖 `warmUpIfNeeded`、`loadAvailableDates` 和 `loadResults`，避免点击蓝策时产生真实 HTTP 请求；第一项测试验证标签顺序和数量，第二项测试真正切换到蓝策并验证列表内容：

```dart
testWidgets('蓝策位于主板策略右侧并显示蓝灯数量', (tester) async {
  SharedPreferences.setMockInitialValues({
    'voice_announcement_asked': true,
  });
  final radarVm = RadarViewModel(RadarRepositoryImpl());
  final strategyVm = _NoNetworkT0StrategyViewModel();
  final voiceVm = VoiceAnnouncementViewModel();
  addTearDown(() {
    radarVm.dispose();
    strategyVm.dispose();
    voiceVm.dispose();
  });

  strategyVm.applyResponseForTest({
    'date': '2026-09-02',
    'results': [
      {'股票代码': '600001.XSHG', '股票名称': '蓝股', '买入信号': 'blue'},
      {'股票代码': '600002.XSHG', '股票名称': '绿股', '买入信号': 'green'},
    ],
  });

  await tester.pumpWidget(
    MultiProvider(
      providers: [
        ChangeNotifierProvider.value(value: radarVm),
          ChangeNotifierProvider<T0StrategyViewModel>.value(value: strategyVm),
        ChangeNotifierProvider.value(value: voiceVm),
      ],
      child: MaterialApp(home: const RadarPage()),
    ),
  );
  await tester.pump();

  final tabs = tester.widgetList<Tab>(find.byType(Tab)).toList();
  expect(tabs.map((tab) => tab.text).toList(), [
    '监控股票(自选)',
    '主板策略(2)',
    '蓝策(1)',
    '自选异动',
    '全市场',
  ]);
});
```

在同一个测试文件中加入只用于 Widget 测试的 ViewModel 替身：

```dart
class _NoNetworkT0StrategyViewModel extends T0StrategyViewModel {
  int loadAvailableDatesCalls = 0;
  int loadResultsCalls = 0;

  @override
  Future<void> warmUpIfNeeded() async {}

  @override
  Future<void> loadAvailableDates() async {
    loadAvailableDatesCalls++;
  }

  @override
  Future<void> loadResults({String? date, bool archived = false}) async {
    loadResultsCalls++;
  }
}
```

使用该替身增加蓝策可见列表测试：

```dart
testWidgets('蓝策只显示蓝灯股票并保留策略卡片行为', (tester) async {
  SharedPreferences.setMockInitialValues({
    'voice_announcement_asked': true,
  });
  final radarVm = RadarViewModel(RadarRepositoryImpl());
  final strategyVm = _NoNetworkT0StrategyViewModel();
  final voiceVm = VoiceAnnouncementViewModel();
  addTearDown(() {
    radarVm.dispose();
    strategyVm.dispose();
    voiceVm.dispose();
  });

  strategyVm.applyResponseForTest({
    'date': '2026-09-02',
    'results': [
      {
        '股票代码': '600001.XSHG',
        '股票名称': '蓝股',
        '买入信号': 'blue',
      },
      {
        '股票代码': '600002.XSHG',
        '股票名称': '绿股',
        '买入信号': 'green',
      },
    ],
  });

  await tester.pumpWidget(
    MultiProvider(
      providers: [
        ChangeNotifierProvider.value(value: radarVm),
        ChangeNotifierProvider<T0StrategyViewModel>.value(value: strategyVm),
        ChangeNotifierProvider.value(value: voiceVm),
      ],
      child: MaterialApp(home: const RadarPage()),
    ),
  );
  await tester.pump();

  await tester.tap(find.text('蓝策(1)'));
  await tester.pumpAndSettle();

  expect(strategyVm.loadAvailableDatesCalls, 1);
  expect(strategyVm.loadResultsCalls, 1);
  expect(find.text('蓝股'), findsOneWidget);
  expect(find.text('绿股'), findsNothing);
  expect(find.byTooltip('复制股票代码'), findsOneWidget);

  await tester.tap(find.text('主板策略(2)'));
  await tester.pumpAndSettle();
  await tester.tap(find.text('蓝策(1)'));
  await tester.pumpAndSettle();
  expect(strategyVm.loadAvailableDatesCalls, 1);
  expect(strategyVm.loadResultsCalls, 1);
});
```

由于 `RadarViewModel` 构造时会启动周期刷新，新增 Widget 测试必须在测试体结束前取消其定时器；建议使用带幂等保护的 cleanup，同时保留 `addTearDown` 作为异常路径兜底，避免 Flutter 测试框架报 pending timer。

另增加一个使用相同 `_NoNetworkT0StrategyViewModel` 的滑动测试：从初始 Tab 对 `TabBarView` 连续向左拖动两页并 `pumpAndSettle()`，断言最终显示蓝股且两个加载计数仍各为 `1`，覆盖手势路径。

- [x] **Step 2: 运行 Widget 测试确认先失败**

Run:

```bash
cd /Users/vb/Projects/go-stock/trading_app
flutter test test/radar_page_test.dart --plain-name '蓝策位于主板策略右侧并显示蓝灯数量'
```

Expected: FAIL，当前页面只有 4 个 `Tab`，没有 `蓝策(1)` 标签；蓝策列表测试也无法找到蓝策页面和蓝股条目。

- [x] **Step 3: 扩展 Tab 数量、标签和加载触发**

在 `radar_page.dart` 中完成以下精确修改：

1. 将 `_tabController = TabController(length: 4, vsync: this)` 改为 `length: 5`。
2. 将 `DefaultTabController(length: 4, ...)` 改为 `length: 5`。
3. 将 `_lazyLoadIfNeeded` 的策略条件改为同时覆盖索引 1 和 2，并把自选异动/全市场索引后移：

```dart
if ((index == 1 || index == 2) && !_strategyLoaded) {
  _strategyLoaded = true;
  final t0Vm = context.read<T0StrategyViewModel>();
  t0Vm.loadAvailableDates();
  t0Vm.loadResults();
} else if (index == 3 && !_watchLoaded) {
  _watchLoaded = true;
  vm.loadWatchChanges();
} else if (index == 4 && !_allLoaded) {
  _allLoaded = true;
  vm.loadAllChanges();
}
```

4. 检查 `_buildChangeListView` 的 `RefreshIndicator` 路由条件：自选异动现在是 index `3`，只有 index `3` 才调用 `loadWatchChanges()`，其他异动 Tab 调用 `loadAllChanges()`。

5. 让点击动画和 `TabBarView` 滑动都能触发同一个有 guard 的加载入口；滑动过程中不在中间偏移位置重复触发：

```dart
void _onTabChanged() {
  if (!_tabController.indexIsChanging && _tabController.offset != 0) {
    return;
  }
  _lazyLoadIfNeeded(_tabController.index);
}
```

6. 将数量 Selector 改为同时监听主板总数和蓝灯数，并把 `TabBar` 设为横向滚动：

```dart
Selector<T0StrategyViewModel, ({int mainCount, int blueCount})>(
  selector: (_, vm) => (
    mainCount: vm.results.length,
    blueCount: vm.blueResults.length,
  ),
  builder: (_, counts, __) {
    final strategyLabel = counts.mainCount > 0
        ? '主板策略(${counts.mainCount})'
        : '主板策略';
    final blueLabel = counts.blueCount > 0
        ? '蓝策(${counts.blueCount})'
        : '蓝策';
    return TabBar(
      controller: _tabController,
      isScrollable: true,
      // 保留现有颜色、字号、indicator 和 onTap 配置。
      tabs: [
        const Tab(text: '监控股票(自选)'),
        Tab(text: strategyLabel),
        Tab(text: blueLabel),
        const Tab(text: '自选异动'),
        const Tab(text: '全市场'),
      ],
    );
  },
)
```

`onTap: _lazyLoadIfNeeded` 保持不变；它与 `_onTabChanged` 重复调用时由 `_strategyLoaded`、`_watchLoaded` 和 `_allLoaded` 保证幂等。

- [x] **Step 4: 插入蓝策页面并参数化策略列表**

在 `TabBarView.children` 中把蓝策插入主板策略之后，并保持后两个 Tab 的新索引顺序：

```dart
// Tab ② 主板策略
Consumer<T0StrategyViewModel>(
  builder: (_, vm, __) =>
      SelectionArea(child: _buildStrategyTab(vm)),
),
// Tab ③ 蓝策
Consumer<T0StrategyViewModel>(
  builder: (_, vm, __) => SelectionArea(
    child: _buildStrategyTab(vm, blueOnly: true),
  ),
),
// Tab ④ 自选异动
// Tab ⑤ 全市场
```

将方法签名改为 `Widget _buildStrategyTab(T0StrategyViewModel vm, {bool blueOnly = false})`，并在方法开头选择数据和空状态文案：

```dart
Widget _buildStrategyTab(
  T0StrategyViewModel vm, {
  bool blueOnly = false,
}) {
  final stocks = blueOnly ? vm.blueResults : vm.results;
  final emptyText = blueOnly ? '暂无蓝色灯股票' : '暂无符合条件的股票';
  final wp = vm.warmProgress;
  // 预热、loading、error、日期栏逻辑保持现有实现。
```

在现有最终列表分支中，只把 `vm.results` 替换为 `stocks`，把固定空状态文本替换为 `emptyText`：

```dart
child: stocks.isEmpty
    ? Center(
        child: Text(
          emptyText,
          style: TextStyle(
            fontSize: 14,
            color: AppColors.textTertiary,
          ),
        ),
      )
    : ListView.builder(
        padding: const EdgeInsets.symmetric(
          horizontal: 16,
          vertical: 8,
        ),
        itemCount: stocks.length,
        itemBuilder: (_, i) => _buildStrategyCard(
          stocks[i],
          preview: vm.showingCandidatePreview,
        ),
      ),
```

不要复制 `_buildStrategyDateBar`、`_buildStrategyCard`、预热进度或错误处理；这样两个 Tab 继续共享日期、状态、卡片行为和股票列表顺序。

- [x] **Step 5: 运行页面测试确认通过**

Run:

```bash
cd /Users/vb/Projects/go-stock/trading_app
dart format lib/features/radar/presentation/radar_list/radar_page.dart test/radar_page_test.dart
flutter test test/radar_page_test.dart
```

Expected: 新增的 5 Tab 测试和文件内现有文本选择、代码复制测试全部 PASS；主板策略仍位于第二个 Tab，蓝策位于第三个 Tab。

- [x] **Step 6: 提交页面切片**

代码审查发现 Tab 顺序调整后「自选异动」下拉刷新仍使用旧索引；已补充回归测试，修复为 index `3` 并单独提交 `fix(t0): refresh watch changes after tab reorder`。

只提交本任务的页面源码和页面测试：

```bash
cd /Users/vb/Projects/go-stock
git add trading_app/lib/features/radar/presentation/radar_list/radar_page.dart trading_app/test/radar_page_test.dart
git commit -m "feat(t0): add blue strategy tab"
```

---

### Task 3: 全量验证与工作区边界检查

**Files:**

- Modify: none
- Test: `trading_app/test/t0_strategy_view_model_test.dart`
- Test: `trading_app/test/radar_page_test.dart`

**Interfaces:**

- Consumes: Task 1 的 `blueResults` 和 Task 2 的 5 Tab 页面。
- Produces: 可复现的全量验证结果，以及确认本次提交未包含既有数据库、缓存和构建产物改动的交付检查。

- [x] **Step 1: 运行静态分析**

Run:

```bash
cd /Users/vb/Projects/go-stock/trading_app
flutter analyze
```

Expected: 分析结果与基线相比不新增本功能相关诊断；已有与本功能无关的 warning/info 记录其原文件和原有状态，不通过修改无关文件来消除。

- [x] **Step 2: 运行相关测试和完整 Flutter 测试**

Run:

```bash
cd /Users/vb/Projects/go-stock/trading_app
flutter test test/t0_strategy_view_model_test.dart test/radar_page_test.dart
flutter test
```

Expected: 两个相关测试文件和完整 Flutter 测试套件全部 PASS。

- [x] **Step 3: 检查格式、差异和提交范围**

从仓库根目录运行：

```bash
cd /Users/vb/Projects/go-stock
BASE_SHA=$(git merge-base HEAD dev)
git diff --check "$BASE_SHA..HEAD"
git diff --name-only "$BASE_SHA..HEAD" | sort -u
git show --stat --oneline "$BASE_SHA..HEAD"
git status --short
```

确认从 `BASE_SHA` 到当前分支的功能提交只包含以下四个实现/测试文件：

```text
trading_app/lib/features/radar/presentation/radar_list/t0_strategy_view_model.dart
trading_app/test/t0_strategy_view_model_test.dart
trading_app/lib/features/radar/presentation/radar_list/radar_page.dart
trading_app/test/radar_page_test.dart
```

同时确认 `backend/data/`、`trading_app/build/`、`.pnpm-store/` 和其他既有脏文件仍保持原状，没有被 stage 或提交。

- [x] **Step 4: 代码审查最终改动**

使用只读审查复核当前分支相对基线的完整 diff；Critical/Important 问题必须先修复并重新运行相关测试，再继续手工验收。

- [x] **Step 5: 重启 Flutter Web 并按设计执行手工验收**

在已有 Flutter 运行流程中验证：

1. 「蓝策」紧跟「主板策略」右侧；5 个 Tab 在窄屏上可横向滚动。
2. 混合多种灯色结果时，蓝策只显示蓝灯，数量与可见蓝灯列表一致。
3. 09:15～09:25 候选预览时，蓝策仍只显示蓝灯候选，实时涨幅和排序与主板策略对应条目一致。
4. 在蓝策中切换下拉日期、前一天和后一天，显示对应日期蓝灯结果；切回主板策略后日期仍一致。
5. 点击或滑动首次进入任一策略 Tab 只触发一次初始化加载；日期切换仍能按现有逻辑发起请求。
6. 没有蓝灯股票时显示「暂无蓝色灯股票」，主板策略列表不受影响。

已在隔离 worktree 启动并热重启 Flutter Web（`http://127.0.0.1:8081`），页面可正常加载；实际雷达页验收需要登录，当前浏览器无可用登录态，因此以上策略页交互由 Widget 测试覆盖，未使用凭据绕过登录。

---

## Plan Self-Review

- 规格覆盖：过滤规则、Tab 位置、数量、排序、日期切换、竞价预览、历史归档、共享请求、空状态、懒加载、测试和非目标均有对应任务或验证步骤。
- 文件职责：ViewModel 只负责派生数据；`RadarPage` 负责 Tab 和复用渲染；两份测试分别覆盖数据投影和页面结构。
- 类型一致性：Task 1 产出 `List<T0StrategyStock> get blueResults`；Task 2 使用同名 getter，并将 `_buildStrategyTab` 扩展为 `bool blueOnly = false`。
- 无后端改动：蓝策只消费现有 `买入信号` 字段，不引入 API 参数或归档格式变化。
- 无构建产物改动：全局约束和最终检查均明确排除 `trading_app/build/web`。
