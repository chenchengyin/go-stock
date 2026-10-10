package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"os"

	"github.com/glebarez/sqlite"
	"go-stock/backend/data"
	"go-stock/backend/db"
	"go-stock/backend/flutter_api"
	"gorm.io/gorm"
)

func main() {
	cache := flag.String("cache", "backend/data/cache", "Gob cache root")
	start := flag.String("start", "", "first T0 date")
	end := flag.String("end", "", "last T0 date, before today")
	refreshDaily := flag.Bool("refresh-daily", false, "refresh missing daily bars using each date's cached stock pool")
	flag.Parse()
	if *start == "" || *end == "" {
		fmt.Fprintln(os.Stderr, "-start and -end are required")
		os.Exit(2)
	}
	if *refreshDaily {
		marketDB, err := gorm.Open(sqlite.Open("file:t0-red-archive-backfill?mode=memory&cache=shared"), &gorm.Config{})
		if err == nil {
			db.Dao = marketDB
			err = marketDB.AutoMigrate(&data.Settings{})
		}
		if err == nil {
			err = marketDB.Create(&data.Settings{CrawlTimeOut: 30}).Error
		}
		if err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(1)
		}
		refresh, err := flutter_api.RefreshGobRedDailyCaches(*cache, *start, *end)
		if err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(1)
		}
		if err := json.NewEncoder(os.Stderr).Encode(refresh); err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(1)
		}
	}
	report, err := flutter_api.BackfillGobRedArchives(*cache, *start, *end)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	if err := json.NewEncoder(os.Stdout).Encode(report); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
