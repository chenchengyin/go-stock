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

func TestDefaultRuleDefinitionsSelectOnlyPrimaryAmplitudeAsDeepRed(t *testing.T) {
	want := map[string]bool{
		"DYIN|YX|MYIN": false,
		"ZT|DYIN|DT":   false,
		"T-2低开+T-1开盘≥3%+T-1振幅≥8%":    true,
		"T-2振幅≥8%+T-1开盘≥8%+T-1振幅<4%": false,
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
