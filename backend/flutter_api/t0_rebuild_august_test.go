package flutter_api

import (
	"os"
	"path/filepath"
	"testing"
	"time"

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
	dates := augustWeekdaysThrough(2026, time.August, 21)
	if len(dates) == 0 {
		t.Fatal("no August weekdays")
	}

	t.Log("fetch stock pool + daily klines through", endDate)
	stocks := fetchStockPoolFromSina()
	if len(stocks) == 0 {
		t.Fatal("股票池为空")
	}
	daily := fetchAllDailyKLine(stocks, endDate, nil)
	if len(daily) == 0 {
		t.Fatal("日线获取为空")
	}
	t.Logf("pool=%d daily=%d", len(stocks), len(daily))

	for _, d := range dates {
		truncated := truncateDailyBarsTo(daily, d)
		onDate := countBarsOnDate(truncated, d)
		t.Logf("%s truncated=%d barsOnDate=%d", d, len(truncated), onDate)
		if onDate < 100 {
			t.Errorf("%s: 当日K过少 (%d)，跳过（可能非交易日）", d, onDate)
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
		t.Logf("%s selection=%d", d, len(results))
	}
}

func augustWeekdaysThrough(year int, month time.Month, lastDay int) []string {
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
