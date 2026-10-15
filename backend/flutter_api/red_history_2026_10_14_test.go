package flutter_api

import (
	"encoding/json"
	"go-stock/backend/db"
	"os"
	"testing"
)

// Prices captured from THS and cross-checked with Tencent on 2026-10-14.
// The old shared GOB stopped at Oct 12; Oct 13 changes both admissions.
func TestRedOctober14FreshHistoryAndOpeningRangeExcludeJiuyangAndDecai(t *testing.T) {
	oldDB := db.Dao
	db.Dao = nil
	t.Cleanup(func() { db.Dao = oldDB })
	var fixture map[string]struct {
		Stock        t0Stock
		Quote        t0Realtime
		Stale, Fresh []dailyBar
	}
	b, err := os.ReadFile("testdata/red_history_2026_10_14.json")
	if err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal(b, &fixture); err != nil {
		t.Fatal(err)
	}
	date := "2026-10-14"
	for code, stock := range fixture {
		for _, fresh := range []bool{false, true} {
			bars := stock.Stale
			if fresh {
				bars = stock.Fresh
			}
			daily := map[string][]dailyBar{code: bars}
			quotes := map[string]t0Realtime{code: stock.Quote}
			pool := []t0Stock{stock.Stock}
			main, gaps := filterOpenGap(filterTurnover(filterLimitUpRecent(pool, daily, 7, 9.89), daily, 5), quotes, 0.01, 3)
			results := assembleT0CandidateResults(date, main, daily)
			for i := range results {
				results[i].OpenGap = round2(gaps[code])
			}
			legacy := filterRedT0Results(results, &t0ModuleSelectionContext{TradeDate: date, Daily: daily})
			independent, _ := matchGobRedRules(date, pool, daily, quotes, "realtime_open", approvedGobRedRules())
			if code == "002242" {
				if !fresh && (len(legacy) != 1 || legacy[0].DisplayRuleHits[0] != "强势连板") {
					t.Fatalf("stale Jiuyang misadmission not reproduced: %+v", legacy)
				}
				if fresh && (len(legacy) > 0 || len(independent) > 0) {
					t.Fatalf("Jiuyang admitted with correct T-1: legacy=%+v independent=%+v", legacy, independent)
				}
			} else if fresh {
				if len(legacy) > 0 || len(independent) > 0 {
					t.Fatalf("Decai low opening should be excluded: legacy=%+v independent=%+v", legacy, independent)
				}
				quote := stock.Quote
				quote.Open = quote.PrevClose * 1.02
				withAllowedOpen, _ := matchGobRedRules(date, pool, daily,
					map[string]t0Realtime{code: quote}, "realtime_open", approvedGobRedRules())
				if len(withAllowedOpen) != 1 || len(withAllowedOpen[0].RedRuleHits) != 1 ||
					withAllowedOpen[0].RedRuleHits[0].Key != "gob_zt_zt_pb_t1_amplitude_6_10_shadow_le_1_v3" {
					t.Fatalf("Decai should match with positive opening gaps too: %+v", withAllowedOpen)
				}
			} else if len(independent) > 0 {
				t.Fatalf("stale Decai history unexpectedly matched: %+v", independent)
			}
		}
	}
}
