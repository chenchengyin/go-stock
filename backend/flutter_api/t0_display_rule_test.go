package flutter_api

import (
	"reflect"
	"testing"
)

func TestDisplayRuleHitsMatchesAnyLimitUpLimitDown(t *testing.T) {
	hist := []dailyBar{
		{Date: "2026-09-01", Close: 10},
		{Date: "2026-09-02", Open: 10, Close: 10, High: 10.2, Low: 9.8},
		{Date: "2026-09-03", Open: 10, Close: 11, High: 11, Low: 10},
		{Date: "2026-09-04", Open: 9.9, Close: 9.9, High: 9.9, Low: 9.8},
	}

	got := displayRuleHitsForHist(hist)
	want := []string{"任意K线＋涨停＋跌停"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("displayRuleHitsForHist()=%v want %v", got, want)
	}
}

func TestDisplayRuleHitsMatchesRedKThenLimitDownT0(t *testing.T) {
	hist := []dailyBar{
		{Date: "2026-09-01", Close: 10},
		{Date: "2026-09-02", Open: 10, Close: 10.5, High: 10.5, Low: 10},
		{Date: "2026-09-03", Open: 9.45, Close: 9.45, High: 9.45, Low: 9.45},
	}

	got := displayRuleHitsForResult(hist, T0SelectionResult{OpenGap: 1.2})
	want := []string{"红K＋跌停后的开盘竞价"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("displayRuleHitsForResult()=%v want %v", got, want)
	}
}

func TestDisplayRuleHitsDoesNotMatchRedKThenLimitDownPreview(t *testing.T) {
	hist := []dailyBar{
		{Date: "2026-09-01", Close: 10},
		{Date: "2026-09-02", Open: 10, Close: 10.5, High: 10.5, Low: 10},
		{Date: "2026-09-03", Open: 9.45, Close: 9.45, High: 9.45, Low: 9.45},
	}

	got := displayRuleHitsForResult(hist, T0SelectionResult{})
	for _, hit := range got {
		if hit == displayRuleRedKLimitDownT0 {
			t.Fatalf("preview unexpectedly matched %q: %v", displayRuleRedKLimitDownT0, got)
		}
	}
}

func TestDisplayRuleHitsDoesNotMatchBearishKThenLimitDownT0(t *testing.T) {
	hist := []dailyBar{
		{Date: "2026-09-01", Close: 10},
		{Date: "2026-09-02", Open: 10.5, Close: 10, High: 10.5, Low: 10},
		{Date: "2026-09-03", Open: 9, Close: 9, High: 9, Low: 9},
	}

	got := displayRuleHitsForResult(hist, T0SelectionResult{OpenGap: 1.2})
	for _, hit := range got {
		if hit == displayRuleRedKLimitDownT0 {
			t.Fatalf("bearish K unexpectedly matched %q: %v", displayRuleRedKLimitDownT0, got)
		}
	}
}

func TestDisplayRuleHitsMatchesLimitUpAndLatestBearishTagLogic(t *testing.T) {
	hist := []dailyBar{
		{Date: "2026-09-01", Close: 10},
		{Date: "2026-09-02", Open: 10, Close: 11, High: 11, Low: 10},
		{Date: "2026-09-03", Open: 11.5, Close: 11, High: 11.8, Low: 10.9},
	}

	got := displayRuleHitsForHist(hist)
	want := []string{"涨停＋阴线标记"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("displayRuleHitsForHist()=%v want %v", got, want)
	}
}

func TestDisplayRuleHitsMatchesBullishZtZtPb(t *testing.T) {
	hist := []dailyBar{
		{Date: "2026-09-01", Close: 10},
		{Date: "2026-09-02", Open: 10, Close: 11, High: 11, Low: 10},
		{Date: "2026-09-03", Open: 11, Close: 12.1, High: 12.1, Low: 11},
		{Date: "2026-09-04", Open: 12.2, Close: 13.2, High: 13.6, Low: 12.1},
	}

	got := displayRuleHitsForHist(hist)
	want := []string{"涨停＋涨停＋阳线破板"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("displayRuleHitsForHist()=%v want %v", got, want)
	}
}

func TestDisplayRuleHitsUsesLatestBearishTagBoundaries(t *testing.T) {
	base := []dailyBar{
		{Date: "2026-09-01", Close: 10},
		{Date: "2026-09-02", Open: 10, Close: 11, High: 11, Low: 10},
	}
	cases := []struct {
		name      string
		open      float64
		close     float64
		high      float64
		wantMatch bool
	}{
		{name: "实体跌幅低于2不命中", open: 11.21, close: 11, high: 11.21, wantMatch: false},
		{name: "实体跌幅超过2命中", open: 11.23, close: 11, high: 11.23, wantMatch: true},
		{name: "实体跌幅低于8命中", open: 11.87, close: 11, high: 11.87, wantMatch: true},
		{name: "实体跌幅超过8不命中", open: 11.9, close: 11, high: 11.9, wantMatch: false},
		{name: "收盘跌超2不命中", open: 10.7, close: 10.76, high: 10.7, wantMatch: false},
		{name: "涨停破板优先不命中阴线标记", open: 11.55, close: 11, high: 12.1, wantMatch: false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			hist := append(append([]dailyBar(nil), base...), dailyBar{
				Date:  "2026-09-03",
				Open:  tc.open,
				Close: tc.close,
				High:  tc.high,
				Low:   tc.close,
			})
			got := len(displayRuleHitsForHist(hist)) > 0
			if got != tc.wantMatch {
				t.Fatalf("match=%v want %v", got, tc.wantMatch)
			}
		})
	}
}

func TestEnrichT0ResultsForDisplayReadsExistingGobAndDoesNotFilter(t *testing.T) {
	oldRoot := t0CacheRootPath
	t0CacheRootPath = t.TempDir()
	t.Cleanup(func() { t0CacheRootPath = oldRoot })

	daily := map[string][]dailyBar{
		"600000": {
			{Date: "2026-09-01", Close: 10},
			{Date: "2026-09-02", Open: 10, Close: 10, High: 10.2, Low: 9.8},
			{Date: "2026-09-03", Open: 10, Close: 11, High: 11, Low: 10},
			{Date: "2026-09-04", Open: 9.9, Close: 9.9, High: 9.9, Low: 9.8},
		},
	}
	if err := saveT0DailyCache("2026-09-05", []t0Stock{{ShortCode: "600000"}}, daily); err != nil {
		t.Fatalf("save cache: %v", err)
	}

	original := []T0SelectionResult{{
		Time:      "2026-09-05",
		StockCode: "600000.XSHG",
		StockName: "测试股",
		OpenGap:   1.2,
		BuySignal: BuySignalBlue,
	}}
	got := enrichT0ResultsForDisplay("2026-09-05", original)

	if len(got) != 1 {
		t.Fatalf("result count=%d want 1", len(got))
	}
	if !reflect.DeepEqual(got[0].DisplayRuleHits,
		[]string{"任意K线＋涨停＋跌停", "红K＋跌停后的开盘竞价"}) {
		t.Fatalf("hits=%v", got[0].DisplayRuleHits)
	}
	if original[0].DisplayRuleHits != nil {
		t.Fatalf("original result mutated: %v", original[0].DisplayRuleHits)
	}
}

func TestEnrichT0ResultsForDisplayMatchesHistoricalTwoLimitUpsAndBearishRule(t *testing.T) {
	daily := map[string][]dailyBar{
		"600000": {
			{Date: "2026-09-01", Close: 10},
			{Date: "2026-09-02", Open: 10, Close: 11, High: 11, Low: 10},
			{Date: "2026-09-03", Open: 11, Close: 12.1, High: 12.1, Low: 11},
			{Date: "2026-09-04", Open: 12.4, Close: 12.1, High: 12.4, Low: 12},
			{Date: "2026-09-05", Open: 12.25, Close: 12.2, High: 12.3, Low: 12.1},
		},
	}
	results := []T0SelectionResult{{
		Time:      "2026-09-05",
		StockCode: "600000.XSHG",
		StockName: "历史命中股",
		OpenGap:   1.2,
	}}

	got := enrichT0ResultsForDisplayWithDaily(results, daily, "2026-09-05")
	want := []string{
		"涨停＋阴线标记",
		"涨停＋涨停＋普通阴线",
	}
	if !reflect.DeepEqual(got[0].DisplayRuleHits, want) {
		t.Fatalf("hits=%v want %v", got[0].DisplayRuleHits, want)
	}
}

func TestEnrichT0ResultsForDisplayDoesNotMarkPreviewWithoutT0OpenGap(t *testing.T) {
	daily := map[string][]dailyBar{
		"600000": {
			{Date: "2026-09-01", Close: 10},
			{Date: "2026-09-02", Open: 10, Close: 11, High: 11, Low: 10},
			{Date: "2026-09-03", Open: 11, Close: 12.1, High: 12.1, Low: 11},
			{Date: "2026-09-04", Open: 12.4, Close: 12.1, High: 12.4, Low: 12},
			{Date: "2026-09-05", Open: 12.25, Close: 12.2, High: 12.3, Low: 12.1},
		},
	}
	results := []T0SelectionResult{{
		Time:      "2026-09-05",
		StockCode: "600000.XSHG",
		StockName: "预览股",
		OpenGap:   0,
	}}

	got := enrichT0ResultsForDisplayWithDaily(results, daily, "2026-09-05")
	for _, hit := range got[0].DisplayRuleHits {
		if hit == displayRuleZtZtBearishT0 {
			t.Fatalf("preview unexpectedly matched new rule: %v", got[0].DisplayRuleHits)
		}
	}
}
