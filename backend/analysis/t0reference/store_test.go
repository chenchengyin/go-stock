package t0reference

import (
	"path/filepath"
	"testing"

	"github.com/glebarez/sqlite"
	"go-stock/backend/models"
	"gorm.io/gorm"
)

func TestPersistObservationIsIdempotentAndPreservesRevisions(t *testing.T) {
	dao, err := gorm.Open(sqlite.Open(filepath.Join(t.TempDir(), "reference.db")), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	if err := dao.AutoMigrate(&models.T0ReferenceObservation{}, &models.T0ReferenceBar{}); err != nil {
		t.Fatal(err)
	}

	obs := testObservation(t)
	if err := PersistObservation(dao, obs, "gob:one", "hash:one", "pool:one"); err != nil {
		t.Fatal(err)
	}
	if err := PersistObservation(dao, obs, "gob:one", "hash:one", "pool:one"); err != nil {
		t.Fatal(err)
	}
	updated := obs
	updated.T0.Close = 12.8
	updated.PnL = (updated.T0.Close - updated.EntryPrice) / updated.EntryPrice * 100
	if err := PersistObservation(dao, updated, "gob:two", "hash:two", "pool:two"); err != nil {
		t.Fatal(err)
	}

	var observations int64
	if err := dao.Model(&models.T0ReferenceObservation{}).Count(&observations).Error; err != nil {
		t.Fatal(err)
	}
	if observations != 2 {
		t.Fatalf("observation count = %d want 2", observations)
	}
	var bars int64
	if err := dao.Model(&models.T0ReferenceBar{}).Count(&bars).Error; err != nil {
		t.Fatal(err)
	}
	if bars != 6 {
		t.Fatalf("bar count = %d want 6", bars)
	}
	var active int64
	if err := dao.Model(&models.T0ReferenceObservation{}).Where("is_active = ?", true).Count(&active).Error; err != nil {
		t.Fatal(err)
	}
	if active != 1 {
		t.Fatalf("active count = %d want 1", active)
	}
	loaded, err := LoadActiveObservations(dao)
	if err != nil {
		t.Fatal(err)
	}
	if len(loaded) != 1 || loaded[0].T0.Close != 12.8 || loaded[0].Bars[-3].Date == "" {
		t.Fatalf("loaded observations = %+v", loaded)
	}
}
