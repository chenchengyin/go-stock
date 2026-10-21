package flutter_api

import (
	"encoding/json"
	"math"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/glebarez/sqlite"
	"go-stock/backend/db"
	"go-stock/backend/models"
	"gorm.io/gorm"
)

func deepDropHistoryForTest() []dailyBar {
	dates := []string{"2026-09-01", "2026-09-02", "2026-09-03", "2026-09-04", "2026-09-07", "2026-09-08", "2026-09-09"}
	hist := make([]dailyBar, len(dates))
	for i, date := range dates {
		hist[i] = dailyBar{Date: date, Open: 101, High: 102, Low: 79, Close: 101}
	}
	hist[5] = dailyBar{Date: dates[5], Open: 84, High: 85, Low: 79, Close: 80}
	hist[6] = dailyBar{Date: dates[6], Open: 79, High: 80, Low: 70, Close: 78}
	return hist
}

func TestGobRedDeepDropV3PositionAndOpeningBoundaries(t *testing.T) {
	rule, ok := gobRedRuleByKey(gobRedT7DropRuleKey)
	if !ok || rule.Version != 3 || rule.ExitHorizon != gobRedBodyHorizon {
		t.Fatalf("missing versioned candle-body rule: %+v", rule)
	}
	for _, tc := range []struct {
		name string
		bar  dailyBar
		want bool
	}{
		{"below 80 percent", dailyBar{Open: 79, High: 80, Low: 70, Close: 77.99}, true},
		{"exactly 80 percent", dailyBar{Open: 79, High: 80, Low: 70, Close: 78}, true},
		{"above 80 percent", dailyBar{Open: 79, High: 80, Low: 70, Close: 78.01}, false},
		{"zero range", dailyBar{Open: 78, High: 78, Low: 78, Close: 78}, false},
		{"invalid low", dailyBar{Open: 79, High: 80, Close: 78}, false},
		{"close below low", dailyBar{Open: 79, High: 80, Low: 79, Close: 78}, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			hist := deepDropHistoryForTest()
			tc.bar.Date = hist[6].Date
			hist[6] = tc.bar
			if got := rule.MatchHistory(hist); got != tc.want {
				t.Fatalf("history match=%v want %v", got, tc.want)
			}
		})
	}
	for _, tc := range []struct {
		open, previous float64
		want           bool
	}{
		{4.8, 5, true}, {4.9, 5, false}, {96, 100, true}, {98, 100, false}, {97, 100, true}, {0, 100, false},
	} {
		if got := rule.MatchOpen(tc.open, tc.previous); got != tc.want {
			t.Fatalf("open=%v previous=%v got=%v want=%v", tc.open, tc.previous, got, tc.want)
		}
	}
	if !strings.Contains(rule.Description, "80%") || !strings.Contains(rule.Description, "实体") {
		t.Fatalf("missing updated explanation: %q", rule.Description)
	}
}

func TestGobRedDeepDropV3OriginalThresholdsRemainInclusive(t *testing.T) {
	rule, _ := gobRedRuleByKey(gobRedT7DropRuleKey)
	hist := deepDropHistoryForTest()
	hist[0].Close = 100
	hist[5] = dailyBar{Date: hist[5].Date, Open: 100, High: 101, Low: 99, Close: 100}
	hist[6] = dailyBar{Date: hist[6].Date, Open: 78, High: 80.1, Low: 73, Close: 77.75}
	if !rule.MatchHistory(hist) {
		t.Fatal("exact 22.25% decline and 7.1% amplitude must pass")
	}
	hist[6].Close = 77.76
	if rule.MatchHistory(hist) {
		t.Fatal("decline below 22.25% must fail")
	}
	hist[6].Close = 77.75
	hist[6].High = 80.09
	if rule.MatchHistory(hist) {
		t.Fatal("amplitude below 7.1% must fail")
	}
}

func TestGobRedDeepDropBodyStatsStrictThresholdUnionAndMissing(t *testing.T) {
	hit := T0RedRuleHit{Key: gobRedT7DropRuleKey, Version: 3, EntrySource: "daily_open", ExitHorizon: gobRedBodyHorizon, EntryOpen: 8}
	samples := []gobRedStatSample{
		{Date: "2026-09-01", StockCode: "600001", Hit: hit, T0Close: 8.2, NextOpen: 8, NextClose: 8.2}, // exactly 2.5% on both: neither
		{Date: "2026-09-02", StockCode: "600002", Hit: hit, T0Close: 8.21, NextOpen: 8, NextClose: 8.1},
		{Date: "2026-09-03", StockCode: "600003", Hit: hit, T0Close: 7.9, NextOpen: 6, NextClose: 6.2}, // T1 body passes despite investment loss
		{Date: "2026-09-04", StockCode: "600004", Hit: hit, T0Close: 8.3, NextOpen: 8, NextClose: 8.3},
		{Date: "2026-09-07", StockCode: "600005", Hit: hit, T0Close: 8.3, NextClose: 8.3}, // missing T1 open: exclude, not a failure
	}
	samples = append(samples, samples[3]) // a duplicated stock-day must not inflate the union
	stats := aggregateGobRedRuleStats(samples, "2026-09-01", "2026-09-07", "body-v1", "body-test")
	if len(stats) != 1 || stats[0].SampleCount != 4 || stats[0].T0BodyTargetCount != 2 ||
		stats[0].T1BodyTargetCount != 2 || stats[0].TargetCount != 3 || stats[0].TargetRate != 75 ||
		stats[0].PositiveCount != 3 {
		t.Fatalf("body target stats must be distinct from investment returns: %+v", stats)
	}
	if gobRedBodyTargetReached(8, 8.2) || !gobRedBodyTargetReached(8, 8.21) || gobRedBodyTargetReached(8, 7.7) {
		t.Fatal("body target must be signed and strictly greater than 2.5 percent")
	}
}

func TestGobRedDeepDropBodyStatsPersistAndDoNotBorrowV2(t *testing.T) {
	previous := db.Dao
	t.Cleanup(func() { db.Dao = previous })
	dao, err := gorm.Open(sqlite.Open(filepath.Join(t.TempDir(), "body.db")), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	db.Dao = dao
	if err := dao.AutoMigrate(&models.T0RedRuleStat{}); err != nil {
		t.Fatal(err)
	}
	hit := T0RedRuleHit{Key: gobRedT7DropRuleKey, Version: 3, EntrySource: "realtime_open", ExitHorizon: gobRedBodyHorizon, StatsStatus: "pending"}
	legacy := models.T0RedRuleStat{RuleKey: gobRedT7DropLegacyRuleKey, RuleVersion: 2, EntrySource: "daily_open", ExitHorizon: "T0_high", PeriodKey: "all", SampleCount: 361, TargetRate: 99, Status: "verified"}
	if err := saveGobRedRuleStats(dao, []models.T0RedRuleStat{legacy}); err != nil {
		t.Fatal(err)
	}
	result := []T0SelectionResult{{RedRuleHits: []T0RedRuleHit{hit}}}
	if got := enrichGobRedRuleStats(result)[0].RedRuleHits[0]; got.Stats != nil || got.StatsStatus != "pending" {
		t.Fatalf("new rule borrowed old high-touch statistics: %+v", got)
	}
	daily := hit
	daily.EntrySource = "daily_open"
	daily.EntryOpen = 8
	stats := aggregateGobRedRuleStats([]gobRedStatSample{{Date: "2026-09-01", StockCode: "600001", Hit: daily, T0Close: 8.3, NextOpen: 8, NextClose: 8.1}}, "2026-09-01", "2026-09-01", "body-v1", "test")
	if err := saveGobRedRuleStats(dao, stats); err != nil {
		t.Fatal(err)
	}
	stats[0].T1BodyTargetCount = 1
	if err := saveGobRedRuleStats(dao, stats); err != nil {
		t.Fatal(err)
	}
	got := enrichGobRedRuleStats(result)[0].RedRuleHits[0]
	if got.Stats == nil || got.Stats.T0BodyTargetRate == nil || got.Stats.T1BodyTargetRate == nil ||
		math.Abs(*got.Stats.T0BodyTargetRate-100) > 1e-9 || math.Abs(*got.Stats.T1BodyTargetRate-100) > 1e-9 {
		t.Fatalf("body counts not persisted/upserted/enriched: %+v", got)
	}
}

func TestGobRedDeepDropBodyReplayReadsFollowingSnapshot(t *testing.T) {
	previous := t0CacheRootPath
	t0CacheRootPath = t.TempDir()
	t.Cleanup(func() { t0CacheRootPath = previous })
	stocks := []t0Stock{{Code: "sh.600101", ShortCode: "600101", Name: "七日深跌测试"}}
	bars := append(deepDropHistoryForTest(), dailyBar{Date: "2026-09-10", Open: 75, High: 79, Low: 74, Close: 78})
	if err := saveT0DailyCache("2026-09-10", stocks, map[string][]dailyBar{"600101": bars}); err != nil {
		t.Fatal(err)
	}
	next := append(append([]dailyBar(nil), bars...), dailyBar{Date: "2026-09-11", Open: 77, High: 81, Low: 76, Close: 80})
	if err := saveT0DailyCache("2026-09-11", stocks, map[string][]dailyBar{"600101": next}); err != nil {
		t.Fatal(err)
	}
	stats, cases, coverage, err := ReplayGobRedRuleStats(t0CacheRootPath, "2026-09-10", "2026-09-10", "two-session-body")
	if err != nil {
		t.Fatal(err)
	}
	for _, stat := range stats {
		if stat.RuleKey == gobRedT7DropRuleKey {
			if stat.SampleCount != 1 || stat.T0BodyTargetCount != 1 || stat.T1BodyTargetCount != 1 || stat.TargetCount != 1 || stat.Status != "verified" {
				t.Fatalf("following-snapshot stats=%+v coverage=%+v", stat, coverage)
			}
			if len(cases) != 1 || cases[0].NextDate != "2026-09-11" {
				t.Fatalf("wrong next date: %+v", cases)
			}
			return
		}
	}
	t.Fatalf("body rule missing from replay: stats=%+v coverage=%+v", stats, coverage)
}

func TestGobRedDeepDropBodyReplayDoesNotSkipMissingNextSession(t *testing.T) {
	previous := t0CacheRootPath
	t0CacheRootPath = t.TempDir()
	t.Cleanup(func() { t0CacheRootPath = previous })
	stocks := []t0Stock{{Code: "sh.600101", ShortCode: "600101"}, {Code: "sh.600102", ShortCode: "600102"}}
	bars := append(deepDropHistoryForTest(), dailyBar{Date: "2026-09-10", Open: 75, High: 79, Low: 74, Close: 78})
	if err := saveT0DailyCache("2026-09-10", stocks, map[string][]dailyBar{"600101": bars, "600102": bars[:6]}); err != nil {
		t.Fatal(err)
	}
	laterOnly := append(append([]dailyBar(nil), bars...), dailyBar{Date: "2026-09-14", Open: 75, High: 80, Low: 74, Close: 80})
	witness := []dailyBar{{Date: "2026-09-11", Open: 10, High: 11, Low: 9, Close: 10}}
	if err := saveT0DailyCache("2026-09-11", stocks, map[string][]dailyBar{"600101": laterOnly, "600102": witness}); err != nil {
		t.Fatal(err)
	}
	stats, cases, coverage, err := ReplayGobRedRuleStats(t0CacheRootPath, "2026-09-10", "2026-09-10", "missing-next-session")
	if err != nil || coverage.MissingT1 != 1 || len(cases) != 0 {
		t.Fatalf("must not borrow T+2: stats=%+v cases=%+v coverage=%+v err=%v", stats, cases, coverage, err)
	}
	for _, stat := range stats {
		if stat.RuleKey == gobRedT7DropRuleKey && (stat.SampleCount != 0 || stat.Status != "partial") {
			t.Fatalf("missing next session must remain pending: %+v", stat)
		}
	}
}

func TestGobRedDeepDropBodyReplayChecksEarlierSessionDespiteFutureBar(t *testing.T) {
	previous := t0CacheRootPath
	t0CacheRootPath = t.TempDir()
	t.Cleanup(func() { t0CacheRootPath = previous })
	stocks := []t0Stock{{Code: "sh.600101", ShortCode: "600101"}}
	bars := append(deepDropHistoryForTest(), dailyBar{Date: "2026-09-10", Open: 75, High: 79, Low: 74, Close: 78})
	future := append(append([]dailyBar(nil), bars...), dailyBar{Date: "2026-09-14", Open: 70, High: 72, Low: 69, Close: 71})
	if err := saveT0DailyCache("2026-09-10", stocks, map[string][]dailyBar{"600101": future}); err != nil {
		t.Fatal(err)
	}
	next := append(append([]dailyBar(nil), bars...), dailyBar{Date: "2026-09-11", Open: 78, High: 81, Low: 77, Close: 80})
	if err := saveT0DailyCache("2026-09-11", stocks, map[string][]dailyBar{"600101": next}); err != nil {
		t.Fatal(err)
	}
	stats, cases, coverage, err := ReplayGobRedRuleStats(t0CacheRootPath, "2026-09-10", "2026-09-10", "earlier-session-witness")
	if err != nil || len(cases) != 1 || cases[0].NextDate != "2026-09-11" || cases[0].PnL <= 0 {
		t.Fatalf("future bar skipped the actual next session: stats=%+v cases=%+v coverage=%+v err=%v", stats, cases, coverage, err)
	}
}

func TestGobRedDeepDropBodyReplayFindsExplicitT1InLaterSnapshot(t *testing.T) {
	previous := t0CacheRootPath
	t0CacheRootPath = t.TempDir()
	t.Cleanup(func() { t0CacheRootPath = previous })
	stocks := []t0Stock{{Code: "sh.600101", ShortCode: "600101"}, {Code: "sh.600102", ShortCode: "600102"}}
	bars := append(deepDropHistoryForTest(), dailyBar{Date: "2026-09-10", Open: 75, High: 79, Low: 74, Close: 78})
	if err := saveT0DailyCache("2026-09-10", stocks, map[string][]dailyBar{"600101": bars, "600102": bars[:6]}); err != nil {
		t.Fatal(err)
	}
	calendar := []dailyBar{{Date: "2026-09-11", Open: 10, High: 11, Low: 9, Close: 10}}
	if err := saveT0DailyCache("2026-09-11", stocks, map[string][]dailyBar{"600101": bars, "600102": calendar}); err != nil {
		t.Fatal(err)
	}
	delayed := append(append([]dailyBar(nil), bars...), dailyBar{Date: "2026-09-11", Open: 77, High: 81, Low: 76, Close: 80}, dailyBar{Date: "2026-09-14", Open: 80, High: 80, Low: 70, Close: 70})
	if err := saveT0DailyCache("2026-09-14", stocks, map[string][]dailyBar{"600101": delayed, "600102": calendar}); err != nil {
		t.Fatal(err)
	}
	stats, cases, coverage, err := ReplayGobRedRuleStats(t0CacheRootPath, "2026-09-10", "2026-09-10", "delayed-T1")
	if err != nil || len(cases) != 1 || cases[0].NextDate != "2026-09-11" || coverage.MissingT1 != 0 {
		t.Fatalf("delayed T1 not recovered: stats=%+v cases=%+v coverage=%+v err=%v", stats, cases, coverage, err)
	}
	for _, stat := range stats {
		if stat.RuleKey == gobRedT7DropRuleKey && (stat.SampleCount != 1 || stat.T1BodyTargetCount != 1 || stat.TargetCount != 1) {
			t.Fatalf("wrong dated candle: %+v", stat)
		}
	}
}

// Optional local regression against the independently scanned historical
// prices. The large research data stays outside the repository.
func TestGobRedDeepDropResearchFixture(t *testing.T) {
	path := os.Getenv("GO_STOCK_DEEP_DROP_FIXTURE")
	if path == "" {
		t.Skip("set GO_STOCK_DEEP_DROP_FIXTURE to the research raw.json")
	}
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	var fixture struct {
		Cases []struct {
			Date, Code string
			Hist       []dailyBar
			T0         dailyBar
			Next       *dailyBar
			Aligned    bool
		} `json:"cases"`
	}
	if err := json.Unmarshal(data, &fixture); err != nil {
		t.Fatal(err)
	}
	rule, _ := gobRedRuleByKey(gobRedT7DropRuleKey)
	var samples []gobRedStatSample
	recent := 0
	for _, c := range fixture.Cases {
		if !c.Aligned || !rule.MatchHistory(c.Hist) || !rule.MatchOpen(c.T0.Open, c.Hist[len(c.Hist)-1].Close) {
			continue
		}
		if c.Next == nil {
			t.Fatalf("missing next bar %s/%s", c.Date, c.Code)
		}
		if strings.HasPrefix(c.Date, "2026") {
			recent++
		}
		samples = append(samples, gobRedStatSample{Date: c.Date, StockCode: c.Code, Hit: T0RedRuleHit{Key: rule.Key, Version: rule.Version, EntrySource: "daily_open", ExitHorizon: rule.ExitHorizon, EntryOpen: c.T0.Open}, T0Close: c.T0.Close, NextOpen: c.Next.Open, NextClose: c.Next.Close})
	}
	stats := aggregateGobRedRuleStats(samples, "2024-01-02", "2026-10-18", "fixture", "independent")
	if len(stats) != 1 || stats[0].SampleCount != 331 || recent != 71 || stats[0].T0BodyTargetCount != 218 || stats[0].T1BodyTargetCount != 118 || stats[0].TargetCount != 259 || stats[0].PositiveCount != 231 {
		t.Fatalf("historical body regression: recent=%d stats=%+v", recent, stats)
	}
	var recentSamples []gobRedStatSample
	for _, s := range samples {
		if strings.HasPrefix(s.Date, "2026") {
			recentSamples = append(recentSamples, s)
		}
	}
	latest := aggregateGobRedRuleStats(recentSamples, "2026-01-01", "2026-10-18", "fixture", "independent")
	if len(latest) != 1 || latest[0].SampleCount != 71 || latest[0].T0BodyTargetCount != 28 || latest[0].T1BodyTargetCount != 30 || latest[0].TargetCount != 51 || latest[0].PositiveCount != 35 {
		t.Fatalf("2026 body regression: %+v", latest)
	}
}
