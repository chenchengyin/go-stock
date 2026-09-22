package flutter_api

import "go-stock/backend/analysis/candlepattern"

const (
	displayRuleAnyLimitUpLimitDown   = "任意K线＋涨停＋跌停"
	displayRuleMediumYangLimitDownT0 = "中阳及以上＋跌停后的开盘竞价"
	displayRuleBullishZtZtPb         = "涨停＋涨停＋阳线破板"
	displayRuleZtZtBearishT0         = "涨停＋涨停＋普通阴线"
	displayRuleZtNonOneWordT0        = "涨停＋非一字涨停后的开盘竞价"
	strongDisplayPrevDayOpenGap      = 3.0
	techBlueT1OpenGap                = 7.0
	bearishBoundaryEpsilon           = 1e-9
)

// t0DisplayRule 定义列表标红的命中条件。
// 它不参与基础股票池过滤或主选股链；后续增加标红条件时，在 t0DisplayRules 中追加一项即可。
type t0DisplayRule struct {
	Name         string
	Match        func([]dailyBar) bool
	MatchResult  func([]dailyBar, T0SelectionResult) bool
	DeepRedMatch func([]dailyBar, T0SelectionResult) bool
}

var t0DisplayRules = []t0DisplayRule{
	{
		Name:         displayRuleAnyLimitUpLimitDown,
		Match:        matchesAnyLimitUpLimitDown,
		DeepRedMatch: matchesAnyLimitUpLimitDownDeepRed,
	},
	{
		Name:        displayRuleMediumYangLimitDownT0,
		MatchResult: matchesMediumYangThenLimitDownT0,
	},
	{
		Name:  displayRuleBullishZtZtPb,
		Match: matchesBullishZtZtPb,
	},
	{
		Name:        displayRuleZtZtBearishT0,
		MatchResult: matchesZtZtBearishT0,
	},
	{
		Name:         displayRuleZtNonOneWordT0,
		MatchResult:  matchesZtNonOneWordT0,
		DeepRedMatch: matchesStrongTwoLimitUpNonOneWordAuction,
	},
}

func displayRuleHitsForHist(hist []dailyBar) []string {
	return displayRuleHitsForResult(hist, T0SelectionResult{})
}

func displayRuleHitsForResult(hist []dailyBar, result T0SelectionResult) []string {
	hits := make([]string, 0, len(t0DisplayRules))
	for _, rule := range t0DisplayRules {
		matched := false
		if rule.MatchResult != nil {
			matched = rule.MatchResult(hist, result)
		} else if rule.Match != nil {
			matched = rule.Match(hist)
		}
		if matched {
			hits = append(hits, rule.Name)
		}
	}
	return hits
}

// matchesStrongPrevDayOpenGap 判断前一交易日相对再前一日收盘价的开盘涨幅 >= 3%。
func matchesStrongPrevDayOpenGap(hist []dailyBar) bool {
	_, openRet, _, ok := prevDayRetsFromHist(hist)
	return ok && openRet >= strongDisplayPrevDayOpenGap
}

// matchesZtNonOneWordT0 匹配：T-2 收盘涨停、T-1 收盘涨停且 T-1 非一字，
// 随后正式 T0 竞价买入。当前正式竞价口径为 0.01%～3%。
func matchesZtNonOneWordT0(hist []dailyBar, result T0SelectionResult) bool {
	if result.OpenGap < 0.01 || result.OpenGap > 3 || len(hist) < 3 {
		return false
	}
	base := hist[len(hist)-3]
	firstLimitUp := hist[len(hist)-2]
	secondLimitUp := hist[len(hist)-1]
	return isCloseLimitUpDay(base.Close, firstLimitUp, t0LimitUpCloseRet) &&
		isNonOneWordLimitUpDay(firstLimitUp.Close, secondLimitUp)
}

// matchesStrongTwoLimitUpNonOneWordAuction 是上述组合中 T-1 开盘涨幅 >= 3% 的子集，
// 仅保留用于兼容列表深红强调；红策准入由 matchesTechBlueRedRule 决定。
func matchesStrongTwoLimitUpNonOneWordAuction(
	hist []dailyBar,
	result T0SelectionResult,
) bool {
	return matchesZtNonOneWordT0(hist, result) && matchesStrongPrevDayOpenGap(hist)
}

// matchesTechBlueRedRule 是红策的硬准入条件：
// T-2、T-1 均为非一字涨停；T-1 开盘涨幅至少 7%。
// T0 竞价仍沿用正式结果口径 0.01%～3%。
func matchesTechBlueRedRule(hist []dailyBar, result T0SelectionResult) bool {
	if result.OpenGap < 0.01 || result.OpenGap > 3 || len(hist) < 3 {
		return false
	}

	base := hist[len(hist)-3]
	firstLimitUp := hist[len(hist)-2]
	secondLimitUp := hist[len(hist)-1]
	if !isNonOneWordLimitUpDay(base.Close, firstLimitUp) ||
		!isNonOneWordLimitUpDay(firstLimitUp.Close, secondLimitUp) {
		return false
	}

	secondOpenGap, ok := openingGapFromPreviousClose(firstLimitUp.Close, secondLimitUp)
	return ok && secondOpenGap >= techBlueT1OpenGap
}

func openingGapFromPreviousClose(prevClose float64, bar dailyBar) (float64, bool) {
	if prevClose <= 0 || bar.Open <= 0 {
		return 0, false
	}
	return (bar.Open - prevClose) / prevClose * 100, true
}

func matchesDeepRedDisplayRule(hist []dailyBar, result T0SelectionResult) bool {
	for _, rule := range t0DisplayRules {
		if rule.DeepRedMatch != nil && rule.DeepRedMatch(hist, result) {
			return true
		}
	}
	return false
}

func isNonOneWordLimitUpDay(prevClose float64, bar dailyBar) bool {
	if !isCloseLimitUpDay(prevClose, bar, t0LimitUpCloseRet) ||
		bar.Open <= 0 || bar.High <= 0 || bar.Low <= 0 {
		return false
	}
	return bar.Open != bar.Close || bar.High != bar.Close || bar.Low != bar.Close
}

// matchesMediumYangThenLimitDownT0 匹配最近三根历史K线：第一根不限，第二根中阳及以上，第三根跌停。
// 中阳及以上沿用形态统计口径：MY、DY、ZT；仅正式T0竞价结果（0.01%～3%）命中。
func matchesMediumYangThenLimitDownT0(hist []dailyBar, result T0SelectionResult) bool {
	if result.OpenGap < 0.01 || result.OpenGap > 3 {
		return false
	}
	if len(hist) < 3 {
		return false
	}

	base := hist[len(hist)-3]
	redK := hist[len(hist)-2]
	limitDown := hist[len(hist)-1]
	redType := classifyDisplayBar(base.Close, redK)
	if !isMediumYangOrAboveType(redType) {
		return false
	}
	return classifyDisplayBar(redK.Close, limitDown) == candlepattern.BarDT
}

func classifyDisplayBar(prevClose float64, bar dailyBar) candlepattern.BarType {
	return candlepattern.ClassifyDailyBar(prevClose, candlepattern.DailyBar{
		Date:     bar.Date,
		Open:     bar.Open,
		Close:    bar.Close,
		High:     bar.High,
		Low:      bar.Low,
		Volume:   bar.Volume,
		AmountYi: bar.AmountYi,
	})
}

func isMediumYangOrAboveType(barType candlepattern.BarType) bool {
	switch barType {
	case candlepattern.BarZT, candlepattern.BarMY, candlepattern.BarDY:
		return true
	default:
		return false
	}
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

func matchesAnyLimitUpLimitDownDeepRed(hist []dailyBar, result T0SelectionResult) bool {
	if result.OpenGap < 0.01 || result.OpenGap > 3 {
		return false
	}
	return matchesAnyLimitUpLimitDown(hist)
}

func matchesBullishZtZtPb(hist []dailyBar) bool {
	if patternFromHist(hist) != "ZT|ZT|PB" || len(hist) == 0 {
		return false
	}
	brokenLimitUp := hist[len(hist)-1]
	return brokenLimitUp.Close > brokenLimitUp.Open
}

// matchesZtZtBearishT0 匹配两连涨停后的普通阴线：收盘相对第二个涨停收盘在-2%～3%，
// 阴线实体相对第二个涨停收盘严格小于8%。
// 正式T0结果的开盘涨幅已由全局选股链限制在0.01%～3%；这里保留同样的范围保护，
// 避免尚未确认开盘价、OpenGap暂为0的预热候选误命中。
func matchesZtZtBearishT0(hist []dailyBar, result T0SelectionResult) bool {
	if result.OpenGap < 0.01 || result.OpenGap > 3 {
		return false
	}
	if len(hist) < 4 {
		return false
	}
	base := hist[len(hist)-4]
	firstLimitUp := hist[len(hist)-3]
	secondLimitUp := hist[len(hist)-2]
	bearish := hist[len(hist)-1]
	if !isCloseLimitUpDay(base.Close, firstLimitUp, t0LimitUpCloseRet) ||
		!isCloseLimitUpDay(firstLimitUp.Close, secondLimitUp, t0LimitUpCloseRet) ||
		bearish.Close >= bearish.Open {
		return false
	}
	closeRet, _, ok := barCloseHighRet(secondLimitUp.Close, bearish)
	if !ok || closeRet < -2-bearishBoundaryEpsilon ||
		closeRet > 3+bearishBoundaryEpsilon {
		return false
	}
	bodyDrop := (bearish.Open - bearish.Close) / secondLimitUp.Close * 100
	return bodyDrop > 0 && bodyDrop < 8-bearishBoundaryEpsilon
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
	referenceRules := loadT0ReferenceRuleRuntimes()
	for i := range out {
		hist := histBarsBeforeTradeDate(
			daily[t0ShortCodeFromResultCode(out[i].StockCode)], tradeDate)
		out[i].DisplayRuleHits = displayRuleHitsForResult(hist, out[i])
		out[i].StrongDisplayRuleHit = matchesDeepRedDisplayRule(hist, out[i])
		enrichT0ReferenceResult(&out[i], hist, referenceRules)
	}
	return out
}
