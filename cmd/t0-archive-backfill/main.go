package main

import (
	"flag"
	"fmt"
	"log"

	"go-stock/backend/db"
	"go-stock/backend/flutter_api"
)

func main() {
	cacheDir := flag.String("cache-dir", "backend/data/cache", "T0 cache root")
	dbPath := flag.String("db", "backend/data/stock.db", "SQLite database path")
	limit := flag.Int("limit", 132, "maximum number of archives")
	reverse := flag.Bool("reverse", true, "process newest missing archive first")
	concurrency := flag.Int("concurrency", 20, "maximum concurrent per-stock daily requests")
	flag.Parse()

	db.Init(*dbPath)
	flutter_api.AutoMigrate()

	report, err := flutter_api.BackfillT0SelectionArchives(*cacheDir, flutter_api.T0ArchiveBackfillOptions{
		Limit:       *limit,
		Reverse:     *reverse,
		Concurrency: *concurrency,
	})
	if err != nil {
		log.Fatal(err)
	}
	first, last := flutter_api.T0ArchiveBackfillDateRange(report)
	fmt.Printf("backfill selected=%d completed=%d failed=%d enriched_results=%d missing_daily=%d range=%s..%s at=%s\n",
		report.SelectedArchives,
		report.CompletedArchives,
		report.FailedArchives,
		report.EnrichedResults,
		report.MissingDailyData,
		first,
		last,
		flutter_api.T0ArchiveBackfillNow(),
	)
}
