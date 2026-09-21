package t0reference

import (
	"sort"
	"strings"
)

const (
	TargetPnLPercent = 2.5

	ResearchTierA            = "A"
	ResearchTierB            = "B"
	ResearchTierNormal       = "normal"
	ResearchTierInsufficient = "insufficient"
)

type RuleStat struct {
	RuleKey       string
	PeriodKey     string
	DateStart     string
	DateEnd       string
	SampleCount   int
	ProfitWinRate float64
	TargetRate    float64
	LossRate      float64
	AvgPnL        float64
	MedianPnL     float64
	ResearchTier  string
}

// ReferenceStatsAccumulator aggregates one rule while a backfill streams
// cache files one trading day at a time. It keeps the exact PnL samples needed
// for medians but avoids retaining every full Observation in memory.
type ReferenceStatsAccumulator struct {
	rule    CompiledRule
	periods map[string]*statsBucket
}

type statsBucket struct {
	dateStart  string
	dateEnd    string
	pnls       []float64
	strictWins int
	targets    int
	losses     int
}

func NewReferenceStatsAccumulator(rule CompiledRule) *ReferenceStatsAccumulator {
	return &ReferenceStatsAccumulator{
		rule:    rule,
		periods: map[string]*statsBucket{"all": &statsBucket{}},
	}
}

// Add evaluates the rule against the observation and adds matching outcomes
// to the all-time and calendar-year buckets.
func (a *ReferenceStatsAccumulator) Add(observation Observation) {
	if a == nil || observation.EntryPrice <= 0 || !observationHasThreeBars(observation) {
		return
	}
	view := BuildPreT0View(observation)
	if !matchesCompiledRule(a.rule, view, EntryView{Gap: observation.EntryGap}) {
		return
	}
	a.add("all", observation)
	if len(observation.TradeDate) >= 4 {
		a.add(observation.TradeDate[:4], observation)
	}
}

func (a *ReferenceStatsAccumulator) add(periodKey string, observation Observation) {
	bucket := a.periods[periodKey]
	if bucket == nil {
		bucket = &statsBucket{}
		a.periods[periodKey] = bucket
	}
	pnl := (observation.T0.Close - observation.EntryPrice) / observation.EntryPrice * 100
	bucket.pnls = append(bucket.pnls, pnl)
	if observation.T0.Close > observation.EntryPrice {
		bucket.strictWins++
	}
	if pnl >= TargetPnLPercent {
		bucket.targets++
	}
	if pnl < 0 {
		bucket.losses++
	}
	if bucket.dateStart == "" || observation.TradeDate < bucket.dateStart {
		bucket.dateStart = observation.TradeDate
	}
	if observation.TradeDate > bucket.dateEnd {
		bucket.dateEnd = observation.TradeDate
	}
}

func (a *ReferenceStatsAccumulator) Stat(periodKey string) RuleStat {
	if periodKey == "" {
		periodKey = "all"
	}
	if a == nil {
		return RuleStat{PeriodKey: periodKey, ResearchTier: ResearchTierInsufficient}
	}
	stat := RuleStat{RuleKey: a.rule.RuleKey, PeriodKey: periodKey}
	bucket := a.periods[periodKey]
	if bucket == nil || len(bucket.pnls) == 0 {
		stat.ResearchTier = ResearchTierInsufficient
		return stat
	}
	denom := float64(len(bucket.pnls))
	stat.DateStart = bucket.dateStart
	stat.DateEnd = bucket.dateEnd
	stat.SampleCount = len(bucket.pnls)
	stat.ProfitWinRate = float64(bucket.strictWins) / denom * 100
	stat.TargetRate = float64(bucket.targets) / denom * 100
	stat.LossRate = float64(bucket.losses) / denom * 100
	stat.AvgPnL = meanPnL(bucket.pnls)
	stat.MedianPnL = medianPnL(bucket.pnls)
	stat.ResearchTier = researchTier(stat.SampleCount, stat.ProfitWinRate)
	return stat
}

func AggregateReferenceStats(rule CompiledRule, observations []Observation, periodKey string) RuleStat {
	stat := RuleStat{RuleKey: rule.RuleKey, PeriodKey: periodKey}
	pnls := make([]float64, 0, len(observations))
	strictWins, targets, losses := 0, 0, 0
	for _, observation := range observations {
		if periodKey != "" && periodKey != "all" && !strings.HasPrefix(observation.TradeDate, periodKey) {
			continue
		}
		if observation.EntryPrice <= 0 || !observationHasThreeBars(observation) {
			continue
		}
		view := BuildPreT0View(observation)
		if !matchesCompiledRule(rule, view, EntryView{Gap: observation.EntryGap}) {
			continue
		}

		pnl := (observation.T0.Close - observation.EntryPrice) / observation.EntryPrice * 100
		stat.SampleCount++
		pnls = append(pnls, pnl)
		if observation.T0.Close > observation.EntryPrice {
			strictWins++
		}
		if pnl >= TargetPnLPercent {
			targets++
		}
		if pnl < 0 {
			losses++
		}
		if stat.DateStart == "" || observation.TradeDate < stat.DateStart {
			stat.DateStart = observation.TradeDate
		}
		if observation.TradeDate > stat.DateEnd {
			stat.DateEnd = observation.TradeDate
		}
	}

	if stat.SampleCount == 0 {
		stat.ResearchTier = ResearchTierInsufficient
		return stat
	}
	denom := float64(stat.SampleCount)
	stat.ProfitWinRate = float64(strictWins) / denom * 100
	stat.TargetRate = float64(targets) / denom * 100
	stat.LossRate = float64(losses) / denom * 100
	stat.AvgPnL = meanPnL(pnls)
	stat.MedianPnL = medianPnL(pnls)
	stat.ResearchTier = researchTier(stat.SampleCount, stat.ProfitWinRate)
	return stat
}

func matchesCompiledRule(rule CompiledRule, view PreT0View, entry EntryView) bool {
	return evaluateCondition(rule.condition, view) && evaluateEntry(rule.entry, entry)
}

func observationHasThreeBars(observation Observation) bool {
	for _, offset := range []int{-3, -2, -1} {
		if _, ok := observation.Bars[offset]; !ok {
			return false
		}
	}
	return true
}

func researchTier(sampleCount int, profitWinRate float64) string {
	switch {
	case sampleCount >= 20 && profitWinRate >= 65:
		return ResearchTierA
	case sampleCount >= 10 && profitWinRate >= 65:
		return ResearchTierB
	case sampleCount < 10:
		return ResearchTierInsufficient
	default:
		return ResearchTierNormal
	}
}

func meanPnL(values []float64) float64 {
	if len(values) == 0 {
		return 0
	}
	var total float64
	for _, value := range values {
		total += value
	}
	return total / float64(len(values))
}

func medianPnL(values []float64) float64 {
	if len(values) == 0 {
		return 0
	}
	ordered := append([]float64(nil), values...)
	sort.Float64s(ordered)
	middle := len(ordered) / 2
	if len(ordered)%2 == 0 {
		return (ordered[middle-1] + ordered[middle]) / 2
	}
	return ordered[middle]
}
