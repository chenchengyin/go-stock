package flutter_api

import (
	"encoding/gob"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"
)

const sharedGobRedStrategyModuleCode = "radar.red_shared_gob_strategy"

type mainboardSharedGobData struct {
	Snapshot       MainboardFloatCap50Gob
	Stocks         []t0Stock
	SelectionPools map[string][]t0Stock
	Daily          map[string][]dailyBar
	Dates          []string
	PoolErrors     map[string]string
}

var (
	mainboardSharedGobMu     sync.RWMutex
	mainboardSharedGobLoadMu sync.Mutex
	mainboardSharedGobUpdate sync.Mutex
	mainboardSharedGob       *mainboardSharedGobData
)

var sharedGobDailyFetcher = fetchAllDailyKLine
var sharedGobRedQuoteBuilder = buildSharedGobRedAuctionQuotes
var sharedGobRedLiveQuoteFetcher = fetchT0RealtimeTencent

func loadMainboardSharedGob() (*mainboardSharedGobData, error) {
	mainboardSharedGobMu.RLock()
	loaded := mainboardSharedGob
	mainboardSharedGobMu.RUnlock()
	if loaded != nil {
		return loaded, nil
	}

	mainboardSharedGobLoadMu.Lock()
	defer mainboardSharedGobLoadMu.Unlock()
	mainboardSharedGobMu.RLock()
	loaded = mainboardSharedGob
	mainboardSharedGobMu.RUnlock()
	if loaded != nil {
		return loaded, nil
	}

	path := mainboardSharedGobPath()
	file, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("打开共享日线GOB失败: %w", err)
	}
	defer file.Close()

	var snapshot MainboardFloatCap50Gob
	if err := gob.NewDecoder(file).Decode(&snapshot); err != nil {
		return nil, fmt.Errorf("读取共享日线GOB失败: %w", err)
	}
	poolErrors := restoreT0PoolArchives(&snapshot)
	loaded, err = buildMainboardSharedGobData(snapshot)
	if err != nil {
		return nil, err
	}
	loaded.PoolErrors = poolErrors
	mainboardSharedGobMu.Lock()
	mainboardSharedGob = loaded
	mainboardSharedGobMu.Unlock()
	return loaded, nil
}

func mainboardSharedGobPath() string {
	return filepath.Join(t0CacheRootPath, "mainboard_floatcap50_2024", "daily.gob")
}

func buildMainboardSharedGobData(snapshot MainboardFloatCap50Gob) (*mainboardSharedGobData, error) {
	if (snapshot.SchemaVersion != 1 && snapshot.SchemaVersion != 2) ||
		len(snapshot.Stocks) == 0 || len(snapshot.Daily) == 0 {
		return nil, fmt.Errorf("共享日线GOB格式或内容无效")
	}
	data := &mainboardSharedGobData{
		Snapshot:       snapshot,
		Stocks:         make([]t0Stock, 0, len(snapshot.Stocks)),
		SelectionPools: make(map[string][]t0Stock, len(snapshot.SelectionPools)),
		Daily:          snapshot.Daily,
	}
	dateSet := make(map[string]struct{})
	for _, bars := range data.Daily {
		for _, bar := range bars {
			if bar.Date != "" {
				dateSet[bar.Date] = struct{}{}
			}
		}
	}
	for date := range dateSet {
		data.Dates = append(data.Dates, date)
	}
	sort.Strings(data.Dates)
	if len(data.Dates) == 0 {
		return nil, fmt.Errorf("共享日线GOB没有可用交易日")
	}

	for _, stock := range snapshot.Stocks {
		code := strings.TrimSpace(stock.Code)
		if len(code) != 6 || stock.FloatMarketCapYi < 50 ||
			!(strings.HasPrefix(code, "60") || strings.HasPrefix(code, "00")) {
			continue
		}
		prefix := "sz."
		if strings.HasPrefix(code, "60") {
			prefix = "sh."
		}
		data.Stocks = append(data.Stocks, t0Stock{
			Code:        prefix + code,
			ShortCode:   code,
			Name:        stock.Name,
			MarketCapYi: stock.FloatMarketCapYi,
		})
	}
	if len(data.Stocks) == 0 {
		return nil, fmt.Errorf("共享日线GOB没有符合条件的主板股票")
	}
	for date, pool := range snapshot.SelectionPools {
		stocks := make([]t0Stock, 0, len(pool))
		for _, stock := range pool {
			code := strings.TrimSpace(stock.Code)
			if len(code) != 6 || !(strings.HasPrefix(code, "60") || strings.HasPrefix(code, "00")) {
				continue
			}
			prefix := "sz."
			if strings.HasPrefix(code, "60") {
				prefix = "sh."
			}
			stocks = append(stocks, t0Stock{
				Code: prefix + code, ShortCode: code, Name: stock.Name, MarketCapYi: stock.MarketCapYi,
			})
		}
		data.SelectionPools[date] = stocks
	}
	return data, nil
}

func sharedGobT0SelectionPool(shared *mainboardSharedGobData, tradeDate string) []t0Stock {
	if shared == nil {
		return nil
	}
	if shared.PoolErrors[tradeDate] != "" {
		return nil
	}
	stocks := shared.SelectionPools[tradeDate]
	if len(stocks) == 0 {
		if archive, err := readT0PoolArchive(tradeDate); err == nil {
			return archive.Stocks
		}
	}
	today := time.Now().In(chinaLocation()).Format("2006-01-02")
	if tradeDate == today {
		if cached, ok := readT0DailyCachePayloadFromDisk(tradeDate); ok &&
			cached.StockPoolComplete && len(cached.Stocks) > 0 && !sameT0StockPool(stocks, cached.Stocks) {
			return cached.Stocks
		}
	}
	return stocks
}

func loadOrFetchSharedGobT0Daily(tradeDate string) ([]t0Stock, map[string][]dailyBar, bool, error) {
	mainboardSharedGobUpdate.Lock()
	defer mainboardSharedGobUpdate.Unlock()

	shared, err := loadMainboardSharedGob()
	if err != nil {
		return nil, nil, false, err
	}
	if issue := shared.PoolErrors[tradeDate]; issue != "" {
		return nil, nil, false, fmt.Errorf("%s 股票池不可用: %s", tradeDate, issue)
	}
	snapshotStocks := shared.SelectionPools[tradeDate]
	stocks := sharedGobT0SelectionPool(shared, tradeDate)
	updated := len(snapshotStocks) == 0 || !sameT0StockPool(snapshotStocks, stocks)
	today := time.Now().In(chinaLocation()).Format("2006-01-02")
	if updated {
		if tradeDate != today && len(stocks) == 0 {
			return nil, nil, false, fmt.Errorf("共享GOB没有%s的选股池快照", tradeDate)
		}
		if len(stocks) == 0 {
			stocks, err = fetchStockPoolFromSinaStrict()
			if err != nil {
				return nil, nil, false, fmt.Errorf("股票池获取失败: %w", err)
			}
		}
	}
	if err := saveT0PoolArchive(tradeDate, stocks, "complete_selection_pool"); err != nil {
		return nil, nil, false, fmt.Errorf("独立保存股票池失败: %w", err)
	}

	endDate := t0DailyFetchEndDate(tradeDate)
	daily, missing := sharedGobDailyForStocks(shared, stocks, endDate, nil)
	priorDate := ""
	if tradeDate == today {
		priorDate, err = sharedGobRequiredPriorDate(tradeDate)
		if err != nil {
			return nil, nil, false, err
		}
		missingSet := make(map[string]bool, len(missing))
		for _, stock := range missing {
			missingSet[stock.ShortCode] = true
		}
		for _, stock := range stocks {
			if !missingSet[stock.ShortCode] && !sharedGobStockHistoryCurrent(shared, stock.ShortCode, daily[stock.ShortCode], tradeDate, priorDate) {
				missing = append(missing, stock)
			}
		}
	}
	fetchEndDate := endDate
	if priorDate != "" {
		fetchEndDate = priorDate
	}
	// Verify non-trading stocks before any daily download. Their absent bars
	// cannot be repaired, and must not delay the 09:25 opening snapshot.
	excluded := make(map[string]string)
	if priorDate != "" {
		for _, stock := range stocks {
			if sharedGobVerifiedNoTradingDate(shared, stock.ShortCode, priorDate) {
				excluded[stock.ShortCode] = "前一交易日无成交日线，连续形态不适用"
			}
		}
		for code, reason := range sharedGobRedExcludedStocks(tradeDate) {
			excluded[code] = reason
		}
		unchecked := make([]t0Stock, 0, len(missing))
		for _, stock := range missing {
			if excluded[stock.ShortCode] == "" {
				unchecked = append(unchecked, stock)
			}
		}
		if len(unchecked) > 0 {
			for code, reason := range sharedGobNonTradingHistoryVerifier(unchecked, daily, tradeDate, priorDate) {
				excluded[code] = reason
			}
		}
		repair := make([]t0Stock, 0, len(missing))
		for _, stock := range missing {
			if excluded[stock.ShortCode] == "" {
				repair = append(repair, stock)
			}
		}
		missing = repair
	}
	fallback := sharedGobDailyFetcher(missing, fetchEndDate, nil)
	freshClosed := make(map[string]bool, len(fallback))
	for code, bars := range fallback {
		freshClosed[code] = priorDate != "" && containsSharedGobBarDate(bars, priorDate)
		// Preserve the full historical GOB when fetching a short recent tail.
		bars = mergeMainboardSharedGobBars(shared.Daily[code], bars)
		fallback[code] = bars
		daily[code] = dailyBarsOnOrBefore(bars, endDate)
	}
	if len(daily) == 0 {
		return nil, nil, false, fmt.Errorf("共享日线GOB和缺失股票回退都没有可用日线")
	}

	if updated || len(fallback) > 0 {
		snapshot := shared.Snapshot
		snapshot.ClosedDailyDates = copySharedGobClosedDates(shared.Snapshot.ClosedDailyDates)
		snapshot.Daily = make(map[string][]MainboardFloatCap50Bar, len(shared.Daily)+len(fallback))
		for code, bars := range shared.Daily {
			snapshot.Daily[code] = bars
		}
		snapshot.SchemaVersion = 2
		snapshot.GeneratedAt = time.Now().In(chinaLocation()).Format(time.RFC3339)
		if updated {
			snapshot.MarketCapSnapshotAt = snapshot.GeneratedAt
			snapshot.MarketCapSource = "Sina hs_a nmc/mktcap (万元 converted to 亿元)"
			selectionPools := make(map[string][]MainboardT0PoolStock, len(shared.Snapshot.SelectionPools)+1)
			for date, pool := range shared.Snapshot.SelectionPools {
				selectionPools[date] = pool
			}
			snapshot.SelectionPools = selectionPools
			pool := make([]MainboardT0PoolStock, 0, len(stocks))
			for _, stock := range stocks {
				pool = append(pool, MainboardT0PoolStock{
					Code: stock.ShortCode, Name: stock.Name, MarketCapYi: stock.MarketCapYi,
				})
			}
			snapshot.SelectionPools[tradeDate] = pool
		}
		for code, bars := range fallback {
			snapshot.Daily[code] = mergeMainboardSharedGobBars(snapshot.Daily[code], bars)
			if freshClosed[code] && snapshot.ClosedDailyDates[code] < priorDate {
				snapshot.ClosedDailyDates[code] = priorDate
			}
		}
		if dates := mainboardSharedGobDates(snapshot.Daily); len(dates) > 0 {
			snapshot.EndDate = dates[len(dates)-1]
		}
		loaded, err := buildMainboardSharedGobData(snapshot)
		if err != nil {
			return nil, nil, false, err
		}
		loaded.PoolErrors = shared.PoolErrors
		if err := writeMainboardFloatCap50Gob(mainboardSharedGobPath(), snapshot); err != nil {
			return nil, nil, false, fmt.Errorf("写入共享日线GOB失败: %w", err)
		}
		mainboardSharedGobMu.Lock()
		mainboardSharedGob = loaded
		mainboardSharedGobMu.Unlock()
		shared = loaded
	}
	if priorDate != "" {
		staleStocks := []t0Stock{}
		for _, stock := range stocks {
			if !sharedGobStockHistoryCurrent(shared, stock.ShortCode, daily[stock.ShortCode], tradeDate, priorDate) {
				staleStocks = append(staleStocks, stock)
			}
		}
		sharedGobPriorDateCache.Lock()
		sharedGobPriorDateCache.excluded[tradeDate] = excluded
		sharedGobPriorDateCache.Unlock()
		missing := 0
		for _, stock := range staleStocks {
			if excluded[stock.ShortCode] == "" {
				missing++
			}
		}
		if missing > 0 {
			return nil, nil, false, fmt.Errorf("%s日线尚未补齐至前一交易日%s，缺少%d只，稍后重试", tradeDate, priorDate, missing)
		}
		if len(excluded) > 0 {
			active := make([]t0Stock, 0, len(stocks)-len(excluded))
			for _, stock := range stocks {
				if excluded[stock.ShortCode] == "" {
					active = append(active, stock)
				} else {
					delete(daily, stock.ShortCode)
				}
			}
			stocks = active
		}
	}
	return stocks, daily, !updated && len(fallback) == 0, nil
}

func sameT0StockPool(a, b []t0Stock) bool {
	if len(a) != len(b) {
		return false
	}
	byCode := make(map[string]t0Stock, len(a))
	for _, stock := range a {
		byCode[stock.ShortCode] = stock
	}
	for _, stock := range b {
		current, ok := byCode[stock.ShortCode]
		if !ok || current.Name != stock.Name || current.MarketCapYi != stock.MarketCapYi {
			return false
		}
	}
	return true
}

func sharedGobRedDatesDescending(data *mainboardSharedGobData) []string {
	dates := make([]string, len(data.Dates))
	for i, date := range data.Dates {
		dates[len(data.Dates)-1-i] = date
	}
	return dates
}

func sharedGobRedDatesDescendingAt(data *mainboardSharedGobData, now time.Time) []string {
	today := now.In(chinaLocation()).Format("2006-01-02")
	dates := make([]string, 0, len(data.Dates))
	seen := make(map[string]bool)
	for _, date := range sharedGobRedDatesDescending(data) {
		if date > today || isT0WeekendDate(date) {
			continue
		}
		_, confirmed, confirmationErr := loadSharedGobRedConfirmation(date)
		if confirmationErr != nil || (data.PoolErrors[date] != "" && !confirmed) {
			continue
		}
		ready := confirmed || len(data.SelectionPools[date]) > 0
		if !ready {
			_, err := readT0PoolArchive(date)
			ready = err == nil
		}
		if !ready {
			_, err := readT0HistoricalPoolReconstruction(date)
			ready = err == nil
		}
		if ready {
			dates = append(dates, date)
			seen[date] = true
		}
	}
	// A confirmed result remains usable even if its day has not yet been
	// appended to the shared daily-price calendar.
	files, _ := filepath.Glob(filepath.Join(t0CacheRootPath, "t0", "selection", "t0_red_confirmed_*.json"))
	for _, path := range files {
		date := strings.TrimSuffix(strings.TrimPrefix(filepath.Base(path), "t0_red_confirmed_"), ".json")
		if !seen[date] && date <= today && !isT0WeekendDate(date) {
			if _, found, err := loadSharedGobRedConfirmation(date); found && err == nil {
				dates = append(dates, date)
			}
		}
	}
	sort.Sort(sort.Reverse(sort.StringSlice(dates)))
	// shortcut: weekday only; use an exchange-session calendar when one is available here.
	if isT0WeekendDate(today) {
		return dates
	}
	if len(dates) == 0 || dates[0] != today {
		return append([]string{today}, dates...)
	}
	return dates
}

func sharedGobHasDate(data *mainboardSharedGobData, date string) bool {
	i := sort.SearchStrings(data.Dates, date)
	return i < len(data.Dates) && data.Dates[i] == date
}

func sharedGobDailyForStocks(
	shared *mainboardSharedGobData,
	stocks []t0Stock,
	endDate string,
	fallback map[string][]dailyBar,
) (map[string][]dailyBar, []t0Stock) {
	daily := make(map[string][]dailyBar, len(stocks))
	missing := make([]t0Stock, 0)
	for _, stock := range stocks {
		var bars []dailyBar
		if shared != nil {
			bars = dailyBarsOnOrBefore(shared.Daily[stock.ShortCode], endDate)
		}
		if len(bars) < 2 {
			bars = dailyBarsOnOrBefore(fallback[stock.ShortCode], endDate)
		}
		if len(bars) >= 2 {
			daily[stock.ShortCode] = bars
			continue
		}
		missing = append(missing, stock)
	}
	return daily, missing
}

func dailyBarsOnOrBefore(bars []dailyBar, endDate string) []dailyBar {
	if endDate == "" {
		return bars
	}
	end := sort.Search(len(bars), func(i int) bool { return bars[i].Date > endDate })
	return bars[:end]
}

func buildSharedGobRedAuctionQuotes(
	tradeDate string, stocks []t0Stock, daily map[string][]dailyBar,
) (map[string]t0Realtime, string, *t0DataWarning) {
	if tradeDate >= time.Now().In(chinaLocation()).Format("2006-01-02") {
		// The THS daily bar has a date but no quote time. The first complete
		// post-09:25 result is permanent, so require Tencent's timestamped quote.
		quoteStocks := stocks
		if shared, err := loadMainboardSharedGob(); err == nil {
			if pool := sharedGobT0SelectionPool(shared, tradeDate); len(pool) > 0 {
				quoteStocks = append([]t0Stock(nil), pool...)
				seen := make(map[string]bool, len(pool))
				for _, stock := range pool {
					seen[stock.ShortCode] = true
				}
				for _, stock := range stocks {
					if !seen[stock.ShortCode] {
						quoteStocks = append(quoteStocks, stock)
					}
				}
			}
		}
		// Capture the entire dated pool once, then require only rule candidates
		// to have opening quotes. Unrelated suspended stocks cannot block it.
		live := sharedGobRedLiveQuoteFetcher(quoteStocks)
		quotes := make(map[string]t0Realtime, len(live))
		for _, stock := range stocks {
			quote, ok := live[stock.ShortCode]
			if ok && quote.Source == "tencent" && quote.TradeDate == tradeDate &&
				quote.Open > 0 && quote.PrevClose > 0 {
				quotes[stock.ShortCode] = quote
			}
		}
		if missing := len(stocks) - len(quotes); missing > 0 {
			return quotes, "realtime_open", &t0DataWarning{
				Code: "T0_AUCTION_DATA_INCOMPLETE", Phase: "auction", Source: "realtime_open",
				Message:  fmt.Sprintf("竞价开盘数据不完整，缺少%d只有时间戳的股票报价", missing),
				Expected: len(stocks), Actual: len(quotes), Missing: missing,
			}
		}
		return quotes, "realtime_open", nil
	}
	const entrySource = "daily_open"
	quotes := make(map[string]t0Realtime, len(stocks))
	for _, stock := range stocks {
		bar, index := dailyBarOnDate(daily[stock.ShortCode], tradeDate)
		if index <= 0 || bar.Open <= 0 || daily[stock.ShortCode][index-1].Close <= 0 {
			continue
		}
		quotes[stock.ShortCode] = t0Realtime{
			Open: bar.Open, Close: bar.Close, PrevClose: daily[stock.ShortCode][index-1].Close,
		}
	}
	missing := len(stocks) - len(quotes)
	if missing == 0 {
		return quotes, entrySource, nil
	}
	return quotes, entrySource, &t0DataWarning{
		Code: "T0_AUCTION_DATA_INCOMPLETE", Phase: "auction", Source: "shared_gob",
		Message:  fmt.Sprintf("共享GOB缺少%d只股票的当日开盘或前收盘", missing),
		Expected: len(stocks), Actual: len(quotes), Missing: missing,
	}
}

func handleSharedGobRedSelectionUnconfirmed(w http.ResponseWriter, requestedDate string) {
	shared, err := loadMainboardSharedGob()
	if err != nil {
		WriteJSONStatus(w, http.StatusServiceUnavailable, map[string]interface{}{
			"error": err.Error(), "module_code": sharedGobRedStrategyModuleCode,
		})
		return
	}
	tradeDate := strings.TrimSpace(requestedDate)
	if tradeDate == "" {
		today := time.Now().In(chinaLocation()).Format("2006-01-02")
		if !isT0WeekendDate(today) && !isBeforeT0AuctionCutoff(time.Now(), today) {
			tradeDate = today
		} else {
			tradeDate = shared.Dates[len(shared.Dates)-1]
		}
	}
	if parsed, err := time.Parse("2006-01-02", tradeDate); err != nil || parsed.Format("2006-01-02") != tradeDate {
		WriteJSON(w, map[string]interface{}{
			"error": fmt.Sprintf("日期格式错误: %s (需为 2006-01-02)", tradeDate), "date": tradeDate,
		})
		return
	}
	today := time.Now().In(chinaLocation()).Format("2006-01-02")
	if isT0WeekendDate(tradeDate) || tradeDate > today ||
		(tradeDate != today && !sharedGobHasDate(shared, tradeDate)) {
		WriteJSON(w, map[string]interface{}{
			"date": tradeDate, "no_data": true, "count": 0,
			"results": []T0SelectionResult{}, "source": "shared_gob",
		})
		return
	}
	if tradeDate == today && isBeforeT0AuctionCutoff(time.Now(), tradeDate) {
		WriteJSON(w, map[string]interface{}{
			"date": tradeDate, "status": "awaiting_auction", "count": 0,
			"results": []T0SelectionResult{}, "source": "shared_gob",
		})
		return
	}

	if issue := shared.PoolErrors[tradeDate]; issue != "" {
		WriteJSONStatus(w, http.StatusServiceUnavailable, map[string]interface{}{
			"date": tradeDate, "data_incomplete": true,
			"code": "T0_HISTORY_POOL_INVALID", "error": issue,
		})
		return
	}
	var stocks []t0Stock
	var daily map[string][]dailyBar
	var reconstruction *t0HistoricalPoolReconstruction
	if tradeDate == today || len(shared.SelectionPools[tradeDate]) > 0 {
		stocks, daily, _, err = loadOrFetchSharedGobT0Daily(tradeDate)
		if err != nil {
			WriteJSONStatus(w, http.StatusServiceUnavailable, map[string]interface{}{
				"error": err.Error(), "module_code": sharedGobRedStrategyModuleCode,
			})
			return
		}
		shared, _ = loadMainboardSharedGob()
	} else {
		// The shared GOB's Stocks is the current float-cap pool. Historical
		// selections must use the pool captured for that trade date.
		cache, ok := readT0DailyCachePayloadFromDisk(tradeDate)
		if !ok || !cache.StockPoolComplete {
			reconstruction, err = readT0HistoricalPoolReconstruction(tradeDate)
			if err == nil {
				stocks = reconstruction.Stocks
				daily, _ = sharedGobDailyForStocks(shared, stocks, t0DailyFetchEndDate(tradeDate), reconstruction.Daily)
			} else {
				WriteJSONStatus(w, http.StatusServiceUnavailable, map[string]interface{}{
					"date": tradeDate, "data_incomplete": true,
					"code":  "T0_HISTORY_POOL_MISSING",
					"error": "缺少当日完整股票池，无法可靠回算历史红策名单",
				})
				return
			}
		} else {
			stocks = cache.Stocks
			// Dated caches may predate the T0 bar; shared dated prices take precedence.
			daily, _ = sharedGobDailyForStocks(shared, stocks, t0DailyFetchEndDate(tradeDate), cache.Daily)
		}
	}

	hist := make(map[string][]dailyBar, len(daily))
	for code, bars := range daily {
		hist[code] = histBarsBeforeTradeDate(bars, tradeDate)
	}
	limitUp := filterLimitUpRecent(stocks, hist, t0LimitUpMemoryDays, t0LimitUpCloseRet)
	turnover := filterTurnover(limitUp, hist, 5.0)
	warnings := make([]t0DataWarning, 0, 4)
	if reconstruction != nil {
		warnings = append(warnings, reconstruction.warning())
	}
	if warning := buildT0DailyCacheWarning(stocks, daily); warning != nil {
		warnings = append(warnings, *warning)
	}
	if tradeDate < today && len(shared.Dates) > 0 && tradeDate == shared.Dates[len(shared.Dates)-1] {
		if warning := buildSharedGobTradeDateWarning(stocks, daily, tradeDate); warning != nil {
			warnings = append(warnings, *warning)
		}
	}
	if warning := buildT0TurnoverWarning(limitUp, hist); warning != nil {
		warnings = append(warnings, *warning)
	}
	// Legacy and independent red rules must share one opening-quote snapshot.
	quoteStocks := append([]t0Stock(nil), turnover...)
	seen := make(map[string]bool, len(quoteStocks))
	for _, stock := range quoteStocks {
		seen[stock.ShortCode] = true
	}
	if tradeDate == today && !seen["600000"] {
		quoteStocks = append(quoteStocks, t0Stock{Code: "sh.600000", ShortCode: "600000", Name: "浦发银行"})
		seen["600000"] = true
	}
	for _, candidate := range buildGobRedPreopenCandidates(tradeDate, stocks, daily, approvedGobRedRules()) {
		code := t0ShortCodeFromResultCode(candidate.StockCode)
		if !seen[code] {
			quoteStocks = append(quoteStocks, resultsToT0Stocks([]T0SelectionResult{candidate})...)
			seen[code] = true
		}
	}
	quotes, _, quoteWarning := sharedGobRedQuoteBuilder(tradeDate, quoteStocks, daily)
	if quoteWarning != nil {
		warnings = append(warnings, *quoteWarning)
	}
	selectedForOpen, gaps := filterOpenGap(turnover, quotes, 0.01, 3.0)
	results := assembleT0CandidateResults(tradeDate, selectedForOpen, hist)
	for i := range results {
		code := t0ShortCodeFromResultCode(results[i].StockCode)
		results[i].OpenGap = round2(gaps[code])
		if tradeDate < today {
			if quote, ok := quotes[code]; ok && quote.PrevClose > 0 && quote.Close > 0 {
				results[i].CloseRet = round2((quote.Close - quote.PrevClose) / quote.PrevClose * 100)
			}
		}
	}
	sort.Slice(results, func(i, j int) bool { return results[i].OpenGap > results[j].OpenGap })

	rules := approvedGobRedRules()
	if tradeDate == today {
		openingRules := make([]redGobRule, 0, len(rules))
		for _, rule := range rules {
			if !rule.RequireT0BearClose {
				openingRules = append(openingRules, rule)
			}
		}
		rules = openingRules
	}
	independent, redWarnings := selectGobRedFromPoolWithRules(
		tradeDate, stocks, daily, latestGobPriorDate(tradeDate, daily), rules,
		func(_ string, _ []t0Stock, _ map[string][]dailyBar) (map[string]t0Realtime, string, *t0DataWarning) {
			if tradeDate == today {
				return quotes, "realtime_open", nil
			}
			return quotes, "daily_open", nil
		})
	warnings = append(warnings, redWarnings...)
	ctx := &t0ModuleSelectionContext{
		TradeDate: tradeDate, Daily: daily, Independent: independent, OpenGaps: gaps,
	}
	response := map[string]interface{}{
		"date": tradeDate, "count": len(results), "results": results, "source": "shared_gob",
	}
	if reconstruction != nil {
		response["pool_reconstructed"] = true
		response["date_pool_exact"] = false
		response["confirmed"] = false
		response["complete"] = false
		response["pool_source"] = reconstruction.Source
		response["pool_reference_date"] = reconstruction.ReferenceDate
		response["pool_reconstruction_method"] = reconstruction.Method
		response["pool_stocks"] = len(stocks)
	}
	if tradeDate != today {
		response["archived"] = true
	}
	if len(warnings) > 0 {
		response["data_incomplete"] = true
		attachT0DataWarnings(response, warnings)
	}
	if tradeDate != today {
		writeScopedT0ResponseWithContext(w, sharedGobRedStrategyModuleCode, response, ctx, "results")
		return
	}
	if len(warnings) > 0 {
		response["status"] = "warming"
		response["confirmed"] = false
		response["results"] = []T0SelectionResult{}
		response["count"] = 0
		WriteJSON(w, response)
		return
	}
	if err := prepareScopedT0ResponseWithContext(sharedGobRedStrategyModuleCode, response, ctx, "results"); err != nil {
		WriteAuthError(w, err)
		return
	}
	now := time.Now().In(chinaLocation()).Format(time.RFC3339)
	response["confirmed"] = true
	response["confirmed_at"] = now
	response["history_date"] = latestGobPriorDate(tradeDate, daily)
	encoded, err := json.Marshal(response)
	if err != nil {
		WriteJSONStatus(w, http.StatusServiceUnavailable, map[string]interface{}{"error": err.Error()})
		return
	}
	confirmed, err := saveSharedGobRedConfirmation(sharedGobRedConfirmation{
		SchemaVersion: 1, Date: tradeDate, ConfirmedAt: now,
		HistoryDate: latestGobPriorDate(tradeDate, daily), GobGeneratedAt: shared.Snapshot.GeneratedAt,
		RuleSetVersion: currentGobRedRuleSetVersion(), Quotes: quotes, Response: encoded,
		ExcludedStocks: sharedGobRedExcludedStocks(tradeDate),
	})
	if err != nil {
		WriteJSONStatus(w, http.StatusServiceUnavailable, map[string]interface{}{"date": tradeDate, "error": "红策确认名单未保存: " + err.Error()})
		return
	}
	WriteJSON(w, confirmed)
}

func buildSharedGobTradeDateWarning(
	stocks []t0Stock, daily map[string][]dailyBar, tradeDate string,
) *t0DataWarning {
	missing := 0
	for _, stock := range stocks {
		if _, index := dailyBarOnDate(daily[stock.ShortCode], tradeDate); index < 0 {
			missing++
		}
	}
	if missing == 0 {
		return nil
	}
	return &t0DataWarning{
		Code: "SHARED_GOB_TRADE_DATE_INCOMPLETE", Phase: "daily_fetch", Source: "shared_gob",
		Message:  fmt.Sprintf("共享GOB的%s日线有%d只股票缺失（可能停牌或数据尚未到齐）", tradeDate, missing),
		Expected: len(stocks), Actual: len(stocks) - missing, Missing: missing,
	}
}

func sharedGobRedKlineWindows(date string, codes []string, daily map[string][]dailyBar) map[string]redKlineWindow {
	items := make(map[string]redKlineWindow, len(codes))
	for _, code := range codes {
		shortCode := t0ShortCodeFromResultCode(code)
		window := buildRedKlineWindow(code, date, daily[shortCode], "shared_gob")
		items[code] = window
	}
	return items
}
