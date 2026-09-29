package flutter_api

import (
	"math"
	"strings"

	"go-stock/backend/analysis/candlepattern"
)

// T0RedRuleHit belongs to an independently admitted Gob rule. Legacy T0
// reference statistics use a different (T0-close) exit and stay separate.
type T0RedRuleHit struct {
	Key         string  `json:"rule_key"`
	Version     int     `json:"rule_version"`
	Name        string  `json:"name"`
	ExitHorizon string  `json:"exit_horizon"`
	EntrySource string  `json:"entry_source"`
	EntryOpen   float64 `json:"entry_open"`
	StatsStatus string  `json:"stats_status"`
}

type redGobRule struct {
	Key              string
	Version          int
	Name             string
	MatchHistory     func([]dailyBar) bool
	MatchOpen        func(open, prevClose float64) bool
	ExcludeLimitDown bool
}

type gobRedMatchStatus struct {
	MissingHistory int
	MissingOpen    int
	UnknownLimit   int
}

func approvedGobRedRules() []redGobRule {
	return []redGobRule{
		{
			Key: "gob_myin_dt_dt_v1", Version: 1, Name: "中阴→跌停→跌停后低开",
			MatchHistory: func(hist []dailyBar) bool { return matchGobPullbackHistory(hist, candlepattern.BarDT) },
			MatchOpen: func(open, prevClose float64) bool {
				return open > prevClose*(1-9.85/100) && open < prevClose*(1-3.0/100)
			},
			ExcludeLimitDown: true,
		},
		{
			Key: "gob_myin_myin_dt_v1", Version: 1, Name: "中阴→中阴→跌停后低开",
			MatchHistory: func(hist []dailyBar) bool { return matchGobPullbackHistory(hist, candlepattern.BarMYIN) },
			MatchOpen: func(open, prevClose float64) bool {
				return open > prevClose*(1-9.85/100) && open < prevClose*(1-3.0/100)
			},
			ExcludeLimitDown: true,
		},
	}
}

func matchGobPullbackHistory(hist []dailyBar, second candlepattern.BarType) bool {
	n := len(hist)
	if n < 5 || hist[n-2].Close <= 0 || hist[n-1].Close <= 0 ||
		classifyDisplayBar(hist[n-4].Close, hist[n-3]) != candlepattern.BarMYIN ||
		classifyDisplayBar(hist[n-3].Close, hist[n-2]) != second ||
		classifyDisplayBar(hist[n-2].Close, hist[n-1]) != candlepattern.BarDT {
		return false
	}
	for offset := 1; offset <= 10; offset++ {
		i := n - offset
		if i < 1 {
			break
		}
		if isCloseLimitUpDay(hist[i-1].Close, hist[i], t0LimitUpCloseRet) {
			return offset >= 2 && hist[n-1].Close < hist[i].Close
		}
	}
	return false
}

// gobRedOpenLimitState uses the stock's date-specific main-board risk-warning
// limit. SSE/SZSE changed risk-warning stocks from 5% to 10% on 2026-07-06.
// Unknown boards are not guessed into a tradable opening.
func gobRedOpenLimitState(stock t0Stock, tradeDate string, prevClose, open float64) string {
	if prevClose <= 0 || open <= 0 || tradeDate == "" {
		return "unknown"
	}
	code := stock.ShortCode
	if !((strings.HasPrefix(stock.Code, "sh.") &&
		(strings.HasPrefix(code, "600") || strings.HasPrefix(code, "601") ||
			strings.HasPrefix(code, "603") || strings.HasPrefix(code, "605"))) ||
		(strings.HasPrefix(stock.Code, "sz.") &&
			(strings.HasPrefix(code, "000") || strings.HasPrefix(code, "001") ||
				strings.HasPrefix(code, "002") || strings.HasPrefix(code, "003")))) {
		return "unknown"
	}
	name := strings.ToUpper(strings.TrimSpace(stock.Name))
	limitPct := 10.0
	if (strings.HasPrefix(name, "ST") || strings.HasPrefix(name, "*ST")) && tradeDate < "2026-07-06" {
		limitPct = 5
	}
	limitPrice := math.Round(prevClose*(1-limitPct/100)*100) / 100
	if open <= limitPrice+1e-9 {
		return "limit_down"
	}
	return "tradable"
}

func matchGobRedRules(
	tradeDate string,
	stocks []t0Stock,
	daily map[string][]dailyBar,
	opens map[string]t0Realtime,
	entrySource string,
	rules []redGobRule,
) ([]T0SelectionResult, gobRedMatchStatus) {
	results := make([]T0SelectionResult, 0)
	var status gobRedMatchStatus
	for _, stock := range stocks {
		bars := daily[stock.ShortCode]
		end := 0
		for end < len(bars) && bars[end].Date < tradeDate {
			end++
		}
		hist := bars[:end]
		if len(hist) < 5 {
			status.MissingHistory++
			continue
		}
		matchedHistory := make([]redGobRule, 0, len(rules))
		for _, rule := range rules {
			if rule.MatchHistory(hist) {
				matchedHistory = append(matchedHistory, rule)
			}
		}
		if len(matchedHistory) == 0 {
			continue
		}
		quote, hasOpen := opens[stock.ShortCode]
		prevClose := hist[len(hist)-1].Close
		if quote.PrevClose > 0 {
			prevClose = quote.PrevClose
		}
		if !hasOpen || quote.Open <= 0 || prevClose <= 0 || hist[len(hist)-1].Close <= 0 {
			status.MissingOpen++
			continue
		}
		gap := (quote.Open/prevClose - 1) * 100
		var hits []T0RedRuleHit
		for _, rule := range matchedHistory {
			if !rule.MatchOpen(quote.Open, prevClose) {
				continue
			}
			if rule.ExcludeLimitDown {
				limitState := gobRedOpenLimitState(stock, tradeDate, prevClose, quote.Open)
				if limitState == "unknown" {
					status.UnknownLimit++
					continue
				}
				if limitState == "limit_down" {
					continue
				}
			}
			hits = append(hits, T0RedRuleHit{
				Key: rule.Key, Version: rule.Version, Name: rule.Name,
				ExitHorizon: "T+1_close", EntrySource: entrySource,
				EntryOpen: quote.Open, StatsStatus: "pending",
			})
		}
		if len(hits) == 0 {
			continue
		}
		userCode := stock.ShortCode + ".XSHE"
		if strings.HasPrefix(stock.Code, "sh.") {
			userCode = stock.ShortCode + ".XSHG"
		}
		result := T0SelectionResult{
			Time: tradeDate, StockCode: userCode, StockName: stock.Name,
			OpenGap: round2(gap), PrevClose: round2(prevClose),
			AmountYi: round2(hist[len(hist)-1].AmountYi), RedRuleHits: hits,
		}
		for _, hit := range hits {
			result.DisplayRuleHits = append(result.DisplayRuleHits, hit.Name)
		}
		results = append(results, result)
	}
	return results, status
}
