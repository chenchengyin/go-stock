package t0reference

import (
	"math"
	"testing"
)

func TestBuildObservationUsesTradingDayOffsetsAndEntryPrice(t *testing.T) {
	bars := []BarSnapshot{
		{Date: "2026-01-02", PrevClose: 8.5, Open: 8.6, High: 9.1, Low: 8.4, Close: 9.0},
		{Date: "2026-01-05", PrevClose: 9.0, Open: 9.1, High: 10.2, Low: 9.0, Close: 10.0},
		{Date: "2026-01-06", PrevClose: 10.0, Open: 10.1, High: 10.8, Low: 9.9, Close: 10.5},
		{Date: "2026-01-07", PrevClose: 9.8, Open: 10.0, High: 10.7, Low: 9.7, Close: 10.5},
	}

	obs, err := BuildObservation(
		"600000",
		"2026-01-07",
		bars,
		Entry{Price: 10.1, Source: EntrySourceAuction0925},
	)
	if err != nil {
		t.Fatal(err)
	}
	if obs.Bars[-3].Date != "2026-01-02" ||
		obs.Bars[-2].Date != "2026-01-05" ||
		obs.Bars[-1].Date != "2026-01-06" {
		t.Fatalf("trading-day offsets = %#v", obs.Bars)
	}
	if obs.T0.Date != "2026-01-07" {
		t.Fatalf("T0 date = %q", obs.T0.Date)
	}
	wantGap := (10.1 - 9.8) / 9.8 * 100
	wantPnL := (10.5 - 10.1) / 10.1 * 100
	if math.Abs(obs.EntryGap-wantGap) > 1e-9 {
		t.Fatalf("entry gap = %.12f want %.12f", obs.EntryGap, wantGap)
	}
	if math.Abs(obs.PnL-wantPnL) > 1e-9 {
		t.Fatalf("pnl = %.12f want %.12f", obs.PnL, wantPnL)
	}
	if obs.EntryPrice != 10.1 || obs.EntrySource != EntrySourceAuction0925 {
		t.Fatalf("entry = %.4f/%q", obs.EntryPrice, obs.EntrySource)
	}
}

func TestBuildObservationRejectsMissingTradingDay(t *testing.T) {
	bars := []BarSnapshot{
		{Date: "2026-01-02", PrevClose: 8.5, Open: 8.6, High: 9.1, Low: 8.4, Close: 9.0},
		{Date: "2026-01-06", PrevClose: 10.0, Open: 10.1, High: 10.8, Low: 9.9, Close: 10.5},
		{Date: "2026-01-07", PrevClose: 9.8, Open: 10.0, High: 10.7, Low: 9.7, Close: 10.5},
	}

	if _, err := BuildObservation(
		"600000",
		"2026-01-07",
		bars,
		Entry{Price: 10.1, Source: EntrySourceDailyOpen},
	); err == nil {
		t.Fatal("missing T-2 should reject the observation")
	}
}
