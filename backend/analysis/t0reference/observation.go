package t0reference

import (
	"sort"
)

// BuildObservation builds one immutable T0 reference sample from trading-day
// bars. The input must contain the T0 bar and at least the three preceding
// trading-day bars; calendar gaps are intentionally ignored.
func BuildObservation(stockCode, tradeDate string, input []BarSnapshot, entry Entry) (Observation, error) {
	if stockCode == "" || tradeDate == "" || entry.Price <= 0 || entry.Source == "" {
		return Observation{}, ErrIncompleteObservation
	}
	if len(input) < 4 {
		return Observation{}, ErrIncompleteObservation
	}

	bars := append([]BarSnapshot(nil), input...)
	sort.SliceStable(bars, func(i, j int) bool {
		return bars[i].Date < bars[j].Date
	})

	t0Index := -1
	for i := range bars {
		if bars[i].Date == tradeDate {
			t0Index = i
		}
	}
	if t0Index < 3 {
		return Observation{}, ErrIncompleteObservation
	}
	t0 := bars[t0Index]
	if t0.PrevClose <= 0 {
		return Observation{}, ErrIncompleteObservation
	}

	entryGap := (entry.Price - t0.PrevClose) / t0.PrevClose * 100
	pnl := (t0.Close - entry.Price) / entry.Price * 100
	return Observation{
		TradeDate:   tradeDate,
		StockCode:   stockCode,
		Bars:        map[int]BarSnapshot{-3: bars[t0Index-3], -2: bars[t0Index-2], -1: bars[t0Index-1]},
		T0:          t0,
		EntryPrice:  entry.Price,
		EntryGap:    entryGap,
		EntrySource: entry.Source,
		PnL:         pnl,
	}, nil
}
