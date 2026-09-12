package flutter_api

import (
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestIsPreopenPrevResultWindow(t *testing.T) {
	loc := chinaLocation()
	today := time.Date(2026, 8, 12, 0, 0, 0, 0, loc).Format("2006-01-02")

	cases := []struct {
		name      string
		now       time.Time
		tradeDate string
		want      bool
	}{
		{"00:00 窗口起点", time.Date(2026, 8, 12, 0, 0, 0, 0, loc), today, true},
		{"08:59 窗口内", time.Date(2026, 8, 12, 8, 59, 0, 0, loc), today, true},
		{"09:00 窗口结束", time.Date(2026, 8, 12, 9, 0, 0, 0, loc), today, false},
		{"非当天不生效", time.Date(2026, 8, 12, 8, 0, 0, 0, loc), "2026-08-11", false},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := isPreopenPrevResultWindow(c.now, c.tradeDate); got != c.want {
				t.Fatalf("got %v want %v", got, c.want)
			}
		})
	}
}

func TestIsPreopenPrevResultWindow_ConvertsToShanghai(t *testing.T) {
	utc := time.Date(2026, 8, 11, 23, 0, 0, 0, time.UTC)
	tradeDate := utc.In(chinaLocation()).Format("2006-01-02")
	if !isPreopenPrevResultWindow(utc, tradeDate) {
		t.Fatal("UTC 输入应按上海时区判定在窗口内")
	}
}

func TestFindLatestSelectionArchiveBefore(t *testing.T) {
	orig := t0CacheRootPath
	t0CacheRootPath = t.TempDir()
	defer func() { t0CacheRootPath = orig }()

	mustSaveArchive(t, "2026-08-07", "600007.XSHG")
	mustSaveArchive(t, "2026-08-10", "600010.XSHG")
	mustSaveArchive(t, "2026-08-12", "600012.XSHG")

	got, ok := findLatestSelectionArchiveBefore("2026-08-10")
	if !ok || got.Date != "2026-08-07" {
		t.Fatalf("got %+v ok=%v", got, ok)
	}

	got, ok = findLatestSelectionArchiveBefore("2026-08-11")
	if !ok || got.Date != "2026-08-10" {
		t.Fatalf("got %+v ok=%v", got, ok)
	}

	if _, ok := findLatestSelectionArchiveBefore("2026-08-06"); ok {
		t.Fatal("不应找到更早归档")
	}
}

func TestFindLatestSelectionArchiveBefore_SkipsCorrupt(t *testing.T) {
	orig := t0CacheRootPath
	t0CacheRootPath = t.TempDir()
	defer func() { t0CacheRootPath = orig }()

	mustSaveArchive(t, "2026-08-07", "600007.XSHG")
	_ = ensureT0CacheDirs()
	if err := os.WriteFile(t0SelectionCachePath("2026-08-10"), []byte("{bad json"), 0o644); err != nil {
		t.Fatal(err)
	}

	got, ok := findLatestSelectionArchiveBefore("2026-08-11")
	if !ok || got.Date != "2026-08-07" {
		t.Fatalf("应跳过损坏文件回退到 08-07，got %+v ok=%v", got, ok)
	}
}

func TestBuildPrewarmReadyResponseInjectsHistorical(t *testing.T) {
	orig := t0CacheRootPath
	t0CacheRootPath = t.TempDir()
	defer func() { t0CacheRootPath = orig }()

	if err := saveT0DailyCache("2026-08-12",
		[]t0Stock{{Code: "sh600000", ShortCode: "600000", Name: "浦发银行"}},
		map[string][]dailyBar{"600000": {{Date: "2026-08-11", Close: 10}}}); err != nil {
		t.Fatal(err)
	}
	mustSaveArchive(t, "2026-08-11", "600011.XSHG")

	resp := buildPrewarmReadyResponseAt("2026-08-12",
		time.Date(2026, 8, 12, 8, 0, 0, 0, chinaLocation()))
	if resp["historical"] != true {
		t.Fatalf("historical=%v", resp["historical"])
	}
	if resp["display_date"] != "2026-08-11" {
		t.Fatalf("display_date=%v", resp["display_date"])
	}
	results, ok := resp["results"].([]T0SelectionResult)
	if !ok || len(results) != 1 || results[0].StockCode != "600011.XSHG" {
		t.Fatalf("results=%v", resp["results"])
	}
}

func TestBuildPrewarmReadyResponseEnrichesHistoricalDisplayRules(t *testing.T) {
	orig := t0CacheRootPath
	t0CacheRootPath = t.TempDir()
	defer func() { t0CacheRootPath = orig }()

	daily := map[string][]dailyBar{
		"600000": {
			{Date: "2026-09-01", Close: 10},
			{Date: "2026-09-02", Open: 10, Close: 11, High: 11, Low: 10},
			{Date: "2026-09-03", Open: 11, Close: 12.1, High: 12.1, Low: 11},
			{Date: "2026-09-04", Open: 12.4, Close: 12.1, High: 12.4, Low: 12},
			{Date: "2026-09-05", Open: 12.25, Close: 12.2, High: 12.3, Low: 12.1},
		},
	}
	stocks := []t0Stock{{Code: "sh600000", ShortCode: "600000", Name: "历史命中股"}}
	if err := saveT0DailyCache("2026-09-05", stocks, daily); err != nil {
		t.Fatal(err)
	}
	if err := saveT0DailyCache("2026-09-06", stocks, daily); err != nil {
		t.Fatal(err)
	}
	if err := saveT0SelectionArchiveFull(&t0SelectionArchive{
		Date:  "2026-09-05",
		Count: 1,
		Results: []T0SelectionResult{{
			Time:      "2026-09-05",
			StockCode: "600000.XSHG",
			StockName: "历史命中股",
			OpenGap:   1.2,
			BuySignal: BuySignalBlue,
		}},
	}, true); err != nil {
		t.Fatal(err)
	}

	resp := buildPrewarmReadyResponseAt("2026-09-06",
		time.Date(2026, 9, 6, 8, 0, 0, 0, chinaLocation()))
	results, ok := resp["results"].([]T0SelectionResult)
	if !ok || len(results) != 1 {
		t.Fatalf("results=%v", resp["results"])
	}
	for _, hit := range results[0].DisplayRuleHits {
		if hit == displayRuleZtZtBearishT0 {
			return
		}
	}
	t.Fatalf("historical display rule missing: %v", results[0].DisplayRuleHits)
}

func TestBuildPrewarmReadyResponseNoHistoricalAfter0900(t *testing.T) {
	orig := t0CacheRootPath
	t0CacheRootPath = t.TempDir()
	defer func() { t0CacheRootPath = orig }()

	if err := saveT0DailyCache("2026-08-12",
		[]t0Stock{{Code: "sh600000", ShortCode: "600000", Name: "浦发银行"}},
		map[string][]dailyBar{"600000": {{Date: "2026-08-11", Close: 10}}}); err != nil {
		t.Fatal(err)
	}
	mustSaveArchive(t, "2026-08-11", "600011.XSHG")

	resp := buildPrewarmReadyResponseAt("2026-08-12",
		time.Date(2026, 8, 12, 9, 0, 0, 0, chinaLocation()))
	if _, has := resp["historical"]; has {
		t.Fatal("09:00 起不应注入历史结果")
	}
	if _, has := resp["results"]; has {
		t.Fatal("09:00 起不应带 results")
	}
}

func TestHandleT0SelectionWeekendReturnsNoData(t *testing.T) {
	orig := t0CacheRootPath
	t0CacheRootPath = t.TempDir()
	defer func() { t0CacheRootPath = orig }()

	for _, date := range []string{"2026-09-12", "2026-09-13"} {
		t.Run(date, func(t *testing.T) {
			if err := saveT0DailyCache(date,
				[]t0Stock{{Code: "sh600000", ShortCode: "600000", Name: "周末测试股"}},
				map[string][]dailyBar{"600000": {{Date: "2026-09-11", Close: 10}}}); err != nil {
				t.Fatal(err)
			}

			req := httptest.NewRequest(http.MethodGet,
				"/api/t0-selection?module_code=radar.main_strategy&date="+date, nil)
			rr := httptest.NewRecorder()
			handleT0Selection(rr, req)

			body := rr.Body.String()
			if rr.Code != http.StatusOK {
				t.Fatalf("status %d body %s", rr.Code, body)
			}
			if !strings.Contains(body, `"no_data":true`) {
				t.Fatalf("weekend response should be marked no_data: %s", body)
			}
			if strings.Contains(body, `"candidates"`) || strings.Contains(body, `"historical":true`) {
				t.Fatalf("weekend response must not expose previous-day data: %s", body)
			}
		})
	}
}

func TestHandleT0SelectionWeekendPurpleReturnsNoData(t *testing.T) {
	orig := t0CacheRootPath
	t0CacheRootPath = t.TempDir()
	defer func() { t0CacheRootPath = orig }()

	date := "2026-09-12"
	if err := saveT0DailyCache(date,
		[]t0Stock{{Code: "sh600000", ShortCode: "600000", Name: "周末测试股"}},
		map[string][]dailyBar{"600000": {{Date: "2026-09-11", Close: 10}}}); err != nil {
		t.Fatal(err)
	}

	req := httptest.NewRequest(http.MethodGet,
		"/api/t0-selection?module_code=radar.purple_strategy&date="+date, nil)
	rr := httptest.NewRecorder()
	handleT0Selection(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("status %d body %s", rr.Code, rr.Body.String())
	}
	if !strings.Contains(rr.Body.String(), `"no_data":true`) {
		t.Fatalf("purple weekend response should be marked no_data: %s", rr.Body.String())
	}
}

func TestListSelectionArchiveDates(t *testing.T) {
	orig := t0CacheRootPath
	t0CacheRootPath = t.TempDir()
	defer func() { t0CacheRootPath = orig }()

	mustSaveArchive(t, "2026-08-06", "600006.XSHG")
	mustSaveArchive(t, "2026-08-11", "600011.XSHG")
	mustSaveArchive(t, "2026-08-10", "600010.XSHG")
	_ = ensureT0CacheDirs()
	if err := os.WriteFile(filepath.Join(filepath.Dir(t0SelectionCachePath("x")), "t0_selection_bad.json"), []byte("{}"), 0o644); err != nil {
		t.Fatal(err)
	}

	got := listSelectionArchiveDates()
	want := []string{"2026-08-11", "2026-08-10", "2026-08-06"}
	if len(got) != len(want) {
		t.Fatalf("got %v want %v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("got %v want %v", got, want)
		}
	}

	mustSaveArchive(t, "2026-08-12", "600012.XSHG")
	got = listSelectionArchiveDates()
	if len(got) != 4 || got[0] != "2026-08-12" {
		t.Fatalf("date cache was not invalidated after archive write: %v", got)
	}
}

func TestHandleT0SelectionListDates(t *testing.T) {
	orig := t0CacheRootPath
	t0CacheRootPath = t.TempDir()
	defer func() { t0CacheRootPath = orig }()
	mustSaveArchive(t, "2026-08-10", "600010.XSHG")

	req := httptest.NewRequest(http.MethodGet, "/api/t0-selection?list_dates=1", nil)
	rr := httptest.NewRecorder()
	handleT0Selection(rr, req)
	if rr.Code != 200 {
		t.Fatalf("status %d body %s", rr.Code, rr.Body.String())
	}
	if !strings.Contains(rr.Body.String(), "2026-08-10") {
		t.Fatalf("body: %s", rr.Body.String())
	}
}

func TestHandleT0SelectionListDatesKeepsRedDatesWithoutDailyCaches(t *testing.T) {
	orig := t0CacheRootPath
	t0CacheRootPath = t.TempDir()
	defer func() { t0CacheRootPath = orig }()

	mustSaveArchive(t, "2026-09-03", "600003.XSHG")
	mustSaveArchive(t, "2026-09-04", "600004.XSHG")

	req := httptest.NewRequest(http.MethodGet,
		"/api/t0-selection?list_dates=1&module_code=radar.red_strategy", nil)
	rr := httptest.NewRecorder()
	handleT0Selection(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("status %d body %s", rr.Code, rr.Body.String())
	}
	if !strings.Contains(rr.Body.String(), "2026-09-04") ||
		!strings.Contains(rr.Body.String(), "2026-09-03") {
		t.Fatalf("red dates without daily cache must remain selectable: %s", rr.Body.String())
	}
}

func mustSaveArchive(t *testing.T, date, code string) {
	t.Helper()
	if err := saveT0SelectionArchive(date, []T0SelectionResult{{StockCode: code}}, true); err != nil {
		t.Fatal(err)
	}
}
