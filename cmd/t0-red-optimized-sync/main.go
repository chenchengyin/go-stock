package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"github.com/glebarez/sqlite"
	"go-stock/backend/flutter_api"
	"gorm.io/gorm"
	"os"
)

func main() {
	cache := flag.String("cache", "backend/data/cache", "input cache root (read only)")
	start := flag.String("start", "2024-01-02", "first T0 date")
	end := flag.String("end", "", "last T0 date, before today")
	batch := flag.String("batch", "optimized-seven-v1", "replay batch")
	archives := flag.String("archive-out", "", "optional separate archive output directory")
	database := flag.String("db", "", "optional SQLite destination; only seven new rules are written")
	flag.Parse()
	stats, cases, coverage, err := flutter_api.ReplayOptimizedRedRules(*cache, *start, *end, *batch, *archives)
	if err == nil && *database != "" {
		if coverage.Dates == 0 || coverage.CompleteDays != coverage.Dates || coverage.MissingT0 > 0 {
			err = fmt.Errorf("覆盖不完整，拒绝写入统计库")
		} else {
			var dao *gorm.DB
			dao, err = gorm.Open(sqlite.Open(*database), &gorm.Config{})
			if err == nil {
				err = flutter_api.SaveGobRedRuleStats(dao, stats)
			}
		}
	}
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	if err := json.NewEncoder(os.Stdout).Encode(map[string]interface{}{"stats": stats, "cases": cases, "coverage": coverage}); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
