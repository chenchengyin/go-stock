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

type bucketRow struct {
	Label      string  `json:"label"`
	N          int     `json:"n"`
	SuccessPct float64 `json:"success_pct"`
	FailPct    float64 `json:"fail_pct"`
	FlatPct    float64 `json:"flat_pct"`
	AvgPnL     float64 `json:"avg_pnl"`
	MedPnL     float64 `json:"med_pnl"`
}

type patternRow struct {
	Pattern string  `json:"pattern"`
	N       int     `json:"n"`
	WinPct  float64 `json:"win_pct"`
	FailPct float64 `json:"fail_pct"`
	AvgPnL  float64 `json:"avg_pnl"`
}

type deepReport struct {
	Range       string       `json:"range"`
	TradingDays int          `json:"trading_days"`
	T0Total     int          `json:"t0_total"`
	GapBuckets  []bucketRow  `json:"gap_buckets"`
	ZTCount3    []bucketRow  `json:"zt_count_in_3bar"`
	LastBarOnly []bucketRow  `json:"last_bar_only"`
	Monthly     []bucketRow  `json:"monthly"`
	Quarterly   []bucketRow  `json:"quarterly"`
	YearCompare []bucketRow  `json:"year_compare"`
	AmountYi    []bucketRow  `json:"amount_yi_bucket"`
	MarketCap   []bucketRow  `json:"market_cap_yi_bucket"`
	Momentum3   []bucketRow  `json:"momentum_3bar"`
	Window5Top  []patternRow `json:"window5_top_win"`
	Window5Note string       `json:"window5_note"`
}

type acc struct {
	pnls []float64
}

func main() {
	dateRange := flag.String("date-range", "2025-01-02:2026-08-22", "START:END")
	out := flag.String("out", "", "json path")
	flag.Parse()

	parts := strings.SplitN(*dateRange, ":", 2)
	root, err := candlepattern.ResolveCacheRoot(".")
	if err != nil {
		log.Fatal(err)
	}
	dates, err := listDates(root, parts[0], parts[1])
	if err != nil {
		log.Fatal(err)
	}

	gapB := map[string]*acc{}
	ztB := map[string]*acc{}
	lastB := map[string]*acc{}
	monthB := map[string]*acc{}
	quarterB := map[string]*acc{}
	yearB := map[string]*acc{}
	amtB := map[string]*acc{}
	capB := map[string]*acc{}
	momB := map[string]*acc{}
	w5groups := map[string]*acc{}

	metaByCode := map[string]candlepattern.StockMeta{}
	t0Total := 0

	for _, d := range dates {
		cache, err := candlepattern.LoadDailyCache(root, d)
		if err != nil {
			continue
		}
		for _, s := range cache.Stocks {
			metaByCode[s.ShortCode] = s
		}
		pool := candlepattern.FilterA2Pool(cache, d)

		for _, o := range candlepattern.CollectObservations(cache, d, 3) {
			if !o.InT0Subset {
				continue
			}
			t0Total++
			pnl := o.T0PnL
			gap := o.T0Gap
			addPnL(gapB, gapBucket(gap), pnl)
			addPnL(monthB, d[:7], pnl)
			addPnL(quarterB, quarterOf(d), pnl)
			addPnL(yearB, d[:4], pnl)

			parts := strings.Split(o.Pattern, "|")
			if len(parts) >= 3 {
				addPnL(lastB, parts[len(parts)-1], pnl)
				addPnL(ztB, fmt.Sprintf("%d根涨停", countZT(parts)), pnl)
				addPnL(momB, momentumKeyParts(parts), pnl)
			}

			if hist, ok := pool[o.ShortCode]; ok && len(hist) > 0 {
				addPnL(amtB, amountBucket(hist[len(hist)-1].AmountYi), pnl)
			}
			addPnL(capB, capBucket(metaByCode[o.ShortCode].MarketCapYi), pnl)
		}

		for _, o := range candlepattern.CollectObservations(cache, d, 5) {
			if !o.InT0Subset {
				continue
			}
			addPnL(w5groups, o.Pattern, o.T0PnL)
		}
	}

	rep := deepReport{
		Range:       *dateRange,
		TradingDays: len(dates),
		T0Total:     t0Total,
		GapBuckets:  rowsFrom(gapB, []string{"1.0-1.5%", "1.5-2.0%", "2.0-2.5%", "2.5-3.0%"}),
		LastBarOnly: rowsFrom(lastB, sortedKeys(lastB)),
		Monthly:     rowsFrom(monthB, sortedKeys(monthB)),
		Quarterly:   rowsFrom(quarterB, sortedKeys(quarterB)),
		YearCompare: rowsFrom(yearB, []string{"2025", "2026"}),
		AmountYi:    rowsFrom(amtB, []string{"5-10亿", "10-20亿", "20-50亿", "50亿+"}),
		MarketCap:   rowsFrom(capB, []string{"<100亿", "100-300亿", "300-800亿", "800亿+", "未知"}),
		Momentum3:   rowsFrom(momB, []string{"三连阳", "两阳一阴", "一阳两阴", "三连阴", "其他"}),
		Window5Top:  topPatterns(w5groups, 10, 15),
		Window5Note: "5根组合仅列 T0 N≥15 且达标率 Top；组合空间 12^5，多数形态样本稀疏",
	}
	rep.ZTCount3 = rowsFrom(ztB, []string{"0根涨停", "1根涨停", "2根涨停", "3根涨停"})

	enc := json.NewEncoder(os.Stdout)
	enc.SetIndent("", "  ")
	_ = enc.Encode(rep)
	if *out != "" {
		f, err := os.Create(*out)
		if err != nil {
			log.Fatal(err)
		}
		defer f.Close()
		enc2 := json.NewEncoder(f)
		enc2.SetIndent("", "  ")
		_ = enc2.Encode(rep)
	}
}

func listDates(root, start, end string) ([]string, error) {
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
	return dates, nil
}

func gapBucket(g float64) string {
	switch {
	case g < 1.5:
		return "1.0-1.5%"
	case g < 2.0:
		return "1.5-2.0%"
	case g < 2.5:
		return "2.0-2.5%"
	default:
		return "2.5-3.0%"
	}
}

func amountBucket(yi float64) string {
	switch {
	case yi < 10:
		return "5-10亿"
	case yi < 20:
		return "10-20亿"
	case yi < 50:
		return "20-50亿"
	default:
		return "50亿+"
	}
}

func capBucket(yi float64) string {
	if yi <= 0 {
		return "未知"
	}
	switch {
	case yi < 100:
		return "<100亿"
	case yi < 300:
		return "100-300亿"
	case yi < 800:
		return "300-800亿"
	default:
		return "800亿+"
	}
}

func quarterOf(d string) string {
	m := d[5:7]
	switch m {
	case "01", "02", "03":
		return d[:4] + "-Q1"
	case "04", "05", "06":
		return d[:4] + "-Q2"
	case "07", "08", "09":
		return d[:4] + "-Q3"
	default:
		return d[:4] + "-Q4"
	}
}

func countZT(parts []string) int {
	n := 0
	for _, p := range parts {
		if p == "ZT" {
			n++
		}
	}
	return n
}

func momentumKeyParts(parts []string) string {
	yang := 0
	for _, p := range parts {
		switch p {
		case "SY", "MY", "DY", "ZT", "YX", "PB":
			yang++
		}
	}
	switch yang {
	case 3:
		return "三连阳"
	case 2:
		return "两阳一阴"
	case 1:
		return "一阳两阴"
	case 0:
		return "三连阴"
	default:
		return "其他"
	}
}

func addPnL(m map[string]*acc, key string, pnl float64) {
	if key == "" {
		return
	}
	a := m[key]
	if a == nil {
		a = &acc{}
		m[key] = a
	}
	a.pnls = append(a.pnls, pnl)
}

func rowsFrom(m map[string]*acc, order []string) []bucketRow {
	if order == nil {
		order = sortedKeys(m)
	}
	out := make([]bucketRow, 0, len(order))
	for _, k := range order {
		a := m[k]
		if a == nil || len(a.pnls) == 0 {
			continue
		}
		out = append(out, bucketRowFrom(k, a.pnls))
	}
	return out
}

func bucketRowFrom(label string, pnls []float64) bucketRow {
	var wins, fails int
	for _, p := range pnls {
		switch {
		case p >= 2.5:
			wins++
		case p < 0:
			fails++
		}
	}
	n := len(pnls)
	cp := append([]float64(nil), pnls...)
	sort.Float64s(cp)
	med := cp[n/2]
	if n%2 == 0 {
		med = (cp[n/2-1] + cp[n/2]) / 2
	}
	var sum float64
	for _, p := range pnls {
		sum += p
	}
	return bucketRow{
		Label:      label,
		N:          n,
		SuccessPct: float64(wins) / float64(n) * 100,
		FailPct:    float64(fails) / float64(n) * 100,
		FlatPct:    float64(n-wins-fails) / float64(n) * 100,
		AvgPnL:     sum / float64(n),
		MedPnL:     med,
	}
}

func topPatterns(m map[string]*acc, topN, minN int) []patternRow {
	type item struct {
		row patternRow
	}
	items := make([]item, 0)
	for p, a := range m {
		if len(a.pnls) < minN {
			continue
		}
		br := bucketRowFrom(p, a.pnls)
		items = append(items, item{patternRow{Pattern: p, N: br.N, WinPct: br.SuccessPct, FailPct: br.FailPct, AvgPnL: br.AvgPnL}})
	}
	sort.Slice(items, func(i, j int) bool {
		if items[i].row.WinPct != items[j].row.WinPct {
			return items[i].row.WinPct > items[j].row.WinPct
		}
		return items[i].row.N > items[j].row.N
	})
	if topN > len(items) {
		topN = len(items)
	}
	out := make([]patternRow, topN)
	for i := 0; i < topN; i++ {
		out[i] = items[i].row
	}
	return out
}

func sortedKeys(m map[string]*acc) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}
