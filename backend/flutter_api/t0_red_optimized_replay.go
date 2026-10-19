package flutter_api

import (
	"encoding/gob"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"go-stock/backend/models"
)

// ReplayOptimizedRedRules uses exactly the page's historical pool and shared-
// first pricing. An explicit output directory writes additive archive copies;
// it never changes input GOBs or confirmed auction snapshots.
func ReplayOptimizedRedRules(cacheRoot, start, end, batch, archiveOutput string) ([]models.T0RedRuleStat, []GobRedReplayCase, GobRedReplayCoverage, error) {
	var coverage GobRedReplayCoverage
	for _, date := range []string{start, end} {
		normalized, err := normalizeT0TradeDate(date)
		if err != nil || normalized != date {
			return nil, nil, coverage, fmt.Errorf("日期无效: %s", date)
		}
	}
	if end < start || end >= time.Now().In(chinaLocation()).Format("2006-01-02") {
		return nil, nil, coverage, fmt.Errorf("结束日必须早于今天且不早于开始日")
	}
	file, err := os.Open(filepath.Join(cacheRoot, "mainboard_floatcap50_2024", "daily.gob"))
	if err != nil {
		return nil, nil, coverage, err
	}
	var snapshot MainboardFloatCap50Gob
	err = gob.NewDecoder(file).Decode(&snapshot)
	file.Close()
	if err != nil {
		return nil, nil, coverage, err
	}
	shared, err := buildMainboardSharedGobData(snapshot)
	if err != nil {
		return nil, nil, coverage, err
	}
	if archiveOutput != "" {
		if err := os.MkdirAll(archiveOutput, 0755); err != nil {
			return nil, nil, coverage, err
		}
		input, _ := filepath.Abs(filepath.Join(cacheRoot, "t0", "selection"))
		output, _ := filepath.Abs(archiveOutput)
		if input == output {
			return nil, nil, coverage, fmt.Errorf("归档输出须使用独立目录，验证后再替换")
		}
	}
	var samples []gobRedStatSample
	var cases []GobRedReplayCase
	missing := map[string]bool{}
	for i, date := range shared.Dates {
		if date < start || date > end {
			continue
		}
		coverage.Dates++
		stocks := shared.SelectionPools[date]
		var fallback map[string][]dailyBar
		if len(stocks) == 0 {
			cache, err := readGobRedReplayCache(filepath.Join(cacheRoot, "t0", "daily", "t0_daily_cache_"+date+".gob"), date)
			if err != nil || !cache.StockPoolComplete {
				if coverage.Rejected == nil {
					coverage.Rejected = map[string]string{}
				}
				coverage.Rejected[date] = "缺少当日完整股票池"
				continue
			}
			stocks, fallback = cache.Stocks, cache.Daily
		}
		coverage.PoolDays++
		daily, absent := sharedGobDailyForStocks(shared, stocks, "", fallback)
		coverage.MissingDailyStocks += len(absent)
		candidates := buildGobRedPreopenCandidates(date, stocks, daily, optimizedGobRedRules())
		quotes, _, _ := buildGobRedOpeningQuotes(date, resultsToT0Stocks(candidates), daily)
		results, status := matchGobRedRules(date, resultsToT0Stocks(candidates), daily, quotes, "daily_open", optimizedGobRedRules())
		coverage.MissingT0 += status.MissingOpen
		complete := len(absent) == 0 && status.MissingOpen == 0
		if complete {
			coverage.CompleteDays++
		}
		nextDate := ""
		if i+1 < len(shared.Dates) {
			nextDate = shared.Dates[i+1]
		}
		for _, result := range results {
			code := t0ShortCodeFromResultCode(result.StockCode)
			nextBar, index := dailyBarOnDate(daily[code], nextDate)
			// Shared snapshots can omit a historical symbol's outcome. Use only
			// the exact market T+1 date from its dated caches, never a later bar.
			if nextDate != "" && (index < 0 || nextBar.Close <= 0) {
				if b, j := dailyBarOnDate(fallback[code], nextDate); j >= 0 && b.Close > 0 {
					nextBar, index = b, j
				} else {
					for _, cacheDate := range []string{date, nextDate} {
						cache, err := readGobRedReplayCache(filepath.Join(cacheRoot, "t0", "daily", "t0_daily_cache_"+cacheDate+".gob"), cacheDate)
						if err != nil || !cache.StockPoolComplete {
							continue
						}
						if b, j := dailyBarOnDate(cache.Daily[code], nextDate); j >= 0 && b.Close > 0 {
							nextBar, index = b, j
							break
						}
					}
				}
			}
			for _, hit := range result.RedRuleHits {
				coverage.Matched++
				if index < 0 || nextBar.Close <= 0 {
					coverage.MissingT1++
					missing[hit.Key] = true
					continue
				}
				coverage.Observed++
				samples = append(samples, gobRedStatSample{Date: date, StockCode: code, Hit: hit, NextClose: nextBar.Close})
				cases = append(cases, GobRedReplayCase{Date: date, StockCode: code, RuleKey: hit.Key, NextDate: nextDate, ExitDate: nextDate, ExitHorizon: hit.ExitHorizon, PnL: (nextBar.Close/hit.EntryOpen - 1) * 100})
			}
		}
		if archiveOutput != "" {
			path := filepath.Join(cacheRoot, "t0", "selection", "t0_red_gob_"+date+".json")
			var archive gobRedArchive
			if data, err := os.ReadFile(path); err == nil {
				if err := json.Unmarshal(data, &archive); err != nil || archive.Date != date {
					return nil, nil, coverage, fmt.Errorf("原归档无效: %s", path)
				}
			}
			// Preserve every legacy admission and its original entry evidence.
			archive.Results, _ = mergeGobRedArchiveResults(archive.Results, results)
			archive.Date = date
			archive.Complete = archive.Complete && complete
			archive.RuleSetVersion = currentGobRedRuleSetVersion()
			archive.SavedAt = time.Now().Format(time.RFC3339)
			if archive.EntrySource == "" {
				archive.EntrySource = "daily_open"
			}
			if archive.Results == nil {
				archive.Results = []T0SelectionResult{}
			}
			data, err := json.Marshal(&archive)
			if err != nil {
				return nil, nil, coverage, err
			}
			if err := os.WriteFile(filepath.Join(archiveOutput, filepath.Base(path)), data, 0644); err != nil {
				return nil, nil, coverage, err
			}
		}
	}
	version := "shared-gob-optimized-v1-" + snapshot.GeneratedAt
	stats := aggregateGobRedRuleStats(samples, start, end, version, batch)
	for i := range stats {
		if coverage.CompleteDays != coverage.Dates || missing[stats[i].RuleKey] {
			stats[i].Status = "partial"
		}
	}
	return stats, cases, coverage, nil
}
