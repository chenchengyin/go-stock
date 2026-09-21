package t0reference

import "encoding/json"

// DefaultRuleDefinitions returns the initial rules used by the historical
// GOB backfill. Definitions are data, not matcher code, so new combinations
// can be added without changing the reference tables.
func DefaultRuleDefinitions() []RuleDefinition {
	entry := `{"field":"entry_gap","op":"between","min":0.01,"max":2.0,"inclusive":true}`
	return []RuleDefinition{
		{
			RuleKind:          "sequence",
			Name:              "YX|ZT|ZT",
			DefinitionVersion: "v1",
			ConditionJSON:     `{"sequence":["YX","ZT","ZT"]}`,
			EntryJSON:         entry,
			ManualRank:        100,
			MinSamples:        10,
		},
		{
			RuleKind:          "condition",
			Name:              "两连涨停-T1非一字-T1开盘≥3%",
			DefinitionVersion: "v1",
			ConditionJSON: `{"all":[
{"field":"t-2.close_type","op":"eq","value":"ZT"},
{"field":"t-1.close_type","op":"eq","value":"ZT"},
{"field":"t-1.is_one_word","op":"eq","value":false},
{"field":"t-1.open_ret","op":"gte","value":3.0}
]}`,
			EntryJSON:  entry,
			ManualRank: 10,
			MinSamples: 10,
		},
		{
			RuleKind:          "condition",
			Name:              "T-2振幅≥8%+T-1开盘≥8%",
			DefinitionVersion: "v1",
			ConditionJSON: `{"all":[
{"field":"t-2.amplitude","op":"gte","value":8.0},
{"field":"t-1.open_ret","op":"gte","value":8.0}
]}`,
			EntryJSON:  entry,
			ManualRank: 20,
			MinSamples: 10,
		},
		{
			RuleKind:          "condition",
			Name:              "T-2非一字+T-1开盘≥8%+T-1振幅<4%",
			DefinitionVersion: "v1",
			ConditionJSON: `{"all":[
{"field":"t-2.is_one_word","op":"eq","value":false},
{"field":"t-1.open_ret","op":"gte","value":8.0},
{"field":"t-1.amplitude","op":"lt","value":4.0}
]}`,
			EntryJSON:  entry,
			ManualRank: 30,
			MinSamples: 10,
		},
		{
			RuleKind:          "condition",
			Name:              "T-2振幅≥8%+T-1开盘≥8%+T-1振幅<4%",
			DefinitionVersion: "v1",
			ConditionJSON: `{"all":[
{"field":"t-2.amplitude","op":"gte","value":8.0},
{"field":"t-1.open_ret","op":"gte","value":8.0},
{"field":"t-1.amplitude","op":"lt","value":4.0}
]}`,
			EntryJSON:  entry,
			ManualRank: 40,
			MinSamples: 10,
		},
		{
			RuleKind:          "condition",
			Name:              "T-2低开+T-1开盘≥3%+T-1振幅≥8%",
			DefinitionVersion: "v1",
			ConditionJSON: `{"all":[
{"field":"t-2.open_ret","op":"lt","value":0.0},
{"field":"t-1.open_ret","op":"gte","value":3.0},
{"field":"t-1.amplitude","op":"gte","value":8.0}
]}`,
			EntryJSON:  entry,
			ManualRank: 50,
			MinSamples: 10,
			DeepRed:    true,
		},
		{
			RuleKind:          "sequence",
			Name:              "DYIN|YX|MYIN",
			DefinitionVersion: "v1",
			ConditionJSON:     `{"sequence":["DYIN","YX","MYIN"]}`,
			EntryJSON:         entry,
			ManualRank:        60,
			MinSamples:        5,
			DeepRed:           false,
		},
		{
			RuleKind:          "sequence",
			Name:              "ZT|DYIN|DT",
			DefinitionVersion: "v1",
			ConditionJSON:     `{"sequence":["ZT","DYIN","DT"]}`,
			EntryJSON:         entry,
			ManualRank:        70,
			MinSamples:        5,
			DeepRed:           false,
		},
	}
}

// RuleJSON is kept as a small helper for persistence adapters that need to
// validate or re-encode a definition before writing it to SQLite.
func RuleJSON(value any) (string, error) {
	encoded, err := json.Marshal(value)
	if err != nil {
		return "", err
	}
	return string(encoded), nil
}
