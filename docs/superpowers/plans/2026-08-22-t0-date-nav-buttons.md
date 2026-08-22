# 主板策略日期快速切换 Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** 在主板策略 Tab 日期条下拉框左右增加「前一天」「后一天」按钮，在 `dropdownDates` 归档列表内相邻切换日期。

**Architecture:** 导航逻辑封装在 `T0StrategyViewModel`（相邻日期计算 + `selectDate` 复用）；`radar_page.dart` 仅渲染 `TextButton` 并绑定 `canGo*` 与导航方法。无后端改动。

**Tech Stack:** Flutter/Dart（`trading_app`）、现有 `/api/t0-selection` 接口。

## Global Constraints

- 导航范围限定为 `dropdownDates`（归档日降序，必要时首位插入「今日」）。
- **前一天** = 列表 index + 1（更旧）；**后一天** = index − 1（更新）。
- `_loading == true` 时两按钮禁用；首尾边界对应按钮禁用。
- 数据请求复用 `selectDate()`：今日 → `loadResults()`；历史 → `loadResults(date, archived: true)`。
- 显示条件与 `showDateSelector` 相同；候选预览不显示日期条。
- 按钮文案为「前一天」「后一天」；字号 12～13；按钮与下拉框间距 4px。
- 不改动后端、不增加快捷键/手势。

## File Structure

- `trading_app/lib/features/radar/presentation/radar_list/t0_strategy_view_model.dart`：新增 `previousArchiveDate`、`nextArchiveDate`、`canGoPreviousArchive`、`canGoNextArchive`、`selectPreviousArchive()`、`selectNextArchive()`。
- `trading_app/test/t0_strategy_view_model_test.dart`：5 条归档导航单测。
- `trading_app/lib/features/radar/presentation/radar_list/radar_page.dart`：日期条 `Row` 内插入两个 `TextButton`。

---

### Task 1: ViewModel 归档相邻导航 API + 单测

**Files:**
- Modify: `trading_app/lib/features/radar/presentation/radar_list/t0_strategy_view_model.dart`
- Modify: `trading_app/test/t0_strategy_view_model_test.dart`

**Interfaces:**
- Consumes: 现有 `dropdownDates`、`selectDate(String date)`、`_selectedDate`、`_loading`
- Produces:
  - `String? get previousArchiveDate`
  - `String? get nextArchiveDate`
  - `bool get canGoPreviousArchive`
  - `bool get canGoNextArchive`
  - `Future<void> selectPreviousArchive()`
  - `Future<void> selectNextArchive()`

- [ ] **Step 1: Write the failing tests**

在 `trading_app/test/t0_strategy_view_model_test.dart` 末尾、`main()` 的 closing brace 前追加：

```dart
  group('archive date navigation', () {
    T0StrategyViewModel vmWithDates({
      required List<String> available,
      required String selectedDate,
    }) {
      final vm = T0StrategyViewModel();
      vm.applyAvailableDatesForTest(available);
      vm.applyResponseForTest({
        'archived': true,
        'date': selectedDate,
        'results': [
          {'股票代码': '600000.XSHG', '股票名称': '浦发银行'},
        ],
      });
      return vm;
    }

    test('中间项：前后相邻日期均可导航', () {
      final vm = vmWithDates(
        available: ['2026-08-12', '2026-08-11', '2026-08-10'],
        selectedDate: '2026-08-11',
      );

      expect(vm.previousArchiveDate, '2026-08-10');
      expect(vm.nextArchiveDate, '2026-08-12');
      expect(vm.canGoPreviousArchive, true);
      expect(vm.canGoNextArchive, true);
    });

    test('最新项：仅可前往更早归档', () {
      final vm = vmWithDates(
        available: ['2026-08-12', '2026-08-11'],
        selectedDate: '2026-08-12',
      );

      expect(vm.nextArchiveDate, isNull);
      expect(vm.canGoNextArchive, false);
      expect(vm.previousArchiveDate, '2026-08-11');
      expect(vm.canGoPreviousArchive, true);
    });

    test('最旧项：仅可前往更新归档', () {
      final vm = vmWithDates(
        available: ['2026-08-12', '2026-08-11'],
        selectedDate: '2026-08-11',
      );

      expect(vm.previousArchiveDate, isNull);
      expect(vm.canGoPreviousArchive, false);
      expect(vm.nextArchiveDate, '2026-08-12');
      expect(vm.canGoNextArchive, true);
    });

    test('仅一项：两方向均不可导航', () {
      final vm = vmWithDates(
        available: ['2026-08-10'],
        selectedDate: '2026-08-10',
      );

      expect(vm.previousArchiveDate, isNull);
      expect(vm.nextArchiveDate, isNull);
      expect(vm.canGoPreviousArchive, false);
      expect(vm.canGoNextArchive, false);
    });

    test('今日不在 availableDates 时，前一天指向归档最新日', () {
      final vm = T0StrategyViewModel();
      vm.applyAvailableDatesForTest(['2026-08-11', '2026-08-10']);
      vm.applyResponseForTest({
        'date': '2026-08-12',
        'results': [
          {'股票代码': '600000.XSHG', '股票名称': '浦发银行'},
        ],
      });

      expect(vm.dropdownDates, ['2026-08-12', '2026-08-11', '2026-08-10']);
      expect(vm.selectedDate, '2026-08-12');
      expect(vm.previousArchiveDate, '2026-08-11');
      expect(vm.nextArchiveDate, isNull);
    });
  });
```

- [ ] **Step 2: Run tests to verify they fail**

Run: `cd trading_app && flutter test test/t0_strategy_view_model_test.dart --name "archive date navigation" -r expanded`
Expected: FAIL，`previousArchiveDate` getter not found（或类似 undefined getter）

- [ ] **Step 3: Write minimal implementation**

在 `t0_strategy_view_model.dart` 的 `selectDate` 方法**之前**插入：

```dart
  /// 归档列表中比当前选中更旧的一项；已在最早归档时为 null
  String? get previousArchiveDate {
    final selected = _selectedDate;
    if (selected == null) return null;
    final dates = dropdownDates;
    final index = dates.indexOf(selected);
    if (index < 0 || index + 1 >= dates.length) return null;
    return dates[index + 1];
  }

  /// 归档列表中比当前选中更新的一项；已在最新项时为 null
  String? get nextArchiveDate {
    final selected = _selectedDate;
    if (selected == null) return null;
    final dates = dropdownDates;
    final index = dates.indexOf(selected);
    if (index <= 0) return null;
    return dates[index - 1];
  }

  bool get canGoPreviousArchive =>
      previousArchiveDate != null && !_loading;

  bool get canGoNextArchive => nextArchiveDate != null && !_loading;

  Future<void> selectPreviousArchive() async {
    final date = previousArchiveDate;
    if (date == null) return;
    await selectDate(date);
  }

  Future<void> selectNextArchive() async {
    final date = nextArchiveDate;
    if (date == null) return;
    await selectDate(date);
  }
```

- [ ] **Step 4: Run tests to verify they pass**

Run: `cd trading_app && flutter test test/t0_strategy_view_model_test.dart --name "archive date navigation" -r expanded`
Expected: PASS（5 tests）

Run full file: `cd trading_app && flutter test test/t0_strategy_view_model_test.dart -r expanded`
Expected: all PASS

- [ ] **Step 5: Commit**

```bash
git add trading_app/lib/features/radar/presentation/radar_list/t0_strategy_view_model.dart \
        trading_app/test/t0_strategy_view_model_test.dart
git commit -m "feat(t0): add archive prev/next date navigation in ViewModel"
```

---

### Task 2: 日期条 UI 增加「前一天」「后一天」按钮

**Files:**
- Modify: `trading_app/lib/features/radar/presentation/radar_list/radar_page.dart`（约 637–680 行，`showDateSelector` 的 `Row`）

**Interfaces:**
- Consumes: `vm.canGoPreviousArchive`、`vm.canGoNextArchive`、`vm.selectPreviousArchive()`、`vm.selectNextArchive()`

- [ ] **Step 1: 修改日期条 Row**

将 `if (vm.showDateSelector)` 内现有 `Row` 的 `children` 从：

```dart
children: [
  Text('当前显示', ...),
  const SizedBox(width: 8),
  DropdownButton<String>(...),
  Text('选股结果', ...),
],
```

替换为：

```dart
children: [
  Text(
    '当前显示',
    style: TextStyle(
      fontSize: 12,
      color: AppColors.textSecondary,
    ),
  ),
  const SizedBox(width: 8),
  TextButton(
    onPressed: vm.canGoPreviousArchive
        ? vm.selectPreviousArchive
        : null,
    style: TextButton.styleFrom(
      padding: const EdgeInsets.symmetric(horizontal: 6),
      minimumSize: Size.zero,
      tapTargetSize: MaterialTapTargetSize.shrinkWrap,
    ),
    child: Text(
      '前一天',
      style: TextStyle(
        fontSize: 12,
        color: vm.canGoPreviousArchive
            ? AppColors.textSecondary
            : AppColors.textSecondary.withValues(alpha: 0.4),
      ),
    ),
  ),
  const SizedBox(width: 4),
  DropdownButton<String>(
    value: vm.selectedDate,
    underline: const SizedBox.shrink(),
    style: TextStyle(
      fontSize: 13,
      fontWeight: FontWeight.w600,
      color: AppColors.textPrimary,
    ),
    items: vm.dropdownDates
        .map(
          (d) => DropdownMenuItem(
            value: d,
            child: Text(d),
          ),
        )
        .toList(),
    onChanged: (d) {
      if (d != null) vm.selectDate(d);
    },
  ),
  const SizedBox(width: 4),
  TextButton(
    onPressed:
        vm.canGoNextArchive ? vm.selectNextArchive : null,
    style: TextButton.styleFrom(
      padding: const EdgeInsets.symmetric(horizontal: 6),
      minimumSize: Size.zero,
      tapTargetSize: MaterialTapTargetSize.shrinkWrap,
    ),
    child: Text(
      '后一天',
      style: TextStyle(
        fontSize: 12,
        color: vm.canGoNextArchive
            ? AppColors.textSecondary
            : AppColors.textSecondary.withValues(alpha: 0.4),
      ),
    ),
  ),
  const SizedBox(width: 8),
  Text(
    '选股结果',
    style: TextStyle(
      fontSize: 12,
      color: AppColors.textSecondary,
    ),
  ),
],
```

注意：若项目 Flutter 版本不支持 `Color.withValues(alpha:)`，改用 `withOpacity(0.4)`（与项目现有写法一致即可）。

- [ ] **Step 2: 静态检查**

Run: `cd trading_app && flutter analyze lib/features/radar/presentation/radar_list/radar_page.dart`
Expected: No issues found

- [ ] **Step 3: 运行 ViewModel 测试确保无回归**

Run: `cd trading_app && flutter test test/t0_strategy_view_model_test.dart -r expanded`
Expected: all PASS

- [ ] **Step 4: 手动验证（可选但推荐）**

1. 启动后端 + Flutter Web，打开盘达 → 主板策略 Tab。
2. 有多个归档日时，确认布局为「当前显示 [前一天] [日期▼] [后一天] 选股结果」。
3. 点「前一天」切换到更旧归档；点「后一天」切回。
4. 在最早/最新日期确认对应按钮变灰且不可点。
5. 切换过程中出现 loading，完成后列表更新。

- [ ] **Step 5: Commit**

```bash
git add trading_app/lib/features/radar/presentation/radar_list/radar_page.dart
git commit -m "feat(t0): add prev/next day buttons on mainboard strategy date bar"
```

---

## Spec Self-Review (Plan vs Spec)

| Spec 要求 | 对应 Task |
|---|---|
| dropdownDates 相邻导航 | Task 1 getters + Task 1 tests |
| 边界禁用 | Task 1 `canGo*` + Task 2 `onPressed: null` |
| loading 禁用 | Task 1 `canGo*` 含 `!_loading` |
| 复用 selectDate | Task 1 `selectPreviousArchive` / `selectNextArchive` |
| showDateSelector 同显 | Task 2 仍在同一 `if` 分支 |
| TextButton 文案与间距 | Task 2 UI |
| 5 条单测 | Task 1 Step 1 |
| 不改后端 | 无 backend 任务 |

无 TBD/占位符；类型与 spec 一致。
