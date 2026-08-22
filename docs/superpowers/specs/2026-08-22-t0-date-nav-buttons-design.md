# 主板策略日期快速切换设计

## 目标

在盘达「主板策略」Tab 顶部日期选择条中，于下拉框左右增加「前一天」「后一天」按钮，在已有选股归档日期之间快速切换，无需展开下拉列表。

## 方案选择

采用 **ViewModel 封装相邻归档导航 + 文字按钮**：

- 导航逻辑集中在 `T0StrategyViewModel`，可单测；
- UI 仅渲染按钮并调用 ViewModel 方法；
- 按钮文案为「前一天」「后一天」，与用户需求一致。

不采用 UI 层直接算 index（难测、难复用），也不采用纯图标按钮（不如文字直观）。

## 行为规则

1. 导航范围限定为 `dropdownDates`（服务端归档日降序列表，必要时在首位插入「今日」）。
2. **前一天**：切换到列表中更旧的一项（index + 1）；自动跳过无归档的自然日/周末/节假日。
3. **后一天**：切换到列表中更新的一项（index − 1）；最新项为今日或最近归档日。
4. 已在最早归档时禁用「前一天」；已在最新项时禁用「后一天」；仅一条可选日期时两按钮均禁用。
5. `_loading == true` 时两按钮均禁用，避免连点重复请求。
6. 数据请求复用现有 `selectDate()`：目标为上海时区今日时走 `loadResults()`；否则走 `loadResults(date: date, archived: true)`。
7. 显示条件与现有 `showDateSelector` 相同；候选预览阶段（`showingCandidatePreview`）不显示日期条与按钮。
8. 不新增后端接口；`loadAvailableDates()` 行为不变。

## 数据流

```
点击 前一天/后一天
  → ViewModel 根据 dropdownDates 计算相邻日期
  → selectDate(相邻日期)
  → 今日 ? loadResults() : loadResults(date, archived: true)
  → _applyResponse 成功后更新 results 与 selectedDate
```

## Flutter UI

文件：`trading_app/lib/features/radar/presentation/radar_list/radar_page.dart`

在现有日期条 `Row` 内，布局为：

```
当前显示  [ 前一天 ]  [ YYYY-MM-DD ▼ ]  [ 后一天 ]  选股结果
```

约定：

| 项 | 说明 |
|---|---|
| 控件 | `TextButton`，文案「前一天」「后一天」 |
| 样式 | 字号 12～13，与「当前显示」次要文字色一致；禁用时降低不透明度 |
| 间距 | 按钮与下拉框之间 `SizedBox(width: 4)` |
| 加载 | 切换期间沿用现有全页 `CircularProgressIndicator` |
| 范围 | 与 `DropdownButton` 同处 `if (vm.showDateSelector)` 分支 |

股票卡片、候选预览、等待/预热页均不改动。

## ViewModel API

文件：`trading_app/lib/features/radar/presentation/radar_list/t0_strategy_view_model.dart`

新增只读属性与方法：

```dart
String? get previousArchiveDate;   // 列表中更旧一项；已在最早时为 null
String? get nextArchiveDate;       // 列表中更新一项；已在最新时为 null
bool get canGoPreviousArchive => previousArchiveDate != null && !_loading;
bool get canGoNextArchive => nextArchiveDate != null && !_loading;

Future<void> selectPreviousArchive();
Future<void> selectNextArchive();
```

实现要点：

- 以 `dropdownDates.indexOf(_selectedDate!)` 定位当前项；
- `previousArchiveDate` = `index + 1 < length ? dropdownDates[index + 1] : null`；
- `nextArchiveDate` = `index > 0 ? dropdownDates[index - 1] : null`；
- `selectPreviousArchive` / `selectNextArchive` 在对应 date 非 null 时调用 `selectDate(date)`，否则 no-op。

## 错误处理与边界

| 场景 | 处理 |
|---|---|
| 请求失败 | 沿用现有 error 展示；`selectedDate` 仅在 `_applyResponse` 成功时更新，失败保持原日期 |
| 加载中 | 按钮禁用；`selectDate` 已有 `_loading` 守卫 |
| 单条归档 | 两按钮禁用 |
| 下拉与按钮并用 | 均走同一 `selectDate()` 路径，行为一致 |

## 测试

文件：`trading_app/test/t0_strategy_view_model_test.dart`

新增用例（通过 `applyAvailableDatesForTest` + `applyResponseForTest` 构造状态，无需网络）：

1. 三项归档、选中中间项：`canGoPreviousArchive` 与 `canGoNextArchive` 均为 true；`previousArchiveDate` / `nextArchiveDate` 指向正确相邻日期。
2. 选中最新项（列表 index 0）：`canGoNextArchive` 为 false；`canGoPreviousArchive` 为 true。
3. 选中最旧项：`canGoPreviousArchive` 为 false；`canGoNextArchive` 为 true。
4. 仅一项：`previousArchiveDate` 与 `nextArchiveDate` 均为 null，两 `canGo*` 为 false。
5. 今日不在 `availableDates` 但插入 `dropdownDates` 首位时，从今日点「前一天」应指向 `availableDates` 第一项。

## 范围外

- 不改动后端 `t0_selection.go`；
- 不增加键盘快捷键或手势滑动切换；
- 不改动下拉框选项排序规则。
