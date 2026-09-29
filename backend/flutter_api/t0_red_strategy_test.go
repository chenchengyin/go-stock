package flutter_api

import (
	"encoding/gob"
	"encoding/json"
	"math"
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

func TestFilterRedT0ResultsKeepsLegacyDisplayRulesAlongsideStrongContinuation(t *testing.T) {
	ctx := &t0ModuleSelectionContext{
		TradeDate: "2026-09-09",
		Daily: map[string][]dailyBar{
			"600000": {
				{Date: "2026-08-31", Close: 9.5},
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
		// 不满足强势连板，也没有其他红策命中，应该剔除。
		{StockCode: "600001.XSHG", PrevClose: 10, PatternWinPct: 20, PatternFailPct: 50},
		// 不满足强势连板，但命中任意涨停＋跌停，应该保留。
		{StockCode: "600002.XSHG", PrevClose: 0, PatternWinPct: 0, PatternFailPct: 100,
			StrongContinuationDisplayRuleHit: true},
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
	if got[0].StrongContinuationDisplayRuleHit {
		t.Fatal("stale strong-continuation marker should be cleared when the custom rule does not match")
	}
}

func TestFilterRedT0ResultsKeepsStrongContinuationWithoutRemovedLegacyRule(t *testing.T) {
	ctx := &t0ModuleSelectionContext{
		TradeDate: "2026-09-04",
		Daily: map[string][]dailyBar{
			"600000": {
				{Date: "2026-08-31", Close: 9.5},
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
	if len(got) != 1 || got[0].StockCode != "600000.XSHG" {
		t.Fatalf("red result count/codes=%d/%v want 1/[600000.XSHG]", len(got), got)
	}
	if !got[0].StrongContinuationDisplayRuleHit {
		t.Fatalf("complete red admission should be marked strong continuation: %+v", got[0])
	}
	if got[0].StrongDisplayRuleHit {
		t.Fatalf("removed legacy two-limit-up display rule should not mark deep red: %+v", got[0])
	}
}

func TestFilterRedT0ResultsKeepsAnyLimitUpLimitDownWithoutStrongContinuation(t *testing.T) {
	ctx := &t0ModuleSelectionContext{
		TradeDate: "2026-09-04",
		Daily: map[string][]dailyBar{
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
		// 旧的涨停＋跌停标红命中，但不是强势连板组合。
		{StockCode: "600002.XSHG", OpenGap: 1.2},
	}, ctx)

	if len(got) != 1 || got[0].StockCode != "600002.XSHG" {
		t.Fatalf("red results = %+v, want legacy any-limit-up-limit-down condition", got)
	}
}

func TestFilterRedT0ResultsMarksNonOneWordLimitUpLimitDownReversal(t *testing.T) {
	previous := db.Dao
	db.Dao = nil
	t.Cleanup(func() { db.Dao = previous })
	ctx := &t0ModuleSelectionContext{
		TradeDate: "2026-09-04",
		Daily: map[string][]dailyBar{
			"600003": {
				{Date: "2026-08-31", Close: 9.5},
				{Date: "2026-09-01", Close: 10},
				{Date: "2026-09-02", Open: 10.2, Close: 11, High: 11, Low: 10.2},
				{Date: "2026-09-03", Open: 9.9, Close: 9.9, High: 9.9, Low: 9.9},
			},
		},
	}
	result := T0SelectionResult{StockCode: "600003.XSHG", OpenGap: 1.2}
	redEntryHits := matchingRedEntryReferenceRuntimes(
		histBarsBeforeTradeDate(ctx.Daily["600003"], ctx.TradeDate),
		result,
		loadT0ReferenceRuleRuntimes(),
	)
	if len(redEntryHits) < 1 {
		t.Fatalf("red-entry reversal hits = %+v", redEntryHits)
	}
	containsReversal := false
	for _, hit := range redEntryHits {
		if hit.rule.Definition.Name == t0reference.RuleNameLimitUpLimitDownReversal {
			containsReversal = true
			break
		}
	}
	if !containsReversal {
		t.Fatalf("red-entry reversal is missing from hits = %+v", redEntryHits)
	}

	got := filterRedT0Results([]T0SelectionResult{result}, ctx)
	if len(got) != 1 || got[0].StrongContinuationDisplayRuleHit {
		t.Fatalf("reversal result = %+v", got)
	}
	if !reflect.DeepEqual(got[0].DisplayRuleHits, []string{
		t0reference.RuleNameLimitUpLimitDownReversal,
		t0reference.RuleNameAnyLimitUpLimitDown,
	}) {
		t.Fatalf("reversal display hits = %v", got[0].DisplayRuleHits)
	}
}

func TestFilterRedT0ResultsAdmitsEveryBuiltInRedPattern(t *testing.T) {
	cases := []struct {
		name     string
		code     string
		openGap  float64
		history  []dailyBar
		wantRule string
	}{
		{
			name: "任意K线＋涨停＋跌停", code: "600010.XSHG", openGap: 1.2,
			history: []dailyBar{
				{Date: "2026-09-01", Close: 10},
				{Date: "2026-09-02", Open: 10, Close: 10, High: 10.2, Low: 9.8},
				{Date: "2026-09-03", Open: 10, Close: 11, High: 11, Low: 10},
				{Date: "2026-09-04", Open: 9.9, Close: 9.9, High: 9.9, Low: 9.8},
			},
			wantRule: t0reference.RuleNameAnyLimitUpLimitDown,
		},
		{
			name: "中阳/大阳＋跌停", code: "600011.XSHG", openGap: 0.5,
			history: []dailyBar{
				{Date: "2026-09-01", Close: 10},
				{Date: "2026-09-02", Open: 10, Close: 10, High: 10.2, Low: 9.8},
				{Date: "2026-09-03", Open: 10, Close: 10.5, High: 10.5, Low: 10},
				{Date: "2026-09-04", Open: 9.45, Close: 9.45, High: 9.45, Low: 9.45},
			},
			wantRule: t0reference.RuleNameMediumYangLimitDown,
		},
		{
			name: "涨停＋涨停＋阳线破板", code: "600012.XSHG", openGap: 1.2,
			history: []dailyBar{
				{Date: "2026-09-01", Close: 10},
				{Date: "2026-09-02", Open: 10, Close: 11, High: 11, Low: 10},
				{Date: "2026-09-03", Open: 11, Close: 12.1, High: 12.1, Low: 11},
				{Date: "2026-09-04", Open: 12.2, Close: 13.2, High: 13.6, Low: 12.1},
			},
			wantRule: t0reference.RuleNameBullishZtZtPb,
		},
		{
			name: "涨停＋涨停＋普通阴线", code: "600013.XSHG", openGap: 1.2,
			history: []dailyBar{
				{Date: "2026-09-01", Close: 10},
				{Date: "2026-09-02", Open: 10, Close: 11, High: 11, Low: 10},
				{Date: "2026-09-03", Open: 11, Close: 12.1, High: 12.1, Low: 11},
				{Date: "2026-09-04", Open: 12.4, Close: 12.2, High: 12.4, Low: 12.1},
			},
			wantRule: t0reference.RuleNameZtZtBearish,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			ctx := &t0ModuleSelectionContext{
				TradeDate: "2026-09-05",
				Daily:     map[string][]dailyBar{tc.code[:6]: tc.history},
			}
			got := filterRedT0Results([]T0SelectionResult{{
				StockCode: tc.code, OpenGap: tc.openGap,
			}}, ctx)
			if len(got) != 1 {
				t.Fatalf("red results=%+v, want one result", got)
			}
			found := false
			for _, hit := range got[0].DisplayRuleHits {
				if hit == tc.wantRule {
					found = true
					break
				}
			}
			if !found {
				t.Fatalf("display hits=%v want %q", got[0].DisplayRuleHits, tc.wantRule)
			}
		})
	}
}

func TestFilterRedT0ResultsMarksBullishZtZtPbAsTechBlue(t *testing.T) {
	ctx := &t0ModuleSelectionContext{
		TradeDate: "2026-09-05",
		Daily: map[string][]dailyBar{
			"600014": {
				{Date: "2026-09-01", Close: 10},
				{Date: "2026-09-02", Open: 10, Close: 11, High: 11, Low: 10},
				{Date: "2026-09-03", Open: 11, Close: 12.1, High: 12.1, Low: 11},
				{Date: "2026-09-04", Open: 12.2, Close: 13.2, High: 13.6, Low: 12.1},
			},
		},
	}

	got := filterRedT0Results([]T0SelectionResult{{
		StockCode: "600014.XSHG", OpenGap: 1.2,
	}}, ctx)
	if len(got) != 1 {
		t.Fatalf("red results=%+v, want one result", got)
	}
	if !got[0].TechBlueDisplayRuleHit {
		t.Fatalf("bullish ZT+ZT+PB should be tech blue: %+v", got[0])
	}
	if got[0].StrongContinuationDisplayRuleHit {
		t.Fatalf("bullish ZT+ZT+PB must not be mislabeled as strong continuation: %+v", got[0])
	}
}

func TestNonOneWordLimitUpLimitDownReversalRejectsOneWordLimitUp(t *testing.T) {
	hist := []dailyBar{
		{Date: "2026-08-31", Close: 9.5},
		{Date: "2026-09-01", Close: 10},
		{Date: "2026-09-02", Open: 11, Close: 11, High: 11, Low: 11},
		{Date: "2026-09-03", Open: 9.9, Close: 9.9, High: 9.9, Low: 9.9},
	}
	hits := matchingRedEntryReferenceRuntimes(
		hist, T0SelectionResult{OpenGap: 1.2}, loadT0ReferenceRuleRuntimes())
	for _, hit := range hits {
		if hit.rule.Definition.Name == t0reference.RuleNameLimitUpLimitDownReversal {
			t.Fatalf("one-word limit-up should not match reversal: %+v", hits)
		}
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
	if !body.Results[0].TechBlueDisplayRuleHit {
		t.Fatalf("bullish ZT+ZT+PB should remain tech blue after response enrichment: %+v", body.Results[0])
	}
	if !reflect.DeepEqual(body.Results[0].DisplayRuleHits,
		[]string{"涨停＋涨停＋阳线破板"}) {
		t.Fatalf("hits=%v", body.Results[0].DisplayRuleHits)
	}
}

func gobRedTestBarsA() []dailyBar {
	return []dailyBar{
		{Date: "2026-09-07", Open: 10, High: 10.1, Low: 9.9, Close: 10},
		{Date: "2026-09-08", Open: 10.2, High: 11, Low: 10.2, Close: 11},     // 最近收盘涨停 T-4
		{Date: "2026-09-09", Open: 10.8, High: 10.9, Low: 10.3, Close: 10.4}, // MYIN
		{Date: "2026-09-10", Open: 9.5, High: 9.5, Low: 9.36, Close: 9.36},   // DT
		{Date: "2026-09-11", Open: 8.5, High: 8.5, Low: 8.424, Close: 8.424}, // DT
	}
}

func gobRedTestBarsB() []dailyBar {
	bars := gobRedTestBarsA()
	bars[3] = dailyBar{Date: "2026-09-10", Open: 10.2, High: 10.3, Low: 9.8, Close: 9.8} // MYIN
	bars[4] = dailyBar{Date: "2026-09-11", Open: 9.0, High: 9.0, Low: 8.82, Close: 8.82} // DT
	return bars
}

func TestGobRedRuleMatchesTwoApprovedShapesOutsideMainPool(t *testing.T) {
	stocks := []t0Stock{
		{Code: "sh.600101", ShortCode: "600101", Name: "A股"},
		{Code: "sh.600102", ShortCode: "600102", Name: "B股"},
	}
	daily := map[string][]dailyBar{"600101": gobRedTestBarsA(), "600102": gobRedTestBarsB()}
	opens := map[string]t0Realtime{
		"600101": {Open: 8.0, PrevClose: 8.424},
		"600102": {Open: 8.4, PrevClose: 8.82},
	}
	got, status := matchGobRedRules("2026-09-14", stocks, daily, opens, "daily_open", approvedGobRedRules())
	if status.MissingOpen != 0 || len(got) != 2 {
		t.Fatalf("matches=%+v status=%+v; want both shapes", got, status)
	}
	if got[0].StockCode != "600101.XSHG" || got[1].StockCode != "600102.XSHG" {
		t.Fatalf("codes=%v", got)
	}
	if len(got[0].RedRuleHits) != 1 || got[0].RedRuleHits[0].Key != "gob_myin_dt_dt_v1" ||
		got[0].RedRuleHits[0].ExitHorizon != "T+1_close" || got[0].RedRuleHits[0].EntrySource != "daily_open" {
		t.Fatalf("A hit=%+v", got[0].RedRuleHits)
	}
	if len(got[1].RedRuleHits) != 1 || got[1].RedRuleHits[0].Key != "gob_myin_myin_dt_v1" {
		t.Fatalf("B hit=%+v", got[1].RedRuleHits)
	}
}

func TestGobRedRuleUsesLatestCloseLimitUpAndRequiresPullback(t *testing.T) {
	stock := []t0Stock{{Code: "sh.600101", ShortCode: "600101", Name: "A股"}}
	base := gobRedTestBarsA()
	tests := []struct {
		name string
		bars []dailyBar
		want int
	}{
		{"valid", base, 1},
		{"broken-only-anchor", append([]dailyBar(nil), base...), 0},
		{"no-pullback", append([]dailyBar(nil), base...), 0},
		{"wrong-pattern", append([]dailyBar(nil), base...), 0},
	}
	tests[1].bars[1].Close = 10.5 // 最高涨停、收盘未涨停
	tests[2].bars[1].Close = 8    // 不是涨停，因此同样不能借更早锚点
	tests[3].bars[2].Open = 10.41 // T-3 不再是 MYIN
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got, _ := matchGobRedRules("2026-09-14", stock,
				map[string][]dailyBar{"600101": tc.bars},
				map[string]t0Realtime{"600101": {Open: 8, PrevClose: 8.424}},
				"daily_open", approvedGobRedRules())
			if len(got) != tc.want {
				t.Fatalf("matches=%d want=%d", len(got), tc.want)
			}
		})
	}
}

func TestGobRedRuleUsesStrictRawGapAndConfirmedOpen(t *testing.T) {
	stock := []t0Stock{{Code: "sh.600101", ShortCode: "600101", Name: "A股"}}
	daily := map[string][]dailyBar{"600101": gobRedTestBarsA()}
	for _, tc := range []struct {
		gap  float64
		want int
	}{
		{-9.85, 0}, {-9.849, 1}, {-3.001, 1}, {-3, 0}, {-2.999, 0},
	} {
		open := 8.424 * (1 + tc.gap/100)
		got, _ := matchGobRedRules("2026-09-14", stock, daily,
			map[string]t0Realtime{"600101": {Open: open, PrevClose: 8.424}},
			"daily_open", approvedGobRedRules())
		if len(got) != tc.want {
			t.Fatalf("requested gap=%v actual gap=%.17f open=%.17f matches=%d want=%d",
				tc.gap, (open/8.424-1)*100, open, len(got), tc.want)
		}
	}
	got, status := matchGobRedRules("2026-09-14", stock, daily, nil,
		"realtime_open", approvedGobRedRules())
	if len(got) != 0 || status.MissingOpen != 1 {
		t.Fatalf("missing open matched: %+v status=%+v", got, status)
	}
}

func TestGobRedRuleAllowsPerRulePositiveFivePercentOpen(t *testing.T) {
	stock := []t0Stock{{Code: "sh.600101", ShortCode: "600101", Name: "A股"}}
	rules := []redGobRule{{
		Key: "test_plus_five_v1", Name: "测试高开", Version: 1,
		MatchHistory: func([]dailyBar) bool { return true },
		MatchOpen: func(open, prevClose float64) bool {
			return open > prevClose*(1+4.9/100) && open < prevClose*(1+5.1/100)
		},
	}}
	got, _ := matchGobRedRules("2026-09-14", stock,
		map[string][]dailyBar{"600101": gobRedTestBarsA()},
		map[string]t0Realtime{"600101": {Open: 8.424 * 1.05, PrevClose: 8.424}},
		"realtime_open", rules)
	if len(got) != 1 || got[0].RedRuleHits[0].Key != "test_plus_five_v1" {
		t.Fatalf("custom +5%% rule did not match: %+v", got)
	}
}

func TestGobRedRuleChecksDateSpecificRiskWarningLimitDown(t *testing.T) {
	st := t0Stock{Code: "sh.600101", ShortCode: "600101", Name: "*ST测试"}
	for _, tc := range []struct {
		date string
		open float64
		want string
	}{
		{"2026-07-03", 9.5, "limit_down"},
		{"2026-07-06", 9.5, "tradable"},
		{"2026-07-06", 9.0, "limit_down"},
	} {
		if got := gobRedOpenLimitState(st, tc.date, 10, tc.open); got != tc.want {
			t.Fatalf("date=%s open=%v got=%q want=%q", tc.date, tc.open, got, tc.want)
		}
	}
	unknown := t0Stock{Code: "sz.300101", ShortCode: "300101", Name: "测试"}
	if got := gobRedOpenLimitState(unknown, "2026-07-06", 10, 9.5); got != "unknown" {
		t.Fatalf("unknown board limit=%q", got)
	}
}

func TestGobRedSelectionKeepsIndependentHitWhenMainIsEmpty(t *testing.T) {
	independent, _ := matchGobRedRules("2026-09-14",
		[]t0Stock{{Code: "sh.600101", ShortCode: "600101", Name: "独立形态"}},
		map[string][]dailyBar{"600101": gobRedTestBarsA()},
		map[string]t0Realtime{"600101": {Open: 8, PrevClose: 8.424}},
		"daily_open", approvedGobRedRules())
	ctx := &t0ModuleSelectionContext{TradeDate: "2026-09-14",
		Daily: map[string][]dailyBar{"600101": gobRedTestBarsA()}, Independent: independent}
	red, err := selectT0ResultsForModule(redT0StrategyModuleCode, nil, ctx)
	if err != nil || len(red) != 1 || red[0].StockCode != "600101.XSHG" {
		t.Fatalf("red=%+v err=%v", red, err)
	}
	main, err := selectT0ResultsForModule("radar.main_strategy", nil, ctx)
	if err != nil || len(main) != 0 {
		t.Fatalf("main changed: %+v err=%v", main, err)
	}
}

func TestGobRedSelectionMergesDuplicateStockWithoutLosingRuleName(t *testing.T) {
	date := "2026-09-14"
	code := "600101.XSHG"
	hist := []dailyBar{
		{Date: "2026-09-08", Close: 10},
		{Date: "2026-09-09", Open: 10.1, High: 11, Low: 10.1, Close: 11},
		{Date: "2026-09-10", Open: 9.9, High: 9.9, Low: 9.9, Close: 9.9},
	}
	main := []T0SelectionResult{{Time: date, StockCode: code, StockName: "重复股", OpenGap: 1}}
	independent := []T0SelectionResult{{Time: date, StockCode: code, StockName: "重复股",
		DisplayRuleHits: []string{"独立低开"},
		RedRuleHits:     []T0RedRuleHit{{Key: "gob_test_v1", Version: 1, Name: "独立低开", ExitHorizon: "T+1_close"}},
	}}
	ctx := &t0ModuleSelectionContext{TradeDate: date, Daily: map[string][]dailyBar{"600101": hist}, Independent: independent}
	got, err := selectT0ResultsForModule(redT0StrategyModuleCode, main, ctx)
	if err != nil || len(got) != 1 || len(got[0].RedRuleHits) != 1 {
		t.Fatalf("merged=%+v err=%v", got, err)
	}
	if len(got[0].DisplayRuleHits) < 2 || got[0].DisplayRuleHits[len(got[0].DisplayRuleHits)-1] != "独立低开" {
		t.Fatalf("did not preserve both names: %+v", got[0].DisplayRuleHits)
	}
}

func TestGobRedLiveSelectionSurvivesEmptyMainFilter(t *testing.T) {
	oldRoot := t0CacheRootPath
	t0CacheRootPath = t.TempDir()
	t.Cleanup(func() { t0CacheRootPath = oldRoot })
	date := "2026-09-14"
	if err := saveT0DailyCache("2026-09-11",
		[]t0Stock{{Code: "sh.600101", ShortCode: "600101", Name: "独立形态"}},
		map[string][]dailyBar{"600101": gobRedTestBarsA()}); err != nil {
		t.Fatal(err)
	}
	bars := append(gobRedTestBarsA(), dailyBar{Date: date, Open: 8, High: 8.4, Low: 7.9, Close: 8.2})
	if err := saveT0DailyCache(date,
		[]t0Stock{{Code: "sh.600101", ShortCode: "600101", Name: "独立形态"}},
		map[string][]dailyBar{"600101": bars}); err != nil {
		t.Fatal(err)
	}
	req := httptest.NewRequest(http.MethodGet,
		"/api/t0-selection?module_code=radar.red_strategy&date="+date, nil)
	rr := httptest.NewRecorder()
	handleT0Selection(rr, req)
	var body struct {
		Count          int                 `json:"count"`
		DataIncomplete bool                `json:"data_incomplete"`
		Results        []T0SelectionResult `json:"results"`
	}
	if err := json.Unmarshal(rr.Body.Bytes(), &body); err != nil {
		t.Fatal(err)
	}
	if body.Count != 1 || len(body.Results) != 1 || body.Results[0].StockCode != "600101.XSHG" || !body.DataIncomplete {
		t.Fatalf("body=%s", rr.Body.String())
	}
}

func TestGobRedPrewarmHTTPIncludesIndependentCandidate(t *testing.T) {
	oldRoot := t0CacheRootPath
	t0CacheRootPath = t.TempDir()
	t.Cleanup(func() { t0CacheRootPath = oldRoot })
	date := "2026-09-14"
	if err := saveT0DailyCache("2026-09-11",
		[]t0Stock{{Code: "sh.600101", ShortCode: "600101", Name: "独立形态"}},
		map[string][]dailyBar{"600101": gobRedTestBarsA()}); err != nil {
		t.Fatal(err)
	}
	if err := saveT0DailyCache(date,
		[]t0Stock{{Code: "sh.600101", ShortCode: "600101", Name: "独立形态"}},
		map[string][]dailyBar{"600101": gobRedTestBarsA()}); err != nil {
		t.Fatal(err)
	}
	rr := httptest.NewRecorder()
	writeRedT0PrewarmReadyHTTP(rr, date, redT0StrategyModuleCode)
	var body struct {
		Candidates []T0SelectionResult `json:"candidates"`
	}
	if err := json.Unmarshal(rr.Body.Bytes(), &body); err != nil {
		t.Fatal(err)
	}
	if len(body.Candidates) != 1 || body.Candidates[0].StockCode != "600101.XSHG" ||
		len(body.Candidates[0].RedRuleHits) != 1 || body.Candidates[0].RedRuleHits[0].StatsStatus != "preopen" {
		t.Fatalf("prewarm body=%s", rr.Body.String())
	}
}

func TestGobRedPrewarmHistoricalResultsIncludeIndependentSnapshot(t *testing.T) {
	oldRoot := t0CacheRootPath
	t0CacheRootPath = t.TempDir()
	t.Cleanup(func() { t0CacheRootPath = oldRoot })
	previous, today := "2026-09-14", "2026-09-15"
	stocks := []t0Stock{{Code: "sh.600101", ShortCode: "600101", Name: "独立形态"}}
	bars := append(gobRedTestBarsA(), dailyBar{Date: previous, Open: 8, Close: 8.2})
	for _, date := range []string{previous, today} {
		if err := saveT0DailyCache(date, stocks, map[string][]dailyBar{"600101": bars}); err != nil {
			t.Fatal(err)
		}
	}
	if err := saveT0SelectionArchive(previous, []T0SelectionResult{}, true); err != nil {
		t.Fatal(err)
	}
	if err := saveGobRedArchive(&gobRedArchive{Date: previous, EntrySource: "realtime_open", Complete: true,
		Results: []T0SelectionResult{{Time: previous, StockCode: "600101.XSHG", StockName: "独立形态",
			RedRuleHits: []T0RedRuleHit{{Key: "gob_myin_dt_dt_v1", Version: 1, Name: "中阴→跌停→跌停后低开",
				EntrySource: "realtime_open", EntryOpen: 8, ExitHorizon: "T+1_close", StatsStatus: "pending"}}}}}); err != nil {
		t.Fatal(err)
	}
	rr := httptest.NewRecorder()
	writeContextT0PrewarmReadyHTTPAt(rr, today, redT0StrategyModuleCode,
		time.Date(2026, 9, 15, 8, 0, 0, 0, chinaLocation()))
	var body struct {
		Results      []T0SelectionResult `json:"results"`
		SourceStatus map[string]string   `json:"source_status"`
	}
	if err := json.Unmarshal(rr.Body.Bytes(), &body); err != nil {
		t.Fatal(err)
	}
	if len(body.Results) != 1 || body.Results[0].StockCode != "600101.XSHG" ||
		len(body.Results[0].RedRuleHits) != 1 || body.SourceStatus["gob"] != "realtime_open" {
		t.Fatalf("historical prewarm missing independent result: %s", rr.Body.String())
	}
}

func TestGobRedPrewarmHistoricalSnapshotWithoutMainArchive(t *testing.T) {
	oldRoot := t0CacheRootPath
	t0CacheRootPath = t.TempDir()
	t.Cleanup(func() { t0CacheRootPath = oldRoot })
	previous, today := "2026-09-14", "2026-09-15"
	stocks := []t0Stock{{Code: "sh.600101", ShortCode: "600101", Name: "独立形态"}}
	bars := append(gobRedTestBarsA(), dailyBar{Date: previous, Open: 8, Close: 8.2})
	if err := saveT0DailyCache(today, stocks, map[string][]dailyBar{"600101": bars}); err != nil {
		t.Fatal(err)
	}
	if err := saveGobRedArchive(&gobRedArchive{Date: previous, EntrySource: "realtime_open", Complete: true,
		Results: []T0SelectionResult{{Time: previous, StockCode: "600101.XSHG", StockName: "独立形态",
			RedRuleHits: []T0RedRuleHit{{Key: "gob_myin_dt_dt_v1", Version: 1, Name: "中阴→跌停→跌停后低开",
				EntrySource: "realtime_open", EntryOpen: 8, ExitHorizon: "T+1_close", StatsStatus: "pending"}}}}}); err != nil {
		t.Fatal(err)
	}
	rr := httptest.NewRecorder()
	writeContextT0PrewarmReadyHTTPAt(rr, today, redT0StrategyModuleCode,
		time.Date(2026, 9, 15, 8, 0, 0, 0, chinaLocation()))
	var body struct {
		Results        []T0SelectionResult `json:"results"`
		DataIncomplete bool                `json:"data_incomplete"`
		SourceStatus   map[string]string   `json:"source_status"`
	}
	if err := json.Unmarshal(rr.Body.Bytes(), &body); err != nil {
		t.Fatal(err)
	}
	if len(body.Results) != 1 || body.Results[0].StockCode != "600101.XSHG" ||
		body.SourceStatus["legacy"] != "unavailable" || !body.DataIncomplete {
		t.Fatalf("prewarm without main archive lost independent snapshot: %s", rr.Body.String())
	}
}

func TestGobRedArchiveKeepsConfirmedSnapshotOnIncompleteRefresh(t *testing.T) {
	oldRoot := t0CacheRootPath
	t0CacheRootPath = t.TempDir()
	t.Cleanup(func() { t0CacheRootPath = oldRoot })
	date := "2026-09-14"
	first := &gobRedArchive{Date: date, EntrySource: "realtime_open", Complete: true,
		Results: []T0SelectionResult{{Time: date, StockCode: "600101.XSHG",
			RedRuleHits: []T0RedRuleHit{{Key: "gob_myin_dt_dt_v1", EntryOpen: 8.1, EntrySource: "realtime_open"}}}}}
	if err := saveGobRedArchive(first); err != nil {
		t.Fatal(err)
	}
	if err := saveGobRedArchive(&gobRedArchive{Date: date, EntrySource: "realtime_open", Complete: false}); err != nil {
		t.Fatal(err)
	}
	got, ok := loadGobRedArchive(date)
	if !ok || !got.Complete || len(got.Results) != 1 || got.Results[0].RedRuleHits[0].EntryOpen != 8.1 {
		t.Fatalf("confirmed snapshot overwritten: %+v ok=%v", got, ok)
	}
	if _, err := os.Stat(t0SelectionCachePath(date)); !os.IsNotExist(err) {
		t.Fatalf("Gob snapshot should not create main archive: %v", err)
	}
}

func TestGobRedArchivedReplaysCompleteGobWithoutMainArchive(t *testing.T) {
	oldRoot := t0CacheRootPath
	t0CacheRootPath = t.TempDir()
	t.Cleanup(func() { t0CacheRootPath = oldRoot })
	date := "2026-09-14"
	bars := append(gobRedTestBarsA(), dailyBar{Date: date, Open: 8, High: 8.4, Low: 7.9, Close: 8.2})
	if err := saveT0DailyCache(date,
		[]t0Stock{{Code: "sh.600101", ShortCode: "600101", Name: "独立形态"}},
		map[string][]dailyBar{"600101": bars}); err != nil {
		t.Fatal(err)
	}
	rr := httptest.NewRecorder()
	handleT0Selection(rr, httptest.NewRequest(http.MethodGet,
		"/api/t0-selection?module_code=radar.red_strategy&archived=1&date="+date, nil))
	var body struct {
		Count          int                 `json:"count"`
		DataIncomplete bool                `json:"data_incomplete"`
		SourceStatus   map[string]string   `json:"source_status"`
		Results        []T0SelectionResult `json:"results"`
	}
	if err := json.Unmarshal(rr.Body.Bytes(), &body); err != nil {
		t.Fatal(err)
	}
	if body.Count != 1 || len(body.Results) != 1 || body.Results[0].StockCode != "600101.XSHG" ||
		body.Results[0].RedRuleHits[0].EntrySource != "daily_open" || !body.DataIncomplete ||
		body.SourceStatus["gob"] != "daily_open" || body.SourceStatus["legacy"] != "unavailable" {
		t.Fatalf("archived body=%s", rr.Body.String())
	}
	if _, ok := loadGobRedArchive(date); ok {
		t.Fatal("read-only historical replay must not create a realtime snapshot")
	}
}

func TestGobRedArchivedPrefersRealtimeSnapshotToDailyReplay(t *testing.T) {
	oldRoot := t0CacheRootPath
	t0CacheRootPath = t.TempDir()
	t.Cleanup(func() { t0CacheRootPath = oldRoot })
	date := "2026-09-14"
	bars := append(gobRedTestBarsA(), dailyBar{Date: date, Open: 8, High: 8.4, Low: 7.9, Close: 8.2})
	if err := saveT0DailyCache(date,
		[]t0Stock{{Code: "sh.600101", ShortCode: "600101", Name: "独立形态"}},
		map[string][]dailyBar{"600101": bars}); err != nil {
		t.Fatal(err)
	}
	if err := saveGobRedArchive(&gobRedArchive{Date: date, EntrySource: "realtime_open", Complete: true,
		Results: []T0SelectionResult{{Time: date, StockCode: "600101.XSHG", StockName: "独立形态",
			RedRuleHits: []T0RedRuleHit{{Key: "old_rule_v1", Version: 1, Name: "旧版", EntryOpen: 8.1,
				EntrySource: "realtime_open", ExitHorizon: "T+1_close"}}}}}); err != nil {
		t.Fatal(err)
	}
	rr := httptest.NewRecorder()
	handleT0Selection(rr, httptest.NewRequest(http.MethodGet,
		"/api/t0-selection?module_code=radar.red_strategy&archived=1&date="+date, nil))
	var body struct {
		Count        int                 `json:"count"`
		SourceStatus map[string]string   `json:"source_status"`
		Results      []T0SelectionResult `json:"results"`
	}
	if err := json.Unmarshal(rr.Body.Bytes(), &body); err != nil {
		t.Fatal(err)
	}
	if body.Count != 1 || body.Results[0].RedRuleHits[0].Key != "old_rule_v1" ||
		body.Results[0].RedRuleHits[0].EntryOpen != 8.1 || body.SourceStatus["gob"] != "realtime_open" {
		t.Fatalf("snapshot not preserved: %s", rr.Body.String())
	}
}

func TestGobRedArchivedMarksIncompleteSourceWithoutFakeTotal(t *testing.T) {
	oldRoot := t0CacheRootPath
	t0CacheRootPath = t.TempDir()
	t.Cleanup(func() { t0CacheRootPath = oldRoot })
	date := "2026-09-14"
	rr := httptest.NewRecorder()
	handleT0Selection(rr, httptest.NewRequest(http.MethodGet,
		"/api/t0-selection?module_code=radar.red_strategy&archived=1&date="+date, nil))
	var body struct {
		DataIncomplete bool              `json:"data_incomplete"`
		SourceStatus   map[string]string `json:"source_status"`
	}
	if err := json.Unmarshal(rr.Body.Bytes(), &body); err != nil {
		t.Fatal(err)
	}
	if !body.DataIncomplete || body.SourceStatus["gob"] != "unavailable" || body.SourceStatus["legacy"] != "unavailable" {
		t.Fatalf("missing sources looked complete: %s", rr.Body.String())
	}
}

func TestGobRedFormalSelectionArchivesWithoutRedPageDependency(t *testing.T) {
	oldRoot := t0CacheRootPath
	t0CacheRootPath = t.TempDir()
	t.Cleanup(func() { t0CacheRootPath = oldRoot })
	oldPrior := gobRedPriorCacheDate
	gobRedPriorCacheDate = func(string) string { return "2026-09-11" }
	t.Cleanup(func() { gobRedPriorCacheDate = oldPrior })
	oldQuotes := gobRedOpeningQuoteBuilder
	gobRedOpeningQuoteBuilder = func(_ string, _ []t0Stock, _ map[string][]dailyBar) (map[string]t0Realtime, string, *t0DataWarning) {
		return map[string]t0Realtime{"600101": {Open: 8, PrevClose: 8.424}}, "realtime_open", nil
	}
	t.Cleanup(func() { gobRedOpeningQuoteBuilder = oldQuotes })
	date := time.Now().In(chinaLocation()).Format("2006-01-02")
	results, warnings := selectGobRedFromPool(date,
		[]t0Stock{{Code: "sh.600101", ShortCode: "600101", Name: "独立形态"}},
		map[string][]dailyBar{"600101": gobRedTestBarsA()})
	if len(results) != 1 || len(warnings) != 0 {
		t.Fatalf("formal selection results=%+v warnings=%+v", results, warnings)
	}
	snapshot, ok := loadGobRedArchive(date)
	if !ok || !snapshot.Complete || snapshot.EntrySource != "realtime_open" || len(snapshot.Results) != 1 {
		t.Fatalf("formal selection did not archive independent rule: %+v ok=%v", snapshot, ok)
	}
	if _, err := os.Stat(t0SelectionCachePath(date)); !os.IsNotExist(err) {
		t.Fatalf("formal Gob selection unexpectedly touched main archive: %v", err)
	}
}

func TestGobRedRuleRejectsStalePreviousBarWhenPoolHasNewerTradingDay(t *testing.T) {
	date := "2026-09-29"
	stocks := []t0Stock{
		{Code: "sh.600101", ShortCode: "600101", Name: "停牌旧K线"},
		{Code: "sh.600102", ShortCode: "600102", Name: "正常交易股"},
	}
	daily := map[string][]dailyBar{
		"600101": gobRedTestBarsA(),
		"600102": {{Date: "2026-09-28", Open: 10, High: 10, Low: 10, Close: 10}},
	}
	got, _ := matchGobRedRules(date, stocks, daily,
		map[string]t0Realtime{"600101": {Open: 8, PrevClose: 8.424}},
		"realtime_open", approvedGobRedRules())
	if len(got) != 0 {
		t.Fatalf("stale T-1 bar became a current entry: %+v", got)
	}
}

func TestGobRedFormalRejectsPoolWideStalePriorDay(t *testing.T) {
	oldRoot := t0CacheRootPath
	t0CacheRootPath = t.TempDir()
	t.Cleanup(func() { t0CacheRootPath = oldRoot })
	date := "2026-09-15"
	stocks := []t0Stock{{Code: "sh.600101", ShortCode: "600101", Name: "全池旧K线"}}
	if err := saveT0DailyCache("2026-09-14", stocks,
		map[string][]dailyBar{"600101": append(gobRedTestBarsA(),
			dailyBar{Date: "2026-09-14", Open: 8, Close: 8.2})}); err != nil {
		t.Fatal(err)
	}
	oldQuotes := gobRedOpeningQuoteBuilder
	gobRedOpeningQuoteBuilder = func(_ string, _ []t0Stock, _ map[string][]dailyBar) (map[string]t0Realtime, string, *t0DataWarning) {
		return map[string]t0Realtime{"600101": {Open: 8, PrevClose: 8.424}}, "realtime_open", nil
	}
	t.Cleanup(func() { gobRedOpeningQuoteBuilder = oldQuotes })
	results, warnings := selectGobRedFromPool(date, stocks,
		map[string][]dailyBar{"600101": gobRedTestBarsA()})
	if len(results) != 0 || len(warnings) == 0 || warnings[0].Code != "RED_GOB_PRIOR_DATE_UNVERIFIED" {
		t.Fatalf("stale whole pool results=%+v warnings=%+v", results, warnings)
	}
}

func TestGobRedFormalRejectsMissingPriorCacheWitness(t *testing.T) {
	oldRoot := t0CacheRootPath
	t0CacheRootPath = t.TempDir()
	t.Cleanup(func() { t0CacheRootPath = oldRoot })
	results, warnings := selectGobRedFromPool("2026-09-14",
		[]t0Stock{{Code: "sh.600101", ShortCode: "600101", Name: "独立形态"}},
		map[string][]dailyBar{"600101": gobRedTestBarsA()})
	if len(results) != 0 || len(warnings) == 0 || warnings[0].Code != "RED_GOB_PRIOR_DATE_UNVERIFIED" {
		t.Fatalf("missing prior witness results=%+v warnings=%+v", results, warnings)
	}
}

func TestGobRedDateListIncludesIndependentGobDay(t *testing.T) {
	oldRoot := t0CacheRootPath
	t0CacheRootPath = t.TempDir()
	t.Cleanup(func() { t0CacheRootPath = oldRoot })
	date := "2024-01-02"
	if err := saveT0DailyCache(date,
		[]t0Stock{{Code: "sh.600101", ShortCode: "600101", Name: "独立形态"}},
		map[string][]dailyBar{"600101": gobRedTestBarsA()}); err != nil {
		t.Fatal(err)
	}
	rr := httptest.NewRecorder()
	handleT0Selection(rr, httptest.NewRequest(http.MethodGet,
		"/api/t0-selection?module_code=radar.red_strategy&list_dates=1", nil))
	var body struct {
		Dates []string `json:"dates"`
	}
	if err := json.Unmarshal(rr.Body.Bytes(), &body); err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(body.Dates, []string{date}) {
		t.Fatalf("red available dates=%v, want Gob day %s", body.Dates, date)
	}
}

func TestGobRedDateListRequiresRedModulePermission(t *testing.T) {
	auth := newTestAuthService(t)
	user := authHTTPRegisterUser(t, auth, "13800000000", "Alice", "device-a")
	for _, query := range []string{"list_dates=1", "archived=1&date=2026-09-14", "prewarm=1&date=2026-09-14", "date=2026-09-14"} {
		rr := httptest.NewRecorder()
		newHTTPHandler(auth.AuthService).ServeHTTP(rr, authHTTPRequest(http.MethodGet,
			"/api/t0-selection?module_code=radar.red_strategy&"+query, user.AccessToken, ""))
		if rr.Code != http.StatusForbidden {
			t.Fatalf("ungranted user read red %s: %d %s", query, rr.Code, rr.Body.String())
		}
	}
}

func TestGobRedStatsUsesNextTradingCloseAndExcludesMissing(t *testing.T) {
	hit := T0RedRuleHit{Key: "a_v1", Version: 1, EntrySource: "daily_open", ExitHorizon: "T+1_close", EntryOpen: 10}
	samples := []gobRedStatSample{
		{Date: "2026-09-01", StockCode: "600001", Hit: hit, NextClose: 10.1},
		{Date: "2026-09-02", StockCode: "600002", Hit: hit, NextClose: 10},
		{Date: "2026-09-03", StockCode: "600003", Hit: hit, NextClose: 10.25},
		{Date: "2026-09-04", StockCode: "600004", Hit: hit},
	}
	stats := aggregateGobRedRuleStats(samples, "2026-09-01", "2026-09-04", "gob-v1", "batch-1")
	if len(stats) != 1 || stats[0].SampleCount != 3 || stats[0].PositiveCount != 2 ||
		stats[0].TargetCount != 1 || math.Abs(stats[0].ProfitWinRate-200.0/3) > 1e-9 ||
		math.Abs(stats[0].TargetRate-100.0/3) > 1e-9 ||
		stats[0].DateEnd != "2026-09-03" {
		t.Fatalf("T+1 stats=%+v", stats)
	}
}

func TestGobRedStatsSeparatesRulesSourcesAndDeduplicates(t *testing.T) {
	base := gobRedStatSample{Date: "2026-09-01", StockCode: "600001", NextClose: 11,
		Hit: T0RedRuleHit{Key: "a_v1", Version: 1, EntrySource: "daily_open", ExitHorizon: "T+1_close", EntryOpen: 10}}
	otherRule := base
	otherRule.Hit.Key = "b_v1"
	otherVersion := base
	otherVersion.Hit.Version = 2
	otherSource := base
	otherSource.Hit.EntrySource = "realtime_open"
	stats := aggregateGobRedRuleStats([]gobRedStatSample{base, base, otherRule, otherVersion, otherSource},
		"2026-09-01", "2026-09-01", "gob-v1", "batch-1")
	if len(stats) != 4 {
		t.Fatalf("stats dimensions/dedup=%+v", stats)
	}
	for _, stat := range stats {
		if stat.SampleCount != 1 || stat.PositiveCount != 1 {
			t.Fatalf("duplicate or mixed stat=%+v", stat)
		}
	}
}

func TestGobRedStatsPersistenceAndPendingDisplay(t *testing.T) {
	old := db.Dao
	t.Cleanup(func() { db.Dao = old })
	dao, err := gorm.Open(sqlite.Open(filepath.Join(t.TempDir(), "red-stat.db")), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	db.Dao = dao
	if err := dao.AutoMigrate(&models.T0RedRuleStat{}); err != nil {
		t.Fatal(err)
	}
	hit := T0RedRuleHit{Key: "a_v1", Version: 1, EntrySource: "daily_open", ExitHorizon: "T+1_close", EntryOpen: 10, StatsStatus: "pending"}
	result := []T0SelectionResult{{StockCode: "600001.XSHG", PatternWinPct: 88, RedRuleHits: []T0RedRuleHit{hit}}}
	if got := enrichGobRedRuleStats(result); got[0].RedRuleHits[0].Stats != nil || got[0].RedRuleHits[0].StatsStatus != "pending" {
		t.Fatalf("unbackfilled rule must not inherit T0 stats: %+v", got)
	}
	stats := aggregateGobRedRuleStats([]gobRedStatSample{{Date: "2026-09-01", StockCode: "600001", Hit: hit, NextClose: 11}},
		"2026-09-01", "2026-09-01", "gob-v1", "batch-1")
	for i := 0; i < 2; i++ {
		if err := saveGobRedRuleStats(dao, stats); err != nil {
			t.Fatal(err)
		}
	}
	partial := append([]models.T0RedRuleStat(nil), stats...)
	partial[0].Status = "partial"
	if err := saveGobRedRuleStats(dao, partial); err == nil {
		t.Fatal("partial coverage must not enter the verified stat table")
	}
	var count int64
	if err := dao.Model(&models.T0RedRuleStat{}).Count(&count).Error; err != nil || count != 1 {
		t.Fatalf("idempotent rows=%d err=%v", count, err)
	}
	got := enrichGobRedRuleStats(result)
	if got[0].RedRuleHits[0].StatsStatus != "verified" || got[0].RedRuleHits[0].Stats == nil ||
		got[0].RedRuleHits[0].Stats.SampleCount != 1 || got[0].RedRuleHits[0].Stats.ProfitWinRate != 100 ||
		got[0].RedRuleHits[0].Stats.ExitHorizon != "T+1_close" {
		t.Fatalf("verified T+1 stats=%+v", got)
	}
	preopen := result[0]
	preopen.RedRuleHits = []T0RedRuleHit{hit}
	preopen.RedRuleHits[0].StatsStatus = "preopen"
	if got := enrichGobRedRuleStats([]T0SelectionResult{preopen}); got[0].RedRuleHits[0].Stats != nil ||
		got[0].RedRuleHits[0].StatsStatus != "preopen" {
		t.Fatalf("preopen candidate must not show verified purchase stats: %+v", got)
	}
}

func TestGobRedReplayReadsOnlyCompleteT0AndImmediateT1(t *testing.T) {
	oldRoot := t0CacheRootPath
	t0CacheRootPath = t.TempDir()
	t.Cleanup(func() { t0CacheRootPath = oldRoot })
	stock := []t0Stock{{Code: "sh.600101", ShortCode: "600101", Name: "独立形态"}}
	day0 := append(gobRedTestBarsA(), dailyBar{Date: "2026-09-14", Open: 8, Close: 8.2})
	if err := saveT0DailyCache("2026-09-14", stock, map[string][]dailyBar{"600101": day0}); err != nil {
		t.Fatal(err)
	}
	day1 := append(append([]dailyBar(nil), day0...), dailyBar{Date: "2026-09-15", Open: 8.3, Close: 8.5})
	if err := saveT0DailyCache("2026-09-15", stock, map[string][]dailyBar{"600101": day1}); err != nil {
		t.Fatal(err)
	}
	stats, cases, coverage, err := ReplayGobRedRuleStats(t0CacheRootPath, "2026-09-14", "2026-09-14", "synthetic")
	if err != nil || coverage.CompleteDays != 1 || coverage.Matched != 1 || coverage.Observed != 1 ||
		len(stats) != 1 || stats[0].SampleCount != 1 || len(cases) != 1 || cases[0].NextDate != "2026-09-15" {
		t.Fatalf("replay stats=%+v cases=%+v coverage=%+v err=%v", stats, cases, coverage, err)
	}
}

func TestGobRedReplayMarksMissingDailyCoveragePartial(t *testing.T) {
	oldRoot := t0CacheRootPath
	t0CacheRootPath = t.TempDir()
	t.Cleanup(func() { t0CacheRootPath = oldRoot })
	stocks := []t0Stock{
		{Code: "sh.600101", ShortCode: "600101", Name: "独立形态"},
		{Code: "sh.600102", ShortCode: "600102", Name: "缺日线"},
	}
	day0 := append(gobRedTestBarsA(), dailyBar{Date: "2026-09-14", Open: 8, Close: 8.2})
	if err := saveT0DailyCache("2026-09-14", stocks, map[string][]dailyBar{"600101": day0}); err != nil {
		t.Fatal(err)
	}
	day1 := append(append([]dailyBar(nil), day0...), dailyBar{Date: "2026-09-15", Open: 8.3, Close: 8.5})
	if err := saveT0DailyCache("2026-09-15", stocks, map[string][]dailyBar{"600101": day1}); err != nil {
		t.Fatal(err)
	}
	stats, _, coverage, err := ReplayGobRedRuleStats(t0CacheRootPath, "2026-09-14", "2026-09-14", "synthetic")
	if err != nil || coverage.MissingDailyStocks != 1 || coverage.CompleteDays != 0 || len(stats) != 1 || stats[0].Status != "partial" {
		t.Fatalf("partial coverage stats=%+v coverage=%+v err=%v", stats, coverage, err)
	}
}

func TestGobRedReplayDoesNotTreatNextAvailableCacheAsNextTradingDay(t *testing.T) {
	oldRoot := t0CacheRootPath
	t0CacheRootPath = t.TempDir()
	t.Cleanup(func() { t0CacheRootPath = oldRoot })
	stock := []t0Stock{{Code: "sh.600101", ShortCode: "600101", Name: "独立形态"}}
	day0 := append(gobRedTestBarsA(), dailyBar{Date: "2026-09-14", Open: 8, Close: 8.2})
	if err := saveT0DailyCache("2026-09-14", stock, map[string][]dailyBar{"600101": day0}); err != nil {
		t.Fatal(err)
	}
	day2 := append(append([]dailyBar(nil), day0...),
		dailyBar{Date: "2026-09-15", Open: 8.1, Close: 8.3},
		dailyBar{Date: "2026-09-16", Open: 8.4, Close: 8.5})
	if err := saveT0DailyCache("2026-09-16", stock, map[string][]dailyBar{"600101": day2}); err != nil {
		t.Fatal(err)
	}
	stats, cases, coverage, err := ReplayGobRedRuleStats(t0CacheRootPath, "2026-09-14", "2026-09-14", "synthetic")
	if err != nil || coverage.Matched != 1 || coverage.Observed != 0 ||
		coverage.MissingNextCacheDays != 1 || len(stats) != 0 || len(cases) != 0 {
		t.Fatalf("skipped trading day replay stats=%+v cases=%+v coverage=%+v err=%v", stats, cases, coverage, err)
	}
}

func TestGobRedArchivedLegacyCacheShowsMatchButMarksIncomplete(t *testing.T) {
	oldRoot := t0CacheRootPath
	t0CacheRootPath = t.TempDir()
	t.Cleanup(func() { t0CacheRootPath = oldRoot })
	date := "2026-09-14"
	stock := []t0Stock{{Code: "sh.600101", ShortCode: "600101", Name: "独立形态"}}
	bars := append(gobRedTestBarsA(), dailyBar{Date: date, Open: 8, Close: 8.2})
	if err := saveT0DailyCache(date, stock, map[string][]dailyBar{"600101": bars}); err != nil {
		t.Fatal(err)
	}
	cache, ok := readT0DailyCachePayload(date)
	if !ok {
		t.Fatal("cannot read synthetic cache")
	}
	cache.DataSchemaVersion = 0
	cache.StockPoolComplete = false
	f, err := os.Create(t0DailyCachePath(date))
	if err != nil {
		t.Fatal(err)
	}
	if err := gob.NewEncoder(f).Encode(cache); err != nil {
		f.Close()
		t.Fatal(err)
	}
	if err := f.Close(); err != nil {
		t.Fatal(err)
	}
	results, _, source, incomplete := archivedGobRedResults(date)
	if len(results) != 1 || source != "incomplete" || !incomplete ||
		results[0].RedRuleHits[0].EntrySource != "daily_open" {
		t.Fatalf("legacy replay results=%+v source=%s incomplete=%v", results, source, incomplete)
	}
}
