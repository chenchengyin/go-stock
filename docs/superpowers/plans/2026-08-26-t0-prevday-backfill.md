# T0 前一交易日自动补全 Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** 在凌晨窗口（00:00～09:00）若前一交易日选股归档缺失，自动补全日线预热、选股归档与收盘涨幅刷新，使用户在当天开盘前能看到昨日正式选股结果。

**Architecture:** 在 `tryStartT0Prewarm(today)` 之前调用 `ensurePrevTradingDayBackfillStarted(today, now)`；补全 job 在后台 goroutine 串行执行 prev 日线 → 选股 → refresh_close，进度挂在 **today** 的 `t0WarmProgress` 上并透传 `backfill_date` / `backfill_phase`；Flutter 复用预热等待 UI。

**Tech Stack:** Go（`backend/flutter_api`）、Flutter/Dart（`trading_app`）、gob 日线缓存、JSON 选股归档。

## Global Constraints

- 窗口：上海时区 00:00（含）~ 09:00（不含），仅对**当天** `tradeDate` 生效。
- 触发：仅当 `resolvePrevTradingDay` 得到的 `prevDate` **选股归档缺失**时补全；归档已存在则跳过（即使 gob 缺失）。
- 补全范围：仅紧邻前一交易日（工作日回退 + K 线校验，最多 5 个自然日）。
- 补全失败不阻塞当天预热；09:00 后不再启动新补全 job。
- 选股无结果时写入 `count: 0` 空归档，避免反复拉取。
- 客户端始终轮询 today 的 `/api/t0-selection?prewarm=1`，不改 `date` 参数。

## File Structure

- `backend/flutter_api/t0_prevday_backfill.go`（新建）：`resolvePrevTradingDay`、`needsPrevDayBackfill`、`ensurePrevTradingDayBackfillStarted`、`runT0PrevDayBackfillJob` 及辅助函数。
- `backend/flutter_api/t0_selection.go`：扩展 `t0WarmProgress`、`buildPrewarmProgressResponse`；`writeT0PrewarmHTTP` 调用补全入口。
- `backend/flutter_api/server.go`：`runT0AutoPrewarmTick` 在 today 预热前先触发补全。
- `backend/flutter_api/t0_prevday_backfill_test.go`（新建）：纯函数与响应字段单测。
- `trading_app/lib/features/radar/presentation/radar_list/t0_strategy_view_model.dart`：`T0WarmProgress` 增加 backfill 字段。
- `trading_app/lib/features/radar/presentation/radar_list/radar_page.dart`：补全进度文案。
- `trading_app/test/t0_strategy_view_model_test.dart`：backfill 响应解析单测。

---

### Task 1: 前一交易日判定纯函数

**Files:**
- Create: `backend/flutter_api/t0_prevday_backfill.go`
- Create: `backend/flutter_api/t0_prevday_backfill_test.go`

**Interfaces:**
- Produces:
  - `func hasDailyBarOnDate(bars []dailyBar, date string) bool`
  - `func isValidTradingDay(date string) bool`
  - `func resolvePrevTradingDay(tradeDate string) (string, bool)`
  - `func needsPrevDayBackfill(tradeDate string, now time.Time) (prevDate string, need bool)`

- [ ] **Step 1: Write the failing tests**

```go
package flutter_api

import (
	"testing"
	"time"
)

func TestResolvePrevTradingDay_TuesdayToMonday(t *testing.T) {
	got, ok := resolvePrevTradingDay("2026-08-26")
	if !ok || got != "2026-08-25" {
		t.Fatalf("got %q ok=%v", got, ok)
	}
}

func TestResolvePrevTradingDay_MondayToFriday(t *testing.T) {
	orig := t0CacheRootPath
	t0CacheRootPath = t.TempDir()
	defer func() { t0CacheRootPath = orig }()

	if err := saveT0DailyCache("2026-08-22",
		[]t0Stock{{Code: "sh600000", ShortCode: "600000", Name: "浦发银行"}},
		map[string][]dailyBar{"600000": {{Date: "2026-08-22", Close: 10}}}); err != nil {
		t.Fatal(err)
	}

	got, ok := resolvePrevTradingDay("2026-08-24")
	if !ok || got != "2026-08-22" {
		t.Fatalf("got %q ok=%v", got, ok)
	}
}

func TestNeedsPrevDayBackfill_RequiresPreopenWindow(t *testing.T) {
	loc := chinaLocation()
	inside := time.Date(2026, 8, 26, 8, 0, 0, 0, loc)
	outside := time.Date(2026, 8, 26, 9, 0, 0, 0, loc)

	_, needIn := needsPrevDayBackfill("2026-08-26", inside)
	_, needOut := needsPrevDayBackfill("2026-08-26", outside)
	if !needIn {
		t.Fatal("08:00 归档缺失时应需要补全")
	}
	if needOut {
		t.Fatal("09:00 不应触发补全")
	}
}

func TestNeedsPrevDayBackfill_SkipsWhenArchiveExists(t *testing.T) {
	orig := t0CacheRootPath
	t0CacheRootPath = t.TempDir()
	defer func() { t0CacheRootPath = orig }()

	mustSaveArchive(t, "2026-08-25", "600011.XSHG")
	loc := chinaLocation()
	now := time.Date(2026, 8, 26, 8, 0, 0, 0, loc)

	_, need := needsPrevDayBackfill("2026-08-26", now)
	if need {
		t.Fatal("归档已存在时不应补全")
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./backend/flutter_api -run 'TestResolvePrevTradingDay|TestNeedsPrevDayBackfill' -count=1 -v`

Expected: FAIL with undefined `resolvePrevTradingDay`

- [ ] **Step 3: Implement minimal code**

```go
package flutter_api

import (
	"time"
)

const t0PrevDayLookbackDays = 5

func hasDailyBarOnDate(bars []dailyBar, date string) bool {
	for _, b := range bars {
		if b.Date == date {
			return true
		}
	}
	return false
}

func isValidTradingDay(date string) bool {
	if isT0DailyCacheFilePresent(date) {
		return true
	}
	bars := fetchDailyKLine("600000", date)
	return hasDailyBarOnDate(bars, date)
}

func resolvePrevTradingDay(tradeDate string) (string, bool) {
	start, err := time.Parse("2006-01-02", tradeDate)
	if err != nil {
		return "", false
	}
	d := start.AddDate(0, 0, -1)
	for i := 0; i < t0PrevDayLookbackDays; i++ {
		for d.Weekday() == time.Saturday || d.Weekday() == time.Sunday {
			d = d.AddDate(0, 0, -1)
			i++
			if i >= t0PrevDayLookbackDays {
				return "", false
			}
		}
		candidate := d.Format("2006-01-02")
		if isValidTradingDay(candidate) {
			return candidate, true
		}
		d = d.AddDate(0, 0, -1)
	}
	return "", false
}

func needsPrevDayBackfill(tradeDate string, now time.Time) (string, bool) {
	if !isPreopenPrevResultWindow(now, tradeDate) {
		return "", false
	}
	prev, ok := resolvePrevTradingDay(tradeDate)
	if !ok {
		return "", false
	}
	if _, found := loadT0SelectionArchive(prev); found {
		return prev, false
	}
	return prev, true
}
```

- [ ] **Step 4: Run tests**

Run: `go test ./backend/flutter_api -run 'TestResolvePrevTradingDay|TestNeedsPrevDayBackfill' -count=1 -v`

Expected: PASS（`TestResolvePrevTradingDay_TuesdayToMonday` 若本地无 K 线可能需 temp gob；可仅跑 Monday 用例或 mock `isValidTradingDay` 为 package-level var 注入——**优先**用 temp gob 覆盖 Tuesday 用例：在 test 里写入 8/25 gob）

补充 Tuesday 测试 setup：

```go
func TestResolvePrevTradingDay_TuesdayToMonday(t *testing.T) {
	orig := t0CacheRootPath
	t0CacheRootPath = t.TempDir()
	defer func() { t0CacheRootPath = orig }()
	if err := saveT0DailyCache("2026-08-25",
		[]t0Stock{{Code: "sh600000", ShortCode: "600000", Name: "浦发银行"}},
		map[string][]dailyBar{"600000": {{Date: "2026-08-25", Close: 10}}}); err != nil {
		t.Fatal(err)
	}
	got, ok := resolvePrevTradingDay("2026-08-26")
	if !ok || got != "2026-08-25" {
		t.Fatalf("got %q ok=%v", got, ok)
	}
}
```

- [ ] **Step 5: Commit**

```bash
git add backend/flutter_api/t0_prevday_backfill.go backend/flutter_api/t0_prevday_backfill_test.go
git commit -m "feat(t0): add prev trading day resolution for backfill"
```

---

### Task 2: 扩展预热进度结构与 HTTP 响应

**Files:**
- Modify: `backend/flutter_api/t0_selection.go:156-165`（`t0WarmProgress` 结构体）
- Modify: `backend/flutter_api/t0_selection.go:662-683`（`buildPrewarmProgressResponse`）
- Modify: `backend/flutter_api/t0_prevday_backfill_test.go`

**Interfaces:**
- Consumes: Task 1 的 `needsPrevDayBackfill`
- Produces: `t0WarmProgress` 新字段 `BackfillDate string`、`BackfillPhase string`；响应 JSON 键 `backfill_date`、`backfill_phase`

- [ ] **Step 1: Write the failing test**

```go
func TestBuildPrewarmProgressResponse_IncludesBackfillFields(t *testing.T) {
	prog := t0WarmProgress{
		Status:         t0WarmStatusWarming,
		StockCount:     100,
		DailyFetched:   50,
		DailyTotal:     100,
		BackfillDate:   "2026-08-25",
		BackfillPhase:  "daily",
		StartedAt:      time.Now(),
	}
	resp := buildPrewarmProgressResponse("2026-08-26", prog)
	if resp["backfill_date"] != "2026-08-25" {
		t.Fatalf("backfill_date=%v", resp["backfill_date"])
	}
	if resp["backfill_phase"] != "daily" {
		t.Fatalf("backfill_phase=%v", resp["backfill_phase"])
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./backend/flutter_api -run TestBuildPrewarmProgressResponse_IncludesBackfillFields -count=1 -v`

Expected: FAIL（无 `BackfillDate` 字段或响应未包含键）

- [ ] **Step 3: Extend struct and response builder**

在 `t0WarmProgress` 增加：

```go
BackfillDate  string
BackfillPhase string // daily | selection | close_refresh
```

在 `buildPrewarmProgressResponse` 末尾、`return resp` 前：

```go
if prog.BackfillDate != "" {
	resp["backfill_date"] = prog.BackfillDate
}
if prog.BackfillPhase != "" {
	resp["backfill_phase"] = prog.BackfillPhase
}
```

- [ ] **Step 4: Run test**

Run: `go test ./backend/flutter_api -run TestBuildPrewarmProgressResponse_IncludesBackfillFields -count=1 -v`

Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add backend/flutter_api/t0_selection.go backend/flutter_api/t0_prevday_backfill_test.go
git commit -m "feat(t0): expose backfill progress fields in prewarm API"
```

---

### Task 3: 补全 job 与预热入口集成

**Files:**
- Modify: `backend/flutter_api/t0_prevday_backfill.go`
- Modify: `backend/flutter_api/t0_selection.go`（`writeT0PrewarmHTTP`）
- Modify: `backend/flutter_api/server.go:318-326`（`runT0AutoPrewarmTick`）
- Modify: `backend/flutter_api/t0_prevday_backfill_test.go`

**Interfaces:**
- Consumes: Task 1 `needsPrevDayBackfill`；Task 2 扩展后的 `t0WarmProgress`；现有 `tryStartT0Prewarm`、`RunT0Selection`、`saveT0SelectionArchive`、`refreshSelectionCloseRet`
- Produces:
  - `func isPrevDayBackfillInProgress(today string) bool`
  - `func ensurePrevTradingDayBackfillStarted(today string, now time.Time)`
  - `func runT0PrevDayBackfillJob(today, prevDate string)`
  - `func waitForT0PrewarmReady(tradeDate string, timeout time.Duration) error`
  - `func syncBackfillDailyProgress(today, prevDate string)`

- [ ] **Step 1: Write failing tests for in-progress detection**

```go
func TestIsPrevDayBackfillInProgress(t *testing.T) {
	setT0WarmProgressForTest("2026-08-26", t0WarmProgress{
		Status:        t0WarmStatusWarming,
		BackfillDate:  "2026-08-25",
		BackfillPhase: "daily",
	})
	if !isPrevDayBackfillInProgress("2026-08-26") {
		t.Fatal("warming + backfill_date 应视为补全进行中")
	}
	setT0WarmProgressForTest("2026-08-26", t0WarmProgress{Status: t0WarmStatusReady})
	if isPrevDayBackfillInProgress("2026-08-26") {
		t.Fatal("ready 不应视为补全进行中")
	}
}
```

- [ ] **Step 2: Run test — expect FAIL**

Run: `go test ./backend/flutter_api -run TestIsPrevDayBackfillInProgress -count=1 -v`

- [ ] **Step 3: Implement backfill orchestration**

在 `t0_prevday_backfill.go` 追加：

```go
var (
	t0BackfillMu        sync.Mutex
	t0BackfillRunning   = map[string]bool{} // key = today
)

func isPrevDayBackfillInProgress(today string) bool {
	p := getT0WarmProgress(today)
	return p.Status == t0WarmStatusWarming && p.BackfillDate != ""
}

func ensurePrevTradingDayBackfillStarted(today string, now time.Time) {
	prev, need := needsPrevDayBackfill(today, now)
	if !need {
		return
	}
	t0BackfillMu.Lock()
	if t0BackfillRunning[today] {
		t0BackfillMu.Unlock()
		return
	}
	t0BackfillRunning[today] = true
	t0BackfillMu.Unlock()

	updateT0WarmProgress(today, func(p *t0WarmProgress) {
		p.Status = t0WarmStatusWarming
		p.BackfillDate = prev
		p.BackfillPhase = "daily"
		if p.StartedAt.IsZero() {
			p.StartedAt = time.Now()
		}
	})
	go runT0PrevDayBackfillJob(today, prev)
}

func syncBackfillDailyProgress(today, prevDate string) {
	prevProg := getT0WarmProgress(prevDate)
	updateT0WarmProgress(today, func(p *t0WarmProgress) {
		p.BackfillDate = prevDate
		p.BackfillPhase = "daily"
		p.StockCount = prevProg.StockCount
		p.DailyFetched = prevProg.DailyFetched
		p.DailyTotal = prevProg.DailyTotal
		p.CandidateCount = prevProg.CandidateCount
	})
}

func waitForT0PrewarmReady(tradeDate string, timeout time.Duration) error {
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		if isT0DailyCacheFilePresent(tradeDate) {
			p := getT0WarmProgress(tradeDate)
			if p.Status == t0WarmStatusReady {
				return nil
			}
		}
		p := getT0WarmProgress(tradeDate)
		if p.Status == t0WarmStatusFailed {
			return fmt.Errorf("prewarm %s failed: %s", tradeDate, p.Err)
		}
		time.Sleep(500 * time.Millisecond)
	}
	return fmt.Errorf("prewarm %s timeout", tradeDate)
}

func runT0PrevDayBackfillJob(today, prevDate string) {
	defer func() {
		t0BackfillMu.Lock()
		delete(t0BackfillRunning, today)
		t0BackfillMu.Unlock()
	}()

	logger.SugaredLogger.Infof("[T0补全] 开始补全前一交易日 %s (today=%s)", prevDate, today)

	if !isT0DailyCacheFilePresent(prevDate) {
		tryStartT0Prewarm(prevDate)
		deadline := time.Now().Add(30 * time.Minute)
		for time.Now().Before(deadline) {
			syncBackfillDailyProgress(today, prevDate)
			if isT0DailyCacheFilePresent(prevDate) &&
				getT0WarmProgress(prevDate).Status == t0WarmStatusReady {
				break
			}
			if getT0WarmProgress(prevDate).Status == t0WarmStatusFailed {
				prevErr := getT0WarmProgress(prevDate).Err
				updateT0WarmProgress(today, func(p *t0WarmProgress) {
					p.Status = t0WarmStatusFailed
					p.Err = fmt.Sprintf("补全 %s 日线失败: %s", prevDate, prevErr)
					p.BackfillDate = prevDate
					p.BackfillPhase = "daily"
				})
				tryStartT0Prewarm(today)
				return
			}
			time.Sleep(500 * time.Millisecond)
		}
		if !isT0DailyCacheFilePresent(prevDate) {
			updateT0WarmProgress(today, func(p *t0WarmProgress) {
				p.Status = t0WarmStatusFailed
				p.Err = fmt.Sprintf("补全 %s 日线超时", prevDate)
			})
			tryStartT0Prewarm(today)
			return
		}
	}

	updateT0WarmProgress(today, func(p *t0WarmProgress) {
		p.BackfillPhase = "selection"
	})
	results, err := RunT0Selection(prevDate)
	if err != nil {
		logger.SugaredLogger.Warnf("[T0补全] 选股 %s: %v，写入空归档", prevDate, err)
		results = []T0SelectionResult{}
	}
	if saveErr := saveT0SelectionArchive(prevDate, results, false); saveErr != nil {
		logger.SugaredLogger.Warnf("[T0补全] 归档写入失败 %s: %v", prevDate, saveErr)
	}

	updateT0WarmProgress(today, func(p *t0WarmProgress) {
		p.BackfillPhase = "close_refresh"
	})
	if _, refreshErr := refreshSelectionCloseRet(prevDate, false); refreshErr != nil {
		logger.SugaredLogger.Warnf("[T0补全] refresh_close %s: %v", prevDate, refreshErr)
	}

	updateT0WarmProgress(today, func(p *t0WarmProgress) {
		p.Status = t0WarmStatusIdle
		p.BackfillDate = ""
		p.BackfillPhase = ""
	})
	logger.SugaredLogger.Infof("[T0补全] 完成 %s", prevDate)
	tryStartT0Prewarm(today)
}
```

- [ ] **Step 4: Wire HTTP 与 ticker**

`writeT0PrewarmHTTP` 改为：

```go
func writeT0PrewarmHTTP(w http.ResponseWriter, tradeDate string) {
	now := time.Now()
	ensurePrevTradingDayBackfillStarted(tradeDate, now)
	if isPrevDayBackfillInProgress(tradeDate) {
		syncBackfillDailyProgress(tradeDate, getT0WarmProgress(tradeDate).BackfillDate)
		WriteJSON(w, buildPrewarmProgressResponse(tradeDate, getT0WarmProgress(tradeDate)))
		return
	}
	// ... 原有 today 预热逻辑不变
}
```

`runT0AutoPrewarmTick`：

```go
func runT0AutoPrewarmTick(now time.Time) {
	if !shouldAutoPrewarmT0(now) {
		return
	}
	tradeDate := now.In(chinaLocation()).Format("2006-01-02")
	ensurePrevTradingDayBackfillStarted(tradeDate, now)
	if isPrevDayBackfillInProgress(tradeDate) {
		return
	}
	if started, _ := tryStartT0Prewarm(tradeDate); started {
		logger.SugaredLogger.Infof("[定时任务] T0 日线主动预热已启动: %s", tradeDate)
	}
}
```

- [ ] **Step 5: Run tests**

Run: `go test ./backend/flutter_api -run 'TestIsPrevDayBackfillInProgress|TestBuildPrewarmReadyResponseInjectsHistorical' -count=1 -v`

Expected: PASS

- [ ] **Step 6: Commit**

```bash
git add backend/flutter_api/t0_prevday_backfill.go backend/flutter_api/t0_selection.go backend/flutter_api/server.go backend/flutter_api/t0_prevday_backfill_test.go
git commit -m "feat(t0): auto backfill missing prev-day archive before prewarm"
```

---

### Task 4: Flutter 解析 backfill 字段

**Files:**
- Modify: `trading_app/lib/features/radar/presentation/radar_list/t0_strategy_view_model.dart:96-117,346-354`
- Modify: `trading_app/test/t0_strategy_view_model_test.dart`

**Interfaces:**
- Consumes: API 响应键 `backfill_date`、`backfill_phase`
- Produces: `T0WarmProgress.backfillDate`、`T0WarmProgress.backfillPhase`

- [ ] **Step 1: Write failing test**

```dart
test('warming 响应解析 backfill_date 与 backfill_phase', () {
  final vm = T0StrategyViewModel();
  vm.applyResponseForTest({
    'prewarm': true,
    'status': 'warming',
    'backfill_date': '2026-08-25',
    'backfill_phase': 'daily',
    'daily_fetched': 10,
    'daily_total': 100,
  });

  final wp = vm.warmProgress!;
  expect(wp.backfillDate, '2026-08-25');
  expect(wp.backfillPhase, 'daily');
  expect(wp.isWarming, isTrue);
});
```

- [ ] **Step 2: Run test — expect FAIL**

Run: `cd trading_app && flutter test test/t0_strategy_view_model_test.dart --name 'backfill_date'`

- [ ] **Step 3: Extend T0WarmProgress and parser**

```dart
class T0WarmProgress {
  final String status;
  final int stockCount;
  final int dailyFetched;
  final int dailyTotal;
  final int candidateCount;
  final bool prewarm;
  final String? backfillDate;
  final String? backfillPhase;

  const T0WarmProgress({
    required this.status,
    this.stockCount = 0,
    this.dailyFetched = 0,
    this.dailyTotal = 0,
    this.candidateCount = 0,
    this.prewarm = false,
    this.backfillDate,
    this.backfillPhase,
  });
  // ...
}
```

在 `_applyResponse` 构造 `T0WarmProgress` 处增加：

```dart
backfillDate: data['backfill_date'] as String?,
backfillPhase: data['backfill_phase'] as String?,
```

- [ ] **Step 4: Run test**

Run: `cd trading_app && flutter test test/t0_strategy_view_model_test.dart --name 'backfill_date'`

Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add trading_app/lib/features/radar/presentation/radar_list/t0_strategy_view_model.dart trading_app/test/t0_strategy_view_model_test.dart
git commit -m "feat(t0): parse prev-day backfill fields in view model"
```

---

### Task 5: 预热等待页补全文案

**Files:**
- Modify: `trading_app/lib/features/radar/presentation/radar_list/radar_page.dart:574-599`

**Interfaces:**
- Consumes: `T0WarmProgress.backfillDate`、`backfillPhase`

- [ ] **Step 1: Update waiting UI copy**

将 `_buildStrategyTab` 中预热文案替换为：

```dart
String _backfillTitle(T0WarmProgress wp) {
  if (wp.backfillDate != null && wp.backfillDate!.isNotEmpty) {
    return '正在补全 ${wp.backfillDate} 数据...';
  }
  return wp.isWarming ? '服务端正在预热数据...' : '数据预热完成，等待选股...';
}

String? _backfillSubtitle(T0WarmProgress wp) {
  if (wp.backfillPhase == 'selection' && wp.backfillDate != null) {
    return '生成 ${wp.backfillDate} 选股结果...';
  }
  if (wp.backfillPhase == 'close_refresh' && wp.backfillDate != null) {
    return '刷新 ${wp.backfillDate} 收盘涨幅...';
  }
  return null;
}
```

在 `Column` 中：

```dart
Text(_backfillTitle(wp), ...),
if (_backfillSubtitle(wp) != null)
  Padding(
    padding: const EdgeInsets.only(top: 8),
    child: Text(_backfillSubtitle(wp)!, style: ...),
  ),
```

保留现有 `daily_fetched/daily_total` 进度行。

- [ ] **Step 2: Manual smoke**

启动服务，删除昨日归档，08:00 前打开主板策略 Tab，确认文案为「正在补全 YYYY-MM-DD 数据…」。

- [ ] **Step 3: Commit**

```bash
git add trading_app/lib/features/radar/presentation/radar_list/radar_page.dart
git commit -m "feat(t0): show prev-day backfill progress copy in radar"
```

---

### Task 6: 端到端验证

**Files:** 无代码改动

- [ ] **Step 1: 后端全量单测**

Run: `go test ./backend/flutter_api -count=1`

Expected: PASS

- [ ] **Step 2: Flutter 单测**

Run: `cd trading_app && flutter test test/t0_strategy_view_model_test.dart`

Expected: PASS

- [ ] **Step 3: 手工场景（8/26 早盘）**

```bash
rm -f backend/data/cache/t0/selection/t0_selection_2026-08-25.json
rm -f backend/data/cache/t0/daily/t0_daily_cache_2026-08-25.gob
# 重启 flutter_api，08:00 前 curl:
curl -s 'http://127.0.0.1:8080/api/t0-selection?prewarm=1' | jq '.backfill_date,.status'
# 完成后应 historical=true, display_date=2026-08-25
curl -s 'http://127.0.0.1:8080/api/t0-selection?list_dates=1' | jq '.dates[:3]'
```

Expected: 补全过程中 `backfill_date` 为 `2026-08-25`；完成后 `list_dates` 含该日。
