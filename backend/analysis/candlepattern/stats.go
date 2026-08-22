package candlepattern

import (
	"sort"
)

const (
	t0MinGap = 0.01
	t0MaxGap = 3.0
	t0WinPnL = 2.5
)

func CollectObservations(cache *DailyCache, tradeDate string, window int) []Observation {
	pool := FilterA2Pool(cache, tradeDate)
	out := make([]Observation, 0, len(pool))
	for sc, hist := range pool {
		bars := cache.Daily[sc]
		if len(bars) == 0 || bars[len(bars)-1].Date != tradeDate {
			continue
		}
		labels, ok := BuildPatternLabels(hist, window)
		if !ok {
			continue
		}
		pattern := FormatPattern(labels)
		obsBar := bars[len(bars)-1]
		prevClose := hist[len(hist)-1].Close
		nextYang := obsBar.Close > obsBar.Open
		var gap, pnl float64
		inT0 := false
		if prevClose > 0 && obsBar.Open > 0 {
			gap = (obsBar.Open - prevClose) / prevClose * 100
			closeRet := (obsBar.Close - prevClose) / prevClose * 100
			pnl = closeRet - gap
			if gap >= t0MinGap && gap <= t0MaxGap {
				inT0 = true
			}
		}
		out = append(out, Observation{
			ShortCode:  sc,
			Pattern:    pattern,
			Window:     window,
			NextYang:   nextYang,
			T0Gap:      gap,
			T0PnL:      pnl,
			InT0Subset: inT0,
		})
	}
	return out
}

func AggregateStats(obs []Observation, window int, minSamples int) []PatternStat {
	type acc struct {
		total, yang int
		t0Total     int
		t0Wins      int
		t0PnLs      []float64
	}
	groups := map[string]*acc{}
	for _, o := range obs {
		if o.Window != window {
			continue
		}
		a := groups[o.Pattern]
		if a == nil {
			a = &acc{}
			groups[o.Pattern] = a
		}
		a.total++
		if o.NextYang {
			a.yang++
		}
		if o.InT0Subset {
			a.t0Total++
			a.t0PnLs = append(a.t0PnLs, o.T0PnL)
			if o.T0PnL >= t0WinPnL {
				a.t0Wins++
			}
		}
	}

	patterns := make([]string, 0, len(groups))
	for p := range groups {
		patterns = append(patterns, p)
	}
	sort.Strings(patterns)

	stats := make([]PatternStat, 0, len(patterns))
	for _, p := range patterns {
		a := groups[p]
		st := PatternStat{
			Pattern:       p,
			Window:        window,
			SampleCount:   a.total,
			T0SubsetCount: a.t0Total,
			Insufficient:  a.total < minSamples,
		}
		if a.total > 0 {
			st.NextYangRate = float64(a.yang) / float64(a.total)
		}
		if a.t0Total > 0 {
			st.T0WinRate2p5 = float64(a.t0Wins) / float64(a.t0Total)
			st.T0AvgPnL = mean(a.t0PnLs)
			st.T0MedianPnL = median(a.t0PnLs)
		}
		stats = append(stats, st)
	}
	return stats
}

func mean(vals []float64) float64 {
	if len(vals) == 0 {
		return 0
	}
	var s float64
	for _, v := range vals {
		s += v
	}
	return s / float64(len(vals))
}

func median(vals []float64) float64 {
	if len(vals) == 0 {
		return 0
	}
	cp := append([]float64(nil), vals...)
	sort.Float64s(cp)
	mid := len(cp) / 2
	if len(cp)%2 == 0 {
		return (cp[mid-1] + cp[mid]) / 2
	}
	return cp[mid]
}
