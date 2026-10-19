package flutter_api

import (
	"encoding/json"
	"os"
	"testing"
)

func TestOptimizedConfirmedHitsKeepFrozenAdmissionAndQuotes(t *testing.T) {
	isolateSharedGobConfirmation(t)
	bars := optimizedTestBars()
	date := bars[30].Date
	mainboardSharedGob = &mainboardSharedGobData{Daily: map[string][]dailyBar{"600001": bars}}
	base, _ := gobRedRuleByKey("gob_zt_t1_bear_open_close_body_4combos_v1")
	// A frozen 1.6% opening must exclude small-gap even if today's GOB says 1%.
	open := bars[29].Close * 1.016
	result := T0SelectionResult{Time: date, StockCode: "600001.XSHG", StockName: "已确认", PrevClose: bars[29].Close, OpenGap: 1.6, RedRuleHits: []T0RedRuleHit{{Key: base.Key, Version: 1, EntrySource: "realtime_open", EntryOpen: open}}, DisplayRuleHits: []string{base.Name}}
	response := map[string]interface{}{"date": date, "confirmed": true, "count": 1, "results": []T0SelectionResult{result}}
	raw, _ := json.Marshal(response)
	snapshot := sharedGobRedConfirmation{SchemaVersion: 1, Date: date, ConfirmedAt: date + "T09:25:10+08:00", HistoryDate: bars[29].Date, Response: raw, Quotes: map[string]t0Realtime{"600001": {Open: open, PrevClose: bars[29].Close}}}
	if _, err := saveSharedGobRedConfirmation(snapshot); err != nil {
		t.Fatal(err)
	}
	before, _ := os.ReadFile(sharedGobRedConfirmationPath(date))
	got, found, err := loadSharedGobRedConfirmation(date)
	if err != nil || !found {
		t.Fatalf("load: %v %v", found, err)
	}
	raw, _ = json.Marshal(got["results"])
	var results []T0SelectionResult
	json.Unmarshal(raw, &results)
	if len(results) != 1 || results[0].OpenGap != 1.6 || len(results[0].RedRuleHits) != 7 {
		t.Fatalf("wanted unchanged stock with base plus six hits: %+v", results)
	}
	for _, hit := range results[0].RedRuleHits {
		if hit.EntryOpen != open || hit.Key == optimizedRedPrefix+"upper_body8_gap15_v1" {
			t.Fatalf("changed frozen quote: %+v", hit)
		}
	}
	after, _ := os.ReadFile(sharedGobRedConfirmationPath(date))
	if string(before) != string(after) {
		t.Fatal("rewrote confirmation file")
	}
}
