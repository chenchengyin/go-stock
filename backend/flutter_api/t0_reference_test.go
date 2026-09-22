package flutter_api

import (
	"path/filepath"
	"reflect"
	"testing"

	"github.com/glebarez/sqlite"
	"go-stock/backend/analysis/t0reference"
	"go-stock/backend/db"
	"go-stock/backend/models"
	"gorm.io/gorm"
)

func TestEnrichT0ReferenceUsesPreT0RulesAndLatestStats(t *testing.T) {
	previous := db.Dao
	t.Cleanup(func() { db.Dao = previous })
	dao, err := gorm.Open(sqlite.Open(filepath.Join(t.TempDir(), "reference.db")), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	db.Dao = dao
	if err := dao.AutoMigrate(&models.T0ReferenceRule{}, &models.T0ReferenceRuleStat{}); err != nil {
		t.Fatal(err)
	}
	rule, err := modelsRuleForTest()
	if err != nil {
		t.Fatal(err)
	}
	if err := dao.Create(&rule).Error; err != nil {
		t.Fatal(err)
	}
	if err := dao.Create(&models.T0ReferenceRuleStat{
		RuleID: rule.ID, BatchID: "batch", PeriodKey: "all", SampleCount: 20,
		ProfitWinRate: 70, TargetRate: 55, LossRate: 20, ResearchTier: "A",
	}).Error; err != nil {
		t.Fatal(err)
	}

	daily := map[string][]dailyBar{
		"600000": {
			{Date: "2026-01-02", Open: 10, Close: 10.2, High: 10.3, Low: 9.9},
			{Date: "2026-01-05", Open: 10.2, Close: 10.4, High: 10.5, Low: 10.1},
			{Date: "2026-01-06", Open: 10.4, Close: 10.6, High: 10.7, Low: 10.3},
			{Date: "2026-01-07", Open: 10.6, Close: 10.8, High: 10.9, Low: 10.5},
		},
	}
	result := []T0SelectionResult{{StockCode: "600000.XSHG", OpenGap: 1}}
	got := enrichT0ResultsForDisplayWithDaily(result, daily, "2026-01-08")
	if len(got) != 1 || len(got[0].T0ReferenceHits) != 1 {
		t.Fatalf("reference hits = %+v", got)
	}
	if got[0].T0ReferenceTier != "A" || got[0].T0ReferenceWinPct != 70 || got[0].T0ReferenceSamples != 20 {
		t.Fatalf("reference summary = %+v", got[0])
	}
	if len(got[0].T0ReferenceHits) != 1 || got[0].T0ReferenceHits[0].TargetRate != 55 ||
		got[0].T0ReferenceHits[0].EarnRate != 80 {
		t.Fatalf("reference stats = %+v", got[0].T0ReferenceHits[0])
	}
}

func TestEnrichT0ReferenceDoesNotPromoteLegacyDeepRedMarker(t *testing.T) {
	previous := db.Dao
	t.Cleanup(func() { db.Dao = previous })
	dao, err := gorm.Open(sqlite.Open(filepath.Join(t.TempDir(), "reference.db")), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	db.Dao = dao
	if err := dao.AutoMigrate(&models.T0ReferenceRule{}, &models.T0ReferenceRuleStat{}); err != nil {
		t.Fatal(err)
	}
	rule, err := modelsRuleForTestName("CUSTOM_SELECTED_PATTERN", `{"sequence":["ZT","DYIN","DT"]}`)
	if err != nil {
		t.Fatal(err)
	}
	deepRed := reflect.ValueOf(&rule).Elem().FieldByName("DeepRed")
	if !deepRed.IsValid() || deepRed.Kind() != reflect.Bool {
		t.Fatal("T0 reference rule must expose a DeepRed marker")
	}
	deepRed.SetBool(true)
	if err := dao.Create(&rule).Error; err != nil {
		t.Fatal(err)
	}
	if err := dao.Create(&models.T0ReferenceRuleStat{
		RuleID: rule.ID, BatchID: "batch", PeriodKey: "all", SampleCount: 8,
		ProfitWinRate: 75, ResearchTier: t0reference.ResearchTierInsufficient,
	}).Error; err != nil {
		t.Fatal(err)
	}

	daily := map[string][]dailyBar{
		"600001": {
			{Date: "2026-01-02", Open: 10, Close: 10, High: 10, Low: 10},
			{Date: "2026-01-05", Open: 10.5, Close: 11, High: 11, Low: 10.5},
			{Date: "2026-01-06", Open: 10.8, Close: 10.05, High: 10.8, Low: 10.05},
			{Date: "2026-01-07", Open: 9.5, Close: 9, High: 9.5, Low: 9},
		},
	}
	got := enrichT0ResultsForDisplayWithDaily([]T0SelectionResult{
		{StockCode: "600001.XSHG", OpenGap: 1},
	}, daily, "2026-01-08")
	if got[0].StrongDisplayRuleHit {
		t.Fatalf("legacy deep-red reference hit should not promote result: %+v", got[0])
	}
}

func TestRedReferenceEnrichmentOnlyShowsRedEntryRules(t *testing.T) {
	condition := `{"all":[
{"field":"t-2.close_type","op":"eq","value":"ZT"},
{"field":"t-1.close_type","op":"eq","value":"ZT"}
]}`
	redDefinition := t0reference.RuleDefinition{
		RuleKind: "condition", Name: "RED_PATTERN", DefinitionVersion: "v1",
		ConditionJSON: condition,
		EntryJSON:     `{"field":"entry_gap","op":"between","min":0.01,"max":3.0,"inclusive":true}`,
		ManualRank:    1, MinSamples: 10, RedEntry: true,
	}
	referenceDefinition := redDefinition
	referenceDefinition.Name = "REFERENCE_ONLY"
	referenceDefinition.DefinitionVersion = "v2"
	referenceDefinition.RedEntry = false
	redRule, err := t0reference.CompileRule(redDefinition)
	if err != nil {
		t.Fatal(err)
	}
	referenceRule, err := t0reference.CompileRule(referenceDefinition)
	if err != nil {
		t.Fatal(err)
	}
	hist := []dailyBar{
		{Date: "2026-01-01", Close: 10},
		{Date: "2026-01-02", Open: 10, Close: 11, High: 11, Low: 10},
		{Date: "2026-01-05", Open: 11, Close: 12.1, High: 12.1, Low: 11},
		{Date: "2026-01-06", Open: 12.2, Close: 13.31, High: 13.31, Low: 12.2},
	}
	result := T0SelectionResult{OpenGap: 1}
	enrichT0ReferenceResult(&result, hist, []t0ReferenceRuleRuntime{
		{rule: redRule, redEntry: true},
		{rule: referenceRule, redEntry: false},
	}, true)
	if len(result.T0ReferenceHits) != 1 || result.T0ReferenceHits[0].Name != "RED_PATTERN" {
		t.Fatalf("red reference hits = %+v, want only RED_PATTERN", result.T0ReferenceHits)
	}
}

func TestLoadT0ReferenceRulesIgnoresStaleSameNameDefinition(t *testing.T) {
	previous := db.Dao
	t.Cleanup(func() { db.Dao = previous })
	dao, err := gorm.Open(sqlite.Open(filepath.Join(t.TempDir(), "reference.db")), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	db.Dao = dao
	if err := dao.AutoMigrate(&models.T0ReferenceRule{}, &models.T0ReferenceRuleStat{}); err != nil {
		t.Fatal(err)
	}
	legacy, err := modelsRuleForTestName(t0reference.RuleNameLimitUpLimitDownReversal, `{"all":[
{"field":"t-2.close_type","op":"eq","value":"ZT"},
{"field":"t-2.is_one_word","op":"eq","value":false},
{"field":"t-1.close_type","op":"eq","value":"DT"}
]}`)
	if err != nil {
		t.Fatal(err)
	}
	legacy.RedEntry = true
	if err := dao.Create(&legacy).Error; err != nil {
		t.Fatal(err)
	}
	canonicalDefinition := t0reference.RuleDefinition{}
	for _, definition := range t0reference.DefaultRuleDefinitions() {
		if definition.Name == t0reference.RuleNameAnyLimitUpLimitDown {
			canonicalDefinition = definition
			break
		}
	}
	canonical, err := t0reference.CompileRule(canonicalDefinition)
	if err != nil {
		t.Fatal(err)
	}
	if err := dao.Create(&models.T0ReferenceRule{
		RuleKey: canonical.RuleKey, RuleKind: canonicalDefinition.RuleKind,
		Name: canonicalDefinition.Name, DefinitionVersion: canonicalDefinition.DefinitionVersion,
		ConditionJSON: canonicalDefinition.ConditionJSON, EntryJSON: canonicalDefinition.EntryJSON,
		Enabled: true, RedEntry: false,
	}).Error; err != nil {
		t.Fatal(err)
	}

	hist := []dailyBar{
		{Date: "2026-09-01", Close: 10},
		{Date: "2026-09-02", Open: 10, Close: 10, High: 10.2, Low: 9.8},
		{Date: "2026-09-03", Open: 10.6, Close: 11, High: 11, Low: 10.6},
		{Date: "2026-09-04", Open: 9.9, Close: 9.9, High: 9.9, Low: 9.9},
	}
	hits := matchingRedEntryReferenceRuntimes(
		hist, T0SelectionResult{OpenGap: 1.2}, loadT0ReferenceRuleRuntimes())
	for _, hit := range hits {
		if hit.rule.Definition.Name == t0reference.RuleNameLimitUpLimitDownReversal {
			t.Fatalf("stale same-name reversal rule still matched: %+v", hit.rule.Definition)
		}
		if hit.rule.Definition.Name == t0reference.RuleNameAnyLimitUpLimitDown && !hit.redEntry {
			t.Fatal("canonical any-limit-up-limit-down rule should remain a red-entry rule")
		}
	}
}

func TestLoadT0ReferenceRulesIgnoresDeprecatedAmplitudeRules(t *testing.T) {
	previous := db.Dao
	t.Cleanup(func() { db.Dao = previous })
	dao, err := gorm.Open(sqlite.Open(filepath.Join(t.TempDir(), "reference.db")), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	db.Dao = dao
	if err := dao.AutoMigrate(&models.T0ReferenceRule{}, &models.T0ReferenceRuleStat{}); err != nil {
		t.Fatal(err)
	}
	legacyRules := []struct {
		name      string
		condition string
	}{
		{t0reference.RuleNameLegacyT2AmplitudeT1Open, `{"sequence":["ZT","ZT","ZT"]}`},
		{t0reference.RuleNameLegacyT2NonOneWordT1Open, `{"sequence":["ZT","ZT","PB"]}`},
		{t0reference.RuleNameLegacyT2AmplitudeT1Narrow, `{"sequence":["ZT","ZT","DT"]}`},
	}
	for _, legacy := range legacyRules {
		name := legacy.name
		condition := legacy.condition
		rule, err := modelsRuleForTestName(name, condition)
		if err != nil {
			t.Fatal(err)
		}
		rule.RedEntry = true
		if err := dao.Create(&rule).Error; err != nil {
			t.Fatal(err)
		}
	}
	for _, runtime := range loadT0ReferenceRuleRuntimes() {
		if t0reference.IsDeprecatedRuleName(runtime.rule.Definition.Name) {
			t.Fatalf("deprecated database rule was loaded: %q", runtime.rule.Definition.Name)
		}
	}
}

func modelsRuleForTest() (models.T0ReferenceRule, error) {
	return modelsRuleForTestName("test", `{"sequence":["SY","SY","SY"]}`)
}

func modelsRuleForTestName(name, conditionJSON string) (models.T0ReferenceRule, error) {
	definition := t0reference.RuleDefinition{
		RuleKind: "sequence", Name: name,
		DefinitionVersion: "v1",
		ConditionJSON:     conditionJSON,
		EntryJSON:         `{"field":"entry_gap","op":"between","min":0.01,"max":2,"inclusive":true}`,
		ManualRank:        1, MinSamples: 10,
	}
	compiled, err := t0reference.CompileRule(definition)
	if err != nil {
		return models.T0ReferenceRule{}, err
	}
	return models.T0ReferenceRule{
		RuleKey: compiled.RuleKey, RuleKind: definition.RuleKind, Name: definition.Name,
		LookbackStart: -3, LookbackEnd: -1, DefinitionVersion: definition.DefinitionVersion,
		ConditionJSON: definition.ConditionJSON, EntryJSON: definition.EntryJSON,
		ManualRank: 1, MinSamples: 10, Enabled: true,
	}, nil
}
