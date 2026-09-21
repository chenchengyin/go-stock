package t0reference

import "go-stock/backend/analysis/candlepattern"

type PreT0Bar struct {
	BarSnapshot
	CloseType string
	IsOneWord bool
	Amplitude float64
	OpenRet   float64
	CloseRet  float64
}

type PreT0View struct {
	Bars map[int]PreT0Bar
}

type EntryView struct {
	Gap float64
}

func BuildPreT0View(obs Observation) PreT0View {
	view := PreT0View{Bars: make(map[int]PreT0Bar, len(obs.Bars))}
	for offset, bar := range obs.Bars {
		view.Bars[offset] = buildPreT0Bar(bar)
	}
	return view
}

func buildPreT0Bar(bar BarSnapshot) PreT0Bar {
	classified := candlepattern.ClassifyDailyBar(bar.PrevClose, candlepattern.DailyBar{
		Date:     bar.Date,
		Open:     bar.Open,
		Close:    bar.Close,
		High:     bar.High,
		Low:      bar.Low,
		Volume:   bar.Volume,
		AmountYi: bar.AmountYi,
	})
	return PreT0Bar{
		BarSnapshot: bar,
		CloseType:   string(classified),
		IsOneWord:   classified == candlepattern.BarZT && bar.Open == bar.Close && bar.High == bar.Close && bar.Low == bar.Close,
		Amplitude:   percentFrom(bar.PrevClose, bar.High-bar.Low),
		OpenRet:     percentChange(bar.PrevClose, bar.Open),
		CloseRet:    percentChange(bar.PrevClose, bar.Close),
	}
}

func percentChange(from, to float64) float64 {
	if from <= 0 {
		return 0
	}
	return (to - from) / from * 100
}

func percentFrom(base, value float64) float64 {
	if base <= 0 {
		return 0
	}
	return value / base * 100
}
