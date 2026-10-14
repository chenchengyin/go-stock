package flutter_api

import (
	"encoding/gob"
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
}

var (
	mainboardSharedGobMu     sync.RWMutex
	mainboardSharedGobLoadMu sync.Mutex
	mainboardSharedGobUpdate sync.Mutex
	mainboardSharedGob       *mainboardSharedGobData
)

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
	loaded, err = buildMainboardSharedGobData(snapshot)
	if err != nil {
		return nil, err
	}
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
	stocks := shared.SelectionPools[tradeDate]
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

	endDate := t0DailyFetchEndDate(tradeDate)
	daily, missing := sharedGobDailyForStocks(shared, stocks, endDate, nil)
	local := time.Now().In(chinaLocation())
	if tradeDate == local.Format("2006-01-02") &&
		local.Hour()*60+local.Minute() >= mainboardSharedGobRefreshStartMinute {
		missingSet := make(map[string]struct{}, len(missing))
		for _, stock := range missing {
			missingSet[stock.ShortCode] = struct{}{}
		}
		for _, stock := range stocks {
			if containsSharedGobBarDate(shared.Daily[stock.ShortCode], tradeDate) {
				continue
			}
			if _, exists := missingSet[stock.ShortCode]; exists {
				continue
			}
			missingSet[stock.ShortCode] = struct{}{}
			missing = append(missing, stock)
		}
	}
	fallback := fetchAllDailyKLine(missing, endDate, nil)
	for code, bars := range fallback {
		daily[code] = bars
	}
	if len(daily) == 0 {
		return nil, nil, false, fmt.Errorf("共享日线GOB和缺失股票回退都没有可用日线")
	}

	if updated || len(fallback) > 0 {
		snapshot := shared.Snapshot
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
		}
		if dates := mainboardSharedGobDates(snapshot.Daily); len(dates) > 0 {
			snapshot.EndDate = dates[len(dates)-1]
		}
		loaded, err := buildMainboardSharedGobData(snapshot)
		if err != nil {
			return nil, nil, false, err
		}
		if err := writeMainboardFloatCap50Gob(mainboardSharedGobPath(), snapshot); err != nil {
			return nil, nil, false, fmt.Errorf("写入共享日线GOB失败: %w", err)
		}
		mainboardSharedGobMu.Lock()
		mainboardSharedGob = loaded
		mainboardSharedGobMu.Unlock()
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
	dates := sharedGobRedDatesDescending(data)
	today := now.In(chinaLocation()).Format("2006-01-02")
	// shortcut: weekday only; use an exchange-session calendar when one is available here.
	if isT0WeekendDate(today) || isBeforeT0AuctionCutoff(now, today) {
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
		quotes, _, warning := buildT0AuctionQuotes(tradeDate, stocks, daily)
		if warning != nil {
			warning.Source = "realtime_open"
		}
		return quotes, "realtime_open", warning
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

func handleSharedGobRedSelection(w http.ResponseWriter, requestedDate string) {
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

	stocks, daily := shared.Stocks, shared.Daily
	if tradeDate == today || len(shared.SelectionPools[tradeDate]) > 0 {
		stocks, daily, _, err = loadOrFetchSharedGobT0Daily(tradeDate)
		if err != nil {
			WriteJSONStatus(w, http.StatusServiceUnavailable, map[string]interface{}{
				"error": err.Error(), "module_code": sharedGobRedStrategyModuleCode,
			})
			return
		}
	}

	hist := make(map[string][]dailyBar, len(daily))
	for code, bars := range daily {
		hist[code] = histBarsBeforeTradeDate(bars, tradeDate)
	}
	limitUp := filterLimitUpRecent(stocks, hist, t0LimitUpMemoryDays, t0LimitUpCloseRet)
	turnover := filterTurnover(limitUp, hist, 5.0)
	warnings := make([]t0DataWarning, 0, 4)
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
	quotes, _, quoteWarning := buildSharedGobRedAuctionQuotes(tradeDate, turnover, daily)
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

	independent, redWarnings := selectGobRedFromSharedPool(
		tradeDate, stocks, daily, latestGobPriorDate(tradeDate, daily))
	warnings = append(warnings, redWarnings...)
	ctx := &t0ModuleSelectionContext{
		TradeDate: tradeDate, Daily: daily, Independent: independent,
	}
	response := map[string]interface{}{
		"date": tradeDate, "count": len(results), "results": results, "source": "shared_gob",
	}
	if len(warnings) > 0 {
		response["data_incomplete"] = true
		attachT0DataWarnings(response, warnings)
	}
	writeScopedT0ResponseWithContext(w, sharedGobRedStrategyModuleCode, response, ctx, "results")
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
