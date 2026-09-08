package flutter_api

const (
	displayRuleAnyLimitUpLimitDown = "任意K线＋涨停＋跌停"
	displayRuleLimitUpBearishTag   = "涨停＋阴线标记"
	displayRuleBullishZtZtPb       = "涨停＋涨停＋阳线破板"
)

// t0DisplayRule 只负责列表展示命中，不参与股票池过滤或选股结果归档。
// 后续增加展示条件时，在 t0DisplayRules 中追加一项即可。
type t0DisplayRule struct {
	Name  string
	Match func([]dailyBar) bool
}

var t0DisplayRules = []t0DisplayRule{
	{
		Name:  displayRuleAnyLimitUpLimitDown,
		Match: matchesAnyLimitUpLimitDown,
	},
	{
		Name:  displayRuleLimitUpBearishTag,
		Match: matchesLimitUpAndBearishTag,
	},
	{
		Name:  displayRuleBullishZtZtPb,
		Match: matchesBullishZtZtPb,
	},
}

func displayRuleHitsForHist(hist []dailyBar) []string {
	hits := make([]string, 0, len(t0DisplayRules))
	for _, rule := range t0DisplayRules {
		if rule.Match != nil && rule.Match(hist) {
			hits = append(hits, rule.Name)
		}
	}
	return hits
}

// matchesAnyLimitUpLimitDown 匹配最近三根历史K线：第一根不限，第二根涨停，第三根跌停。
func matchesAnyLimitUpLimitDown(hist []dailyBar) bool {
	if len(hist) < 3 {
		return false
	}
	first := hist[len(hist)-3]
	limitUp := hist[len(hist)-2]
	limitDown := hist[len(hist)-1]
	return isCloseLimitUpDay(first.Close, limitUp, t0LimitUpCloseRet) &&
		isCloseLimitDownDay(limitUp.Close, limitDown)
}

// matchesLimitUpAndBearishTag 匹配最近两根历史K线：前一根涨停，最新一根符合阴线标记逻辑。
func matchesLimitUpAndBearishTag(hist []dailyBar) bool {
	if len(hist) < 3 {
		return false
	}
	limitUpBase := hist[len(hist)-3]
	limitUp := hist[len(hist)-2]
	latest := hist[len(hist)-1]
	if !isCloseLimitUpDay(limitUpBase.Close, limitUp, t0LimitUpCloseRet) {
		return false
	}

	// 复用列表现有的前一天标记计算，避免显示条件和标记字段出现口径分叉。
	latestHist := []dailyBar{limitUpBase, limitUp, latest}
	highRet, openRet, closeRet, ok := prevDayRetsFromHist(latestHist)
	return ok && isPrevDayBearishTag(highRet, openRet, closeRet)
}

func matchesBullishZtZtPb(hist []dailyBar) bool {
	if patternFromHist(hist) != "ZT|ZT|PB" || len(hist) == 0 {
		return false
	}
	brokenLimitUp := hist[len(hist)-1]
	return brokenLimitUp.Close > brokenLimitUp.Open
}

func isCloseLimitDownDay(prevClose float64, bar dailyBar) bool {
	closeRet, _, ok := barCloseHighRet(prevClose, bar)
	return ok && closeRet <= -9.9
}

// enrichT0ResultsForDisplay 只读指定交易日的本地 gob 缓存。
// 缓存不存在时原样返回，绝不触发股票池或K线请求。
func enrichT0ResultsForDisplay(tradeDate string, results []T0SelectionResult) []T0SelectionResult {
	if len(results) == 0 || tradeDate == "" {
		return results
	}
	cached, ok := loadT0DailyCache(tradeDate)
	if !ok || cached == nil {
		return results
	}
	return enrichT0ResultsForDisplayWithDaily(results, cached.Daily, tradeDate)
}

func enrichT0ResultsForDisplayWithDaily(
	results []T0SelectionResult,
	daily map[string][]dailyBar,
	tradeDate string,
) []T0SelectionResult {
	out := make([]T0SelectionResult, len(results))
	copy(out, results)
	for i := range out {
		hist := histBarsBeforeTradeDate(
			daily[t0ShortCodeFromResultCode(out[i].StockCode)], tradeDate)
		out[i].DisplayRuleHits = displayRuleHitsForHist(hist)
	}
	return out
}
