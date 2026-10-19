package flutter_api

import "strings"

const optimizedRedPrefix = "gob_zt_bear_opt_"

// Each optimized rule retains the original four-combination gate. They are
// independent, overlapping admissions, never mutually exclusive categories.
func optimizedGobRedRules() []redGobRule {
	base := "形态条件\nT-2收盘涨停，T-1收阴，且满足原四组之一。T-1开盘/收盘涨幅、实体均以T-2收盘为分母：\n第1组：开盘[2%,4%)，收盘[-5%,-2%)，实体[5%,8%)；第2组：开盘[4%,6%)，收盘[-2%,0%)，实体[5%,8%)；第3组：开盘[2%,4%)，收盘[-2%,0%)，实体(0%,3%)；第4组：开盘[6%,10%)，收盘[0%,2%)，实体[5%,8%)。\n前日指涨停前一天T-3；涨停实体=(收盘−开盘)/前收盘；前日阳实体与振幅均以T-4收盘为分母。\n"
	type variant struct {
		key, name, extra string
		match            func([]dailyBar) bool
		maxGap           float64
	}
	upperBody := func(h []dailyBar) bool {
		n := len(h)
		p, z := h[n-3], h[n-2]
		return (z.Close-z.Open)/p.Close*100 >= 8-1e-9 && p.Close+1e-9 >= (p.High+p.Low)/2
	}
	bull := func(h []dailyBar) bool { n := len(h); return (h[n-3].Close-h[n-3].Open)/h[n-4].Close*100 >= .5-1e-9 }
	narrow := func(h []dailyBar) bool { n := len(h); return (h[n-3].High-h[n-3].Low)/h[n-4].Close*100 <= 8+1e-9 }
	variants := []variant{
		{"upper_body8", "板阴·长实上半", "涨停实体≥8%；前日收盘≥(最高+最低)/2，包含中点。", upperBody, 3},
		{"upper_body8_amp8", "板阴·窄幅增强", "涨停实体≥8%；前日收在振幅上半区，且前日振幅≤8%。", func(h []dailyBar) bool { return upperBody(h) && narrow(h) }, 3},
		{"upper_body8_no_prior_limit", "板阴·非连板", "涨停实体≥8%；前日收在振幅上半区，且前日未收盘涨停（按现规则涨幅≥9.89%识别）。", func(h []dailyBar) bool {
			n := len(h)
			return upperBody(h) && !isCloseLimitUpDay(h[n-4].Close, h[n-3], t0LimitUpCloseRet)
		}, 3},
		{"upper_body8_gap15", "板阴·小高开", "涨停实体≥8%；前日收在振幅上半区；T0高开上限进一步收紧至1.5%。", upperBody, 1.5},
		{"upper_body8_ma20", "板阴·均线过滤", "涨停实体≥8%；前日收在振幅上半区；涨停收盘相对MA20偏离≤20%，MA20包含涨停日、历史不足不入选。", func(h []dailyBar) bool {
			n := len(h)
			if !upperBody(h) || n < 21 {
				return false
			}
			sum := 0.
			for _, b := range h[n-21 : n-1] {
				if b.Close <= 0 {
					return false
				}
				sum += b.Close
			}
			return (h[n-2].Close/(sum/20)-1)*100 <= 20+1e-9
		}, 3},
		{"prior_bull_amp8", "板阴·前阳窄幅", "前日阳实体≥0.5%，且前日振幅≤8%。", func(h []dailyBar) bool { return bull(h) && narrow(h) }, 3},
		{"prior_bull_body8", "板阴·前阳长实", "涨停实体≥8%，且前日阳实体≥0.5%。", func(h []dailyBar) bool {
			n := len(h)
			return bull(h) && (h[n-2].Close-h[n-2].Open)/h[n-3].Close*100 >= 8-1e-9
		}, 3},
	}
	out := make([]redGobRule, 0, len(variants))
	for _, v := range variants {
		v := v
		out = append(out, redGobRule{Key: optimizedRedPrefix + v.key + "_v1", Version: 1, Name: v.name, Description: base + "\n附加条件\n" + v.extra + "\n\nT0开盘\n高开0.01%～" + map[bool]string{true: "1.5", false: "3"}[v.maxGap == 1.5] + "%，两端包含；平开不入选。允许同时命中多组，命中组数不代表独立信号数。" + gobRedT1StatsDescription, HistoryWindowDays: 4, ExitHorizon: "T+1_close", MatchHistory: func(h []dailyBar) bool {
			if len(h) < 4 || !matchGobZTBearOpenCloseBodyCombinations(h) {
				return false
			}
			n := len(h)
			for _, b := range h[n-4 : n-2] {
				if b.Close <= 0 || b.Open <= 0 || b.Low <= 0 || b.High < b.Low || b.High < b.Open || b.High < b.Close || b.Low > b.Open || b.Low > b.Close {
					return false
				}
			}
			return v.match(h)
		}, MatchOpen: func(open, prev float64) bool {
			if open <= 0 || prev <= 0 {
				return false
			}
			gap := (open/prev - 1) * 100
			return gap >= .01-1e-9 && gap <= v.maxGap+1e-9
		}})
	}
	return out
}
func isOptimizedRedRule(key string) bool { return strings.HasPrefix(key, optimizedRedPrefix) }

func enrichGobRedHistoryOptions(options []gobRedRuleHistoryOption, runtimes []t0ReferenceRuleRuntime) []gobRedRuleHistoryOption {
	out := append([]gobRedRuleHistoryOption(nil), options...)
	results := make([]T0SelectionResult, len(out))
	for i, o := range out {
		if o.RuleSource != redHistorySourceGob {
			continue
		}
		horizon := "T+1_close"
		entry := "daily_open"
		if rule, ok := gobRedRuleByKey(o.RuleKey); ok && rule.Version == o.RuleVersion {
			horizon = gobRedRuleExitHorizon(rule)
			if rule.RequireT0BearClose {
				entry = "daily_close"
			}
		}
		results[i].RedRuleHits = []T0RedRuleHit{{Key: o.RuleKey, Version: o.RuleVersion, EntrySource: entry, ExitHorizon: horizon, StatsStatus: "pending"}}
	}
	results = enrichGobRedRuleStats(results)
	for i := range out {
		if len(results[i].RedRuleHits) > 0 {
			out[i].Stats = results[i].RedRuleHits[0].Stats
		}
		if out[i].RuleSource == redHistorySourceReference {
			for _, rt := range runtimes {
				if rt.rule.RuleKey == out[i].RuleKey && rt.sampleCount > 0 {
					out[i].Stats = &T0RedRuleStatView{SampleCount: rt.sampleCount, ProfitWinRate: rt.strictWinRate, TargetRate: rt.targetRate, EntrySource: "daily_open", ExitHorizon: "T0_close"}
				}
			}
		}
	}
	return out
}
