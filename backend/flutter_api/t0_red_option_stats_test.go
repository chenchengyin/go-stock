package flutter_api

import (
	"github.com/glebarez/sqlite"
	"go-stock/backend/db"
	"go-stock/backend/models"
	"gorm.io/gorm"
	"path/filepath"
	"testing"
)

func TestRedHistoryOptionsUseVerifiedOutcomeCount(t *testing.T) {
	old := db.Dao
	dao, err := gorm.Open(sqlite.Open(filepath.Join(t.TempDir(), "stats.db")), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	db.Dao = dao
	t.Cleanup(func() { db.Dao = old })
	if err := dao.AutoMigrate(&models.T0RedRuleStat{}); err != nil {
		t.Fatal(err)
	}
	key := optimizedRedPrefix + "upper_body8_v1"
	if err := dao.Create(&models.T0RedRuleStat{RuleKey: key, RuleVersion: 1, EntrySource: "daily_open", ExitHorizon: "T+1_close", PeriodKey: "all", DataVersion: "test", BatchID: "test", SampleCount: 66, PositiveCount: 47, TargetCount: 38, ProfitWinRate: 71.21, TargetRate: 57.58, Status: "verified"}).Error; err != nil {
		t.Fatal(err)
	}
	opts := enrichGobRedHistoryOptions([]gobRedRuleHistoryOption{{RuleSource: "gob", RuleKey: key, RuleVersion: 1, MatchCount: 99}}, nil)
	if opts[0].Stats == nil || opts[0].Stats.SampleCount != 66 || opts[0].Stats.TargetRate != 57.58 || opts[0].MatchCount != 99 {
		t.Fatalf("wrong option statistics: %+v", opts[0])
	}
}
