package flutter_api

import (
	"testing"

	"go-stock/backend/data"
)

func TestSelectPreviousHighUsesLatestCompletedTradingDay(t *testing.T) {
	bars := []data.KLineData{
		{Day: "2026-09-18", High: "12.30"},
		{Day: "2026-09-19", High: "12.80"},
		{Day: "2026-09-21", High: "13.10"},
	}

	got, ok := selectPreviousHigh(bars, "2026-09-21")
	if !ok {
		t.Fatal("selectPreviousHigh returned no completed bar")
	}
	if got != 12.80 {
		t.Fatalf("previous high = %.2f, want 12.80", got)
	}
}

func TestSelectPreviousHighSkipsCurrentAndFutureBars(t *testing.T) {
	bars := []data.KLineData{
		{Day: "2026-09-21", High: "13.10"},
		{Day: "2026-09-22", High: "14.10"},
	}

	if _, ok := selectPreviousHigh(bars, "2026-09-21"); ok {
		t.Fatal("expected no previous high when no completed bar exists")
	}
}

func TestSelectPreviousHighRejectsInvalidHigh(t *testing.T) {
	bars := []data.KLineData{
		{Day: "2026-09-18", High: "not-a-price"},
	}

	if _, ok := selectPreviousHigh(bars, "2026-09-21"); ok {
		t.Fatal("expected invalid high to be rejected")
	}
}
