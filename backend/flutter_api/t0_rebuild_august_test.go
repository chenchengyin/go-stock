package flutter_api

import (
	"os"
	"path/filepath"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"go.uber.org/zap"

	"go-stock/backend/db"
	"go-stock/backend/logger"
)

// TestRebuildAugustPoolAndSelection 重拉 8 月股池日线并强制覆盖选股归档。
// 日线只按月末交易日拉一次，再按日截断写入；选股对每个交易日 RunT0Selection + save=force。
// 用法: RUN_REBUILD_AUG=1 go test ./backend/flutter_api -run TestRebuildAugustPoolAndSelection -count=1 -v -timeout 45m
func TestRebuildAugustPoolAndSelection(t *testing.T) {
	if os.Getenv("RUN_REBUILD_AUG") != "1" {
		t.Skip("set RUN_REBUILD_AUG=1 to rebuild August T0 pool and selection")
	}
	if logger.SugaredLogger == nil {
		logger.InitLogger()
	}

	db.Init(stockDBPath(t))
	AutoMigrate()

	projectRoot := filepath.Clean(filepath.Join(filepath.Dir(stockDBPath(t)), "..", ".."))
	root, err := resolveT0CacheRoot("", projectRoot, "")
	if err != nil {
		t.Fatalf("resolve cache root: %v", err)
	}
	orig := t0CacheRootPath
	t0CacheRootPath = root
	defer func() { t0CacheRootPath = orig }()
	if err := ensureT0CacheDirs(); err != nil {
		t.Fatal(err)
	}
	t.Logf("cache root: %s", t0CacheRootPath)

	endDate := "2026-08-21"
	dates := weekdaysThrough(2026, time.August, 21)
	rebuildT0PoolAndSelection(t, endDate, 0, dates)
}

// TestRebuildJuneJulyPoolAndSelection 重拉 6–7 月股池日线并强制覆盖选股归档。
// 日线只按 7 月末拉一次，再按日截断写入。
// 用法: RUN_REBUILD_JUN_JUL=1 go test ./backend/flutter_api -run TestRebuildJuneJulyPoolAndSelection -count=1 -v -timeout 90m
func TestRebuildJuneJulyPoolAndSelection(t *testing.T) {
	if os.Getenv("RUN_REBUILD_JUN_JUL") != "1" {
		t.Skip("set RUN_REBUILD_JUN_JUL=1 to rebuild June–July T0 pool and selection")
	}
	if logger.SugaredLogger == nil {
		logger.InitLogger()
	}

	db.Init(stockDBPath(t))
	AutoMigrate()

	projectRoot := filepath.Clean(filepath.Join(filepath.Dir(stockDBPath(t)), "..", ".."))
	root, err := resolveT0CacheRoot("", projectRoot, "")
	if err != nil {
		t.Fatalf("resolve cache root: %v", err)
	}
	orig := t0CacheRootPath
	t0CacheRootPath = root
	defer func() { t0CacheRootPath = orig }()
	if err := ensureT0CacheDirs(); err != nil {
		t.Fatal(err)
	}
	t.Logf("cache root: %s", t0CacheRootPath)

	endDate := "2026-07-31"
	dates := append(weekdaysThrough(2026, time.June, 30), weekdaysThrough(2026, time.July, 31)...)
	rebuildT0PoolAndSelection(t, endDate, 0, dates)
}

// TestRebuildJanFebPoolAndSelection 重拉 1–2 月股池日线并强制覆盖选股归档。
// 日线用 8 月窗口 + 180 根回看再截断（春节休市日会因 K 过少自动跳过）。
// 用法: RUN_REBUILD_JAN_FEB=1 go test ./backend/flutter_api -run TestRebuildJanFebPoolAndSelection -count=1 -v -timeout 90m
func TestRebuildJanFebPoolAndSelection(t *testing.T) {
	if os.Getenv("RUN_REBUILD_JAN_FEB") != "1" {
		t.Skip("set RUN_REBUILD_JAN_FEB=1 to rebuild January–February T0 pool and selection")
	}
	if logger.SugaredLogger == nil {
		logger.InitLogger()
	}

	db.Init(stockDBPath(t))
	AutoMigrate()

	projectRoot := filepath.Clean(filepath.Join(filepath.Dir(stockDBPath(t)), "..", ".."))
	root, err := resolveT0CacheRoot("", projectRoot, "")
	if err != nil {
		t.Fatalf("resolve cache root: %v", err)
	}
	orig := t0CacheRootPath
	t0CacheRootPath = root
	defer func() { t0CacheRootPath = orig }()
	if err := ensureT0CacheDirs(); err != nil {
		t.Fatal(err)
	}
	t.Logf("cache root: %s", t0CacheRootPath)

	dates := append(weekdaysThrough(2026, time.January, 31), weekdaysThrough(2026, time.February, 28)...)
	rebuildT0PoolAndSelection(t, "2026-08-21", 180, dates)
}

// TestRebuildMarchPoolAndSelection 重拉 3 月股池日线并强制覆盖选股归档。
// 日线用 8 月窗口 + 180 根回看再截断到 3 月（与 4/5 月同一套，避免东财 end= 空数据）。
// 用法: RUN_REBUILD_MAR=1 go test ./backend/flutter_api -run TestRebuildMarchPoolAndSelection -count=1 -v -timeout 60m
func TestRebuildMarchPoolAndSelection(t *testing.T) {
	if os.Getenv("RUN_REBUILD_MAR") != "1" {
		t.Skip("set RUN_REBUILD_MAR=1 to rebuild March T0 pool and selection")
	}
	if logger.SugaredLogger == nil {
		logger.InitLogger()
	}

	db.Init(stockDBPath(t))
	AutoMigrate()

	projectRoot := filepath.Clean(filepath.Join(filepath.Dir(stockDBPath(t)), "..", ".."))
	root, err := resolveT0CacheRoot("", projectRoot, "")
	if err != nil {
		t.Fatalf("resolve cache root: %v", err)
	}
	orig := t0CacheRootPath
	t0CacheRootPath = root
	defer func() { t0CacheRootPath = orig }()
	if err := ensureT0CacheDirs(); err != nil {
		t.Fatal(err)
	}
	t.Logf("cache root: %s", t0CacheRootPath)

	dates := weekdaysThrough(2026, time.March, 31)
	rebuildT0PoolAndSelection(t, "2026-08-21", 180, dates)
}

// TestRebuildAprilMayPoolAndSelection 重拉 4–5 月股池日线并强制覆盖选股归档。
// 日线只按 5 月末拉一次，再按日截断写入。清明 / 五一休市日会因当日 K 过少而跳过。
// 用法: RUN_REBUILD_APR_MAY=1 go test ./backend/flutter_api -run TestRebuildAprilMayPoolAndSelection -count=1 -v -timeout 90m
func TestRebuildAprilMayPoolAndSelection(t *testing.T) {
	if os.Getenv("RUN_REBUILD_APR_MAY") != "1" {
		t.Skip("set RUN_REBUILD_APR_MAY=1 to rebuild April–May T0 pool and selection")
	}
	if logger.SugaredLogger == nil {
		logger.InitLogger()
	}

	db.Init(stockDBPath(t))
	AutoMigrate()

	projectRoot := filepath.Clean(filepath.Join(filepath.Dir(stockDBPath(t)), "..", ".."))
	root, err := resolveT0CacheRoot("", projectRoot, "")
	if err != nil {
		t.Fatalf("resolve cache root: %v", err)
	}
	orig := t0CacheRootPath
	t0CacheRootPath = root
	defer func() { t0CacheRootPath = orig }()
	if err := ensureT0CacheDirs(); err != nil {
		t.Fatal(err)
	}
	t.Logf("cache root: %s", t0CacheRootPath)

	// 东财按 5 月末 end= 会大面积空数据；用 8 月已验证能通的窗口多拉回看，再截断到 4/5 月。
	dates := append(weekdaysThrough(2026, time.April, 30), weekdaysThrough(2026, time.May, 29)...)
	rebuildT0PoolAndSelection(t, "2026-08-21", 180, dates)
}

func rebuildT0PoolAndSelection(t *testing.T, fetchEnd string, klineLimit int, dates []string) {
	t.Helper()
	if len(dates) == 0 {
		t.Fatal("no weekdays")
	}

	t.Log("fetch stock pool + daily klines through", fetchEnd, "limit", klineLimit)
	silenceRebuildLogger()
	stocks := fetchStockPoolFromSina()
	if len(stocks) == 0 {
		t.Fatal("股票池为空")
	}
	daily := fetchAllDailyKLineRebuild(t, stocks, fetchEnd, klineLimit)
	if len(daily) == 0 {
		t.Fatal("日线获取为空")
	}
	t.Logf("pool=%d daily=%d", len(stocks), len(daily))

	okDays := 0
	for _, d := range dates {
		truncated := truncateDailyBarsTo(daily, d)
		onDate := countBarsOnDate(truncated, d)
		t.Logf("%s truncated=%d barsOnDate=%d", d, len(truncated), onDate)
		if onDate < 100 {
			t.Logf("%s: 当日K过少 (%d)，跳过（可能非交易日）", d, onDate)
			continue
		}
		if err := saveT0DailyCache(d, stocks, truncated); err != nil {
			t.Errorf("%s: 写日线缓存失败: %v", d, err)
			continue
		}
		results, err := RunT0Selection(d)
		if err != nil {
			t.Errorf("%s: 选股失败: %v", d, err)
			continue
		}
		if err := saveT0SelectionArchive(d, results, true); err != nil {
			t.Errorf("%s: 写选股归档失败: %v", d, err)
			continue
		}
		okDays++
		t.Logf("%s selection=%d", d, len(results))
	}
	if okDays == 0 {
		t.Fatal("没有成功重建任何一个交易日")
	}
	t.Logf("rebuilt %d / %d weekdays", okDays, len(dates))
}

func silenceRebuildLogger() {
	nop := zap.NewNop()
	logger.Logger = nop
	logger.SugaredLogger = nop.Sugar()
}

func fetchAllDailyKLineRebuild(t *testing.T, stocks []t0Stock, endDate string, limit int) map[string][]dailyBar {
	t.Helper()
	cache := make(map[string][]dailyBar)
	var mu sync.Mutex
	var wg sync.WaitGroup
	sem := make(chan struct{}, 8)
	var done atomic.Int32
	total := int32(len(stocks))
	for _, s := range stocks {
		wg.Add(1)
		go func(stock t0Stock) {
			defer wg.Done()
			sem <- struct{}{}
			defer func() { <-sem }()
			var bars []dailyBar
			if limit > 0 {
				bars = fetchDailyKLineWithLimit(stock.ShortCode, endDate, limit)
			} else {
				bars = fetchDailyKLine(stock.ShortCode, endDate)
			}
			if len(bars) < 2 {
				time.Sleep(150 * time.Millisecond)
				if limit > 0 {
					bars = fetchDailyKLineWithLimit(stock.ShortCode, endDate, limit)
				} else {
					bars = fetchDailyKLine(stock.ShortCode, endDate)
				}
			}
			if len(bars) >= 2 {
				mu.Lock()
				cache[stock.ShortCode] = bars
				mu.Unlock()
			}
			n := done.Add(1)
			if n%200 == 0 || n == total {
				t.Logf("kline progress %d/%d", n, total)
			}
		}(s)
	}
	wg.Wait()
	return cache
}

func weekdaysThrough(year int, month time.Month, lastDay int) []string {
	var out []string
	for day := 1; day <= lastDay; day++ {
		d := time.Date(year, month, day, 0, 0, 0, 0, time.UTC)
		if d.Weekday() == time.Saturday || d.Weekday() == time.Sunday {
			continue
		}
		out = append(out, d.Format("2006-01-02"))
	}
	return out
}

func truncateDailyBarsTo(daily map[string][]dailyBar, endDate string) map[string][]dailyBar {
	out := make(map[string][]dailyBar, len(daily))
	for sc, bars := range daily {
		cut := len(bars)
		for i, b := range bars {
			if b.Date > endDate {
				cut = i
				break
			}
		}
		if cut < 2 {
			continue
		}
		nb := make([]dailyBar, cut)
		copy(nb, bars[:cut])
		out[sc] = nb
	}
	return out
}

func countBarsOnDate(daily map[string][]dailyBar, date string) int {
	n := 0
	for _, bars := range daily {
		if len(bars) > 0 && bars[len(bars)-1].Date == date {
			n++
		}
	}
	return n
}
