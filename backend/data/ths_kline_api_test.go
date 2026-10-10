package data

import (
	"encoding/json"
	"os"
	"strconv"
	"testing"
	"time"
)

func TestTHSCodeAndMarket(t *testing.T) {
	tests := []struct {
		input      string
		wantCode   string
		wantMarket string
		wantOK     bool
	}{
		{input: "600519", wantCode: "600519", wantMarket: "17", wantOK: true},
		{input: "000409.SZ", wantCode: "000409", wantMarket: "33", wantOK: true},
		{input: "SH600519", wantCode: "600519", wantMarket: "17", wantOK: true},
		{input: "830799.BJ", wantOK: false},
		{input: "bad-code", wantOK: false},
	}
	for _, test := range tests {
		code, market, ok := thsCodeAndMarket(test.input)
		if code != test.wantCode || market != test.wantMarket || ok != test.wantOK {
			t.Errorf("thsCodeAndMarket(%q) = (%q, %q, %v), want (%q, %q, %v)", test.input, code, market, ok, test.wantCode, test.wantMarket, test.wantOK)
		}
	}
}

func TestTHSTimePeriod(t *testing.T) {
	for klt, want := range map[string]string{
		"1": "min_1", "5": "min_5", "60": "min_60",
		"101": "day_1", "102": "week_1", "103": "month_1", "104": "quarter_1", "106": "year_1",
	} {
		got, ok := thsTimePeriod(klt)
		if !ok || got != want {
			t.Errorf("thsTimePeriod(%q) = (%q, %v), want (%q, true)", klt, got, ok, want)
		}
	}
	if _, ok := thsTimePeriod("105"); ok {
		t.Fatal("thsTimePeriod(105) unexpectedly accepted an unsupported period")
	}
}

func TestTHSEndTime(t *testing.T) {
	got, ok := thsEndTime("2026-01-30")
	if !ok {
		t.Fatal("date-only end should parse")
	}
	want := time.Date(2026, 1, 30, 23, 59, 59, 999000000, time.FixedZone("Asia/Shanghai", 8*60*60)).UnixMilli()
	if got != want {
		t.Fatalf("thsEndTime(date) = %d, want %d", got, want)
	}
	if got, ok := thsEndTime(""); !ok || got != 0 {
		t.Fatalf("thsEndTime(empty) = (%d, %v), want (0, true)", got, ok)
	}
	if _, ok := thsEndTime("not-a-date"); ok {
		t.Fatal("invalid end date should be rejected")
	}
}

func TestParseTHSKLineValues(t *testing.T) {
	loc := time.FixedZone("Asia/Shanghai", 8*60*60)
	jan29 := time.Date(2026, 1, 29, 0, 0, 0, 0, loc).UnixMilli()
	jan30 := time.Date(2026, 1, 30, 0, 0, 0, 0, loc).UnixMilli()
	feb2 := time.Date(2026, 2, 2, 0, 0, 0, 0, loc).UnixMilli()
	fields := []string{"19", "1", "13", "7", "8", "9", "11"}
	rows := [][]json.RawMessage{
		{json.RawMessage(`10000`), json.RawMessage(strconv.FormatInt(jan29, 10)), json.RawMessage(`10000`), json.RawMessage(`10`), json.RawMessage(`11`), json.RawMessage(`9`), json.RawMessage(`10`)},
		{json.RawMessage(`20000`), json.RawMessage(strconv.FormatInt(jan30, 10)), json.RawMessage(`10030`), json.RawMessage(`10`), json.RawMessage(`11`), json.RawMessage(`10`), json.RawMessage(`10.5`)},
		{json.RawMessage(`30000`), json.RawMessage(strconv.FormatInt(feb2, 10)), json.RawMessage(`12000`), json.RawMessage(`10.5`), json.RawMessage(`12`), json.RawMessage(`10`), json.RawMessage(`11`)},
	}
	endTime, _ := thsEndTime("2026-01-30")
	got := parseTHSKLineValues(fields, rows, "day_1", endTime)
	if len(got) != 2 {
		t.Fatalf("parsed %d rows, want 2", len(got))
	}
	last := got[1]
	if last.Day != "2026-01-30" || last.Open != "10.00" || last.Close != "10.50" || last.Volume != "100.3" || last.Amount != "20000.00" {
		t.Fatalf("unexpected mapped bar: %+v", last)
	}
	if last.ChangeValue != "0.50" || last.ChangePercent != "5.00" || last.Amplitude != "10.00" {
		t.Fatalf("unexpected derived fields: %+v", last)
	}
}

func TestTHSKLineLiveAdapter(t *testing.T) {
	if os.Getenv("THS_KLINE_LIVE_PROBE") != "1" {
		t.Skip("set THS_KLINE_LIVE_PROBE=1 to call the live THS endpoint")
	}
	result := fetchFromTHS("000409.SZ", "101", 5, "")
	if result == nil || result.Data == nil || len(*result.Data) == 0 {
		t.Fatal("THS adapter returned no daily bars")
	}
	rows := *result.Data
	start := len(rows) - 3
	if start < 0 {
		start = 0
	}
	t.Logf("source=ths code=000409 rows=%d latest=%+v", len(rows), rows[start:])

	historical := fetchFromTHS("600519.SH", "101", 5, "2026-01-30")
	if historical == nil || historical.Data == nil || len(*historical.Data) == 0 {
		t.Fatal("THS adapter returned no historical bars")
	}
	historicalRows := *historical.Data
	last := historicalRows[len(historicalRows)-1]
	if last.Day != "2026-01-30" {
		t.Fatalf("historical cutoff returned %s, want 2026-01-30", last.Day)
	}
	t.Logf("source=ths code=600519 cutoff=2026-01-30 latest=%+v", last)
}
