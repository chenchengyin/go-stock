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
	blueMinWin  = 45.0
	blueMaxFail = 40.0
	minSamples  = 10
	t0Win       = 2.5
)

func pct(n, d int) float64 {
	if d == 0 {
		return 0
	}
	return float64(n) / float64(d) * 100
}

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
	for _, b := range buckets {
		cnt := 0
		for _, p := range pnls {
			if p >= b.lo && p < b.hi {
				cnt++
			}
		}
		bdist[b.label] = math.Round(pct(cnt, len(obs))*10) / 10
	}
	exp := 0.0
	if len(obs) > 0 {
		exp = mean(pnls)
	}
	return map[string]interface{}{
		"label":          label,
		"n":              len(obs),
		"win_pct":        math.Round(pct(w, len(obs))*10) / 10,
		"fail_pct":       math.Round(pct(f, len(obs))*10) / 10,
		"flat_pct":       math.Round(pct(flat, len(obs))*10) / 10,
		"t0_pnl_avg":     math.Round(mean(pnls)*100) / 100,
		"t0_pnl_med":     math.Round(median(pnls)*100) / 100,
		"expectancy":     math.Round(exp*100) / 100,
		"avg_win_when_pos": func() float64 {
			if posN == 0 {
				return 0
			}
			return math.Round(posSum/float64(posN)*100) / 100
		}(),
		"avg_loss_when_neg": func() float64 {
			if negN == 0 {
				return 0
			}
			return math.Round(negSum/float64(negN)*100) / 100
		}(),
		"gap_avg":       math.Round(mean(gaps)*100) / 100,
		"close_ret_avg": math.Round(mean(closeRets)*100) / 100,
		"t0_pnl_dist_pct": bdist,
	}
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
	var all []candlepattern.Observation
	for _, d := range dates {
		cache, err := candlepattern.LoadDailyCache(root, d)
		if err != nil {
			continue
		}
		obs := candlepattern.CollectObservations(cache, d, 3)
		all = append(all, obs...)
		for _, o := range obs {
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

	bluePatterns := map[string]struct{}{}
	type patRow struct {
		Pattern  string  `json:"pattern"`
		N        int     `json:"n"`
		WinPct   float64 `json:"win_pct"`
		FailPct  float64 `json:"fail_pct"`
		AvgPnL   float64 `json:"avg_pnl"`
	}
	var blueRows []patRow
	for p, a := range groups {
		n := len(a.pnls)
		if n < minSamples {
			continue
		}
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
		if winR >= blueMinWin && failR <= blueMaxFail {
			bluePatterns[p] = struct{}{}
			blueRows = append(blueRows, patRow{
				Pattern: p, N: n,
				WinPct: math.Round(winR*10) / 10,
				FailPct: math.Round(failR*10) / 10,
				AvgPnL: math.Round(mean(a.pnls)*100) / 100,
			})
		}
	}
	sort.Slice(blueRows, func(i, j int) bool { return blueRows[i].WinPct > blueRows[j].WinPct })

	var allT0, blueT0 []candlepattern.Observation
	type sample struct {
		Date, Code, Name, Pattern string
		Gap, T0PnL, CloseRet      float64
	}
	var winSamples, failSamples []sample
	nameOf := map[string]string{}

	for _, d := range dates {
		cache, err := candlepattern.LoadDailyCache(root, d)
		if err != nil {
			continue
		}
		for _, s := range cache.Stocks {
			nameOf[s.ShortCode] = s.Name
		}
	}

	for _, d := range dates {
		cache, err := candlepattern.LoadDailyCache(root, d)
		if err != nil {
			continue
		}
		for _, o := range candlepattern.CollectObservations(cache, d, 3) {
			if !o.InT0Subset {
				continue
			}
			allT0 = append(allT0, o)
			if _, ok := bluePatterns[o.Pattern]; !ok {
				continue
			}
			blueT0 = append(blueT0, o)
			s := sample{
				Date: d, Code: o.ShortCode, Name: nameOf[o.ShortCode], Pattern: o.Pattern,
				Gap: o.T0Gap, T0PnL: o.T0PnL, CloseRet: o.T0Gap + o.T0PnL,
			}
			if o.T0PnL >= t0Win && len(winSamples) < 5 {
				winSamples = append(winSamples, s)
			}
			if o.T0PnL < 0 && len(failSamples) < 5 {
				failSamples = append(failSamples, s)
			}
		}
	}

	out := map[string]interface{}{
		"date_start":         dates[0],
		"date_end":           dates[len(dates)-1],
		"trading_days":       len(dates),
		"blue_rule":          fmt.Sprintf("N>=%d 且 达标>=%.0f%% 且 真亏<=%.0f%%", minSamples, blueMinWin, blueMaxFail),
		"blue_pattern_count": len(bluePatterns),
		"blue_patterns":      blueRows,
		"all_t0":             summarize("全T0池", allT0),
		"blue_t0":            summarize("蓝色形态命中T0", blueT0),
		"win_samples":        winSamples,
		"fail_samples":       failSamples,
		"lift_vs_all": map[string]interface{}{
			"win_pp":  math.Round((summarize("", blueT0)["win_pct"].(float64) - summarize("", allT0)["win_pct"].(float64)) * 10) / 10,
			"fail_pp": math.Round((summarize("", blueT0)["fail_pct"].(float64) - summarize("", allT0)["fail_pct"].(float64)) * 10) / 10,
			"avg_pnl_pp": math.Round((summarize("", blueT0)["t0_pnl_avg"].(float64) - summarize("", allT0)["t0_pnl_avg"].(float64)) * 100) / 100,
		},
	}
	enc := json.NewEncoder(os.Stdout)
	enc.SetIndent("", "  ")
	_ = enc.Encode(out)
}
