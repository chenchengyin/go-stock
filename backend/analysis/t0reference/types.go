package t0reference

import "errors"

const (
	EntrySourceDailyOpen   = "daily_open"
	EntrySourceAuction0925 = "auction_0925"
)

var ErrIncompleteObservation = errors.New("incomplete T0 reference observation")

type Entry struct {
	Price  float64
	Source string
}

type BarSnapshot struct {
	Date      string
	PrevClose float64
	Open      float64
	High      float64
	Low       float64
	Close     float64
	Volume    float64
	AmountYi  float64
}

type Observation struct {
	TradeDate   string
	StockCode   string
	Bars        map[int]BarSnapshot
	T0          BarSnapshot
	EntryPrice  float64
	EntryGap    float64
	EntrySource string
	PnL         float64
}
