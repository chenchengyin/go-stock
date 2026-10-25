package flutter_api

import (
	"encoding/gob"
	"os"
	"testing"
	"time"
)

func TestSharedGobDailyForStocksUsesGobFirstAndFallsBackByCode(t *testing.T) {
	shared := &mainboardSharedGobData{Daily: map[string][]dailyBar{
		"600000": {
			{Date: "2024-01-02", Close: 10},
			{Date: "2024-01-03", Close: 11},
			{Date: "2024-01-04", Close: 12},
		},
		"600002": {{Date: "2024-02-01", Close: 20}, {Date: "2024-02-02", Close: 21}},
	}}
	stocks := []t0Stock{
		{ShortCode: "600000"},
		{ShortCode: "600001"},
		{ShortCode: "600002"},
	}
	fallback := map[string][]dailyBar{
		"600000": {{Date: "2024-01-02", Close: 90}, {Date: "2024-01-03", Close: 91}},
		"600001": {{Date: "2024-01-02", Close: 30}, {Date: "2024-01-03", Close: 31}},
	}

	daily, missing := sharedGobDailyForStocks(shared, stocks, "2024-01-03", fallback)
	if got := daily["600000"][1].Close; got != 11 {
		t.Fatalf("GOB should take precedence for covered stock, close = %v, want 11", got)
	}
	if len(daily["600000"]) != 2 {
		t.Fatalf("GOB bars after end date were not trimmed: got %d bars, want 2", len(daily["600000"]))
	}
	if got := daily["600001"][1].Close; got != 31 {
		t.Fatalf("fallback should supply a stock missing from GOB, close = %v, want 31", got)
	}
	if len(missing) != 1 || missing[0].ShortCode != "600002" {
		t.Fatalf("missing stocks = %+v, want only 600002", missing)
	}
}

func TestLoadOrFetchSharedGobT0DailyPrefersCompleteTodayLegacyPool(t *testing.T) {
	isolateSharedGobConfirmation(t)
	oldRoot := t0CacheRootPath
	t0CacheRootPath = t.TempDir()
	t.Cleanup(func() { t0CacheRootPath = oldRoot })

	mainboardSharedGobMu.Lock()
	oldShared := mainboardSharedGob
	mainboardSharedGob = nil
	mainboardSharedGobMu.Unlock()
	t.Cleanup(func() {
		mainboardSharedGobMu.Lock()
		mainboardSharedGob = oldShared
		mainboardSharedGobMu.Unlock()
	})

	today := time.Now().In(chinaLocation()).Format("2006-01-02")
	previous := time.Now().In(chinaLocation()).AddDate(0, 0, -1).Format("2006-01-02")
	sharedGobPriorDateFetcher = func(string) (string, error) { return previous, nil }
	sharedPool := []MainboardT0PoolStock{{Code: "600001", Name: "A", MarketCapYi: 100}}
	sharedDaily := map[string][]dailyBar{
		"600001": {{Date: previous, Open: 10, Close: 10, High: 10, Low: 10, Volume: 1, AmountYi: 1, AmountVerified: true}, {Date: today, Open: 10, Close: 10, High: 10, Low: 10, Volume: 1, AmountYi: 1, AmountVerified: true}},
		"600002": {{Date: previous, Open: 20, Close: 20, High: 20, Low: 20, Volume: 1, AmountYi: 1, AmountVerified: true}, {Date: today, Open: 20, Close: 20, High: 20, Low: 20, Volume: 1, AmountYi: 1, AmountVerified: true}},
	}
	if err := writeMainboardFloatCap50Gob(mainboardSharedGobPath(), MainboardFloatCap50Gob{
		SchemaVersion:    2,
		ClosedDailyDates: map[string]string{"600001": previous, "600002": previous},
		Stocks: []MainboardFloatCap50Stock{
			{Code: "600001", Name: "A", FloatMarketCapYi: 100},
			{Code: "600002", Name: "B", FloatMarketCapYi: 200},
		},
		Daily:          sharedDaily,
		SelectionPools: map[string][]MainboardT0PoolStock{today: sharedPool},
	}); err != nil {
		t.Fatal(err)
	}
	if err := ensureT0CacheDirs(); err != nil {
		t.Fatal(err)
	}
	cacheFile, err := os.Create(t0DailyCachePath(today))
	if err != nil {
		t.Fatal(err)
	}
	oldPool := []t0Stock{
		{Code: "sh.600001", ShortCode: "600001", Name: "A", MarketCapYi: 100},
		{Code: "sh.600002", ShortCode: "600002", Name: "B", MarketCapYi: 200},
	}
	if err := gob.NewEncoder(cacheFile).Encode(t0DailyCachePayload{
		DataSchemaVersion: t0DailyCacheSchemaVersion,
		TradeDate:         today,
		Stocks:            oldPool,
		Daily:             map[string][]dailyBar{},
		StockPoolComplete: true,
	}); err != nil {
		_ = cacheFile.Close()
		t.Fatal(err)
	}
	if err := cacheFile.Close(); err != nil {
		t.Fatal(err)
	}
	if !isT0DailyCacheFilePresent(today) {
		t.Fatal("complete old pool should be treated as prewarmed")
	}
	prewarm, ok := readT0DailyCachePayload(today)
	if !ok || len(prewarm.Stocks) != len(oldPool) {
		t.Fatalf("prewarm pool = %d, want complete old pool size %d", len(prewarm.Stocks), len(oldPool))
	}

	stocks, daily, _, err := loadOrFetchSharedGobT0Daily(today)
	if err != nil {
		t.Fatal(err)
	}
	if len(stocks) != len(oldPool) || len(daily) != len(oldPool) {
		t.Fatalf("loaded stocks/daily = %d/%d, want complete old pool size %d", len(stocks), len(daily), len(oldPool))
	}
	loaded, err := loadMainboardSharedGob()
	if err != nil {
		t.Fatal(err)
	}
	if got := len(loaded.SelectionPools[today]); got != len(oldPool) {
		t.Fatalf("persisted shared selection pool size = %d, want %d", got, len(oldPool))
	}
}
