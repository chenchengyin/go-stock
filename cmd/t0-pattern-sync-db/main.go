package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"os"
	"time"

	"go-stock/backend/db"
	"go-stock/backend/flutter_api"
	"go-stock/backend/models"

	"gorm.io/gorm"
)

type aggregateFile struct {
	Range      string   `json:"range"`
	DateStart  string   `json:"date_start"`
	DateEnd    string   `json:"date_end"`
	Patterns   []aggRow `json:"patterns"`
}

type aggRow struct {
	Pattern  string  `json:"pattern"`
	T0N      int     `json:"t0_n"`
	WinRate  float64 `json:"win_rate"`
	FailRate float64 `json:"fail_rate"`
	AvgT0    float64 `json:"avg_t0"`
	MedT0    float64 `json:"med_t0"`
}

func main() {
	from := flag.String("from", "", "path to pattern aggregate JSON")
	dbPath := flag.String("db", "", "sqlite path (default data/stock.db)")
	flag.Parse()
	if *from == "" {
		log.Fatal("--from required")
	}
	if *dbPath == "" {
		*dbPath = "data/stock.db"
	}
	db.Init(*dbPath)
	flutter_api.AutoMigrate()

	n, batch, err := syncPatternStats(db.Dao, *from)
	if err != nil {
		log.Fatal(err)
	}
	fmt.Printf("synced %d patterns batch=%s\n", n, batch)
}

func syncPatternStats(dao *gorm.DB, path string) (int, string, error) {
	raw, err := os.ReadFile(path)
	if err != nil {
		return 0, "", err
	}
	var file aggregateFile
	if err := json.Unmarshal(raw, &file); err != nil {
		return 0, "", err
	}
	batchID := file.Range
	if batchID == "" && file.DateStart != "" && file.DateEnd != "" {
		batchID = file.DateStart + ":" + file.DateEnd
	}
	if batchID == "" {
		batchID = "unknown"
	}
	now := time.Now()

	err = dao.Transaction(func(tx *gorm.DB) error {
		if err := tx.Where("1 = 1").Delete(&models.T0PatternStat{}).Error; err != nil {
			return err
		}
		rows := make([]models.T0PatternStat, 0, len(file.Patterns))
		for _, p := range file.Patterns {
			if p.Pattern == "" {
				continue
			}
			rows = append(rows, models.T0PatternStat{
				Pattern:   p.Pattern,
				Window:    3,
				T0N:       p.T0N,
				WinRate:   p.WinRate,
				FailRate:  p.FailRate,
				AvgPnL:    p.AvgT0,
				MedPnL:    p.MedT0,
				BatchID:   batchID,
				UpdatedAt: now,
			})
		}
		if len(rows) > 0 {
			if err := tx.CreateInBatches(rows, 200).Error; err != nil {
				return err
			}
		}
		cfg := models.DefaultT0PatternConfig(batchID)
		cfg.UpdatedAt = now
		return tx.Save(&cfg).Error
	})
	if err != nil {
		return 0, "", err
	}
	return len(file.Patterns), batchID, nil
}
