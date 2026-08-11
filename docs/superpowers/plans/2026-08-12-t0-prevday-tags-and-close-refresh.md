# 主板策略前日标记与收盘刷新 Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** 为主板策略结果增加唯一前日标记（写入选股 JSON），并在交易日 15:05 只刷新归档中的 `T0收盘涨幅(%)`，不重跑选股、不刷日线 gob。

**Architecture:** 标记在 `RunT0Selection` 组装阶段用已有 `hist` OHLC 纯计算；收盘刷新读取当日 selection JSON，对已有代码批量 `fetchT0Realtime` 后只改 CloseRet 并原子写回。Flutter 解析 `标记` 并在策略卡片名称旁展示。

**Tech Stack:** Go（`backend/flutter_api`）、Flutter/Dart（`trading_app`）、腾讯行情 `fetchT0Realtime`、本地 selection JSON 归档

## Global Constraints

- 标记字段名必须为 JSON 键 `标记`（string；无标记为 `""`）
- 优先级：破板 > 跌停 > 大阴线；同时破板且跌停 → `标记=""`，股票保留
- 阈值：最高涨幅 ≥ 9.85 且收盘涨幅 < 9.85 → 涨停破板；收盘涨幅 ≤ -9.9 → 前一天跌停；开盘涨幅−收盘涨幅 ≥ 4 → 前一天大阴线
- ST 与普通股同一套 10% 口径，不做 5% 分支
- 收盘刷新：周一到周五上海时区 ≥ 15:05，每天一次；只改 `T0收盘涨幅(%)`；行情缺失或价格 ≤ 0 保留旧值
- 不删除/重拉日线 gob；不因标记剔除股票；旧 JSON 无 `标记` 时前端按无标记

## File Structure

| 文件 | 职责 |
|------|------|
| `backend/flutter_api/t0_selection.go` | `Tag` 字段、`pickPrevDayTag`、组装接入、`close_updated_at`、收盘刷新、handler 参数、ticker |
| `backend/flutter_api/t0_prevday_tag_test.go` | 标记判定单测 |
| `backend/flutter_api/t0_close_refresh_test.go` | 收盘补丁与窗口判定单测 |
| `backend/flutter_api/server.go` | 注册收盘刷新 ticker |
| `trading_app/.../t0_strategy_view_model.dart` | 解析 `标记` |
| `trading_app/.../radar_page.dart` | 卡片展示红/绿标记 |

---

### Task 1: `pickPrevDayTag` 纯函数 + 单测

**Files:**
- Create: `backend/flutter_api/t0_prevday_tag_test.go`
- Modify: `backend/flutter_api/t0_selection.go`（在 `T0SelectionResult` 附近新增函数）

**Interfaces:**
- Produces: `func pickPrevDayTag(highRet, openRet, closeRet float64) string`

- [ ] **Step 1: Write the failing test**

```go
package flutter_api

import "testing"

func TestPickPrevDayTag(t *testing.T) {
	cases := []struct {
		name              string
		high, open, close float64
		want              string
	}{
		{"涨停破板", 10.0, 2.0, 5.0, "涨停破板"},
		{"最高刚好9.85且收盘更低", 9.85, 1.0, 9.84, "涨停破板"},
		{"封死涨停不算破板", 10.0, 2.0, 10.0, ""},
		{"前一天跌停", 1.0, -2.0, -9.9, "前一天跌停"},
		{"前一天大阴线", 3.0, 5.0, 0.5, "前一天大阴线"},
		{"破板优先于大阴线", 10.0, 8.0, 3.0, "涨停破板"},
		{"跌停优先于大阴线", 2.0, 1.0, -10.0, "前一天跌停"},
		{"破板且跌停则清空", 10.0, 2.0, -10.0, ""},
		{"无标记", 3.0, 1.0, 0.5, ""},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got := pickPrevDayTag(c.high, c.open, c.close)
			if got != c.want {
				t.Fatalf("pickPrevDayTag(%v,%v,%v)=%q want %q", c.high, c.open, c.close, got, c.want)
			}
		})
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `cd /Users/Zhuanz/aiproject/go-stock && go test ./backend/flutter_api/ -run TestPickPrevDayTag -count=1`

Expected: FAIL（`pickPrevDayTag` undefined）

- [ ] **Step 3: Write minimal implementation**

在 `t0_selection.go` 的 `T0SelectionResult` 结构体后增加：

```go
func pickPrevDayTag(highRet, openRet, closeRet float64) string {
	brokenLimitUp := highRet >= 9.85 && closeRet < 9.85
	limitDown := closeRet <= -9.9
	bigYin := openRet-closeRet >= 4.0

	if brokenLimitUp && limitDown {
		return ""
	}
	if brokenLimitUp {
		return "涨停破板"
	}
	if limitDown {
		return "前一天跌停"
	}
	if bigYin {
		return "前一天大阴线"
	}
	return ""
}
```

- [ ] **Step 4: Run test to verify it passes**

Run: `cd /Users/Zhuanz/aiproject/go-stock && go test ./backend/flutter_api/ -run TestPickPrevDayTag -count=1`

Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add backend/flutter_api/t0_selection.go backend/flutter_api/t0_prevday_tag_test.go
git commit -m "feat(t0): add pickPrevDayTag for prev-day board tags"
```

---

### Task 2: 结果模型写入 `标记` 并接入组装循环

**Files:**
- Modify: `backend/flutter_api/t0_selection.go`（`T0SelectionResult`、`RunT0Selection` 组装循环约 1013–1070 行）
- Test: `backend/flutter_api/t0_prevday_tag_test.go`（追加 hist 计算辅助测，可选）

**Interfaces:**
- Consumes: `pickPrevDayTag(highRet, openRet, closeRet float64) string`
- Produces: `T0SelectionResult.Tag` with `json:"标记"`

- [ ] **Step 1: Write failing test for hist-based helper**

追加：

```go
func TestPrevDayRetsFromHist(t *testing.T) {
	hist := []dailyBar{
		{Date: "2026-08-09", Close: 10},
		{Date: "2026-08-10", Open: 10.2, High: 11.0, Close: 10.5},
	}
	high, open, close, ok := prevDayRetsFromHist(hist)
	if !ok {
		t.Fatal("expected ok")
	}
	// high=(11-10)/10*100=10, open=2, close=5
	if round2(high) != 10 || round2(open) != 2 || round2(close) != 5 {
		t.Fatalf("got high=%v open=%v close=%v", high, open, close)
	}
	if _, _, _, ok := prevDayRetsFromHist(hist[:1]); ok {
		t.Fatal("len<2 should fail")
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `cd /Users/Zhuanz/aiproject/go-stock && go test ./backend/flutter_api/ -run TestPrevDayRetsFromHist -count=1`

Expected: FAIL（`prevDayRetsFromHist` undefined）

- [ ] **Step 3: Implement helper + struct field + wire assemble**

1) 给 `T0SelectionResult` 加字段：

```go
Tag string `json:"标记"`
```

2) 新增：

```go
func prevDayRetsFromHist(hist []dailyBar) (highRet, openRet, closeRet float64, ok bool) {
	if len(hist) < 2 {
		return 0, 0, 0, false
	}
	base := hist[len(hist)-2].Close
	if base == 0 {
		return 0, 0, 0, false
	}
	prev := hist[len(hist)-1]
	highRet = (prev.High - base) / base * 100
	openRet = (prev.Open - base) / base * 100
	closeRet = (prev.Close - base) / base * 100
	return highRet, openRet, closeRet, true
}
```

3) 在 `RunT0Selection` 组装 `T0SelectionResult{...}` 处计算：

```go
tag := ""
if highRet, openRet, closeRet, ok := prevDayRetsFromHist(hist); ok {
	tag = pickPrevDayTag(highRet, openRet, closeRet)
}
// ...
Tag: tag,
```

注意：组装循环里已有变量名 `prevRet` 表示前日收盘涨幅，不要与 `closeRet`（T0 当日）混淆；标记用 `prevDayRetsFromHist` 返回值。

- [ ] **Step 4: Run tests**

Run: `cd /Users/Zhuanz/aiproject/go-stock && go test ./backend/flutter_api/ -run 'TestPickPrevDayTag|TestPrevDayRetsFromHist' -count=1`

Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add backend/flutter_api/t0_selection.go backend/flutter_api/t0_prevday_tag_test.go
git commit -m "feat(t0): persist prev-day tag on selection results"
```

---

### Task 3: Flutter 解析与卡片展示标记

**Files:**
- Modify: `trading_app/lib/features/radar/presentation/radar_list/t0_strategy_view_model.dart`
- Modify: `trading_app/lib/features/radar/presentation/radar_list/radar_page.dart`（`_buildStrategyCard`）

**Interfaces:**
- Consumes: JSON `标记`
- Produces: `T0StrategyStock.tag`；UI 红/绿文本 `[涨停破板]` 等

- [ ] **Step 1: Extend model**

在 `T0StrategyStock` 增加：

```dart
final String tag; // 标记：涨停破板 / 前一天跌停 / 前一天大阴线 / 空

// constructor + fromJson:
tag: json['标记'] as String? ?? '',
```

- [ ] **Step 2: Render in `_buildStrategyCard`**

在股票名称 `Text` 之后、最新涨幅之前插入（仅 `stock.tag.isNotEmpty`）：

```dart
if (stock.tag.isNotEmpty) ...[
  const SizedBox(width: 4),
  Text(
    '[${stock.tag}]',
    style: TextStyle(
      fontSize: 11,
      fontWeight: FontWeight.w600,
      color: stock.tag == '涨停破板' ? AppColors.tagRed : AppColors.tagGreen,
    ),
  ),
],
```

确保文件已 `import` 使用到的 `AppColors`（`radar_page.dart` 通常已有）。

- [ ] **Step 3: Analyze（无独立 widget 测试时做静态检查）**

Run: `cd /Users/Zhuanz/aiproject/go-stock/trading_app && dart analyze lib/features/radar/presentation/radar_list/t0_strategy_view_model.dart lib/features/radar/presentation/radar_list/radar_page.dart`

Expected: No issues（或仅有预先存在的无关告警）

- [ ] **Step 4: Commit**

```bash
git add trading_app/lib/features/radar/presentation/radar_list/t0_strategy_view_model.dart trading_app/lib/features/radar/presentation/radar_list/radar_page.dart
git commit -m "feat(radar): show T0 prev-day tags on strategy cards"
```

---

### Task 4: `patchSelectionCloseRets` + 窗口判定单测

**Files:**
- Create: `backend/flutter_api/t0_close_refresh_test.go`
- Modify: `backend/flutter_api/t0_selection.go`

**Interfaces:**
- Produces:
  - `func shouldRefreshSelectionClose(now time.Time, closeUpdatedAt string) bool`
  - `func patchSelectionCloseRets(results []T0SelectionResult, quotes map[string]t0Realtime) (updated, kept int)`
  - shortCode 从 `股票代码` 解析：取 `.` 前缀数字部分

- [ ] **Step 1: Write failing tests**

```go
package flutter_api

import (
	"testing"
	"time"
)

func TestShouldRefreshSelectionClose(t *testing.T) {
	loc := chinaLocation()
	cases := []struct {
		name, updatedAt string
		now             time.Time
		want            bool
	}{
		{"周一15:05且未刷新", "", time.Date(2026, 8, 10, 15, 5, 0, 0, loc), true},
		{"周一15:04太早", "", time.Date(2026, 8, 10, 15, 4, 0, 0, loc), false},
		{"已刷新跳过", "2026-08-10T15:06:00+08:00", time.Date(2026, 8, 10, 15, 30, 0, 0, loc), false},
		{"周六不刷", "", time.Date(2026, 8, 15, 15, 10, 0, 0, loc), false},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := shouldRefreshSelectionClose(c.now, c.updatedAt); got != c.want {
				t.Fatalf("got %v want %v", got, c.want)
			}
		})
	}
}

func TestPatchSelectionCloseRets(t *testing.T) {
	results := []T0SelectionResult{
		{StockCode: "000537.XSHE", StockName: "绿发电力", CloseRet: 5.53, PrevClose: 8.32},
		{StockCode: "002194.XSHE", StockName: "武汉凡谷", CloseRet: 2.6, PrevClose: 10},
		{StockCode: "605179.XSHG", StockName: "一鸣食品", CloseRet: 9.67, PrevClose: 25.12},
	}
	quotes := map[string]t0Realtime{
		"000537": {Close: 8.77, PrevClose: 8.32},
		"002194": {Close: 0, PrevClose: 10}, // 价格为0 → 保留旧值
		// 605179 缺失 → 保留旧值
	}
	updated, kept := patchSelectionCloseRets(results, quotes)
	if updated != 1 || kept != 2 {
		t.Fatalf("updated=%d kept=%d", updated, kept)
	}
	if results[0].CloseRet != round2((8.77-8.32)/8.32*100) {
		t.Fatalf("000537 CloseRet=%v", results[0].CloseRet)
	}
	if results[1].CloseRet != 2.6 || results[2].CloseRet != 9.67 {
		t.Fatalf("kept values mutated: %v %v", results[1].CloseRet, results[2].CloseRet)
	}
}
```

- [ ] **Step 2: Run tests to verify they fail**

Run: `cd /Users/Zhuanz/aiproject/go-stock && go test ./backend/flutter_api/ -run 'TestShouldRefreshSelectionClose|TestPatchSelectionCloseRets' -count=1`

Expected: FAIL（函数未定义）

- [ ] **Step 3: Implement**

扩展归档结构：

```go
type t0SelectionArchive struct {
	Date            string              `json:"date"`
	SavedAt         string              `json:"saved_at"`
	CloseUpdatedAt  string              `json:"close_updated_at,omitempty"`
	Count           int                 `json:"count"`
	Results         []T0SelectionResult `json:"results"`
}
```

实现：

```go
func t0ShortCodeFromResultCode(stockCode string) string {
	if i := strings.IndexByte(stockCode, '.'); i > 0 {
		return stockCode[:i]
	}
	return stockCode
}

func shouldRefreshSelectionClose(now time.Time, closeUpdatedAt string) bool {
	if strings.TrimSpace(closeUpdatedAt) != "" {
		return false
	}
	local := now.In(chinaLocation())
	switch local.Weekday() {
	case time.Saturday, time.Sunday:
		return false
	}
	return local.Hour()*60+local.Minute() >= 15*60+5
}

func patchSelectionCloseRets(results []T0SelectionResult, quotes map[string]t0Realtime) (updated, kept int) {
	for i := range results {
		sc := t0ShortCodeFromResultCode(results[i].StockCode)
		rt, ok := quotes[sc]
		if !ok || rt.Close <= 0 {
			kept++
			continue
		}
		prev := rt.PrevClose
		if prev <= 0 {
			prev = results[i].PrevClose
		}
		if prev <= 0 {
			kept++
			continue
		}
		results[i].CloseRet = round2((rt.Close - prev) / prev * 100)
		updated++
	}
	return updated, kept
}
```

- [ ] **Step 4: Run tests to verify they pass**

Run: `cd /Users/Zhuanz/aiproject/go-stock && go test ./backend/flutter_api/ -run 'TestShouldRefreshSelectionClose|TestPatchSelectionCloseRets' -count=1`

Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add backend/flutter_api/t0_selection.go backend/flutter_api/t0_close_refresh_test.go
git commit -m "feat(t0): add close-ret patch helpers for selection archive"
```

---

### Task 5: `refreshSelectionCloseRet` + handler + ticker

**Files:**
- Modify: `backend/flutter_api/t0_selection.go`（`handleT0Selection`、新增 refresh 函数）
- Modify: `backend/flutter_api/server.go`（在主动预热 goroutine 旁增加收盘刷新 ticker）

**Interfaces:**
- Consumes: `loadT0SelectionArchive`, `saveT0SelectionArchive(..., force=true)`, `fetchT0Realtime`, `patchSelectionCloseRets`, `shouldRefreshSelectionClose`
- Produces: `func refreshSelectionCloseRet(tradeDate string, force bool) (map[string]any, error)`
- HTTP: `GET /api/t0-selection?refresh_close=1[&date=YYYY-MM-DD]`

- [ ] **Step 1: Implement `refreshSelectionCloseRet`**

```go
func resultsToT0Stocks(results []T0SelectionResult) []t0Stock {
	out := make([]t0Stock, 0, len(results))
	for _, r := range results {
		sc := t0ShortCodeFromResultCode(r.StockCode)
		prefix := "sz."
		if strings.HasSuffix(r.StockCode, ".XSHG") {
			prefix = "sh."
		}
		out = append(out, t0Stock{Code: prefix + sc, ShortCode: sc, Name: r.StockName})
	}
	return out
}

func refreshSelectionCloseRet(tradeDate string, force bool) (map[string]any, error) {
	a, ok := loadT0SelectionArchive(tradeDate)
	if !ok || a == nil {
		return nil, fmt.Errorf("该日无选股归档")
	}
	if !force && !shouldRefreshSelectionClose(time.Now(), a.CloseUpdatedAt) {
		return map[string]any{
			"date":              a.Date,
			"skipped":           true,
			"reason":            "not_due_or_already_updated",
			"close_updated_at":  a.CloseUpdatedAt,
			"count":             a.Count,
		}, nil
	}
	quotes := fetchT0Realtime(resultsToT0Stocks(a.Results))
	updated, kept := patchSelectionCloseRets(a.Results, quotes)
	now := time.Now().In(chinaLocation()).Format(time.RFC3339)
	a.SavedAt = now
	a.CloseUpdatedAt = now
	a.Count = len(a.Results)
	if err := saveT0SelectionArchive(tradeDate, a.Results, true); err != nil {
		return nil, err
	}
	// saveT0SelectionArchive 当前只写 Date/SavedAt/Count/Results —— 必须扩展它写入 CloseUpdatedAt
	return map[string]any{
		"date":             tradeDate,
		"count":            a.Count,
		"updated":          updated,
		"kept":             kept,
		"close_updated_at": now,
		"results":          a.Results,
	}, nil
}
```

**重要：** 同步改 `saveT0SelectionArchive`，使其接受并持久化 `CloseUpdatedAt`。最小改法：把函数签名改为写入完整 archive，或增加参数 `closeUpdatedAt string`。推荐改为：

```go
func saveT0SelectionArchiveFull(a *t0SelectionArchive, force bool) error {
	// 与现有原子写相同；force=false 且文件存在则跳过
	// payload 使用 *a，包含 CloseUpdatedAt
}
```

并让旧的 `saveT0SelectionArchive(tradeDate, results, force)` 内部构造 archive（`CloseUpdatedAt` 留空）调用之，避免破坏现有调用点。

`refreshSelectionCloseRet` 必须走 `saveT0SelectionArchiveFull`，保证 `close_updated_at` 落盘。

- [ ] **Step 2: Wire handler**

在 `handleT0Selection` 里，`archived` 分支之后、`prewarm` 之前加入：

```go
if isTruthyQuery(q.Get("refresh_close")) {
	out, err := refreshSelectionCloseRet(tradeDate, true) // 手动强制
	if err != nil {
		WriteJSON(w, map[string]any{"error": err.Error(), "date": tradeDate})
		return
	}
	WriteJSON(w, out)
	return
}
```

`archived` 响应顺带返回 `close_updated_at`（若有）。

- [ ] **Step 3: Wire ticker in `server.go`**

在主动预热 goroutine 旁：

```go
go func() {
	runT0CloseRefreshTick(time.Now())
	ticker := time.NewTicker(60 * time.Second)
	defer ticker.Stop()
	for tick := range ticker.C {
		runT0CloseRefreshTick(tick)
	}
}()
```

在 `t0_selection.go`：

```go
func runT0CloseRefreshTick(now time.Time) {
	tradeDate := now.In(chinaLocation()).Format("2006-01-02")
	a, ok := loadT0SelectionArchive(tradeDate)
	if !ok || a == nil {
		return
	}
	if !shouldRefreshSelectionClose(now, a.CloseUpdatedAt) {
		return
	}
	out, err := refreshSelectionCloseRet(tradeDate, false)
	if err != nil {
		logger.SugaredLogger.Warnf("[定时任务] T0归档收盘涨幅刷新失败 %s: %v", tradeDate, err)
		return
	}
	if skipped, _ := out["skipped"].(bool); skipped {
		return
	}
	logger.SugaredLogger.Infof("[定时任务] T0归档收盘涨幅已刷新: %s updated=%v kept=%v",
		tradeDate, out["updated"], out["kept"])
}
```

- [ ] **Step 4: Compile / unit tests**

Run: `cd /Users/Zhuanz/aiproject/go-stock && go test ./backend/flutter_api/ -run 'TestPickPrevDayTag|TestPrevDayRetsFromHist|TestShouldRefreshSelectionClose|TestPatchSelectionCloseRets' -count=1 && go build -o /tmp/go-stock-server-check ./cmd/server`

Expected: PASS + build OK

- [ ] **Step 5: Commit**

```bash
git add backend/flutter_api/t0_selection.go backend/flutter_api/server.go
git commit -m "feat(t0): refresh selection close ret after 15:05 and via refresh_close"
```

---

### Task 6: 手动修正 2026-08-11 归档并抽检

**Files:**
- Modify（运行时数据）: `backend/data/cache/t0/selection/t0_selection_2026-08-11.json`

**Interfaces:**
- Consumes: 运行中的服务 `:8080` + `refresh_close=1`

- [ ] **Step 1: 确认服务在跑且含新代码**

若仍是旧进程，用 `./scripts/ensure-flutter-api.sh` 前先停掉旧 `:8080`，再启动新 `go run ./cmd/server`。

- [ ] **Step 2: 强制刷新 8 月 11 日**

```bash
curl -s --max-time 60 'http://127.0.0.1:8080/api/t0-selection?date=2026-08-11&refresh_close=1' | python3 -m json.tool | head -40
```

Expected: `updated` 接近 30，`kept` 很少；`close_updated_at` 有值；`results` 名单与刷新前代码集合一致。

- [ ] **Step 3: 与腾讯抽检**

用归档中的代码拉腾讯现价，对比 `T0收盘涨幅(%)`，抽至少 5 只，要求 `|Δ| < 0.15`（舍入误差）。重点复查此前偏差大的：绿发电力、武汉凡谷、盛达资源。

- [ ] **Step 4: Commit 数据文件（若仓库跟踪该 JSON）**

```bash
git add backend/data/cache/t0/selection/t0_selection_2026-08-11.json
git commit -m "data(t0): refresh 2026-08-11 selection close returns after market close"
```

若该路径被 gitignore，则跳过 commit，只保留磁盘文件。

---

## Spec Coverage Self-Review

| Spec 要求 | Task |
|-----------|------|
| `标记` 字段 + 判定/唯一化 | Task 1–2 |
| 写入选股 JSON | Task 2（随归档） |
| Flutter 红/绿展示 | Task 3 |
| 15:05 只刷 CloseRet + 幂等 `close_updated_at` | Task 4–5 |
| 行情缺失保留旧值 | Task 4 |
| 手动 `refresh_close` | Task 5–6 |
| 不刷 gob / 不重跑选股 | Task 5–6 约束 |
| ST 同口径 | Global Constraints + Task 1 阈值 |
| 修 2026-08-11 | Task 6 |

## Placeholder Scan

无 TBD / “类似 Task N” / 空测试步骤。

## Type Consistency

- `pickPrevDayTag` / `prevDayRetsFromHist` / `patchSelectionCloseRets` / `shouldRefreshSelectionClose` / `refreshSelectionCloseRet` 命名在各 Task 一致
- JSON 键统一为 `标记`、`close_updated_at`
