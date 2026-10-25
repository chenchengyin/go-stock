package flutter_api

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"go-stock/backend/data"
)

func TestMainboardKlineRouteReturnsSelectedDate(t *testing.T) {
	previousRoot, previousFetcher := t0CacheRootPath, fetchRedKlineSeries
	t0CacheRootPath = t.TempDir()
	fetchRedKlineSeries = func(code string, limit int, end string) *data.KLineSourceResult {
		rows := []data.KLineData{
			{Day: "2026-09-23", Open: "10", High: "12", Low: "9", Close: "11"},
			{Day: "2026-09-24", Open: "11", High: "13", Low: "10", Close: "12"},
			{Day: "2026-09-25", Open: "12", High: "14", Low: "11", Close: "13"},
		}
		return &data.KLineSourceResult{Data: &rows, Source: "fixture"}
	}
	t.Cleanup(func() { t0CacheRootPath, fetchRedKlineSeries = previousRoot, previousFetcher })
	recorder := httptest.NewRecorder()
	newRedKlineHandler().ServeHTTP(recorder, httptest.NewRequest(http.MethodGet,
		"/?module_code=radar.main_strategy&date=2026-09-24&codes=600000.XSHG", nil))
	if recorder.Code != http.StatusOK {
		t.Fatalf("HTTP %d: %s", recorder.Code, recorder.Body.String())
	}
	var response struct {
		Items map[string]redKlineWindow `json:"items"`
	}
	if err := json.Unmarshal(recorder.Body.Bytes(), &response); err != nil {
		t.Fatal(err)
	}
	window := response.Items["600000.XSHG"]
	if window.AnchorIndex != 1 || len(window.Bars) != 3 || window.Bars[1].Date != "2026-09-24" || window.Bars[1].Close != 12 {
		t.Fatalf("wrong mainboard window: %+v", window)
	}
}

func TestMainboardIntradayRouteUsesSelectedSessionAndBaseline(t *testing.T) {
	previousRoot, previousFetcher, previousShared := t0CacheRootPath, redT1IntradayFetcher, mainboardSharedGob
	t0CacheRootPath = t.TempDir()
	mainboardSharedGob = &mainboardSharedGobData{Dates: []string{"2026-09-24", "2026-09-25", "2026-09-28"}, Daily: map[string][]dailyBar{
		"600000": {{Date: "2026-09-24", Close: 10}, {Date: "2026-09-25", Close: 10.2}, {Date: "2026-09-28", Close: 10.3}},
	}}
	redT1IntradayFetcher = func(code string, date string, limit int) *data.KLineSourceResult {
		rows := []data.KLineData{
			{Day: "2026-09-25 09:30", Close: "10.1"},
			{Day: "2026-09-25 15:00", Close: "10.2"},
		}
		return &data.KLineSourceResult{Data: &rows, Source: "fixture"}
	}
	t.Cleanup(func() {
		t0CacheRootPath, redT1IntradayFetcher, mainboardSharedGob = previousRoot, previousFetcher, previousShared
	})
	recorder := httptest.NewRecorder()
	newRedT1IntradayHandler().ServeHTTP(recorder, httptest.NewRequest(http.MethodGet,
		"/?module_code=radar.main_strategy&date=2026-09-24&codes=600000.XSHG&session_date=2026-09-25", nil))
	if recorder.Code != http.StatusOK {
		t.Fatalf("HTTP %d: %s", recorder.Code, recorder.Body.String())
	}
	var response struct {
		Date         string                         `json:"date"`
		PreviousDate string                         `json:"previous_date"`
		Items        map[string]redT1IntradaySeries `json:"items"`
	}
	if err := json.Unmarshal(recorder.Body.Bytes(), &response); err != nil {
		t.Fatal(err)
	}
	series := response.Items["600000.XSHG"]
	if response.Date != "2026-09-25" || response.PreviousDate != "2026-09-24" || series.PrevClose != 10 || len(series.Bars) != 2 {
		t.Fatalf("wrong mainboard intraday: %+v %+v", response, series)
	}
}

func TestStrategyChartRoutesRejectUnknownModulesAndFutureSessions(t *testing.T) {
	for _, handler := range []http.HandlerFunc{newRedKlineHandler(), newRedT1IntradayHandler()} {
		recorder := httptest.NewRecorder()
		handler.ServeHTTP(recorder, httptest.NewRequest(http.MethodGet,
			"/?module_code=radar.unknown&date=2026-09-24&codes=600000.XSHG", nil))
		if recorder.Code != http.StatusBadRequest || !strings.Contains(recorder.Body.String(), "模块不存在") {
			t.Fatalf("unknown module accepted: %d %s", recorder.Code, recorder.Body.String())
		}
	}
	recorder := httptest.NewRecorder()
	newRedT1IntradayHandler().ServeHTTP(recorder, httptest.NewRequest(http.MethodGet,
		"/?module_code=radar.main_strategy&date=2026-09-24&codes=600000.XSHG&session_date=2999-01-01", nil))
	if recorder.Code != http.StatusBadRequest || !strings.Contains(recorder.Body.String(), "session_date") {
		t.Fatalf("mainboard must validate future session: %d %s", recorder.Code, recorder.Body.String())
	}
}
