package flutter_api

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"

	"go-stock/backend/db"
)

// TestBacktestAug4Selection 用日线开盘价回测 2026-08-04 竞价选股，结果写到 /tmp
func TestBacktestAug4Selection(t *testing.T) {
	const date = "2026-08-04"

	db.Init(stockDBPath(t))
	AutoMigrate()

	// 使用正式缓存目录，便于复用预热结果
	orig := t0CacheRootPath
	t0CacheRootPath = "/tmp/go-stock-cache"
	defer func() { t0CacheRootPath = orig }()
	_ = ensureT0CacheDirs()

	if !isT0DailyCacheFilePresent(date) {
		t.Log("无日线缓存，开始预热…")
		tryStartT0Prewarm(date)
		deadline := time.Now().Add(20 * time.Minute)
		for !isT0DailyCacheFilePresent(date) {
			if time.Now().After(deadline) {
				prog := getT0WarmProgress(date)
				t.Fatalf("预热超时: %+v", prog)
			}
			prog := getT0WarmProgress(date)
			if prog.Status == t0WarmStatusFailed {
				t.Fatalf("预热失败: %s", prog.Err)
			}
			t.Logf("预热中 fetched=%d/%d", prog.DailyFetched, prog.DailyTotal)
			time.Sleep(3 * time.Second)
		}
	} else {
		t.Logf("复用日线缓存: %s", t0DailyCachePath(date))
	}

	results, err := RunT0Selection(date)
	if err != nil {
		t.Fatalf("选股失败: %v", err)
	}

	outPath := filepath.Join("/tmp", "t0_selection_backtest_2026-08-04.json")
	payload := map[string]interface{}{
		"date":    date,
		"count":   len(results),
		"note":    "竞价涨幅=(当日日线Open-昨收)/昨收；非实时涨幅",
		"results": results,
	}
	data, err := json.MarshalIndent(payload, "", "  ")
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(outPath, data, 0o644); err != nil {
		t.Fatal(err)
	}

	t.Logf("回测完成 count=%d 文件=%s", len(results), outPath)
	n := len(results)
	if n > 15 {
		n = 15
	}
	for i := 0; i < n; i++ {
		r := results[i]
		t.Logf("[%d] %s %s 竞价开盘%.2f%% 收盘%.2f%% 昨额%.2f亿 涨停:%s",
			i+1, r.StockCode, r.StockName, r.OpenGap, r.CloseRet, r.AmountYi, r.LimitUpDates)
	}
}
