package flutter_api

import (
	"testing"
	"time"
)

func TestResolvePrevTradingDay_TuesdayToMonday(t *testing.T) {
	orig := t0CacheRootPath
	t0CacheRootPath = t.TempDir()
	defer func() { t0CacheRootPath = orig }()

	if err := saveT0DailyCache("2026-08-25",
		[]t0Stock{{Code: "sh600000", ShortCode: "600000", Name: "浦发银行"}},
		map[string][]dailyBar{"600000": {{Date: "2026-08-25", Close: 10}}}); err != nil {
		t.Fatal(err)
	}

	got, ok := resolvePrevTradingDay("2026-08-26")
	if !ok || got != "2026-08-25" {
		t.Fatalf("got %q ok=%v", got, ok)
	}
}

func TestResolvePrevTradingDay_MondayToFriday(t *testing.T) {
	orig := t0CacheRootPath
	t0CacheRootPath = t.TempDir()
	defer func() { t0CacheRootPath = orig }()

	if err := saveT0DailyCache("2026-08-21",
		[]t0Stock{{Code: "sh600000", ShortCode: "600000", Name: "浦发银行"}},
		map[string][]dailyBar{"600000": {{Date: "2026-08-21", Close: 10}}}); err != nil {
		t.Fatal(err)
	}

	got, ok := resolvePrevTradingDay("2026-08-24")
	if !ok || got != "2026-08-21" {
		t.Fatalf("got %q ok=%v", got, ok)
	}
}

func TestNeedsPrevDayBackfill_RequiresPreopenWindow(t *testing.T) {
	orig := t0CacheRootPath
	t0CacheRootPath = t.TempDir()
	defer func() { t0CacheRootPath = orig }()

	if err := saveT0DailyCache("2026-08-25",
		[]t0Stock{{Code: "sh600000", ShortCode: "600000", Name: "浦发银行"}},
		map[string][]dailyBar{"600000": {{Date: "2026-08-25", Close: 10}}}); err != nil {
		t.Fatal(err)
	}

	loc := chinaLocation()
	inside := time.Date(2026, 8, 26, 8, 0, 0, 0, loc)
	outside := time.Date(2026, 8, 26, 9, 0, 0, 0, loc)

	_, needIn := needsPrevDayBackfill("2026-08-26", inside)
	_, needOut := needsPrevDayBackfill("2026-08-26", outside)
	if !needIn {
		t.Fatal("08:00 归档缺失时应需要补全")
	}
	if needOut {
		t.Fatal("09:00 不应触发补全")
	}
}

func TestNeedsPrevDayBackfill_SkipsWhenArchiveExists(t *testing.T) {
	orig := t0CacheRootPath
	t0CacheRootPath = t.TempDir()
	defer func() { t0CacheRootPath = orig }()

	mustSaveArchive(t, "2026-08-25", "600011.XSHG")
	if err := saveT0DailyCache("2026-08-25",
		[]t0Stock{{Code: "sh600000", ShortCode: "600000", Name: "浦发银行"}},
		map[string][]dailyBar{"600000": {{Date: "2026-08-25", Close: 10}}}); err != nil {
		t.Fatal(err)
	}
	loc := chinaLocation()
	now := time.Date(2026, 8, 26, 8, 0, 0, 0, loc)

	_, need := needsPrevDayBackfill("2026-08-26", now)
	if need {
		t.Fatal("归档已存在时不应补全")
	}
}
