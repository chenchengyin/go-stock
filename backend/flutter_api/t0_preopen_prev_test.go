package flutter_api

import (
	"os"
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

func mustSaveArchive(t *testing.T, date, code string) {
	t.Helper()
	if err := saveT0SelectionArchive(date, []T0SelectionResult{{StockCode: code}}, true); err != nil {
		t.Fatal(err)
	}
}
