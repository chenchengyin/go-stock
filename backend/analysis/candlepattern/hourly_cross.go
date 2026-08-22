package candlepattern

type hourlyCrossAcc struct {
	total, yang int
	t0Total     int
	t0Wins      int
	t0PnLs      []float64
}

func CollectHourlyCrossStats(
	cache *DailyCache,
	cacheRoot, tradeDate string,
	window int,
	tailNs []int,
	fetcher HourlyFetcher,
) []HourlyCrossStat {
	obs := CollectObservations(cache, tradeDate, window)
	type key struct {
		daily, hourly string
		tail          int
	}
	groups := map[key]*hourlyCrossAcc{}

	for _, o := range obs {
		bars := cache.Daily[o.ShortCode]
		dates := tradingDatesBefore(bars, tradeDate, 3)
		hourly, err := LoadOrFetchHourly3D(cacheRoot, o.ShortCode, tradeDate, fetcher)
		if err != nil {
			continue
		}
		hourly = filterHourlyByDates(hourly, dates)
		if len(hourly) == 0 {
			continue
		}
		for _, tailN := range tailNs {
			hp, ok := tailHourPattern(hourly, tailN)
			if !ok {
				continue
			}
			k := key{daily: o.Pattern, hourly: hp, tail: tailN}
			a := groups[k]
			if a == nil {
				a = &hourlyCrossAcc{}
				groups[k] = a
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
	}

	out := make([]HourlyCrossStat, 0, len(groups))
	for k, a := range groups {
		st := HourlyCrossStat{
			DailyPattern:  k.daily,
			HourlyPattern: k.hourly,
			Window:        window,
			HourlyTail:    k.tail,
			SampleCount:   a.total,
			T0SubsetCount: a.t0Total,
		}
		if a.total > 0 {
			st.NextYangRate = float64(a.yang) / float64(a.total)
		}
		if a.t0Total > 0 {
			st.T0WinRate2p5 = float64(a.t0Wins) / float64(a.t0Total)
			st.T0AvgPnL = mean(a.t0PnLs)
		}
		out = append(out, st)
	}
	return out
}

func MarkInsufficientHourly(stats []HourlyCrossStat, minSamples int) {
	for i := range stats {
		stats[i].Insufficient = stats[i].SampleCount < minSamples
	}
}
