package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"os"

	"github.com/glebarez/sqlite"
	"go-stock/backend/flutter_api"
	"go-stock/backend/models"
	"gorm.io/gorm"
)

func main() {
	cache := flag.String("cache", "backend/data/cache", "Gob cache root")
	start := flag.String("start", "2024-01-02", "first T0 date")
	end := flag.String("end", "", "last T0 date, before today")
	batch := flag.String("batch", "manual-gob-replay", "source batch identity")
	database := flag.String("db", "", "optional explicit SQLite destination; empty means read-only")
	flag.Parse()
	if *end == "" {
		fmt.Fprintln(os.Stderr, "-end is required")
		os.Exit(2)
	}
	stats, cases, coverage, err := flutter_api.ReplayGobRedRuleStats(*cache, *start, *end, *batch)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	if *database != "" {
		if coverage.CompleteDays != coverage.Dates {
			fmt.Fprintln(os.Stderr, "Gob 回放覆盖不完整，拒绝写入统计库")
			os.Exit(1)
		}
		for _, stat := range stats {
			if stat.Status != "verified" {
				fmt.Fprintln(os.Stderr, "Gob 回放覆盖不完整，拒绝写入统计库")
				os.Exit(1)
			}
		}
		dao, err := gorm.Open(sqlite.Open(*database), &gorm.Config{})
		if err == nil {
			err = dao.AutoMigrate(&models.T0RedRuleStat{})
		}
		if err == nil {
			err = flutter_api.SaveGobRedRuleStats(dao, stats)
		}
		if err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(1)
		}
	}
	_ = json.NewEncoder(os.Stdout).Encode(map[string]interface{}{
		"stats": stats, "cases": cases, "coverage": coverage,
	})
}
