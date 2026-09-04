package flutter_api

import (
	"context"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"go-stock/backend/models"
)

func TestT0DailyCachePathUsesGoStockCacheDir(t *testing.T) {
	p := t0DailyCachePath("2026-08-05")
	if !strings.Contains(p, filepath.Join("t0", "daily")) {
		t.Fatalf("unexpected daily path: %s", p)
	}
	if !strings.HasSuffix(p, "t0_daily_cache_2026-08-05.gob") {
		t.Fatalf("unexpected daily filename: %s", p)
	}
}

func TestT0SelectionCachePathUsesGoStockCacheDir(t *testing.T) {
	p := t0SelectionCachePath("2026-08-05")
	if !strings.Contains(p, filepath.Join("t0", "selection")) {
		t.Fatalf("unexpected selection path: %s", p)
	}
	if !strings.HasSuffix(p, "t0_selection_2026-08-05.json") {
		t.Fatalf("unexpected selection filename: %s", p)
	}
}

func TestSaveT0SelectionArchiveWriteOnceAndForce(t *testing.T) {
	orig := t0CacheRootPath
	t0CacheRootPath = t.TempDir()
	defer func() { t0CacheRootPath = orig }()

	date := "2026-08-05"
	first := []T0SelectionResult{{StockCode: "600000.XSHG", StockName: "浦发银行", AmountYi: 10}}
	if err := saveT0SelectionArchive(date, first, false); err != nil {
		t.Fatal(err)
	}
	second := []T0SelectionResult{{StockCode: "000001.XSHE", StockName: "平安银行", AmountYi: 20}}
	if err := saveT0SelectionArchive(date, second, false); err != nil {
		t.Fatal(err)
	}
	got, ok := loadT0SelectionArchive(date)
	if !ok || got.Count != 1 || got.Results[0].StockCode != "600000.XSHG" {
		t.Fatalf("expected first snapshot kept, got %+v ok=%v", got, ok)
	}
	if err := saveT0SelectionArchive(date, second, true); err != nil {
		t.Fatal(err)
	}
	got, ok = loadT0SelectionArchive(date)
	if !ok || got.Count != 1 || got.Results[0].StockCode != "000001.XSHE" {
		t.Fatalf("expected force overwrite, got %+v ok=%v", got, ok)
	}
}

func TestTryStartT0PrewarmIdempotentWhenWarming(t *testing.T) {
	orig := t0CacheRootPath
	t0CacheRootPath = t.TempDir()
	defer func() { t0CacheRootPath = orig }()

	date := "2099-01-02"
	setT0WarmProgressForTest(date, t0WarmProgress{
		Status: t0WarmStatusWarming, DailyFetched: 10, DailyTotal: 100, StartedAt: time.Now(),
	})
	started, p := tryStartT0Prewarm(date)
	if started {
		t.Fatal("should not start second prewarm")
	}
	if p.Status != t0WarmStatusWarming || p.DailyFetched != 10 {
		t.Fatalf("unexpected progress: %+v", p)
	}
}

func TestSelectionBlockedWhenWarmingWithoutDailyFile(t *testing.T) {
	orig := t0CacheRootPath
	t0CacheRootPath = t.TempDir()
	defer func() { t0CacheRootPath = orig }()
	date := "2099-01-03"
	setT0WarmProgressForTest(date, t0WarmProgress{Status: t0WarmStatusWarming, StartedAt: time.Now()})
	if isT0DailyCacheFilePresent(date) {
		t.Fatal("file should not exist")
	}
	if !shouldReturnWarmingForSelection(date) {
		t.Fatal("selection should report warming")
	}
}

func TestHandleT0SelectionArchived(t *testing.T) {
	orig := t0CacheRootPath
	t0CacheRootPath = t.TempDir()
	defer func() { t0CacheRootPath = orig }()
	auth := newTestAuthService(t)
	user := authHTTPRegisterUser(t, auth, "13800000000", "Alice", "device-a")
	modules := NewModuleService(auth.dao)
	if err := modules.ReplaceUserAccess(context.Background(), "admin-a",
		[]string{user.User.ID}, []string{"radar.main_strategy"}); err != nil {
		t.Fatalf("grant main: %v", err)
	}

	date := "2026-08-05"
	_ = saveT0SelectionArchive(date, []T0SelectionResult{{
		StockCode: "600000.XSHG", BuySignal: BuySignalGreen,
	}}, true)

	rr := httptest.NewRecorder()
	handler := newHTTPHandler(auth.AuthService)
	handler.ServeHTTP(rr, authHTTPRequest(http.MethodGet,
		"/api/t0-selection?module_code=radar.main_strategy&archived=1&date="+date,
		user.AccessToken, ""))
	if rr.Code != 200 {
		t.Fatalf("status %d body %s", rr.Code, rr.Body.String())
	}
	if !strings.Contains(rr.Body.String(), `"archived":true`) && !strings.Contains(rr.Body.String(), `"archived": true`) {
		t.Fatalf("body: %s", rr.Body.String())
	}
	if !strings.Contains(rr.Body.String(), "600000.XSHG") {
		t.Fatalf("missing stock: %s", rr.Body.String())
	}
}

func TestT0PrewarmHistoricalLegacyArchiveIsEnrichedBeforeModuleScope(t *testing.T) {
	orig := t0CacheRootPath
	t0CacheRootPath = t.TempDir()
	defer func() { t0CacheRootPath = orig }()

	auth := newTestAuthService(t)
	useGlobalTestDB(t, auth)
	AutoMigrate()
	if err := auth.dao.Create(&models.T0PatternStat{
		Pattern:  "SY|YX|YX",
		Window:   3,
		T0N:      20,
		WinRate:  41,
		FailRate: 39,
	}).Error; err != nil {
		t.Fatal(err)
	}
	cfg := models.DefaultT0PatternConfig("test")
	if err := auth.dao.Save(&cfg).Error; err != nil {
		t.Fatal(err)
	}

	archiveDate := "2026-01-08"
	tradeDate := "2026-01-09"
	hist := []dailyBar{
		{Date: "2026-01-02", Open: 10, Close: 10.1, High: 10.2, Low: 9.9},
		{Date: "2026-01-03", Open: 10.1, Close: 10.2, High: 10.3, Low: 10.0},
		{Date: "2026-01-06", Open: 10.2, Close: 10.25, High: 10.3, Low: 10.15},
		{Date: "2026-01-07", Open: 10.25, Close: 10.3, High: 10.35, Low: 10.2},
	}
	stocks := []t0Stock{{Code: "sh600000", ShortCode: "600000", Name: "浦发银行"}}
	daily := map[string][]dailyBar{"600000": hist}
	for _, date := range []string{archiveDate, tradeDate} {
		if err := saveT0DailyCache(date, stocks, daily); err != nil {
			t.Fatal(err)
		}
	}

	if err := os.WriteFile(t0SelectionCachePath(archiveDate), []byte(`{
  "date": "2026-01-08",
  "saved_at": "2026-01-08T09:30:00Z",
  "count": 1,
  "results": [{"股票代码": "600000.XSHG", "股票名称": "浦发银行"}]
}`), 0o644); err != nil {
		t.Fatal(err)
	}

	response := buildPrewarmReadyResponseAt(tradeDate,
		time.Date(2026, 1, 9, 8, 0, 0, 0, chinaLocation()))
	if err := scopeT0ResponseResults(
		"radar.purple_strategy", response, "results"); err != nil {
		t.Fatal(err)
	}
	results, ok := response["results"].([]T0SelectionResult)
	if !ok || len(results) != 1 {
		t.Fatalf("scoped legacy historical results = %#v, want one enriched result", response["results"])
	}
	if results[0].Pattern != "SY|YX|YX" || results[0].PatternT0N != 20 ||
		results[0].PatternWinPct != 41 || results[0].PatternFailPct != 39 {
		t.Fatalf("historical result was not enriched: %#v", results[0])
	}
}

func TestIsBeforeT0AuctionCutoff(t *testing.T) {
	loc := chinaLocation()
	today := time.Date(2026, 8, 5, 0, 0, 0, 0, loc)
	date := today.Format("2006-01-02")

	cases := []struct {
		name string
		at   time.Time
		want bool
	}{
		{"midnight", time.Date(2026, 8, 5, 0, 0, 0, 0, loc), true},
		{"nine", time.Date(2026, 8, 5, 9, 0, 0, 0, loc), true},
		{"nine24", time.Date(2026, 8, 5, 9, 24, 59, 0, loc), true},
		{"nine25", time.Date(2026, 8, 5, 9, 25, 0, 0, loc), false},
		{"nine25_10", time.Date(2026, 8, 5, 9, 25, 10, 0, loc), false},
		{"other_day", time.Date(2026, 8, 5, 9, 0, 0, 0, loc) /* tradeDate mismatch below */, true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			tradeDate := date
			if tc.name == "other_day" {
				tradeDate = "2026-08-04"
				if isBeforeT0AuctionCutoff(tc.at, tradeDate) {
					t.Fatal("non-today tradeDate should not enter pre-auction window")
				}
				return
			}
			if got := isBeforeT0AuctionCutoff(tc.at, tradeDate); got != tc.want {
				t.Fatalf("at %s got %v want %v", tc.at.Format("15:04:05"), got, tc.want)
			}
		})
	}
}
