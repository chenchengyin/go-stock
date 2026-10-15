package flutter_api

import (
	"fmt"
	"sort"
	"sync"
	"time"

	"go-stock/backend/logger"
)

const (
	mainboardSharedGobRefreshStartMinute = 15*60 + 10
	mainboardSharedGobRefreshRetry       = 30 * time.Minute
	mainboardSharedGobRefreshMaxAttempts = 4
	mainboardSharedGobRefreshProbeStocks = 10
	mainboardSharedGobRefreshWorkers     = 6
	mainboardSharedGobRefreshOverlapBars = 60
)

var mainboardSharedGobRefreshState struct {
	sync.Mutex
	running       bool
	attemptDate   string
	attemptCount  int
	lastAttemptAt time.Time
}

type mainboardSharedGobRefreshResult struct {
	index  int
	bars   []MainboardFloatCap50Bar
	source string
	err    error
}

// runMainboardSharedGobRefreshTick refreshes the fixed export-time pool after
// the close. A date probe prevents full-pool downloads on exchange holidays.
func runMainboardSharedGobRefreshTick(now time.Time) {
	local := now.In(chinaLocation())
	today := local.Format("2006-01-02")
	minute := local.Hour()*60 + local.Minute()
	if isT0WeekendDate(today) || minute < mainboardSharedGobRefreshStartMinute {
		return
	}

	mainboardSharedGobRefreshState.Lock()
	if mainboardSharedGobRefreshState.running {
		mainboardSharedGobRefreshState.Unlock()
		return
	}
	if mainboardSharedGobRefreshState.attemptDate != today {
		mainboardSharedGobRefreshState.attemptDate = today
		mainboardSharedGobRefreshState.attemptCount = 0
	}
	if mainboardSharedGobRefreshState.attemptCount >= mainboardSharedGobRefreshMaxAttempts ||
		(!mainboardSharedGobRefreshState.lastAttemptAt.IsZero() && local.Sub(mainboardSharedGobRefreshState.lastAttemptAt) < mainboardSharedGobRefreshRetry) {
		mainboardSharedGobRefreshState.Unlock()
		return
	}
	mainboardSharedGobRefreshState.running = true
	mainboardSharedGobRefreshState.attemptCount++
	mainboardSharedGobRefreshState.lastAttemptAt = local
	mainboardSharedGobRefreshState.Unlock()
	defer func() {
		mainboardSharedGobRefreshState.Lock()
		mainboardSharedGobRefreshState.running = false
		mainboardSharedGobRefreshState.Unlock()
	}()

	mainboardSharedGobUpdate.Lock()
	defer mainboardSharedGobUpdate.Unlock()

	current, err := loadMainboardSharedGob()
	if err != nil {
		logger.SugaredLogger.Warnf("[共享GOB] 加载日线失败，稍后重试: %v", err)
		return
	}
	pending := mainboardSharedGobPendingIndices(current, today)
	selectionPoolPending := mainboardSharedGobPendingSelectionPool(current, today)
	if len(pending) == 0 && len(selectionPoolPending) == 0 {
		return
	}
	if len(selectionPoolPending) == 0 && !mainboardSharedGobSnapshotHasDate(current, today) &&
		!mainboardSharedGobProviderHasDate(current, today) {
		return
	}

	updated, missing, err := refreshMainboardSharedGob(current, today, local, pending)
	if err != nil {
		logger.SugaredLogger.Warnf("[共享GOB] %s 日线增量刷新失败: %v", today, err)
		return
	}
	logger.SugaredLogger.Infof("[共享GOB] %s 日线增量刷新完成: refreshed_stocks=%d missing_current_bar=%d",
		today, updated, missing)
}

func mainboardSharedGobProviderHasDate(current *mainboardSharedGobData, date string) bool {
	limit := len(current.Stocks)
	if limit > mainboardSharedGobRefreshProbeStocks {
		limit = mainboardSharedGobRefreshProbeStocks
	}
	for i := 0; i < limit; i++ {
		index := 0
		if limit > 1 {
			index = i * (len(current.Stocks) - 1) / (limit - 1)
		}
		stock := current.Stocks[index]
		candidate := mainboardFloatCap50Candidate{
			code:             stock.ShortCode,
			name:             stock.Name,
			floatMarketCapYi: stock.MarketCapYi,
		}
		bars, _, err := fetchMainboardFloatCap50Bars(candidate, 10, date, date)
		if err == nil && len(bars) > 0 {
			return true
		}
	}
	return false
}

func refreshMainboardSharedGob(current *mainboardSharedGobData, targetDate string, now time.Time, indices []int) (int, int, error) {
	next := current.Snapshot
	next.ClosedDailyDates = copySharedGobClosedDates(current.Snapshot.ClosedDailyDates)
	next.Stocks = append([]MainboardFloatCap50Stock(nil), current.Snapshot.Stocks...)
	next.Daily = make(map[string][]MainboardFloatCap50Bar, len(current.Daily))
	for code, bars := range current.Daily {
		next.Daily[code] = bars
	}

	jobs := make(chan int)
	results := make(chan mainboardSharedGobRefreshResult, len(indices))
	workers := mainboardSharedGobRefreshWorkers
	if workers > len(indices) {
		workers = len(indices)
	}
	var wg sync.WaitGroup
	for i := 0; i < workers; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for index := range jobs {
				stock := current.Snapshot.Stocks[index]
				candidate := mainboardFloatCap50Candidate{
					code: stock.Code, name: stock.Name, floatMarketCapYi: stock.FloatMarketCapYi,
				}
				latestDate := stock.LastDate
				if bars := current.Daily[stock.Code]; len(bars) > 0 {
					latestDate = bars[len(bars)-1].Date
				}
				limit := mainboardSharedGobRefreshFetchLimit(latestDate, targetDate)
				bars, source, err := fetchMainboardFloatCap50Bars(candidate, limit, next.StartDate, targetDate)
				results <- mainboardSharedGobRefreshResult{index: index, bars: bars, source: source, err: err}
			}
		}()
	}
	go func() {
		for _, i := range indices {
			jobs <- i
		}
		close(jobs)
		wg.Wait()
		close(results)
	}()

	updated, missing, targetBars := 0, 0, 0
	for result := range results {
		stock := &next.Stocks[result.index]
		if result.err != nil {
			stock.Error = result.err.Error()
			missing++
			continue
		}
		oldBars := next.Daily[stock.Code]
		stockHasTargetBar := containsSharedGobBarDate(result.bars, targetDate)
		if stockHasTargetBar {
			targetBars++
		} else {
			missing++
		}
		merged := mergeMainboardSharedGobBars(oldBars, result.bars)
		if len(merged) == 0 {
			stock.Error = "no valid daily bars after refresh"
			if stockHasTargetBar {
				missing++
			}
			continue
		}
		next.Daily[stock.Code] = merged
		if stockHasTargetBar {
			next.ClosedDailyDates[stock.Code] = targetDate
		}
		stock.KLineSource = result.source
		stock.BarCount = len(merged)
		stock.FirstDate = merged[0].Date
		stock.LastDate = merged[len(merged)-1].Date
		if len(merged) < mainboardFloatCap50MinBars {
			stock.Status = "insufficient_bars"
			stock.Error = fmt.Sprintf("got %d valid bars; need at least %d", len(merged), mainboardFloatCap50MinBars)
		} else {
			stock.Status = "ok"
			stock.Error = ""
		}
		updated++
	}
	for _, stock := range mainboardSharedGobPendingSelectionPool(current, targetDate) {
		candidate := mainboardFloatCap50Candidate{
			code: stock.Code, name: stock.Name, floatMarketCapYi: stock.MarketCapYi,
		}
		latestDate := ""
		if bars := next.Daily[stock.Code]; len(bars) > 0 {
			latestDate = bars[len(bars)-1].Date
		}
		limit := mainboardSharedGobRefreshFetchLimit(latestDate, targetDate)
		bars, _, err := fetchMainboardFloatCap50Bars(candidate, limit, next.StartDate, targetDate)
		if err != nil {
			missing++
			continue
		}
		if containsSharedGobBarDate(bars, targetDate) {
			targetBars++
			next.ClosedDailyDates[stock.Code] = targetDate
		} else {
			missing++
		}
		next.Daily[stock.Code] = mergeMainboardSharedGobBars(next.Daily[stock.Code], bars)
		updated++
	}
	if targetBars == 0 && !mainboardSharedGobSnapshotHasDate(current, targetDate) {
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

func mainboardSharedGobPendingIndices(current *mainboardSharedGobData, targetDate string) []int {
	indices := make([]int, 0)
	for i, stock := range current.Snapshot.Stocks {
		if !containsSharedGobBarDate(current.Daily[stock.Code], targetDate) || current.Snapshot.ClosedDailyDates[stock.Code] < targetDate {
			indices = append(indices, i)
		}
	}
	return indices
}

func mainboardSharedGobPendingSelectionPool(current *mainboardSharedGobData, targetDate string) []MainboardT0PoolStock {
	baseCodes := make(map[string]struct{}, len(current.Snapshot.Stocks))
	for _, stock := range current.Snapshot.Stocks {
		baseCodes[stock.Code] = struct{}{}
	}
	pending := make([]MainboardT0PoolStock, 0)
	for _, stock := range current.Snapshot.SelectionPools[targetDate] {
		if _, exists := baseCodes[stock.Code]; exists || (containsSharedGobBarDate(current.Daily[stock.Code], targetDate) && current.Snapshot.ClosedDailyDates[stock.Code] >= targetDate) {
			continue
		}
		pending = append(pending, stock)
	}
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
