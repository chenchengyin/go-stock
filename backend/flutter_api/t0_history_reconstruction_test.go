package flutter_api

import (
	"encoding/json"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
	"time"

	"go-stock/backend/data"
)

func reconstructedHistoryFixture(t *testing.T) {
	t.Helper()
	isolateSharedGobConfirmation(t)
	if err := writeMainboardFloatCap50Gob(mainboardSharedGobPath(), MainboardFloatCap50Gob{SchemaVersion: 2, Stocks: []MainboardFloatCap50Stock{{Code: "600000", FloatMarketCapYi: 100}}, Daily: map[string][]dailyBar{"600000": {{Date: "2026-10-19", Open: 10, High: 10, Low: 10, Close: 10, AmountYi: 6}, {Date: "2026-10-20", Open: 10.1, High: 10.2, Low: 10, Close: 10.1, AmountYi: 6}}}}); err != nil {
		t.Fatal(err)
	}
	dir := filepath.Join(t0CacheRootPath, "t0", "reconstructed_pools")
	os.MkdirAll(dir, 0755)
	payload := map[string]interface{}{"schema_version": 1, "date": "2026-10-20", "reference_date": "2026-10-19", "source": "baostock_dated_history", "method": "prior_close_float_cap_from_volume_turnover", "evidence_complete": true, "universe_count": 3195, "fetched_code_count": 3195, "stocks": []t0Stock{{Code: "sh.600000", ShortCode: "600000", Name: "历史重建", MarketCapYi: 100}}}
	b, _ := json.Marshal(payload)
	if err := os.WriteFile(filepath.Join(dir, "pool_2026-10-20.json"), b, 0600); err != nil {
		t.Fatal(err)
	}
}

func TestHistoricalReconstructionServedWithExplicitUncertainty(t *testing.T) {
	reconstructedHistoryFixture(t)
	r := httptest.NewRecorder()
	handleSharedGobRedSelectionUnconfirmed(r, "2026-10-20")
	var body map[string]interface{}
	json.Unmarshal(r.Body.Bytes(), &body)
	if r.Code != 200 || body["pool_reconstructed"] != true || body["date_pool_exact"] != false || body["confirmed"] != false || body["data_incomplete"] != true {
		t.Fatalf("reconstruction not served honestly: %d %s", r.Code, r.Body.String())
	}
	if _, err := os.Stat(t0PoolArchivePath("2026-10-20")); !os.IsNotExist(err) {
		t.Fatal("reconstruction promoted into original pool archive")
	}
	shared, err := loadMainboardSharedGob()
	if err != nil {
		t.Fatal(err)
	}
	dates := sharedGobRedDatesDescendingAt(shared, time.Date(2026, 10, 25, 12, 0, 0, 0, chinaLocation()))
	if len(dates) != 1 || dates[0] != "2026-10-20" {
		t.Fatalf("reconstructed date absent: %v", dates)
	}
}

func TestHistoricalReconstructionOriginalPoolTakesPrecedence(t *testing.T) {
	reconstructedHistoryFixture(t)
	if err := saveT0PoolArchive("2026-10-20", []t0Stock{{Code: "sh.600000", ShortCode: "600000", Name: "原始池", MarketCapYi: 110}}, "original_capture"); err != nil {
		t.Fatal(err)
	}
	r := httptest.NewRecorder()
	handleSharedGobRedSelectionUnconfirmed(r, "2026-10-20")
	var body map[string]interface{}
	json.Unmarshal(r.Body.Bytes(), &body)
	if r.Code != 200 || body["pool_reconstructed"] == true {
		t.Fatalf("original pool replaced: %d %s", r.Code, r.Body.String())
	}
}

func TestHistoricalReconstructionRejectsPartialAndFutureReference(t *testing.T) {
	for _, mutation := range []map[string]interface{}{{"evidence_complete": false}, {"fetched_code_count": 3194}, {"reference_date": "2026-10-21"}, {"daily": map[string][]dailyBar{"600000": {{Date: "2026-10-19", Open: -1, Close: 10}}}}, {"stocks": []t0Stock{{ShortCode: "600000", MarketCapYi: 100}, {ShortCode: "600000", MarketCapYi: 100}}}} {
		t.Run("invalid", func(t *testing.T) {
			reconstructedHistoryFixture(t)
			p := filepath.Join(t0CacheRootPath, "t0", "reconstructed_pools", "pool_2026-10-20.json")
			b, _ := os.ReadFile(p)
			var body map[string]interface{}
			json.Unmarshal(b, &body)
			for k, v := range mutation {
				body[k] = v
			}
			b, _ = json.Marshal(body)
			os.WriteFile(p, b, 0600)
			r := httptest.NewRecorder()
			handleSharedGobRedSelectionUnconfirmed(r, "2026-10-20")
			if r.Code != 503 {
				t.Fatalf("invalid reconstruction accepted: %d %s", r.Code, r.Body.String())
			}
		})
	}
}

func TestHistoricalReconstructionAuditDoesNotPromoteToOriginal(t *testing.T) {
	reconstructedHistoryFixture(t)
	report, err := AuditT0History(t0CacheRootPath, false)
	if err != nil {
		t.Fatal(err)
	}
	b, _ := json.Marshal(report)
	var body map[string]interface{}
	json.Unmarshal(b, &body)
	if body["reconstructed_dates"] != float64(1) || report.AvailableDates != 0 || report.MissingPoolDates != 2 || report.Days[1].Status != "reconstructed" {
		t.Fatalf("audit lost reconstruction provenance: %s", b)
	}
}

func TestHistoricalReconstructionChartUsesSupplementWhenOffline(t *testing.T) {
	reconstructedHistoryFixture(t)
	p := filepath.Join(t0CacheRootPath, "t0", "reconstructed_pools", "pool_2026-10-20.json")
	b, _ := os.ReadFile(p)
	var payload map[string]interface{}
	json.Unmarshal(b, &payload)
	bars := []dailyBar{}
	for day := time.Date(2026, 7, 1, 0, 0, 0, 0, chinaLocation()); day.Format("2006-01-02") <= "2026-10-23"; day = day.AddDate(0, 0, 1) {
		if day.Weekday() == time.Saturday || day.Weekday() == time.Sunday {
			continue
		}
		bars = append(bars, dailyBar{Date: day.Format("2006-01-02"), Open: 10, High: 11, Low: 9, Close: 10, Volume: 100, VolumeUnit: "hands", AmountYi: 1, AmountSource: "baostock_daily", AmountUnit: "yuan", AmountVerified: true})
	}
	payload["stocks"] = []t0Stock{{Code: "sh.600001", ShortCode: "600001", MarketCapYi: 100}}
	payload["daily"] = map[string][]dailyBar{"600001": bars}
	b, _ = json.Marshal(payload)
	os.WriteFile(p, b, 0600)
	oldFetcher := fetchRedKlineSeries
	fetchRedKlineSeries = func(string, int, string) *data.KLineSourceResult { return nil }
	t.Cleanup(func() { fetchRedKlineSeries = oldFetcher })
	r := httptest.NewRecorder()
	newRedKlineHandler().ServeHTTP(r, httptest.NewRequest("GET", "/?module_code=radar.red_shared_gob_strategy&date=2026-10-20&codes=600001.XSHG", nil))
	var body struct {
		Items map[string]redKlineWindow `json:"items"`
	}
	json.Unmarshal(r.Body.Bytes(), &body)
	window := body.Items["600001.XSHG"]
	if r.Code != 200 || window.AnchorIndex < 0 || window.Source != "cache" || window.Status != "ok" || window.BeforeCount != 60 || window.AfterCount != 3 {
		t.Fatalf("offline chart lost already fetched history: %d %+v", r.Code, window)
	}
}
