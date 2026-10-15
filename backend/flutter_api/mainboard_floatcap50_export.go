package flutter_api

import (
	"encoding/gob"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"

	"go-stock/backend/data"
)

const (
	mainboardFloatCap50MinBars    = 20
	mainboardFloatCap50KLineLimit = 800
)

// MainboardFloatCap50Gob is a standalone, reusable snapshot of current
// mainboard stocks with at least 50 Yi yuan float market cap and their daily
// bars since 2024. Daily is keyed by the six-digit stock code.
type MainboardFloatCap50Gob struct {
	SchemaVersion       int
	GeneratedAt         string
	MarketCapSnapshotAt string
	MarketCapSource     string
	StartDate           string
	EndDate             string
	StockPoolTotal      int
	EligibleStockCount  int
	Stocks              []MainboardFloatCap50Stock
	Daily               map[string][]MainboardFloatCap50Bar
	// SelectionPools preserves each T0 universe captured after cutover.
	SelectionPools map[string][]MainboardT0PoolStock
	// Per-symbol completion witness; a bar bearing today's date may still be intraday.
	ClosedDailyDates map[string]string
}

// MainboardT0PoolStock stores the exact market-cap-filtered pool used for a T0 day.
type MainboardT0PoolStock struct {
	Code        string
	Name        string
	MarketCapYi float64
}

// MainboardFloatCap50Stock keeps the snapshot metadata and explains missing
// or short K-line histories without silently dropping eligible stocks.
type MainboardFloatCap50Stock struct {
	Code             string
	Name             string
	FloatMarketCapYi float64
	KLineSource      string
	BarCount         int
	FirstDate        string
	LastDate         string
	Status           string
	Error            string
}

// MainboardFloatCap50Bar stores normalized daily prices, source-adapter volume,
// and directly reported turnover. THS volume is normalized to hands. AmountYi
// is in 100-million yuan.
type MainboardFloatCap50Bar struct {
	Date           string
	Open           float64
	Close          float64
	High           float64
	Low            float64
	Volume         float64
	VolumeUnit     string
	AmountYi       float64
	AmountSource   string
	AmountUnit     string
	AmountVerified bool
}

// MainboardFloatCap50ExportReport summarizes the one-off cache generation.
type MainboardFloatCap50ExportReport struct {
	OutputPath          string         `json:"output_path"`
	StockPoolTotal      int            `json:"stock_pool_total"`
	EligibleStockCount  int            `json:"eligible_stock_count"`
	StocksWithDailyBars int            `json:"stocks_with_daily_bars"`
	StocksUnder20Bars   int            `json:"stocks_under_20_bars"`
	StocksUnavailable   int            `json:"stocks_unavailable"`
	DailyBarCount       int            `json:"daily_bar_count"`
	KLineSourceCounts   map[string]int `json:"kline_source_counts"`
	StartDate           string         `json:"start_date"`
	EndDate             string         `json:"end_date"`
}

type mainboardFloatCap50Candidate struct {
	code             string
	name             string
	floatMarketCapYi float64
}

type mainboardFloatCap50Result struct {
	stock MainboardFloatCap50Stock
	bars  []MainboardFloatCap50Bar
}

// ExportMainboardFloatCap50DailyGob snapshots the current Sina float-cap
// universe once, then downloads daily bars using the application's normal
// THS-first, turnover-verified fallback chain.
func ExportMainboardFloatCap50DailyGob(outputPath, startDate, endDate string, workers int, onProgress func(done, total int, stock MainboardFloatCap50Stock)) (MainboardFloatCap50ExportReport, error) {
	var report MainboardFloatCap50ExportReport
	start, err := time.Parse("2006-01-02", startDate)
	if err != nil {
		return report, fmt.Errorf("invalid start date %q: %w", startDate, err)
	}
	end, err := time.Parse("2006-01-02", endDate)
	if err != nil {
		return report, fmt.Errorf("invalid end date %q: %w", endDate, err)
	}
	if start.After(end) {
		return report, fmt.Errorf("start date %s is after end date %s", startDate, endDate)
	}
	if workers < 1 {
		workers = 1
	}
	if workers > 10 {
		workers = 10
	}

	poolSnapshotAt := time.Now().In(time.FixedZone("Asia/Shanghai", 8*60*60))
	candidates, mainboardCount, err := fetchMainboardFloatCap50Candidates()
	if err != nil {
		return report, err
	}
	if len(candidates) == 0 {
		return report, fmt.Errorf("no mainboard stocks with float market cap >= 50 Yi yuan")
	}

	snapshot := MainboardFloatCap50Gob{
		SchemaVersion:       2,
		GeneratedAt:         time.Now().In(poolSnapshotAt.Location()).Format(time.RFC3339),
		MarketCapSnapshotAt: poolSnapshotAt.Format(time.RFC3339),
		MarketCapSource:     "Sina hs_a nmc (万元 converted to 亿元)",
		StartDate:           startDate,
		EndDate:             endDate,
		StockPoolTotal:      mainboardCount,
		EligibleStockCount:  len(candidates),
		Stocks:              make([]MainboardFloatCap50Stock, 0, len(candidates)),
		Daily:               make(map[string][]MainboardFloatCap50Bar, len(candidates)),
	}

	jobs := make(chan mainboardFloatCap50Candidate)
	results := make(chan mainboardFloatCap50Result, len(candidates))
	var wg sync.WaitGroup
	if workers > len(candidates) {
		workers = len(candidates)
	}
	for i := 0; i < workers; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for candidate := range jobs {
				results <- fetchMainboardFloatCap50Stock(candidate, startDate, endDate)
			}
		}()
	}
	go func() {
		for _, candidate := range candidates {
			jobs <- candidate
		}
		close(jobs)
		wg.Wait()
		close(results)
	}()

	sourceCounts := make(map[string]int)
	done := 0
	for result := range results {
		done++
		snapshot.Stocks = append(snapshot.Stocks, result.stock)
		if result.stock.KLineSource != "" {
			sourceCounts[result.stock.KLineSource]++
		}
		if result.stock.Status == "ok" {
			snapshot.Daily[result.stock.Code] = result.bars
		}
		if onProgress != nil {
			onProgress(done, len(candidates), result.stock)
		}
	}

	sort.Slice(snapshot.Stocks, func(i, j int) bool {
		return snapshot.Stocks[i].Code < snapshot.Stocks[j].Code
	})
	for _, stock := range snapshot.Stocks {
		switch stock.Status {
		case "ok":
			report.StocksWithDailyBars++
			report.DailyBarCount += stock.BarCount
		case "insufficient_bars":
			report.StocksUnder20Bars++
		default:
			report.StocksUnavailable++
		}
	}
	report.OutputPath = outputPath
	report.StockPoolTotal = mainboardCount
	report.EligibleStockCount = len(candidates)
	report.KLineSourceCounts = sourceCounts
	report.StartDate = startDate
	report.EndDate = endDate

	if err := writeMainboardFloatCap50Gob(outputPath, snapshot); err != nil {
		return report, err
	}
	return report, nil
}

func fetchMainboardFloatCap50Candidates() ([]mainboardFloatCap50Candidate, int, error) {
	candidates := make([]mainboardFloatCap50Candidate, 0, 2000)
	mainboardCount := 0
	shortPage, shortPageCount := 0, 0
	for page := 1; page <= t0StockPoolMaxPages; page++ {
		var items []sinaStockPoolItem
		var err error
		for attempt := 1; attempt <= t0StockPoolMaxAttempts; attempt++ {
			items, err = fetchStockPoolPageFromSina(page)
			if err == nil {
				break
			}
			if attempt < t0StockPoolMaxAttempts {
				time.Sleep(t0StockPoolRetryDelay * time.Duration(attempt))
			}
		}
		if err != nil {
			return nil, mainboardCount, fmt.Errorf("Sina stock-pool page %d failed after %d attempts: %w", page, t0StockPoolMaxAttempts, err)
		}
		if len(items) == 0 {
			if page == 1 {
				return nil, mainboardCount, fmt.Errorf("Sina mainboard stock pool is empty")
			}
			break
		}
		if shortPage > 0 {
			return nil, mainboardCount, fmt.Errorf("Sina stock-pool page %d returned only %d rows, but page %d still has data; pagination is incomplete", shortPage, shortPageCount, page)
		}

		for _, item := range items {
			if !(strings.HasPrefix(item.Code, "60") || strings.HasPrefix(item.Code, "00")) {
				continue
			}
			mainboardCount++
			// nmc is Sina's circulating market cap in ten-thousand yuan. Do not
			// substitute total market cap when the float-cap field is missing.
			floatCapYi := float64(item.Nmc) / 10000
			if floatCapYi < 50 || item.Nmc <= 0 {
				continue
			}
			candidates = append(candidates, mainboardFloatCap50Candidate{
				code:             item.Code,
				name:             item.Name,
				floatMarketCapYi: floatCapYi,
			})
		}

		if page == t0StockPoolMaxPages {
			return nil, mainboardCount, fmt.Errorf("Sina stock pool exceeded %d pages; completeness is unknown", t0StockPoolMaxPages)
		}
		if len(items) < t0StockPoolPageSize {
			shortPage, shortPageCount = page, len(items)
		}
	}

	sort.Slice(candidates, func(i, j int) bool { return candidates[i].code < candidates[j].code })
	return candidates, mainboardCount, nil
}

func fetchMainboardFloatCap50Stock(candidate mainboardFloatCap50Candidate, startDate, endDate string) mainboardFloatCap50Result {
	stock := MainboardFloatCap50Stock{
		Code:             candidate.code,
		Name:             candidate.name,
		FloatMarketCapYi: candidate.floatMarketCapYi,
		Status:           "unavailable",
	}
	bars, source, err := fetchMainboardFloatCap50Bars(candidate, mainboardFloatCap50KLineLimit, startDate, endDate)
	if err != nil {
		stock.Error = err.Error()
		return mainboardFloatCap50Result{stock: stock}
	}
	stock.KLineSource = source
	stock.BarCount = len(bars)
	if len(bars) > 0 {
		stock.FirstDate = bars[0].Date
		stock.LastDate = bars[len(bars)-1].Date
	}
	if len(bars) < mainboardFloatCap50MinBars {
		stock.Status = "insufficient_bars"
		stock.Error = fmt.Sprintf("got %d valid bars; need at least %d", len(bars), mainboardFloatCap50MinBars)
		return mainboardFloatCap50Result{stock: stock}
	}
	stock.Status = "ok"
	return mainboardFloatCap50Result{stock: stock, bars: bars}
}

func fetchMainboardFloatCap50Bars(candidate mainboardFloatCap50Candidate, limit int, startDate, endDate string) ([]MainboardFloatCap50Bar, string, error) {
	result := data.FetchDailyKLineWithTurnoverFallback(candidate.code, candidate.name, limit, endDate)
	if result == nil || result.Data == nil || len(*result.Data) == 0 {
		return nil, "", fmt.Errorf("all configured K-line sources returned no usable daily bars")
	}

	byDate := make(map[string]MainboardFloatCap50Bar, len(*result.Data))
	for _, row := range *result.Data {
		bar, ok := parseKLineToDailyBar(row, result.Source)
		if !ok || bar.Date < startDate || bar.Date > endDate || bar.Open <= 0 || bar.Close <= 0 || bar.High <= 0 || bar.Low <= 0 || bar.Volume < 0 {
			continue
		}
		if _, err := time.Parse("2006-01-02", bar.Date); err != nil {
			continue
		}
		bar.VolumeUnit = "source"
		if result.Source == "ths" {
			bar.VolumeUnit = "hands"
		}
		byDate[bar.Date] = bar
	}

	bars := make([]MainboardFloatCap50Bar, 0, len(byDate))
	for _, bar := range byDate {
		bars = append(bars, bar)
	}
	sort.Slice(bars, func(i, j int) bool { return bars[i].Date < bars[j].Date })
	if len(bars) == 0 {
		return nil, result.Source, fmt.Errorf("K-line sources returned no valid bars in %s..%s", startDate, endDate)
	}
	return bars, result.Source, nil
}

func writeMainboardFloatCap50Gob(path string, snapshot MainboardFloatCap50Gob) error {
	if strings.TrimSpace(path) == "" {
		return fmt.Errorf("output path is empty")
	}
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("create cache directory: %w", err)
	}
	temp, err := os.CreateTemp(dir, ".mainboard_floatcap50_*.tmp")
	if err != nil {
		return fmt.Errorf("create temporary Gob: %w", err)
	}
	defer os.Remove(temp.Name())
	if err := gob.NewEncoder(temp).Encode(snapshot); err != nil {
		_ = temp.Close()
		return fmt.Errorf("encode Gob: %w", err)
	}
	if err := temp.Sync(); err != nil {
		_ = temp.Close()
		return fmt.Errorf("sync Gob: %w", err)
	}
	if err := temp.Close(); err != nil {
		return fmt.Errorf("close Gob: %w", err)
	}
	if err := os.Rename(temp.Name(), path); err != nil {
		return fmt.Errorf("publish Gob: %w", err)
	}
	return nil
}
