# T0 预热状态返回 + 选股结果归档 Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** 预热中接口立刻返回进度；正式选股在无日线且预热中时不阻塞；按日归档选股结果（默认只写一次，`save=1` 强制覆盖）；缓存统一落到 `/tmp/go-stock-cache/t0/`。

**Architecture:** 进程内按交易日维护 `idle|warming|ready|failed` 状态与拉日线进度；预热在后台 goroutine 执行；日线仍用 gob 原子写；选股结果用 JSON 归档。同一 `/api/t0-selection` 用 `prewarm` / `archived` / `save` 分流。

**Tech Stack:** Go、`encoding/gob`、`encoding/json`、标准库 `sync`/`atomic`/`os`；单测用 `testing` + 临时目录。

## Global Constraints

- 不新增独立 HTTP 路由，只用 `/api/t0-selection` 查询参数。
- 缓存根目录必须为 `/tmp/go-stock-cache/`，T0 子路径为 `t0/daily/` 与 `t0/selection/`。
- 不改选股过滤阈值；MA20 门闸保持暂缓。
- 不改 Flutter；不强制全量 `go build`（可用针对单测文件的 `go test`）。
- Spec：`docs/superpowers/specs/2026-08-05-t0-cache-status-and-selection-archive-design.md`。

## File Map

| File | Responsibility |
|------|----------------|
| `backend/flutter_api/t0_selection.go` | 路径常量、状态机、后台预热、归档读写、handler 分流 |
| `backend/flutter_api/t0_selection_cache_test.go` | 路径/归档写一次/状态机单测（用临时目录，不打外网） |

---

### Task 1: 缓存目录路径与日线路径切换

**Files:**
- Modify: `backend/flutter_api/t0_selection.go`（常量与 `t0DailyCachePath` / `saveT0DailyCache`）
- Test: `backend/flutter_api/t0_selection_cache_test.go`

**Interfaces:**
- Produces: `t0CacheRoot = "/tmp/go-stock-cache"`；可变 `t0CacheRootPath`；`t0DailyCachePath(date) string`；`t0SelectionCachePath(date) string`；`ensureT0CacheDirs() error`

- [ ] **Step 1: 写失败单测（路径）**

在 `backend/flutter_api/t0_selection_cache_test.go`：

```go
package flutter_api

import (
	"path/filepath"
	"strings"
	"testing"
)

func TestT0DailyCachePathUsesGoStockCacheDir(t *testing.T) {
	p := t0DailyCachePath("2026-08-05")
	if !strings.Contains(p, filepath.Join("go-stock-cache", "t0", "daily")) {
		t.Fatalf("unexpected daily path: %s", p)
	}
	if !strings.HasSuffix(p, "t0_daily_cache_2026-08-05.gob") {
		t.Fatalf("unexpected daily filename: %s", p)
	}
}

func TestT0SelectionCachePathUsesGoStockCacheDir(t *testing.T) {
	p := t0SelectionCachePath("2026-08-05")
	if !strings.Contains(p, filepath.Join("go-stock-cache", "t0", "selection")) {
		t.Fatalf("unexpected selection path: %s", p)
	}
	if !strings.HasSuffix(p, "t0_selection_2026-08-05.json") {
		t.Fatalf("unexpected selection filename: %s", p)
	}
}
```

- [ ] **Step 2: 跑测确认失败**

Run: `cd /Users/Zhuanz/aiproject/go-stock && go test ./backend/flutter_api/ -run 'TestT0DailyCachePath|TestT0SelectionCachePath' -count=1`

Expected: FAIL（函数未定义或仍指向 `/tmp/t0_daily_cache_`）

- [ ] **Step 3: 实现路径常量与 ensure**

替换 `t0_selection.go` 中相关常量：

```go
const (
	t0MinMarketCapYi = 50.0
	t0MaxMarketCapYi = 9000.0
	t0CacheRoot      = "/tmp/go-stock-cache"
)

var t0CacheRootPath = t0CacheRoot

func t0DailyCachePath(tradeDate string) string {
	return filepath.Join(t0CacheRootPath, "t0", "daily", "t0_daily_cache_"+tradeDate+".gob")
}

func t0SelectionCachePath(tradeDate string) string {
	return filepath.Join(t0CacheRootPath, "t0", "selection", "t0_selection_"+tradeDate+".json")
}

func ensureT0CacheDirs() error {
	for _, dir := range []string{
		filepath.Join(t0CacheRootPath, "t0", "daily"),
		filepath.Join(t0CacheRootPath, "t0", "selection"),
	} {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			return err
		}
	}
	return nil
}
```

在 `saveT0DailyCache` / `loadT0DailyCache` 开头调用 `ensureT0CacheDirs()`。删除旧的 `t0DailyCacheDir = "/tmp"`。

- [ ] **Step 4: 再跑路径单测**

Run: 同 Step 2  
Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add backend/flutter_api/t0_selection.go backend/flutter_api/t0_selection_cache_test.go
git commit -m "feat(t0): move cache files under /tmp/go-stock-cache/t0"
```

---

### Task 2: 选股结果归档（写一次 + save 强制覆盖）

**Files:**
- Modify: `backend/flutter_api/t0_selection.go`
- Test: `backend/flutter_api/t0_selection_cache_test.go`

**Interfaces:**
- Produces:
  - `type t0SelectionArchive struct { Date string; SavedAt string; Count int; Results []T0SelectionResult }`
  - `loadT0SelectionArchive(date string) (*t0SelectionArchive, bool)`
  - `saveT0SelectionArchive(date string, results []T0SelectionResult, force bool) error`  
    — `force=false` 且文件已存在则 no-op 返回 nil；`force=true` 覆盖

- [ ] **Step 1: 写失败单测（归档）**

```go
func TestSaveT0SelectionArchiveWriteOnceAndForce(t *testing.T) {
	orig := t0CacheRootPath
	t0CacheRootPath = t.TempDir()
	defer func() { t0CacheRootPath = orig }()

	date := "2026-08-05"
	first := []T0SelectionResult{{StockCode: "600000.XSHG", StockName: "浦发银行", AmountYi: 10}}
	if err := saveT0SelectionArchive(date, first, false); err != nil {
		t.Fatal(err)
	}
	second := []T0SelectionResult{{StockCode: "000001.XSHE", StockName: "平安银行", AmountYi: 20}}
	if err := saveT0SelectionArchive(date, second, false); err != nil {
		t.Fatal(err)
	}
	got, ok := loadT0SelectionArchive(date)
	if !ok || got.Count != 1 || got.Results[0].StockCode != "600000.XSHG" {
		t.Fatalf("expected first snapshot kept, got %+v ok=%v", got, ok)
	}
	if err := saveT0SelectionArchive(date, second, true); err != nil {
		t.Fatal(err)
	}
	got, ok = loadT0SelectionArchive(date)
	if !ok || got.Count != 1 || got.Results[0].StockCode != "000001.XSHE" {
		t.Fatalf("expected force overwrite, got %+v ok=%v", got, ok)
	}
}
```

- [ ] **Step 2: 跑测确认失败**

Run: `go test ./backend/flutter_api/ -run TestSaveT0SelectionArchiveWriteOnceAndForce -count=1`  
Expected: FAIL

- [ ] **Step 3: 实现 load/save**

```go
type t0SelectionArchive struct {
	Date    string              `json:"date"`
	SavedAt string              `json:"saved_at"`
	Count   int                 `json:"count"`
	Results []T0SelectionResult `json:"results"`
}

func saveT0SelectionArchive(tradeDate string, results []T0SelectionResult, force bool) error {
	if err := ensureT0CacheDirs(); err != nil {
		return err
	}
	path := t0SelectionCachePath(tradeDate)
	if !force {
		if _, err := os.Stat(path); err == nil {
			return nil
		}
	}
	payload := t0SelectionArchive{
		Date:    tradeDate,
		SavedAt: time.Now().Format(time.RFC3339),
		Count:   len(results),
		Results: results,
	}
	data, err := json.MarshalIndent(payload, "", "  ")
	if err != nil {
		return err
	}
	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, data, 0o644); err != nil {
		return err
	}
	return os.Rename(tmp, path)
}

func loadT0SelectionArchive(tradeDate string) (*t0SelectionArchive, bool) {
	data, err := os.ReadFile(t0SelectionCachePath(tradeDate))
	if err != nil {
		return nil, false
	}
	var a t0SelectionArchive
	if err := json.Unmarshal(data, &a); err != nil {
		return nil, false
	}
	return &a, true
}
```

- [ ] **Step 4: 跑测通过**

Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add backend/flutter_api/t0_selection.go backend/flutter_api/t0_selection_cache_test.go
git commit -m "feat(t0): archive daily selection results with write-once and force save"
```

---

### Task 3: 预热状态机（非阻塞 + 进度）

**Files:**
- Modify: `backend/flutter_api/t0_selection.go`
- Test: `backend/flutter_api/t0_selection_cache_test.go`

**Interfaces:**
- Produces:
  - `type t0WarmStatus string` 常量 `idle|warming|ready|failed`
  - `type t0WarmProgress struct { Status; StockCount; DailyFetched; DailyTotal; CandidateCount; Err string; StartedAt time.Time }`
  - `getT0WarmProgress(date string) t0WarmProgress`
  - `setT0WarmProgressForTest(date string, p t0WarmProgress)`（仅测试）
  - `tryStartT0Prewarm(date string) (started bool, progress t0WarmProgress)`
  - `isT0DailyCacheFilePresent(date string) bool`
  - `shouldReturnWarmingForSelection(date string) bool`
  - 改造 `fetchAllDailyKLine`：增加可选 `onDone func()`，每完成一只调用以更新进度

- [ ] **Step 1: 写状态机单测（无外网）**

```go
func TestTryStartT0PrewarmIdempotentWhenWarming(t *testing.T) {
	orig := t0CacheRootPath
	t0CacheRootPath = t.TempDir()
	defer func() { t0CacheRootPath = orig }()

	date := "2099-01-02"
	setT0WarmProgressForTest(date, t0WarmProgress{
		Status: t0WarmStatusWarming, DailyFetched: 10, DailyTotal: 100, StartedAt: time.Now(),
	})
	started, p := tryStartT0Prewarm(date)
	if started {
		t.Fatal("should not start second prewarm")
	}
	if p.Status != t0WarmStatusWarming || p.DailyFetched != 10 {
		t.Fatalf("unexpected progress: %+v", p)
	}
}

func TestSelectionBlockedWhenWarmingWithoutDailyFile(t *testing.T) {
	orig := t0CacheRootPath
	t0CacheRootPath = t.TempDir()
	defer func() { t0CacheRootPath = orig }()
	date := "2099-01-03"
	setT0WarmProgressForTest(date, t0WarmProgress{Status: t0WarmStatusWarming, StartedAt: time.Now()})
	if isT0DailyCacheFilePresent(date) {
		t.Fatal("file should not exist")
	}
	if !shouldReturnWarmingForSelection(date) {
		t.Fatal("selection should report warming")
	}
}
```

```go
func shouldReturnWarmingForSelection(tradeDate string) bool {
	return !isT0DailyCacheFilePresent(tradeDate) && getT0WarmProgress(tradeDate).Status == t0WarmStatusWarming
}
```

- [ ] **Step 2: 跑测确认失败**

Run: `go test ./backend/flutter_api/ -run 'TestTryStartT0Prewarm|TestSelectionBlockedWhenWarming' -count=1`  
Expected: FAIL

- [ ] **Step 3: 实现状态表与 tryStart / 后台预热**

```go
type t0WarmStatus string

const (
	t0WarmStatusIdle    t0WarmStatus = "idle"
	t0WarmStatusWarming t0WarmStatus = "warming"
	t0WarmStatusReady   t0WarmStatus = "ready"
	t0WarmStatusFailed  t0WarmStatus = "failed"
)

type t0WarmProgress struct {
	Status         t0WarmStatus
	StockCount     int
	DailyFetched   int
	DailyTotal     int
	CandidateCount int
	Err            string
	StartedAt      time.Time
}

var (
	t0WarmMu     sync.Mutex
	t0WarmByDate = map[string]*t0WarmProgress{}
)
```

行为：

1. `tryStartT0Prewarm`：持锁；若已 `warming` → `(false, copy)`；若日线文件存在 → 标 `ready` 并 `(false, copy)`；否则写入 `warming`、`go runT0PrewarmJob(date)`、`(true, copy)`。
2. `runT0PrewarmJob`：拉股票池 → 设 `DailyTotal` → `fetchAllDailyKLine` 带进度 → `saveT0DailyCache` → 涨停+成交额得 `CandidateCount` → `ready`；失败 → `failed` + `Err`。
3. 删除 `PrewarmT0Daily` / `RunT0Selection` 里对 `t0DailyDateMu` 的整段阻塞锁（可删掉 `t0DailyDateMu` 相关代码）。
4. Handler（Task 4）在调用 `RunT0Selection` 前用 `shouldReturnWarmingForSelection` 短路。

- [ ] **Step 4: 跑测通过**

Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add backend/flutter_api/t0_selection.go backend/flutter_api/t0_selection_cache_test.go
git commit -m "feat(t0): non-blocking prewarm status machine with progress"
```

---

### Task 4: Handler 分流（archived / prewarm / save / 选股）

**Files:**
- Modify: `backend/flutter_api/t0_selection.go` 中 `handleT0Selection`、`PrewarmT0Daily`（改为走 tryStart 或删除同步版）、`RunT0Selection`
- Test: `backend/flutter_api/t0_selection_cache_test.go`

**Interfaces:**
- Consumes: Task 2–3 全部函数
- Produces: `handleT0Selection` 行为对齐 spec

- [ ] **Step 1: 写 handler 归档单测**

```go
func TestHandleT0SelectionArchived(t *testing.T) {
	orig := t0CacheRootPath
	t0CacheRootPath = t.TempDir()
	defer func() { t0CacheRootPath = orig }()

	date := "2026-08-05"
	_ = saveT0SelectionArchive(date, []T0SelectionResult{{StockCode: "600000.XSHG"}}, true)

	req := httptest.NewRequest(http.MethodGet, "/api/t0-selection?archived=1&date="+date, nil)
	rr := httptest.NewRecorder()
	handleT0Selection(rr, req)
	if rr.Code != 200 {
		t.Fatalf("status %d body %s", rr.Code, rr.Body.String())
	}
	if !strings.Contains(rr.Body.String(), `"archived":true`) {
		t.Fatalf("body: %s", rr.Body.String())
	}
	if !strings.Contains(rr.Body.String(), "600000.XSHG") {
		t.Fatalf("missing stock: %s", rr.Body.String())
	}
}
```

- [ ] **Step 2: 跑测确认失败**

Run: `go test ./backend/flutter_api/ -run TestHandleT0SelectionArchived -count=1`  
Expected: FAIL（若尚未接 archived）

- [ ] **Step 3: 实现 handler 分流**

处理顺序：

1. `archived=1` → 读归档返回（优先于 prewarm）
2. `prewarm=1` → 文件已存在则 ready 统计；否则 `tryStartT0Prewarm` 并立刻返回进度 JSON
3. 正式选股：`shouldReturnWarmingForSelection` → warming 空结果
4. 否则 `RunT0Selection`；成功后 `saveT0SelectionArchive(date, results, forceSave)`，归档错误只打日志

`prewarm` 且文件已存在时：load gob，跑涨停+成交额得到 `candidate_count`，返回 `status: ready`、`cache_hit: true`。

更新文件头 API 注释。

- [ ] **Step 4: 跑全部相关单测**

Run: `go test ./backend/flutter_api/ -run 'TestT0|TestSaveT0|TestTryStart|TestSelectionBlocked|TestHandleT0' -count=1`  
Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add backend/flutter_api/t0_selection.go backend/flutter_api/t0_selection_cache_test.go
git commit -m "feat(t0): wire archived/save/prewarm status into t0-selection handler"
```

---

### Task 5: Spec 覆盖自检

- [ ] **Step 1: 对照 spec 确认**

| Spec 要求 | 任务 |
|-----------|------|
| `/tmp/go-stock-cache/t0/daily|selection` | Task 1 |
| prewarm 中立刻返回进度 | Task 3–4 |
| 选股无文件且 warming → warming | Task 3–4 |
| 无文件非 warming → 现场拉取 | 保留 `loadOrFetchT0Daily` |
| 结果默认写一次、`save=1` 强制 | Task 2–4 |
| `archived=1` 优先 | Task 4 |
| 归档失败不影响选股响应 | Task 4 |

- [ ] **Step 2: 若有注释修订则单独 commit**

---

## Spec Coverage Checklist

- [x] 缓存目录层级
- [x] 非阻塞预热 + 进度字段
- [x] 选股与预热交互三条
- [x] 结果归档写一次 + save
- [x] archived 读取与优先级
- [x] 错误不阻断选股返回
