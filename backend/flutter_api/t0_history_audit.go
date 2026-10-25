package flutter_api

import (
	"encoding/gob"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"time"
)

type T0HistoryAuditDay struct {
	Date       string `json:"date"`
	Status     string `json:"status"`
	PoolStocks int    `json:"pool_stocks"`
	PoolSource string `json:"pool_source,omitempty"`
	Confirmed  bool   `json:"confirmed"`
	Reason     string `json:"reason,omitempty"`
}

type T0HistoryAuditReport struct {
	CheckedAt            string              `json:"checked_at"`
	CacheRoot            string              `json:"cache_root"`
	TradingDates         int                 `json:"trading_dates"`
	AvailableDates       int                 `json:"available_dates"`
	MissingPoolDates     int                 `json:"missing_pool_dates"`
	RecoveredPools       int                 `json:"recovered_pools"`
	ReconstructedDates   int                 `json:"reconstructed_dates"`
	UnusableLegacyCaches []string            `json:"unusable_legacy_caches"`
	Errors               []string            `json:"errors,omitempty"`
	Days                 []T0HistoryAuditDay `json:"days"`
}

// AuditT0History never infers historical membership from the current Stocks.
// recoverPools copies only witnessed dated pools to the independent archive.
// This offline operation is intended for the standalone CLI, not concurrent
// requests: like existing backfill commands it temporarily selects a cache root.
func AuditT0History(cacheRoot string, recoverPools bool) (T0HistoryAuditReport, error) {
	var report T0HistoryAuditReport
	root, err := filepath.Abs(cacheRoot)
	if err != nil {
		return report, err
	}
	oldRoot := t0CacheRootPath
	t0CacheRootPath = root
	defer func() { t0CacheRootPath = oldRoot }()
	report.CheckedAt = time.Now().In(chinaLocation()).Format(time.RFC3339)
	report.CacheRoot = root
	f, err := os.Open(mainboardSharedGobPath())
	if err != nil {
		return report, err
	}
	var snapshot MainboardFloatCap50Gob
	err = gob.NewDecoder(f).Decode(&snapshot)
	f.Close()
	if err != nil {
		return report, err
	}
	shared, err := buildMainboardSharedGobData(snapshot)
	if err != nil {
		return report, err
	}
	pools, issues, legacy := collectT0PoolSources(snapshot)
	report.UnusableLegacyCaches = legacy
	issueDates := make([]string, 0, len(issues))
	for d := range issues {
		issueDates = append(issueDates, d)
	}
	sort.Strings(issueDates)
	for _, d := range issueDates {
		report.Errors = append(report.Errors, issues[d])
	}
	if recoverPools {
		dates := make([]string, 0, len(pools))
		for d := range pools {
			dates = append(dates, d)
		}
		sort.Strings(dates)
		for _, d := range dates {
			if issues[d] != "" {
				continue
			}
			if _, err := readT0PoolArchive(d); err == nil {
				continue
			} else if !os.IsNotExist(err) {
				report.Errors = append(report.Errors, err.Error())
				continue
			}
			if err := saveT0PoolArchive(d, pools[d].Stocks, pools[d].Source); err != nil {
				return report, fmt.Errorf("恢复 %s: %w", d, err)
			}
			report.RecoveredPools++
		}
	}
	for _, date := range shared.Dates {
		if isT0WeekendDate(date) || date >= time.Now().In(chinaLocation()).Format("2006-01-02") {
			continue
		}
		row := T0HistoryAuditDay{Date: date, PoolStocks: len(pools[date].Stocks), PoolSource: pools[date].Source}
		_, found, err := loadSharedGobRedConfirmation(date)
		row.Confirmed = err == nil && found
		if err != nil {
			report.Errors = append(report.Errors, err.Error())
		}
		if row.Confirmed || (row.PoolStocks > 0 && issues[date] == "" && err == nil) {
			row.Status = "available"
			report.AvailableDates++
		} else {
			row.Status = "missing_pool"
			row.Reason = "缺少当日完整股票池，不能使用当前股票池冒充历史"
			if issues[date] != "" {
				row.Status = "pool_error"
				row.Reason = issues[date]
			}
			report.MissingPoolDates++
			if issues[date] == "" {
				if reconstructed, reconstructionErr := readT0HistoricalPoolReconstruction(date); reconstructionErr == nil {
					row.Status = "reconstructed"
					row.PoolStocks = len(reconstructed.Stocks)
					row.PoolSource = reconstructed.Source
					row.Reason = reconstructed.warning().Message
					report.ReconstructedDates++
				}
			}
		}
		report.Days = append(report.Days, row)
		report.TradingDates++
	}
	return report, nil
}
