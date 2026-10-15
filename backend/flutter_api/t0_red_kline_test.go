package flutter_api

import (
	"context"
	"encoding/json"
	"fmt"
	"go-stock/backend/data"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"reflect"
	"testing"
)

func TestBuildRedKlineWindowSlicesAroundSelectedTradingDate(t *testing.T) {
	bars := make([]dailyBar, 0, 72)
	for i := 1; i <= 72; i++ {
		bars = append(bars, dailyBar{Date: redKlineTestDate(i), Open: float64(i), Close: float64(i), High: float64(i), Low: float64(i)})
	}
	got := buildRedKlineWindow("600000.XSHG", redKlineTestDate(65), bars, "cache")
	if got.AnchorIndex != 60 || got.BeforeCount != 60 || got.AfterCount != 5 || len(got.Bars) != 66 {
		t.Fatalf("window wrong: %+v", got)
	}
	if got.Bars[0].Date != redKlineTestDate(5) || got.Bars[65].Date != redKlineTestDate(70) {
		t.Fatalf("bounds wrong: %s..%s", got.Bars[0].Date, got.Bars[65].Date)
	}
}

func TestRedKlineMovingAveragesUsePreWindowClosesWithoutExtraVisibleCandles(t *testing.T) {
	bars := make([]dailyBar, 135)
	for i := range bars {
		price := float64(i + 1)
		bars[i] = dailyBar{Date: fmt.Sprintf("2026-%02d-%02d", i/28+1, i%28+1), Open: price, High: price, Low: price, Close: price}
	}
	got := buildRedKlineWindow("600000.XSHG", bars[129].Date, bars, "cache")
	if len(got.Bars) != 66 || got.AnchorIndex != 60 {
		t.Fatalf("visible window changed: %+v", got)
	}
	if got.Bars[0].MA["60"] != 40.5 || got.Bars[0].MA["5"] != 68 || got.Bars[60].MA["60"] != 100.5 {
		t.Fatalf("moving averages incorrect: first=%v anchor=%v", got.Bars[0].MA, got.Bars[60].MA)
	}
	short := buildRedKlineWindow("600000.XSHG", bars[7].Date, bars[:9], "cache")
	if _, exists := short.Bars[0].MA["5"]; exists {
		t.Fatal("insufficient samples must not invent MA5")
	}
	if _, exists := short.Bars[7].MA["10"]; exists {
		t.Fatal("insufficient samples must not invent MA10")
	}
}

func TestBuildRedKlineWindowKeepsGapsAndDoesNotInventAnchor(t *testing.T) {
	bars := []dailyBar{
		{Date: "2026-09-24", Open: 10, High: 12, Low: 9, Close: 11},
		{Date: "2026-09-28", Open: 11, High: 13, Low: 10, Close: 12},
		{Date: "2026-10-08", Open: 12, High: 14, Low: 11, Close: 13},
	}
	got := buildRedKlineWindow("600000.XSHG", "2026-09-25", bars, "cache")
	if got.AnchorIndex != -1 || got.BeforeCount != 1 || got.AfterCount != 0 || len(got.Bars) != 1 {
		t.Fatalf("missing anchor must not borrow later bar: %+v", got)
	}
	got = buildRedKlineWindow("600000.XSHG", "2026-09-24", bars, "cache")
	if got.AfterCount != 2 || !reflect.DeepEqual([]string{got.Bars[1].Date, got.Bars[2].Date}, []string{"2026-09-28", "2026-10-08"}) {
		t.Fatalf("trading gaps should stay gaps: %+v", got)
	}
}

func TestSelectRedKlineWindowDoesNotMixConflictingSources(t *testing.T) {
	local := []dailyBar{{Date: "2026-09-23", Open: 10, High: 12, Low: 9, Close: 11}, {Date: "2026-09-24", Open: 11, High: 13, Low: 10, Close: 12}}
	remote := []dailyBar{{Date: "2026-09-23", Open: 20, High: 22, Low: 19, Close: 21}, {Date: "2026-09-24", Open: 21, High: 23, Low: 20, Close: 22}, {Date: "2026-09-25", Open: 22, High: 24, Low: 21, Close: 23}}
	got := selectRedKlineWindow("600000.XSHG", "2026-09-24", local, remote, "tencent")
	if got.Source != "tencent" || got.Bars[0].Open != 20 || got.Bars[got.AnchorIndex].Close != 22 {
		t.Fatalf("mixed quote scales: %+v", got)
	}
}

func TestRedKlineWindowsUsesFullRemoteSeriesWithoutBlendingCashSnapshots(t *testing.T) {
	local := map[string][]dailyBar{"600000": {{Date: "2026-09-23", Open: 10, High: 12, Low: 9, Close: 11}, {Date: "2026-09-24", Open: 11, High: 13, Low: 10, Close: 12}}}
	calls := 0
	fetch := func(code string, limit int, end string) *data.KLineSourceResult {
		calls++
		rows := []data.KLineData{
			{Day: "2026-09-23", Open: "20", High: "22", Low: "19", Close: "21"},
			{Day: "2026-09-24", Open: "21", High: "23", Low: "20", Close: "22"},
			{Day: "2026-09-25", Open: "22", High: "24", Low: "21", Close: "23"},
		}
		return &data.KLineSourceResult{Source: "tencent", Data: &rows}
	}
	got := redKlineWindows("2026-09-24", []string{"600000.XSHG"}, local, fetch)["600000.XSHG"]
	if calls != 1 || got.Source != "tencent" || got.AfterCount != 1 || got.Bars[got.AnchorIndex].Close != 22 {
		t.Fatalf("window=%+v fetches=%d", got, calls)
	}
}

func TestRedKlineWindowsFetchesSingleSeriesToCompleteMA60Warmup(t *testing.T) {
	all := make([]dailyBar, 126)
	for i := range all {
		price := float64(i + 10)
		all[i] = dailyBar{Date: redKlineTestDate(i + 1), Open: price, High: price, Low: price, Close: price}
	}
	anchor := all[119].Date
	local := map[string][]dailyBar{"600000": all[59:]}
	fetches := 0
	fetch := func(_ string, _ int, _ string) *data.KLineSourceResult {
		fetches++
		rows := make([]data.KLineData, 0, len(all))
		for _, b := range all {
			value := fmt.Sprintf("%.0f", b.Close)
			rows = append(rows, data.KLineData{Day: b.Date, Open: value, High: value, Low: value, Close: value})
		}
		return &data.KLineSourceResult{Data: &rows, Source: "tencent"}
	}
	got := redKlineWindows(anchor, []string{"600000.XSHG"}, local, fetch)["600000.XSHG"]
	if fetches != 1 || len(got.Bars) != 66 || got.AnchorIndex != 60 || got.Bars[0].MA["60"] == 0 || got.Source != "tencent" {
		t.Fatalf("MA60 warmup not completed: fetches=%d window=%+v", fetches, got)
	}
}

func TestRedKlineWindowsRequestsHistoricalWarmupWhenLatestWindowMissesArchive(t *testing.T) {
	all := make([]dailyBar, 126)
	for i := range all {
		price := float64(i + 10)
		all[i] = dailyBar{Date: redKlineTestDate(i + 1), Open: price, High: price, Low: price, Close: price}
	}
	anchor := all[119].Date
	local := map[string][]dailyBar{"600000": all[59:]}
	calls := 0
	fetch := func(_ string, limit int, end string) *data.KLineSourceResult {
		calls++
		if end == "" {
			return &data.KLineSourceResult{Data: &[]data.KLineData{}, Source: "recent"}
		}
		if limit < 125 {
			t.Fatalf("historical request lost MA60 warmup: limit=%d", limit)
		}
		rows := make([]data.KLineData, 0, len(all))
		for _, b := range all {
			value := fmt.Sprintf("%.0f", b.Close)
			rows = append(rows, data.KLineData{Day: b.Date, Open: value, High: value, Low: value, Close: value})
		}
		return &data.KLineSourceResult{Data: &rows, Source: "historical"}
	}
	got := redKlineWindows(anchor, []string{"600000.XSHG"}, local, fetch)["600000.XSHG"]
	if calls != 2 || len(got.Bars) != 66 || got.Bars[0].MA["60"] == 0 || got.Source != "historical" {
		t.Fatalf("historical MA60 not recovered: calls=%d firstMA=%v source=%s", calls, got.Bars[0].MA, got.Source)
	}
}

func TestRedKlineRouteDoesNotRequireLoginOrModuleAccess(t *testing.T) {
	auth := newTestAuthService(t)
	handler := newHTTPHandler(auth.AuthService)
	invalid := httptest.NewRequest(http.MethodGet,
		"/api/t0-red/kline-window?date=2026-09-24&codes=not-a-stock", nil)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, invalid)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("unauthenticated request did not reach code validation: %d %s", rec.Code, rec.Body.String())
	}
	var decoded map[string]string
	if err := json.Unmarshal(rec.Body.Bytes(), &decoded); err != nil || decoded["error"] == "" {
		t.Fatalf("invalid response: %v %v", decoded, err)
	}
}

func TestMarketQuoteRoutesDoNotRequireLogin(t *testing.T) {
	auth := newTestAuthService(t)
	handler := newHTTPHandler(auth.AuthService)
	for _, path := range []string{
		"/api/kline",
		"/api/stock-realtime",
	} {
		t.Run(path, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodGet, path, nil)
			rec := httptest.NewRecorder()
			handler.ServeHTTP(rec, req)
			if rec.Code == http.StatusUnauthorized {
				t.Fatalf("market quote route requires login: %d %s", rec.Code, rec.Body.String())
			}
		})
	}
}

func TestParseRedKlineTencentPayloadSkipsNonCandleRows(t *testing.T) {
	payload := []byte(`{"code":0,"data":{"sh600000":{"qfqday":[["2026-09-23","10","11","12","9"],{"unexpected":"metadata"},["2026-09-24","11","12","13","10"]]},"sh000001":{"qfqday":{"error":"upstream anomaly"}}}}`)
	bars := parseRedKlineTencentPayload(payload, "sh600000")
	if len(bars) != 2 || bars[1].Day != "2026-09-24" || bars[1].High != "13" {
		t.Fatalf("parser failed: %+v", bars)
	}
	ipo := []byte(`{"code":0,"data":{"sh601091":{"day":[["2026-09-17","13","20.8","20.8","11.73"]]}}}`)
	bars = parseRedKlineTencentPayload(ipo, "sh601091")
	if len(bars) != 1 || bars[0].Day != "2026-09-17" {
		t.Fatalf("IPO day fallback failed: %+v", bars)
	}
}

func TestPreferFollowupCacheOnlyAsWholeSeries(t *testing.T) {
	original := []dailyBar{{Date: "2026-09-23", Open: 10, High: 12, Low: 9, Close: 11}, {Date: "2026-09-24", Open: 11, High: 13, Low: 10, Close: 12}}
	later := []dailyBar{{Date: "2026-09-23", Open: 20, High: 22, Low: 19, Close: 21}, {Date: "2026-09-24", Open: 21, High: 23, Low: 20, Close: 22}, {Date: "2026-09-28", Open: 22, High: 24, Low: 21, Close: 23}}
	chosen := preferRedKlineFollowup("2026-09-24", original, later)
	if len(chosen) != 3 || chosen[0].Open != 20 || chosen[2].Close != 23 {
		t.Fatalf("later snapshot should replace whole series: %+v", chosen)
	}
	noAnchor := []dailyBar{{Date: "2026-09-28", Open: 22, High: 24, Low: 21, Close: 23}}
	chosen = preferRedKlineFollowup("2026-09-24", original, noAnchor)
	if len(chosen) != 2 || chosen[0].Open != 10 {
		t.Fatalf("cannot use snapshot without selected day: %+v", chosen)
	}
}

func TestRedKlineRouteReturnsScopedWindow(t *testing.T) {
	auth := newTestAuthService(t)
	user := authHTTPRegisterUser(t, auth, "13877775555", "Chart User", "device-chart-2")
	modules := NewModuleService(auth.dao)
	if err := modules.ReplaceUserAccess(context.Background(), "admin-chart", []string{user.User.ID}, []string{sharedGobRedStrategyModuleCode}); err != nil {
		t.Fatal(err)
	}
	oldRoot := t0CacheRootPath
	oldFetcher := fetchRedKlineSeries
	oldShared := mainboardSharedGob
	mainboardSharedGob = &mainboardSharedGobData{Daily: map[string][]dailyBar{}}
	t0CacheRootPath = t.TempDir()
	fetchRedKlineSeries = func(code string, limit int, end string) *data.KLineSourceResult {
		rows := []data.KLineData{
			{Day: "2026-09-23", Open: "10", High: "12", Low: "9", Close: "11"},
			{Day: "2026-09-24", Open: "11", High: "13", Low: "10", Close: "12"},
			{Day: "2026-09-28", Open: "12", High: "14", Low: "11", Close: "13"},
		}
		return &data.KLineSourceResult{Data: &rows, Source: "stub"}
	}
	defer func() { t0CacheRootPath = oldRoot; fetchRedKlineSeries = oldFetcher; mainboardSharedGob = oldShared }()
	req := httptest.NewRequest(http.MethodGet,
		"/api/t0-red/kline-window?date=2026-09-24&codes=600000.XSHG&module_code=radar.red_shared_gob_strategy", nil)
	req.Header.Set("Authorization", "Bearer "+user.AccessToken)
	rec := httptest.NewRecorder()
	newHTTPHandler(auth.AuthService).ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
	}
	var body struct {
		Date  string                    `json:"date"`
		Items map[string]redKlineWindow `json:"items"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatal(err)
	}
	item := body.Items["600000.XSHG"]
	if body.Date != "2026-09-24" || item.Source != "stub" || item.AnchorIndex != 1 || item.AfterCount != 1 || len(item.Bars) != 3 || item.Bars[2].Date != "2026-09-28" {
		t.Fatalf("invalid API payload: %+v", body)
	}
}

func TestRedKlineLiveProbe(t *testing.T) {
	if os.Getenv("GO_STOCK_RED_KLINE_LIVE_PROBE") != "1" {
		t.Skip("run on demand against local Gob and market data")
	}
	date := os.Getenv("GO_STOCK_RED_KLINE_PROBE_DATE")
	if date == "" {
		date = "2026-09-16"
	}
	code := os.Getenv("GO_STOCK_RED_KLINE_PROBE_CODE")
	if code == "" {
		code = "000823.XSHE"
	}
	cacheRoot, err := filepath.Abs(filepath.Join("..", "data", "cache"))
	if err != nil {
		t.Fatal(err)
	}
	previousRoot := t0CacheRootPath
	t0CacheRootPath = cacheRoot
	defer func() { t0CacheRootPath = previousRoot }()
	t.Logf("dedicated tencent bars=%d", len(fetchRedKlineTencent(t0ShortCodeFromResultCode(code), redKlineFetchLimit)))
	daily := loadRedKlineDaily(date)
	followup := loadRedKlineFollowupDaily(date)
	shortCode := t0ShortCodeFromResultCode(code)
	if len(followup[shortCode]) > 0 {
		daily[shortCode] = preferRedKlineFollowup(date, daily[shortCode], followup[shortCode])
	}
	got := redKlineWindows(date, []string{code}, daily, fetchRedKlineSeries)[code]
	wantBefore := 60
	if code == "601091.XSHG" && date == "2026-09-30" {
		wantBefore = 8
	} // listed on 2026-09-17
	if got.BeforeCount != wantBefore || got.AnchorIndex != wantBefore || len(got.Bars) < wantBefore+1 || got.Bars[got.AnchorIndex].Date != date {
		t.Fatalf("real red K-line window failed: source=%s status=%s before=%d anchor=%d after=%d range=%v", got.Source, got.Status, got.BeforeCount, got.AnchorIndex, got.AfterCount, got.Bars)
	}
	t.Logf("real red K-line %d bars source=%s range=%s..%s after=%d", len(got.Bars), got.Source, got.Bars[0].Date, got.Bars[len(got.Bars)-1].Date, got.AfterCount)
}

func redKlineTestDate(i int) string {
	return fmt.Sprintf("2026-%02d-%02d", (i-1)/28+1, (i-1)%28+1)
}
