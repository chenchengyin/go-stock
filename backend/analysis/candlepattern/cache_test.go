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
