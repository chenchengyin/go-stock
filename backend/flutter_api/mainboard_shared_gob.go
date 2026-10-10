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
	Snapshot MainboardFloatCap50Gob
	Stocks   []t0Stock
	Daily    map[string][]dailyBar
	Dates    []string
}

var (
	mainboardSharedGobMu     sync.RWMutex
	mainboardSharedGobLoadMu sync.Mutex
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
	if snapshot.SchemaVersion != 1 || len(snapshot.Stocks) == 0 || len(snapshot.Daily) == 0 {
		return nil, fmt.Errorf("共享日线GOB格式或内容无效")
	}
	data := &mainboardSharedGobData{
		Snapshot: snapshot,
		Stocks:   make([]t0Stock, 0, len(snapshot.Stocks)),
		Daily:    snapshot.Daily,
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
	return data, nil
}

func sharedGobRedDatesDescending(data *mainboardSharedGobData) []string {
	dates := make([]string, len(data.Dates))
	for i, date := range data.Dates {
		dates[len(data.Dates)-1-i] = date
	}
	return dates
}

func sharedGobHasDate(data *mainboardSharedGobData, date string) bool {
	i := sort.SearchStrings(data.Dates, date)
	return i < len(data.Dates) && data.Dates[i] == date
}

func sharedGobPriorDate(data *mainboardSharedGobData, tradeDate string) string {
	i := sort.SearchStrings(data.Dates, tradeDate)
	if i == 0 {
		return ""
	}
	return data.Dates[i-1]
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
		tradeDate = shared.Dates[len(shared.Dates)-1]
	}
	if parsed, err := time.Parse("2006-01-02", tradeDate); err != nil || parsed.Format("2006-01-02") != tradeDate {
		WriteJSON(w, map[string]interface{}{
			"error": fmt.Sprintf("日期格式错误: %s (需为 2006-01-02)", tradeDate), "date": tradeDate,
		})
		return
	}
	if isT0WeekendDate(tradeDate) || !sharedGobHasDate(shared, tradeDate) {
		WriteJSON(w, map[string]interface{}{
			"date": tradeDate, "no_data": true, "count": 0,
			"results": []T0SelectionResult{}, "source": "shared_gob",
		})
		return
	}

	hist := make(map[string][]dailyBar, len(shared.Daily))
	for code, bars := range shared.Daily {
		hist[code] = histBarsBeforeTradeDate(bars, tradeDate)
	}
	limitUp := filterLimitUpRecent(shared.Stocks, hist, t0LimitUpMemoryDays, t0LimitUpCloseRet)
	turnover := filterTurnover(limitUp, hist, 5.0)
	warnings := make([]t0DataWarning, 0, 4)
	if warning := buildT0DailyCacheWarning(shared.Stocks, shared.Daily); warning != nil {
		warnings = append(warnings, *warning)
	}
	if tradeDate == shared.Dates[len(shared.Dates)-1] {
		if warning := buildSharedGobTradeDateWarning(shared.Stocks, shared.Daily, tradeDate); warning != nil {
			warnings = append(warnings, *warning)
		}
	}
	if warning := buildT0TurnoverWarning(limitUp, hist); warning != nil {
		warnings = append(warnings, *warning)
	}
	quotes, _, quoteWarning := buildSharedGobRedAuctionQuotes(tradeDate, turnover, shared.Daily)
	if quoteWarning != nil {
		warnings = append(warnings, *quoteWarning)
	}
	selectedForOpen, gaps := filterOpenGap(turnover, quotes, 0.01, 3.0)
	results := assembleT0CandidateResults(tradeDate, selectedForOpen, hist)
	for i := range results {
		code := t0ShortCodeFromResultCode(results[i].StockCode)
		results[i].OpenGap = round2(gaps[code])
		if quote, ok := quotes[code]; ok && quote.PrevClose > 0 && quote.Close > 0 {
			results[i].CloseRet = round2((quote.Close - quote.PrevClose) / quote.PrevClose * 100)
		}
	}
	sort.Slice(results, func(i, j int) bool { return results[i].OpenGap > results[j].OpenGap })

	independent, redWarnings := selectGobRedFromSharedPool(
		tradeDate, shared.Stocks, shared.Daily, sharedGobPriorDate(shared, tradeDate))
	warnings = append(warnings, redWarnings...)
	ctx := &t0ModuleSelectionContext{
		TradeDate: tradeDate, Daily: shared.Daily, Independent: independent,
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
