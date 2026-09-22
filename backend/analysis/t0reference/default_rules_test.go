package t0reference

import (
	"reflect"
	"testing"
)

func TestDefaultRuleDefinitionsCompile(t *testing.T) {
	rules := DefaultRuleDefinitions()
	if len(rules) < 5 {
		t.Fatalf("default rule count = %d, want at least 5", len(rules))
	}
	seen := make(map[string]bool, len(rules))
	for _, definition := range rules {
		compiled, err := CompileRule(definition)
		if err != nil {
			t.Fatalf("compile %q: %v", definition.Name, err)
		}
		if seen[compiled.RuleKey] {
			t.Fatalf("duplicate rule key for %q", definition.Name)
		}
		seen[compiled.RuleKey] = true
	}
}

func TestDefaultRedEntryDefinitionsIncludeAllSixRedPatterns(t *testing.T) {
	want := map[string]bool{
		RuleNameStrongContinuation:       true,
		RuleNameLimitUpLimitDownReversal: true,
		RuleNameAnyLimitUpLimitDown:      true,
		RuleNameMediumYangLimitDown:      true,
		RuleNameBullishZtZtPb:            true,
		RuleNameZtZtBearish:              true,
	}
	got := make(map[string]bool)
	for _, definition := range DefaultRuleDefinitions() {
		if _, ok := want[definition.Name]; !ok {
			continue
		}
		if !definition.RedEntry {
			t.Fatalf("rule %q must be a red-entry pattern", definition.Name)
		}
		got[definition.Name] = true
	}
	if len(got) != len(want) {
		t.Fatalf("red-entry custom patterns=%v want=%v", got, want)
	}
}

func TestDefaultRulesIncludeBuiltInRedPatternsWithoutDeepRed(t *testing.T) {
	want := map[string]bool{
		RuleNameAnyLimitUpLimitDown: true,
		RuleNameMediumYangLimitDown: true,
		RuleNameBullishZtZtPb:       true,
		RuleNameZtZtBearish:         true,
	}
	got := make(map[string]bool)
	for _, definition := range DefaultRuleDefinitions() {
		if _, ok := want[definition.Name]; !ok {
			continue
		}
		if !definition.RedEntry {
			t.Fatalf("red pattern %q must gate red entry", definition.Name)
		}
		if definition.DeepRed {
			t.Fatalf("display-only rule %q must not be marked deep red", definition.Name)
		}
		got[definition.Name] = true
	}
	if len(got) != len(want) {
		t.Fatalf("built-in red patterns=%v want=%v", got, want)
	}
}

func TestLimitUpLimitDownReversalRequiresFivePercentT2Body(t *testing.T) {
	var definition RuleDefinition
	found := false
	for _, candidate := range DefaultRuleDefinitions() {
		if candidate.Name == RuleNameLimitUpLimitDownReversal {
			definition = candidate
			found = true
			break
		}
	}
	if !found {
		t.Fatalf("default rule %q is missing", RuleNameLimitUpLimitDownReversal)
	}
	rule, err := CompileRule(definition)
	if err != nil {
		t.Fatal(err)
	}
	for _, test := range []struct {
		body float64
		want bool
	}{
		{body: 5.0, want: true},
		{body: 4.99, want: false},
	} {
		view := PreT0View{Bars: map[int]PreT0Bar{
			-2: {CloseType: "ZT", BodyRet: test.body},
			-1: {CloseType: "DT"},
		}}
		hits := MatchReferenceRules(view, EntryView{Gap: 1.0}, []CompiledRule{rule})
		if (len(hits) == 1) != test.want {
			t.Fatalf("body %.2f hit=%v want %v", test.body, len(hits) == 1, test.want)
		}
	}
}

func TestRemovedReferenceRuleIsNotReintroducedByDefaults(t *testing.T) {
	for _, definition := range DefaultRuleDefinitions() {
		if definition.Name == RuleNameRemovedTwoLimitUpNonOneWord {
			t.Fatalf("removed reference rule %q was reintroduced", definition.Name)
		}
	}
}

func TestLegacyMediumYangRuleIsDeprecated(t *testing.T) {
	if !IsDeprecatedRuleName("中阳及以上＋跌停后的开盘竞价") {
		t.Fatal("legacy broad medium-yang rule should be deprecated")
	}
}

func TestLegacyAmplitudeOpenReferenceRulesAreDeprecated(t *testing.T) {
	removed := []string{
		"T-2振幅≥8%+T-1开盘≥8%",
		"T-2非一字+T-1开盘≥8%+T-1振幅<4%",
		"T-2振幅≥8%+T-1开盘≥8%+T-1振幅<4%",
	}
	for _, name := range removed {
		if !IsDeprecatedRuleName(name) {
			t.Fatalf("legacy reference rule %q should be deprecated", name)
		}
		for _, definition := range DefaultRuleDefinitions() {
			if definition.Name == name {
				t.Fatalf("deprecated reference rule %q remains in defaults", name)
			}
		}
	}
}

func TestBuiltInRedDisplayRulesMatchTheirDefinedShapes(t *testing.T) {
	view := PreT0View{Bars: map[int]PreT0Bar{
		-3: {CloseType: "ZT", BarSnapshot: BarSnapshot{Open: 90, Close: 100}},
		-2: {CloseType: "ZT", BarSnapshot: BarSnapshot{Open: 100, Close: 110}},
		-1: {CloseType: "PB", BarSnapshot: BarSnapshot{Open: 110, Close: 111}},
	}}

	cases := []struct {
		name string
		view PreT0View
	}{
		{
			name: RuleNameAnyLimitUpLimitDown,
			view: PreT0View{Bars: map[int]PreT0Bar{
				-3: {CloseType: "YX"},
				-2: {CloseType: "ZT"},
				-1: {CloseType: "DT"},
			}},
		},
		{
			name: RuleNameMediumYangLimitDown,
			view: PreT0View{Bars: map[int]PreT0Bar{
				-3: {CloseType: "YX"},
				-2: {CloseType: "MY"},
				-1: {CloseType: "DT"},
			}},
		},
		{name: RuleNameBullishZtZtPb, view: view},
		{
			name: RuleNameZtZtBearish,
			view: PreT0View{Bars: map[int]PreT0Bar{
				-3: {CloseType: "ZT"},
				-2: {CloseType: "ZT", BarSnapshot: BarSnapshot{Close: 100}},
				-1: {BarSnapshot: BarSnapshot{Open: 103, Close: 101}},
			}},
		},
	}

	definitions := make(map[string]RuleDefinition)
	for _, definition := range DefaultRuleDefinitions() {
		definitions[definition.Name] = definition
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			definition, ok := definitions[tc.name]
			if !ok {
				t.Fatalf("definition %q is missing", tc.name)
			}
			rule, err := CompileRule(definition)
			if err != nil {
				t.Fatal(err)
			}
			hits := MatchReferenceRules(tc.view, EntryView{Gap: 0.5}, []CompiledRule{rule})
			if len(hits) != 1 {
				t.Fatalf("rule %q hits=%v", tc.name, hits)
			}
		})
	}

	definition := definitions[RuleNameMediumYangLimitDown]
	rule, err := CompileRule(definition)
	if err != nil {
		t.Fatal(err)
	}
	limitUpView := PreT0View{Bars: map[int]PreT0Bar{
		-3: {CloseType: "YX"},
		-2: {CloseType: "ZT"},
		-1: {CloseType: "DT"},
	}}
	if hits := MatchReferenceRules(limitUpView, EntryView{Gap: 0.5}, []CompiledRule{rule}); len(hits) != 0 {
		t.Fatalf("limit-up T-2 should not match optimized medium-yang rule: %v", hits)
	}
	mediumView := cases[1].view
	if hits := MatchReferenceRules(mediumView, EntryView{Gap: 0.76}, []CompiledRule{rule}); len(hits) != 0 {
		t.Fatalf("entry gap above 0.75 should not match optimized medium-yang rule: %v", hits)
	}
}

func TestDefaultRuleDefinitionsSelectOnlyPrimaryAmplitudeAsDeepRed(t *testing.T) {
	want := map[string]bool{
		"DYIN|YX|MYIN": false,
		"ZT|DYIN|DT":   false,
		"T-2低开+T-1开盘≥3%+T-1振幅≥8%": true,
	}
	got := make(map[string]bool)
	for _, definition := range DefaultRuleDefinitions() {
		wantDeepRed, selected := want[definition.Name]
		if !selected {
			continue
		}
		got[definition.Name] = true
		field := reflect.ValueOf(definition).FieldByName("DeepRed")
		if !field.IsValid() || field.Kind() != reflect.Bool || field.Bool() != wantDeepRed {
			t.Fatalf("pattern %q deep-red=%v want %v", definition.Name, field.Bool(), wantDeepRed)
		}
	}
	if len(got) != len(want) {
		t.Fatalf("selected deep-red patterns = %v want %v", got, want)
	}
}
