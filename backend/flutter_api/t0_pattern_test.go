package flutter_api

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"go-stock/backend/db"
	"go-stock/backend/models"
)

func TestSignalFromRates(t *testing.T) {
	cfg := models.DefaultT0PatternConfig("test")
	cases := []struct {
		name      string
		win, fail float64
		n         int
		want      string
	}{
		{"blue super win", 50, 35, 20, BuySignalBlue},
		{"blue boundary", 45, 40, 10, BuySignalBlue},
		{"orange boundary", 55, 45, 10, BuySignalOrange},
		{"orange overrides blue", 60, 35, 10, BuySignalOrange},
		{"orange requires win threshold", 54.9, 45, 10, BuySignalGreen},
		{"orange requires earn threshold", 55, 45.1, 10, BuySignalYellow},
		{"green not blue", 41, 44, 56, BuySignalGreen},
		{"green below blue win", 44, 38, 12, BuySignalGreen},
		{"red high fail", 19, 62, 42, BuySignalRed},
		{"red low win", 20, 48, 30, BuySignalRed},
		{"yellow", 28, 48, 30, BuySignalYellow},
		{"insufficient small sample", 50, 40, 5, BuySignalInsufficient},
		{"insufficient zero", 41, 44, 0, BuySignalInsufficient},
		{"green boundary", 30, 45, 10, BuySignalGreen},
		{"red fail boundary", 25, 52.1, 10, BuySignalRed},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got := signalFromRates(c.win, c.fail, c.n, cfg)
			if got != c.want {
				t.Fatalf("signalFromRates(%v,%v,%v)=%q want %q", c.win, c.fail, c.n, got, c.want)
			}
		})
	}
}

func TestEnrichResultWithPattern(t *testing.T) {
	dir := t.TempDir()
	dbPath := filepath.Join(dir, "test.db")
	db.Init(dbPath)
	AutoMigrate()

	stat := models.T0PatternStat{
		Pattern:  "SY|YX|YX",
		Window:   3,
		T0N:      20,
		WinRate:  35,
		FailRate: 45,
	}
	if err := db.Dao.Create(&stat).Error; err != nil {
		t.Fatal(err)
	}
	cfg := models.DefaultT0PatternConfig("test")
	if err := db.Dao.Save(&cfg).Error; err != nil {
		t.Fatal(err)
	}

	hist := []dailyBar{
		{Date: "2026-01-02", Open: 10, Close: 10.1, High: 10.2, Low: 9.9},
		{Date: "2026-01-03", Open: 10.1, Close: 10.2, High: 10.3, Low: 10.0},
		{Date: "2026-01-06", Open: 10.2, Close: 10.25, High: 10.3, Low: 10.15},
		{Date: "2026-01-07", Open: 10.25, Close: 10.3, High: 10.35, Low: 10.2},
	}
	var r T0SelectionResult
	enrichResultWithPattern(&r, hist)
	if r.Pattern != "SY|YX|YX" {
		t.Fatalf("pattern=%q", r.Pattern)
	}
	if r.BuySignal != BuySignalGreen {
		t.Fatalf("signal=%q win=%v fail=%v", r.BuySignal, r.PatternWinPct, r.PatternFailPct)
	}
}

func TestSelectT0ResultsForModuleUsesIndependentStrategyViews(t *testing.T) {
	results := []T0SelectionResult{
		{StockCode: "purple", PatternT0N: 2, PatternWinPct: 40.1, PatternFailPct: 39.9, BuySignal: BuySignalGreen},
		{StockCode: "purple-win-boundary", PatternT0N: 2, PatternWinPct: 40, PatternFailPct: 39.9, BuySignal: BuySignalGreen},
		{StockCode: "purple-earn-boundary", PatternT0N: 2, PatternWinPct: 40.1, PatternFailPct: 40, BuySignal: BuySignalGreen},
		{StockCode: "blue", PatternT0N: 1, BuySignal: BuySignalBlue},
		{StockCode: "other", PatternT0N: 2, PatternWinPct: 40.1, PatternFailPct: 40.1, BuySignal: BuySignalGreen},
	}
	original := append([]T0SelectionResult(nil), results...)

	purple, err := selectT0ResultsForModule("radar.purple_strategy", results)
	if err != nil {
		t.Fatalf("select purple: %v", err)
	}
	if got := []string{purple[0].StockCode}; !reflect.DeepEqual(got, []string{"purple"}) {
		t.Fatalf("purple results = %v", got)
	}
	if &purple[0] == &results[0] {
		t.Fatal("purple results share the input backing array")
	}
	purple[0].StockCode = "mutated"

	blue, err := selectT0ResultsForModule("radar.blue_strategy", results)
	if err != nil {
		t.Fatalf("select blue: %v", err)
	}
	if len(blue) != 1 || blue[0].StockCode != "blue" {
		t.Fatalf("blue results = %#v", blue)
	}
	if &blue[0] == &results[3] {
		t.Fatal("blue results share the input backing array")
	}
	blue[0].StockCode = "mutated"

	main, err := selectT0ResultsForModule("radar.main_strategy", results)
	if err != nil {
		t.Fatalf("select main: %v", err)
	}
	if !reflect.DeepEqual(main, results) {
		t.Fatalf("main results = %#v, want %#v", main, results)
	}
	if !reflect.DeepEqual(results, original) {
		t.Fatalf("input results mutated: got %#v want %#v", results, original)
	}
}

func TestSelectT0ResultsForModuleRejectsUnknownModule(t *testing.T) {
	if _, err := selectT0ResultsForModule("radar.monitored", nil); !IsAuthCode(err, "INVALID_ARGUMENT") {
		t.Fatalf("error = %v, want INVALID_ARGUMENT", err)
	}
}

func TestPatternBuySignalArchivedSmoke(t *testing.T) {
	dbPath := stockDBPath(t)
	if _, err := os.Stat(dbPath); err != nil {
		t.Skip("stock.db not found")
	}
	if err := initT0CacheRoot(); err != nil {
		t.Fatalf("init cache root: %v", err)
	}
	archive := t0SelectionCachePath("2026-08-11")
	if _, err := os.Stat(archive); err != nil {
		t.Skip("archived selection not found")
	}

	db.Init(dbPath)
	AutoMigrate()
	auth := newTestAuthService(t)
	user := authHTTPRegisterUser(t, auth, "13800000000", "Alice", "device-a")
	modules := NewModuleService(auth.dao)
	if err := modules.ReplaceUserAccess(context.Background(), "admin-a",
		[]string{user.User.ID}, []string{"radar.main_strategy"}); err != nil {
		t.Fatalf("grant main: %v", err)
	}

	rr := httptest.NewRecorder()
	handler := newHTTPHandler(auth.AuthService)
	handler.ServeHTTP(rr, authHTTPRequest(http.MethodGet,
		"/api/t0-selection?module_code=radar.main_strategy&archived=1&date=2026-08-11",
		user.AccessToken, ""))
	if rr.Code != http.StatusOK {
		t.Fatalf("status %d body %s", rr.Code, rr.Body.String())
	}
	body := rr.Body.String()
	if !strings.Contains(body, `"买入信号"`) {
		t.Fatalf("missing buy signal field: %s", body[:min(len(body), 500)])
	}

	var resp struct {
		Results []T0SelectionResult `json:"results"`
	}
	if err := json.Unmarshal(rr.Body.Bytes(), &resp); err != nil {
		t.Fatal(err)
	}
	if len(resp.Results) == 0 {
		t.Fatal("empty results")
	}
	withSignal := 0
	for _, r := range resp.Results {
		if r.BuySignal != "" && r.Pattern != "" {
			withSignal++
		}
	}
	if withSignal == 0 {
		t.Fatalf("no result enriched with pattern signal, first=%+v", resp.Results[0])
	}
	t.Logf("archived smoke: %d/%d with pattern signal, sample pattern=%q signal=%q win=%.0f fail=%.0f",
		withSignal, len(resp.Results), resp.Results[0].Pattern, resp.Results[0].BuySignal,
		resp.Results[0].PatternWinPct, resp.Results[0].PatternFailPct)
}
