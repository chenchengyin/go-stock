package flutter_api

import "testing"

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
