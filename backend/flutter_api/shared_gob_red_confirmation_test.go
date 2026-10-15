package flutter_api

import (
	"encoding/json"
	"fmt"
	"go-stock/backend/analysis/t0reference"
	"net/http"
	"net/http/httptest"
	"os"
	"sync"
	"testing"
	"time"
)

func isolateSharedGobConfirmation(t *testing.T) {
	t.Helper()
	oldRoot, oldShared := t0CacheRootPath, mainboardSharedGob
	oldPrior, oldFetch, oldStarter := sharedGobPriorDateFetcher, sharedGobDailyFetcher, t0PrewarmStarter
	oldVerifier := sharedGobNonTradingHistoryVerifier
	oldQuotes := sharedGobRedQuoteBuilder
	sharedGobNonTradingHistoryVerifier = func([]t0Stock, map[string][]dailyBar, string, string) map[string]string { return nil }
	sharedGobPriorDateCache.Lock()
	oldDates := sharedGobPriorDateCache.dates
	oldExcluded := sharedGobPriorDateCache.excluded
	sharedGobPriorDateCache.excluded = map[string]map[string]string{}
	sharedGobPriorDateCache.dates = map[string]string{}
	sharedGobPriorDateCache.Unlock()
	t0CacheRootPath = t.TempDir()
	mainboardSharedGob = nil
	t.Cleanup(func() {
		t0CacheRootPath = oldRoot
		mainboardSharedGob = oldShared
		sharedGobPriorDateFetcher = oldPrior
		sharedGobNonTradingHistoryVerifier = oldVerifier
		sharedGobRedQuoteBuilder = oldQuotes
		sharedGobDailyFetcher = oldFetch
		t0PrewarmStarter = oldStarter
		sharedGobPriorDateCache.Lock()
		sharedGobPriorDateCache.dates = oldDates
		sharedGobPriorDateCache.excluded = oldExcluded
		sharedGobPriorDateCache.Unlock()
	})
}

func confirmationFixture(t *testing.T, date, name string) sharedGobRedConfirmation {
	t.Helper()
	results := []T0SelectionResult{}
	if name != "" {
		results = append(results, T0SelectionResult{StockName: name, StockCode: "605287.XSHG"})
	}
	b, err := json.Marshal(map[string]interface{}{"date": date, "count": len(results), "results": results, "confirmed": true, "confirmed_at": date + "T09:25:10+08:00"})
	if err != nil {
		t.Fatal(err)
	}
	return sharedGobRedConfirmation{SchemaVersion: 1, Date: date, ConfirmedAt: date + "T09:25:10+08:00", HistoryDate: "2026-10-13", Response: b}
}

func TestSharedGobRedConfirmationSurvivesReloadAndCannotBeReplaced(t *testing.T) {
	isolateSharedGobConfirmation(t)
	first := confirmationFixture(t, "2026-10-14", "德才股份")
	initial, err := saveSharedGobRedConfirmation(first)
	if err != nil {
		t.Fatal(err)
	}
	// No in-memory result cache is involved in this read, just like a restart.
	replacement, err := saveSharedGobRedConfirmation(confirmationFixture(t, first.Date, "九阳股份"))
	if err != nil {
		t.Fatal(err)
	}
	a, _ := json.Marshal(initial)
	b, _ := json.Marshal(replacement)
	if string(a) != string(b) {
		t.Fatalf("confirmed result was replaced: %s -> %s", a, b)
	}
	recorder := httptest.NewRecorder()
	handleSharedGobRedSelectionAt(recorder, first.Date, time.Date(2026, 10, 14, 11, 0, 0, 0, chinaLocation()))
	if recorder.Code != 200 {
		t.Fatalf("reload needs GOB or quotes: %d %s", recorder.Code, recorder.Body.String())
	}
	var got map[string]interface{}
	json.Unmarshal(recorder.Body.Bytes(), &got)
	c, _ := json.Marshal(got)
	if string(a) != string(c) {
		t.Fatalf("restart response changed: %s", c)
	}
}

func TestSharedGobRedConfirmationRefreshesDescriptionsWithoutChangingFacts(t *testing.T) {
	isolateSharedGobConfirmation(t)
	rule, _ := gobRedRuleByKey("gob_zt_t1_bear_open_close_body_4combos_v1")
	definition := t0reference.DefaultRuleDefinitions()[0]
	compiled, err := t0reference.CompileRule(definition)
	if err != nil {
		t.Fatal(err)
	}
	snapshot := confirmationFixture(t, "2026-10-14", "已确认股")
	var response map[string]interface{}
	json.Unmarshal(snapshot.Response, &response)
	response["results"] = []T0SelectionResult{{
		StockCode: "600101.XSHG", StockName: "已确认股", OpenGap: 1.23,
		RedRuleHits: []T0RedRuleHit{
			{Key: rule.Key, Version: rule.Version, Name: "中阴线组合", Description: "旧说明", EntryOpen: 10.12},
			{Key: "gob_zt_zt_pb_t1_amplitude_6_10_shadow_le_1_v2", Version: 2, Description: "v2原口径"},
		},
		T0ReferenceHits: []T0ReferenceHit{{RuleKey: compiled.RuleKey, Description: "旧参考说明", SampleCount: 20}},
	}}
	snapshot.Response, _ = json.Marshal(response)
	if _, err := saveSharedGobRedConfirmation(snapshot); err != nil {
		t.Fatal(err)
	}
	before, _ := os.ReadFile(sharedGobRedConfirmationPath(snapshot.Date))
	got, found, err := loadSharedGobRedConfirmation(snapshot.Date)
	if err != nil || !found {
		t.Fatalf("load confirmation: found=%v err=%v", found, err)
	}
	b, _ := json.Marshal(got["results"])
	var results []T0SelectionResult
	json.Unmarshal(b, &results)
	if len(results) != 1 || results[0].OpenGap != 1.23 || results[0].RedRuleHits[0].EntryOpen != 10.12 ||
		results[0].RedRuleHits[0].Key != rule.Key || results[0].RedRuleHits[0].Name != rule.Name ||
		results[0].RedRuleHits[0].Description != rule.Description || results[0].RedRuleHits[1].Description != "v2原口径" ||
		results[0].T0ReferenceHits[0].Description != definition.Description || results[0].T0ReferenceHits[0].SampleCount != 20 {
		t.Fatalf("description refresh changed confirmed facts or kept stale wording: %+v", results)
	}
	after, _ := os.ReadFile(sharedGobRedConfirmationPath(snapshot.Date))
	if string(before) != string(after) {
		t.Fatal("refresh rewrote confirmed snapshot on disk")
	}
}

func TestSharedGobRedHistoryWithoutDatePoolDoesNotUseCurrentPool(t *testing.T) {
	isolateSharedGobConfirmation(t)
	date := "2026-10-13"
	if err := writeMainboardFloatCap50Gob(mainboardSharedGobPath(), MainboardFloatCap50Gob{
		SchemaVersion: 2,
		Stocks:        []MainboardFloatCap50Stock{{Code: "600000", Name: "当前股票", FloatMarketCapYi: 100}},
		Daily: map[string][]dailyBar{
			"600000": {
				{Date: "2026-10-12", Open: 10, High: 10, Low: 10, Close: 10},
				{Date: date, Open: 11, High: 11, Low: 11, Close: 11},
			},
		},
	}); err != nil {
		t.Fatal(err)
	}
	recorder := httptest.NewRecorder()
	handleSharedGobRedSelectionAt(recorder, date, time.Date(2026, 10, 14, 10, 0, 0, 0, chinaLocation()))
	if recorder.Code != http.StatusServiceUnavailable {
		t.Fatalf("历史日期缺少当日股票池时不应使用当前股票池: %d %s", recorder.Code, recorder.Body.String())
	}
	var body map[string]interface{}
	if err := json.Unmarshal(recorder.Body.Bytes(), &body); err != nil {
		t.Fatal(err)
	}
	if body["data_incomplete"] != true || body["date"] != date {
		t.Fatalf("历史股票池缺失状态不明确: %s", recorder.Body.String())
	}
}

func TestSharedGobRedHistoryUsesDatedCachePoolAndSharedBars(t *testing.T) {
	isolateSharedGobConfirmation(t)
	date := "2026-10-09"
	bars := []dailyBar{
		{Date: "2026-10-07", Open: 9.8, High: 10, Low: 9.8, Close: 10, AmountYi: 6, AmountVerified: true},
		{Date: "2026-10-08", Open: 10.5, High: 11, Low: 10.5, Close: 11, AmountYi: 6, AmountVerified: true},
		{Date: date, Open: 11.2, High: 11.5, Low: 11, Close: 11.3, AmountYi: 6, AmountVerified: true},
	}
	if err := writeMainboardFloatCap50Gob(mainboardSharedGobPath(), MainboardFloatCap50Gob{
		SchemaVersion: 2,
		Stocks:        []MainboardFloatCap50Stock{{Code: "600000", Name: "当前股票", FloatMarketCapYi: 100}},
		Daily:         map[string][]dailyBar{"600000": bars, "600001": bars},
	}); err != nil {
		t.Fatal(err)
	}
	dayStock := t0Stock{Code: "sh.600001", ShortCode: "600001", Name: "历史股票", MarketCapYi: 100}
	// The dated cache may only preserve the pool; shared GOB supplies its bars.
	if err := saveT0DailyCache(date, []t0Stock{dayStock}, map[string][]dailyBar{}); err != nil {
		t.Fatal(err)
	}
	var quoteStocks []t0Stock
	sharedGobRedQuoteBuilder = func(date string, stocks []t0Stock, daily map[string][]dailyBar) (map[string]t0Realtime, string, *t0DataWarning) {
		quoteStocks = append([]t0Stock(nil), stocks...)
		return buildSharedGobRedAuctionQuotes(date, stocks, daily)
	}
	recorder := httptest.NewRecorder()
	handleSharedGobRedSelectionAt(recorder, date, time.Date(2026, 10, 14, 10, 0, 0, 0, chinaLocation()))
	if recorder.Code != http.StatusOK {
		t.Fatalf("历史红策未能使用当日股票池: %d %s", recorder.Code, recorder.Body.String())
	}
	if len(quoteStocks) != 1 || quoteStocks[0].ShortCode != dayStock.ShortCode {
		t.Fatalf("历史红策使用了非当日股票池: %+v", quoteStocks)
	}
}

func TestSharedGobRedDateListRevisionChangesWhenListIsConfirmed(t *testing.T) {
	isolateSharedGobConfirmation(t)
	date, _ := writeStaleSharedGobFixture(t)
	getRevision := func() string {
		t.Helper()
		r := httptest.NewRecorder()
		handleT0Selection(r, httptest.NewRequest(http.MethodGet,
			"/api/t0-selection?module_code=radar.red_shared_gob_strategy&list_dates=1", nil))
		if r.Code != http.StatusOK {
			t.Fatalf("date list: %d %s", r.Code, r.Body.String())
		}
		var response struct {
			GeneratedAt string `json:"generated_at"`
		}
		if err := json.Unmarshal(r.Body.Bytes(), &response); err != nil {
			t.Fatal(err)
		}
		return response.GeneratedAt
	}
	before := getRevision()
	if _, err := saveSharedGobRedConfirmation(confirmationFixture(t, date, "")); err != nil {
		t.Fatal(err)
	}
	after := getRevision()
	if before == after || after == "" {
		t.Fatalf("confirmation did not change date-list revision: %q -> %q", before, after)
	}
}

func TestSharedGobRedConfirmationIsFirstWriterWinsAcrossConcurrentWriters(t *testing.T) {
	isolateSharedGobConfirmation(t)
	var wg sync.WaitGroup
	type outcome struct {
		response map[string]interface{}
		err      error
	}
	outcomes := make(chan outcome, 8)
	for i := 0; i < 8; i++ {
		wg.Add(1)
		go func(index int) {
			defer wg.Done()
			response, err := saveSharedGobRedConfirmation(confirmationFixture(t, "2026-10-14", fmt.Sprintf("stock-%d", index)))
			outcomes <- outcome{response, err}
		}(i)
	}
	wg.Wait()
	close(outcomes)
	final, ok, err := loadSharedGobRedConfirmation("2026-10-14")
	if err != nil || !ok {
		t.Fatalf("missing confirmation: %v", err)
	}
	finalJSON, _ := json.Marshal(final)
	for result := range outcomes {
		if result.err != nil {
			t.Fatal(result.err)
		}
		b, _ := json.Marshal(result.response)
		if string(b) != string(finalJSON) {
			t.Fatal("concurrent writers observed different confirmed lists")
		}
	}
}

func TestSharedGobRedEmptyConfirmationIsRetainedAndNextDayWaitsForAuction(t *testing.T) {
	isolateSharedGobConfirmation(t)
	snapshot := confirmationFixture(t, "2026-10-14", "")
	if _, err := saveSharedGobRedConfirmation(snapshot); err != nil {
		t.Fatal(err)
	}
	if _, err := saveSharedGobRedConfirmation(confirmationFixture(t, snapshot.Date, "九阳股份")); err != nil {
		t.Fatal(err)
	}
	saved, _, _ := loadSharedGobRedConfirmation(snapshot.Date)
	if saved["count"] != float64(0) {
		t.Fatal("empty confirmed selection changed")
	}
	r := httptest.NewRecorder()
	handleSharedGobRedSelectionAt(r, "", time.Date(2026, 10, 15, 9, 24, 59, 0, chinaLocation()))
	var body map[string]interface{}
	json.Unmarshal(r.Body.Bytes(), &body)
	if body["date"] != "2026-10-15" || body["status"] != "awaiting_auction" {
		t.Fatalf("next day reused yesterday: %s", r.Body.String())
	}
	r = httptest.NewRecorder()
	handleSharedGobRedSelectionAt(r, "2026-10-14", time.Date(2026, 10, 15, 9, 25, 0, 0, chinaLocation()))
	body = nil
	json.Unmarshal(r.Body.Bytes(), &body)
	if body["archived"] != true || body["count"] != float64(0) {
		t.Fatalf("confirmed prior day was not shown as its own history: %s", r.Body.String())
	}
	called := ""
	t0PrewarmStarter = func(date string) (bool, t0WarmProgress) {
		called = date
		return true, t0WarmProgress{Status: t0WarmStatusWarming}
	}
	r = httptest.NewRecorder()
	handleSharedGobRedSelectionAt(r, "", time.Date(2026, 10, 15, 9, 25, 0, 0, chinaLocation()))
	if called != "2026-10-15" {
		t.Fatalf("next day did not prepare new data: %s", r.Body.String())
	}
	if _, ok, _ := loadSharedGobRedConfirmation("2026-10-15"); ok {
		t.Fatal("incomplete history was frozen")
	}
}

func TestSharedGobRedCorruptConfirmationFailsWithoutOverwriting(t *testing.T) {
	isolateSharedGobConfirmation(t)
	ensureT0CacheDirs()
	path := sharedGobRedConfirmationPath("2026-10-14")
	os.WriteFile(path, []byte("broken"), 0600)
	r := httptest.NewRecorder()
	handleSharedGobRedSelectionAt(r, "2026-10-14", time.Date(2026, 10, 14, 10, 0, 0, 0, chinaLocation()))
	if r.Code != 503 {
		t.Fatalf("corrupt confirmation was ignored: %s", r.Body.String())
	}
	b, _ := os.ReadFile(path)
	if string(b) != "broken" {
		t.Fatal("corrupt evidence overwritten")
	}
}

func writeStaleSharedGobFixture(t *testing.T) (string, string) {
	t.Helper()
	today := time.Now().In(chinaLocation()).Format("2006-01-02")
	day, _ := time.Parse("2006-01-02", today)
	prior := day.AddDate(0, 0, -1).Format("2006-01-02")
	sharedGobPriorDateFetcher = func(string) (string, error) { return prior, nil }
	a, b := day.AddDate(0, 0, -3).Format("2006-01-02"), day.AddDate(0, 0, -2).Format("2006-01-02")
	bar := func(date string) dailyBar {
		return dailyBar{Date: date, Open: 10, High: 11, Low: 9, Close: 10, AmountYi: 8, AmountVerified: true}
	}
	// Keep the already-current stock out of the unrelated after-close T0 refresh.
	err := writeMainboardFloatCap50Gob(mainboardSharedGobPath(), MainboardFloatCap50Gob{SchemaVersion: 2, ClosedDailyDates: map[string]string{"002242": prior}, Stocks: []MainboardFloatCap50Stock{{Code: "605287", Name: "德才股份", FloatMarketCapYi: 100}, {Code: "002242", Name: "九阳股份", FloatMarketCapYi: 100}}, SelectionPools: map[string][]MainboardT0PoolStock{today: {{Code: "605287", Name: "德才股份", MarketCapYi: 100}, {Code: "002242", Name: "九阳股份", MarketCapYi: 100}}}, Daily: map[string][]dailyBar{"605287": {bar(a), bar(b)}, "002242": {bar(b), bar(prior), bar(today)}}})
	if err != nil {
		t.Fatal(err)
	}
	return today, prior
}

func TestSharedGobDailyRepairsStaleHistoryEvenWithManyCachedBars(t *testing.T) {
	isolateSharedGobConfirmation(t)
	today, prior := writeStaleSharedGobFixture(t)
	fetched := []string{}
	sharedGobDailyFetcher = func(stocks []t0Stock, end string, _ func()) map[string][]dailyBar {
		out := map[string][]dailyBar{}
		for _, s := range stocks {
			fetched = append(fetched, s.ShortCode)
			out[s.ShortCode] = []dailyBar{{Date: prior, Open: 10, Close: 10, AmountYi: 8, AmountVerified: true}}
		}
		return out
	}
	_, daily, _, err := loadOrFetchSharedGobT0Daily(today)
	if err != nil {
		t.Fatal(err)
	}
	if len(fetched) != 1 || fetched[0] != "605287" {
		t.Fatalf("stale history not repaired selectively: %v", fetched)
	}
	if len(daily["605287"]) != 3 || daily["605287"][2].Date != prior {
		t.Fatalf("full history lost or stale: %+v", daily["605287"])
	}
	if !sharedGobRedHistoryReady(today) {
		t.Fatal("repaired history is not ready")
	}
	// Reload disk independently of the current process's cached GOB.
	mainboardSharedGob = nil
	if !sharedGobRedHistoryReady(today) {
		t.Fatal("repair not persisted")
	}
}

func TestSharedGobDailyRejectsFailedPreviousDayRepair(t *testing.T) {
	isolateSharedGobConfirmation(t)
	today, _ := writeStaleSharedGobFixture(t)
	sharedGobDailyFetcher = func([]t0Stock, string, func()) map[string][]dailyBar { return nil }
	if _, _, _, err := loadOrFetchSharedGobT0Daily(today); err == nil {
		t.Fatal("stale history admitted as ready")
	}
	if sharedGobRedHistoryReady(today) {
		t.Fatal("failed repair considered complete")
	}
	if _, ok, _ := loadSharedGobRedConfirmation(today); ok {
		t.Fatal("stale history was confirmed")
	}
}

func TestSharedGobRedMissingAuctionDoesNotFreezeAndCompleteEmptyResultDoes(t *testing.T) {
	if isT0WeekendDate(time.Now().In(chinaLocation()).Format("2006-01-02")) {
		t.Skip("formal selection only runs on weekdays")
	}
	isolateSharedGobConfirmation(t)
	today, prior := writeStaleSharedGobFixture(t)
	sharedGobDailyFetcher = func(stocks []t0Stock, _ string, _ func()) map[string][]dailyBar {
		out := map[string][]dailyBar{}
		for _, s := range stocks {
			out[s.ShortCode] = []dailyBar{{Date: prior, Open: 10, Close: 10, High: 10, Low: 10, AmountYi: 8, AmountVerified: true}}
		}
		return out
	}
	if _, _, _, err := loadOrFetchSharedGobT0Daily(today); err != nil {
		t.Fatal(err)
	}
	calls := 0
	sharedGobRedQuoteBuilder = func(string, []t0Stock, map[string][]dailyBar) (map[string]t0Realtime, string, *t0DataWarning) {
		calls++
		return nil, "realtime_open", &t0DataWarning{Code: "T0_AUCTION_DATA_INCOMPLETE", Missing: 1}
	}
	// Use the actual current date/time for the full selection pipeline.
	now := time.Now().In(chinaLocation())
	now = time.Date(now.Year(), now.Month(), now.Day(), 11, 0, 0, 0, chinaLocation())
	r := httptest.NewRecorder()
	handleSharedGobRedSelectionAt(r, today, now)
	if _, ok, _ := loadSharedGobRedConfirmation(today); ok {
		t.Fatal("missing auction data was frozen")
	}
	var incomplete map[string]interface{}
	json.Unmarshal(r.Body.Bytes(), &incomplete)
	if incomplete["status"] != "warming" {
		t.Fatalf("missing auction did not retry: %s", r.Body.String())
	}
	sharedGobRedQuoteBuilder = func(string, []t0Stock, map[string][]dailyBar) (map[string]t0Realtime, string, *t0DataWarning) {
		calls++
		return map[string]t0Realtime{"600000": {Open: 10, PrevClose: 10, Source: "test"}}, "realtime_open", nil
	}
	r = httptest.NewRecorder()
	handleSharedGobRedSelectionAt(r, today, now)
	frozen, ok, err := loadSharedGobRedConfirmation(today)
	if err != nil || !ok {
		t.Fatalf("complete selection not saved: %v %s", err, r.Body.String())
	}
	if frozen["count"] != float64(0) || frozen["confirmed"] != true {
		t.Fatalf("empty selection invalid: %v", frozen)
	}
	r = httptest.NewRecorder()
	handleSharedGobRedSelectionAt(r, today, now)
	if calls != 2 {
		t.Fatalf("confirmed selection queried quotes again: %d", calls)
	}
}

func TestSharedGobDailyVerifiedSuspensionPreservesStoredPool(t *testing.T) {
	isolateSharedGobConfirmation(t)
	today, _ := writeStaleSharedGobFixture(t)
	sharedGobDailyFetcher = func([]t0Stock, string, func()) map[string][]dailyBar { return nil }
	sharedGobNonTradingHistoryVerifier = func([]t0Stock, map[string][]dailyBar, string, string) map[string]string {
		return map[string]string{"605287": "当日停牌，无竞价开盘"}
	}
	stocks, daily, _, err := loadOrFetchSharedGobT0Daily(today)
	if err != nil || len(stocks) != 1 || stocks[0].ShortCode != "002242" || len(daily) != 1 {
		t.Fatalf("verified suspension not excluded: stocks=%v daily=%v err=%v", stocks, daily, err)
	}
	shared, _ := loadMainboardSharedGob()
	if len(shared.SelectionPools[today]) != 2 {
		t.Fatal("suspension shrank the original dated pool")
	}
	if !sharedGobRedHistoryReady(today) {
		t.Fatal("verified suspension blocked confirmation")
	}
}

func TestSharedGobHistoryReadinessDoesNotBlockOnBenchmarkDownload(t *testing.T) {
	isolateSharedGobConfirmation(t)
	entered, release, done := make(chan struct{}), make(chan struct{}), make(chan struct{})
	sharedGobPriorDateFetcher = func(string) (string, error) { close(entered); <-release; return "2026-10-13", nil }
	go func() { sharedGobRequiredPriorDate("2026-10-14"); close(done) }()
	<-entered
	ready := make(chan bool, 1)
	go func() { ready <- sharedGobRedHistoryReady("2026-10-14") }()
	select {
	case result := <-ready:
		if result {
			t.Error("unverified history was ready")
		}
	case <-time.After(time.Second):
		t.Error("HTTP readiness blocked on network")
	}
	close(release)
	<-done
}

func TestSharedGobDateListIncludesTodayBeforeAuction(t *testing.T) {
	dates := sharedGobRedDatesDescendingAt(&mainboardSharedGobData{Dates: []string{"2026-10-14"}}, time.Date(2026, 10, 15, 8, 0, 0, 0, chinaLocation()))
	if len(dates) != 2 || dates[0] != "2026-10-15" {
		t.Fatalf("tomorrow follows old list before auction: %v", dates)
	}
}

func addUnconfirmedPreviousDayBar(t *testing.T, today, prior string) {
	t.Helper()
	shared, err := loadMainboardSharedGob()
	if err != nil {
		t.Fatal(err)
	}
	snapshot := shared.Snapshot
	snapshot.Daily["605287"] = append(snapshot.Daily["605287"], dailyBar{Date: prior, Open: 10, High: 12, Low: 9, Close: 123, AmountYi: 8, AmountVerified: true})
	if err := writeMainboardFloatCap50Gob(mainboardSharedGobPath(), snapshot); err != nil {
		t.Fatal(err)
	}
	mainboardSharedGob = nil
}

func TestSharedGobDailyReplacesIntradayPreviousBarBeforeConfirmation(t *testing.T) {
	isolateSharedGobConfirmation(t)
	today, prior := writeStaleSharedGobFixture(t)
	addUnconfirmedPreviousDayBar(t, today, prior)
	sharedGobDailyFetcher = func(stocks []t0Stock, _ string, _ func()) map[string][]dailyBar {
		if len(stocks) != 1 || stocks[0].ShortCode != "605287" {
			t.Fatalf("wrong repair set: %v", stocks)
		}
		return map[string][]dailyBar{"605287": {{Date: prior, Open: 10, High: 12, Low: 9, Close: 11, AmountYi: 8, AmountVerified: true}}}
	}
	_, daily, _, err := loadOrFetchSharedGobT0Daily(today)
	if err != nil {
		t.Fatal(err)
	}
	bars := daily["605287"]
	if bars[len(bars)-1].Close != 11 {
		t.Fatal("intraday candle became T-1")
	}
	shared, _ := loadMainboardSharedGob()
	if shared.Snapshot.ClosedDailyDates["605287"] != prior {
		t.Fatal("closing witness not saved")
	}
}

func TestSharedGobDailyDoesNotFinalizeCachedIntradayBarOnStaleFetch(t *testing.T) {
	isolateSharedGobConfirmation(t)
	today, prior := writeStaleSharedGobFixture(t)
	addUnconfirmedPreviousDayBar(t, today, prior)
	day, _ := time.Parse("2006-01-02", prior)
	stale := day.AddDate(0, 0, -1).Format("2006-01-02")
	sharedGobDailyFetcher = func([]t0Stock, string, func()) map[string][]dailyBar {
		return map[string][]dailyBar{"605287": {{Date: stale, Close: 10}}}
	}
	if _, _, _, err := loadOrFetchSharedGobT0Daily(today); err == nil {
		t.Fatal("old cached partial bar was falsely finalized by merging stale data")
	}
	if sharedGobRedHistoryReady(today) {
		t.Fatal("unconfirmed closing bar was ready")
	}
}

func TestSharedGobCloseRefreshRevisitsExistingIntradayBars(t *testing.T) {
	date := "2026-10-14"
	shared := &mainboardSharedGobData{Snapshot: MainboardFloatCap50Gob{Stocks: []MainboardFloatCap50Stock{{Code: "605287"}, {Code: "002242"}}, ClosedDailyDates: map[string]string{"002242": date}, SelectionPools: map[string][]MainboardT0PoolStock{date: {{Code: "600003"}}}}, Daily: map[string][]dailyBar{"605287": {{Date: date}}, "002242": {{Date: date}}, "600003": {{Date: date}}}}
	pending := mainboardSharedGobPendingIndices(shared, date)
	if len(pending) != 1 || pending[0] != 0 {
		t.Fatalf("existing intraday bar skipped after close: %v", pending)
	}
	extra := mainboardSharedGobPendingSelectionPool(shared, date)
	if len(extra) != 1 || extra[0].Code != "600003" {
		t.Fatalf("intraday bar for new pool stock skipped: %v", extra)
	}
}
