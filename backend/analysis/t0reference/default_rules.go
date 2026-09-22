package t0reference

import "encoding/json"

const (
	RuleNameStrongContinuation          = "强势连板"
	RuleNameLimitUpLimitDownReversal    = "涨跌停反转"
	RuleNameAnyLimitUpLimitDown         = "任意K线＋涨停＋跌停"
	RuleNameMediumYangLimitDown         = "中阳/大阳＋跌停后的开盘竞价（T0开盘涨幅0.01%～0.75%）"
	RuleNameBullishZtZtPb               = "涨停＋涨停＋阳线破板"
	RuleNameZtZtBearish                 = "涨停＋涨停＋普通阴线"
	RuleNameRemovedTwoLimitUpNonOneWord = "两连涨停-T1非一字-T1开盘≥3%"
	RuleNameLegacyMediumYangLimitDown   = "中阳及以上＋跌停后的开盘竞价"
)

// IsDeprecatedRuleName identifies reference labels that were removed from
// the product logic but whose historical rows and statistics are retained.
func IsDeprecatedRuleName(name string) bool {
	return name == RuleNameRemovedTwoLimitUpNonOneWord || name == RuleNameLegacyMediumYangLimitDown
}

// DeprecatedRuleNames returns labels that should stay in the database for
// historical auditability but must no longer participate in runtime matching.
func DeprecatedRuleNames() []string {
	return []string{RuleNameRemovedTwoLimitUpNonOneWord, RuleNameLegacyMediumYangLimitDown}
}

// DefaultRuleDefinitions returns the initial rules used by the historical
// GOB backfill. Definitions are data, not matcher code, so new combinations
// can be added without changing the reference tables.
func DefaultRuleDefinitions() []RuleDefinition {
	entry := `{"field":"entry_gap","op":"between","min":0.01,"max":2.0,"inclusive":true}`
	return []RuleDefinition{
		{
			RuleKind:          "condition",
			Name:              RuleNameStrongContinuation,
			DefinitionVersion: "v1",
			ConditionJSON: `{"all":[
{"field":"t-2.close_type","op":"eq","value":"ZT"},
{"field":"t-1.close_type","op":"eq","value":"ZT"},
{"field":"t-2.is_one_word","op":"eq","value":false},
{"field":"t-1.is_one_word","op":"eq","value":false},
{"field":"t-1.open_ret","op":"gte","value":7.0}
]}`,
			EntryJSON:  `{"field":"entry_gap","op":"between","min":0.01,"max":3.0,"inclusive":true}`,
			ManualRank: 5, MinSamples: 10, RedEntry: true,
		},
		{
			RuleKind:          "condition",
			Name:              RuleNameLimitUpLimitDownReversal,
			DefinitionVersion: "v1",
			ConditionJSON: `{"all":[
{"field":"t-2.close_type","op":"eq","value":"ZT"},
{"field":"t-2.is_one_word","op":"eq","value":false},
{"field":"t-2.body_ret","op":"gte","value":5.0},
{"field":"t-1.close_type","op":"eq","value":"DT"}
]}`,
			EntryJSON:  `{"field":"entry_gap","op":"between","min":0.01,"max":3.0,"inclusive":true}`,
			ManualRank: 15, MinSamples: 10, RedEntry: true,
		},
		{
			RuleKind:          "condition",
			Name:              RuleNameAnyLimitUpLimitDown,
			DefinitionVersion: "v1",
			ConditionJSON: `{"all":[
{"field":"t-2.close_type","op":"eq","value":"ZT"},
{"field":"t-1.close_type","op":"eq","value":"DT"}
]}`,
			EntryJSON:  `{"field":"entry_gap","op":"between","min":0.01,"max":3.0,"inclusive":true}`,
			ManualRank: 20, MinSamples: 10, RedEntry: true,
		},
		{
			RuleKind:          "condition",
			Name:              RuleNameMediumYangLimitDown,
			DefinitionVersion: "v1",
			ConditionJSON: `{"all":[
{"any":[
{"field":"t-2.close_type","op":"eq","value":"MY"},
{"field":"t-2.close_type","op":"eq","value":"DY"}
]},
{"field":"t-1.close_type","op":"eq","value":"DT"}
]}`,
			EntryJSON:  `{"field":"entry_gap","op":"between","min":0.01,"max":0.75,"inclusive":true}`,
			ManualRank: 30, MinSamples: 10, RedEntry: true,
		},
		{
			RuleKind:          "condition",
			Name:              RuleNameBullishZtZtPb,
			DefinitionVersion: "v1",
			ConditionJSON: `{"all":[
{"field":"t-3.close_type","op":"eq","value":"ZT"},
{"field":"t-2.close_type","op":"eq","value":"ZT"},
{"field":"t-1.close_type","op":"eq","value":"PB"},
{"field":"t-1.close_gt_open","op":"eq","value":true}
]}`,
			EntryJSON:  `{"field":"entry_gap","op":"between","min":0.01,"max":3.0,"inclusive":true}`,
			ManualRank: 40, MinSamples: 10, RedEntry: true,
		},
		{
			RuleKind:          "condition",
			Name:              RuleNameZtZtBearish,
			DefinitionVersion: "v1",
			ConditionJSON: `{"all":[
{"field":"t-3.close_type","op":"eq","value":"ZT"},
{"field":"t-2.close_type","op":"eq","value":"ZT"},
{"field":"t-1.is_bearish","op":"eq","value":true},
{"field":"t-1.close_vs_t-2_ret","op":"between","min":-2.0,"max":3.0,"inclusive":true},
{"field":"t-1.body_drop_vs_t-2_close","op":"gt","value":0.0},
{"field":"t-1.body_drop_vs_t-2_close","op":"lt","value":8.0}
]}`,
			EntryJSON:  `{"field":"entry_gap","op":"between","min":0.01,"max":3.0,"inclusive":true}`,
			ManualRank: 50, MinSamples: 10, RedEntry: true,
		},
		{
			RuleKind:          "sequence",
			Name:              "YX|ZT|ZT",
			DefinitionVersion: "v1",
			ConditionJSON:     `{"sequence":["YX","ZT","ZT"]}`,
			EntryJSON:         entry,
			ManualRank:        120,
			MinSamples:        10,
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
			ManualRank: 60,
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
			ManualRank: 70,
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
			ManualRank: 80,
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
			ManualRank: 90,
			MinSamples: 10,
			DeepRed:    true,
		},
		{
			RuleKind:          "sequence",
			Name:              "DYIN|YX|MYIN",
			DefinitionVersion: "v1",
			ConditionJSON:     `{"sequence":["DYIN","YX","MYIN"]}`,
			EntryJSON:         entry,
			ManualRank:        100,
			MinSamples:        5,
			DeepRed:           false,
		},
		{
			RuleKind:          "sequence",
			Name:              "ZT|DYIN|DT",
			DefinitionVersion: "v1",
			ConditionJSON:     `{"sequence":["ZT","DYIN","DT"]}`,
			EntryJSON:         entry,
			ManualRank:        110,
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
