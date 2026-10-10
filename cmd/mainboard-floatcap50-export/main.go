package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"time"

	"github.com/glebarez/sqlite"
	"go-stock/backend/data"
	"go-stock/backend/db"
	"go-stock/backend/flutter_api"
	"gorm.io/gorm"
)

func main() {
	startDate := flag.String("start", "2024-01-01", "first daily K-line date, inclusive")
	endDate := flag.String("end", "", "last daily K-line date, inclusive; defaults to today in China")
	outputPath := flag.String("output", "backend/data/cache/mainboard_floatcap50_2024/daily.gob", "output Gob path")
	workers := flag.Int("workers", 6, "parallel stock downloads (maximum 10)")
	flag.Parse()

	if *endDate == "" {
		chinaTime := time.Now().In(time.FixedZone("Asia/Shanghai", 8*60*60))
		*endDate = chinaTime.Format("2006-01-02")
	}

	dao, err := gorm.Open(sqlite.Open("file:mainboard-floatcap50-export?mode=memory&cache=shared"), &gorm.Config{})
	if err != nil {
		fail(err)
	}
	db.Dao = dao
	if err := dao.AutoMigrate(&data.Settings{}); err != nil {
		fail(err)
	}
	if err := dao.Create(&data.Settings{CrawlTimeOut: 30}).Error; err != nil {
		fail(err)
	}
	data.ConfigureFromSettings(data.GetSettingConfig())

	lastProgress := time.Now()
	report, err := flutter_api.ExportMainboardFloatCap50DailyGob(
		*outputPath, *startDate, *endDate, *workers,
		func(done, total int, _ flutter_api.MainboardFloatCap50Stock) {
			if done == total || done%50 == 0 || time.Since(lastProgress) >= 15*time.Second {
				fmt.Fprintf(os.Stderr, "\rK线拉取进度 %d/%d", done, total)
				lastProgress = time.Now()
			}
		},
	)
	if err != nil {
		fmt.Fprintln(os.Stderr)
		fail(err)
	}
	fmt.Fprintln(os.Stderr)
	if err := json.NewEncoder(os.Stdout).Encode(report); err != nil {
		fail(err)
	}
}

func fail(err error) {
	fmt.Fprintln(os.Stderr, err)
	os.Exit(1)
}
