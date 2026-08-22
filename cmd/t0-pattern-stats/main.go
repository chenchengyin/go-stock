package main

import (
	"flag"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"go-stock/backend/analysis/candlepattern"
)

func main() {
	date := flag.String("date", "", "trade date YYYY-MM-DD")
	dateRange := flag.String("date-range", "", "inclusive range YYYY-MM-DD:YYYY-MM-DD")
	window := flag.String("window", "all", "3, 5, or all")
	minSamples := flag.Int("min-samples", 30, "minimum sample count for top list")
	topN := flag.Int("top", 20, "console top N")
	hourly := flag.Bool("hourly", false, "enable 3-day hourly cross stats")
	hourlyDays := flag.Int("hourly-days", 3, "hourly lookback trading days (only 3 supported)")
	hourlyTail := flag.String("hourly-tail", "2,3", "comma-separated hourly tail lengths")
	flag.Parse()

	if *hourlyDays != 3 {
		log.Fatal("--hourly-days only supports 3 in this version")
	}

	dates, err := resolveDates(*date, *dateRange)
	if err != nil {
		log.Fatal(err)
	}

	windows, err := resolveWindows(*window)
	if err != nil {
		log.Fatal(err)
	}

	tailNs, err := parseIntList(*hourlyTail)
	if err != nil {
		log.Fatal(err)
	}

	root, err := candlepattern.ResolveCacheRoot(".")
	if err != nil {
		log.Fatal(err)
	}

	var fetcher candlepattern.HourlyFetcher
	if *hourly {
		fetcher = candlepattern.NewEastMoneyHourlyFetcher()
	}

	for _, tradeDate := range dates {
		if err := runDate(root, tradeDate, windows, *minSamples, *topN, *hourly, tailNs, fetcher); err != nil {
			log.Printf("[%s] skip: %v", tradeDate, err)
		}
	}
}

func resolveDates(date, dateRange string) ([]string, error) {
	if dateRange != "" {
		parts := strings.SplitN(dateRange, ":", 2)
		if len(parts) != 2 {
			return nil, fmt.Errorf("invalid --date-range, want START:END")
		}
		return listGobDates(parts[0], parts[1])
	}
	if date == "" {
		return nil, fmt.Errorf("--date or --date-range required")
	}
	return []string{date}, nil
}

func listGobDates(start, end string) ([]string, error) {
	root, err := candlepattern.ResolveCacheRoot(".")
	if err != nil {
		return nil, err
	}
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
	if len(dates) == 0 {
		return nil, fmt.Errorf("no gob files in range %s:%s", start, end)
	}
	return dates, nil
}

func resolveWindows(window string) ([]int, error) {
	switch window {
	case "3":
		return []int{3}, nil
	case "5":
		return []int{5}, nil
	case "all":
		return []int{3, 5}, nil
	default:
		return nil, fmt.Errorf("invalid --window %q", window)
	}
}

func parseIntList(s string) ([]int, error) {
	parts := strings.Split(s, ",")
	out := make([]int, 0, len(parts))
	for _, p := range parts {
		p = strings.TrimSpace(p)
		if p == "" {
			continue
		}
		n, err := strconv.Atoi(p)
		if err != nil {
			return nil, fmt.Errorf("invalid int %q", p)
		}
		out = append(out, n)
	}
	return out, nil
}

func runDate(
	root, tradeDate string,
	windows []int,
	minSamples, topN int,
	hourly bool,
	tailNs []int,
	fetcher candlepattern.HourlyFetcher,
) error {
	cache, err := candlepattern.LoadDailyCache(root, tradeDate)
	if err != nil {
		return err
	}

	for _, w := range windows {
		obs := candlepattern.CollectObservations(cache, tradeDate, w)
		stats := candlepattern.AggregateStats(obs, w, minSamples)
		path, err := candlepattern.WritePatternStats(root, tradeDate, stats, w)
		if err != nil {
			return err
		}
		fmt.Fprintf(os.Stderr, "[%s] window=%d observations=%d patterns=%d -> %s\n",
			tradeDate, w, len(obs), len(stats), path)
		candlepattern.PrintTopSummary(stats, topN, minSamples)
	}

	if hourly {
		for _, w := range windows {
			cross := candlepattern.CollectHourlyCrossStats(cache, root, tradeDate, w, tailNs, fetcher)
			candlepattern.MarkInsufficientHourly(cross, minSamples)
			path, err := candlepattern.WriteHourlyCrossStats(root, tradeDate, cross, w)
			if err != nil {
				return err
			}
			fmt.Fprintf(os.Stderr, "[%s] hourly window=%d cross=%d -> %s\n",
				tradeDate, w, len(cross), path)
			candlepattern.PrintHourlyTopSummary(cross, topN, minSamples)
		}
	}
	return nil
}
