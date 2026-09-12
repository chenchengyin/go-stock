package flutter_api

import (
	"fmt"
	"net/http"
	"sync"
	"time"
)

const (
	redT0StrategyModuleCode    = "radar.red_strategy"
	purpleT0StrategyModuleCode = "radar.purple_strategy"
)

var redT0DailyKLineFetcher = fetchDailyKLineWithLimit

func isRedT0StrategyModule(moduleCode string) bool {
	return moduleCode == redT0StrategyModuleCode
}

func isT0DailyContextModule(moduleCode string) bool {
	return isRedT0StrategyModule(moduleCode) ||
		moduleCode == purpleT0StrategyModuleCode
}

func t0DailyContextModuleName(moduleCode string) string {
	if moduleCode == purpleT0StrategyModuleCode {
		return "紫策"
	}
	return "红策"
}

func loadT0ModuleSelectionContext(
	moduleCode string,
	tradeDate string,
) (*t0ModuleSelectionContext, error) {
	if !isT0DailyContextModule(moduleCode) {
		return nil, nil
	}
	cached, ok := loadT0DailyCache(tradeDate)
	if !ok || cached == nil {
		return nil, fmt.Errorf("%s日线缓存未就绪: %s",
			t0DailyContextModuleName(moduleCode), tradeDate)
	}
	return &t0ModuleSelectionContext{
		TradeDate: tradeDate,
		Daily:     cached.Daily,
	}, nil
}

func loadT0ModuleSelectionContextForResults(
	moduleCode string,
	tradeDate string,
	results []T0SelectionResult,
) (*t0ModuleSelectionContext, error) {
	if !isT0DailyContextModule(moduleCode) {
		return nil, nil
	}
	if cached, ok := loadT0DailyCache(tradeDate); ok && cached != nil {
		return &t0ModuleSelectionContext{
			TradeDate: tradeDate,
			Daily:     cached.Daily,
		}, nil
	}
	return &t0ModuleSelectionContext{
		TradeDate: tradeDate,
		Daily:     fetchRedT0DailyKLines(results, tradeDate),
	}, nil
}

func fetchRedT0DailyKLines(
	results []T0SelectionResult,
	tradeDate string,
) map[string][]dailyBar {
	codes := make(map[string]struct{}, len(results))
	for _, result := range results {
		code := t0ShortCodeFromResultCode(result.StockCode)
		if code != "" {
			codes[code] = struct{}{}
		}
	}

	daily := make(map[string][]dailyBar, len(codes))
	var mu sync.Mutex
	var wg sync.WaitGroup
	sem := make(chan struct{}, 20)
	for code := range codes {
		wg.Add(1)
		go func(shortCode string) {
			defer wg.Done()
			sem <- struct{}{}
			defer func() { <-sem }()
			bars := fetchRedT0DailyKLineSafely(shortCode, tradeDate)
			if len(bars) < 2 {
				return
			}
			mu.Lock()
			daily[shortCode] = bars
			mu.Unlock()
		}(code)
	}
	wg.Wait()
	return daily
}

func passesT0HistoricalPriceFloor(result T0SelectionResult, hist []dailyBar) bool {
	return len(hist) > 0 && result.PrevClose >= hist[0].Close
}

func fetchRedT0DailyKLineSafely(shortCode, tradeDate string) (bars []dailyBar) {
	defer func() {
		if recover() != nil {
			bars = nil
		}
	}()
	return redT0DailyKLineFetcher(shortCode, tradeDate, 60)
}

func validateT0ModuleCode(moduleCode string) error {
	switch moduleCode {
	case "radar.main_strategy", redT0StrategyModuleCode,
		purpleT0StrategyModuleCode, "radar.blue_strategy":
		return nil
	default:
		return newAuthError(http.StatusBadRequest,
			"INVALID_ARGUMENT", "模块不存在")
	}
}

func writeRedT0PrewarmReadyHTTP(
	w http.ResponseWriter,
	tradeDate string,
	moduleCode string,
) {
	writeContextT0PrewarmReadyHTTP(w, tradeDate, moduleCode)
}

func writeContextT0PrewarmReadyHTTP(
	w http.ResponseWriter,
	tradeDate string,
	moduleCode string,
) {
	cached, ok := loadT0DailyCache(tradeDate)
	if !ok || cached == nil {
		WriteJSON(w, map[string]interface{}{
			"error":  fmt.Sprintf("%s日线缓存未就绪", t0DailyContextModuleName(moduleCode)),
			"date":   tradeDate,
			"status": string(t0WarmStatusWarming),
		})
		return
	}

	response := buildPrewarmReadyResponseAtWithCache(
		tradeDate, time.Now(), cached)
	contextDate := tradeDate
	if displayDate, ok := response["display_date"].(string); ok && displayDate != "" {
		contextDate = displayDate
	}
	contextPayload := cached
	if contextDate != tradeDate {
		var contextOK bool
		contextPayload, contextOK = loadT0DailyCache(contextDate)
		if !contextOK || contextPayload == nil {
			WriteJSON(w, map[string]interface{}{
				"error":    fmt.Sprintf("%s历史日线缓存未就绪: %s", t0DailyContextModuleName(moduleCode), contextDate),
				"date":     contextDate,
				"archived": true,
				"count":    0,
			})
			return
		}
	}

	ctx := &t0ModuleSelectionContext{
		TradeDate: contextDate,
		Daily:     contextPayload.Daily,
	}
	writeScopedT0ResponseWithContext(
		w, moduleCode, response, ctx, "results", "candidates")
}

type t0ModuleSelectionContext struct {
	TradeDate string
	Daily     map[string][]dailyBar
}

func filterRedT0Results(
	results []T0SelectionResult,
	ctx *t0ModuleSelectionContext,
) []T0SelectionResult {
	if ctx == nil {
		return nil
	}

	filtered := make([]T0SelectionResult, 0, len(results))
	for _, result := range results {
		shortCode := t0ShortCodeFromResultCode(result.StockCode)
		hist := histBarsBeforeTradeDate(ctx.Daily[shortCode], ctx.TradeDate)
		displayRuleHits := displayRuleHitsForResult(hist, result)
		if len(displayRuleHits) == 0 {
			continue
		}

		result.DisplayRuleHits = displayRuleHits
		result.Tag = ""
		if highRet, openRet, closeRet, ok := prevDayRetsFromHist(hist); ok {
			result.Tag = pickPrevDayTag(highRet, openRet, closeRet)
		}
		filtered = append(filtered, result)
	}
	return filtered
}

func selectT0ResultsForModule(
	moduleCode string,
	results []T0SelectionResult,
	ctx *t0ModuleSelectionContext,
) ([]T0SelectionResult, error) {
	if err := validateT0ModuleCode(moduleCode); err != nil {
		return nil, err
	}
	switch moduleCode {
	case "radar.main_strategy":
		return results, nil
	case redT0StrategyModuleCode:
		if ctx == nil {
			return nil, newAuthError(http.StatusInternalServerError,
				"T0_DATA_NOT_READY", "红策日线数据未就绪")
		}
		return filterRedT0Results(results, ctx), nil
	case purpleT0StrategyModuleCode:
		if ctx == nil {
			return nil, newAuthError(http.StatusInternalServerError,
				"T0_DATA_NOT_READY", "紫策日线数据未就绪")
		}
		return filterPurpleT0Results(results, ctx), nil
	case "radar.blue_strategy":
		return filterBlueT0Results(results), nil
	default:
		return nil, validateT0ModuleCode(moduleCode)
	}
}
