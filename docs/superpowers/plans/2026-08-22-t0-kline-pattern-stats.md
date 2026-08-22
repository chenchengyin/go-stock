# T0 K 线形态离线统计 Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** 在独立模块 `backend/analysis/candlepattern/` 中实现日 K 12 类分类、3/5 根序列统计、A2 动量池样本过滤，并通过 CLI 输出 JSON 报表；Phase 1.5 可选 3 日 hourly 交叉统计。全程只读现有 gob 缓存，不改动 T0 选股主链。

**Architecture:** 新建 `candlepattern` 包承载分类/池判定/序列/聚合逻辑；`cmd/t0-pattern-stats/` 作为唯一入口读取 gob、跑统计、写 `backend/data/cache/t0/pattern/`。A2 池逻辑从 `t0_selection.go` **复制**纯函数（带注释对齐行号），不 import `flutter_api` 运行时过滤链。gob 跨包解码用 `gob.RegisterName` 映射 `flutter_api.*` 类型名。

**Tech Stack:** Go 1.26、`encoding/gob`、`github.com/stretchr/testify`、现有 `go-stock/backend/data.EastMoneyKLineApi`（仅 Phase 1.5 hourly）。

## Global Constraints

- **禁止修改：** `backend/flutter_api/t0_selection.go`、`backend/flutter_api/server.go`、`trading_app/`、任何现有测试行为
- **样本池：** A2 动量池 = 过滤 1（gob `Stocks`）+ 过滤 2（近 7 日涨停记忆，收盘 ≥ 9.89% 或破板 high ≥ 9.89% & close < 9.85%）+ 过滤 3（前一交易日成交额 ≥ 5 亿）
- **单根分类：** 12 类，实体涨幅 = `(Close−Open)/PrevClose×100`；涨停 ≥ 9.89%、跌停 ≤ −9.9%、十字 < 0.5%、小 ±2.5%、中 ±6%
- **成功指标：** 下一日阳线率（Close > Open）；T0 子集浮盈 ≥ 2.5%（CloseRet−OpenGap，高开 0.01%～3%）
- **样本下限：** N < 30 不进入 Top 榜
- **hourly：** 默认关闭；`--hourly-days 3`；5 日扩展见独立 spec，本 plan 不实现
- **输出路径：** `backend/data/cache/t0/pattern/pattern_stats_{3,5}bar_YYYY-MM-DD.json`

## File Structure

| 文件 | 职责 |
|------|------|
| `backend/analysis/candlepattern/types.go` | `DailyBar`、`BarType`、`PatternStat`、`Observation` |
| `backend/analysis/candlepattern/classify.go` | 单根日 K 12 类分类 |
| `backend/analysis/candlepattern/classify_test.go` | 分类边界单测 |
| `backend/analysis/candlepattern/pool.go` | A2 池判定、hist 裁剪（对齐 `histBarsBeforeTradeDate`） |
| `backend/analysis/candlepattern/pool_test.go` | 池判定单测 |
| `backend/analysis/candlepattern/cache.go` | gob 只读加载、缓存根目录解析 |
| `backend/analysis/candlepattern/cache_test.go` | 真实 gob fixture 加载 |
| `backend/analysis/candlepattern/sequence.go` | 3/5 根序列字符串生成 |
| `backend/analysis/candlepattern/sequence_test.go` | 序列单测 |
| `backend/analysis/candlepattern/stats.go` | 观察样本收集 + 聚合 |
| `backend/analysis/candlepattern/stats_test.go` | 聚合单测 |
| `backend/analysis/candlepattern/report.go` | JSON 写入 + Top N 控制台摘要 |
| `backend/analysis/candlepattern/classify_hour.go` | hourly 6 类（Phase 1.5） |
| `backend/analysis/candlepattern/hourly_fetch.go` | 东财 60min 拉取 + 本地 JSON 缓存（Phase 1.5） |
| `backend/analysis/candlepattern/hourly_cross.go` | 日 K × hourly 末 2/3 根交叉统计（Phase 1.5） |
| `cmd/t0-pattern-stats/main.go` | CLI 入口 |

---

### Task 1: 类型定义与 gob 缓存只读加载

**Files:**
- Create: `backend/analysis/candlepattern/types.go`
- Create: `backend/analysis/candlepattern/cache.go`
- Create: `backend/analysis/candlepattern/cache_test.go`

**Interfaces:**
- Produces:
  - `type DailyBar struct { Date string; Open, Close, High, Low, Volume, AmountYi float64 }`
  - `type DailyCache struct { TradeDate string; Stocks []StockMeta; Daily map[string][]DailyBar }`
  - `type StockMeta struct { Code, ShortCode, Name string; MarketCapYi float64 }`
  - `func ResolveCacheRoot(startDir string) (string, error)`
  - `func LoadDailyCache(cacheRoot, tradeDate string) (*DailyCache, error)`
  - `func RegisterGobTypes()` — 在 `init` 或 Load 前调用
- Consumes: 磁盘 `backend/data/cache/t0/daily/t0_daily_cache_{date}.gob`

- [ ] **Step 1: Write the failing test**

`backend/analysis/candlepattern/cache_test.go`:

```go
package candlepattern

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLoadDailyCache_Aug21(t *testing.T) {
	root, err := ResolveCacheRoot(".")
	if err != nil {
		t.Skip("project root not found:", err)
	}
	path := filepath.Join(root, "t0", "daily", "t0_daily_cache_2026-08-21.gob")
	if _, err := os.Stat(path); err != nil {
		t.Skip("fixture gob missing:", path)
	}
	cache, err := LoadDailyCache(root, "2026-08-21")
	if err != nil {
		t.Fatal(err)
	}
	if cache.TradeDate != "2026-08-21" {
		t.Fatalf("tradeDate=%q", cache.TradeDate)
	}
	if len(cache.Stocks) < 1500 || len(cache.Stocks) > 2100 {
		t.Fatalf("stocks=%d want ~1887", len(cache.Stocks))
	}
	if len(cache.Daily) < 1500 {
		t.Fatalf("daily map too small: %d", len(cache.Daily))
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `cd backend && go test ./analysis/candlepattern/ -run TestLoadDailyCache_Aug21 -v`

Expected: FAIL — package or function not found

- [ ] **Step 3: Write minimal implementation**

`types.go` — 核心类型：

```go
package candlepattern

type BarType string

const (
	BarZT   BarType = "ZT"
	BarDT   BarType = "DT"
	BarPB   BarType = "PB"
	BarYX   BarType = "YX"
	BarYXN  BarType = "YXN"
	BarSY   BarType = "SY"
	BarXY   BarType = "XY"
	BarMY   BarType = "MY"
	BarMYIN BarType = "MYIN"
	BarDY   BarType = "DY"
	BarDYIN BarType = "DYIN"
	BarXX   BarType = "XX"
)

type DailyBar struct {
	Date     string
	Open     float64
	Close    float64
	High     float64
	Low      float64
	Volume   float64
	AmountYi float64
}

type StockMeta struct {
	Code        string
	ShortCode   string
	Name        string
	MarketCapYi float64
}

type DailyCache struct {
	TradeDate string
	Stocks    []StockMeta
	Daily     map[string][]DailyBar
}
```

`cache.go` — gob 跨包解码 + 根目录解析：

```go
package candlepattern

import (
	"encoding/gob"
	"fmt"
	"os"
	"path/filepath"
)

func init() { RegisterGobTypes() }

func RegisterGobTypes() {
	gob.RegisterName("flutter_api.dailyBar", DailyBar{})
	gob.RegisterName("flutter_api.t0Stock", StockMeta{})
	gob.RegisterName("flutter_api.t0DailyCachePayload", dailyCacheGob{})
}

type dailyCacheGob struct {
	TradeDate string
	Stocks    []StockMeta
	Daily     map[string][]DailyBar
}

func ResolveCacheRoot(startDir string) (string, error) {
	dir, err := filepath.Abs(startDir)
	if err != nil {
		return "", err
	}
	for {
		if _, err := os.Stat(filepath.Join(dir, "go.mod")); err == nil {
			if fi, err := os.Stat(filepath.Join(dir, "backend")); err == nil && fi.IsDir() {
				return filepath.Join(dir, "backend", "data", "cache"), nil
			}
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			return "", fmt.Errorf("project root not found from %s", startDir)
		}
		dir = parent
	}
}

func LoadDailyCache(cacheRoot, tradeDate string) (*DailyCache, error) {
	path := filepath.Join(cacheRoot, "t0", "daily", "t0_daily_cache_"+tradeDate+".gob")
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	var payload dailyCacheGob
	if err := gob.NewDecoder(f).Decode(&payload); err != nil {
		return nil, fmt.Errorf("decode gob %s: %w", path, err)
	}
	return &DailyCache{
		TradeDate: payload.TradeDate,
		Stocks:    payload.Stocks,
		Daily:     payload.Daily,
	}, nil
}
```

- [ ] **Step 4: Run test to verify it passes**

Run: `cd backend && go test ./analysis/candlepattern/ -run TestLoadDailyCache_Aug21 -v`

Expected: PASS（若 gob 类型名不匹配，调整 `RegisterName` 字符串直至 PASS）

- [ ] **Step 5: Commit**

```bash
git add backend/analysis/candlepattern/types.go backend/analysis/candlepattern/cache.go backend/analysis/candlepattern/cache_test.go
git commit -m "feat(candlepattern): add types and read-only gob cache loader"
```

---

### Task 2: 单根日 K 12 类分类

**Files:**
- Create: `backend/analysis/candlepattern/classify.go`
- Create: `backend/analysis/candlepattern/classify_test.go`

**Interfaces:**
- Produces: `func ClassifyDailyBar(prevClose float64, bar DailyBar) BarType`
- Consumes: `DailyBar`, `BarType` constants

- [ ] **Step 1: Write the failing test**

`classify_test.go`:

```go
package candlepattern

import "testing"

func bar(o, h, l, c float64) DailyBar {
	return DailyBar{Open: o, High: h, Low: l, Close: c}
}

func TestClassifyDailyBar(t *testing.T) {
	const p = 10.0
	cases := []struct {
		name string
		b    DailyBar
		want BarType
	}{
		{"涨停", bar(10, 11, 10, 10.989), BarZT},
		{"跌停", bar(10, 10, 9.01, 9.01), BarDT},
		{"破板", bar(10, 11, 10, 10.5), BarPB},
		{"大阳非涨停", bar(10, 10.8, 10, 10.7), BarDY},
		{"大阴非跌停", bar(10, 10, 9.2, 9.3), BarDYIN},
		{"阳十字", bar(10, 10.1, 9.9, 10.02), BarYX},
		{"阴十字", bar(10, 10.1, 9.9, 9.98), BarYXN},
		{"小阳", bar(10, 10.2, 10, 10.15), BarSY},
		{"小阴", bar(10, 10, 9.85, 9.85), BarXY},
		{"中阳", bar(10, 10.5, 10, 10.4), BarMY},
		{"中阴", bar(10, 10, 9.5, 9.5), BarMYIN},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got := ClassifyDailyBar(p, c.b)
			if got != c.want {
				t.Fatalf("got %s want %s", got, c.want)
			}
		})
	}
	if ClassifyDailyBar(0, bar(1, 1, 1, 1)) != BarXX {
		t.Fatal("prevClose=0 -> XX")
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `cd backend && go test ./analysis/candlepattern/ -run TestClassifyDailyBar -v`

Expected: FAIL

- [ ] **Step 3: Write minimal implementation**

`classify.go`:

```go
package candlepattern

const (
	limitUpCloseRet = 9.89
	brokenLimitRet  = 9.85
	limitDownRet    = -9.9
	dojiPct         = 0.5
	smallBodyPct    = 2.5
	mediumBodyPct   = 6.0
)

func pctChange(from, to float64) float64 {
	if from <= 0 {
		return 0
	}
	return (to - from) / from * 100
}

func bodyPct(prevClose, open, close float64) float64 {
	if prevClose <= 0 {
		return 0
	}
	return (close - open) / prevClose * 100
}

func ClassifyDailyBar(prevClose float64, bar DailyBar) BarType {
	if prevClose <= 0 {
		return BarXX
	}
	closeRet := pctChange(prevClose, bar.Close)
	highRet := pctChange(prevClose, bar.High)
	body := bodyPct(prevClose, bar.Open, bar.Close)
	absBody := body
	if absBody < 0 {
		absBody = -absBody
	}

	if closeRet >= limitUpCloseRet {
		return BarZT
	}
	if closeRet <= limitDownRet {
		return BarDT
	}
	if highRet >= limitUpCloseRet && closeRet < brokenLimitRet {
		return BarPB
	}
	if absBody < dojiPct {
		if bar.Close >= bar.Open {
			return BarYX
		}
		return BarYXN
	}
	if body >= dojiPct && body <= smallBodyPct {
		return BarSY
	}
	if body <= -dojiPct && body >= -smallBodyPct {
		return BarXY
	}
	if body > smallBodyPct && body <= mediumBodyPct {
		return BarMY
	}
	if body < -smallBodyPct && body >= -mediumBodyPct {
		return BarMYIN
	}
	if body > mediumBodyPct {
		return BarDY
	}
	if body < -mediumBodyPct {
		return BarDYIN
	}
	return BarXX
}
```

- [ ] **Step 4: Run test to verify it passes**

Run: `cd backend && go test ./analysis/candlepattern/ -run TestClassifyDailyBar -v`

Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add backend/analysis/candlepattern/classify.go backend/analysis/candlepattern/classify_test.go
git commit -m "feat(candlepattern): classify daily bars into 12 types"
```

---

### Task 3: A2 动量池判定（复制 t0_selection 逻辑）

**Files:**
- Create: `backend/analysis/candlepattern/pool.go`
- Create: `backend/analysis/candlepattern/pool_test.go`

**Interfaces:**
- Produces:
  - `func HistBeforeTradeDate(bars []DailyBar, tradeDate string) []DailyBar`
  - `func InA2Pool(hist []DailyBar) bool`
  - `func FilterA2Pool(cache *DailyCache, tradeDate string) map[string][]DailyBar` — 返回 shortCode → hist（已裁剪）
- Consumes: `DailyBar`, `DailyCache`
- 对齐注释：`t0_selection.go` `histBarsBeforeTradeDate` ~L1413、`hasLimitUpMemory` ~L1276、`filterTurnover` ~L1335

- [ ] **Step 1: Write the failing test**

`pool_test.go`:

```go
package candlepattern

import "testing"

func d(date string, vol, close float64) DailyBar {
	return DailyBar{Date: date, Volume: vol, Close: close, AmountYi: vol * close / 1e8}
}

func TestHistBeforeTradeDate(t *testing.T) {
	bars := []DailyBar{d("2026-08-20", 1, 10), d("2026-08-21", 1, 10)}
	got := HistBeforeTradeDate(bars, "2026-08-21")
	if len(got) != 1 || got[0].Date != "2026-08-20" {
		t.Fatalf("got %#v", got)
	}
}

func TestInA2Pool_LimitUpAndTurnover(t *testing.T) {
	// 7 日窗口内无涨停
	flat := make([]DailyBar, 8)
	for i := range flat {
		flat[i] = d("2026-08-10", 1e8, 10)
	}
	if InA2Pool(flat) {
		t.Fatal("flat should not pass")
	}
	// 昨日涨停 + 成交额 5 亿+
	hist := append([]DailyBar{}, flat...)
	hist[len(hist)-1] = DailyBar{Date: "2026-08-19", Open: 10, Close: 10.989, High: 10.989, Volume: 6e8, AmountYi: 6.59}
	if !InA2Pool(hist) {
		t.Fatal("limit up + turnover should pass")
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `cd backend && go test ./analysis/candlepattern/ -run 'TestHist|TestInA2' -v`

Expected: FAIL

- [ ] **Step 3: Write minimal implementation**

`pool.go` — 复制阈值与逻辑：

```go
package candlepattern

const (
	limitUpMemoryDays = 7
	minTurnoverYi     = 5.0
)

func HistBeforeTradeDate(bars []DailyBar, tradeDate string) []DailyBar {
	if len(bars) == 0 {
		return bars
	}
	if bars[len(bars)-1].Date == tradeDate {
		return bars[:len(bars)-1]
	}
	return bars
}

func isLimitUpMemoryDay(prevClose float64, bar DailyBar) bool {
	if prevClose <= 0 {
		return false
	}
	closeRet := pctChange(prevClose, bar.Close)
	highRet := pctChange(prevClose, bar.High)
	if closeRet >= limitUpCloseRet {
		return true
	}
	return highRet >= limitUpCloseRet && closeRet < brokenLimitRet
}

func hasLimitUpMemory(hist []DailyBar, days int) bool {
	if len(hist) < 2 {
		return false
	}
	tail := hist
	if len(tail) > days+1 {
		tail = tail[len(tail)-days-1:]
	}
	for i := 1; i < len(tail); i++ {
		if isLimitUpMemoryDay(tail[i-1].Close, tail[i]) {
			return true
		}
	}
	return false
}

func InA2Pool(hist []DailyBar) bool {
	if len(hist) == 0 {
		return false
	}
	if !hasLimitUpMemory(hist, limitUpMemoryDays) {
		return false
	}
	last := hist[len(hist)-1]
	return last.AmountYi >= minTurnoverYi
}

func FilterA2Pool(cache *DailyCache, tradeDate string) map[string][]DailyBar {
	out := make(map[string][]DailyBar)
	for _, s := range cache.Stocks {
		bars := cache.Daily[s.ShortCode]
		hist := HistBeforeTradeDate(bars, tradeDate)
		if InA2Pool(hist) {
			out[s.ShortCode] = hist
		}
	}
	return out
}
```

- [ ] **Step 4: Run test + 集成抽查 A2 数量**

Run: `cd backend && go test ./analysis/candlepattern/ -run 'TestHist|TestInA2' -v`

Expected: PASS

新增 `pool_test.go` 集成：

```go
func TestFilterA2Pool_Aug21_Magnitude(t *testing.T) {
	root, _ := ResolveCacheRoot(".")
	cache, err := LoadDailyCache(root, "2026-08-21")
	if err != nil {
		t.Skip(err)
	}
	pool := FilterA2Pool(cache, "2026-08-21")
	if len(pool) < 200 || len(pool) > 800 {
		t.Fatalf("A2 pool size=%d want ~几百", len(pool))
	}
}
```

Run: `cd backend && go test ./analysis/candlepattern/ -run TestFilterA2Pool_Aug21 -v`

Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add backend/analysis/candlepattern/pool.go backend/analysis/candlepattern/pool_test.go
git commit -m "feat(candlepattern): add A2 momentum pool filter mirroring T0 chain"
```

---

### Task 4: 3/5 根序列生成

**Files:**
- Create: `backend/analysis/candlepattern/sequence.go`
- Create: `backend/analysis/candlepattern/sequence_test.go`

**Interfaces:**
- Produces:
  - `func BuildPatternLabels(hist []DailyBar, window int) ([]BarType, bool)` — 取 hist 末 window 根（不含观察日）的分类序列
  - `func FormatPattern(types []BarType) string` — `XY|MYIN|SY`
- Consumes: `ClassifyDailyBar`, `BarType`

- [ ] **Step 1: Write the failing test**

```go
func TestBuildPatternLabels(t *testing.T) {
	hist := []DailyBar{
		{Date: "d1", Open: 10, Close: 9.85, High: 10, Low: 9.8},
		{Date: "d2", Open: 10, Close: 9.5, High: 10, Low: 9.5},
		{Date: "d3", Open: 10, Close: 10.15, High: 10.2, Low: 10},
	}
	// prev closes: d1 uses 10, d2 uses 9.85, d3 uses 9.5
	types, ok := BuildPatternLabels(hist, 3)
	if !ok {
		t.Fatal("expected ok")
	}
	if FormatPattern(types) == "" {
		t.Fatal("empty pattern")
	}
}
```

- [ ] **Step 2–4: Implement, test, verify PASS**

`sequence.go`:

```go
func BuildPatternLabels(hist []DailyBar, window int) ([]BarType, bool) {
	if len(hist) < window+1 {
		return nil, false
	}
	seg := hist[len(hist)-window:]
	out := make([]BarType, window)
	for i, bar := range seg {
		prevClose := hist[len(hist)-window-1+i].Close
		if i == 0 {
			prevClose = hist[len(hist)-window-1].Close
		}
		// 修正：第 i 根的前收是 seg 前一根或 hist 中前一项
		idx := len(hist) - window + i
		prevClose = hist[idx-1].Close
		out[i] = ClassifyDailyBar(prevClose, bar)
	}
	return out, true
}

func FormatPattern(types []BarType) string {
	if len(types) == 0 {
		return ""
	}
	s := string(types[0])
	for i := 1; i < len(types); i++ {
		s += "|" + string(types[i])
	}
	return s
}
```

Run: `cd backend && go test ./analysis/candlepattern/ -run TestBuildPatternLabels -v`

- [ ] **Step 5: Commit**

```bash
git add backend/analysis/candlepattern/sequence.go backend/analysis/candlepattern/sequence_test.go
git commit -m "feat(candlepattern): build 3/5-bar pattern label sequences"
```

---

### Task 5: 观察样本收集与聚合统计

**Files:**
- Create: `backend/analysis/candlepattern/stats.go`
- Create: `backend/analysis/candlepattern/stats_test.go`

**Interfaces:**
- Produces:
  - `type Observation struct { Pattern string; Window int; NextYang bool; T0Gap, T0PnL float64; InT0Subset bool }`
  - `type PatternStat struct { Pattern string; Window, SampleCount, T0SubsetCount int; NextYangRate, T0WinRate2p5, T0AvgPnL, T0MedianPnL float64 }`
  - `func CollectObservations(cache *DailyCache, tradeDate string, window int) []Observation`
  - `func AggregateStats(obs []Observation, window int, minSamples int) []PatternStat`
- Consumes: `FilterA2Pool`, `BuildPatternLabels`, 观察日 bar 从 `cache.Daily[sc]` 含 `tradeDate` 当日 K

- [ ] **Step 1: Write the failing test**

```go
func TestAggregateStats_MinSamples(t *testing.T) {
	obs := []Observation{
		{Pattern: "A|B|C", Window: 3, NextYang: true, InT0Subset: true, T0PnL: 3},
		{Pattern: "A|B|C", Window: 3, NextYang: false, InT0Subset: true, T0PnL: 1},
		{Pattern: "X|Y|Z", Window: 3, NextYang: true, InT0Subset: false, T0PnL: 0},
	}
	stats := AggregateStats(obs, 3, 30)
	if len(stats) != 0 {
		t.Fatalf("N<30 should produce no top stats, got %d", len(stats))
	}
	stats2 := AggregateStats(obs, 3, 2)
	if len(stats2) != 1 || stats2[0].Pattern != "A|B|C" {
		t.Fatalf("got %#v", stats2)
	}
}
```

- [ ] **Step 3: Implement CollectObservations + AggregateStats**

核心逻辑：

```go
const (
	t0MinGap = 0.01
	t0MaxGap = 3.0
	t0WinPnL = 2.5
)

func CollectObservations(cache *DailyCache, tradeDate string, window int) []Observation {
	pool := FilterA2Pool(cache, tradeDate)
	var out []Observation
	for sc, hist := range pool {
		bars := cache.Daily[sc]
		// 观察日 T 必须在 bars 末位
		if len(bars) == 0 || bars[len(bars)-1].Date != tradeDate {
			continue
		}
		labels, ok := BuildPatternLabels(hist, window)
		if !ok {
			continue
		}
		pattern := FormatPattern(labels)
		obsBar := bars[len(bars)-1]
		prevClose := hist[len(hist)-1].Close
		nextYang := obsBar.Close > obsBar.Open
		var gap, pnl float64
		inT0 := false
		if prevClose > 0 && obsBar.Open > 0 {
			gap = (obsBar.Open - prevClose) / prevClose * 100
			closeRet := (obsBar.Close - prevClose) / prevClose * 100
			pnl = closeRet - gap
			if gap >= t0MinGap && gap <= t0MaxGap {
				inT0 = true
			}
		}
		out = append(out, Observation{
			Pattern: pattern, Window: window, NextYang: nextYang,
			T0Gap: gap, T0PnL: pnl, InT0Subset: inT0,
		})
	}
	return out
}
```

`AggregateStats` 按 pattern 分组，算阳线率、T0 子集胜率 ≥2.5%、均值/中位数；`minSamples` 过滤。

- [ ] **Step 4: Run tests**

Run: `cd backend && go test ./analysis/candlepattern/ -run TestAggregate -v`

- [ ] **Step 5: Commit**

```bash
git add backend/analysis/candlepattern/stats.go backend/analysis/candlepattern/stats_test.go
git commit -m "feat(candlepattern): collect observations and aggregate pattern stats"
```

---

### Task 6: JSON 报表与 Top 摘要

**Files:**
- Create: `backend/analysis/candlepattern/report.go`

**Interfaces:**
- Produces:
  - `func PatternOutputDir(cacheRoot string) string` → `{cacheRoot}/t0/pattern`
  - `func WritePatternStats(cacheRoot, tradeDate string, stats []PatternStat, window int) error`
  - `func PrintTopSummary(stats []PatternStat, topN, minSamples int)`

- [ ] **Step 1–3: Implement + manual smoke**

```go
func WritePatternStats(cacheRoot, tradeDate string, stats []PatternStat, window int) error {
	dir := filepath.Join(cacheRoot, "t0", "pattern")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}
	name := fmt.Sprintf("pattern_stats_%dbar_%s.json", window, tradeDate)
	path := filepath.Join(dir, name)
	f, err := os.Create(path)
	if err != nil {
		return err
	}
	defer f.Close()
	enc := json.NewEncoder(f)
	enc.SetIndent("", "  ")
	return enc.Encode(stats)
}
```

- [ ] **Step 4: Commit**

```bash
git add backend/analysis/candlepattern/report.go
git commit -m "feat(candlepattern): write pattern stats JSON and console summary"
```

---

### Task 7: CLI 入口（Phase 1 日 K）

**Files:**
- Create: `cmd/t0-pattern-stats/main.go`

**Interfaces:**
- Consumes: `LoadDailyCache`, `CollectObservations`, `AggregateStats`, `WritePatternStats`, `PrintTopSummary`

- [ ] **Step 1: Implement CLI**

```go
package main

import (
	"flag"
	"fmt"
	"log"
	"os"

	"go-stock/backend/analysis/candlepattern"
)

func main() {
	date := flag.String("date", "", "trade date YYYY-MM-DD")
	window := flag.String("window", "all", "3, 5, or all")
	minSamples := flag.Int("min-samples", 30, "minimum sample count for top list")
	topN := flag.Int("top", 20, "console top N")
	flag.Parse()
	if *date == "" {
		log.Fatal("--date required")
	}
	root, err := candlepattern.ResolveCacheRoot(".")
	if err != nil {
		log.Fatal(err)
	}
	cache, err := candlepattern.LoadDailyCache(root, *date)
	if err != nil {
		log.Fatal(err)
	}
	windows := []int{3, 5}
	if *window == "3" {
		windows = []int{3}
	} else if *window == "5" {
		windows = []int{5}
	}
	for _, w := range windows {
		obs := candlepattern.CollectObservations(cache, *date, w)
		stats := candlepattern.AggregateStats(obs, w, *minSamples)
		if err := candlepattern.WritePatternStats(root, *date, stats, w); err != nil {
			log.Fatal(err)
		}
		fmt.Fprintf(os.Stderr, "window=%d observations=%d patterns=%d\n", w, len(obs), len(stats))
		candlepattern.PrintTopSummary(stats, *topN, *minSamples)
	}
}
```

- [ ] **Step 2: Smoke run**

Run from repo root:

```bash
go run ./cmd/t0-pattern-stats/ --date 2026-08-21 --window all
```

Expected:
- 写出 `backend/data/cache/t0/pattern/pattern_stats_3bar_2026-08-21.json`
- 写出 `backend/data/cache/t0/pattern/pattern_stats_5bar_2026-08-21.json`
- stderr 打印 A2 观察样本数百条、Top 20 摘要

- [ ] **Step 3: Commit**

```bash
git add cmd/t0-pattern-stats/main.go
git commit -m "feat: add t0-pattern-stats CLI for daily K pattern research"
```

---

### Task 8: Phase 1.5 — hourly 3 日交叉统计（可选 flag）

**Files:**
- Create: `backend/analysis/candlepattern/classify_hour.go`
- Create: `backend/analysis/candlepattern/hourly_fetch.go`
- Create: `backend/analysis/candlepattern/hourly_cross.go`
- Modify: `cmd/t0-pattern-stats/main.go`

**Interfaces:**
- Produces:
  - `func ClassifyHourBar(prevClose float64, open, close float64) string`
  - `func LoadOrFetchHourly3D(cacheRoot, shortCode, tradeDate string, fetcher HourlyFetcher) ([]HourBar, error)`
  - `func CrossStatsDailyHourly(cache *DailyCache, tradeDate string, window int, tailN int) []HourlyCrossStat`
- Consumes: `data.NewEastMoneyKLineApi().GetMinuteKLine(code, data.KLineType60Min, days)`

- [ ] **Step 1: hourly 6 类单测 + 实现**（阈值：十字 ≤0.3%，阳/阴 >0.3%）

- [ ] **Step 2: JSON 缓存** `backend/data/cache/t0/pattern/hourly/hourly_3d_{shortCode}_{tradeDate}.json`

- [ ] **Step 3: CLI 增加 flags**

```
--hourly           启用 hourly 交叉统计
--hourly-days 3    固定 3（本 plan 不实现 5）
--hourly-tail 2,3  末 N 根 hourly
```

- [ ] **Step 4: 输出** `hourly_cross_stats_3d_YYYY-MM-DD.json`；hourly 拉取失败跳过，不影响日 K 报表

- [ ] **Step 5: Commit**

```bash
git add backend/analysis/candlepattern/classify_hour.go backend/analysis/candlepattern/hourly_fetch.go backend/analysis/candlepattern/hourly_cross.go cmd/t0-pattern-stats/main.go
git commit -m "feat(candlepattern): optional 3-day hourly cross stats with local cache"
```

---

### Task 9: 全量回归（154 交易日可选）

**Files:**
- Modify: `cmd/t0-pattern-stats/main.go` — 增加 `--date-range 2026-01-05:2026-08-21`

- [ ] **Step 1: 实现 date-range 循环**（跳过无 gob 文件的日期）

- [ ] **Step 2: 跑通**

```bash
go run ./cmd/t0-pattern-stats/ --date-range 2026-08-01:2026-08-21 --window all
```

- [ ] **Step 3: Commit**

```bash
git commit -m "feat(t0-pattern-stats): support date-range batch runs"
```

---

## Spec Coverage Checklist

| Spec 要求 | Task |
|-----------|------|
| 12 类日 K 分类 | Task 2 |
| 3/5 根序列 | Task 4 |
| A2 动量池 | Task 3 |
| 阳线率 + T0 ≥2.5% | Task 5 |
| 只读 gob | Task 1 |
| JSON 输出 + Top 20 | Task 6–7 |
| hourly 3 日（可选） | Task 8 |
| 不改 t0_selection/server/flutter | Global Constraints |
| 5 日 hourly | 独立 spec，本 plan 不覆盖 |

## Self-Review Notes

- gob 跨包解码需在 Task 1 用真实 fixture 验证；若 `RegisterName` 失败，备选方案：在 `flutter_api` 新增 **只读** `t0_cache_export.go`（仍不碰 `t0_selection.go`）。
- `BuildPatternLabels` 前收索引在 Task 4 实现时以单测固定，避免 off-by-one。
- hourly 任务默认 `--hourly` 关闭，满足「不影响现有逻辑、不强制打网络」。

---

**Plan complete and saved to `docs/superpowers/plans/2026-08-22-t0-kline-pattern-stats.md`. Two execution options:**

**1. Subagent-Driven (recommended)** — 每个 Task 派独立 subagent，Task 间做 review，迭代快

**2. Inline Execution** — 本会话按 Task 顺序直接实现，checkpoint 处暂停给你看

**Which approach?**
