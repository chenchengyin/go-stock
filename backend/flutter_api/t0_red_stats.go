package flutter_api

import (
	"encoding/gob"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"time"

	"go-stock/backend/db"
	"go-stock/backend/models"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

type GobRedReplayCase struct {
	Date        string  `json:"date"`
	StockCode   string  `json:"stock_code"`
	RuleKey     string  `json:"rule_key"`
	NextDate    string  `json:"next_date"`
	ExitDate    string  `json:"exit_date"`
	ExitHorizon string  `json:"exit_horizon"`
	PnL         float64 `json:"pnl"`
}

type GobRedReplayCoverage struct {
	Dates                int               `json:"dates"`
	PoolDays             int               `json:"pool_days"`
	CompleteDays         int               `json:"complete_days"`
	MissingDailyStocks   int               `json:"missing_daily_stocks"`
	Matched              int               `json:"matched"`
	Observed             int               `json:"observed"`
	MissingT0            int               `json:"missing_t0"`
	MissingT1            int               `json:"missing_t1"`
	MissingNextCacheDays int               `json:"missing_next_cache_days"`
	Rejected             map[string]string `json:"rejected,omitempty"`
}

type gobRedRuleStatScope struct {
	ruleKey, entrySource, exitHorizon string
	ruleVersion                       int
}

func gobRedRuleStatLookupScopes(hit T0RedRuleHit) []gobRedRuleStatScope {
	scopes := []gobRedRuleStatScope{{hit.Key, hit.EntrySource, hit.ExitHorizon, hit.Version}}
	switch hit.EntrySource {
	case "realtime_open":
		scopes = append(scopes, gobRedRuleStatScope{hit.Key, "daily_open", hit.ExitHorizon, hit.Version})
	case "realtime_close":
		scopes = append(scopes, gobRedRuleStatScope{hit.Key, "daily_close", hit.ExitHorizon, hit.Version})
	}
	return scopes
}

func readGobRedReplayCache(path, date string) (*t0DailyCachePayload, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	var payload t0DailyCachePayload
	if err := gob.NewDecoder(f).Decode(&payload); err != nil {
		return nil, err
	}
	if (payload.DataSchemaVersion != 0 && payload.DataSchemaVersion != t0DailyCacheSchemaVersion) ||
		payload.TradeDate != date || len(payload.Stocks) == 0 || len(payload.Daily) == 0 {
		return nil, fmt.Errorf("Gob %s 不完整: schema=%d date=%s pool=%v stocks=%d daily=%d",
			date, payload.DataSchemaVersion, payload.TradeDate, payload.StockPoolComplete,
			len(payload.Stocks), len(payload.Daily))
	}
	return &payload, nil
}

// ReplayGobRedRuleStats is read-only. T0 observations come from that day's
// Gob. T+1 observations use the next session date present in the cached bars,
// falling back to its dated Gob snapshot when the T0 cache lacks that bar.
func ReplayGobRedRuleStats(cacheRoot, start, end, batchID string) (
	[]models.T0RedRuleStat, []GobRedReplayCase, GobRedReplayCoverage, error,
) {
	var coverage GobRedReplayCoverage
	if _, err := normalizeT0TradeDate(start); err != nil {
		return nil, nil, coverage, err
	}
	if _, err := normalizeT0TradeDate(end); err != nil {
		return nil, nil, coverage, err
	}
	if end < start || end >= time.Now().In(chinaLocation()).Format("2006-01-02") {
		return nil, nil, coverage, fmt.Errorf("历史回放结束日必须早于今天且不早于开始日")
	}
	dir := filepath.Join(cacheRoot, "t0", "daily")
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, nil, coverage, err
	}
	const prefix, suffix = "t0_daily_cache_", ".gob"
	dates := make([]string, 0, len(entries))
	for _, entry := range entries {
		name := entry.Name()
		if entry.IsDir() || !strings.HasPrefix(name, prefix) || !strings.HasSuffix(name, suffix) {
			continue
		}
		date := strings.TrimSuffix(strings.TrimPrefix(name, prefix), suffix)
		if _, err := normalizeT0TradeDate(date); err == nil && !isT0WeekendDate(date) {
			dates = append(dates, date)
		}
	}
	sort.Strings(dates)
	var samples []gobRedStatSample
	var cases []GobRedReplayCase
	missingByScope := make(map[gobRedRuleStatScope]int)
	for _, date := range dates {
		if date < start || date > end {
			continue
		}
		coverage.Dates++
		cache, err := readGobRedReplayCache(filepath.Join(dir, prefix+date+suffix), date)
		if err != nil {
			if coverage.Rejected == nil {
				coverage.Rejected = make(map[string]string)
			}
			coverage.Rejected[date] = err.Error()
			continue
		}
		if cache.StockPoolComplete {
			coverage.PoolDays++
		}
		missingDailyStocks := missingGobRedDailyStocks(cache.Stocks, cache.Daily)
		coverage.MissingDailyStocks += missingDailyStocks
		if cache.StockPoolComplete && cache.DataSchemaVersion == t0DailyCacheSchemaVersion &&
			missingDailyStocks == 0 {
			coverage.CompleteDays++
		}
		candidates := buildGobRedPreopenCandidates(date, cache.Stocks, cache.Daily, approvedGobRedRules())
		candidateStocks := resultsToT0Stocks(candidates)
		quotes, _, _ := buildGobRedOpeningQuotes(date, candidateStocks, cache.Daily)
		results, _ := matchGobRedRules(date, candidateStocks, cache.Daily, quotes, "daily_open", approvedGobRedRules())
		nextDate := firstDailyDateAfter(cache.Daily, date)
		var nextCache *t0DailyCachePayload
		nextCacheChecked := false
		loadNextCache := func() *t0DailyCachePayload {
			if nextCacheChecked {
				return nextCache
			}
			nextCacheChecked = true
			if nextDate == "" {
				coverage.MissingNextCacheDays++
				return nil
			}
			loaded, err := readGobRedReplayCache(filepath.Join(dir, prefix+nextDate+suffix), nextDate)
			if err != nil || loaded.DataSchemaVersion != t0DailyCacheSchemaVersion ||
				!loaded.StockPoolComplete || missingGobRedDailyStocks(loaded.Stocks, loaded.Daily) > 0 {
				coverage.MissingNextCacheDays++
				return nil
			}
			nextCache = loaded
			return nextCache
		}
		for _, result := range results {
			code := t0ShortCodeFromResultCode(result.StockCode)
			var t0High, t0Close, nextClose, nextHigh float64
			for _, bar := range cache.Daily[code] {
				if bar.Date == date {
					t0High = bar.High
					t0Close = bar.Close
				}
				if bar.Date == nextDate {
					nextClose = bar.Close
					nextHigh = bar.High
				}
			}
			for _, hit := range result.RedRuleHits {
				coverage.Matched++
				scope := gobRedRuleStatScope{hit.Key, hit.EntrySource, hit.ExitHorizon, hit.Version}
				if (hit.ExitHorizon == "T+1_close" && nextClose <= 0) ||
					(hit.ExitHorizon == "T+1_target_2_5" && (nextClose <= 0 || nextHigh <= 0)) {
					if nextCache := loadNextCache(); nextCache != nil {
						for _, bar := range nextCache.Daily[code] {
							if bar.Date == nextDate {
								if nextClose <= 0 {
									nextClose = bar.Close
								}
								if nextHigh <= 0 {
									nextHigh = bar.High
								}
								break
							}
						}
					}
				}
				exitPrice, exitDate := 0.0, ""
				switch hit.ExitHorizon {
				case "T0_high":
					exitPrice, exitDate = t0High, date
					if exitPrice <= 0 {
						coverage.MissingT0++
						missingByScope[scope]++
						continue
					}
				case "T0_close":
					exitPrice, exitDate = t0Close, date
					if exitPrice <= 0 {
						coverage.MissingT0++
						missingByScope[scope]++
						continue
					}
				case "T+1_close":
					exitPrice, exitDate = nextClose, nextDate
					if exitPrice <= 0 {
						coverage.MissingT1++
						missingByScope[scope]++
						continue
					}
				case "T+1_target_2_5":
					exitPrice, exitDate = nextClose, nextDate
					if exitPrice <= 0 || nextHigh <= 0 {
						coverage.MissingT1++
						missingByScope[scope]++
						continue
					}
				default:
					continue
				}
				coverage.Observed++
				samples = append(samples, gobRedStatSample{
					Date: date, StockCode: code, Hit: hit, T0High: t0High, T0Close: t0Close,
					NextClose: nextClose, NextHigh: nextHigh,
				})
				cases = append(cases, GobRedReplayCase{
					Date: date, StockCode: code, RuleKey: hit.Key, NextDate: nextDate,
					ExitDate: exitDate, ExitHorizon: hit.ExitHorizon,
					PnL: (exitPrice/gobRedRuleEntryPrice(hit) - 1) * 100,
				})
			}
		}
	}
	dataVersion := fmt.Sprintf("gob-schema-%d-t0-close-next-session-red-v7", t0DailyCacheSchemaVersion)
	if coverage.CompleteDays != coverage.Dates || coverage.MissingT0 > 0 || coverage.MissingT1 > 0 || coverage.MissingNextCacheDays > 0 {
		dataVersion = "gob-mixed-t0-close-next-session-red-v7"
	}
	stats := aggregateGobRedRuleStats(samples, start, end, dataVersion, batchID)
	statIndexes := make(map[gobRedRuleStatScope]int, len(stats))
	for i := range stats {
		scope := gobRedRuleStatScope{
			stats[i].RuleKey, stats[i].EntrySource, stats[i].ExitHorizon, stats[i].RuleVersion,
		}
		statIndexes[scope] = i
		if missingByScope[scope] > 0 {
			stats[i].Status = "partial"
		}
	}
	for scope := range missingByScope {
		if _, exists := statIndexes[scope]; exists {
			continue
		}
		stats = append(stats, models.T0RedRuleStat{
			RuleKey: scope.ruleKey, RuleVersion: scope.ruleVersion,
			EntrySource: scope.entrySource, ExitHorizon: scope.exitHorizon,
			PeriodKey: "all", DataVersion: dataVersion, BatchID: batchID, Status: "partial",
		})
	}
	if coverage.CompleteDays != coverage.Dates {
		for i := range stats {
			stats[i].Status = "partial"
		}
	}
	return stats, cases, coverage, nil
}

func missingGobRedDailyStocks(stocks []t0Stock, daily map[string][]dailyBar) int {
	missing := 0
	for _, stock := range stocks {
		if code := strings.TrimSpace(stock.ShortCode); code == "" || len(daily[code]) == 0 {
			missing++
		}
	}
	return missing
}

func firstDailyDateAfter(daily map[string][]dailyBar, tradeDate string) string {
	date := ""
	for _, bars := range daily {
		for _, bar := range bars {
			if bar.Date > tradeDate && (date == "" || bar.Date < date) {
				date = bar.Date
			}
		}
	}
	return date
}

type T0RedRuleStatView struct {
	SampleCount   int     `json:"sample_count"`
	PositiveCount int     `json:"positive_count"`
	TargetCount   int     `json:"target_count"`
	ProfitWinRate float64 `json:"profit_win_rate"`
	TargetRate    float64 `json:"target_rate"`
	AvgPnL        float64 `json:"avg_pnl"`
	DateStart     string  `json:"date_start"`
	DateEnd       string  `json:"date_end"`
	EntrySource   string  `json:"entry_source"`
	ExitHorizon   string  `json:"exit_horizon"`
	DataVersion   string  `json:"data_version"`
	BatchID       string  `json:"batch_id"`
}

type gobRedStatSample struct {
	Date      string
	StockCode string
	Hit       T0RedRuleHit
	T0High    float64
	T0Close   float64
	NextClose float64
	NextHigh  float64
}

func gobRedRuleEntryPrice(hit T0RedRuleHit) float64 {
	if hit.ExitHorizon == "T+1_target_2_5" {
		return hit.EntryClose
	}
	return hit.EntryOpen
}

func aggregateGobRedRuleStats(samples []gobRedStatSample, start, end, dataVersion, batchID string) []models.T0RedRuleStat {
	grouped := make(map[gobRedRuleStatScope]*models.T0RedRuleStat)
	seen := make(map[string]struct{})
	for _, sample := range samples {
		hit := sample.Hit
		exitPrice := 0.0
		switch hit.ExitHorizon {
		case "T0_high":
			exitPrice = sample.T0High
		case "T0_close":
			exitPrice = sample.T0Close
		case "T+1_close":
			exitPrice = sample.NextClose
		case "T+1_target_2_5":
			exitPrice = sample.NextClose
		default:
			continue
		}
		entryPrice := gobRedRuleEntryPrice(hit)
		if sample.Date < start || sample.Date > end || isT0WeekendDate(sample.Date) ||
			entryPrice <= 0 || exitPrice <= 0 || hit.Key == "" || sample.StockCode == "" {
			continue
		}
		identity := sample.Date + ":" + sample.StockCode + ":" + hit.Key + ":" +
			strconv.Itoa(hit.Version) + ":" + hit.EntrySource + ":" + hit.ExitHorizon
		if _, exists := seen[identity]; exists {
			continue
		}
		seen[identity] = struct{}{}
		key := gobRedRuleStatScope{hit.Key, hit.EntrySource, hit.ExitHorizon, hit.Version}
		stat := grouped[key]
		if stat == nil {
			stat = &models.T0RedRuleStat{
				RuleKey: hit.Key, RuleVersion: hit.Version, EntrySource: hit.EntrySource,
				ExitHorizon: hit.ExitHorizon, PeriodKey: "all", DataVersion: dataVersion,
				BatchID: batchID, Status: "verified",
			}
			grouped[key] = stat
		}
		pnl := (exitPrice/entryPrice - 1) * 100
		stat.SampleCount++
		stat.AvgPnL += pnl
		if pnl > 0 {
			stat.PositiveCount++
		}
		targetReached := ((hit.ExitHorizon == "T0_close" || hit.ExitHorizon == "T0_high") && pnl > 2.5+1e-9) ||
			(hit.ExitHorizon == "T+1_close" && pnl+1e-9 >= 2.5)
		if hit.TargetReturnPct > 0 {
			targetReached = pnl+1e-9 >= hit.TargetReturnPct
		}
		if hit.ExitHorizon == "T+1_target_2_5" {
			targetReached = gobRedT0CloseTargetReached(sample.NextHigh, gobRedT0CloseTargetPrice(entryPrice))
		}
		if targetReached {
			stat.TargetCount++
		}
		if stat.DateStart == "" || sample.Date < stat.DateStart {
			stat.DateStart = sample.Date
		}
		if sample.Date > stat.DateEnd {
			stat.DateEnd = sample.Date
		}
	}
	stats := make([]models.T0RedRuleStat, 0, len(grouped))
	for _, stat := range grouped {
		stat.ProfitWinRate = float64(stat.PositiveCount) / float64(stat.SampleCount) * 100
		stat.TargetRate = float64(stat.TargetCount) / float64(stat.SampleCount) * 100
		stat.AvgPnL /= float64(stat.SampleCount)
		stats = append(stats, *stat)
	}
	sort.Slice(stats, func(i, j int) bool {
		if stats[i].RuleKey != stats[j].RuleKey {
			return stats[i].RuleKey < stats[j].RuleKey
		}
		if stats[i].RuleVersion != stats[j].RuleVersion {
			return stats[i].RuleVersion < stats[j].RuleVersion
		}
		return stats[i].EntrySource < stats[j].EntrySource
	})
	return stats
}

func saveGobRedRuleStats(dao *gorm.DB, stats []models.T0RedRuleStat) error {
	for _, stat := range stats {
		if (stat.Status != "verified" && stat.Status != "partial") ||
			stat.SampleCount < 0 || (stat.Status == "verified" && stat.SampleCount == 0) {
			return fmt.Errorf("红策规则统计未核验，拒绝写入: %s", stat.RuleKey)
		}
	}
	return dao.Transaction(func(tx *gorm.DB) error {
		for _, stat := range stats {
			stat.UpdatedAt = time.Now()
			if err := tx.Clauses(clause.OnConflict{
				Columns: []clause.Column{{Name: "rule_key"}, {Name: "rule_version"}, {Name: "entry_source"},
					{Name: "exit_horizon"}, {Name: "period_key"}, {Name: "data_version"}, {Name: "batch_id"}},
				DoUpdates: clause.AssignmentColumns([]string{"date_start", "date_end", "sample_count", "positive_count",
					"target_count", "profit_win_rate", "target_rate", "avg_pnl", "status", "updated_at"}),
			}).Create(&stat).Error; err != nil {
				return err
			}
		}
		return nil
	})
}

// SaveGobRedRuleStats writes only the supplied aggregate table in the caller's
// explicitly selected database. Replay itself never writes a database.
func SaveGobRedRuleStats(dao *gorm.DB, stats []models.T0RedRuleStat) error {
	return saveGobRedRuleStats(dao, stats)
}

func enrichGobRedRuleStats(results []T0SelectionResult) []T0SelectionResult {
	if len(results) == 0 {
		return results
	}
	out := append([]T0SelectionResult(nil), results...)
	for i := range out {
		out[i].RedRuleHits = append([]T0RedRuleHit(nil), results[i].RedRuleHits...)
	}
	descriptions := make(map[gobRedRuleStatScope]string)
	descriptions[gobRedRuleStatScope{ruleKey: "gob_zt_zt_pb_t1_amplitude_6_10_shadow_le_1_v1", ruleVersion: 1}] = legacyGobZTztPbV1Description
	descriptions[gobRedRuleStatScope{ruleKey: "gob_zt_zt_pb_t1_amplitude_6_10_shadow_le_1_v2", ruleVersion: 2}] = legacyGobZTztPbV2Description
	currentRules := make(map[gobRedRuleStatScope]redGobRule)
	for _, rule := range approvedGobRedRules() {
		scope := gobRedRuleStatScope{ruleKey: rule.Key, ruleVersion: rule.Version}
		descriptions[scope] = rule.Description
		currentRules[scope] = rule
	}
	keys := make(map[string]struct{})
	for i := range out {
		for j := range out[i].RedRuleHits {
			hit := &out[i].RedRuleHits[j]
			scope := gobRedRuleStatScope{ruleKey: hit.Key, ruleVersion: hit.Version}
			if rule, ok := currentRules[scope]; ok {
				hit.Name = rule.Name
				hit.Description = rule.Description
			} else if hit.Description == "" {
				hit.Description = descriptions[scope]
			}
		}
	}
	if db.Dao == nil {
		return out
	}
	for _, result := range out {
		for _, hit := range result.RedRuleHits {
			keys[hit.Key] = struct{}{}
		}
	}
	if len(keys) == 0 {
		return out
	}
	ruleKeys := make([]string, 0, len(keys))
	for key := range keys {
		ruleKeys = append(ruleKeys, key)
	}
	var rows []models.T0RedRuleStat
	if err := db.Dao.Where("status IN ? AND period_key = ? AND rule_key IN ?", []string{"verified", "partial"}, "all", ruleKeys).
		Order("updated_at DESC, id DESC").Find(&rows).Error; err != nil {
		return out
	}
	latest := make(map[gobRedRuleStatScope]models.T0RedRuleStat)
	for _, row := range rows {
		key := gobRedRuleStatScope{row.RuleKey, row.EntrySource, row.ExitHorizon, row.RuleVersion}
		if _, exists := latest[key]; !exists {
			latest[key] = row
		}
	}
	for i := range out {
		for j := range out[i].RedRuleHits {
			hit := &out[i].RedRuleHits[j]
			if hit.StatsStatus == "preopen" {
				continue
			}
			var row models.T0RedRuleStat
			var ok bool
			for _, scope := range gobRedRuleStatLookupScopes(*hit) {
				if row, ok = latest[scope]; ok {
					break
				}
			}
			if !ok || row.Status != "verified" || row.SampleCount <= 0 {
				continue
			}
			hit.StatsStatus = "verified"
			hit.Stats = &T0RedRuleStatView{
				SampleCount: row.SampleCount, PositiveCount: row.PositiveCount, TargetCount: row.TargetCount,
				ProfitWinRate: row.ProfitWinRate, TargetRate: row.TargetRate, AvgPnL: row.AvgPnL,
				DateStart: row.DateStart, DateEnd: row.DateEnd, EntrySource: row.EntrySource,
				ExitHorizon: row.ExitHorizon, DataVersion: row.DataVersion, BatchID: row.BatchID,
			}
		}
	}
	return out
}
