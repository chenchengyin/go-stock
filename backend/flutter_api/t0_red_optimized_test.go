package flutter_api

import (
	"strings"
	"testing"
	"time"
)

func optimizedTestBars() []dailyBar {
	bars := make([]dailyBar, 32)
	d := time.Date(2026, 1, 2, 0, 0, 0, 0, time.UTC)
	for i := range bars {
		for d.Weekday() == time.Saturday || d.Weekday() == time.Sunday {
			d = d.AddDate(0, 0, 1)
		}
		bars[i] = dailyBar{Date: d.Format("2006-01-02"), Open: 100, Close: 100, High: 101, Low: 99, Volume: 100, AmountYi: 10}
		d = d.AddDate(0, 0, 1)
	}
	bars[27].Open = 100
	bars[27].Close = 102
	bars[27].High = 103
	bars[27].Low = 99
	bars[28].Open = 102
	bars[28].Close = 112.2
	bars[28].High = 112.2
	bars[28].Low = 102
	bars[29].Open = 117.81
	bars[29].Close = 111.078
	bars[29].High = 118
	bars[29].Low = 110.9
	bars[30].Open = bars[29].Close * 1.01
	bars[30].Close = 116
	bars[30].High = 117
	bars[30].Low = 110
	bars[31].Open = 116
	bars[31].Close = 117
	bars[31].High = 118
	bars[31].Low = 115
	return bars
}
func optimizedTestHits(bars []dailyBar) map[string]bool {
	out := map[string]bool{}
	date := bars[30].Date
	results, _ := matchGobRedRules(date, []t0Stock{{Code: "sh.600001", ShortCode: "600001", Name: "测试"}}, map[string][]dailyBar{"600001": bars}, map[string]t0Realtime{"600001": {Open: bars[30].Open, PrevClose: bars[29].Close}}, "daily_open", approvedGobRedRules())
	for _, r := range results {
		for _, h := range r.RedRuleHits {
			if strings.HasPrefix(h.Name, "板阴·") {
				out[h.Name] = true
			}
		}
	}
	return out
}
func TestOptimizedRedRulesAllowSevenSimultaneousHits(t *testing.T) {
	got := optimizedTestHits(optimizedTestBars())
	if len(got) != 7 {
		t.Fatalf("got optimized hits %v; want all seven without duplicating the stock", got)
	}
	for _, name := range []string{"板阴·长实上半", "板阴·窄幅增强", "板阴·非连板", "板阴·小高开", "板阴·均线过滤", "板阴·前阳窄幅", "板阴·前阳长实"} {
		if !got[name] {
			t.Errorf("missing %s", name)
		}
	}
}
func TestOptimizedRedRulesKeepIndependentFilters(t *testing.T) {
	tests := []struct {
		name    string
		change  func([]dailyBar)
		missing []string
		present string
	}{
		{"wide prior day", func(b []dailyBar) { b[27].Low = 94 }, []string{"板阴·窄幅增强", "板阴·前阳窄幅"}, "板阴·长实上半"},
		{"prior bearish", func(b []dailyBar) { b[27].Open = 102.2 }, []string{"板阴·前阳窄幅", "板阴·前阳长实"}, "板阴·长实上半"},
		{"T0 over 1.5 percent", func(b []dailyBar) { b[30].Open = b[29].Close * 1.016 }, []string{"板阴·小高开"}, "板阴·长实上半"},
		{"short limit body", func(b []dailyBar) { b[28].Open = 105.06; b[28].Low = 105.06 }, []string{"板阴·长实上半", "板阴·前阳长实"}, "板阴·前阳窄幅"},
		{"far above MA20", func(b []dailyBar) {
			for i := 0; i < 26; i++ {
				b[i].Open = 60
				b[i].Close = 60
				b[i].High = 61
				b[i].Low = 59
			}
		}, []string{"板阴·均线过滤"}, "板阴·长实上半"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			b := optimizedTestBars()
			tt.change(b)
			got := optimizedTestHits(b)
			for _, n := range tt.missing {
				if got[n] {
					t.Errorf("unexpected %s", n)
				}
			}
			if !got[tt.present] {
				t.Errorf("missing unaffected %s; hits=%v", tt.present, got)
			}
		})
	}
}

func TestOptimizedHistoryKeepsAllHitsWhenFilteringOneRule(t *testing.T) {
	oldRoot := t0CacheRootPath
	t0CacheRootPath = t.TempDir()
	t.Cleanup(func() { t0CacheRootPath = oldRoot })
	hits := []T0RedRuleHit{{Key: optimizedRedPrefix + "upper_body8_v1", Version: 1, Name: "板阴·长实上半", EntrySource: "daily_open", ExitHorizon: "T+1_close"}, {Key: optimizedRedPrefix + "upper_body8_amp8_v1", Version: 1, Name: "板阴·窄幅增强", EntrySource: "daily_open", ExitHorizon: "T+1_close"}}
	date := "2026-09-10"
	if err := saveGobRedArchive(&gobRedArchive{Date: date, Complete: true, RuleSetVersion: currentGobRedRuleSetVersion(), Results: []T0SelectionResult{{Time: date, StockCode: "600001.XSHG", RedRuleHits: hits}}}); err != nil {
		t.Fatal(err)
	}
	index := buildGobRedRuleHistoryIndex([]string{date}, redHistoryReplayArchiveOnly, nil)
	for _, hit := range hits {
		rs := redHistoryResults(index.matchesByRule[redHistoryIdentity{source: "gob", key: hit.Key, version: 1}])
		if len(rs) != 1 || len(rs[0].RedRuleHits) != 2 {
			t.Fatalf("filter %s lost other matches: %+v", hit.Key, rs)
		}
	}
}

func TestOptimizedRedInclusiveBoundaries(t *testing.T) {
	bars := optimizedTestBars()
	bars[27].High, bars[27].Low = 106, 98               // close102 is exactly midpoint; amplitude8%.
	bars[28].Open = bars[28].Close - bars[27].Close*.08 // exactly8% body.
	bars[30].Open = bars[29].Close * 1.015              // exactly1.5% gap.
	if got := optimizedTestHits(bars); len(got) != 7 {
		t.Fatalf("inclusive boundaries lost hits: %v", got)
	}
	bars[30].Open = bars[29].Close * 1.015001
	if got := optimizedTestHits(bars); got["板阴·小高开"] || len(got) != 6 {
		t.Fatalf("gap beyond boundary admitted: %v", got)
	}
}
