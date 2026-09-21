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
	SampleCount   int     `json:"sample_count"`
	ManualRank    int     `json:"manual_rank"`
	DeepRed       bool    `json:"deep_red,omitempty"`
}

type t0ReferenceRuleRuntime struct {
	rule          t0reference.CompiledRule
	researchTier  string
	strictWinRate float64
	sampleCount   int
	deepRed       bool
}

func loadT0ReferenceRuleRuntimes() []t0ReferenceRuleRuntime {
	if db.Dao == nil {
		return nil
	}
	var rows []models.T0ReferenceRule
	if err := db.Dao.Where("enabled = ?", true).Order("manual_rank, id").Find(&rows).Error; err != nil {
		return nil
	}
	runtimes := make([]t0ReferenceRuleRuntime, 0, len(rows))
	for _, row := range rows {
		compiled, err := t0reference.CompileRule(t0reference.RuleDefinition{
			RuleKind: row.RuleKind, Name: row.Name, DefinitionVersion: row.DefinitionVersion,
			ConditionJSON: row.ConditionJSON, EntryJSON: row.EntryJSON,
			ManualRank: row.ManualRank, MinSamples: row.MinSamples, DeepRed: row.DeepRed,
		})
		if err != nil || compiled.RuleKey != row.RuleKey {
			continue
		}
		var stat models.T0ReferenceRuleStat
		statErr := db.Dao.Where("rule_id = ? AND period_key = ?", row.ID, "all").
			Order("updated_at DESC, id DESC").First(&stat).Error
		tier := t0reference.ResearchTierInsufficient
		winRate := 0.0
		sampleCount := 0
		if statErr == nil {
			tier = stat.ResearchTier
			if tier == "" {
				tier = t0reference.ResearchTierInsufficient
			}
			winRate = stat.ProfitWinRate
			sampleCount = stat.SampleCount
		}
		runtimes = append(runtimes, t0ReferenceRuleRuntime{
			rule: compiled, researchTier: tier, strictWinRate: winRate, sampleCount: sampleCount,
			deepRed: row.DeepRed,
		})
	}
	return runtimes
}

func enrichT0ReferenceResult(result *T0SelectionResult, hist []dailyBar, runtimes []t0ReferenceRuleRuntime) {
	if result == nil || len(runtimes) == 0 || len(hist) < 4 {
		return
	}
	view, ok := preT0ViewFromDaily(hist)
	if !ok {
		return
	}
	compiled := make([]t0reference.CompiledRule, 0, len(runtimes))
	byKey := make(map[string]t0ReferenceRuleRuntime, len(runtimes))
	for _, runtime := range runtimes {
		compiled = append(compiled, runtime.rule)
		byKey[runtime.rule.RuleKey] = runtime
	}
	hits := t0reference.MatchReferenceRules(view, t0reference.EntryView{Gap: result.OpenGap}, compiled)
	if len(hits) == 0 {
		return
	}
	result.T0ReferenceHits = make([]T0ReferenceHit, 0, len(hits))
	for _, hit := range hits {
		runtime := byKey[hit.RuleKey]
		result.T0ReferenceHits = append(result.T0ReferenceHits, T0ReferenceHit{
			RuleKey: hit.RuleKey, Name: hit.Name, ResearchTier: runtime.researchTier,
			StrictWinRate: runtime.strictWinRate, SampleCount: runtime.sampleCount,
			ManualRank: runtime.rule.Definition.ManualRank, DeepRed: runtime.deepRed,
		})
	}
	hasDeepRed := false
	for _, hit := range result.T0ReferenceHits {
		if hit.DeepRed {
			hasDeepRed = true
			break
		}
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
	if hasDeepRed {
		result.StrongDisplayRuleHit = true
	}
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
