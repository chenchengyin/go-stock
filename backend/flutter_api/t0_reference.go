package flutter_api

import (
	"sort"
	"strings"

	"go-stock/backend/analysis/t0reference"
	"go-stock/backend/db"
	"go-stock/backend/models"
)

// T0ReferenceHit is the optional, explainable result of the unified T0
// reference matcher. It never replaces legacy pattern fields or BuySignal.
type T0ReferenceHit struct {
	RuleKey       string  `json:"rule_key"`
	Name          string  `json:"name"`
	ResearchTier  string  `json:"research_tier"`
	StrictWinRate float64 `json:"strict_win_rate"`
	TargetRate    float64 `json:"target_rate"`
	EarnRate      float64 `json:"earn_rate"`
	SampleCount   int     `json:"sample_count"`
	ManualRank    int     `json:"manual_rank"`
	DeepRed       bool    `json:"deep_red,omitempty"` // legacy metadata; not a red-entry trigger
}

type t0ReferenceRuleRuntime struct {
	rule          t0reference.CompiledRule
	researchTier  string
	strictWinRate float64
	targetRate    float64
	earnRate      float64
	sampleCount   int
	deepRed       bool
	redEntry      bool
}

func loadT0ReferenceRuleRuntimes() []t0ReferenceRuleRuntime {
	// Default definitions are also loaded when the SQLite registry has not
	// been synchronized yet. This keeps the red-entry matcher available on a
	// fresh deployment; the stats remain insufficient until backfill runs.
	defaults := t0reference.DefaultRuleDefinitions()
	runtimes := make([]t0ReferenceRuleRuntime, 0, len(defaults))
	byKey := make(map[string]int, len(defaults))
	for _, definition := range defaults {
		compiled, err := t0reference.CompileRule(definition)
		if err != nil {
			continue
		}
		byKey[compiled.RuleKey] = len(runtimes)
		runtimes = append(runtimes, t0ReferenceRuleRuntime{
			rule: compiled, researchTier: t0reference.ResearchTierInsufficient,
			redEntry: definition.RedEntry,
		})
	}
	if db.Dao == nil {
		return runtimes
	}
	var rows []models.T0ReferenceRule
	if err := db.Dao.Order("manual_rank, id").Find(&rows).Error; err != nil {
		return runtimes
	}
	for _, row := range rows {
		if t0reference.IsDeprecatedRuleName(row.Name) {
			continue
		}
		if !row.Enabled {
			if compiledIndex, ok := byKey[row.RuleKey]; ok {
				runtimes[compiledIndex].rule = t0reference.CompiledRule{}
			}
			continue
		}
		compiled, err := t0reference.CompileRule(t0reference.RuleDefinition{
			RuleKind: row.RuleKind, Name: row.Name, DefinitionVersion: row.DefinitionVersion,
			ConditionJSON: row.ConditionJSON, EntryJSON: row.EntryJSON,
			ManualRank: row.ManualRank, MinSamples: row.MinSamples,
			DeepRed: row.DeepRed, RedEntry: row.RedEntry,
		})
		if err != nil || compiled.RuleKey != row.RuleKey {
			continue
		}
		var stat models.T0ReferenceRuleStat
		statErr := db.Dao.Where("rule_id = ? AND period_key = ?", row.ID, "all").
			Order("updated_at DESC, id DESC").First(&stat).Error
		tier := t0reference.ResearchTierInsufficient
		winRate := 0.0
		targetRate := 0.0
		earnRate := 0.0
		sampleCount := 0
		if statErr == nil {
			tier = stat.ResearchTier
			if tier == "" {
				tier = t0reference.ResearchTierInsufficient
			}
			winRate = stat.ProfitWinRate
			targetRate = stat.TargetRate
			sampleCount = stat.SampleCount
			if sampleCount > 0 {
				earnRate = 100 - stat.LossRate
			}
		}
		runtime := t0ReferenceRuleRuntime{
			rule: compiled, researchTier: tier, strictWinRate: winRate, sampleCount: sampleCount,
			targetRate: targetRate, earnRate: earnRate, deepRed: row.DeepRed,
			redEntry: row.RedEntry,
		}
		if compiledIndex, ok := byKey[row.RuleKey]; ok {
			runtimes[compiledIndex] = runtime
			continue
		}
		runtimes = append(runtimes, runtime)
	}
	return runtimes
}

func matchingT0ReferenceRuntimes(
	hist []dailyBar,
	result T0SelectionResult,
	runtimes []t0ReferenceRuleRuntime,
) []t0ReferenceRuleRuntime {
	if len(runtimes) == 0 || len(hist) < 4 {
		return nil
	}
	view, ok := preT0ViewFromDaily(hist)
	if !ok {
		return nil
	}
	compiled := make([]t0reference.CompiledRule, 0, len(runtimes))
	for _, runtime := range runtimes {
		if runtime.rule.RuleKey == "" {
			continue
		}
		compiled = append(compiled, runtime.rule)
	}
	hits := t0reference.MatchReferenceRules(view, t0reference.EntryView{Gap: result.OpenGap}, compiled)
	if len(hits) == 0 {
		return nil
	}
	byKey := make(map[string]t0ReferenceRuleRuntime, len(runtimes))
	for _, runtime := range runtimes {
		if runtime.rule.RuleKey != "" {
			byKey[runtime.rule.RuleKey] = runtime
		}
	}
	matched := make([]t0ReferenceRuleRuntime, 0, len(hits))
	for _, hit := range hits {
		if runtime, ok := byKey[hit.RuleKey]; ok {
			matched = append(matched, runtime)
		}
	}
	return matched
}

func matchingRedEntryReferenceRuntimes(
	hist []dailyBar,
	result T0SelectionResult,
	runtimes []t0ReferenceRuleRuntime,
) []t0ReferenceRuleRuntime {
	matched := matchingT0ReferenceRuntimes(hist, result, runtimes)
	redEntry := matched[:0]
	for _, runtime := range matched {
		if runtime.redEntry {
			redEntry = append(redEntry, runtime)
		}
	}
	return redEntry
}

func enrichT0ReferenceResult(result *T0SelectionResult, hist []dailyBar, runtimes []t0ReferenceRuleRuntime) {
	if result == nil {
		return
	}
	result.T0ReferenceHits = nil
	result.T0ReferenceTier = ""
	result.T0ReferenceWinPct = 0
	result.T0ReferenceSamples = 0
	if len(runtimes) == 0 || len(hist) < 4 {
		return
	}
	matched := matchingT0ReferenceRuntimes(hist, *result, runtimes)
	if len(matched) == 0 {
		return
	}
	result.T0ReferenceHits = make([]T0ReferenceHit, 0, len(matched))
	for _, runtime := range matched {
		result.T0ReferenceHits = append(result.T0ReferenceHits, T0ReferenceHit{
			RuleKey: runtime.rule.RuleKey, Name: runtime.rule.Definition.Name, ResearchTier: runtime.researchTier,
			StrictWinRate: runtime.strictWinRate, SampleCount: runtime.sampleCount,
			TargetRate: runtime.targetRate, EarnRate: runtime.earnRate,
			ManualRank: runtime.rule.Definition.ManualRank, DeepRed: runtime.deepRed,
		})
	}
	sort.SliceStable(result.T0ReferenceHits, func(i, j int) bool {
		left, right := result.T0ReferenceHits[i], result.T0ReferenceHits[j]
		if referenceTierRank(left.ResearchTier) != referenceTierRank(right.ResearchTier) {
			return referenceTierRank(left.ResearchTier) < referenceTierRank(right.ResearchTier)
		}
		if left.ManualRank != right.ManualRank {
			return left.ManualRank < right.ManualRank
		}
		if left.StrictWinRate != right.StrictWinRate {
			return left.StrictWinRate > right.StrictWinRate
		}
		return left.RuleKey < right.RuleKey
	})
	if len(result.T0ReferenceHits) > 3 {
		result.T0ReferenceHits = result.T0ReferenceHits[:3]
	}
	best := result.T0ReferenceHits[0]
	result.T0ReferenceTier = best.ResearchTier
	result.T0ReferenceWinPct = best.StrictWinRate
	result.T0ReferenceSamples = best.SampleCount
}

func preT0ViewFromDaily(hist []dailyBar) (t0reference.PreT0View, bool) {
	if len(hist) < 4 {
		return t0reference.PreT0View{}, false
	}
	view := t0reference.PreT0View{Bars: make(map[int]t0reference.PreT0Bar, 3)}
	start := len(hist) - 3
	for i, offset := range []int{-3, -2, -1} {
		index := start + i
		prevClose := hist[index-1].Close
		if prevClose <= 0 {
			return t0reference.PreT0View{}, false
		}
		bar := t0reference.BarSnapshot{
			Date: hist[index].Date, PrevClose: prevClose, Open: hist[index].Open,
			High: hist[index].High, Low: hist[index].Low, Close: hist[index].Close,
			Volume: hist[index].Volume, AmountYi: hist[index].AmountYi,
		}
		view.Bars[offset] = buildReferencePreT0Bar(bar)
	}
	return view, true
}

func buildReferencePreT0Bar(bar t0reference.BarSnapshot) t0reference.PreT0Bar {
	// BuildPreT0View is intentionally the public derivation boundary. Use a
	// one-observation wrapper here because the daily selection path already has
	// the historical bars but not a persisted T0 fact row.
	view := t0reference.BuildPreT0View(t0reference.Observation{Bars: map[int]t0reference.BarSnapshot{-1: bar}})
	return view.Bars[-1]
}

func referenceTierRank(tier string) int {
	switch strings.TrimSpace(tier) {
	case t0reference.ResearchTierA:
		return 0
	case t0reference.ResearchTierB:
		return 1
	case t0reference.ResearchTierNormal:
		return 2
	default:
		return 3
	}
}
