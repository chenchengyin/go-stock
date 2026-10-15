package data

import (
	"os"
	"testing"
	"time"
)

func TestParseTHSRealtimeDailyQuoteRequiresCurrentDateAndPreviousClose(t *testing.T) {
	rows := []KLineData{
		{Day: "2026-10-09", Close: "9.80"},
		{Day: "2026-10-10", Open: "10.00", Close: "10.20"},
	}
	quote, ok := parseTHSRealtimeDailyQuote(rows, "2026-10-10")
	if !ok {
		t.Fatal("expected a usable same-day THS quote")
	}
	if quote.Open != 10 || quote.Price != 10.2 || quote.PrevClose != 9.8 || quote.PrevDate != "2026-10-09" {
		t.Fatalf("unexpected quote: %+v", quote)
	}
	if _, ok := parseTHSRealtimeDailyQuote(rows[:1], "2026-10-10"); ok {
		t.Fatal("stale daily bar must not be accepted as today's quote")
	}
}

func TestTHSRealtimeDailyQuoteLiveProbe(t *testing.T) {
	if os.Getenv("THS_KLINE_LIVE_PROBE") != "1" {
		t.Skip("set THS_KLINE_LIVE_PROBE=1 to query THS")
	}
	loc := time.FixedZone("Asia/Shanghai", 8*60*60)
	asOf := time.Date(2026, 10, 9, 16, 0, 0, 0, loc)
	quote, ok := FetchTHSRealtimeDailyQuote("000409.SZ", asOf)
	if !ok {
		t.Fatal("THS did not return a same-day daily quote with previous close")
	}
	if quote.Date != "2026-10-09" || quote.Open <= 0 || quote.Price <= 0 || quote.PrevClose <= 0 {
		t.Fatalf("unexpected THS quote: %+v", quote)
	}
	t.Logf("as_of=%s quote=%+v", asOf.Format(time.RFC3339), quote)
}
