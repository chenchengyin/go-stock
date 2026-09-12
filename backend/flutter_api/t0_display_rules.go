package flutter_api

import "go-stock/backend/analysis/candlepattern"

const (
	displayRuleAnyLimitUpLimitDown = "任意K线＋涨停＋跌停"
	displayRuleRedKLimitDownT0     = "红K＋跌停后的开盘竞价"
	displayRuleLimitUpBearishTag   = "涨停＋阴线标记"
	displayRuleBullishZtZtPb       = "涨停＋涨停＋阳线破板"
	displayRuleZtZtBearishT0       = "涨停＋涨停＋普通阴线"
)

// t0DisplayRule 只负责列表展示命中，不参与股票池过滤或选股结果归档。
// 后续增加展示条件时，在 t0DisplayRules 中追加一项即可。
type t0DisplayRule struct {
	Name        string
	Match       func([]dailyBar) bool
	MatchResult func([]dailyBar, T0SelectionResult) bool
}

var t0DisplayRules = []t0DisplayRule{
	{
		Name:  displayRuleAnyLimitUpLimitDown,
		Match: matchesAnyLimitUpLimitDown,
	},
	{
		Name:        displayRuleRedKLimitDownT0,
		MatchResult: matchesRedKThenLimitDownT0,
	},
	{
		Name:  displayRuleLimitUpBearishTag,
		Match: matchesLimitUpAndBearishTag,
	},
	{
		Name:  displayRuleBullishZtZtPb,
		Match: matchesBullishZtZtPb,
	},
	{
		Name:        displayRuleZtZtBearishT0,
		MatchResult: matchesZtZtBearishT0,
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

// matchesRedKThenLimitDownT0 匹配最近三根历史K线：第一根不限，第二根红K，第三根跌停。
// 红K沿用形态统计口径：ZT、YX、SY、MY、DY；仅正式T0竞价结果（0.01%～3%）命中。
func matchesRedKThenLimitDownT0(hist []dailyBar, result T0SelectionResult) bool {
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
	if !isRedKType(redType) {
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

func isRedKType(barType candlepattern.BarType) bool {
	switch barType {
	case candlepattern.BarZT, candlepattern.BarYX, candlepattern.BarSY,
		candlepattern.BarMY, candlepattern.BarDY:
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

// matchesZtZtBearishT0 匹配两连涨停后的普通阴线。
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
	if !ok || closeRet < -2 || closeRet > 3 {
		return false
	}
	bodyDrop := (bearish.Open - bearish.Close) / secondLimitUp.Close * 100
	return bodyDrop > 0 && bodyDrop <= 8
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
		out[i].DisplayRuleHits = displayRuleHitsForResult(hist, out[i])
	}
	return out
}
