package flutter_api

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

func optimizedReplayFixture(t *testing.T, missingNext bool) (string, string) {
	t.Helper()
	oldRoot := t0CacheRootPath
	t0CacheRootPath = t.TempDir()
	t.Cleanup(func() { t0CacheRootPath = oldRoot })
	bars := optimizedTestBars()
	date := bars[30].Date
	sharedBars := append([]dailyBar(nil), bars...)
	if missingNext {
		sharedBars = sharedBars[:31]
	}
	// The current universe is different; historical replay must use its dated pool.
	snapshot := MainboardFloatCap50Gob{SchemaVersion: 2, GeneratedAt: "test", Stocks: []MainboardFloatCap50Stock{{Code: "600002", Name: "当前池", FloatMarketCapYi: 100}}, Daily: map[string][]dailyBar{"600001": sharedBars, "600002": bars}}
	if err := writeMainboardFloatCap50Gob(mainboardSharedGobPath(), snapshot); err != nil {
		t.Fatal(err)
	}
	// Deliberately conflicting old prices must lose to shared-GOB prices.
	legacy := append([]dailyBar(nil), bars...)
	legacy[28].Close = 100
	if missingNext {
		legacy = legacy[:31]
	}
	if err := saveT0DailyCache(date, []t0Stock{{Code: "sh.600001", ShortCode: "600001", Name: "历史池"}}, map[string][]dailyBar{"600001": legacy}); err != nil {
		t.Fatal(err)
	}
	return t0CacheRootPath, date
}

func TestOptimizedReplayUsesDatedPoolSharedPricesAndAdditiveArchives(t *testing.T) {
	root, date := optimizedReplayFixture(t, false)
	old := gobRedArchive{Date: date, Complete: true, Results: []T0SelectionResult{{Time: date, StockCode: "600001.XSHG", RedRuleHits: []T0RedRuleHit{{Key: "legacy", Version: 1, EntryOpen: 123.45}}}}}
	if err := saveGobRedArchive(&old); err != nil {
		t.Fatal(err)
	}
	path := gobRedArchivePath(date)
	before, _ := os.ReadFile(path)
	out := t.TempDir()
	stats, cases, coverage, err := ReplayOptimizedRedRules(root, date, date, "test", out)
	if err != nil || len(stats) != 7 || len(cases) != 7 || coverage.CompleteDays != 1 {
		t.Fatalf("stats=%+v cases=%+v coverage=%+v err=%v", stats, cases, coverage, err)
	}
	for _, s := range stats {
		if s.Status != "verified" || s.SampleCount != 1 {
			t.Fatalf("wrong stats %+v", s)
		}
	}
	for _, c := range cases {
		if c.StockCode != "600001" || c.NextDate != optimizedTestBars()[31].Date {
			t.Fatalf("wrong pool or date %+v", c)
		}
	}
	data, _ := os.ReadFile(filepath.Join(out, filepath.Base(path)))
	var got gobRedArchive
	json.Unmarshal(data, &got)
	if len(got.Results) != 1 || len(got.Results[0].RedRuleHits) != 8 || got.Results[0].RedRuleHits[0].EntryOpen != 123.45 {
		t.Fatalf("lost archive evidence: %+v", got)
	}
	after, _ := os.ReadFile(path)
	if string(before) != string(after) {
		t.Fatal("mutated source archive")
	}
}

func TestOptimizedReplayDoesNotSkipSuspendedT1(t *testing.T) {
	root, date := optimizedReplayFixture(t, true)
	stats, cases, coverage, err := ReplayOptimizedRedRules(root, date, date, "test", "")
	if err != nil || len(cases) != 0 || coverage.MissingT1 != 7 {
		t.Fatalf("suspension must not use later close: %+v %+v %v", cases, coverage, err)
	}
	if len(stats) != 0 {
		t.Fatalf("unobserved samples cannot get a win rate: %+v", stats)
	}
}
