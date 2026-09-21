package flutter_api

import (
	"path/filepath"
	"testing"

	"go-stock/backend/db"
	"go-stock/backend/models"
)

func TestAutoMigrateCreatesT0ReferenceTablesAndActiveVersionIndex(t *testing.T) {
	db.Init(filepath.Join(t.TempDir(), "t0-reference.db"))
	AutoMigrate()

	first := models.T0ReferenceObservation{
		TradeDate: "2026-01-07", StockCode: "600000", SourceBatch: "gob:one", IsActive: true,
	}
	if err := db.Dao.Create(&first).Error; err != nil {
		t.Fatal(err)
	}
	second := models.T0ReferenceObservation{
		TradeDate: "2026-01-07", StockCode: "600000", SourceBatch: "gob:two", IsActive: true,
	}
	if err := db.Dao.Create(&second).Error; err == nil {
		t.Fatal("database should reject two active source versions for one stock date")
	}

	var barCount int64
	if err := db.Dao.Model(&models.T0ReferenceBar{}).Count(&barCount).Error; err != nil {
		t.Fatal(err)
	}
	var ruleCount int64
	if err := db.Dao.Model(&models.T0ReferenceRule{}).Count(&ruleCount).Error; err != nil {
		t.Fatal(err)
	}
	var statCount int64
	if err := db.Dao.Model(&models.T0ReferenceRuleStat{}).Count(&statCount).Error; err != nil {
		t.Fatal(err)
	}
}
