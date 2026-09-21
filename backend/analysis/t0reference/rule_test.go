package t0reference

import "testing"

func TestCompileRuleCanonicalizesAllConditionOrder(t *testing.T) {
	base := RuleDefinition{
		RuleKind:          "condition",
		DefinitionVersion: "v1",
		ConditionJSON:     `{"all":[{"field":"t-2.amplitude","op":"gte","value":8},{"field":"t-1.open_ret","op":"gte","value":8}]}`,
		EntryJSON:         `{"field":"entry_gap","op":"between","min":0.01,"max":2,"inclusive":true}`,
	}
	reordered := base
	reordered.ConditionJSON = `{"all":[{"field":"t-1.open_ret","op":"gte","value":8},{"field":"t-2.amplitude","op":"gte","value":8}]}`

	one, err := CompileRule(base)
	if err != nil {
		t.Fatal(err)
	}
	two, err := CompileRule(reordered)
	if err != nil {
		t.Fatal(err)
	}
	if one.RuleKey != two.RuleKey {
		t.Fatalf("equivalent rule keys differ: %q != %q", one.RuleKey, two.RuleKey)
	}
}

func TestMatchReferenceRulesUsesOneEntryForSequenceAndConditionRules(t *testing.T) {
	obs := testObservation(t)
	view := BuildPreT0View(obs)
	entry := EntryView{Gap: obs.EntryGap}
	rules := []RuleDefinition{
		{
			RuleKind:          "sequence",
			Name:              "普通三K",
			DefinitionVersion: "v1",
			ConditionJSON:     `{"sequence":["YX","ZT","ZT"]}`,
			EntryJSON:         `{"field":"entry_gap","op":"between","min":0.01,"max":2,"inclusive":true}`,
		},
		{
			RuleKind:          "condition",
			Name:              "接力数值",
			DefinitionVersion: "v1",
			ConditionJSON: `{"all":[
				{"field":"t-2.close_type","op":"eq","value":"ZT"},
				{"field":"t-1.close_type","op":"eq","value":"ZT"},
				{"field":"t-1.is_one_word","op":"eq","value":false},
				{"field":"t-2.amplitude","op":"gte","value":8},
				{"field":"t-1.open_ret","op":"gte","value":8}
			]}`,
			EntryJSON: `{"field":"entry_gap","op":"between","min":0.01,"max":2,"inclusive":true}`,
		},
	}

	compiled := make([]CompiledRule, 0, len(rules))
	for _, definition := range rules {
		rule, err := CompileRule(definition)
		if err != nil {
			t.Fatal(err)
		}
		compiled = append(compiled, rule)
	}
	hits := MatchReferenceRules(view, entry, compiled)
	if len(hits) != 2 {
		t.Fatalf("hit count = %d want 2: %#v", len(hits), hits)
	}
}

func TestEntryGapBetweenIsInclusive(t *testing.T) {
	definition := RuleDefinition{
		RuleKind:          "sequence",
		DefinitionVersion: "v1",
		ConditionJSON:     `{"sequence":["YX","ZT","ZT"]}`,
		EntryJSON:         `{"field":"entry_gap","op":"between","min":0.01,"max":2,"inclusive":true}`,
	}
	rule, err := CompileRule(definition)
	if err != nil {
		t.Fatal(err)
	}
	view := BuildPreT0View(testObservation(t))
	for _, test := range []struct {
		gap  float64
		want bool
	}{
		{gap: 0.01, want: true},
		{gap: 2.0, want: true},
		{gap: 0.0, want: false},
		{gap: 2.01, want: false},
	} {
		hits := MatchReferenceRules(view, EntryView{Gap: test.gap}, []CompiledRule{rule})
		if (len(hits) == 1) != test.want {
			t.Fatalf("gap %.2f hit=%v want %v", test.gap, len(hits) == 1, test.want)
		}
	}
}

func TestCompileRuleRejectsT0OutcomeField(t *testing.T) {
	_, err := CompileRule(RuleDefinition{
		RuleKind:          "condition",
		DefinitionVersion: "v1",
		ConditionJSON:     `{"field":"t0.close","op":"gt","value":0}`,
		EntryJSON:         `{"field":"entry_gap","op":"gte","value":0}`,
	})
	if err == nil {
		t.Fatal("T0 outcome field should be rejected")
	}
}

func TestCompileRuleRejectsUnknownFieldAndOperator(t *testing.T) {
	for _, condition := range []string{
		`{"field":"t-1.not_registered","op":"eq","value":1}`,
		`{"field":"t-1.open_ret","op":"contains","value":1}`,
	} {
		if _, err := CompileRule(RuleDefinition{
			RuleKind:          "condition",
			DefinitionVersion: "v1",
			ConditionJSON:     condition,
			EntryJSON:         `{"field":"entry_gap","op":"gte","value":0}`,
		}); err == nil {
			t.Fatalf("condition should be rejected: %s", condition)
		}
	}
}

func TestCompileRuleRejectsTrailingJSONValues(t *testing.T) {
	if _, err := CompileRule(RuleDefinition{
		RuleKind:          "sequence",
		DefinitionVersion: "v1",
		ConditionJSON:     `{"sequence":["YX","ZT","ZT"]} {"extra":true}`,
		EntryJSON:         `{"field":"entry_gap","op":"gte","value":0}`,
	}); err == nil {
		t.Fatal("trailing JSON value should be rejected")
	}
}

func testObservation(t *testing.T) Observation {
	t.Helper()
	obs, err := BuildObservation("600000", "2026-01-07", []BarSnapshot{
		{Date: "2026-01-02", PrevClose: 8.5, Open: 9.0, High: 9.1, Low: 8.4, Close: 9.0},
		{Date: "2026-01-05", PrevClose: 9.02, Open: 9.2, High: 10.2, Low: 9.0, Close: 10.0},
		{Date: "2026-01-06", PrevClose: 10.0, Open: 12.0, High: 12.2, Low: 11.9, Close: 11.2},
		{Date: "2026-01-07", PrevClose: 12.2, Open: 12.3, High: 12.6, Low: 12.1, Close: 12.5},
	}, Entry{Price: 12.3, Source: EntrySourceDailyOpen})
	if err != nil {
		t.Fatal(err)
	}
	return obs
}
