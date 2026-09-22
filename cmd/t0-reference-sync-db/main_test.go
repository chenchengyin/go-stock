package main

import (
	"path/filepath"
	"testing"

	"github.com/glebarez/sqlite"
	"go-stock/backend/analysis/candlepattern"
	"go-stock/backend/analysis/t0reference"
	"go-stock/backend/models"
	"gorm.io/gorm"
)

func TestSyncCacheBackfillsCompleteObservationsIdempotently(t *testing.T) {
	dao, err := gorm.Open(sqlite.Open(filepath.Join(t.TempDir(), "reference.db")), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	if err := dao.AutoMigrate(&models.T0ReferenceObservation{}, &models.T0ReferenceBar{}); err != nil {
		t.Fatal(err)
	}

	cache := &candlepattern.DailyCache{
		TradeDate: "2026-01-08",
		Stocks:    []candlepattern.StockMeta{{ShortCode: "000001"}},
		Daily: map[string][]candlepattern.DailyBar{
			"000001": {
				{Date: "2026-01-02", Open: 10, Close: 10.2, High: 10.3, Low: 9.9},
				{Date: "2026-01-05", Open: 10.2, Close: 10.4, High: 10.5, Low: 10.1},
				{Date: "2026-01-06", Open: 10.4, Close: 10.6, High: 10.7, Low: 10.3},
				{Date: "2026-01-07", Open: 10.6, Close: 10.8, High: 10.9, Low: 10.5},
				{Date: "2026-01-08", Open: 10.8, Close: 11.0, High: 11.1, Low: 10.7},
			},
		},
	}

	first, err := syncCache(dao, cache, "gob:test", "pool:test")
	if err != nil {
		t.Fatal(err)
	}
	second, err := syncCache(dao, cache, "gob:test", "pool:test")
	if err != nil {
		t.Fatal(err)
	}
	if first.Complete != 1 || first.Incomplete != 0 || second.Complete != 1 {
		t.Fatalf("first=%+v second=%+v", first, second)
	}
	var observations, bars int64
	if err := dao.Model(&models.T0ReferenceObservation{}).Count(&observations).Error; err != nil {
		t.Fatal(err)
	}
	if err := dao.Model(&models.T0ReferenceBar{}).Count(&bars).Error; err != nil {
		t.Fatal(err)
	}
	if observations != 1 || bars != 3 {
		t.Fatalf("observations=%d bars=%d", observations, bars)
	}
}

func TestPersistRulesAndStatsClearsRemovedDeepRedMarker(t *testing.T) {
	dao, err := gorm.Open(sqlite.Open(filepath.Join(t.TempDir(), "reference.db")), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	if err := dao.AutoMigrate(&models.T0ReferenceRule{}, &models.T0ReferenceRuleStat{}); err != nil {
		t.Fatal(err)
	}
	definition := t0reference.RuleDefinition{
		RuleKind: "sequence", Name: "DYIN|YX|MYIN", DefinitionVersion: "v1",
		ConditionJSON: `{"sequence":["DYIN","YX","MYIN"]}`,
		EntryJSON:     `{"field":"entry_gap","op":"between","min":0.01,"max":2,"inclusive":true}`,
		ManualRank:    1, MinSamples: 5, DeepRed: false, RedEntry: true,
	}
	compiled, err := t0reference.CompileRule(definition)
	if err != nil {
		t.Fatal(err)
	}
	if err := dao.Create(&models.T0ReferenceRule{
		RuleKey: compiled.RuleKey, RuleKind: definition.RuleKind, Name: definition.Name,
		DefinitionVersion: definition.DefinitionVersion, ConditionJSON: definition.ConditionJSON,
		EntryJSON: definition.EntryJSON, ManualRank: definition.ManualRank,
		MinSamples: definition.MinSamples, DeepRed: true, Enabled: true,
	}).Error; err != nil {
		t.Fatal(err)
	}

	_, _, err = persistRulesAndStats(dao, []ruleRuntime{{
		compiled: compiled,
		stats:    t0reference.NewReferenceStatsAccumulator(compiled),
	}}, "batch", []string{"2026-01-08"})
	if err != nil {
		t.Fatal(err)
	}
	var got models.T0ReferenceRule
	if err := dao.Where("rule_key = ?", compiled.RuleKey).First(&got).Error; err != nil {
		t.Fatal(err)
	}
	if got.DeepRed {
		t.Fatal("removed deep-red marker was not cleared")
	}
	if !got.RedEntry {
		t.Fatal("red-entry marker was not persisted")
	}
}

func TestPersistRulesAndStatsDisablesDeprecatedRule(t *testing.T) {
	dao, err := gorm.Open(sqlite.Open(filepath.Join(t.TempDir(), "reference.db")), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	if err := dao.AutoMigrate(&models.T0ReferenceRule{}, &models.T0ReferenceRuleStat{}); err != nil {
		t.Fatal(err)
	}
	deprecated := t0reference.RuleDefinition{
		RuleKind: "condition", Name: t0reference.RuleNameRemovedTwoLimitUpNonOneWord,
		DefinitionVersion: "v1",
		ConditionJSON:     `{"field":"t-1.open_ret","op":"gte","value":3}`,
		EntryJSON:         `{"field":"entry_gap","op":"gte","value":0}`,
	}
	compiled, err := t0reference.CompileRule(deprecated)
	if err != nil {
		t.Fatal(err)
	}
	if err := dao.Create(&models.T0ReferenceRule{
		RuleKey: compiled.RuleKey, RuleKind: deprecated.RuleKind, Name: deprecated.Name,
		DefinitionVersion: deprecated.DefinitionVersion, ConditionJSON: deprecated.ConditionJSON,
		EntryJSON: deprecated.EntryJSON, Enabled: true,
	}).Error; err != nil {
		t.Fatal(err)
	}

	if _, _, err := persistRulesAndStats(dao, nil, "batch", []string{"2026-01-08"}); err != nil {
		t.Fatal(err)
	}
	var got models.T0ReferenceRule
	if err := dao.Where("rule_key = ?", compiled.RuleKey).First(&got).Error; err != nil {
		t.Fatal(err)
	}
	if got.Enabled {
		t.Fatal("deprecated rule remains enabled")
	}
}

func TestPersistRulesAndStatsDisablesStaleSameNameRule(t *testing.T) {
	dao, err := gorm.Open(sqlite.Open(filepath.Join(t.TempDir(), "reference.db")), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	if err := dao.AutoMigrate(&models.T0ReferenceRule{}, &models.T0ReferenceRuleStat{}); err != nil {
		t.Fatal(err)
	}
	legacy, err := t0reference.CompileRule(t0reference.RuleDefinition{
		RuleKind: "condition", Name: t0reference.RuleNameLimitUpLimitDownReversal,
		DefinitionVersion: "v1",
		ConditionJSON:     `{"all":[{"field":"t-2.close_type","op":"eq","value":"ZT"},{"field":"t-1.close_type","op":"eq","value":"DT"}]}`,
		EntryJSON:         `{"field":"entry_gap","op":"between","min":0.01,"max":3,"inclusive":true}`,
	})
	if err != nil {
		t.Fatal(err)
	}
	if err := dao.Create(&models.T0ReferenceRule{
		RuleKey: legacy.RuleKey, RuleKind: legacy.Definition.RuleKind,
		Name: legacy.Definition.Name, DefinitionVersion: legacy.Definition.DefinitionVersion,
		ConditionJSON: legacy.Definition.ConditionJSON, EntryJSON: legacy.Definition.EntryJSON,
		Enabled: true, RedEntry: true,
	}).Error; err != nil {
		t.Fatal(err)
	}

	var canonical t0reference.RuleDefinition
	for _, definition := range t0reference.DefaultRuleDefinitions() {
		if definition.Name == t0reference.RuleNameLimitUpLimitDownReversal {
			canonical = definition
			break
		}
	}
	compiled, err := t0reference.CompileRule(canonical)
	if err != nil {
		t.Fatal(err)
	}
	if _, _, err := persistRulesAndStats(dao, []ruleRuntime{{
		compiled: compiled,
		stats:    t0reference.NewReferenceStatsAccumulator(compiled),
	}}, "batch", []string{"2026-01-08"}); err != nil {
		t.Fatal(err)
	}

	var got models.T0ReferenceRule
	if err := dao.Where("rule_key = ?", legacy.RuleKey).First(&got).Error; err != nil {
		t.Fatal(err)
	}
	if got.Enabled {
		t.Fatalf("stale same-name rule remains enabled: %+v", got)
	}
}
