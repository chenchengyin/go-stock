package models

import (
	"path/filepath"
	"testing"

	"github.com/glebarez/sqlite"
	"gorm.io/gorm"
)

func TestT0ReferenceObservationKeepsSourceVersionsAndOneActiveVersion(t *testing.T) {
	dao, err := gorm.Open(sqlite.Open(filepath.Join(t.TempDir(), "reference.db")), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	if err := dao.AutoMigrate(&T0ReferenceObservation{}, &T0ReferenceBar{}); err != nil {
		t.Fatal(err)
	}

	old := T0ReferenceObservation{
		TradeDate: "2026-01-07", StockCode: "600000", SourceBatch: "gob:old",
		SourceHash: "old-hash", IsActive: true, DataStatus: "complete",
	}
	if err := dao.Create(&old).Error; err != nil {
		t.Fatal(err)
	}
	if err := dao.Create(&old).Error; err == nil {
		t.Fatal("same source batch should not be inserted twice")
	}

	if err := dao.Model(&T0ReferenceObservation{}).
		Where("trade_date = ? AND stock_code = ?", old.TradeDate, old.StockCode).
		Update("is_active", false).Error; err != nil {
		t.Fatal(err)
	}
	newVersion := T0ReferenceObservation{
		TradeDate: "2026-01-07", StockCode: "600000", SourceBatch: "gob:new",
		SourceHash: "new-hash", IsActive: true, DataStatus: "complete",
	}
	if err := dao.Create(&newVersion).Error; err != nil {
		t.Fatal(err)
	}
	if err := dao.Create(&T0ReferenceObservation{
		TradeDate: "2026-01-07", StockCode: "000001", SourceBatch: "gob:new",
		SourceHash: "other-stock", IsActive: true, DataStatus: "complete",
	}).Error; err != nil {
		t.Fatalf("same source batch should be reusable across stocks: %v", err)
	}

	var total int64
	if err := dao.Model(&T0ReferenceObservation{}).
		Where("trade_date = ? AND stock_code = ?", old.TradeDate, old.StockCode).
		Count(&total).Error; err != nil {
		t.Fatal(err)
	}
	if total != 2 {
		t.Fatalf("version count = %d want 2", total)
	}
	var active int64
	if err := dao.Model(&T0ReferenceObservation{}).
		Where("trade_date = ? AND stock_code = ? AND is_active = ?", old.TradeDate, old.StockCode, true).
		Count(&active).Error; err != nil {
		t.Fatal(err)
	}
	if active != 1 {
		t.Fatalf("active version count = %d want 1", active)
	}
}
