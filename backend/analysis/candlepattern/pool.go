package candlepattern

// A2 动量池逻辑对齐 backend/flutter_api/t0_selection.go:
// histBarsBeforeTradeDate (~L1413), hasLimitUpMemory (~L1276), filterTurnover (~L1335)

const (
	limitUpMemoryDays = 7
	minTurnoverYi     = 5.0
)

func HistBeforeTradeDate(bars []DailyBar, tradeDate string) []DailyBar {
	if len(bars) == 0 {
		return bars
	}
	if bars[len(bars)-1].Date == tradeDate {
		return bars[:len(bars)-1]
	}
	return bars
}

func barCloseHighRet(prevClose float64, bar DailyBar) (closeRet, highRet float64, ok bool) {
	if prevClose == 0 {
		return 0, 0, false
	}
	closeRet = (bar.Close - prevClose) / prevClose * 100
	highRet = (bar.High - prevClose) / prevClose * 100
	return closeRet, highRet, true
}

func isCloseLimitUpDay(prevClose float64, bar DailyBar, closeThreshold float64) bool {
	closeRet, _, ok := barCloseHighRet(prevClose, bar)
	return ok && closeRet >= closeThreshold
}

func isBrokenLimitUpDay(prevClose float64, bar DailyBar) bool {
	closeRet, highRet, ok := barCloseHighRet(prevClose, bar)
	return ok && highRet >= limitUpCloseRet && closeRet < brokenLimitRet
}

func limitUpMemoryTail(hist []DailyBar, days int) []DailyBar {
	if days < 1 || len(hist) < 2 {
		return nil
	}
	need := days + 1
	if len(hist) <= need {
		return hist
	}
	return hist[len(hist)-need:]
}

func hasLimitUpMemory(hist []DailyBar, days int, closeThreshold float64) bool {
	tail := limitUpMemoryTail(hist, days)
	last := len(tail) - 1
	for i := 1; i < len(tail); i++ {
		prev, bar := tail[i-1].Close, tail[i]
		if isCloseLimitUpDay(prev, bar, closeThreshold) {
			return true
		}
		if i == last && isBrokenLimitUpDay(prev, bar) {
			return true
		}
	}
	return false
}

func InA2Pool(hist []DailyBar) bool {
	if len(hist) == 0 {
		return false
	}
	if !hasLimitUpMemory(hist, limitUpMemoryDays, limitUpCloseRet) {
		return false
	}
	last := hist[len(hist)-1]
	return last.AmountYi >= minTurnoverYi
}

func FilterA2Pool(cache *DailyCache, tradeDate string) map[string][]DailyBar {
	out := make(map[string][]DailyBar)
	for _, s := range cache.Stocks {
		bars := cache.Daily[s.ShortCode]
		hist := HistBeforeTradeDate(bars, tradeDate)
		if InA2Pool(hist) {
			out[s.ShortCode] = hist
		}
	}
	return out
}
