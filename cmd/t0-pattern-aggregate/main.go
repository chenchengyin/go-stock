package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"go-stock/backend/analysis/candlepattern"
)

type patternAcc struct {
	t0PnLs []float64
}

type aggRow struct {
	Pattern  string  `json:"pattern"`
	T0N      int     `json:"t0_n"`
	AvgT0    float64 `json:"avg_t0"`
	MedT0    float64 `json:"med_t0"`
	WinRate  float64 `json:"win_rate"`
	FailRate float64 `json:"fail_rate"`
}

type lastBarAcc struct {
	total, wins, fails int
}

type report struct {
	DateStart     string             `json:"date_start"`
	DateEnd       string             `json:"date_end"`
	TradingDays   int                `json:"trading_days"`
	Window        int                `json:"window"`
	T0Total       int                `json:"t0_total"`
	T0Success     int                `json:"t0_success"`
	T0Fail        int                `json:"t0_fail"`
	T0Flat        int                `json:"t0_flat"`
	SuccessRate   float64            `json:"success_rate"`
	FailRate      float64            `json:"fail_rate"`
	FlatRate      float64            `json:"flat_rate"`
	FailCloseDown float64            `json:"fail_close_down_pct"`
	FailPullback  float64            `json:"fail_pullback_pct"`
	LastBar       map[string]lastRow  `json:"last_bar"`
	Patterns      []aggRow           `json:"patterns"`
}

type lastRow struct {
	Sample   int     `json:"sample"`
	Success  float64 `json:"success_pct"`
	Fail     float64 `json:"fail_pct"`
}

func main() {
	dateRange := flag.String("date-range", "", "inclusive YYYY-MM-DD:YYYY-MM-DD")
	window := flag.Int("window", 3, "pattern window (3 or 5)")
	minT0 := flag.Int("min-t0", 1, "min T0 samples per pattern")
	out := flag.String("out", "", "write JSON report path")
	flag.Parse()

	if *dateRange == "" {
		log.Fatal("--date-range required")
	}
	parts := strings.SplitN(*dateRange, ":", 2)
	if len(parts) != 2 {
		log.Fatal("invalid --date-range")
	}

	root, err := candlepattern.ResolveCacheRoot(".")
	if err != nil {
		log.Fatal(err)
	}

	dates, err := listGobDates(root, parts[0], parts[1])
	if err != nil {
		log.Fatal(err)
	}

	var all []candlepattern.Observation
	for _, d := range dates {
		cache, err := candlepattern.LoadDailyCache(root, d)
		if err != nil {
			log.Printf("skip %s: %v", d, err)
			continue
		}
		all = append(all, candlepattern.CollectObservations(cache, d, *window)...)
	}

	rep := buildReport(parts[0], parts[1], dates, *window, *minT0, all)

	enc := json.NewEncoder(os.Stdout)
	enc.SetIndent("", "  ")
	if err := enc.Encode(rep); err != nil {
		log.Fatal(err)
	}

	if *out != "" {
		f, err := os.Create(*out)
		if err != nil {
			log.Fatal(err)
		}
		defer f.Close()
		enc2 := json.NewEncoder(f)
		enc2.SetIndent("", "  ")
		if err := enc2.Encode(rep); err != nil {
			log.Fatal(err)
		}
		fmt.Fprintf(os.Stderr, "wrote %s\n", *out)
	}
}

func listGobDates(root, start, end string) ([]string, error) {
	dir := filepath.Join(root, "t0", "daily")
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, err
	}
	var dates []string
	for _, e := range entries {
		name := e.Name()
		if !strings.HasPrefix(name, "t0_daily_cache_") || !strings.HasSuffix(name, ".gob") {
			continue
		}
		d := strings.TrimSuffix(strings.TrimPrefix(name, "t0_daily_cache_"), ".gob")
		if d >= start && d <= end {
			dates = append(dates, d)
		}
	}
	sort.Strings(dates)
	if len(dates) == 0 {
		return nil, fmt.Errorf("no gob in range")
	}
	return dates, nil
}

func buildReport(start, end string, dates []string, window, minT0 int, obs []candlepattern.Observation) report {
	groups := map[string]*patternAcc{}
	lastBars := map[string]*lastBarAcc{}

	var t0Total, t0Success, t0Fail, t0Flat int
	var failCloseDown, failPullback int

	for _, o := range obs {
		if !o.InT0Subset {
			continue
		}
		t0Total++
		closeRet := o.T0PnL + o.T0Gap
		switch {
		case o.T0PnL >= 2.5:
			t0Success++
		case o.T0PnL < 0:
			t0Fail++
			if closeRet < 0 {
				failCloseDown++
			} else {
				failPullback++
			}
		default:
			t0Flat++
		}

		a := groups[o.Pattern]
		if a == nil {
			a = &patternAcc{}
			groups[o.Pattern] = a
		}
		a.t0PnLs = append(a.t0PnLs, o.T0PnL)

		last := lastBarType(o.Pattern)
		lb := lastBars[last]
		if lb == nil {
			lb = &lastBarAcc{}
			lastBars[last] = lb
		}
		lb.total++
		if o.T0PnL >= 2.5 {
			lb.wins++
		}
		if o.T0PnL < 0 {
			lb.fails++
		}
	}

	patterns := make([]aggRow, 0, len(groups))
	for p, a := range groups {
		if len(a.t0PnLs) < minT0 {
			continue
		}
		var wins, fails int
		for _, pnl := range a.t0PnLs {
			if pnl >= 2.5 {
				wins++
			}
			if pnl < 0 {
				fails++
			}
		}
		n := len(a.t0PnLs)
		patterns = append(patterns, aggRow{
			Pattern:  p,
			T0N:      n,
			AvgT0:    mean(a.t0PnLs),
			MedT0:    median(a.t0PnLs),
			WinRate:  pct(wins, n),
			FailRate: pct(fails, n),
		})
	}
	sort.Slice(patterns, func(i, j int) bool {
		if patterns[i].AvgT0 != patterns[j].AvgT0 {
			return patterns[i].AvgT0 > patterns[j].AvgT0
		}
		return patterns[i].T0N > patterns[j].T0N
	})

	lastBarOut := map[string]lastRow{}
	lastKeys := make([]string, 0, len(lastBars))
	for k := range lastBars {
		lastKeys = append(lastKeys, k)
	}
	sort.Strings(lastKeys)
	for _, k := range lastKeys {
		lb := lastBars[k]
		if lb.total == 0 {
			continue
		}
		lastBarOut[k] = lastRow{
			Sample:  lb.total,
			Success: pct(lb.wins, lb.total),
			Fail:    pct(lb.fails, lb.total),
		}
	}

	failDenom := failCloseDown + failPullback
	return report{
		DateStart:     start,
		DateEnd:       end,
		TradingDays:   len(dates),
		Window:        window,
		T0Total:       t0Total,
		T0Success:     t0Success,
		T0Fail:        t0Fail,
		T0Flat:        t0Flat,
		SuccessRate:   pct(t0Success, t0Total),
		FailRate:      pct(t0Fail, t0Total),
		FlatRate:      pct(t0Flat, t0Total),
		FailCloseDown: pct(failCloseDown, failDenom),
		FailPullback:  pct(failPullback, failDenom),
		LastBar:       lastBarOut,
		Patterns:      patterns,
	}
}

func lastBarType(pattern string) string {
	parts := strings.Split(pattern, "|")
	return parts[len(parts)-1]
}

func pct(n, total int) float64 {
	if total == 0 {
		return 0
	}
	return float64(n) / float64(total) * 100
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
