package t0reference

import (
	"math"
	"testing"
)

func TestAggregateReferenceStatsSeparatesStrictTargetAndLossRates(t *testing.T) {
	rule := compileTestSequenceRule(t)
	observations := []Observation{
		observationWithClose(t, "2026-01-07", 12.7), // target and strict win
		observationWithClose(t, "2026-01-08", 12.5), // strict win only
		observationWithClose(t, "2026-01-09", 12.3), // flat, not a strict win
		observationWithClose(t, "2026-01-12", 12.2), // loss
	}

	stat := AggregateReferenceStats(rule, observations, "all")
	if stat.SampleCount != 4 {
		t.Fatalf("sample count = %d want 4", stat.SampleCount)
	}
	if math.Abs(stat.ProfitWinRate-50) > 1e-9 {
		t.Fatalf("strict win rate = %.4f want 50", stat.ProfitWinRate)
	}
	if math.Abs(stat.TargetRate-25) > 1e-9 {
		t.Fatalf("target rate = %.4f want 25", stat.TargetRate)
	}
	if math.Abs(stat.LossRate-25) > 1e-9 {
		t.Fatalf("loss rate = %.4f want 25", stat.LossRate)
	}
	if stat.ResearchTier != ResearchTierInsufficient {
		t.Fatalf("tier = %q want %q", stat.ResearchTier, ResearchTierInsufficient)
	}
}

func TestResearchTierUsesSampleCountAndStrictWinRate(t *testing.T) {
	rule := compileTestSequenceRule(t)
	for _, test := range []struct {
		name string
		n    int
		wins int
		want string
	}{
		{name: "A boundary", n: 20, wins: 13, want: ResearchTierA},
		{name: "B", n: 10, wins: 7, want: ResearchTierB},
		{name: "normal", n: 20, wins: 12, want: ResearchTierNormal},
	} {
		t.Run(test.name, func(t *testing.T) {
			observations := make([]Observation, 0, test.n)
			for i := 0; i < test.n; i++ {
				close := 12.2
				if i < test.wins {
					close = 12.5
				}
				observations = append(observations,
					observationWithClose(t, "2026-01-07", close))
			}
			stat := AggregateReferenceStats(rule, observations, "all")
			if stat.ResearchTier != test.want {
				t.Fatalf("tier = %q want %q (stat=%+v)", stat.ResearchTier, test.want, stat)
			}
		})
	}
}

func TestAggregateReferenceStatsFiltersYearPeriod(t *testing.T) {
	rule := compileTestSequenceRule(t)
	observations := []Observation{
		observationWithClose(t, "2025-01-07", 12.5),
		observationWithClose(t, "2026-01-07", 12.2),
		observationWithClose(t, "2026-01-08", 12.5),
	}

	stat := AggregateReferenceStats(rule, observations, "2026")
	if stat.SampleCount != 2 || stat.DateStart != "2026-01-07" || stat.DateEnd != "2026-01-08" {
		t.Fatalf("2026 stat = %+v", stat)
	}
}

func TestReferenceStatsAccumulatorKeepsAllAndAnnualPeriods(t *testing.T) {
	rule := compileTestSequenceRule(t)
	acc := NewReferenceStatsAccumulator(rule)
	acc.Add(observationWithClose(t, "2025-01-07", 12.5))
	acc.Add(observationWithClose(t, "2026-01-07", 12.2))

	all := acc.Stat("all")
	annual := acc.Stat("2026")
	if all.SampleCount != 2 || annual.SampleCount != 1 {
		t.Fatalf("all=%+v annual=%+v", all, annual)
	}
	if annual.DateStart != "2026-01-07" || annual.DateEnd != "2026-01-07" {
		t.Fatalf("annual dates=%q..%q", annual.DateStart, annual.DateEnd)
	}
}

func compileTestSequenceRule(t *testing.T) CompiledRule {
	t.Helper()
	rule, err := CompileRule(RuleDefinition{
		RuleKind:          "sequence",
		Name:              "test",
		DefinitionVersion: "v1",
		ConditionJSON:     `{"sequence":["YX","ZT","ZT"]}`,
		EntryJSON:         `{"field":"entry_gap","op":"gte","value":0}`,
	})
	if err != nil {
		t.Fatal(err)
	}
	return rule
}

func observationWithClose(t *testing.T, date string, close float64) Observation {
	t.Helper()
	obs := testObservation(t)
	obs.TradeDate = date
	obs.T0.Close = close
	obs.PnL = (close - obs.EntryPrice) / obs.EntryPrice * 100
	return obs
}
