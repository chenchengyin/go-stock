package flutter_api

import (
	"encoding/gob"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"
)

type t0PoolSource struct {
	Stocks []t0Stock
	Source string
}

// Reconcile all witnessed sources before copying any of them. Legacy caches
// without schema/completeness proof are diagnostic evidence, never membership.
func collectT0PoolSources(snapshot MainboardFloatCap50Gob) (map[string]t0PoolSource, map[string]string, []string) {
	pools := map[string]t0PoolSource{}
	issues := map[string]string{}
	legacy := []string{}
	today := time.Now().In(chinaLocation()).Format("2006-01-02")
	add := func(date string, stocks []t0Stock, source string) {
		a := t0PoolArchive{SchemaVersion: 1, Date: date, Complete: true, Stocks: stocks, Source: source}
		if err := validateT0PoolArchive(a, date); err != nil {
			issues[date] = err.Error()
			return
		}
		if old, ok := pools[date]; ok && !sameT0StockPool(old.Stocks, stocks) && date != today {
			issues[date] = fmt.Sprintf("历史股票池来源冲突: %s (%s / %s)", date, old.Source, source)
			return
		}
		pools[date] = t0PoolSource{Stocks: stocks, Source: source}
	}
	for date, pool := range snapshot.SelectionPools {
		if len(pool) == 0 {
			continue
		}
		stocks := make([]t0Stock, 0, len(pool))
		for _, s := range pool {
			prefix := "sz."
			if strings.HasPrefix(s.Code, "60") {
				prefix = "sh."
			}
			stocks = append(stocks, t0Stock{Code: prefix + s.Code, ShortCode: s.Code, Name: s.Name, MarketCapYi: s.MarketCapYi})
		}
		add(date, stocks, "shared_gob_dated_pool")
	}
	files, _ := filepath.Glob(filepath.Join(t0CacheRootPath, "t0", "pools", "pool_*.json"))
	for _, p := range files {
		date := strings.TrimSuffix(strings.TrimPrefix(filepath.Base(p), "pool_"), ".json")
		a, err := readT0PoolArchive(date)
		if err != nil {
			issues[date] = err.Error()
			continue
		}
		add(date, a.Stocks, a.Source)
	}
	// At most once per shared-GOB load/audit, never per quote refresh. Keep only
	// pool metadata; do not retain or duplicate the legacy daily-price maps.
	files, _ = filepath.Glob(filepath.Join(t0CacheRootPath, "t0", "daily", "t0_daily_cache_*.gob"))
	for _, p := range files {
		date := strings.TrimSuffix(strings.TrimPrefix(filepath.Base(p), "t0_daily_cache_"), ".gob")
		f, err := os.Open(p)
		if err != nil {
			issues[date] = err.Error()
			continue
		}
		var cache struct {
			DataSchemaVersion int
			TradeDate         string
			Stocks            []t0Stock
			StockPoolComplete bool
		}
		err = gob.NewDecoder(f).Decode(&cache)
		f.Close()
		if err != nil || cache.DataSchemaVersion != t0DailyCacheSchemaVersion || cache.TradeDate != date || !cache.StockPoolComplete || len(cache.Stocks) == 0 {
			legacy = append(legacy, date)
			continue
		}
		add(date, cache.Stocks, "schema2_complete_dated_cache")
	}
	return pools, issues, legacy
}
