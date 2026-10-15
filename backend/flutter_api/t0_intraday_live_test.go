package flutter_api

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// Opt-in tests exercise the real HTTP route/providers with read-only daily facts,
// while keeping all generated minute caches in a disposable directory.
func TestRedIntradayHTTPHistoricalLive(t *testing.T) {
	root := os.Getenv("GO_STOCK_INTRADAY_LIVE_CACHE_ROOT")
	if root == "" {
		t.Skip("set GO_STOCK_INTRADAY_LIVE_CACHE_ROOT to enable real provider validation")
	}
	oldRoot, oldShared := t0CacheRootPath, mainboardSharedGob
	t0CacheRootPath = root
	mainboardSharedGob = nil
	shared, err := loadMainboardSharedGob()
	if err != nil {
		t.Fatal(err)
	}
	t0CacheRootPath = t.TempDir()
	mainboardSharedGob = shared
	defer func() { t0CacheRootPath = oldRoot; mainboardSharedGob = oldShared }()
	server := httptest.NewServer(newRedT1IntradayHandler())
	defer server.Close()
	client := &http.Client{Timeout: 40 * time.Second}
	stocks := []string{"600498.XSHG", "600000.XSHG", "600519.XSHG", "601318.XSHG", "603083.XSHG", "600617.XSHG", "000823.XSHE", "000001.XSHE", "000636.XSHE", "002552.XSHE", "002290.XSHE", "003043.XSHE"}
	dates := []string{"2024-01-03", "2024-06-03", "2024-12-03", "2025-01-07", "2025-06-03", "2025-09-17", "2026-01-06", "2026-07-23", "2026-09-17", "2026-10-14"}
	type response struct {
		Date     string                         `json:"date"`
		Previous string                         `json:"previous_date"`
		Next     string                         `json:"next_date"`
		Items    map[string]redT1IntradaySeries `json:"items"`
	}
	get := func(t0, session, code string) response {
		t.Helper()
		q := url.Values{"date": {t0}, "codes": {code}, "module_code": {sharedGobRedStrategyModuleCode}}
		if session != "" {
			q.Set("session_date", session)
		}
		r, e := client.Get(server.URL + "?" + q.Encode())
		if e != nil {
			t.Fatal(e)
		}
		defer r.Body.Close()
		var out response
		if r.StatusCode != 200 {
			t.Fatalf("HTTP %d: %s", r.StatusCode, q.Encode())
		}
		if e = json.NewDecoder(r.Body).Decode(&out); e != nil {
			t.Fatal(e)
		}
		return out
	}
	reports := []map[string]any{}
	for _, date := range dates {
		previous, ok := resolvePrevTradingDay(date)
		if !ok {
			t.Fatalf("no previous session for %s", date)
		}
		for _, code := range stocks {
			start := time.Now()
			out := get(previous, "", code)
			x := out.Items[code]
			if out.Date != date || out.Previous != previous || x.Date != date || x.Status != "ok" || x.PrevClose <= 0 || len(x.Bars) != 241 || x.Bars[0].Time != "09:30" || x.Bars[len(x.Bars)-1].Time != "15:00" {
				t.Fatalf("%s %s: date=%s previous=%s series=%+v", code, date, out.Date, out.Previous, x)
			}
			short := strings.Split(code, ".")[0]
			bar, _ := dailyBarOnDate(shared.Daily[short], date)
			if bar.Close > 0 && (x.Bars[len(x.Bars)-1].Price > bar.Close+.011 || x.Bars[len(x.Bars)-1].Price < bar.Close-.011) {
				t.Fatalf("%s %s minute/daily close mismatch", code, date)
			}
			reports = append(reports, map[string]any{"code": code, "date": date, "previous_date": out.Previous, "status": x.Status, "source": x.Source, "points": len(x.Bars), "prevClose": x.PrevClose, "elapsed_ms": time.Since(start).Milliseconds()})
		}
		t.Logf("passed %s: %d stocks", date, len(stocks))
	}
	for _, session := range []string{"", "2026-10-08"} {
		out := get("2026-09-30", session, "000823.XSHE")
		if out.Date != "2026-10-08" || out.Previous != "2026-09-30" || out.Next != "2026-10-09" || out.Items["000823.XSHE"].Status != "ok" {
			t.Fatalf("holiday navigation failed: %+v", out)
		}
	}
	for _, day := range []string{"2026-10-01", "2026-10-11"} {
		q := url.Values{"date": {"2026-09-30"}, "codes": {"000823.XSHE"}, "session_date": {day}}
		r, err := client.Get(server.URL + "?" + q.Encode())
		if err != nil {
			t.Fatal(err)
		}
		r.Body.Close()
		if r.StatusCode != 400 {
			t.Fatalf("closed day accepted: %s HTTP %d", day, r.StatusCode)
		}
	}
	if path := os.Getenv("GO_STOCK_INTRADAY_LIVE_REPORT"); path != "" {
		raw, _ := json.MarshalIndent(reports, "", "  ")
		if err := os.WriteFile(filepath.Clean(path), raw, 0644); err != nil {
			t.Fatal(err)
		}
	}
	t.Logf("passed %d stock/date cases plus holiday navigation and closed-day rejection", len(reports))
}
