package candlepattern

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
)

func PatternOutputDir(cacheRoot string) string {
	return filepath.Join(cacheRoot, "t0", "pattern")
}

func WritePatternStats(cacheRoot, tradeDate string, stats []PatternStat, window int) (string, error) {
	dir := PatternOutputDir(cacheRoot)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return "", err
	}
	name := fmt.Sprintf("pattern_stats_%dbar_%s.json", window, tradeDate)
	path := filepath.Join(dir, name)
	f, err := os.Create(path)
	if err != nil {
		return "", err
	}
	defer f.Close()
	enc := json.NewEncoder(f)
	enc.SetIndent("", "  ")
	if err := enc.Encode(stats); err != nil {
		return "", err
	}
	return path, nil
}

func WriteHourlyCrossStats(cacheRoot, tradeDate string, stats []HourlyCrossStat, window int) (string, error) {
	dir := PatternOutputDir(cacheRoot)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return "", err
	}
	name := fmt.Sprintf("hourly_cross_stats_3d_%dbar_%s.json", window, tradeDate)
	path := filepath.Join(dir, name)
	f, err := os.Create(path)
	if err != nil {
		return "", err
	}
	defer f.Close()
	enc := json.NewEncoder(f)
	enc.SetIndent("", "  ")
	if err := enc.Encode(stats); err != nil {
		return "", err
	}
	return path, nil
}

func PrintTopSummary(stats []PatternStat, topN, minSamples int) {
	eligible := make([]PatternStat, 0, len(stats))
	for _, s := range stats {
		if s.SampleCount >= minSamples {
			eligible = append(eligible, s)
		}
	}
	sort.Slice(eligible, func(i, j int) bool {
		if eligible[i].T0WinRate2p5 != eligible[j].T0WinRate2p5 {
			return eligible[i].T0WinRate2p5 > eligible[j].T0WinRate2p5
		}
		return eligible[i].SampleCount > eligible[j].SampleCount
	})
	if topN > len(eligible) {
		topN = len(eligible)
	}
	fmt.Printf("\n=== Top %d patterns (N>=%d, by T0 win rate >=2.5%%) ===\n", topN, minSamples)
	for i := 0; i < topN; i++ {
		s := eligible[i]
		fmt.Printf("%2d. %s  N=%d  yang=%.1f%%  t0N=%d  t0Win=%.1f%%  t0Avg=%.2f%%\n",
			i+1, s.Pattern, s.SampleCount, s.NextYangRate*100,
			s.T0SubsetCount, s.T0WinRate2p5*100, s.T0AvgPnL)
	}
}

func PrintHourlyTopSummary(stats []HourlyCrossStat, topN, minSamples int) {
	eligible := make([]HourlyCrossStat, 0, len(stats))
	for _, s := range stats {
		if s.SampleCount >= minSamples {
			eligible = append(eligible, s)
		}
	}
	sort.Slice(eligible, func(i, j int) bool {
		if eligible[i].T0WinRate2p5 != eligible[j].T0WinRate2p5 {
			return eligible[i].T0WinRate2p5 > eligible[j].T0WinRate2p5
		}
		return eligible[i].SampleCount > eligible[j].SampleCount
	})
	if topN > len(eligible) {
		topN = len(eligible)
	}
	fmt.Printf("\n=== Top %d hourly cross patterns (N>=%d) ===\n", topN, minSamples)
	for i := 0; i < topN; i++ {
		s := eligible[i]
		fmt.Printf("%2d. %s + %s  N=%d  t0Win=%.1f%%\n",
			i+1, s.DailyPattern, s.HourlyPattern, s.SampleCount, s.T0WinRate2p5*100)
	}
}
