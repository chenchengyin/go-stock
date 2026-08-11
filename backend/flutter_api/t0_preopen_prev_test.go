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

func mustSaveArchive(t *testing.T, date, code string) {
	t.Helper()
	if err := saveT0SelectionArchive(date, []T0SelectionResult{{StockCode: code}}, true); err != nil {
		t.Fatal(err)
	}
}
