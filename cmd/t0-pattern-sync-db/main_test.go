package main

import (
	"os"
	"path/filepath"
	"testing"

	"go-stock/backend/db"
	"go-stock/backend/flutter_api"
	"go-stock/backend/models"
)

func TestSyncPatternStats(t *testing.T) {
	dir := t.TempDir()
	dbPath := filepath.Join(dir, "test.db")
	db.Init(dbPath)
	flutter_api.AutoMigrate()

	jsonPath := filepath.Join(dir, "agg.json")
	if err := os.WriteFile(jsonPath, []byte(`{
  "range": "2025-01-02:2026-08-22",
  "patterns": [
    {"pattern":"XY|ZT|ZT","t0_n":56,"win_rate":41.1,"fail_rate":44.6,"avg_t0":1.1,"med_t0":1.0},
    {"pattern":"SY|XY|SY","t0_n":42,"win_rate":19.0,"fail_rate":61.9,"avg_t0":-0.1,"med_t0":-1.0}
  ]
}`), 0o644); err != nil {
		t.Fatal(err)
	}

	n, batch, err := syncPatternStats(db.Dao, jsonPath)
	if err != nil {
		t.Fatal(err)
	}
	if n != 2 || batch != "2025-01-02:2026-08-22" {
		t.Fatalf("n=%d batch=%q", n, batch)
	}
	var count int64
	if err := db.Dao.Model(&models.T0PatternStat{}).Count(&count).Error; err != nil {
		t.Fatal(err)
	}
	if count != 2 {
		t.Fatalf("count=%d", count)
	}
	var st models.T0PatternStat
	if err := db.Dao.Where("pattern = ?", "XY|ZT|ZT").First(&st).Error; err != nil {
		t.Fatal(err)
	}
	if st.WinRate != 41.1 || st.T0N != 56 {
		t.Fatalf("stat=%+v", st)
	}
	var cfg models.T0PatternConfig
	if err := db.Dao.First(&cfg, 1).Error; err != nil {
		t.Fatal(err)
	}
	if cfg.BatchID != "2025-01-02:2026-08-22" || cfg.MinSamples != 1 {
		t.Fatalf("cfg=%+v", cfg)
	}
}
