# 主板策略凌晨展示前日结果 Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** 上海时区 00:00～09:00 且当天预热完成时，主板策略直接展示早于当天的最近一份选股归档，而非停留在"等待选股"页。

**Architecture:** 后端在预热就绪响应里，按上海时区窗口判断后附带"最近历史归档"结果；纯函数负责时间窗口与最近归档查找，便于单测。Flutter 解析历史结果并在列表顶部提示实际日期。

**Tech Stack:** Go（`backend/flutter_api`）、Flutter/Dart（`trading_app`）、本地 selection JSON 归档。

## Global Constraints

- 窗口：上海时区 00:00（含）~ 09:00（不含），仅对"当天"请求生效。
- 只读 selection JSON，不重跑选股、不读取/刷新 gob、不修改归档。
- 历史归档取"日期 < 目标日"的最新有效 JSON，不按自然日减一天。
- 无历史归档或文件损坏时，保持现有 ready 响应且不返回 `results`，前端继续显示等待页。

## File Structure

- `backend/flutter_api/t0_selection.go`：新增 `isPreopenPrevResultWindow(now, tradeDate)`、`findLatestSelectionArchiveBefore(tradeDate)`；在 `buildPrewarmReadyResponse` 注入历史结果。
- `backend/flutter_api/t0_preopen_prev_test.go`（新建）：后端纯函数与查找单测。
- `trading_app/lib/features/radar/presentation/radar_list/t0_strategy_view_model.dart`：新增 `displayDate`、`showingHistorical` 状态与解析逻辑。
- `trading_app/lib/features/radar/presentation/radar_list/radar_page.dart`：历史列表顶部提示条。
- `trading_app/test/t0_strategy_view_model_test.dart`（新建）：ViewModel 解析单测。

---

### Task 1: 时间窗口纯函数 + 单测

**Files:**
- Modify: `backend/flutter_api/t0_selection.go`
- Create: `backend/flutter_api/t0_preopen_prev_test.go`

**Interfaces:**
- Produces: `func isPreopenPrevResultWindow(now time.Time, tradeDate string) bool`

- [ ] **Step 1: Write the failing test**

```go
package flutter_api

import (
	"testing"
	"time"
)

func TestIsPreopenPrevResultWindow(t *testing.T) {
	loc := chinaLocation()
	today := time.Date(2026, 8, 12, 0, 0, 0, 0, loc).Format("2006-01-02")

	cases := []struct {
		name      string
		now       time.Time
		tradeDate string
		want      bool
	}{
		{"00:00 窗口起点", time.Date(2026, 8, 12, 0, 0, 0, 0, loc), today, true},
		{"08:59 窗口内", time.Date(2026, 8, 12, 8, 59, 0, 0, loc), today, true},
		{"09:00 窗口结束", time.Date(2026, 8, 12, 9, 0, 0, 0, loc), today, false},
		{"非当天不生效", time.Date(2026, 8, 12, 8, 0, 0, 0, loc), "2026-08-11", false},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := isPreopenPrevResultWindow(c.now, c.tradeDate); got != c.want {
				t.Fatalf("got %v want %v", got, c.want)
			}
		})
	}
}

func TestIsPreopenPrevResultWindow_ConvertsToShanghai(t *testing.T) {
	// UTC 周二 23:00 == 上海周三 07:00，属于当天窗口内
	utc := time.Date(2026, 8, 11, 23, 0, 0, 0, time.UTC)
	tradeDate := utc.In(chinaLocation()).Format("2006-01-02")
	if !isPreopenPrevResultWindow(utc, tradeDate) {
		t.Fatal("UTC 输入应按上海时区判定在窗口内")
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./backend/flutter_api/ -run TestIsPreopenPrevResultWindow -count=1`
Expected: FAIL，`undefined: isPreopenPrevResultWindow`

- [ ] **Step 3: Write minimal implementation**

在 `t0AutoPrewarmEndHM` 常量附近新增：

```go
// isPreopenPrevResultWindow 判断 now 是否处于「上海时区当天 00:00~09:00」且 tradeDate 即当天。
// 命中时主板策略可展示前一交易日归档；09:00 起交回等待流程。
func isPreopenPrevResultWindow(now time.Time, tradeDate string) bool {
	local := now.In(chinaLocation())
	if local.Format("2006-01-02") != tradeDate {
		return false
	}
	return local.Hour()*60+local.Minute() < t0AutoPrewarmEndHM
}
```

- [ ] **Step 4: Run test to verify it passes**

Run: `go test ./backend/flutter_api/ -run TestIsPreopenPrevResultWindow -count=1`
Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add backend/flutter_api/t0_selection.go backend/flutter_api/t0_preopen_prev_test.go
git commit -m "feat(t0): add preopen prev-result time window predicate"
```

---

### Task 2: 最近历史归档查找 + 单测

**Files:**
- Modify: `backend/flutter_api/t0_selection.go`
- Modify: `backend/flutter_api/t0_preopen_prev_test.go`

**Interfaces:**
- Consumes: `t0SelectionCachePath`、`t0CacheRootPath`、`loadT0SelectionArchive`
- Produces: `func findLatestSelectionArchiveBefore(tradeDate string) (*t0SelectionArchive, bool)`

- [ ] **Step 1: Write the failing test**

追加到 `t0_preopen_prev_test.go`：

```go
func TestFindLatestSelectionArchiveBefore(t *testing.T) {
	orig := t0CacheRootPath
	t0CacheRootPath = t.TempDir()
	defer func() { t0CacheRootPath = orig }()

	// 周五、周一归档 + 当天(未来)归档
	mustSaveArchive(t, "2026-08-07", "600007.XSHG")
	mustSaveArchive(t, "2026-08-10", "600010.XSHG")
	mustSaveArchive(t, "2026-08-12", "600012.XSHG")

	// 周一(08-10) 之前的最新应为 08-07
	got, ok := findLatestSelectionArchiveBefore("2026-08-10")
	if !ok || got.Date != "2026-08-07" {
		t.Fatalf("got %+v ok=%v", got, ok)
	}

	// 08-11 之前应为 08-10，忽略当天/未来的 08-12
	got, ok = findLatestSelectionArchiveBefore("2026-08-11")
	if !ok || got.Date != "2026-08-10" {
		t.Fatalf("got %+v ok=%v", got, ok)
	}

	// 没有更早归档
	if _, ok := findLatestSelectionArchiveBefore("2026-08-06"); ok {
		t.Fatal("不应找到更早归档")
	}
}

func TestFindLatestSelectionArchiveBefore_SkipsCorrupt(t *testing.T) {
	orig := t0CacheRootPath
	t0CacheRootPath = t.TempDir()
	defer func() { t0CacheRootPath = orig }()

	mustSaveArchive(t, "2026-08-07", "600007.XSHG")
	// 写一个日期更新但内容损坏的文件
	_ = ensureT0CacheDirs()
	if err := os.WriteFile(t0SelectionCachePath("2026-08-10"), []byte("{bad json"), 0o644); err != nil {
		t.Fatal(err)
	}

	got, ok := findLatestSelectionArchiveBefore("2026-08-11")
	if !ok || got.Date != "2026-08-07" {
		t.Fatalf("应跳过损坏文件回退到 08-07，got %+v ok=%v", got, ok)
	}
}

func mustSaveArchive(t *testing.T, date, code string) {
	t.Helper()
	if err := saveT0SelectionArchive(date, []T0SelectionResult{{StockCode: code}}, true); err != nil {
		t.Fatal(err)
	}
}
```

在 `t0_preopen_prev_test.go` 顶部 import 补 `"os"`。

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./backend/flutter_api/ -run TestFindLatestSelectionArchiveBefore -count=1`
Expected: FAIL，`undefined: findLatestSelectionArchiveBefore`

- [ ] **Step 3: Write minimal implementation**

在 `loadT0SelectionArchive` 之后新增（文件已 import `os`、`path/filepath`、`strings`）：

```go
// findLatestSelectionArchiveBefore 在 selection 目录里找日期严格早于 tradeDate 的最新有效归档。
// 文件名形如 t0_selection_YYYY-MM-DD.json；损坏或非法文件跳过。
func findLatestSelectionArchiveBefore(tradeDate string) (*t0SelectionArchive, bool) {
	dir := filepath.Dir(t0SelectionCachePath(tradeDate))
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, false
	}

	const prefix = "t0_selection_"
	const suffix = ".json"
	dates := make([]string, 0, len(entries))
	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		name := e.Name()
		if !strings.HasPrefix(name, prefix) || !strings.HasSuffix(name, suffix) {
			continue
		}
		d := name[len(prefix) : len(name)-len(suffix)]
		if _, perr := time.Parse("2006-01-02", d); perr != nil {
			continue
		}
		if d >= tradeDate {
			continue
		}
		dates = append(dates, d)
	}
	if len(dates) == 0 {
		return nil, false
	}
	sort.Sort(sort.Reverse(sort.StringSlice(dates)))
	for _, d := range dates {
		if a, ok := loadT0SelectionArchive(d); ok {
			return a, true
		}
	}
	return nil, false
}
```

- [ ] **Step 4: Run test to verify it passes**

Run: `go test ./backend/flutter_api/ -run TestFindLatestSelectionArchiveBefore -count=1`
Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add backend/flutter_api/t0_selection.go backend/flutter_api/t0_preopen_prev_test.go
git commit -m "feat(t0): find latest selection archive before a date"
```

---

### Task 3: 预热就绪响应注入历史结果

**Files:**
- Modify: `backend/flutter_api/t0_selection.go`（`buildPrewarmReadyResponse`）

**Interfaces:**
- Consumes: `isPreopenPrevResultWindow`、`findLatestSelectionArchiveBefore`
- Produces: ready 响应在命中窗口且有历史归档时新增键 `results`、`display_date`、`historical:true`

- [ ] **Step 1: Write the failing test**

追加到 `t0_preopen_prev_test.go`：

```go
func TestBuildPrewarmReadyResponseInjectsHistorical(t *testing.T) {
	orig := t0CacheRootPath
	t0CacheRootPath = t.TempDir()
	defer func() { t0CacheRootPath = orig }()

	// 预热缓存（当天）+ 历史归档（前一日）
	if err := saveT0DailyCache("2026-08-12",
		[]t0Stock{{Code: "sh600000", ShortCode: "600000", Name: "浦发银行"}},
		map[string][]dailyBar{"600000": {{Date: "2026-08-11", Close: 10}}}); err != nil {
		t.Fatal(err)
	}
	mustSaveArchive(t, "2026-08-11", "600011.XSHG")

	resp := buildPrewarmReadyResponseAt("2026-08-12",
		time.Date(2026, 8, 12, 8, 0, 0, 0, chinaLocation()))
	if resp["historical"] != true {
		t.Fatalf("historical=%v", resp["historical"])
	}
	if resp["display_date"] != "2026-08-11" {
		t.Fatalf("display_date=%v", resp["display_date"])
	}
	results, ok := resp["results"].([]T0SelectionResult)
	if !ok || len(results) != 1 || results[0].StockCode != "600011.XSHG" {
		t.Fatalf("results=%v", resp["results"])
	}
}

func TestBuildPrewarmReadyResponseNoHistoricalAfter0900(t *testing.T) {
	orig := t0CacheRootPath
	t0CacheRootPath = t.TempDir()
	defer func() { t0CacheRootPath = orig }()

	if err := saveT0DailyCache("2026-08-12",
		[]t0Stock{{Code: "sh600000", ShortCode: "600000", Name: "浦发银行"}},
		map[string][]dailyBar{"600000": {{Date: "2026-08-11", Close: 10}}}); err != nil {
		t.Fatal(err)
	}
	mustSaveArchive(t, "2026-08-11", "600011.XSHG")

	resp := buildPrewarmReadyResponseAt("2026-08-12",
		time.Date(2026, 8, 12, 9, 0, 0, 0, chinaLocation()))
	if _, has := resp["historical"]; has {
		t.Fatal("09:00 起不应注入历史结果")
	}
	if _, has := resp["results"]; has {
		t.Fatal("09:00 起不应带 results")
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./backend/flutter_api/ -run TestBuildPrewarmReadyResponse -count=1`
Expected: FAIL，`undefined: buildPrewarmReadyResponseAt`

- [ ] **Step 3: Write minimal implementation**

把现有 `buildPrewarmReadyResponse` 改为薄封装，抽出可注入时间的实现，并在末尾注入历史结果：

```go
func buildPrewarmReadyResponse(tradeDate string) map[string]interface{} {
	return buildPrewarmReadyResponseAt(tradeDate, time.Now())
}

func buildPrewarmReadyResponseAt(tradeDate string, now time.Time) map[string]interface{} {
	tStart := time.Now()
	stocks, daily, ok := []t0Stock(nil), map[string][]dailyBar(nil), false
	if cached, hit := loadT0DailyCache(tradeDate); hit {
		stocks, daily, ok = cached.Stocks, cached.Daily, true
	}
	candidateCount := 0
	if ok {
		hist := make(map[string][]dailyBar, len(daily))
		for sc, bars := range daily {
			hist[sc] = histBarsBeforeTradeDate(bars, tradeDate)
		}
		step1 := filterLimitUpRecent(stocks, hist, 7, 9.8)
		step2 := filterTurnover(step1, hist, 5.0)
		candidateCount = len(step2)
	}
	prog := getT0WarmProgress(tradeDate)
	if prog.CandidateCount > 0 {
		candidateCount = prog.CandidateCount
	}
	resp := map[string]interface{}{
		"date":            tradeDate,
		"prewarm":         true,
		"status":          string(t0WarmStatusReady),
		"stock_count":     len(stocks),
		"daily_count":     len(daily),
		"daily_fetched":   len(daily),
		"daily_total":     len(stocks),
		"candidate_count": candidateCount,
		"cache_hit":       true,
		"elapsed_sec":     round2(time.Since(tStart).Seconds()),
	}

	// 凌晨窗口内：附带最近历史归档，供前端直接展示前一交易日结果
	if isPreopenPrevResultWindow(now, tradeDate) {
		if a, found := findLatestSelectionArchiveBefore(tradeDate); found {
			resp["historical"] = true
			resp["display_date"] = a.Date
			resp["results"] = a.Results
		}
	}
	return resp
}
```

- [ ] **Step 4: Run test to verify it passes**

Run: `go test ./backend/flutter_api/ -run TestBuildPrewarmReadyResponse -count=1`
Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add backend/flutter_api/t0_selection.go backend/flutter_api/t0_preopen_prev_test.go
git commit -m "feat(t0): inject latest historical results in preopen ready response"
```

---

### Task 4: Flutter ViewModel 解析历史结果 + 单测

**Files:**
- Modify: `trading_app/lib/features/radar/presentation/radar_list/t0_strategy_view_model.dart`
- Create: `trading_app/test/t0_strategy_view_model_test.dart`

**Interfaces:**
- Produces: `T0StrategyViewModel.displayDate`（`String?`）、`T0StrategyViewModel.showingHistorical`（`bool`）
- Produces: `T0StrategyViewModel.applyResponseForTest(Map<String, dynamic> data)`（测试用同步入口，复用 `loadResults` 内解析分支）

- [ ] **Step 1: Write the failing test**

```dart
import 'package:flutter_test/flutter_test.dart';
import 'package:trading_app/features/radar/presentation/radar_list/t0_strategy_view_model.dart';

void main() {
  test('凌晨 ready 历史响应：解析列表并标记历史', () {
    final vm = T0StrategyViewModel();
    vm.applyResponseForTest({
      'prewarm': true,
      'status': 'ready',
      'historical': true,
      'display_date': '2026-08-11',
      'results': [
        {'股票代码': '600011.XSHG', '股票名称': '测试股', '标记': '前一天跌停'},
      ],
    });

    expect(vm.showingHistorical, true);
    expect(vm.displayDate, '2026-08-11');
    expect(vm.results.length, 1);
    expect(vm.results.first.tag, '前一天跌停');
    expect(vm.warmProgress, isNull);
  });

  test('当天正常结果：清空历史标记', () {
    final vm = T0StrategyViewModel();
    vm.applyResponseForTest({
      'prewarm': true,
      'status': 'ready',
      'historical': true,
      'display_date': '2026-08-11',
      'results': [
        {'股票代码': '600011.XSHG', '股票名称': '测试股'},
      ],
    });
    vm.applyResponseForTest({
      'results': [
        {'股票代码': '600000.XSHG', '股票名称': '浦发银行'},
      ],
    });

    expect(vm.showingHistorical, false);
    expect(vm.displayDate, isNull);
    expect(vm.results.first.stockCode, '600000.XSHG');
  });
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `cd trading_app && flutter test test/t0_strategy_view_model_test.dart`
Expected: FAIL，`applyResponseForTest`/`showingHistorical`/`displayDate` 未定义

- [ ] **Step 3: Write minimal implementation**

在类里新增字段与 getter，并把 `loadResults` 的解析主体抽到同步方法 `_applyResponse`，`applyResponseForTest` 转调它：

```dart
  String? _displayDate;
  bool _showingHistorical = false;

  String? get displayDate => _displayDate;
  bool get showingHistorical => _showingHistorical;

  @visibleForTesting
  void applyResponseForTest(Map<String, dynamic> data) {
    _applyResponse(data, null);
    notifyListeners();
  }
```

把 `loadResults` 中 `final data = resp.data ...` 之后到 `catch` 之前的解析逻辑整体移入 `_applyResponse(data, date)`，`loadResults` 里改为调用 `_applyResponse(data, date)`。在 `_applyResponse` 内：

- 命中 `prewarm==true && status=='ready' && results 非空` 分支时（现有 L138 分支），追加：
```dart
          _displayDate = data['display_date'] as String?;
          _showingHistorical = data['historical'] as bool? ?? false;
```
- 在"正常数据"分支（现有 L156）与"空结果"分支（L163）里，追加清空：
```dart
        _displayDate = null;
        _showingHistorical = false;
```

文件顶部 import 补 `package:flutter/foundation.dart` 已存在（用于 `ChangeNotifier`），`@visibleForTesting` 同源可用。

- [ ] **Step 4: Run test to verify it passes**

Run: `cd trading_app && flutter test test/t0_strategy_view_model_test.dart`
Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add trading_app/lib/features/radar/presentation/radar_list/t0_strategy_view_model.dart trading_app/test/t0_strategy_view_model_test.dart
git commit -m "feat(radar): parse preopen historical results in T0 view model"
```

---

### Task 5: Flutter 列表顶部历史日期提示

**Files:**
- Modify: `trading_app/lib/features/radar/presentation/radar_list/radar_page.dart`（`_buildStrategyTab` 列表分支）

**Interfaces:**
- Consumes: `vm.showingHistorical`、`vm.displayDate`

- [ ] **Step 1: 手动实现（UI 展示，无独立单测；随构建验证）**

把 `_buildStrategyTab` 结尾的 `ListView.builder` 结果分支替换为在列表上方条件插入提示条：

```dart
                  : Column(
                      crossAxisAlignment: CrossAxisAlignment.stretch,
                      children: [
                        if (vm.showingHistorical && vm.displayDate != null)
                          Container(
                            width: double.infinity,
                            padding: const EdgeInsets.symmetric(
                                horizontal: 16, vertical: 8),
                            color: AppColors.cardBg,
                            child: Text(
                              '当前显示 ${vm.displayDate} 选股结果',
                              style: TextStyle(
                                  fontSize: 12, color: AppColors.textSecondary),
                            ),
                          ),
                        Expanded(
                          child: ListView.builder(
                            padding: const EdgeInsets.symmetric(
                                horizontal: 16, vertical: 8),
                            itemCount: vm.results.length,
                            itemBuilder: (_, i) =>
                                _buildStrategyCard(vm.results[i]),
                          ),
                        ),
                      ],
                    ),
```

- [ ] **Step 2: 静态分析**

Run: `cd trading_app && dart analyze lib/features/radar/presentation/radar_list/radar_page.dart`
Expected: 无新增 error（已存在的 warning 可忽略）

- [ ] **Step 3: 构建校验后端**

Run: `go build ./backend/...`
Expected: 成功

- [ ] **Step 4: Commit**

```bash
git add trading_app/lib/features/radar/presentation/radar_list/radar_page.dart
git commit -m "feat(radar): show historical date banner above strategy list"
```

---

### Task 6: 端到端验证

**Files:** 无（运行验证）

- [ ] **Step 1: 跑后端相关单测**

Run: `go test ./backend/flutter_api/ -run 'TestIsPreopenPrevResultWindow|TestFindLatestSelectionArchiveBefore|TestBuildPrewarmReadyResponse' -count=1`
Expected: PASS

- [ ] **Step 2: 跑 Flutter 单测**

Run: `cd trading_app && flutter test test/t0_strategy_view_model_test.dart`
Expected: PASS

- [ ] **Step 3: 重启服务并验证凌晨窗口响应**

Run:
```bash
lsof -tiTCP:8080 -sTCP:LISTEN | xargs kill 2>/dev/null; sleep 3
bash scripts/ensure-flutter-api.sh
curl -s "http://127.0.0.1:8080/api/t0-selection?prewarm=1" | python3 -m json.tool | head -20
```
Expected: 若当前为 00:00~09:00 交易日且已预热，返回含 `historical:true`、`display_date` 与 `results`；否则为普通 ready 响应（符合窗口规则）。

## Self-Review

- **Spec coverage:** 窗口判断(Task1)、最近归档回退+跳过损坏(Task2)、ready 注入 `results/display_date/historical`(Task3)、Flutter 解析+清空(Task4)、顶部提示(Task5)、只读不改归档/gob（各任务实现均无写操作）。全部覆盖。
- **Placeholder scan:** 无 TBD/TODO，代码步骤均给出完整代码。
- **Type consistency:** `buildPrewarmReadyResponseAt(tradeDate string, now time.Time)`、`findLatestSelectionArchiveBefore(tradeDate string) (*t0SelectionArchive, bool)`、`isPreopenPrevResultWindow(now, tradeDate)`、Dart `showingHistorical`/`displayDate`/`applyResponseForTest` 在各任务间一致。
