# 主板 T0 近 7 日涨停记忆纳入破板 Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** 过滤 2 把近 7 日「收盘涨停 ≥ 9.89%」或「涨停破板」都算涨停记忆，并让 `涨停日期` 与入选判定共用同一套纯函数。

**Architecture:** 在 `t0_selection.go` 增加窗口切片与单日判定纯函数；`filterLimitUpRecent` 只做布尔扫描；两处结果组装用同一函数填 `涨停日期`。不新拉行情、不扫全历史。`pickPrevDayTag` 不动。

**Tech Stack:** Go 1.x，`backend/flutter_api` 包内单测（`go test`），现有 `dailyBar` 内存日线。

## Global Constraints

- 收盘涨停：`closeRet ≥ 9.89`；破板入选：`highRet ≥ 9.85` 且 `closeRet < 9.85`（破板门槛写死在纯函数内，不经参数传入）
- 破板后收跌停仍计入涨停记忆
- 夹缝 `9.85 ≤ closeRet < 9.89` 且不满足破板 → 该日两边都不算
- 窗口：`hist` 末尾最多 `days+1` 根（7 日 + 最早日前收）；不足则有几段算几段；不额外 HTTP / K 线
- 过滤阶段禁止拼 `涨停日期` 字符串、禁止为破板再扫一遍全市场
- `涨停日期`：命中日时间升序，取最近 3 个逗号拼接；没有则 `-`；破板与封板混排、无后缀
- 保留 `filterLimitUpRecent(stocks, cache, days, threshold)` 签名；调用处 `days=7`、`threshold=9.89`
- 不改 `pickPrevDayTag`、市值/成交额/开盘涨幅、MA20 暂缓、Flutter、Python 脚本、旧归档回填
- 不改 `backend/flutter_api/stock_selection_handler.go` 里同花顺 9.8% 语句

## File Structure

| 文件 | 职责 |
|------|------|
| `backend/flutter_api/t0_selection.go` | 常量、纯函数、过滤 2、两处 `涨停日期` 组装、文件头注释、四处调用门槛 |
| `backend/flutter_api/t0_limit_up_memory_test.go` | 纯函数与过滤 2 单测（不打网络） |
| `backend/flutter_api/t0_prevday_tag_test.go` | 不改；回归确认标记仍为 9.85 |

---

### Task 1: 涨停记忆纯函数 + 单测

**Files:**
- Create: `backend/flutter_api/t0_limit_up_memory_test.go`
- Modify: `backend/flutter_api/t0_selection.go`（`const` 块增加常量；在「过滤层」注释之前插入纯函数）

**Interfaces:**
- Produces:
  - `const t0LimitUpCloseRet = 9.89`
  - `const t0BrokenLimitRet = 9.85`
  - `const t0LimitUpMemoryDays = 7`
  - `func isLimitUpMemoryDay(prevClose float64, bar dailyBar, closeThreshold float64) bool`
  - `func limitUpMemoryTail(hist []dailyBar, days int) []dailyBar`
  - `func hasLimitUpMemory(hist []dailyBar, days int, closeThreshold float64) bool`
  - `func collectLimitUpMemoryDates(hist []dailyBar, days int, closeThreshold float64) []string`
  - `func formatLimitUpDates(dates []string) string`
- Consumes: 现有 `dailyBar`（`Date/Open/Close/High/Low/Volume/AmountYi`）

- [ ] **Step 1: Write the failing test**

创建 `backend/flutter_api/t0_limit_up_memory_test.go`：

```go
package flutter_api

import (
	"reflect"
	"testing"
)

func memBar(date string, high, close float64) dailyBar {
	return dailyBar{Date: date, High: high, Close: close}
}

func TestIsLimitUpMemoryDay(t *testing.T) {
	const p = 10.0
	cases := []struct {
		name string
		bar  dailyBar
		want bool
	}{
		{"收盘刚好9.89", memBar("d", 10.989, 10.989), true},
		{"收盘9.88不算涨停也不算破板", memBar("d", 10.988, 10.988), false},
		{"破板最高10收盘5", memBar("d", 11.0, 10.5), true},
		{"最高刚好9.85收盘更低", memBar("d", 10.985, 10.98), true},
		{"封死涨停走收盘门槛", memBar("d", 11.0, 11.0), true},
		{"夹缝收盘9.86", memBar("d", 10.986, 10.986), false},
		{"冲涨停后收跌停", memBar("d", 11.0, 9.01), true},
		{"最高0仍可靠收盘9.89", memBar("d", 0, 10.989), true},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got := isLimitUpMemoryDay(p, c.bar, t0LimitUpCloseRet)
			if got != c.want {
				t.Fatalf("got %v want %v", got, c.want)
			}
		})
	}
	if isLimitUpMemoryDay(0, memBar("d", 11, 11), t0LimitUpCloseRet) {
		t.Fatal("prevClose==0 must be false")
	}
}

func TestLimitUpMemoryWindowAndDates(t *testing.T) {
	// 8 根：仅最早候选日（bars[1] vs bars[0]）收盘涨停。旧过滤切 7 根会漏掉这一天。
	hist := []dailyBar{
		memBar("2026-08-10", 10, 10),
		memBar("2026-08-11", 10.989, 10.989),
		memBar("2026-08-12", 11, 11),
		memBar("2026-08-13", 11, 11),
		memBar("2026-08-14", 11, 11),
		memBar("2026-08-17", 11, 11),
		memBar("2026-08-18", 11, 11),
		memBar("2026-08-19", 11, 11),
	}
	if !hasLimitUpMemory(hist, 7, t0LimitUpCloseRet) {
		t.Fatal("oldest of 7 days sealed limit-up must count")
	}
	got := collectLimitUpMemoryDates(hist, 7, t0LimitUpCloseRet)
	want := []string{"2026-08-11"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("dates=%v want %v", got, want)
	}

	// 只有破板、没收盘涨停
	brokenOnly := []dailyBar{
		memBar("2026-08-18", 10, 10),
		memBar("2026-08-19", 11.0, 10.5),
	}
	if !hasLimitUpMemory(brokenOnly, 7, t0LimitUpCloseRet) {
		t.Fatal("broken limit-up must pass")
	}
	if got := collectLimitUpMemoryDates(brokenOnly, 7, t0LimitUpCloseRet); !reflect.DeepEqual(got, []string{"2026-08-19"}) {
		t.Fatalf("broken dates=%v", got)
	}

	// 夹缝不得因这一天命中
	gap := []dailyBar{
		memBar("2026-08-18", 10, 10),
		memBar("2026-08-19", 10.986, 10.986),
	}
	if hasLimitUpMemory(gap, 7, t0LimitUpCloseRet) {
		t.Fatal("gap 9.86 must not count")
	}

	if formatLimitUpDates(nil) != "-" {
		t.Fatal("empty dates must be -")
	}
	four := []string{"a", "b", "c", "d"}
	if got := formatLimitUpDates(four); got != "b, c, d" {
		t.Fatalf("last3=%q", got)
	}
}

func TestHasLimitUpMemoryAgreesWithCollect(t *testing.T) {
	hists := [][]dailyBar{
		{memBar("d0", 10, 10), memBar("d1", 11, 10.5)},
		{memBar("d0", 10, 10), memBar("d1", 10.986, 10.986)},
		{memBar("d0", 10, 10), memBar("d1", 10.989, 10.989)},
		nil,
		{memBar("d0", 10, 10)},
	}
	for i, h := range hists {
		has := hasLimitUpMemory(h, 7, t0LimitUpCloseRet)
		n := len(collectLimitUpMemoryDates(h, 7, t0LimitUpCloseRet))
		if has != (n > 0) {
			t.Fatalf("case %d has=%v n=%d", i, has, n)
		}
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run:

```bash
cd /Users/vb/Projects/go-stock && go test ./backend/flutter_api/ -run 'TestIsLimitUpMemoryDay|TestLimitUpMemoryWindowAndDates|TestHasLimitUpMemoryAgreesWithCollect' -count=1
```

Expected: FAIL（`isLimitUpMemoryDay` / `hasLimitUpMemory` 等 undefined）

- [ ] **Step 3: Write minimal implementation**

在 `backend/flutter_api/t0_selection.go` 的 `const (` 块中，`t0MaxMarketCapYi` 后面追加：

```go
	t0LimitUpCloseRet   = 9.89
	t0BrokenLimitRet    = 9.85
	t0LimitUpMemoryDays = 7
```

在 `// ── 过滤层 ──` 注释**之前**插入：

```go
func isLimitUpMemoryDay(prevClose float64, bar dailyBar, closeThreshold float64) bool {
	if prevClose == 0 {
		return false
	}
	closeRet := (bar.Close - prevClose) / prevClose * 100
	highRet := (bar.High - prevClose) / prevClose * 100
	if closeRet >= closeThreshold {
		return true
	}
	return highRet >= t0BrokenLimitRet && closeRet < t0BrokenLimitRet
}

func limitUpMemoryTail(hist []dailyBar, days int) []dailyBar {
	if days < 1 || len(hist) < 2 {
		return nil
	}
	need := days + 1
	if len(hist) <= need {
		return hist
	}
	return hist[len(hist)-need:]
}

func hasLimitUpMemory(hist []dailyBar, days int, closeThreshold float64) bool {
	tail := limitUpMemoryTail(hist, days)
	for i := 1; i < len(tail); i++ {
		if isLimitUpMemoryDay(tail[i-1].Close, tail[i], closeThreshold) {
			return true
		}
	}
	return false
}

func collectLimitUpMemoryDates(hist []dailyBar, days int, closeThreshold float64) []string {
	tail := limitUpMemoryTail(hist, days)
	var dates []string
	for i := 1; i < len(tail); i++ {
		if isLimitUpMemoryDay(tail[i-1].Close, tail[i], closeThreshold) {
			dates = append(dates, tail[i].Date)
		}
	}
	return dates
}

func formatLimitUpDates(dates []string) string {
	if len(dates) == 0 {
		return "-"
	}
	start := 0
	if len(dates) > 3 {
		start = len(dates) - 3
	}
	return strings.Join(dates[start:], ", ")
}
```

`strings` 已在该文件 import 中。不要在 `hasLimitUpMemory` 里调用 `formatLimitUpDates` 或拼接展示字符串。

- [ ] **Step 4: Run test to verify it passes**

Run:

```bash
cd /Users/vb/Projects/go-stock && go test ./backend/flutter_api/ -run 'TestIsLimitUpMemoryDay|TestLimitUpMemoryWindowAndDates|TestHasLimitUpMemoryAgreesWithCollect' -count=1
```

Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add backend/flutter_api/t0_limit_up_memory_test.go backend/flutter_api/t0_selection.go
git commit -m "$(cat <<'EOF'
feat(t0): 抽出近7日涨停记忆纯函数（含破板）

EOF
)"
```

---

### Task 2: 接入过滤 2 与两处 `涨停日期`

**Files:**
- Modify: `backend/flutter_api/t0_selection.go`
  - 文件头过滤链第 2 条（约第 32 行）
  - `filterLimitUpRecent` 函数体（约第 1255–1283 行）
  - `assembleT0CandidateResults` 中手写 `ret >= 9.8` 循环（约第 624–645 行）
  - `RunT0Selection` 组装中同样循环（约第 1518–1539 行）
  - 四处 `filterLimitUpRecent(..., 7, 9.8)`：约第 521、554、604、1458 行

**Interfaces:**
- Consumes: Task 1 的 `hasLimitUpMemory`、`collectLimitUpMemoryDates`、`formatLimitUpDates`、`t0LimitUpMemoryDays`、`t0LimitUpCloseRet`
- Produces: 预热 / 候选 / 正式选股过滤 2 与 `涨停日期` 口径一致

- [ ] **Step 1: Write a failing filter test**

在 `backend/flutter_api/t0_limit_up_memory_test.go` 追加：

```go
func TestFilterLimitUpRecentUsesMemoryRules(t *testing.T) {
	makeStock := func(code string) t0Stock {
		return t0Stock{Code: "sz." + code, ShortCode: code, Name: code}
	}
	cache := map[string][]dailyBar{
		"000001": {memBar("2026-08-18", 10, 10), memBar("2026-08-19", 11.0, 10.5)},
		"000002": {memBar("2026-08-18", 10, 10), memBar("2026-08-19", 10.986, 10.986)},
		"000003": {memBar("2026-08-18", 10, 10), memBar("2026-08-19", 10.989, 10.989)},
	}
	in := []t0Stock{makeStock("000001"), makeStock("000002"), makeStock("000003")}
	got := filterLimitUpRecent(in, cache, t0LimitUpMemoryDays, t0LimitUpCloseRet)
	if len(got) != 2 {
		t.Fatalf("got %d stocks, want 2 (broken + sealed)", len(got))
	}
	names := map[string]bool{}
	for _, s := range got {
		names[s.ShortCode] = true
	}
	if !names["000001"] || !names["000003"] || names["000002"] {
		t.Fatalf("unexpected set: %+v", names)
	}
}
```

- [ ] **Step 2: Run test to verify it fails（或仍按旧 9.8 收盘逻辑误过）**

Run:

```bash
cd /Users/vb/Projects/go-stock && go test ./backend/flutter_api/ -run TestFilterLimitUpRecentUsesMemoryRules -count=1
```

Expected: FAIL。在接入前，`000001` 破板收盘 5% 不会过过滤；若意外 PASS，说明过滤已接好，仍继续 Step 3 核对源码无 `ret >= 9.8`。

- [ ] **Step 3: Wire filter, call sites, and date formatting**

把文件头过滤链第 2 条改成：

```go
//  2. 近 7 日有涨停记忆（收盘涨幅 ≥ 9.89%，或最高 ≥ 9.85% 且收盘 < 9.85% 的涨停破板）
```

`filterLimitUpRecent` 整函数替换为：

```go
// filterLimitUpRecent 过滤2：近 N 日有涨停记忆（收盘涨幅 ≥ threshold%，或涨停破板）
func filterLimitUpRecent(stocks []t0Stock, cache map[string][]dailyBar, days int, threshold float64) []t0Stock {
	logger.SugaredLogger.Infof("[T0选股] 过滤2(涨停记忆): %d只 -> 检查近%d日收盘≥%.2f%%或涨停破板", len(stocks), days, threshold)

	var result []t0Stock
	for _, s := range stocks {
		if hasLimitUpMemory(cache[s.ShortCode], days, threshold) {
			result = append(result, s)
		}
	}
	logger.SugaredLogger.Infof("[T0选股] 过滤2通过: %d只", len(result))
	return result
}
```

四处调用全部改为：

```go
filterLimitUpRecent(stocks, hist, t0LimitUpMemoryDays, t0LimitUpCloseRet)
```

（第三处实参是 `cached.Stocks` / `hist`；第四处是 `allStocks` / `histCache`。只改后两个数字实参为常量，不要改切片变量名。）

`assembleT0CandidateResults` 里从 `var limitUpDates []string` 到赋 `limitUpInfo` 的整段，替换为：

```go
		limitUpInfo := formatLimitUpDates(collectLimitUpMemoryDates(hist, t0LimitUpMemoryDays, t0LimitUpCloseRet))
```

`RunT0Selection` 结果循环里同样从 `var limitUpDates []string` 到赋 `limitUpInfo` 的整段，替换为同一行（`hist` 变量名与现网一致）。

用搜索确认 `t0_selection.go` 中不再出现 `ret >= 9.8` 和 `filterLimitUpRecent(..., 7, 9.8)`。不要改 `pickPrevDayTag` 里的 `9.85`。

- [ ] **Step 4: Run tests to verify they pass**

Run:

```bash
cd /Users/vb/Projects/go-stock && go test ./backend/flutter_api/ -run 'TestIsLimitUpMemoryDay|TestLimitUpMemoryWindowAndDates|TestHasLimitUpMemoryAgreesWithCollect|TestFilterLimitUpRecentUsesMemoryRules|TestPickPrevDayTag' -count=1
```

Expected: PASS（含未改的 `TestPickPrevDayTag`）

再跑包内相关回归（无网络依赖的 T0 测试即可；若环境已能跑全包更好）：

```bash
cd /Users/vb/Projects/go-stock && go test ./backend/flutter_api/ -count=1
```

Expected: PASS。若个别测试因缺行情/缓存失败，至少保证上面 `-run` 列表 PASS，并记录失败名后再查是否与本改动有关。

- [ ] **Step 5: Commit**

```bash
git add backend/flutter_api/t0_selection.go backend/flutter_api/t0_limit_up_memory_test.go
git commit -m "$(cat <<'EOF'
feat(t0): 过滤2与涨停日期纳入近7日破板

EOF
)"
```

---

### Task 3: 抽检接入点无残留旧门槛

**Files:**
- Modify: 无（只读核对）。若 Step 1 搜到残留，回到 Task 2 修并补测。

**Interfaces:**
- Consumes: Task 2 改完的 `t0_selection.go`

- [ ] **Step 1: Search for leftover thresholds**

Run:

```bash
cd /Users/vb/Projects/go-stock && rg -n '9\.8|filterLimitUpRecent' backend/flutter_api/t0_selection.go
```

Expected:

- `filterLimitUpRecent` 定义与四处调用（实参为 `t0LimitUpMemoryDays` / `t0LimitUpCloseRet`）
- 文件头注释含 `9.89` 与破板 `9.85`
- `pickPrevDayTag` 仍使用 `9.85` / `-9.9`
- **没有** `ret >= 9.8`

`stock_selection_handler.go` 的同花顺 `9.8%` 语句必须原样保留。

- [ ] **Step 2: Optional local candidate smoke（有 gob 才做，禁止新拉全市场）**

若存在 `backend/data/cache/t0/daily/t0_daily_cache_*.gob`，可对某一历史日只读跑候选（用现有 HTTP 选股接口或已有测试夹具），抽检一只「昨日最高≥9.85% 且收盘<9.85%、近 7 日无收盘≥9.89%」的票是否出现在候选里，且 `涨停日期` 含那天。没有 gob 则跳过，单测已覆盖该规则。

不要写新的全市场爬取脚本。

- [ ] **Step 3: Commit only if Step 1 found leftover and you fixed it**

无代码改动则不空提交。有修复则：

```bash
git add backend/flutter_api/t0_selection.go
git commit -m "$(cat <<'EOF'
fix(t0): 去掉涨停记忆旧9.8门槛残留

EOF
)"
```

---

## Spec coverage

| Spec 项 | 任务 |
|---------|------|
| 收盘 ≥ 9.89 或破板过过滤 2 | Task 1 + Task 2 |
| 窗口最多 8 根、不新拉 K | Task 1 `limitUpMemoryTail` |
| 破板后跌停仍算 | Task 1 `冲涨停后收跌停` |
| 夹缝 9.86 不算 | Task 1 + Task 2 过滤测 |
| `涨停日期` 与过滤同一判定、最近 3 个、无后缀 | Task 1 `formatLimitUpDates` + Task 2 组装 |
| 过滤不拼日期字符串 | Task 1 `hasLimitUpMemory` |
| 调用处 9.89、破板写死 9.85 | Task 2 常量 / `t0BrokenLimitRet` |
| 保留 `filterLimitUpRecent` 签名 | Task 2 |
| 不改 `pickPrevDayTag` | Task 2 明确禁止；回归 `TestPickPrevDayTag` |
| 不改 Flutter / Python / 同花顺语句 / 旧归档 | 全局约束 + Task 3 |
| 性能红线 | `hasLimitUpMemory` 早退；只扫 tail |

## Execution notes

实现时不要改 `docs/superpowers/specs/` 里其它 T0 旧文的 9.8 叙述（历史文档）。本需求以 `2026-08-20-t0-limit-up-memory-broken-design.md` 为准。
