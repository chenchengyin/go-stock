package flutter_api

import (
	"fmt"
	"sync"
	"testing"
	"time"
)

func TestSharedGobRefreshIncludesEveryStoredStock(t *testing.T) {
	target := "2026-10-19"
	shared := &mainboardSharedGobData{
		Snapshot: MainboardFloatCap50Gob{Stocks: []MainboardFloatCap50Stock{{Code: "600000"}}, SelectionPools: map[string][]MainboardT0PoolStock{
			"2026-10-16": {{Code: "600001", Name: "old pool only"}},
			"2026-10-20": {{Code: "600002", Name: "new pool only"}},
		}},
		Daily: map[string][]dailyBar{"600000": {{Date: "2026-10-16"}}, "600003": {{Date: "2026-10-16"}}},
	}
	got := mainboardSharedGobPendingSelectionPool(shared, target)
	codes := map[string]bool{}
	for _, s := range got {
		codes[s.Code] = true
	}
	for _, code := range []string{"600001", "600002", "600003"} {
		if !codes[code] {
			t.Errorf("stored stock %s outside target day's selection pool was skipped", code)
		}
	}
	if codes["600000"] || len(got) != 3 {
		t.Fatalf("duplicates or base stock included: %v", got)
	}
}

func isolateSharedGobNightRefresh(t *testing.T) {
	t.Helper()
	isolateSharedGobConfirmation(t)
	oldBars, oldQuotes := mainboardSharedGobBarsFetcher, mainboardSharedGobClosingQuoteFetcher
	mainboardSharedGobRefreshState.Lock()
	oldDate, oldAt := mainboardSharedGobRefreshState.attemptDate, mainboardSharedGobRefreshState.lastAttemptAt
	mainboardSharedGobRefreshState.attemptDate = ""
	mainboardSharedGobRefreshState.lastAttemptAt = time.Time{}
	mainboardSharedGobRefreshState.Unlock()
	t.Cleanup(func() {
		mainboardSharedGobBarsFetcher = oldBars
		mainboardSharedGobClosingQuoteFetcher = oldQuotes
		mainboardSharedGobRefreshState.Lock()
		mainboardSharedGobRefreshState.attemptDate = oldDate
		mainboardSharedGobRefreshState.lastAttemptAt = oldAt
		mainboardSharedGobRefreshState.Unlock()
	})
}

func TestSharedGobNightRefreshRepairsAllScopesAndRetriesFailures(t *testing.T) {
	isolateSharedGobNightRefresh(t)
	target := "2026-10-19"
	snapshot := MainboardFloatCap50Gob{SchemaVersion: 2, StartDate: "2026-10-01", Stocks: []MainboardFloatCap50Stock{{Code: "600000", FloatMarketCapYi: 100}, {Code: "601198", FloatMarketCapYi: 100}}, Daily: map[string][]dailyBar{
		"600000": {{Date: "2026-10-16", Close: 10}, {Date: target, Open: 10, Close: 10.1}},
		"601198": {{Date: "2026-10-16", Close: 10}},
		"600519": {{Date: "2026-10-16", Close: 10}},
	}, SelectionPools: map[string][]MainboardT0PoolStock{"2026-10-16": {{Code: "600001"}}, "2026-10-20": {{Code: "600000"}, {Code: "601198"}}}}
	if err := writeMainboardFloatCap50Gob(mainboardSharedGobPath(), snapshot); err != nil {
		t.Fatal(err)
	}
	sharedGobPriorDateFetcher = func(day string) (string, error) {
		if day != "2026-10-20" {
			t.Errorf("wrong witness day %s", day)
		}
		return target, nil
	}
	mainboardSharedGobClosingQuoteFetcher = func(_ []t0Stock, date string) map[string]t0Realtime {
		if date != target {
			t.Fatal(date)
		}
		return map[string]t0Realtime{"601198": {Source: "tencent", TradeDate: target, QuoteTime: "20261019150000", PrevClose: 10}}
	}
	var mu sync.Mutex
	fetched := map[string]int{}
	fail := true
	mainboardSharedGobBarsFetcher = func(s mainboardFloatCap50Candidate, _ int, _, end string) ([]dailyBar, string, error) {
		mu.Lock()
		defer mu.Unlock()
		fetched[s.code]++
		if end != target {
			t.Errorf("requested unfinished date %s", end)
		}
		if s.code == "600001" && fail {
			return nil, "", fmt.Errorf("temporary download failure")
		}
		return []dailyBar{{Date: "2026-10-16", Close: 10}, {Date: target, Open: 10, High: 12, Low: 9, Close: 11}}, "test", nil
	}
	now := time.Date(2026, 10, 20, 0, 0, 0, 0, chinaLocation())
	runMainboardSharedGobRefreshTick(now)
	current, err := loadMainboardSharedGob()
	if err != nil {
		t.Fatal(err)
	}
	if fetched["600000"] != 1 || fetched["600519"] != 1 || fetched["600001"] != 1 || fetched["601198"] != 0 {
		t.Fatalf("wrong downloads: %v", fetched)
	}
	if current.Snapshot.NonTradingDates["601198"] != target || containsSharedGobBarDate(current.Daily["601198"], target) {
		t.Fatal("suspension evidence was lost or fake candle inserted")
	}
	if current.Snapshot.ClosedDailyDates["600001"] == target {
		t.Fatal("download failure marked complete")
	}
	if got, _ := dailyBarOnDate(current.Daily["600000"], target); got.Close != 11 {
		t.Fatal("intraday candle was not replaced")
	}
	runMainboardSharedGobRefreshTick(now.Add(time.Minute))
	if fetched["600001"] != 1 {
		t.Fatal("retry cooldown ignored")
	}
	fail = false
	runMainboardSharedGobRefreshTick(now.Add(31 * time.Minute))
	if fetched["600001"] != 2 || fetched["600519"] != 1 || fetched["600000"] != 1 {
		t.Fatalf("retry fetched already complete stocks: %v", fetched)
	}
	mainboardSharedGob = nil
	current, err = loadMainboardSharedGob()
	if err != nil {
		t.Fatal(err)
	}
	if len(mainboardSharedGobPendingIndices(current, target)) != 0 || len(mainboardSharedGobPendingSelectionPool(current, target)) != 0 {
		t.Fatal("all stored stocks not covered after retry/reload")
	}
	if !sharedGobRedHistoryReady("2026-10-20") {
		t.Fatal("persisted no-trade evidence did not unblock dated-pool readiness")
	}
	runMainboardSharedGobRefreshTick(time.Date(2026, 10, 20, 9, 24, 0, 0, chinaLocation()))
	if fetched["600001"] != 2 {
		t.Fatal("complete data downloaded again")
	}
}

func TestSharedGobNightRefreshStopsAtAuctionAndDoesNotRunAfterClose(t *testing.T) {
	isolateSharedGobNightRefresh(t)
	sharedGobPriorDateFetcher = func(string) (string, error) { t.Fatal("out-of-window refresh queried network"); return "", nil }
	for _, hm := range [][2]int{{9, 25}, {15, 10}, {23, 59}} {
		runT0AutoPrewarmTick(time.Date(2026, 10, 20, hm[0], hm[1], 0, 0, chinaLocation()))
	}
}

func TestSharedGobNightRefreshUsesLatestSessionOnWeekend(t *testing.T) {
	isolateSharedGobNightRefresh(t)
	target := "2026-10-23"
	if err := writeMainboardFloatCap50Gob(mainboardSharedGobPath(), MainboardFloatCap50Gob{SchemaVersion: 2, StartDate: "2026-10-01", Stocks: []MainboardFloatCap50Stock{{Code: "600000", FloatMarketCapYi: 100}}, Daily: map[string][]dailyBar{"600000": {{Date: "2026-10-22", Close: 10}}}}); err != nil {
		t.Fatal(err)
	}
	sharedGobPriorDateFetcher = func(day string) (string, error) {
		if day != "2026-10-24" {
			t.Fatal(day)
		}
		return target, nil
	}
	mainboardSharedGobClosingQuoteFetcher = func([]t0Stock, string) map[string]t0Realtime { return nil }
	fetched := false
	mainboardSharedGobBarsFetcher = func(_ mainboardFloatCap50Candidate, _ int, _, end string) ([]dailyBar, string, error) {
		fetched = true
		if end != target {
			t.Error(end)
		}
		return []dailyBar{{Date: target, Open: 10, Close: 11, High: 12, Low: 9}}, "test", nil
	}
	runMainboardSharedGobRefreshTick(time.Date(2026, 10, 24, 0, 0, 0, 0, chinaLocation()))
	if !fetched {
		t.Fatal("weekend skipped the latest completed trading day")
	}
}

func TestSharedGobNightRefreshRetriesBeyondFourAttempts(t *testing.T) {
	isolateSharedGobNightRefresh(t)
	target := "2026-10-19"
	if err := writeMainboardFloatCap50Gob(mainboardSharedGobPath(), MainboardFloatCap50Gob{SchemaVersion: 2, StartDate: "2026-10-01", Stocks: []MainboardFloatCap50Stock{{Code: "600000", FloatMarketCapYi: 100}}, Daily: map[string][]dailyBar{"600000": {{Date: "2026-10-16", Close: 10}}}}); err != nil {
		t.Fatal(err)
	}
	sharedGobPriorDateFetcher = func(string) (string, error) { return target, nil }
	mainboardSharedGobClosingQuoteFetcher = func([]t0Stock, string) map[string]t0Realtime { return nil }
	attempts := 0
	mainboardSharedGobBarsFetcher = func(_ mainboardFloatCap50Candidate, _ int, _, _ string) ([]dailyBar, string, error) {
		attempts++
		if attempts <= 5 {
			return nil, "", fmt.Errorf("provider unavailable")
		}
		return []dailyBar{{Date: target, Open: 10, Close: 11, High: 12, Low: 9}}, "test", nil
	}
	midnight := time.Date(2026, 10, 20, 0, 0, 0, 0, chinaLocation())
	for i := 0; i < 5; i++ {
		runMainboardSharedGobRefreshTick(midnight.Add(time.Duration(i) * 31 * time.Minute))
	}
	runMainboardSharedGobRefreshTick(time.Date(2026, 10, 20, 9, 24, 0, 0, chinaLocation()))
	if attempts != 6 {
		t.Fatalf("late recovery never retried: %d", attempts)
	}
	current, err := loadMainboardSharedGob()
	if err != nil {
		t.Fatal(err)
	}
	if current.Snapshot.ClosedDailyDates["600000"] != target {
		t.Fatal("late retry not persisted")
	}
}
