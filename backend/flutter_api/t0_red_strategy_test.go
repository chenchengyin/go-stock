package flutter_api

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"reflect"
	"sync"
	"testing"
	"time"
)

func TestFilterRedT0Results(t *testing.T) {
	ctx := &t0ModuleSelectionContext{
		TradeDate: "2026-09-09",
		Daily: map[string][]dailyBar{
			"600000": {
				{Date: "2026-09-01", Close: 10},
				{Date: "2026-09-02", Close: 10.5},
			},
			"600001": {
				{Date: "2026-09-01", Close: 10},
				{Date: "2026-09-02", Close: 10.5},
			},
			"600002": {
				{Date: "2026-09-07", Close: 20},
				{Date: "2026-09-08", Close: 20.5},
			},
			"600003": {
				{Date: "2026-09-01", Close: 10},
				{Date: "2026-09-02", Close: 10.5},
			},
			"600004": {
				{Date: "2026-09-01", Close: 10},
				{Date: "2026-09-02", Close: 10.5},
			},
		},
	}
	results := []T0SelectionResult{
		{StockCode: "600000.XSHG", PrevClose: 10, PatternWinPct: 20, PatternFailPct: 50},
		{StockCode: "600001.XSHG", PrevClose: 9.99, PatternWinPct: 20, PatternFailPct: 50},
		{StockCode: "600002.XSHG", PrevClose: 20, PatternWinPct: 20, PatternFailPct: 50},
		{StockCode: "600003.XSHG", PrevClose: 10, PatternWinPct: 20, PatternFailPct: 50.01},
		{StockCode: "600004.XSHG", PrevClose: 10, PatternWinPct: 19.99, PatternFailPct: 50},
		{StockCode: "600005.XSHG", PrevClose: 10, PatternWinPct: 20, PatternFailPct: 50},
	}

	got := filterRedT0Results(results, ctx)
	codes := make([]string, 0, len(got))
	for _, result := range got {
		codes = append(codes, result.StockCode)
	}
	want := []string{"600000.XSHG", "600002.XSHG"}
	if !reflect.DeepEqual(codes, want) {
		t.Fatalf("red codes = %v, want %v", codes, want)
	}
}

func TestSelectPurpleT0ResultsAppliesRedPriceFloor(t *testing.T) {
	ctx := &t0ModuleSelectionContext{
		TradeDate: "2026-09-10",
		Daily: map[string][]dailyBar{
			"600000": {
				{Date: "2026-07-01", Close: 10},
				{Date: "2026-09-09", Close: 11},
			},
			"600001": {
				{Date: "2026-07-01", Close: 10},
				{Date: "2026-09-09", Close: 10},
			},
			"600002": {
				{Date: "2026-07-01", Close: 10},
				{Date: "2026-09-09", Close: 9.99},
			},
		},
	}
	results := []T0SelectionResult{
		{StockCode: "600000.XSHG", PrevClose: 11, PatternT0N: 2, PatternWinPct: 30, PatternFailPct: 39},
		{StockCode: "600001.XSHG", PrevClose: 10, PatternT0N: 2, PatternWinPct: 30, PatternFailPct: 39},
		{StockCode: "600002.XSHG", PrevClose: 9.99, PatternT0N: 2, PatternWinPct: 30, PatternFailPct: 39},
	}

	got, err := selectT0ResultsForModule("radar.purple_strategy", results, ctx)
	if err != nil {
		t.Fatal(err)
	}
	codes := make([]string, 0, len(got))
	for _, result := range got {
		codes = append(codes, result.StockCode)
	}
	want := []string{"600000.XSHG", "600001.XSHG"}
	if !reflect.DeepEqual(codes, want) {
		t.Fatalf("purple codes = %v, want %v", codes, want)
	}
}

func TestSelectPurpleT0ResultsRequiresDailyContext(t *testing.T) {
	results := []T0SelectionResult{{StockCode: "600000.XSHG"}}
	selected, err := selectT0ResultsForModule("radar.purple_strategy", results, nil)
	if err == nil || selected != nil {
		t.Fatalf("purple selection without context = %#v, err=%v", selected, err)
	}
}

func TestHandleT0SelectionArchivedPurpleAppliesPriceFloor(t *testing.T) {
	orig := t0CacheRootPath
	t0CacheRootPath = t.TempDir()
	t.Cleanup(func() { t0CacheRootPath = orig })

	date := "2026-09-10"
	stocks := []t0Stock{
		{Code: "sh600000", ShortCode: "600000", Name: "价格通过股"},
		{Code: "sh600001", ShortCode: "600001", Name: "价格不通过股"},
	}
	daily := map[string][]dailyBar{
		"600000": {{Date: "2026-07-01", Close: 10}, {Date: "2026-09-09", Close: 11}},
		"600001": {{Date: "2026-07-01", Close: 10}, {Date: "2026-09-09", Close: 9.99}},
	}
	if err := saveT0DailyCache(date, stocks, daily); err != nil {
		t.Fatal(err)
	}
	if err := saveT0SelectionArchive(date, []T0SelectionResult{
		{StockCode: "600000.XSHG", PrevClose: 11, PatternT0N: 2, PatternWinPct: 30, PatternFailPct: 39, BuySignal: BuySignalGreen},
		{StockCode: "600001.XSHG", PrevClose: 9.99, PatternT0N: 2, PatternWinPct: 30, PatternFailPct: 39, BuySignal: BuySignalGreen},
	}, true); err != nil {
		t.Fatal(err)
	}

	req := httptest.NewRequest(http.MethodGet,
		"/api/t0-selection?module_code=radar.purple_strategy&archived=1&date="+date, nil)
	rr := httptest.NewRecorder()
	handleT0Selection(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("status %d body %s", rr.Code, rr.Body.String())
	}
	var body struct {
		Count   int                 `json:"count"`
		Results []T0SelectionResult `json:"results"`
	}
	if err := json.Unmarshal(rr.Body.Bytes(), &body); err != nil {
		t.Fatal(err)
	}
	if body.Count != 1 || len(body.Results) != 1 || body.Results[0].StockCode != "600000.XSHG" {
		t.Fatalf("purple archived response=%+v", body)
	}
}

func TestPrewarmCandidatesPurpleApplyPriceFloor(t *testing.T) {
	tradeDate := "2026-09-11"
	cached := &t0DailyCachePayload{
		TradeDate: tradeDate,
		Stocks: []t0Stock{
			{Code: "sh600000", ShortCode: "600000", Name: "价格通过股"},
			{Code: "sh600001", ShortCode: "600001", Name: "价格不通过股"},
		},
		Daily: map[string][]dailyBar{
			"600000": {
				{Date: "2026-07-01", Close: 10},
				{Date: "2026-09-09", Close: 10},
				{Date: "2026-09-10", Close: 11, AmountYi: 10},
				{Date: tradeDate, Open: 11.11, Close: 11.11},
			},
			"600001": {
				{Date: "2026-07-01", Close: 12},
				{Date: "2026-09-09", Close: 10},
				{Date: "2026-09-10", Close: 11, AmountYi: 10},
				{Date: tradeDate, Open: 11.11, Close: 11.11},
			},
		},
		StockPoolComplete: true,
	}
	response := buildPrewarmReadyResponseAtWithCache(
		tradeDate, time.Date(2026, 9, 11, 9, 10, 0, 0, chinaLocation()), cached)
	// 重新写回切片，给预热候选补上满足紫策既有条件的统计字段。
	candidatesForTest := response["candidates"].([]T0SelectionResult)
	for i := range candidatesForTest {
		candidatesForTest[i].PatternT0N = 2
		candidatesForTest[i].PatternWinPct = 30
		candidatesForTest[i].PatternFailPct = 39
	}
	response["candidates"] = candidatesForTest
	ctx := &t0ModuleSelectionContext{TradeDate: tradeDate, Daily: cached.Daily}
	if err := scopeT0ResponseResultsWithContext(
		purpleT0StrategyModuleCode, response, "candidates", ctx); err != nil {
		t.Fatal(err)
	}
	candidates, ok := response["candidates"].([]T0SelectionResult)
	if !ok || len(candidates) != 1 || candidates[0].StockCode != "600000.XSHG" {
		t.Fatalf("purple prewarm candidates=%+v", response["candidates"])
	}
	if response["candidate_count"] != 1 {
		t.Fatalf("candidate_count=%v", response["candidate_count"])
	}
}

func TestFilterRedT0ResultsRecalculatesTagsFromDaily(t *testing.T) {
	ctx := &t0ModuleSelectionContext{
		TradeDate: "2026-09-09",
		Daily: map[string][]dailyBar{
			"600000": {
				{Date: "2026-09-01", Open: 10, Close: 10, High: 10, Low: 9.8},
				{Date: "2026-09-08", Open: 10.8, Close: 10.5, High: 11, Low: 10.4},
			},
		},
	}
	results := []T0SelectionResult{
		{
			StockCode:      "600000.XSHG",
			PrevClose:      10,
			PatternWinPct:  20,
			PatternFailPct: 50,
			Tag:            "旧标签",
		},
	}

	got := filterRedT0Results(results, ctx)
	if len(got) != 1 {
		t.Fatalf("red result count = %d, want 1", len(got))
	}
	if got[0].Tag != "涨停破板" {
		t.Fatalf("red tag = %q, want 涨停破板", got[0].Tag)
	}
	if results[0].Tag != "旧标签" {
		t.Fatalf("input tag mutated to %q", results[0].Tag)
	}
}

func TestSelectT0ResultsForModuleRedRequiresDailyContext(t *testing.T) {
	results, err := selectT0ResultsForModule("radar.red_strategy", nil, nil)
	if err == nil || results != nil {
		t.Fatalf("red selection without context = %#v, err=%v", results, err)
	}
}

func TestLoadRedT0ContextFetchesMissingDailyKLinesWithoutSavingGob(t *testing.T) {
	origRoot := t0CacheRootPath
	origFetcher := redT0DailyKLineFetcher
	t0CacheRootPath = t.TempDir()
	t.Cleanup(func() {
		t0CacheRootPath = origRoot
		redT0DailyKLineFetcher = origFetcher
	})

	fetched := make(map[string]bool)
	var fetchedMu sync.Mutex
	redT0DailyKLineFetcher = func(shortCode, endDate string, limit int) []dailyBar {
		fetchedMu.Lock()
		fetched[shortCode] = true
		fetchedMu.Unlock()
		if endDate != "2026-09-03" || limit != 60 {
			t.Fatalf("unexpected fetch arguments: code=%s end=%s limit=%d", shortCode, endDate, limit)
		}
		return []dailyBar{
			{Date: "2026-09-01", Close: 10},
			{Date: "2026-09-02", Close: 10.5},
		}
	}

	ctx, err := loadT0ModuleSelectionContextForResults(
		redT0StrategyModuleCode,
		"2026-09-03",
		[]T0SelectionResult{
			{StockCode: "600000.XSHG"},
			{StockCode: "000001.XSHE"},
		},
	)
	if err != nil {
		t.Fatal(err)
	}
	if len(ctx.Daily) != 2 || !fetched["600000"] || !fetched["000001"] {
		t.Fatalf("unexpected fetched daily data: %+v fetched=%v", ctx.Daily, fetched)
	}
	if _, err := os.Stat(t0DailyCachePath("2026-09-03")); !os.IsNotExist(err) {
		t.Fatalf("missing Gob must not be created, stat err=%v", err)
	}
}

func TestWriteScopedRedResponseFiltersAndEnrichesFromDaily(t *testing.T) {
	daily := map[string][]dailyBar{
		"600000": {
			{Date: "2026-09-01", Open: 10, Close: 10, High: 10, Low: 9.8},
			{Date: "2026-09-02", Open: 10, Close: 11, High: 11, Low: 10},
			{Date: "2026-09-03", Open: 11, Close: 12.1, High: 12.1, Low: 11},
			{Date: "2026-09-04", Open: 12.2, Close: 13.2, High: 13.6, Low: 12.1},
		},
		"600001": {
			{Date: "2026-09-01", Close: 10},
			{Date: "2026-09-02", Close: 11},
		},
	}
	ctx := &t0ModuleSelectionContext{
		TradeDate: "2026-09-05",
		Daily:     daily,
	}
	response := map[string]interface{}{
		"date":  "2026-09-05",
		"count": 2,
		"results": []T0SelectionResult{
			{
				StockCode:      "600000.XSHG",
				PrevClose:      10,
				PatternWinPct:  20,
				PatternFailPct: 50,
				Tag:            "旧标签",
			},
			{
				StockCode:      "600001.XSHG",
				PrevClose:      9,
				PatternWinPct:  20,
				PatternFailPct: 50,
			},
		},
	}

	rr := httptest.NewRecorder()
	writeScopedT0ResponseWithContext(
		rr, redT0StrategyModuleCode, response, ctx, "results")
	if rr.Code != 200 {
		t.Fatalf("status=%d body=%s", rr.Code, rr.Body.String())
	}
	var body struct {
		Count   int                 `json:"count"`
		Results []T0SelectionResult `json:"results"`
	}
	if err := json.Unmarshal(rr.Body.Bytes(), &body); err != nil {
		t.Fatal(err)
	}
	if body.Count != 1 || len(body.Results) != 1 {
		t.Fatalf("red response=%+v", body)
	}
	if body.Results[0].Tag != "涨停破板" {
		t.Fatalf("tag=%q", body.Results[0].Tag)
	}
	if !reflect.DeepEqual(body.Results[0].DisplayRuleHits,
		[]string{"涨停＋涨停＋阳线破板"}) {
		t.Fatalf("hits=%v", body.Results[0].DisplayRuleHits)
	}
}
