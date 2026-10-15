package flutter_api

import (
	"encoding/json"
	"go-stock/backend/data"
	"math"
	"os"
	"path/filepath"
	"testing"
)

func TestIntradayTradingCalendarIgnoresHolidayCacheAndPools(t *testing.T) {
	oldRoot, oldShared := t0CacheRootPath, mainboardSharedGob
	t0CacheRootPath = t.TempDir()
	mainboardSharedGob = &mainboardSharedGobData{
		Dates:          []string{"2026-09-30", "2026-10-08", "2026-10-09", "2026-10-12"},
		Daily:          map[string][]dailyBar{"600498": {{Date: "2026-09-30", Close: 10}, {Date: "2026-10-08", Close: 11}, {Date: "2026-10-09", Close: 12}, {Date: "2026-10-12", Close: 13}}},
		SelectionPools: map[string][]t0Stock{"2026-10-01": {{ShortCode: "600498"}}, "2026-10-07": {{ShortCode: "600498"}}},
	}
	defer func() { t0CacheRootPath = oldRoot; mainboardSharedGob = oldShared }()
	if err := saveT0DailyCache("2026-10-01", []t0Stock{{ShortCode: "600498"}}, mainboardSharedGob.Daily); err != nil {
		t.Fatal(err)
	}
	if isValidTradingDay("2026-10-01") {
		t.Fatal("holiday cache/pool accepted as trading day")
	}
	for _, tc := range []struct{ t0, requested, want, prev, next string }{
		{"2026-09-30", "", "2026-10-08", "2026-09-30", "2026-10-09"},
		{"2026-09-30", "2026-10-08", "2026-10-08", "2026-09-30", "2026-10-09"},
		{"2026-09-30", "2026-10-12", "2026-10-12", "2026-10-09", ""},
	} {
		got, prev, next, ok := resolveRedT1Session(tc.t0, tc.requested, "2026-10-12", isValidTradingDay, resolveNextTradingDay, resolvePrevTradingDay)
		if !ok || got != tc.want || prev != tc.prev || next != tc.next {
			t.Fatalf("session %s: %s %s %s %v", tc.requested, got, prev, next, ok)
		}
	}
	if got := redT1PreviousClose("600498", "2026-09-30"); got != 10 {
		t.Fatalf("did not use exact cached previous close: %v", got)
	}
}

func TestIntradayUsesSourceBasePriceAndInvalidatesLegacyCache(t *testing.T) {
	oldRoot, oldFetcher := t0CacheRootPath, redT1IntradayFetcher
	t0CacheRootPath = t.TempDir()
	defer func() { t0CacheRootPath = oldRoot; redT1IntradayFetcher = oldFetcher }()
	const code = "600498.XSHG"
	const date = "2026-07-23"
	legacy := redT1IntradayCacheRecord{Version: 1, Series: redT1IntradaySeries{Code: code, Date: date, Source: "ths", PrevClose: 99, Bars: []redT1IntradayBar{{Time: "09:30", Price: 99}, {Time: "15:00", Price: 99}}}}
	raw, _ := json.Marshal(legacy)
	path := redT1IntradayCachePath(code, date)
	_ = os.MkdirAll(filepath.Dir(path), 0755)
	_ = os.WriteFile(path, raw, 0644)
	if _, ok := loadRedT1IntradayCache(code, date); ok {
		t.Fatal("legacy cache retained")
	}
	calls := 0
	redT1IntradayFetcher = func(short, date string, limit int) *data.KLineSourceResult {
		calls++
		rows := []data.KLineData{{Day: date + " 09:30:00", Close: "11", Volume: "1", Amount: "1100"}, {Day: date + " 09:31:00", Close: "12", Volume: "2", Amount: "2400"}}
		return &data.KLineSourceResult{Data: &rows, Source: "ths", PrevClose: 10}
	}
	// An unavailable daily baseline must not block a valid source-provided base_price.
	item := redT1IntradayItems("1900-01-01", date, []string{code})[code]
	if item.Status != "ok" || item.PrevClose != 10 || len(item.Bars) != 2 || calls != 1 {
		t.Fatalf("source baseline unused: %+v calls=%d", item, calls)
	}
	if math.Abs(item.Bars[1].AveragePrice-3500.0/300) > 0.000001 {
		t.Fatalf("wrong cumulative average: %+v", item.Bars)
	}
	again := redT1IntradayItems("1900-01-01", date, []string{code})[code]
	if calls != 1 || again.PrevClose != 10 {
		t.Fatalf("new cache not reused: %+v calls=%d", again, calls)
	}
}

func TestIntradayCalendarRejectsDiskCacheWithoutExactDailyBar(t *testing.T) {
	oldRoot, oldShared := t0CacheRootPath, mainboardSharedGob
	oldFetcher := tradingDayDailyFetcher
	t0CacheRootPath = t.TempDir()
	mainboardSharedGob = nil
	defer func() { t0CacheRootPath = oldRoot; mainboardSharedGob = oldShared; tradingDayDailyFetcher = oldFetcher }()
	tradingDayDailyFetcher = func(string, string) []dailyBar { return []dailyBar{{Date: "2026-09-30", Close: 10}} }
	daily := map[string][]dailyBar{"600498": {{Date: "2026-09-30", Close: 10}}}
	if err := saveT0DailyCache("2026-10-01", []t0Stock{{ShortCode: "600498"}}, daily); err != nil {
		t.Fatal(err)
	}
	if isValidTradingDay("2026-10-01") {
		t.Fatal("cache with prior-day bars proves holiday is a trading day")
	}
}

func TestIntradayCalendarChecksLiveDayBeyondSharedCoverage(t *testing.T) {
	oldRoot, oldShared, oldFetcher := t0CacheRootPath, mainboardSharedGob, tradingDayDailyFetcher
	t0CacheRootPath = t.TempDir()
	mainboardSharedGob = &mainboardSharedGobData{Dates: []string{"2026-10-14"}}
	defer func() { t0CacheRootPath = oldRoot; mainboardSharedGob = oldShared; tradingDayDailyFetcher = oldFetcher }()
	if err := saveT0DailyCache("2026-10-15", []t0Stock{{ShortCode: "600000"}}, map[string][]dailyBar{}); err != nil {
		t.Fatal(err)
	}
	calls := 0
	tradingDayDailyFetcher = func(code, date string) []dailyBar { calls++; return []dailyBar{{Date: "2026-10-15", Close: 10}} }
	if !isValidTradingDay("2026-10-15") || calls != 1 {
		t.Fatalf("prewarm cache hid a real trading day: calls=%d", calls)
	}
}
