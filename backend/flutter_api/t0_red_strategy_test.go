package flutter_api

import (
	"encoding/gob"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"reflect"
	"sync"
	"testing"
	"time"

	"github.com/glebarez/sqlite"
	"go-stock/backend/analysis/t0reference"
	"go-stock/backend/db"
	"go-stock/backend/models"
	"gorm.io/gorm"
)

func TestFilterRedT0ResultsKeepsLegacyDisplayRulesAlongsideTechBlue(t *testing.T) {
	ctx := &t0ModuleSelectionContext{
		TradeDate: "2026-09-09",
		Daily: map[string][]dailyBar{
			"600000": {
				{Date: "2026-09-01", Close: 10},
				{Date: "2026-09-02", Open: 10, Close: 10, High: 10.2, Low: 9.8},
				{Date: "2026-09-03", Open: 10, Close: 11, High: 11, Low: 10},
				{Date: "2026-09-04", Open: 9.9, Close: 9.9, High: 9.9, Low: 9.8},
			},
			"600001": {
				{Date: "2026-09-01", Close: 10},
				{Date: "2026-09-02", Open: 10, Close: 10.2, High: 10.2, Low: 10},
				{Date: "2026-09-03", Open: 10.2, Close: 10.1, High: 10.3, Low: 10},
			},
			"600002": {
				{Date: "2026-09-01", Close: 10},
				{Date: "2026-09-02", Open: 10, Close: 11, High: 11, Low: 10},
				{Date: "2026-09-03", Open: 9.9, Close: 9.9, High: 9.9, Low: 9.8},
			},
		},
	}
	results := []T0SelectionResult{
		// 不满足科技蓝，也没有其他红策命中，应该剔除。
		{StockCode: "600001.XSHG", PrevClose: 10, PatternWinPct: 20, PatternFailPct: 50},
		// 不满足科技蓝，但命中任意涨停＋跌停，应该保留。
		{StockCode: "600002.XSHG", PrevClose: 0, PatternWinPct: 0, PatternFailPct: 100},
	}

	got := filterRedT0Results(results, ctx)
	codes := make([]string, 0, len(got))
	for _, result := range got {
		codes = append(codes, result.StockCode)
	}
	want := []string{"600002.XSHG"}
	if !reflect.DeepEqual(codes, want) {
		t.Fatalf("red codes = %v, want %v", codes, want)
	}
}

func TestFilterRedT0ResultsKeepsLegacyTwoLimitUpRuleAndMarksTechBlueSubset(t *testing.T) {
	ctx := &t0ModuleSelectionContext{
		TradeDate: "2026-09-04",
		Daily: map[string][]dailyBar{
			"600000": {
				{Date: "2026-09-01", Close: 10},
				{Date: "2026-09-02", Open: 10.2, Close: 11, High: 11, Low: 10.2},
				{Date: "2026-09-03", Open: 12.1, Close: 12.1, High: 12.1, Low: 11.4},
			},
			"600001": {
				{Date: "2026-09-01", Close: 10},
				{Date: "2026-09-02", Open: 10, Close: 11, High: 11, Low: 10},
				{Date: "2026-09-03", Open: 11.1, Close: 12.1, High: 12.1, Low: 11.1},
			},
		},
	}
	results := []T0SelectionResult{
		{StockCode: "600000.XSHG", OpenGap: 1.2, PatternWinPct: 20, PatternFailPct: 50},
		{StockCode: "600001.XSHG", OpenGap: 1.2, PatternWinPct: 20, PatternFailPct: 50},
	}

	got := filterRedT0Results(results, ctx)
	if len(got) != 2 {
		t.Fatalf("red result count=%d want 2", len(got))
	}
	if !got[0].StrongDisplayRuleHit {
		t.Fatalf("exact two-limit-up non-one-word combination should be deep red: %+v", got[0])
	}
	if !got[0].TechBlueDisplayRuleHit {
		t.Fatalf("complete red admission should be marked tech blue: %+v", got[0])
	}
	if got[1].StrongDisplayRuleHit || got[1].TechBlueDisplayRuleHit {
		t.Fatalf("legacy two-limit-up result should remain ordinary red: %+v", got[1])
	}
}

func TestFilterRedT0ResultsKeepsAnyLimitUpLimitDownWithoutTechBlue(t *testing.T) {
	ctx := &t0ModuleSelectionContext{
		TradeDate: "2026-09-04",
		Daily: map[string][]dailyBar{
			"600000": {
				{Date: "2026-09-01", Close: 10},
				{Date: "2026-09-02", Open: 10.2, Close: 11, High: 11, Low: 10.2},
				{Date: "2026-09-03", Open: 12.1, Close: 12.1, High: 12.1, Low: 11.4},
			},
			"600001": {
				{Date: "2026-09-01", Close: 10},
				{Date: "2026-09-02", Open: 10, Close: 11, High: 11, Low: 10},
				{Date: "2026-09-03", Open: 11.4, Close: 12.1, High: 12.1, Low: 11.4},
			},
			"600002": {
				{Date: "2026-09-01", Close: 10},
				{Date: "2026-09-02", Open: 10, Close: 11, High: 11, Low: 10},
				{Date: "2026-09-03", Open: 9.9, Close: 9.9, High: 9.9, Low: 9.8},
			},
		},
	}

	got := filterRedT0Results([]T0SelectionResult{
		{StockCode: "600000.XSHG", OpenGap: 1.2},
		// 旧的涨停＋跌停标红命中，但不是新红策组合。
		{StockCode: "600002.XSHG", OpenGap: 1.2},
	}, ctx)

	if len(got) != 2 || got[0].StockCode != "600000.XSHG" || got[1].StockCode != "600002.XSHG" {
		t.Fatalf("red results = %+v, want tech-blue and legacy any-limit-up-limit-down conditions", got)
	}
}

func TestFilterRedT0ResultsMarksAnyLimitUpLimitDownDeepRed(t *testing.T) {
	ctx := &t0ModuleSelectionContext{
		TradeDate: "2026-09-05",
		Daily: map[string][]dailyBar{
			"600002": {
				{Date: "2026-09-01", Close: 10},
				{Date: "2026-09-02", Open: 10, Close: 10, High: 10.2, Low: 9.8},
				{Date: "2026-09-03", Open: 10, Close: 11, High: 11, Low: 10},
				{Date: "2026-09-04", Open: 9.9, Close: 9.9, High: 9.9, Low: 9.8},
			},
		},
	}

	got := filterRedT0Results([]T0SelectionResult{
		{StockCode: "600002.XSHG", OpenGap: 1.2},
	}, ctx)
	if len(got) != 1 || !got[0].StrongDisplayRuleHit {
		t.Fatalf("any-limit-up-limit-down combination should enter red and be deep red: %+v", got)
	}
}

func TestFilterRedT0ResultsRejectsExplicitDeepRedReferencePattern(t *testing.T) {
	previous := db.Dao
	t.Cleanup(func() { db.Dao = previous })
	dao, err := gorm.Open(sqlite.Open(filepath.Join(t.TempDir(), "reference.db")), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	db.Dao = dao
	if err := dao.AutoMigrate(&models.T0ReferenceRule{}, &models.T0ReferenceRuleStat{}); err != nil {
		t.Fatal(err)
	}
	rule, err := modelsRuleForTestName("CUSTOM_SELECTED_PATTERN", `{"sequence":["ZT","DYIN","DT"]}`)
	if err != nil {
		t.Fatal(err)
	}
	rule.DeepRed = true
	if err := dao.Create(&rule).Error; err != nil {
		t.Fatal(err)
	}
	if err := dao.Create(&models.T0ReferenceRuleStat{
		RuleID: rule.ID, BatchID: "batch", PeriodKey: "all", SampleCount: 8,
		ProfitWinRate: 75, ResearchTier: t0reference.ResearchTierInsufficient,
	}).Error; err != nil {
		t.Fatal(err)
	}

	ctx := &t0ModuleSelectionContext{
		TradeDate: "2026-01-08",
		Daily: map[string][]dailyBar{
			"600001": {
				{Date: "2026-01-02", Open: 10, Close: 10, High: 10, Low: 10},
				{Date: "2026-01-05", Open: 10.5, Close: 11, High: 11, Low: 10.5},
				{Date: "2026-01-06", Open: 10.8, Close: 10.05, High: 10.8, Low: 10.05},
				{Date: "2026-01-07", Open: 9.5, Close: 9, High: 9.5, Low: 9},
			},
		},
	}
	got := filterRedT0Results([]T0SelectionResult{
		{StockCode: "600001.XSHG", OpenGap: 1},
	}, ctx)
	if len(got) != 0 {
		t.Fatalf("selected deep-red reference pattern should not enter red: %+v", got)
	}
}

func TestSelectPurpleT0ResultsUsesTrueEarnRateWithoutPriceFloor(t *testing.T) {
	results := []T0SelectionResult{
		{StockCode: "600000.XSHG", PrevClose: 11, PatternT0N: 3, PatternWinPct: 0, PatternFailPct: 30},
		{StockCode: "600001.XSHG", PrevClose: 10, PatternT0N: 3, PatternWinPct: 100, PatternFailPct: 30.01},
		{StockCode: "600002.XSHG", PrevClose: 9.99, PatternT0N: 3, PatternWinPct: 0, PatternFailPct: 0},
	}

	got, err := selectT0ResultsForModule("radar.purple_strategy", results, nil)
	if err != nil {
		t.Fatal(err)
	}
	codes := make([]string, 0, len(got))
	for _, result := range got {
		codes = append(codes, result.StockCode)
	}
	want := []string{"600000.XSHG", "600002.XSHG"}
	if !reflect.DeepEqual(codes, want) {
		t.Fatalf("purple codes = %v, want %v", codes, want)
	}
}

func TestSelectPurpleT0ResultsDoesNotRequireDailyContext(t *testing.T) {
	results := []T0SelectionResult{{
		StockCode: "600000.XSHG", PatternT0N: 3, PatternFailPct: 30,
	}}
	selected, err := selectT0ResultsForModule("radar.purple_strategy", results, nil)
	if err != nil || len(selected) != 1 {
		t.Fatalf("purple selection without context = %#v, err=%v", selected, err)
	}
}

func TestSelectGoldT0ResultsFiltersAndSortsByCompositeScore(t *testing.T) {
	results := []T0SelectionResult{
		// 强金策优先：真赚率 85%，综合分 64。
		{StockCode: "600000.XSHG", PatternT0N: 20, PatternWinPct: 50, PatternFailPct: 15},
		// 普通金策即使综合分更高，也排在强金策后面。
		{StockCode: "600001.XSHG", PatternT0N: 40, PatternWinPct: 70, PatternFailPct: 36},
		// 样本数不足。
		{StockCode: "600002.XSHG", PatternT0N: 19, PatternWinPct: 90, PatternFailPct: 0},
		// 真赚率不足 58%。
		{StockCode: "600003.XSHG", PatternT0N: 20, PatternWinPct: 90, PatternFailPct: 42.01},
		// 达标率不足 20%。
		{StockCode: "600004.XSHG", PatternT0N: 20, PatternWinPct: 19.99, PatternFailPct: 0},
	}

	got, err := selectT0ResultsForModule("radar.gold_strategy", results, nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 2 {
		t.Fatalf("gold result count=%d want 2: %+v", len(got), got)
	}
	if got[0].StockCode != "600000.XSHG" || got[1].StockCode != "600001.XSHG" {
		t.Fatalf("gold result order=%v", codes(got))
	}
	if got[0].PatternScore != 64 || got[1].PatternScore != 67.6 {
		t.Fatalf("gold scores = %.2f, %.2f; want 64, 67.6", got[0].PatternScore, got[1].PatternScore)
	}
	if !got[0].StrongGoldSignal || got[1].StrongGoldSignal {
		t.Fatalf("gold strong flags = %v, %v; want true, false", got[0].StrongGoldSignal, got[1].StrongGoldSignal)
	}
}

func TestHandleT0SelectionArchivedPurpleUsesTrueEarnRate(t *testing.T) {
	orig := t0CacheRootPath
	t0CacheRootPath = t.TempDir()
	t.Cleanup(func() { t0CacheRootPath = orig })

	date := "2026-09-10"
	if err := saveT0SelectionArchive(date, []T0SelectionResult{
		{StockCode: "600000.XSHG", PatternT0N: 3, PatternWinPct: 0, PatternFailPct: 30, BuySignal: BuySignalGreen},
		{StockCode: "600001.XSHG", PatternT0N: 3, PatternWinPct: 100, PatternFailPct: 30.01, BuySignal: BuySignalGreen},
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

func TestPrewarmCandidatesPurpleUseTrueEarnRate(t *testing.T) {
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
	// 重新写回切片，给预热候选补上满足紫策条件的统计字段。
	candidatesForTest := response["candidates"].([]T0SelectionResult)
	for i := range candidatesForTest {
		candidatesForTest[i].PatternT0N = 3
		candidatesForTest[i].PatternWinPct = 0
		candidatesForTest[i].PatternFailPct = 30
	}
	candidatesForTest[1].PatternFailPct = 30.01
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
				{Date: "2026-09-01", Close: 10},
				{Date: "2026-09-02", Open: 10, Close: 11, High: 11, Low: 10},
				{Date: "2026-09-03", Open: 11, Close: 12.1, High: 12.1, Low: 11},
				{Date: "2026-09-08", Open: 13.1, Close: 13.31, High: 13.31, Low: 12.1},
			},
		},
	}
	results := []T0SelectionResult{
		{
			StockCode:      "600000.XSHG",
			OpenGap:        1.2,
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
	if got[0].Tag != "" {
		t.Fatalf("red tag = %q, want empty tag for a complete two-limit-up admission", got[0].Tag)
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

func TestLoadT0ModuleSelectionContextForResultsUsesResultScopedDailyCache(t *testing.T) {
	origRoot := t0CacheRootPath
	origFetcher := redT0DailyKLineFetcher
	t0CacheRootPath = t.TempDir()
	t.Cleanup(func() {
		t0CacheRootPath = origRoot
		redT0DailyKLineFetcher = origFetcher
	})

	date := "2026-07-14"
	if err := ensureT0CacheDirs(); err != nil {
		t.Fatal(err)
	}
	f, err := os.Create(t0DailyCachePath(date))
	if err != nil {
		t.Fatal(err)
	}
	payload := &t0DailyCachePayload{
		DataSchemaVersion: t0DailyCacheSchemaVersion,
		TradeDate:         date,
		Stocks:            []t0Stock{{Code: "sz000811", ShortCode: "000811", Name: "结果股"}},
		Daily: map[string][]dailyBar{
			"000811": {
				{Date: "2026-07-10", Close: 30},
				{Date: "2026-07-13", Close: 32},
			},
		},
		StockPoolComplete: false,
	}
	if err := gob.NewEncoder(f).Encode(payload); err != nil {
		_ = f.Close()
		t.Fatal(err)
	}
	if err := f.Close(); err != nil {
		t.Fatal(err)
	}

	fetchCalls := 0
	redT0DailyKLineFetcher = func(shortCode, endDate string, limit int) []dailyBar {
		fetchCalls++
		return nil
	}

	ctx, err := loadT0ModuleSelectionContextForResults(
		redT0StrategyModuleCode,
		date,
		[]T0SelectionResult{{StockCode: "000811.XSHE"}},
	)
	if err != nil {
		t.Fatal(err)
	}
	if fetchCalls != 0 {
		t.Fatalf("result-scoped cache miss triggered live fetch %d times", fetchCalls)
	}
	if ctx == nil || len(ctx.Daily["000811"]) != 2 {
		t.Fatalf("result-scoped cached daily data = %+v", ctx)
	}
}

func TestLoadT0ModuleSelectionContextForResultsRejectsMissingResultDaily(t *testing.T) {
	origRoot := t0CacheRootPath
	origFetcher := redT0DailyKLineFetcher
	t0CacheRootPath = t.TempDir()
	t.Cleanup(func() {
		t0CacheRootPath = origRoot
		redT0DailyKLineFetcher = origFetcher
	})

	redT0DailyKLineFetcher = func(shortCode, endDate string, limit int) []dailyBar {
		if shortCode == "600000" {
			return []dailyBar{
				{Date: "2026-07-10", Close: 30},
				{Date: "2026-07-13", Close: 32},
			}
		}
		return nil
	}

	ctx, err := loadT0ModuleSelectionContextForResults(
		redT0StrategyModuleCode,
		"2026-07-14",
		[]T0SelectionResult{
			{StockCode: "600000.XSHG"},
			{StockCode: "000001.XSHE"},
		},
	)
	if err == nil || ctx != nil {
		t.Fatalf("missing result daily data should be rejected: ctx=%+v err=%v", ctx, err)
	}
}

func TestHandleT0SelectionArchivedPurpleDoesNotRequireDailyData(t *testing.T) {
	origRoot := t0CacheRootPath
	t0CacheRootPath = t.TempDir()
	t.Cleanup(func() { t0CacheRootPath = origRoot })

	date := "2026-07-15"
	if err := saveT0SelectionArchive(date, []T0SelectionResult{
		{StockCode: "600000.XSHG", PatternT0N: 3, PatternFailPct: 30, BuySignal: BuySignalGreen},
	}, true); err != nil {
		t.Fatal(err)
	}

	rr := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet,
		"/api/t0-selection?module_code="+purpleT0StrategyModuleCode+"&archived=1&date="+date, nil)
	handleT0Selection(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("status %d body %s", rr.Code, rr.Body.String())
	}
	var body map[string]interface{}
	if err := json.Unmarshal(rr.Body.Bytes(), &body); err != nil {
		t.Fatal(err)
	}
	if body["code"] != nil || body["data_incomplete"] != nil || body["count"] != float64(1) {
		t.Fatalf("purple archived response=%s", rr.Body.String())
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
