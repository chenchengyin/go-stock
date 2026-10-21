package flutter_api

import (
	"fmt"
	"sort"
	"strings"
	"sync"
	"time"

	"go-stock/backend/logger"
)

const (
	mainboardSharedGobRefreshRetry       = 30 * time.Minute
	mainboardSharedGobRefreshWorkers     = 6
	mainboardSharedGobRefreshOverlapBars = 60
)

var mainboardSharedGobRefreshState struct {
	sync.Mutex
	running       bool
	attemptDate   string
	lastAttemptAt time.Time
}

var mainboardSharedGobBarsFetcher = fetchMainboardFloatCap50Bars
var mainboardSharedGobClosingQuoteFetcher = func(stocks []t0Stock, targetDate string) map[string]t0Realtime {
	day, err := time.ParseInLocation("2006-01-02", targetDate, chinaLocation())
	if err != nil {
		return nil
	}
	return fetchT0RealtimeTencentAt(stocks, day.Add(24*time.Hour-time.Second))
}

type mainboardSharedGobRefreshResult struct {
	code   string
	bars   []MainboardFloatCap50Bar
	source string
	err    error
}

// Refresh all stored symbols overnight, using a fresh market witness for the
// latest completed trading session. There is no after-close GOB refresh.
func runMainboardSharedGobRefreshTick(now time.Time) {
	local := now.In(chinaLocation())
	if local.Hour()*60+local.Minute() >= t0AuctionCutoffHM {
		return
	}
	today := local.Format("2006-01-02")
	mainboardSharedGobRefreshState.Lock()
	if mainboardSharedGobRefreshState.running {
		mainboardSharedGobRefreshState.Unlock()
		return
	}
	if mainboardSharedGobRefreshState.attemptDate != today {
		mainboardSharedGobRefreshState.attemptDate = today
		mainboardSharedGobRefreshState.lastAttemptAt = time.Time{}
	}
	if !mainboardSharedGobRefreshState.lastAttemptAt.IsZero() && local.Sub(mainboardSharedGobRefreshState.lastAttemptAt) < mainboardSharedGobRefreshRetry {
		mainboardSharedGobRefreshState.Unlock()
		return
	}
	mainboardSharedGobRefreshState.running = true
	mainboardSharedGobRefreshState.Unlock()
	defer func() {
		mainboardSharedGobRefreshState.Lock()
		mainboardSharedGobRefreshState.running = false
		mainboardSharedGobRefreshState.Unlock()
	}()

	target, err := sharedGobRequiredPriorDate(today)
	if err != nil {
		logger.SugaredLogger.Warnf("[共享GOB] 凌晨无法确认最近交易日，稍后重试: %v", err)
		return
	}
	mainboardSharedGobUpdate.Lock()
	defer mainboardSharedGobUpdate.Unlock()
	current, err := loadMainboardSharedGob()
	if err != nil {
		logger.SugaredLogger.Warnf("[共享GOB] 凌晨加载日线失败，稍后重试: %v", err)
		return
	}
	pending := mainboardSharedGobPendingIndices(current, target)
	if len(pending) == 0 && len(mainboardSharedGobPendingSelectionPool(current, target)) == 0 {
		return
	}
	mainboardSharedGobRefreshState.Lock()
	mainboardSharedGobRefreshState.lastAttemptAt = local
	mainboardSharedGobRefreshState.Unlock()
	updated, missing, err := refreshMainboardSharedGob(current, target, local, pending)
	if err != nil {
		logger.SugaredLogger.Warnf("[共享GOB] 凌晨补齐 %s 失败，稍后重试: %v", target, err)
		return
	}
	logger.SugaredLogger.Infof("[共享GOB] 凌晨补齐最近交易日 %s: refreshed_stocks=%d unresolved_stocks=%d", target, updated, missing)
}

func refreshMainboardSharedGob(current *mainboardSharedGobData, targetDate string, now time.Time, indices []int) (int, int, error) {
	next := current.Snapshot
	next.ClosedDailyDates = copySharedGobClosedDates(current.Snapshot.ClosedDailyDates)
	next.NonTradingDates = copySharedGobClosedDates(current.Snapshot.NonTradingDates)
	next.Stocks = append([]MainboardFloatCap50Stock(nil), current.Snapshot.Stocks...)
	next.Daily = make(map[string][]MainboardFloatCap50Bar, len(current.Daily))
	for code, bars := range current.Daily {
		next.Daily[code] = bars
	}
	baseIndex := make(map[string]int, len(next.Stocks))
	for i, stock := range next.Stocks {
		baseIndex[stock.Code] = i
	}
	candidates := mainboardSharedGobPendingSelectionPool(current, targetDate)
	for _, i := range indices {
		stock := current.Snapshot.Stocks[i]
		candidates = append(candidates, MainboardT0PoolStock{Code: stock.Code, Name: stock.Name, MarketCapYi: stock.FloatMarketCapYi})
	}
	quoteStocks := make([]t0Stock, 0, len(candidates))
	for _, stock := range candidates {
		prefix := "sz."
		if strings.HasPrefix(stock.Code, "60") {
			prefix = "sh."
		}
		quoteStocks = append(quoteStocks, t0Stock{Code: prefix + stock.Code, ShortCode: stock.Code, Name: stock.Name})
	}
	closingQuotes := mainboardSharedGobClosingQuoteFetcher(quoteStocks, targetDate)
	pending := make([]MainboardT0PoolStock, 0, len(candidates))
	for _, stock := range candidates {
		if isConfirmedSharedGobPriorSessionSuspension(closingQuotes[stock.Code], current.Daily[stock.Code], now.In(chinaLocation()).Format("2006-01-02"), targetDate) {
			next.NonTradingDates[stock.Code] = targetDate
		} else {
			pending = append(pending, stock)
		}
	}
	jobs := make(chan MainboardT0PoolStock)
	results := make(chan mainboardSharedGobRefreshResult, len(pending))
	workers := mainboardSharedGobRefreshWorkers
	if workers > len(pending) {
		workers = len(pending)
	}
	var wg sync.WaitGroup
	for i := 0; i < workers; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for stock := range jobs {
				latest := ""
				if bars := current.Daily[stock.Code]; len(bars) > 0 {
					latest = bars[len(bars)-1].Date
				}
				bars, source, err := mainboardSharedGobBarsFetcher(mainboardFloatCap50Candidate{code: stock.Code, name: stock.Name, floatMarketCapYi: stock.MarketCapYi}, mainboardSharedGobRefreshFetchLimit(latest, targetDate), next.StartDate, targetDate)
				results <- mainboardSharedGobRefreshResult{code: stock.Code, bars: bars, source: source, err: err}
			}
		}()
	}
	go func() {
		for _, stock := range pending {
			jobs <- stock
		}
		close(jobs)
		wg.Wait()
		close(results)
	}()
	updated, missing, targetBars := 0, 0, 0
	for result := range results {
		index, isBase := baseIndex[result.code]
		if result.err != nil {
			missing++
			if isBase {
				next.Stocks[index].Error = result.err.Error()
			}
			continue
		}
		hasTarget := containsSharedGobBarDate(result.bars, targetDate)
		if hasTarget {
			targetBars++
			next.ClosedDailyDates[result.code] = targetDate
			delete(next.NonTradingDates, result.code)
		} else {
			missing++
		}
		merged := mergeMainboardSharedGobBars(current.Daily[result.code], result.bars)
		if len(merged) == 0 {
			if hasTarget {
				missing++
			}
			continue
		}
		next.Daily[result.code] = merged
		if isBase {
			stock := &next.Stocks[index]
			stock.KLineSource = result.source
			stock.BarCount = len(merged)
			stock.FirstDate = merged[0].Date
			stock.LastDate = merged[len(merged)-1].Date
			stock.Status = "ok"
			stock.Error = ""
			if len(merged) < mainboardFloatCap50MinBars {
				stock.Status = "insufficient_bars"
				stock.Error = fmt.Sprintf("got %d valid bars; need at least %d", len(merged), mainboardFloatCap50MinBars)
			}
		}
		updated++
	}
	if targetBars == 0 && !mainboardSharedGobSnapshotHasDate(current, targetDate) && missing > 0 {
		return 0, missing, fmt.Errorf("no stock returned a valid %s bar; keeping the previous snapshot", targetDate)
	}
	next.GeneratedAt = now.Format(time.RFC3339)
	dates := mainboardSharedGobDates(next.Daily)
	if len(dates) == 0 {
		return 0, missing, fmt.Errorf("refreshed snapshot has no dates")
	}
	next.EndDate = dates[len(dates)-1]
	loaded, err := buildMainboardSharedGobData(next)
	if err != nil {
		return 0, missing, err
	}
	if err := writeMainboardFloatCap50Gob(mainboardSharedGobPath(), next); err != nil {
		return 0, missing, fmt.Errorf("publish refreshed GOB: %w", err)
	}
	mainboardSharedGobMu.Lock()
	mainboardSharedGob = loaded
	mainboardSharedGobMu.Unlock()
	return updated, missing, nil
}

func sharedGobVerifiedNoTradingDate(current *mainboardSharedGobData, code, targetDate string) bool {
	return current.Snapshot.NonTradingDates[code] == targetDate && !containsSharedGobBarDate(current.Daily[code], targetDate)
}

func mainboardSharedGobPendingIndices(current *mainboardSharedGobData, targetDate string) []int {
	indices := make([]int, 0)
	for i, stock := range current.Snapshot.Stocks {
		if !sharedGobVerifiedNoTradingDate(current, stock.Code, targetDate) && (!containsSharedGobBarDate(current.Daily[stock.Code], targetDate) || current.Snapshot.ClosedDailyDates[stock.Code] < targetDate) {
			indices = append(indices, i)
		}
	}
	return indices
}

// Include Daily-only symbols and every stored dated pool, even when the
// symbol is outside today's market-cap filter or has no bars yet.
func mainboardSharedGobPendingSelectionPool(current *mainboardSharedGobData, targetDate string) []MainboardT0PoolStock {
	baseCodes := make(map[string]bool, len(current.Snapshot.Stocks))
	for _, stock := range current.Snapshot.Stocks {
		baseCodes[stock.Code] = true
	}
	all := make(map[string]MainboardT0PoolStock, len(current.Daily))
	dates := make([]string, 0, len(current.Snapshot.SelectionPools))
	for date := range current.Snapshot.SelectionPools {
		dates = append(dates, date)
	}
	sort.Sort(sort.Reverse(sort.StringSlice(dates)))
	for _, date := range dates {
		for _, stock := range current.Snapshot.SelectionPools[date] {
			if _, exists := all[stock.Code]; !exists {
				all[stock.Code] = stock
			}
		}
	}
	for code := range current.Daily {
		if _, exists := all[code]; !exists {
			all[code] = MainboardT0PoolStock{Code: code}
		}
	}
	pending := make([]MainboardT0PoolStock, 0)
	for code, stock := range all {
		if baseCodes[code] || sharedGobVerifiedNoTradingDate(current, code, targetDate) || (containsSharedGobBarDate(current.Daily[code], targetDate) && current.Snapshot.ClosedDailyDates[code] >= targetDate) {
			continue
		}
		pending = append(pending, stock)
	}
	sort.Slice(pending, func(i, j int) bool { return pending[i].Code < pending[j].Code })
	return pending
}

func mainboardSharedGobSnapshotHasDate(current *mainboardSharedGobData, targetDate string) bool {
	index := sort.SearchStrings(current.Dates, targetDate)
	return index < len(current.Dates) && current.Dates[index] == targetDate
}

func mainboardSharedGobRefreshFetchLimit(lastDate, targetDate string) int {
	if lastDate == "" {
		return mainboardFloatCap50KLineLimit
	}
	last, lastErr := time.Parse("2006-01-02", lastDate)
	target, targetErr := time.Parse("2006-01-02", targetDate)
	if lastErr != nil || targetErr != nil || !target.After(last) {
		return mainboardSharedGobRefreshOverlapBars
	}
	calendarGap := int(target.Sub(last).Hours()/24) + mainboardSharedGobRefreshOverlapBars
	if calendarGap < mainboardSharedGobRefreshOverlapBars {
		calendarGap = mainboardSharedGobRefreshOverlapBars
	}
	if calendarGap > mainboardFloatCap50KLineLimit {
		calendarGap = mainboardFloatCap50KLineLimit
	}
	return calendarGap
}

func mergeMainboardSharedGobBars(existing, fresh []MainboardFloatCap50Bar) []MainboardFloatCap50Bar {
	byDate := make(map[string]MainboardFloatCap50Bar, len(existing)+len(fresh))
	for _, bar := range existing {
		byDate[bar.Date] = bar
	}
	for _, bar := range fresh {
		byDate[bar.Date] = bar
	}
	merged := make([]MainboardFloatCap50Bar, 0, len(byDate))
	for _, bar := range byDate {
		merged = append(merged, bar)
	}
	sort.Slice(merged, func(i, j int) bool { return merged[i].Date < merged[j].Date })
	return merged
}

func containsSharedGobBarDate(bars []MainboardFloatCap50Bar, date string) bool {
	i := sort.Search(len(bars), func(i int) bool { return bars[i].Date >= date })
	return i < len(bars) && bars[i].Date == date
}

func mainboardSharedGobDates(daily map[string][]MainboardFloatCap50Bar) []string {
	dateSet := make(map[string]struct{})
	for _, bars := range daily {
		for _, bar := range bars {
			if bar.Date != "" {
				dateSet[bar.Date] = struct{}{}
			}
		}
	}
	dates := make([]string, 0, len(dateSet))
	for date := range dateSet {
		dates = append(dates, date)
	}
	sort.Strings(dates)
	return dates
}
