package flutter_api

import (
	"math"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"time"

	"go-stock/backend/analysis/candlepattern"
)

// T0RedRuleHit belongs to an independently admitted Gob rule. Legacy T0
// reference statistics use a different (T0-close) exit and stay separate.
type T0RedRuleHit struct {
	Key             string             `json:"rule_key"`
	Version         int                `json:"rule_version"`
	Name            string             `json:"name"`
	Description     string             `json:"description,omitempty"`
	ExitHorizon     string             `json:"exit_horizon"`
	EntrySource     string             `json:"entry_source"`
	EntryOpen       float64            `json:"entry_open"`
	EntryClose      float64            `json:"entry_close,omitempty"`
	TargetPrice     float64            `json:"target_price,omitempty"`
	TargetReturnPct float64            `json:"target_return_pct,omitempty"`
	StatsStatus     string             `json:"stats_status"`
	Stats           *T0RedRuleStatView `json:"stats,omitempty"`
}

func gobRedT0CloseTargetPrice(entryClose float64) float64 {
	if entryClose <= 0 {
		return 0
	}
	return math.Ceil(entryClose*1.025*100-1e-9) / 100
}

func gobRedT0CloseTargetReached(price, target float64) bool {
	return price > 0 && target > 0 && price+1e-9 >= target
}

type redGobRule struct {
	Key                string
	Version            int
	Name               string
	Description        string
	MatchHistory       func([]dailyBar) bool
	MatchOpen          func(open, prevClose float64) bool
	ExcludeLimitDown   bool
	HistoryWindowDays  int
	ExitHorizon        string
	TargetReturnPct    float64
	RequireT0BearClose bool
}

const legacyGobZTztPbV1Description = "T-3、T-2收盘涨停，T-1为破板；T-1振幅=(最高-最低)/T-2收盘为6%～10%（含边界），下影=(min(开盘,收盘)-最低)/T-2收盘≤1%。若T-1实体跌幅≤-5%，仅保留T-1收盘相对T-2收盘涨幅<1%或3%～4%（含3%、不含4%）。建议T0开盘涨幅0%≤T0开盘涨幅<5%作为买入参考，不是筛选门槛：历史日线开盘代理样本中T0收盘高于开盘18/22=81.8%，样本较少，不代表实时竞价效果，也不是红策T+1真赚率；红策真赚率按T+1收盘计算。"

const legacyGobZTztPbV2Description = "历史v2规则：T-3、T-2收盘涨停，T-1破板；T-1振幅6%～10%、下影≤1%。若T-1按开盘价计算的实体跌幅≥5%，仅保留收盘较T-2收盘涨幅<1%或≥3%且<4%的样本。T0开盘缺口<−2%或≥0%，排除跌停开盘；以T0开盘为入场参考，按T+1收盘评判。本版本允许深度低开，与当前0%～3%的v3规则不同。"

const gobRedT1StatsDescription = "\n\n统计口径\n以T0开盘价为入场参考，按下一交易日（T+1）收盘评判。收益=(T+1收盘/T0开盘−1)×100%；真赚率指收益>0%，达标率指收益≥2.5%（包含2.5%）。历史日线开盘为入场代理，样本统计不代表实际成交。"

const gobRedBearClassificationDescription = "\n\n阴线分类\n实体跌幅按(开盘−收盘)/前收盘×100%计算：中阴>2.5%且≤6%，大阴>6%。K线先按涨停、跌停或破板判断，其余再按实体大小划分。"

const gobRedSmallBearHistoryDescription = "形态条件\nT-2收盘涨停；T-1收盘低于开盘，且较T-2收盘下跌0%～2%（包含两端，可与昨收持平）。下影长度=收盘−最低，不能超过阴线实体长度=开盘−收盘。\nT-2、T-1收盘分别严格高于各自5日均价（MA5），T-1收盘严格高于60日均价（MA60）；等于均线不算。这里按收盘跌幅筛选，阴线实体大小不限。"

type gobRedMatchStatus struct {
	MissingHistory int
	MissingOpen    int
	UnknownLimit   int
}

var gobRedPriorCacheDate = verifiedGobPriorCacheDate

// The latest complete preceding Gob is an independent trading-date witness
// for a preopen cache. Longer gaps may be holidays or missing cache days;
// without a calendar we conservatively leave the rule unconfirmed.
func verifiedGobPriorCacheDate(tradeDate string) string {
	dir := filepath.Join(t0CacheRootPath, "t0", "daily")
	entries, err := os.ReadDir(dir)
	if err != nil {
		return ""
	}
	latest := ""
	const prefix, suffix = "t0_daily_cache_", ".gob"
	for _, entry := range entries {
		name := entry.Name()
		if entry.IsDir() || !strings.HasPrefix(name, prefix) || !strings.HasSuffix(name, suffix) {
			continue
		}
		date := strings.TrimSuffix(strings.TrimPrefix(name, prefix), suffix)
		if date < tradeDate && date > latest {
			latest = date
		}
	}
	if latest == "" {
		return ""
	}
	tradeTime, tradeErr := time.Parse("2006-01-02", tradeDate)
	priorTime, priorErr := time.Parse("2006-01-02", latest)
	if tradeErr != nil || priorErr != nil || tradeTime.Sub(priorTime) > 72*time.Hour {
		return ""
	}
	cache, err := readGobRedReplayCache(filepath.Join(dir, prefix+latest+suffix), latest)
	if err != nil || cache.DataSchemaVersion != t0DailyCacheSchemaVersion || !cache.StockPoolComplete {
		return ""
	}
	return latest
}

func latestGobPriorDate(tradeDate string, daily map[string][]dailyBar) string {
	latest := ""
	for _, bars := range daily {
		for i := len(bars) - 1; i >= 0; i-- {
			if bars[i].Date < tradeDate {
				if bars[i].Date > latest {
					latest = bars[i].Date
				}
				break
			}
		}
	}
	return latest
}

type gobRedRuleContext struct {
	priorDates []string
}

func buildGobRedRuleContext(tradeDate string, daily map[string][]dailyBar) gobRedRuleContext {
	dateSet := make(map[string]struct{})
	for _, bars := range daily {
		for _, bar := range bars {
			if bar.Date != "" && bar.Date < tradeDate {
				dateSet[bar.Date] = struct{}{}
			}
		}
	}
	dates := make([]string, 0, len(dateSet))
	for date := range dateSet {
		dates = append(dates, date)
	}
	sort.Strings(dates)
	if len(dates) > 7 {
		dates = dates[len(dates)-7:]
	}
	return gobRedRuleContext{priorDates: dates}
}

func gobRedRuleHistoryMatches(rule redGobRule, hist []dailyBar, ctx gobRedRuleContext) bool {
	window := rule.HistoryWindowDays
	if window > 0 {
		if len(ctx.priorDates) < window || len(hist) < window {
			return false
		}
		expected := ctx.priorDates[len(ctx.priorDates)-window:]
		actual := hist[len(hist)-window:]
		for i := range expected {
			if actual[i].Date != expected[i] {
				return false
			}
		}
	}
	return rule.MatchHistory != nil && rule.MatchHistory(hist)
}

func gobRedRuleExitHorizon(rule redGobRule) string {
	if rule.ExitHorizon != "" {
		return rule.ExitHorizon
	}
	return "T+1_close"
}

func approvedGobRedRules() []redGobRule {
	return []redGobRule{
		{
			Key: "gob_myin_dt_dt_v1", Version: 1, Name: "中阴→跌停→跌停后低开",
			Description: "形态条件\nT-3为中阴，T-2、T-1连续收盘跌停。最近一次收盘涨停位于T-2～T-10（包含两端），且T-1收盘严格低于该涨停日收盘；不能跳过最近涨停去借用更早的锚点。" +
				gobRedBearClassificationDescription + "\n\nT0开盘\n较T-1收盘低开，−9.85%<开盘涨幅<−3%，两端均不包含；跌停开盘不入选。盘前只展示候选，竞价确认后才入选。" + gobRedT1StatsDescription,
			MatchHistory: func(hist []dailyBar) bool { return matchGobPullbackHistory(hist, candlepattern.BarDT) },
			MatchOpen: func(open, prevClose float64) bool {
				return open > prevClose*(1-9.85/100) && open < prevClose*(1-3.0/100)
			},
			ExcludeLimitDown: true,
		},
		{
			Key: "gob_myin_myin_dt_v1", Version: 1, Name: "中阴→中阴→跌停后低开",
			Description: "形态条件\nT-3、T-2均为中阴，T-1收盘跌停。最近一次收盘涨停位于T-2～T-10（包含两端），且T-1收盘严格低于该涨停日收盘；不能跳过最近涨停去借用更早的锚点。" +
				gobRedBearClassificationDescription + "\n\nT0开盘\n较T-1收盘低开，−9.85%<开盘涨幅<−3%，两端均不包含；跌停开盘不入选。盘前只展示候选，竞价确认后才入选。" + gobRedT1StatsDescription,
			MatchHistory: func(hist []dailyBar) bool { return matchGobPullbackHistory(hist, candlepattern.BarMYIN) },
			MatchOpen: func(open, prevClose float64) bool {
				return open > prevClose*(1-9.85/100) && open < prevClose*(1-3.0/100)
			},
			ExcludeLimitDown: true,
		},
		{
			Key: "gob_dyin_myin_myin_t1_body_le_430_v1", Version: 1,
			Name: "大阴→中阴→中阴（T-1实体≤-4.3%，A2）",
			Description: "形态条件\nT-3为大阴，T-2、T-1均为中阴。另要求T-1实体跌幅=(T-1开盘−T-1收盘)/T-1开盘×100%至少4.3%（包含4.3%）；此处以开盘为分母，与阴线分类不同。\n属于A2池：近7个交易日有收盘涨停记忆，T-1成交额至少5亿元（包含5亿）。" +
				gobRedBearClassificationDescription + "\n\nT0开盘\n允许低开、平开或高开，不设涨幅区间；跌停开盘不入选。盘前只展示候选，竞价确认后才入选。" + gobRedT1StatsDescription,
			MatchHistory:     matchGobDyinMyinMyinA2BodyCutoff,
			MatchOpen:        func(open, prevClose float64) bool { return true },
			ExcludeLimitDown: true,
		},
		{
			Key: "gob_zt_zt_pb_t1_amplitude_6_10_shadow_le_1_v3", Version: 3,
			Name: "涨停→涨停→破板（T-1振幅6%～10%、下影≤1%）",
			Description: "形态条件\nT-3、T-2连续收盘涨停；T-1涨停破板，收阳或收阴均可。\nT-1振幅=(最高−最低)/T-2收盘×100%，须在6%～10%内（包含两端）；下影比例=(开盘与收盘中较低者−最低)/T-2收盘×100%，须≤1%（包含1%）。\n若T-1实体跌幅=(开盘−收盘)/开盘×100%≥5%，还要求T-1收盘较T-2收盘涨幅<1%，或≥3%且<4%（包含3%、不含4%）。" +
				"\n\nT0开盘\n较T-1收盘涨幅为0%～3%，包含两端：允许平开，低开或高开超过3%不入选。盘前只展示候选，竞价确认后才入选。" + gobRedT1StatsDescription,
			MatchHistory: matchGobZTztPbT1Filters,
			MatchOpen: func(open, prevClose float64) bool {
				if open <= 0 || prevClose <= 0 {
					return false
				}
				return open >= prevClose && open <= prevClose*1.03+1e-9
			},
			ExcludeLimitDown: true,
		},
		{
			Key: "gob_zt_t1_bear_close_m2_0_shadow_le_body_ma5_ma60_gap_001_3_v1", Version: 1,
			Name: "涨停→收跌0%～2%的阴线（下影≤实体）",
			Description: gobRedSmallBearHistoryDescription +
				"\n\nT0开盘\n较T-1收盘上涨0.01%～3%，包含两端；平开不入选。盘前只展示候选，竞价确认后才入选。" + gobRedT1StatsDescription,
			MatchHistory: matchGobZTSmallBearishT1Filters,
			MatchOpen: func(open, prevClose float64) bool {
				if open <= 0 || prevClose <= 0 {
					return false
				}
				gap := (open/prevClose - 1) * 100
				return gap >= 0.01 && gap <= 3
			},
			HistoryWindowDays: 3,
		},
		{
			Key: "gob_zt_t1_bear_open_close_body_4combos_v1", Version: 1,
			Name: "涨停→阴线四组组合",
			Description: "形态条件\nT-2收盘涨停，T-1收盘低于开盘，并满足下面任意一组。这里不要求分类为中阴线。\nT-1开盘涨幅、收盘涨幅均相对T-2收盘计算；实体跌幅=(T-1开盘−T-1收盘)/T-2收盘×100%。\n第1组：开盘≥2%且<4%；收盘≥−5%且<−2%；实体≥5%且<8%。\n第2组：开盘≥4%且<6%；收盘≥−2%且<0%；实体≥5%且<8%。\n第3组：开盘≥2%且<4%；收盘≥−2%且<0%；实体>0%且<3%。\n第4组：开盘≥6%且<10%；收盘≥0%且<2%；实体≥5%且<8%。\n各组下限包含、上限不包含；第3组实体的0%也不包含。" +
				"\n\nT0开盘\n较T-1收盘上涨0.01%～3%，包含两端；平开不入选。盘前只展示候选，竞价确认后才入选。" + gobRedT1StatsDescription,
			MatchHistory: matchGobZTBearOpenCloseBodyCombinations,
			MatchOpen: func(open, prevClose float64) bool {
				if open <= 0 || prevClose <= 0 {
					return false
				}
				gap := (open/prevClose - 1) * 100
				return gap >= 0.01 && gap <= 3
			},
			HistoryWindowDays: 3, ExitHorizon: "T+1_close",
		},
		{
			Key: "gob_zt_t1_long_bear_low_open_boll_rebound_v1", Version: 1,
			Name: "涨停→高开长阴→低开修复",
			Description: "形态条件\nT-2收盘涨停且不是一字板；布林20日Z值≥1.5且<3（包含1.5、不含3）。Z值=(T-2收盘−20日收盘均价)/20日收盘标准差，20日窗口截至T-2。\nT-1开盘较T-2收盘上涨至少2%，阴线实体跌幅=(开盘−收盘)/开盘×100%至少6%；两个下限均包含。" +
				"\n\nT0开盘\n较T-1收盘低开，−8%≤开盘涨幅<−2%（包含−8%、不含−2%）。盘前只展示候选，竞价确认后才入选。" +
				"\n\n统计口径\n以T0开盘价为入场参考，观察T0收盘：收盘高于开盘率指收益>0%，达标率指收益≥4%（包含4%）。这是当日价格表现，T0买入的A股不能当日卖出，也不是T+1收盘收益。历史日线开盘为入场代理，样本统计不代表实际成交。",
			MatchHistory: matchGobZTLongBearLowOpenBollRebound,
			MatchOpen: func(open, prevClose float64) bool {
				if open <= 0 || prevClose <= 0 {
					return false
				}
				gap := (open/prevClose - 1) * 100
				// OHLC is cent-priced; avoid admitting an exact -2% gap from float noise.
				return gap >= -8-1e-9 && gap < -2-1e-9
			},
			HistoryWindowDays: 3, ExitHorizon: "T0_close", TargetReturnPct: 4,
		},
		{
			Key: "gob_zt_t1_bear_t0_bear_close_entry_target_2_5_v1", Version: 1,
			Name: "涨停→收跌0%～2%的阴线→T0收阴收盘入场",
			Description: gobRedSmallBearHistoryDescription +
				"\n\nT0开盘与入场\n开盘较T-1收盘上涨0.01%～3%（包含两端，平开不入选）。还须T0最终收盘严格低于开盘，以T0收盘价为入场参考；盘前、尾盘候选均不代表已确认收阴。本组合需收盘确认，当前当日竞价名单不包含它。" +
				"\n\n统计口径\n目标价为T0收盘价×1.025，向上取整到分。目标触及率按T+1最高价≥目标价计算；T+1现价达到目标时提示卖出，不自动下单。真赚率另按T0收盘到T+1收盘收益>0%计算。触及目标不代表实际成交，也不代表T+1收盘仍盈利。",
			MatchHistory: matchGobZTSmallBearishT1Filters,
			MatchOpen: func(open, prevClose float64) bool {
				if open <= 0 || prevClose <= 0 {
					return false
				}
				gap := (open/prevClose - 1) * 100
				return gap >= 0.01 && gap <= 3
			},
			HistoryWindowDays: 3, ExitHorizon: "T+1_target_2_5", RequireT0BearClose: true,
		},
		{
			Key: "gob_t7_drop_le_22_25_gap_m4_m2_t1_amp_ge_7_1_t0_high_v2", Version: 2,
			Name: "七日深跌后低开（T−1振幅≥7.1%）",
			Description: "形态条件\nT-1收盘较T-7收盘累计下跌至少22.25%（包含22.25%），按(T-1收盘/T-7收盘−1)×100%≤−22.25%判断；七个交易日日线须逐日对齐。\nT-1振幅=(T-1最高−T-1最低)/T-2收盘×100%至少7.1%（包含7.1%）。" +
				"\n\nT0开盘\n较T-1收盘低开，−4%≤开盘涨幅<−2%（包含−4%、不含−2%）。盘前只展示候选，竞价确认后才入选。" +
				"\n\n统计口径\n以T0开盘价为入场参考，观察T0盘中最高价。浮盈触及率指最高浮盈>0%；目标触及率指最高浮盈>2.5%（不包含2.5%）。这是盘中最大浮盈触及，不能称为已实现的真赚率。T0买入的A股不能当日卖出，历史日线最高价也不代表实际成交或落袋收益。",
			MatchHistory: matchGobT7CloseDropAndT1Amplitude,
			MatchOpen: func(open, prevClose float64) bool {
				if open <= 0 || prevClose <= 0 {
					return false
				}
				gap := (open/prevClose - 1) * 100
				return gap >= -4 && gap < -2
			},
			HistoryWindowDays: 7, ExitHorizon: "T0_high",
		},
	}
}

func matchGobZTSmallBearishT1Filters(hist []dailyBar) bool {
	n := len(hist)
	if n < 60 {
		return false
	}
	t2, t1 := hist[n-2], hist[n-1]
	if !isCloseLimitUpDay(hist[n-3].Close, t2, t0LimitUpCloseRet) ||
		t2.Close <= 0 || t1.Open <= 0 || t1.High <= 0 || t1.Low <= 0 || t1.Close <= 0 ||
		t1.Open <= t1.Close || t1.Low > t1.Close || t1.High < t1.Open {
		return false
	}
	closeReturn := (t1.Close/t2.Close - 1) * 100
	if closeReturn < -2 || closeReturn > 0 || t1.Close-t1.Low > t1.Open-t1.Close {
		return false
	}
	return gobCloseAboveSMA(hist, n-2, 5) &&
		gobCloseAboveSMA(hist, n-1, 5) &&
		gobCloseAboveSMA(hist, n-1, 60)
}

func matchGobZTBearOpenCloseBodyCombinations(hist []dailyBar) bool {
	n := len(hist)
	if n < 3 {
		return false
	}
	t2, t1 := hist[n-2], hist[n-1]
	if !isCloseLimitUpDay(hist[n-3].Close, t2, t0LimitUpCloseRet) ||
		t2.Close <= 0 || t1.Open <= 0 || t1.High <= 0 || t1.Low <= 0 || t1.Close <= 0 ||
		t1.Open <= t1.Close || t1.Low > t1.Close || t1.High < t1.Open {
		return false
	}

	openRet := (t1.Open/t2.Close - 1) * 100
	closeRet := (t1.Close/t2.Close - 1) * 100
	body := (t1.Open - t1.Close) / t2.Close * 100
	return (openRet >= 2 && openRet < 4 && closeRet >= -5 && closeRet < -2 && body >= 5 && body < 8) ||
		(openRet >= 4 && openRet < 6 && closeRet >= -2 && closeRet < 0 && body >= 5 && body < 8) ||
		(openRet >= 2 && openRet < 4 && closeRet >= -2 && closeRet < 0 && body > 0 && body < 3) ||
		(openRet >= 6 && openRet < 10 && closeRet >= 0 && closeRet < 2 && body >= 5 && body < 8)
}

func matchGobZTLongBearLowOpenBollRebound(hist []dailyBar) bool {
	n := len(hist)
	if n < 21 {
		return false
	}
	t2, t1 := hist[n-2], hist[n-1]
	if !isCloseLimitUpDay(hist[n-3].Close, t2, t0LimitUpCloseRet) ||
		t2.High <= t2.Low || t2.Close <= 0 || t1.Open <= 0 || t1.Close <= 0 ||
		(t1.Open/t2.Close-1)*100 < 2 || (t1.Open-t1.Close)/t1.Open*100 < 6 {
		return false
	}
	mean := 0.0
	for _, bar := range hist[n-21 : n-1] {
		if bar.Close <= 0 {
			return false
		}
		mean += bar.Close
	}
	mean /= 20
	variance := 0.0
	for _, bar := range hist[n-21 : n-1] {
		diff := bar.Close - mean
		variance += diff * diff
	}
	std := math.Sqrt(variance / 20)
	if std <= 0 {
		return false
	}
	z := (t2.Close - mean) / std
	return z >= 1.5 && z < 3
}

func gobCloseAboveSMA(hist []dailyBar, end, period int) bool {
	if period <= 0 || end < period-1 || end >= len(hist) {
		return false
	}
	sum := 0.0
	for _, bar := range hist[end-period+1 : end+1] {
		if bar.Close <= 0 {
			return false
		}
		sum += bar.Close
	}
	return hist[end].Close > sum/float64(period)
}

func matchGobT7CloseDrop(hist []dailyBar) bool {
	if len(hist) < 7 {
		return false
	}
	t7Close, t1Close := hist[len(hist)-7].Close, hist[len(hist)-1].Close
	if t7Close <= 0 || t1Close <= 0 {
		return false
	}
	return (t1Close/t7Close-1)*100 <= -22.25
}

func matchGobT7CloseDropAndT1Amplitude(hist []dailyBar) bool {
	if !matchGobT7CloseDrop(hist) {
		return false
	}
	t1 := hist[len(hist)-1]
	t2Close := hist[len(hist)-2].Close
	if t2Close <= 0 || t1.High <= 0 || t1.Low <= 0 || t1.High < t1.Low {
		return false
	}
	return (t1.High-t1.Low)/t2Close*100 >= 7.1
}

func currentGobRedRuleSetVersion() string {
	rules := approvedGobRedRules()
	parts := make([]string, 0, len(rules))
	for _, rule := range rules {
		parts = append(parts, rule.Key+"@"+strconv.Itoa(rule.Version))
	}
	return strings.Join(parts, "|")
}

func matchGobZTztPbT1Filters(hist []dailyBar) bool {
	n := len(hist)
	if n < 4 || hist[n-2].Close <= 0 {
		return false
	}
	t3, t2, t1 := hist[n-3], hist[n-2], hist[n-1]
	if !isCloseLimitUpDay(hist[n-4].Close, t3, t0LimitUpCloseRet) ||
		!isCloseLimitUpDay(t3.Close, t2, t0LimitUpCloseRet) ||
		!isBrokenLimitUpDay(t2.Close, t1) ||
		t1.Open <= 0 || t1.High <= 0 || t1.Low <= 0 || t1.Close <= 0 ||
		t1.Low > math.Min(t1.Open, t1.Close) || t1.High < math.Max(t1.Open, t1.Close) {
		return false
	}
	base := t2.Close
	amplitude := (t1.High - t1.Low) / base * 100
	lowerShadow := (math.Min(t1.Open, t1.Close) - t1.Low) / base * 100
	if amplitude < 6 || amplitude > 10 || lowerShadow > 1 {
		return false
	}
	bodyReturn := (t1.Close/t1.Open - 1) * 100
	if bodyReturn <= -5 {
		closeReturn := (t1.Close/base - 1) * 100
		return closeReturn < 1 || (closeReturn >= 3 && closeReturn < 4)
	}
	return true
}

func matchGobDyinMyinMyinA2BodyCutoff(hist []dailyBar) bool {
	n := len(hist)
	if n < 5 || hist[n-1].Open <= 0 || hist[n-1].Close <= 0 ||
		classifyDisplayBar(hist[n-4].Close, hist[n-3]) != candlepattern.BarDYIN ||
		classifyDisplayBar(hist[n-3].Close, hist[n-2]) != candlepattern.BarMYIN ||
		classifyDisplayBar(hist[n-2].Close, hist[n-1]) != candlepattern.BarMYIN ||
		hist[n-1].Close > hist[n-1].Open*0.957 {
		return false
	}
	a2Bars := make([]candlepattern.DailyBar, len(hist))
	for i, bar := range hist {
		a2Bars[i] = candlepattern.DailyBar{
			Date: bar.Date, Open: bar.Open, High: bar.High, Low: bar.Low,
			Close: bar.Close, AmountYi: bar.AmountYi,
		}
	}
	return candlepattern.InA2Pool(a2Bars)
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
	priorDate := latestGobPriorDate(tradeDate, daily)
	ruleContext := buildGobRedRuleContext(tradeDate, daily)
	now := time.Now().In(chinaLocation())
	for _, stock := range stocks {
		bars := daily[stock.ShortCode]
		end := 0
		for end < len(bars) && bars[end].Date < tradeDate {
			end++
		}
		hist := bars[:end]
		if len(hist) < 5 || hist[len(hist)-1].Date != priorDate {
			status.MissingHistory++
			continue
		}
		matchedHistory := make([]redGobRule, 0, len(rules))
		for _, rule := range rules {
			if gobRedRuleHistoryMatches(rule, hist, ruleContext) {
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
			hitEntrySource := entrySource
			entryClose := 0.0
			if rule.RequireT0BearClose {
				if quote.Close <= 0 || quote.Close >= quote.Open ||
					(entrySource == "realtime_open" && tradeDate == now.Format("2006-01-02") && now.Hour()*60+now.Minute() < 15*60+5) {
					continue
				}
				entryClose = quote.Close
				if entrySource == "daily_open" {
					hitEntrySource = "daily_close"
				} else {
					hitEntrySource = "realtime_close"
				}
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
				Key: rule.Key, Version: rule.Version, Name: rule.Name, Description: rule.Description,
				ExitHorizon: gobRedRuleExitHorizon(rule), EntrySource: hitEntrySource,
				EntryOpen: quote.Open, EntryClose: entryClose,
				TargetPrice: gobRedT0CloseTargetPrice(entryClose), TargetReturnPct: rule.TargetReturnPct,
				StatsStatus: "pending",
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

func buildGobRedPreopenCandidates(
	tradeDate string, stocks []t0Stock, daily map[string][]dailyBar, rules []redGobRule,
) []T0SelectionResult {
	results := make([]T0SelectionResult, 0)
	priorDate := latestGobPriorDate(tradeDate, daily)
	ruleContext := buildGobRedRuleContext(tradeDate, daily)
	for _, stock := range stocks {
		bars := daily[stock.ShortCode]
		end := 0
		for end < len(bars) && bars[end].Date < tradeDate {
			end++
		}
		hist := bars[:end]
		if len(hist) < 5 || hist[len(hist)-1].Close <= 0 || hist[len(hist)-1].Date != priorDate {
			continue
		}
		var hits []T0RedRuleHit
		for _, rule := range rules {
			if gobRedRuleHistoryMatches(rule, hist, ruleContext) {
				entrySource, statsStatus := "pending_open", "preopen"
				if rule.RequireT0BearClose {
					entrySource, statsStatus = "pending_close", "preclose"
				}
				hits = append(hits, T0RedRuleHit{
					Key: rule.Key, Version: rule.Version, Name: rule.Name, Description: rule.Description,
					ExitHorizon: gobRedRuleExitHorizon(rule), EntrySource: entrySource,
					TargetReturnPct: rule.TargetReturnPct, StatsStatus: statsStatus,
				})
			}
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
			PrevClose: round2(hist[len(hist)-1].Close),
			AmountYi:  round2(hist[len(hist)-1].AmountYi), RedRuleHits: hits,
		}
		for _, hit := range hits {
			result.DisplayRuleHits = append(result.DisplayRuleHits, hit.Name)
		}
		results = append(results, result)
	}
	return results
}
