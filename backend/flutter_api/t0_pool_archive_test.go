package flutter_api

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestPoolArchiveSurvivesSharedGobReplacement(t *testing.T) {
	isolateSharedGobConfirmation(t)
	date := "2026-10-20"
	snapshot := MainboardFloatCap50Gob{SchemaVersion: 2, Stocks: []MainboardFloatCap50Stock{{Code: "600000", FloatMarketCapYi: 100}}, Daily: map[string][]dailyBar{"600000": {{Date: "2026-10-19", Close: 10}, {Date: date, Open: 11, Close: 11}}}, SelectionPools: map[string][]MainboardT0PoolStock{date: {{Code: "600000", Name: "原始池", MarketCapYi: 100}}}}
	if err := writeMainboardFloatCap50Gob(mainboardSharedGobPath(), snapshot); err != nil {
		t.Fatal(err)
	}
	if _, err := loadMainboardSharedGob(); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(t0CacheRootPath, "t0", "pools", "pool_2026-10-20.json")); err != nil {
		t.Fatalf("dated pool not archived separately: %v", err)
	}
	snapshot.SelectionPools = nil
	if err := writeMainboardFloatCap50Gob(mainboardSharedGobPath(), snapshot); err != nil {
		t.Fatal(err)
	}
	mainboardSharedGob = nil
	got, err := loadMainboardSharedGob()
	if err != nil {
		t.Fatal(err)
	}
	if len(got.SelectionPools[date]) != 1 || got.SelectionPools[date][0].Name != "原始池" {
		t.Fatalf("replacement lost historical pool: %+v", got.SelectionPools)
	}
}

func TestPoolArchiveRetainedBeforeDailyFailure(t *testing.T) {
	isolateSharedGobConfirmation(t)
	today := time.Now().In(chinaLocation()).Format("2006-01-02")
	// A today cache is an existing independently complete pool. The shared GOB
	// has none, and the prior-session witness fails before any GOB publication.
	stock := t0Stock{Code: "sh.600001", ShortCode: "600001", Name: "当日池", MarketCapYi: 100}
	if err := saveT0DailyCache(today, []t0Stock{stock}, nil); err != nil {
		t.Fatal(err)
	}
	if err := writeMainboardFloatCap50Gob(mainboardSharedGobPath(), MainboardFloatCap50Gob{SchemaVersion: 2, Stocks: []MainboardFloatCap50Stock{{Code: "600000", FloatMarketCapYi: 100}}, Daily: map[string][]dailyBar{"600000": {{Date: "2026-10-19", Close: 10}}}}); err != nil {
		t.Fatal(err)
	}
	sharedGobPriorDateFetcher = func(string) (string, error) { return "", fmt.Errorf("daily source unavailable") }
	if _, _, _, err := loadOrFetchSharedGobT0Daily(today); err == nil {
		t.Fatal("expected witness failure")
	}
	path := filepath.Join(t0CacheRootPath, "t0", "pools", "pool_"+today+".json")
	if _, err := os.Stat(path); err != nil {
		t.Fatalf("daily failure lost the already obtained pool: %v", err)
	}
}

func TestPoolArchiveRejectsCorruptOrConflictingHistory(t *testing.T) {
	isolateSharedGobConfirmation(t)
	date := "2026-10-20"
	snapshot := MainboardFloatCap50Gob{SchemaVersion: 2, Stocks: []MainboardFloatCap50Stock{{Code: "600000", FloatMarketCapYi: 100}}, Daily: map[string][]dailyBar{"600000": {{Date: date, Close: 10}}}, SelectionPools: map[string][]MainboardT0PoolStock{date: {{Code: "600000", MarketCapYi: 100}}}}
	snapshot.SelectionPools["2026-10-19"] = []MainboardT0PoolStock{{Code: "600000", MarketCapYi: 100}}
	snapshot.Daily["600000"] = append([]dailyBar{{Date: "2026-10-19", Close: 10}}, snapshot.Daily["600000"]...)
	if err := writeMainboardFloatCap50Gob(mainboardSharedGobPath(), snapshot); err != nil {
		t.Fatal(err)
	}
	if _, err := loadMainboardSharedGob(); err != nil {
		t.Fatal(err)
	}
	p := filepath.Join(t0CacheRootPath, "t0", "pools", "pool_"+date+".json")
	if err := os.WriteFile(p, []byte(`{"schema_version":1,"date":"2026-10-19","complete":true,"stocks":[{"ShortCode":"600001"}]}`), 0600); err != nil {
		t.Fatal(err)
	}
	mainboardSharedGob = nil
	got, err := loadMainboardSharedGob()
	if err != nil {
		t.Fatalf("one bad date disabled healthy daily data: %v", err)
	}
	if got.PoolErrors[date] == "" {
		t.Fatal("corrupt date was not isolated and reported")
	}
	if len(got.SelectionPools["2026-10-19"]) != 1 {
		t.Fatal("healthy date was disabled by another date's corrupt archive")
	}
	r := httptest.NewRecorder()
	handleSharedGobRedSelectionAt(r, date, time.Date(2026, 10, 25, 12, 0, 0, 0, chinaLocation()))
	var body map[string]interface{}
	json.Unmarshal(r.Body.Bytes(), &body)
	if r.Code != 503 || body["code"] != "T0_HISTORY_POOL_INVALID" {
		t.Fatalf("isolated pool error was silently bypassed: %d %s", r.Code, r.Body.String())
	}

}

func TestHistoricalPoolMissNeverFetchesTodaysUniverse(t *testing.T) {
	isolateSharedGobConfirmation(t)
	calls := 0
	withSinaTestHTTPClient(t, func(r *http.Request) (*http.Response, error) {
		calls++
		return sinaHTTPResponse(200, []byte("[]")), nil
	})
	_, _, _, err := loadOrFetchT0Daily("2026-10-20")
	if err == nil || calls != 0 {
		t.Fatalf("historical miss queried live universe: calls=%d err=%v", calls, err)
	}
}

func TestPoolCaptureAfterAuctionRetriesWithoutDailyDownload(t *testing.T) {
	isolateSharedGobConfirmation(t)
	oldFetch, oldAttempt := t0PoolFetcher, t0PoolCaptureLastAttempt
	t0PoolCaptureLastAttempt = time.Time{}
	t.Cleanup(func() { t0PoolFetcher = oldFetch; t0PoolCaptureLastAttempt = oldAttempt })
	now := time.Date(2026, 10, 20, 10, 0, 0, 0, chinaLocation())
	calls := 0
	t0PoolFetcher = func() ([]t0Stock, error) {
		calls++
		if calls == 1 {
			return nil, fmt.Errorf("temporary network failure")
		}
		return []t0Stock{{Code: "sh.600000", ShortCode: "600000", MarketCapYi: 100}}, nil
	}
	runT0PoolArchiveTick(now)
	runT0PoolArchiveTick(now.Add(time.Minute))
	if calls != 1 {
		t.Fatalf("did not back off: %d", calls)
	}
	runT0PoolArchiveTick(now.Add(5 * time.Minute))
	runT0PoolArchiveTick(now.Add(6 * time.Minute))
	if calls != 2 {
		t.Fatalf("completed pool captured repeatedly: %d", calls)
	}
	if _, err := readT0PoolArchive("2026-10-20"); err != nil {
		t.Fatalf("late startup missed pool: %v", err)
	}
	runT0PoolArchiveTick(time.Date(2026, 10, 21, 15, 5, 0, 0, chinaLocation()))
	runT0PoolArchiveTick(time.Date(2026, 10, 25, 10, 0, 0, 0, chinaLocation()))
	if calls != 2 {
		t.Fatalf("captured after market window or on weekend: %d", calls)
	}
}
