package flutter_api

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"go-stock/backend/db"
)

func TestRedHistoryPricesReplacePlaceholderCloseWithoutChangingArchive(t *testing.T) {
	results := []T0SelectionResult{
		{Time: "2026-09-10", StockCode: "600000.XSHG", OpenGap: 99, CloseRet: 0},
		{Time: "2026-09-10", StockCode: "000001.XSHE", OpenGap: -2, CloseRet: 0},
	}
	daily := map[string][]dailyBar{"600000": {
		{Date: "2026-09-09", Close: 10},
		{Date: "2026-09-10", Open: 9.7, Close: 10.2},
	}}
	got := redHistoryResultsWithPrices(results, daily)
	if got[0].OpenGap != -3 || got[0].CloseRet != 2 || got[0].T0IntradayChangePP == nil || *got[0].T0IntradayChangePP != 5 {
		t.Fatalf("must use T0 close gap minus open gap from one price series: %+v", got[0])
	}
	if got[1].T0IntradayChangePP != nil {
		t.Fatal("missing daily prices must not turn the archived zero placeholder into a sortable value")
	}
	if results[0].CloseRet != 0 || results[0].OpenGap != 99 {
		t.Fatal("history price projection changed the source archive")
	}
	encoded, err := json.Marshal(got)
	if err != nil || !strings.Contains(string(encoded), `"t0_intraday_change_pp":null`) {
		t.Fatalf("missing prices must be explicit in JSON: %s %v", encoded, err)
	}
}

func TestRedHistoryPricesUseArchivedDailyOncePerMissingDate(t *testing.T) {
	results := []T0SelectionResult{
		{Time: "2026-09-10", StockCode: "600000.XSHG"},
		{Time: "2026-09-10", StockCode: "000001.XSHE"},
	}
	calls := 0
	got := redHistoryResultsWithCachedPrices(results, nil, func(date string) map[string][]dailyBar {
		calls++
		return map[string][]dailyBar{"600000": {
			{Date: "2026-09-09", Close: 10},
			{Date: date, Open: 9.7, Close: 10.2},
		}}
	})
	if calls != 1 || got[0].T0IntradayChangePP == nil || *got[0].T0IntradayChangePP != 5 || got[1].T0IntradayChangePP != nil {
		t.Fatalf("local daily fallback or missing-price handling failed: calls=%d results=%+v", calls, got)
	}
}

func TestGobRedHistoryRetainsArchivedRulesAndRechecksV3OpeningRange(t *testing.T) {
	oldRoot, oldDB, oldShared := t0CacheRootPath, db.Dao, mainboardSharedGob
	t0CacheRootPath = t.TempDir()
	db.Dao, mainboardSharedGob = nil, nil
	t.Cleanup(func() { t0CacheRootPath, db.Dao, mainboardSharedGob = oldRoot, oldDB, oldShared })
	current, ok := gobRedRuleByKey("gob_zt_zt_pb_t1_amplitude_6_10_shadow_le_1_v3")
	if !ok {
		t.Fatal("missing current rule")
	}
	oldKey := "gob_zt_zt_pb_t1_amplitude_6_10_shadow_le_1_v2"
	archive := gobRedArchive{Date: "2026-09-10", Complete: true,
		RuleSetVersion: strings.ReplaceAll(currentGobRedRuleSetVersion(), current.Key+"@3", oldKey+"@2")}
	for i, prices := range [][2]float64{{100, 97}, {100, 100}, {100, 102}, {100, 103}, {100, 104}, {0, 102}, {100, 0}} {
		archive.Results = append(archive.Results, T0SelectionResult{
			Time: archive.Date, StockCode: string(rune('A' + i)), PrevClose: prices[0],
			RedRuleHits: []T0RedRuleHit{{Key: oldKey, Version: 2, Name: current.Name,
				EntryOpen: prices[1], EntrySource: "daily_open", ExitHorizon: "T+1_close",
				StatsStatus: "verified", Stats: &T0RedRuleStatView{}}},
		})
	}
	path := gobRedArchivePath(archive.Date)
	if err := os.MkdirAll(filepath.Dir(path), 0700); err != nil {
		t.Fatal(err)
	}
	before, err := json.Marshal(archive)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, before, 0600); err != nil {
		t.Fatal(err)
	}
	index := buildGobRedRuleHistoryIndex([]string{archive.Date}, redHistoryReplayArchiveOnly, nil)
	legacy := redHistoryIdentity{source: redHistorySourceGob, key: oldKey, version: 2}
	identity := redHistoryIdentity{source: redHistorySourceGob, key: current.Key, version: current.Version}
	if len(index.matchesByRule[legacy]) != 7 {
		t.Errorf("old-version history disappeared: got %d, want 7", len(index.matchesByRule[legacy]))
	}
	if option := index.rules[legacy]; option == nil || !strings.Contains(option.Name, "历史 v2") {
		t.Errorf("old-version history must remain selectable and identify its version: %+v", option)
	}
	if len(index.matchesByRule[identity]) != 3 {
		t.Errorf("current rule must include only flat to +3%% openings with stored prices: got %d, want 3", len(index.matchesByRule[identity]))
	}
	for _, result := range index.matchesByRule[identity] {
		hit := result.RedRuleHits[0]
		if hit.Key != current.Key || hit.Version != 3 || hit.Description != current.Description || hit.Stats != nil || hit.StatsStatus != "pending" {
			t.Errorf("converted history must use current metadata without old statistics: %+v", hit)
		}
	}
	for _, query := range []struct {
		key, version string
		count        int
	}{{oldKey, "2", 7}, {current.Key, "3", 3}} {
		response := httptest.NewRecorder()
		handleGobRedRuleHistory(response, redHistorySourceGob, query.key, query.version)
		var body struct {
			Count   int                 `json:"count"`
			Results []T0SelectionResult `json:"results"`
		}
		if err := json.Unmarshal(response.Body.Bytes(), &body); err != nil {
			t.Fatal(err)
		}
		if response.Code != http.StatusOK || body.Count != query.count || len(body.Results) != query.count {
			t.Errorf("history endpoint lost version %s: %d %s", query.version, response.Code, response.Body.String())
		}
	}
	after, err := os.ReadFile(path)
	if err != nil || string(after) != string(before) {
		t.Fatal("history query changed the archived snapshot")
	}
}

func TestReferenceRedHistoryWithoutFactsDoesNotReportZeroMatches(t *testing.T) {
	oldDB := db.Dao
	db.Dao = nil
	t.Cleanup(func() { db.Dao = oldDB })
	for _, runtime := range loadT0ReferenceRuleRuntimes() {
		if !runtime.redEntry {
			continue
		}
		response := httptest.NewRecorder()
		handleGobRedRuleHistory(response, redHistorySourceReference, runtime.rule.RuleKey, "1")
		if response.Code != http.StatusServiceUnavailable {
			t.Fatalf("missing facts must be an explicit unavailable error, not 0 matches: %d %s", response.Code, response.Body.String())
		}
		return
	}
	t.Fatal("missing reference rule")
}
