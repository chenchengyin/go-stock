package t0reference

import "encoding/json"

const redReferenceAdmissionDescription = "\n\n红策入选前提\n近7个交易日有收盘涨停，或T-1涨停破板；T-1成交额至少5亿元。"

const redReferenceStatsDescription = "\n\n统计口径\n以T0开盘价为入场参考，观察T0收盘：正收益率指收益>0%，不亏率指收益≥0%（包含持平），达标率指收益≥2.5%。这是当日价格表现，T0买入的A股不能当日卖出，也不是T+1收盘收益。历史日线开盘为入场代理，样本统计不代表实际成交。"

const (
	RuleNameStrongContinuation          = "强势连板"
	RuleNameLimitUpLimitDownReversal    = "涨跌停反转"
	RuleNameAnyLimitUpLimitDown         = "任意K线＋涨停＋跌停"
	RuleNameMediumYangLimitDown         = "中阳/大阳＋跌停后的开盘竞价（T0开盘涨幅0.01%～0.75%）"
	RuleNameBullishZtZtPb               = "涨停＋涨停＋阳线破板"
	RuleNameZtZtBearish                 = "涨停＋涨停＋普通阴线"
	RuleNameRemovedTwoLimitUpNonOneWord = "两连涨停-T1非一字-T1开盘≥3%"
	RuleNameLegacyMediumYangLimitDown   = "中阳及以上＋跌停后的开盘竞价"
	RuleNameLegacyT2AmplitudeT1Open     = "T-2振幅≥8%+T-1开盘≥8%"
	RuleNameLegacyT2NonOneWordT1Open    = "T-2非一字+T-1开盘≥8%+T-1振幅<4%"
	RuleNameLegacyT2AmplitudeT1Narrow   = "T-2振幅≥8%+T-1开盘≥8%+T-1振幅<4%"
)

// IsDeprecatedRuleName identifies reference labels that were removed from
// the product logic but whose historical rows and statistics are retained.
func IsDeprecatedRuleName(name string) bool {
	return name == RuleNameRemovedTwoLimitUpNonOneWord ||
		name == RuleNameLegacyMediumYangLimitDown ||
		name == RuleNameLegacyT2AmplitudeT1Open ||
		name == RuleNameLegacyT2NonOneWordT1Open ||
		name == RuleNameLegacyT2AmplitudeT1Narrow
}

// DeprecatedRuleNames returns labels that should stay in the database for
// historical auditability but must no longer participate in runtime matching.
func DeprecatedRuleNames() []string {
	return []string{
		RuleNameRemovedTwoLimitUpNonOneWord,
		RuleNameLegacyMediumYangLimitDown,
		RuleNameLegacyT2AmplitudeT1Open,
		RuleNameLegacyT2NonOneWordT1Open,
		RuleNameLegacyT2AmplitudeT1Narrow,
	}
}

// DefaultRuleDefinitions returns the initial rules used by the historical
// GOB backfill. Definitions are data, not matcher code, so new combinations
// can be added without changing the reference tables.
func DefaultRuleDefinitions() []RuleDefinition {
	entry := `{"field":"entry_gap","op":"between","min":0.01,"max":2.0,"inclusive":true}`
	return []RuleDefinition{
		{
			RuleKind: "condition",
			Name:     RuleNameStrongContinuation,
			Description: "形态条件\nT-2、T-1连续收盘涨停，两天都不是一字板。T-1开盘较T-2收盘上涨至少7%（包含7%）。" +
				redReferenceAdmissionDescription + "\n\nT0开盘\n较T-1收盘上涨0.01%～3%，包含两端；平开不入选。" + redReferenceStatsDescription,
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
			RuleKind: "condition",
			Name:     RuleNameLimitUpLimitDownReversal,
			Description: "形态条件\nT-2收盘涨停且不是一字板；阳线实体涨幅至少5%（包含5%），按(T-2收盘−T-2开盘)/T-3收盘×100%计算。T-1收盘跌停。" +
				redReferenceAdmissionDescription + "\n\nT0开盘\n较T-1收盘上涨0.01%～3%，包含两端；平开不入选。" + redReferenceStatsDescription,
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
			RuleKind: "condition",
			Name:     RuleNameAnyLimitUpLimitDown,
			Description: "形态条件\nT-3形态不限；T-2收盘涨停，T-1收盘跌停。T-2可以是一字板，也不要求实体涨幅达到5%；可与“涨跌停反转”同时命中。" +
				redReferenceAdmissionDescription + "\n\nT0开盘\n较T-1收盘上涨0.01%～3%，包含两端；平开不入选。" + redReferenceStatsDescription,
			DefinitionVersion: "v1",
			ConditionJSON: `{"all":[
{"field":"t-2.close_type","op":"eq","value":"ZT"},
{"field":"t-1.close_type","op":"eq","value":"DT"}
]}`,
			EntryJSON:  `{"field":"entry_gap","op":"between","min":0.01,"max":3.0,"inclusive":true}`,
			ManualRank: 20, MinSamples: 10, RedEntry: true,
		},
		{
			RuleKind: "condition",
			Name:     RuleNameMediumYangLimitDown,
			Description: "形态条件\nT-2为中阳或大阳，T-1收盘跌停。阳线实体按(收盘−开盘)/前收盘×100%计算：中阳>2.5%且≤6%，大阳>6%；涨停、跌停或破板不计入这里的中阳/大阳。" +
				redReferenceAdmissionDescription + "\n\nT0开盘\n较T-1收盘上涨0.01%～0.75%，包含两端；平开不入选。" + redReferenceStatsDescription,
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
			RuleKind: "condition",
			Name:     RuleNameBullishZtZtPb,
			Description: "形态条件\nT-3、T-2连续收盘涨停；T-1涨停破板，并且收盘价严格高于开盘价（必须收阳，收平不算）。本组合没有振幅6%～10%或下影≤1%的额外限制。" +
				redReferenceAdmissionDescription + "\n\nT0开盘\n较T-1收盘上涨0.01%～3%，包含两端；平开不入选。" + redReferenceStatsDescription,
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
			RuleKind: "condition",
			Name:     RuleNameZtZtBearish,
			Description: "形态条件\nT-3、T-2连续收盘涨停；T-1收盘低于开盘。T-1收盘较T-2收盘涨幅为−2%～3%（包含两端）；阴线实体跌幅=(T-1开盘−T-1收盘)/T-2收盘×100%，严格大于0%、小于8%（不含两端）。不再额外限定为小阴或中阴。" +
				redReferenceAdmissionDescription + "\n\nT0开盘\n较T-1收盘上涨0.01%～3%，包含两端；平开不入选。" + redReferenceStatsDescription,
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
