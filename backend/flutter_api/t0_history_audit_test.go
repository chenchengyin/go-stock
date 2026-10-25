package flutter_api

import (
	"encoding/json"
	"net/http/httptest"
	"os"
	"reflect"
	"testing"
	"time"
)

func TestSharedGobDateListExcludesUnreplayableHistoricalSessions(t *testing.T) {
	isolateSharedGobConfirmation(t)
	if _, err := saveSharedGobRedConfirmation(confirmationFixture(t, "2026-10-21", "正式股")); err != nil {
		t.Fatal(err)
	}
	shared := &mainboardSharedGobData{Dates: []string{"2026-10-19", "2026-10-20", "2026-10-21", "2026-10-23"}, SelectionPools: map[string][]t0Stock{"2026-10-19": {{ShortCode: "600000"}}}}
	got := sharedGobRedDatesDescendingAt(shared, time.Date(2026, 10, 25, 12, 0, 0, 0, chinaLocation()))
	if want := []string{"2026-10-21", "2026-10-19"}; !reflect.DeepEqual(got, want) {
		t.Fatalf("offered dates with bars but no replayable pool: %v, want %v", got, want)
	}
}

func TestPoolMissingResponseHasActionableCode(t *testing.T) {
	isolateSharedGobConfirmation(t)
	date := "2026-10-20"
	if err := writeMainboardFloatCap50Gob(mainboardSharedGobPath(), MainboardFloatCap50Gob{SchemaVersion: 2, Stocks: []MainboardFloatCap50Stock{{Code: "600000", FloatMarketCapYi: 100}}, Daily: map[string][]dailyBar{"600000": {{Date: "2026-10-19", Close: 10}, {Date: date, Open: 11, Close: 11}}}}); err != nil {
		t.Fatal(err)
	}
	r := httptest.NewRecorder()
	handleSharedGobRedSelectionAt(r, date, time.Date(2026, 10, 25, 12, 0, 0, 0, chinaLocation()))
	var got map[string]interface{}
	json.Unmarshal(r.Body.Bytes(), &got)
	if r.Code != 503 || got["code"] != "T0_HISTORY_POOL_MISSING" || got["date"] != date || got["data_incomplete"] != true {
		t.Fatalf("missing pool not distinguished from outage: %d %s", r.Code, r.Body.String())
	}
}

func TestHistoryAuditRecoversOnlyWitnessedPools(t *testing.T) {
	isolateSharedGobConfirmation(t)
	snapshot := MainboardFloatCap50Gob{SchemaVersion: 2, GeneratedAt: "2026-10-21T09:25:00+08:00", Stocks: []MainboardFloatCap50Stock{{Code: "600000", FloatMarketCapYi: 100}}, Daily: map[string][]dailyBar{"600000": {{Date: "2026-10-19", Close: 10}, {Date: "2026-10-20", Close: 11}, {Date: "2026-10-21", Close: 12}}}, SelectionPools: map[string][]MainboardT0PoolStock{"2026-10-19": {{Code: "600000", MarketCapYi: 100}}}}
	if err := writeMainboardFloatCap50Gob(mainboardSharedGobPath(), snapshot); err != nil {
		t.Fatal(err)
	}
	if err := saveT0DailyCache("2026-10-20", []t0Stock{{Code: "sh.600001", ShortCode: "600001", MarketCapYi: 100}}, nil); err != nil {
		t.Fatal(err)
	}
	report, err := AuditT0History(t0CacheRootPath, true)
	if err != nil {
		t.Fatal(err)
	}
	if report.TradingDates != 3 || report.AvailableDates != 2 || report.MissingPoolDates != 1 {
		t.Fatalf("incorrect coverage: %+v", report)
	}
	if a, err := readT0PoolArchive("2026-10-20"); err != nil || a.Stocks[0].ShortCode != "600001" {
		t.Fatalf("dated pool was not recovered: %+v %v", a, err)
	}
	if _, err := readT0PoolArchive("2026-10-21"); !os.IsNotExist(err) {
		t.Fatalf("invented a pool for an unwitnessed date: %v", err)
	}
}

func TestHistoryAuditReportsConflictingDatedSources(t *testing.T) {
	isolateSharedGobConfirmation(t)
	date := "2026-10-20"
	if err := writeMainboardFloatCap50Gob(mainboardSharedGobPath(), MainboardFloatCap50Gob{SchemaVersion: 2, Stocks: []MainboardFloatCap50Stock{{Code: "600000", FloatMarketCapYi: 100}}, Daily: map[string][]dailyBar{"600000": {{Date: date, Close: 10}}}, SelectionPools: map[string][]MainboardT0PoolStock{date: {{Code: "600000", MarketCapYi: 100}}}}); err != nil {
		t.Fatal(err)
	}
	if err := saveT0DailyCache(date, []t0Stock{{Code: "sh.600001", ShortCode: "600001", MarketCapYi: 100}}, nil); err != nil {
		t.Fatal(err)
	}
	report, err := AuditT0History(t0CacheRootPath, true)
	if err != nil {
		t.Fatal(err)
	}
	if len(report.Errors) == 0 || report.AvailableDates != 0 || report.RecoveredPools != 0 {
		t.Fatalf("conflicting sources reported as repaired: %+v", report)
	}
	if _, err := readT0PoolArchive(date); !os.IsNotExist(err) {
		t.Fatalf("conflict was archived as verified: %v", err)
	}
}

func TestSharedGobDateListIncludesCompleteDatedCacheWithoutManualMigration(t *testing.T) {
	isolateSharedGobConfirmation(t)
	date := "2026-10-20"
	if err := writeMainboardFloatCap50Gob(mainboardSharedGobPath(), MainboardFloatCap50Gob{SchemaVersion: 2, Stocks: []MainboardFloatCap50Stock{{Code: "600000", FloatMarketCapYi: 100}}, Daily: map[string][]dailyBar{"600000": {{Date: date, Close: 10}}}}); err != nil {
		t.Fatal(err)
	}
	if err := saveT0DailyCache(date, []t0Stock{{Code: "sh.600000", ShortCode: "600000", MarketCapYi: 100}}, nil); err != nil {
		t.Fatal(err)
	}
	shared, err := loadMainboardSharedGob()
	if err != nil {
		t.Fatal(err)
	}
	got := sharedGobRedDatesDescendingAt(shared, time.Date(2026, 10, 25, 12, 0, 0, 0, chinaLocation()))
	if !reflect.DeepEqual(got, []string{date}) {
		t.Fatalf("valid dated cache disappeared on upgrade: %v", got)
	}
}
