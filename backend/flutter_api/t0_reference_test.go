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
		ProfitWinRate: 70, ResearchTier: "A",
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
}

func TestEnrichT0ReferenceHonorsExplicitDeepRedMarker(t *testing.T) {
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
	if !got[0].StrongDisplayRuleHit {
		t.Fatalf("selected deep-red reference hit should be deep red: %+v", got[0])
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
