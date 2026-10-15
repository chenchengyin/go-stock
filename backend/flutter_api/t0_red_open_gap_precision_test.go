package flutter_api

import (
	"go-stock/backend/db"
	"testing"
)

func TestRedReferenceAdmissionUsesExactAuctionGapAtSubPercentBoundary(t *testing.T) {
	previous := db.Dao
	db.Dao = nil
	t.Cleanup(func() { db.Dao = previous })
	hist := []dailyBar{
		{Date: "2026-09-01", Close: 5.5},
		{Date: "2026-09-02", Open: 5.6, High: 5.6, Low: 5.6, Close: 5.6},
		{Date: "2026-09-03", Open: 5.6, High: 5.89, Low: 5.6, Close: 5.89},
		{Date: "2026-09-04", Open: 5.3, High: 5.3, Low: 5.3, Close: 5.3},
	}
	const code = "600101"
	result := T0SelectionResult{StockCode: code + ".XSHG", OpenGap: 0.75}
	if !matchesMediumYangThenLimitDownT0(hist, result) {
		t.Fatal("rounded display gap should reproduce the false positive")
	}
	ctx := &t0ModuleSelectionContext{
		TradeDate: "2026-09-07",
		Daily:     map[string][]dailyBar{code: hist},
		OpenGaps:  map[string]float64{code: (5.34/5.3 - 1) * 100},
	}
	if got := filterRedT0Results([]T0SelectionResult{result}, ctx); len(got) != 0 {
		t.Fatalf("actual auction gap above 0.75%% entered red list: %+v", got)
	}
	displayed := enrichT0ResultsForDisplayWithDailyForModule(
		[]T0SelectionResult{result}, ctx.Daily, ctx.TradeDate, redT0StrategyModuleCode, ctx.OpenGaps)
	for _, hit := range displayed[0].DisplayRuleHits {
		if hit == displayRuleMediumYangLimitDownT0 {
			t.Fatalf("actual auction gap above 0.75%% displayed as a hit: %+v", displayed[0])
		}
	}
	if displayed[0].OpenGap != 0.75 {
		t.Fatalf("display gap changed: %f", displayed[0].OpenGap)
	}
}
