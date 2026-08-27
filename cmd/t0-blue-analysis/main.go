package main

import (
	"encoding/json"
	"fmt"
	"math"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"go-stock/backend/analysis/candlepattern"
)

const (
	blueMinWin   = 45.0
	blueMaxFail  = 40.0
	greenMinWin  = 30.0
	greenMaxFail = 45.0
	redMinFail   = 52.0
	redMaxWin    = 22.0
	minSamples   = 10
	t0Win        = 2.5
)

func pct(n, d int) float64 {
	if d == 0 {
		return 0
	}
	return float64(n) / float64(d) * 100
}

func r1(x float64) float64 { return math.Round(x*10) / 10 }
func r2(x float64) float64 { return math.Round(x*100) / 100 }

func median(xs []float64) float64 {
	if len(xs) == 0 {
		return 0
	}
	cp := append([]float64(nil), xs...)
	sort.Float64s(cp)
	m := len(cp) / 2
	if len(cp)%2 == 1 {
		return cp[m]
	}
	return (cp[m-1] + cp[m]) / 2
}

func mean(xs []float64) float64 {
	if len(xs) == 0 {
		return 0
	}
	s := 0.0
	for _, x := range xs {
		s += x
	}
	return s / float64(len(xs))
}

type ohlcPct struct {
	O float64 `json:"o"`
	C float64 `json:"c"`
	H float64 `json:"h"`
	L float64 `json:"l"`
}

func relOHLC(bar candlepattern.DailyBar, prevClose float64) ohlcPct {
	if prevClose == 0 {
		return ohlcPct{}
	}
	f := func(p float64) float64 { return r2((p - prevClose) / prevClose * 100) }
	return ohlcPct{O: f(bar.Open), C: f(bar.Close), H: f(bar.High), L: f(bar.Low)}
}

type bucket struct {
	label string
	lo    float64
	hi    float64
}

func summarize(label string, obs []candlepattern.Observation) map[string]interface{} {
	pnls := make([]float64, len(obs))
	gaps := make([]float64, len(obs))
	closeRets := make([]float64, len(obs))
	w, f, flat := 0, 0, 0
	posSum, negSum := 0.0, 0.0
	posN, negN := 0, 0
	closeRed, fade := 0, 0
	for i, o := range obs {
		pnls[i] = o.T0PnL
		gaps[i] = o.T0Gap
		closeRets[i] = o.T0PnL + o.T0Gap
		switch {
		case o.T0PnL >= t0Win:
			w++
		case o.T0PnL < 0:
			f++
			negSum += o.T0PnL
			negN++
			if closeRets[i] < 0 {
				closeRed++
			} else {
				fade++
			}
		default:
			flat++
		}
		if o.T0PnL > 0 {
			posSum += o.T0PnL
			posN++
		}
	}
	buckets := []bucket{
		{"<-3%", -999, -3},
		{"-3~0%", -3, 0},
		{"0~2.5%", 0, t0Win},
		{"2.5~5%", t0Win, 5},
		{"5~10%", 5, 10},
		{">=10%", 10, 999},
	}
	bdist := map[string]float64{}
	bcnt := map[string]int{}
	for _, b := range buckets {
		cnt := 0
		for _, p := range pnls {
			if p >= b.lo && p < b.hi {
				cnt++
			}
		}
		bdist[b.label] = r1(pct(cnt, len(obs)))
		bcnt[b.label] = cnt
	}
	avgWin := 0.0
	if posN > 0 {
		avgWin = r2(posSum / float64(posN))
	}
	avgLoss := 0.0
	if negN > 0 {
		avgLoss = r2(negSum / float64(negN))
	}
	return map[string]interface{}{
		"label":             label,
		"n":                 len(obs),
		"win_pct":           r1(pct(w, len(obs))),
		"fail_pct":          r1(pct(f, len(obs))),
		"flat_pct":          r1(pct(flat, len(obs))),
		"win_n":             w,
		"fail_n":            f,
		"flat_n":            flat,
		"t0_pnl_avg":        r2(mean(pnls)),
		"t0_pnl_med":        r2(median(pnls)),
		"avg_win_when_pos":  avgWin,
		"avg_loss_when_neg": avgLoss,
		"gap_avg":           r2(mean(gaps)),
		"close_ret_avg":     r2(mean(closeRets)),
		"t0_pnl_dist_pct":   bdist,
		"t0_pnl_dist_n":     bcnt,
		"fail_close_red_n":  closeRed,
		"fail_fade_n":       fade,
		"fail_close_red_pct": func() float64 {
			if f == 0 {
				return 0
			}
			return r1(pct(closeRed, f))
		}(),
		"fail_fade_pct": func() float64 {
			if f == 0 {
				return 0
			}
			return r1(pct(fade, f))
		}(),
	}
}

type sample struct {
	Date     string    `json:"date"`
	Code     string    `json:"code"`
	Name     string    `json:"name"`
	Pattern  string    `json:"pattern"`
	Gap      float64   `json:"gap"`
	T0PnL    float64   `json:"t0_pnl"`
	CloseRet float64   `json:"close_ret"`
	Hist     []ohlcPct `json:"hist,omitempty"`
	T0Bar    *ohlcPct  `json:"t0_bar,omitempty"`
}

func signalOf(winR, failR float64, n int) string {
	if n < minSamples {
		return "insufficient"
	}
	if winR >= blueMinWin && failR <= blueMaxFail {
		return "blue"
	}
	if failR <= greenMaxFail && winR >= greenMinWin {
		return "green"
	}
	if failR > redMinFail || winR < redMaxWin {
		return "red"
	}
	return "yellow"
}

func main() {
	root, err := candlepattern.ResolveCacheRoot(".")
	if err != nil {
		panic(err)
	}
	dailyDir := filepath.Join(root, "t0", "daily")
	entries, _ := os.ReadDir(dailyDir)
	var dates []string
	for _, e := range entries {
		name := e.Name()
		if !strings.HasPrefix(name, "t0_daily_cache_") || !strings.HasSuffix(name, ".gob") {
			continue
		}
		d := strings.TrimSuffix(strings.TrimPrefix(name, "t0_daily_cache_"), ".gob")
		dates = append(dates, d)
	}
	sort.Strings(dates)

	type acc struct{ pnls []float64 }
	groups := map[string]*acc{}
	nameOf := map[string]string{}
	for _, d := range dates {
		cache, err := candlepattern.LoadDailyCache(root, d)
		if err != nil {
			continue
		}
		for _, s := range cache.Stocks {
			nameOf[s.ShortCode] = s.Name
		}
		for _, o := range candlepattern.CollectObservations(cache, d, 3) {
			if !o.InT0Subset {
				continue
			}
			a := groups[o.Pattern]
			if a == nil {
				a = &acc{}
				groups[o.Pattern] = a
			}
			a.pnls = append(a.pnls, o.T0PnL)
		}
	}

	type patRow struct {
		Pattern string  `json:"pattern"`
		N       int     `json:"n"`
		WinPct  float64 `json:"win_pct"`
		FailPct float64 `json:"fail_pct"`
		AvgPnL  float64 `json:"avg_pnl"`
		MedPnL  float64 `json:"med_pnl"`
		Signal  string  `json:"signal"`
	}
	bluePatterns := map[string]struct{}{}
	patSignal := map[string]string{}
	var blueRows, allPatRows []patRow
	for p, a := range groups {
		n := len(a.pnls)
		wins, fails := 0, 0
		for _, pnl := range a.pnls {
			if pnl >= t0Win {
				wins++
			}
			if pnl < 0 {
				fails++
			}
		}
		winR := pct(wins, n)
		failR := pct(fails, n)
		sig := signalOf(winR, failR, n)
		patSignal[p] = sig
		row := patRow{
			Pattern: p, N: n,
			WinPct: r1(winR), FailPct: r1(failR),
			AvgPnL: r2(mean(a.pnls)), MedPnL: r2(median(a.pnls)),
			Signal: sig,
		}
		allPatRows = append(allPatRows, row)
		if sig == "blue" {
			bluePatterns[p] = struct{}{}
			blueRows = append(blueRows, row)
		}
	}
	sort.Slice(blueRows, func(i, j int) bool {
		if blueRows[i].WinPct != blueRows[j].WinPct {
			return blueRows[i].WinPct > blueRows[j].WinPct
		}
		return blueRows[i].N > blueRows[j].N
	})

	var allT0, blueT0, greenT0, yellowT0, redT0, insuffT0 []candlepattern.Observation
	byMonthBlue := map[string][]candlepattern.Observation{}
	var winSamples, failSamples, featured []sample
	bestByPat := map[string]sample{}

	attachBars := func(cache *candlepattern.DailyCache, d, sc string, s *sample) {
		bars := cache.Daily[sc]
		hist := candlepattern.HistBeforeTradeDate(bars, d)
		if len(hist) < 4 || len(bars) == 0 {
			return
		}
		obsBar := bars[len(bars)-1]
		if obsBar.Date != d {
			return
		}
		h3 := hist[len(hist)-3:]
		prev := hist[len(hist)-4].Close
		out := make([]ohlcPct, 0, 3)
		for _, b := range h3 {
			out = append(out, relOHLC(b, prev))
			prev = b.Close
		}
		s.Hist = out
		t0 := relOHLC(obsBar, hist[len(hist)-1].Close)
		s.T0Bar = &t0
	}

	for _, d := range dates {
		cache, err := candlepattern.LoadDailyCache(root, d)
		if err != nil {
			continue
		}
		obsList := candlepattern.CollectObservations(cache, d, 3)
		for _, o := range obsList {
			if !o.InT0Subset {
				continue
			}
			allT0 = append(allT0, o)
			switch patSignal[o.Pattern] {
			case "blue":
				blueT0 = append(blueT0, o)
			case "green":
				greenT0 = append(greenT0, o)
			case "yellow":
				yellowT0 = append(yellowT0, o)
			case "red":
				redT0 = append(redT0, o)
			default:
				insuffT0 = append(insuffT0, o)
			}
			if _, ok := bluePatterns[o.Pattern]; !ok {
				continue
			}
			month := d[:7]
			byMonthBlue[month] = append(byMonthBlue[month], o)
			s := sample{
				Date: d, Code: o.ShortCode, Name: nameOf[o.ShortCode], Pattern: o.Pattern,
				Gap: r2(o.T0Gap), T0PnL: r2(o.T0PnL), CloseRet: r2(o.T0Gap + o.T0PnL),
			}
			if o.T0PnL >= t0Win {
				if len(winSamples) < 8 {
					attachBars(cache, d, o.ShortCode, &s)
					winSamples = append(winSamples, s)
				}
				prev, ok := bestByPat[o.Pattern]
				if !ok || o.T0PnL > prev.T0PnL {
					attachBars(cache, d, o.ShortCode, &s)
					bestByPat[o.Pattern] = s
				}
			}
			if o.T0PnL < 0 && len(failSamples) < 8 {
				attachBars(cache, d, o.ShortCode, &s)
				failSamples = append(failSamples, s)
			}
		}
	}

	for _, row := range blueRows {
		if s, ok := bestByPat[row.Pattern]; ok {
			featured = append(featured, s)
		}
		if len(featured) >= 8 {
			break
		}
	}

	type monthRow struct {
		Month   string  `json:"month"`
		N       int     `json:"n"`
		WinPct  float64 `json:"win_pct"`
		FailPct float64 `json:"fail_pct"`
		AvgPnL  float64 `json:"avg_pnl"`
	}
	var months []string
	for m := range byMonthBlue {
		months = append(months, m)
	}
	sort.Strings(months)
	var monthRows []monthRow
	for _, m := range months {
		sm := summarize(m, byMonthBlue[m])
		monthRows = append(monthRows, monthRow{
			Month: m, N: sm["n"].(int),
			WinPct: sm["win_pct"].(float64), FailPct: sm["fail_pct"].(float64),
			AvgPnL: sm["t0_pnl_avg"].(float64),
		})
	}

	gapBuckets := []bucket{
		{"0.01~0.5%", 0.01, 0.5},
		{"0.5~1%", 0.5, 1},
		{"1~2%", 1, 2},
		{"2~3%", 2, 3.0001},
	}
	type gapRow struct {
		Label  string  `json:"label"`
		N      int     `json:"n"`
		WinPct float64 `json:"win_pct"`
		FailPct float64 `json:"fail_pct"`
		AvgPnL float64 `json:"avg_pnl"`
	}
	var gapRows []gapRow
	for _, b := range gapBuckets {
		var sub []candlepattern.Observation
		for _, o := range blueT0 {
			if o.T0Gap >= b.lo && o.T0Gap < b.hi {
				sub = append(sub, o)
			}
		}
		sm := summarize(b.label, sub)
		gapRows = append(gapRows, gapRow{
			Label: b.label, N: sm["n"].(int),
			WinPct: sm["win_pct"].(float64), FailPct: sm["fail_pct"].(float64),
			AvgPnL: sm["t0_pnl_avg"].(float64),
		})
	}

	blueS := summarize("蓝色形态命中T0", blueT0)
	allS := summarize("全T0池", allT0)
	greenS := summarize("绿灯形态", greenT0)
	yellowS := summarize("黄灯形态", yellowT0)
	redS := summarize("红灯形态", redT0)
	insS := summarize("灰灯/样本不足", insuffT0)

	out := map[string]interface{}{
		"date_start":         dates[0],
		"date_end":           dates[len(dates)-1],
		"trading_days":       len(dates),
		"blue_rule":          fmt.Sprintf("N>=%d 且 达标>=%.0f%% 且 真亏<=%.0f%%", minSamples, blueMinWin, blueMaxFail),
		"blue_pattern_count": len(bluePatterns),
		"pattern_universe_n": len(allPatRows),
		"blue_patterns":      blueRows,
		"all_t0":             allS,
		"blue_t0":            blueS,
		"green_t0":           greenS,
		"yellow_t0":          yellowS,
		"red_t0":             redS,
		"insufficient_t0":    insS,
		"by_month":           monthRows,
		"by_gap":             gapRows,
		"win_samples":        winSamples,
		"fail_samples":       failSamples,
		"featured":           featured,
		"lift_vs_all": map[string]interface{}{
			"win_pp":     r1(blueS["win_pct"].(float64) - allS["win_pct"].(float64)),
			"fail_pp":    r1(blueS["fail_pct"].(float64) - allS["fail_pct"].(float64)),
			"avg_pnl_pp": r2(blueS["t0_pnl_avg"].(float64) - allS["t0_pnl_avg"].(float64)),
		},
	}
	enc := json.NewEncoder(os.Stdout)
	enc.SetIndent("", "  ")
	_ = enc.Encode(out)
}
