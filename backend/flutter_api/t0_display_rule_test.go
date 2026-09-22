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

func TestDeepRedDisplayRuleMatchesAnyLimitUpLimitDown(t *testing.T) {
	hist := []dailyBar{
		{Date: "2026-09-01", Close: 10},
		{Date: "2026-09-02", Open: 10, Close: 10, High: 10.2, Low: 9.8},
		{Date: "2026-09-03", Open: 10, Close: 11, High: 11, Low: 10},
		{Date: "2026-09-04", Open: 9.9, Close: 9.9, High: 9.9, Low: 9.8},
	}

	if !matchesDeepRedDisplayRule(hist, T0SelectionResult{OpenGap: 1.2}) {
		t.Fatal("any-limit-up-limit-down combination should be deep red")
	}
	if matchesDeepRedDisplayRule(hist, T0SelectionResult{}) {
		t.Fatal("deep red rule should require a confirmed T0 auction")
	}
}

func TestDisplayRuleHitsMatchesMediumYangThenLimitDownT0(t *testing.T) {
	hist := []dailyBar{
		{Date: "2026-09-01", Close: 10},
		{Date: "2026-09-02", Open: 10, Close: 10.5, High: 10.5, Low: 10},
		{Date: "2026-09-03", Open: 9.45, Close: 9.45, High: 9.45, Low: 9.45},
	}

	got := displayRuleHitsForResult(hist, T0SelectionResult{OpenGap: 1.2})
	want := []string{"中阳及以上＋跌停后的开盘竞价"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("displayRuleHitsForResult()=%v want %v", got, want)
	}
}

func TestDisplayRuleHitsMediumYangThenLimitDownRequiresMediumYangOrAbove(t *testing.T) {
	cases := []struct {
		name       string
		redOpen    float64
		redClose   float64
		limitDown  float64
		wantMarked bool
	}{
		{name: "小阳不命中", redOpen: 10, redClose: 10.2, limitDown: 9.18, wantMarked: false},
		{name: "中阳命中", redOpen: 10, redClose: 10.3, limitDown: 9.27, wantMarked: true},
		{name: "大阳命中", redOpen: 10, redClose: 10.7, limitDown: 9.63, wantMarked: true},
		{name: "涨停命中", redOpen: 10, redClose: 11, limitDown: 9.9, wantMarked: true},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			hist := []dailyBar{
				{Date: "2026-09-01", Close: 10},
				{Date: "2026-09-02", Open: tc.redOpen, Close: tc.redClose, High: tc.redClose, Low: tc.redOpen},
				{Date: "2026-09-03", Open: tc.limitDown, Close: tc.limitDown, High: tc.limitDown, Low: tc.limitDown},
			}
			got := false
			for _, hit := range displayRuleHitsForResult(hist, T0SelectionResult{OpenGap: 1.2}) {
				if hit == displayRuleMediumYangLimitDownT0 {
					got = true
				}
			}
			if got != tc.wantMarked {
				t.Fatalf("red K + limit-down hit=%v want %v", got, tc.wantMarked)
			}
		})
	}
}

func TestDisplayRuleHitsDoesNotMatchMediumYangThenLimitDownPreview(t *testing.T) {
	hist := []dailyBar{
		{Date: "2026-09-01", Close: 10},
		{Date: "2026-09-02", Open: 10, Close: 10.5, High: 10.5, Low: 10},
		{Date: "2026-09-03", Open: 9.45, Close: 9.45, High: 9.45, Low: 9.45},
	}

	got := displayRuleHitsForResult(hist, T0SelectionResult{})
	for _, hit := range got {
		if hit == displayRuleMediumYangLimitDownT0 {
			t.Fatalf("preview unexpectedly matched %q: %v", displayRuleMediumYangLimitDownT0, got)
		}
	}
}

func TestStrongDisplayRuleMatchesPreviousDayOpenGapAtLeastThreePercent(t *testing.T) {
	base := []dailyBar{
		{Date: "2026-09-01", Close: 10},
	}
	cases := []struct {
		name      string
		open      float64
		wantMatch bool
	}{
		{name: "刚好三个百分点命中", open: 10.3, wantMatch: true},
		{name: "低于三个百分点不命中", open: 10.299, wantMatch: false},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			hist := append(append([]dailyBar(nil), base...), dailyBar{
				Date:  "2026-09-02",
				Open:  tc.open,
				Close: 10.1,
				High:  tc.open,
				Low:   10,
			})
			if got := matchesStrongPrevDayOpenGap(hist); got != tc.wantMatch {
				t.Fatalf("matchesStrongPrevDayOpenGap()=%v want %v", got, tc.wantMatch)
			}
		})
	}
}

func TestDisplayRuleHitsMatchesTwoLimitUpThenNonOneWordAuction(t *testing.T) {
	hist := []dailyBar{
		{Date: "2026-09-01", Close: 10},
		{Date: "2026-09-02", Open: 10, Close: 11, High: 11, Low: 10},
		{Date: "2026-09-03", Open: 11.4, Close: 12.1, High: 12.1, Low: 11.4},
	}

	got := displayRuleHitsForResult(hist, T0SelectionResult{OpenGap: 1.2})
	want := []string{"涨停＋非一字涨停后的开盘竞价"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("displayRuleHitsForResult()=%v want %v", got, want)
	}
}

func TestDisplayRuleHitsDoesNotMatchOneWordSecondLimitUp(t *testing.T) {
	hist := []dailyBar{
		{Date: "2026-09-01", Close: 10},
		{Date: "2026-09-02", Open: 10, Close: 11, High: 11, Low: 10},
		{Date: "2026-09-03", Open: 12.1, Close: 12.1, High: 12.1, Low: 12.1},
	}

	got := displayRuleHitsForResult(hist, T0SelectionResult{OpenGap: 1.2})
	for _, hit := range got {
		if hit == "涨停＋非一字涨停后的开盘竞价" {
			t.Fatalf("one-word second limit-up unexpectedly matched: %v", got)
		}
	}
}

func TestStrongDisplayRuleRequiresTwoLimitUpNonOneWordAuction(t *testing.T) {
	hist := []dailyBar{
		{Date: "2026-09-01", Close: 10},
		{Date: "2026-09-02", Open: 10, Close: 11, High: 11, Low: 10},
		{Date: "2026-09-03", Open: 11.4, Close: 12.1, High: 12.1, Low: 11.4},
	}
	if !matchesStrongTwoLimitUpNonOneWordAuction(hist, T0SelectionResult{OpenGap: 1.2}) {
		t.Fatal("strong display rule should match the exact two-limit-up auction combination")
	}

	hist[len(hist)-1].Open = 11.1
	if matchesStrongTwoLimitUpNonOneWordAuction(hist, T0SelectionResult{OpenGap: 1.2}) {
		t.Fatal("strong display rule matched T-1 open gap below 3%")
	}
}

func TestStrongDisplayRuleDoesNotMatchOneWordSecondLimitUp(t *testing.T) {
	hist := []dailyBar{
		{Date: "2026-09-01", Close: 10},
		{Date: "2026-09-02", Open: 10, Close: 11, High: 11, Low: 10},
		{Date: "2026-09-03", Open: 12.1, Close: 12.1, High: 12.1, Low: 12.1},
	}
	if matchesStrongTwoLimitUpNonOneWordAuction(hist, T0SelectionResult{OpenGap: 1.2}) {
		t.Fatal("strong display rule matched a one-word second limit-up")
	}
}

func TestTechBlueRedRuleRequiresAllConfirmedConditions(t *testing.T) {
	baseHist := []dailyBar{
		{Date: "2026-09-01", Close: 10},
		{Date: "2026-09-02", Open: 10.2, Close: 11, High: 11, Low: 10.2},
		{Date: "2026-09-03", Open: 12.1, Close: 12.1, High: 12.1, Low: 11.4},
	}
	cases := []struct {
		name      string
		mutate    func([]dailyBar)
		wantMatch bool
	}{
		{name: "全部条件满足", wantMatch: true},
		{
			name: "T-2一字板",
			mutate: func(hist []dailyBar) {
				hist[1].Open = hist[1].Close
				hist[1].High = hist[1].Close
				hist[1].Low = hist[1].Close
			},
		},
		{
			name: "T-1一字板",
			mutate: func(hist []dailyBar) {
				hist[2].Low = hist[2].Close
			},
		},
		{
			name: "T-1开盘涨幅低于7%",
			mutate: func(hist []dailyBar) {
				hist[2].Open = 11.7
			},
		},
		{
			name:      "T-1开盘涨幅达到7%",
			mutate:    func(hist []dailyBar) { hist[2].Open = 11.78 },
			wantMatch: true,
		},
		{
			name: "T-2开盘强于T-1",
			mutate: func(hist []dailyBar) {
				hist[1].Open = 11.2
				hist[1].High = 11.2
				hist[1].Low = 10.8
			},
			wantMatch: true,
		},
		{
			name:      "T0开盘涨幅超出范围",
			mutate:    func([]dailyBar) {},
			wantMatch: false,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			hist := append([]dailyBar(nil), baseHist...)
			if tc.mutate != nil {
				tc.mutate(hist)
			}
			openGap := 1.2
			if tc.name == "T0开盘涨幅超出范围" {
				openGap = 3.1
			}
			got := matchesTechBlueRedRule(hist, T0SelectionResult{OpenGap: openGap})
			want := tc.wantMatch
			if tc.name == "全部条件满足" {
				want = true
			}
			if got != want {
				t.Fatalf("matchesTechBlueRedRule()=%v want %v", got, want)
			}
		})
	}
}

func TestStrongDisplayRuleDoesNotUpgradeAnotherDisplayRule(t *testing.T) {
	hist := []dailyBar{
		{Date: "2026-09-01", Close: 10},
		{Date: "2026-09-02", Open: 10, Close: 11, High: 11, Low: 10},
		{Date: "2026-09-03", Open: 11.4, Close: 9.9, High: 11.4, Low: 9.9},
	}

	hits := displayRuleHitsForResult(hist, T0SelectionResult{OpenGap: 1.2})
	if len(hits) == 0 {
		t.Fatal("expected the other display rule to remain matched")
	}
	if matchesStrongTwoLimitUpNonOneWordAuction(hist, T0SelectionResult{OpenGap: 1.2}) {
		t.Fatalf("another display rule was incorrectly upgraded to deep red: %v", hits)
	}
}

func TestEnrichStrongDisplayRuleRequiresAnExistingDisplayRuleHit(t *testing.T) {
	daily := map[string][]dailyBar{
		"600000": {
			{Date: "2026-09-01", Close: 10},
			{Date: "2026-09-02", Open: 10.3, Close: 10.1, High: 10.3, Low: 10},
			{Date: "2026-09-03", Open: 10.1, Close: 10.1, High: 10.1, Low: 10},
		},
	}
	results := []T0SelectionResult{{
		StockCode: "600000.XSHG",
	}}

	got := enrichT0ResultsForDisplayWithDaily(results, daily, "2026-09-03")
	if len(got[0].DisplayRuleHits) != 0 {
		t.Fatalf("display rule hits=%v want none", got[0].DisplayRuleHits)
	}
	if got[0].StrongDisplayRuleHit {
		t.Fatal("strong display flag should require an existing display rule hit")
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
		if hit == displayRuleMediumYangLimitDownT0 {
			t.Fatalf("bearish K unexpectedly matched %q: %v", displayRuleMediumYangLimitDownT0, got)
		}
	}
}

func TestDisplayRuleHitsDoesNotUseStandaloneLimitUpBearishTag(t *testing.T) {
	hist := []dailyBar{
		{Date: "2026-09-01", Close: 10},
		{Date: "2026-09-02", Open: 10, Close: 11, High: 11, Low: 10},
		{Date: "2026-09-03", Open: 11.5, Close: 11, High: 11.8, Low: 10.9},
	}

	got := displayRuleHitsForHist(hist)
	if len(got) != 0 {
		t.Fatalf("standalone limit-up plus bearish candle should be removed, got %v", got)
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

func TestMatchesZtZtBearishT0UsesRequestedBoundaries(t *testing.T) {
	base := []dailyBar{
		{Date: "2026-09-01", Close: 8.28},
		{Date: "2026-09-02", Open: 8.5, Close: 9.1, High: 9.1, Low: 8.5},
		{Date: "2026-09-03", Open: 9.5, Close: 10, High: 10, Low: 9.5},
	}
	cases := []struct {
		name      string
		open      float64
		close     float64
		wantMatch bool
	}{
		{name: "收盘跌幅等于负2允许", open: 10.1, close: 9.8, wantMatch: true},
		{name: "收盘涨幅等于3允许", open: 10.31, close: 10.3, wantMatch: true},
		{name: "阴线实体等于8不允许", open: 10.8, close: 10, wantMatch: false},
		{name: "阴线实体小于8允许", open: 10.79, close: 10, wantMatch: true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			hist := append(append([]dailyBar(nil), base...), dailyBar{
				Date:  "2026-09-04",
				Open:  tc.open,
				Close: tc.close,
				High:  tc.open,
				Low:   tc.close,
			})
			got := matchesZtZtBearishT0(hist, T0SelectionResult{OpenGap: 1.2})
			if got != tc.wantMatch {
				t.Fatalf("matchesZtZtBearishT0()=%v want %v", got, tc.wantMatch)
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
		[]string{"任意K线＋涨停＋跌停", "中阳及以上＋跌停后的开盘竞价"}) {
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
		"涨停＋涨停＋普通阴线",
	}
	if !reflect.DeepEqual(got[0].DisplayRuleHits, want) {
		t.Fatalf("hits=%v want %v", got[0].DisplayRuleHits, want)
	}
}

func TestEnrichT0ResultsForDisplayDoesNotMarkTechBlueOutsideRedSelection(t *testing.T) {
	daily := map[string][]dailyBar{
		"600000": {
			{Date: "2026-09-01", Close: 10},
			{Date: "2026-09-02", Open: 10.2, Close: 11, High: 11, Low: 10.2},
			{Date: "2026-09-03", Open: 12.1, Close: 12.1, High: 12.1, Low: 11.4},
		},
	}
	results := []T0SelectionResult{{
		StockCode: "600000.XSHG",
		OpenGap:   1.2,
	}}

	got := enrichT0ResultsForDisplayWithDaily(results, daily, "2026-09-04")
	if got[0].TechBlueDisplayRuleHit {
		t.Fatalf("generic display enrichment should not mark tech blue: %+v", got[0])
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
