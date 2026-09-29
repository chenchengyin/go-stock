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
	Date      string  `json:"date"`
	StockCode string  `json:"stock_code"`
	RuleKey   string  `json:"rule_key"`
	NextDate  string  `json:"next_date"`
	PnL       float64 `json:"pnl"`
}

type GobRedReplayCoverage struct {
	Dates                int               `json:"dates"`
	PoolDays             int               `json:"pool_days"`
	CompleteDays         int               `json:"complete_days"`
	MissingDailyStocks   int               `json:"missing_daily_stocks"`
	Matched              int               `json:"matched"`
	Observed             int               `json:"observed"`
	MissingT1            int               `json:"missing_t1"`
	MissingNextCacheDays int               `json:"missing_next_cache_days"`
	Rejected             map[string]string `json:"rejected,omitempty"`
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

// ReplayGobRedRuleStats is read-only. T0 admission uses that day's complete
// Gob; T+1 close comes only from the immediately following trading-day Gob.
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
		if _, err := normalizeT0TradeDate(date); err == nil {
			dates = append(dates, date)
		}
	}
	sort.Strings(dates)
	var samples []gobRedStatSample
	var cases []GobRedReplayCase
	for i, date := range dates {
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
		if missing := len(cache.Stocks) - len(cache.Daily); missing > 0 {
			coverage.MissingDailyStocks += missing
		}
		if cache.StockPoolComplete && cache.DataSchemaVersion == t0DailyCacheSchemaVersion &&
			len(cache.Daily) >= len(cache.Stocks) {
			coverage.CompleteDays++
		}
		candidates := buildGobRedPreopenCandidates(date, cache.Stocks, cache.Daily, approvedGobRedRules())
		candidateStocks := resultsToT0Stocks(candidates)
		quotes, _, _ := buildGobRedOpeningQuotes(date, candidateStocks, cache.Daily)
		results, _ := matchGobRedRules(date, candidateStocks, cache.Daily, quotes, "daily_open", approvedGobRedRules())
		var next *t0DailyCachePayload
		var nextDate string
		if i+1 < len(dates) {
			nextDate = dates[i+1]
			next, _ = readGobRedReplayCache(filepath.Join(dir, prefix+nextDate+suffix), nextDate)
			if next != nil && latestGobPriorDate(nextDate, next.Daily) != date {
				next = nil
			}
		}
		if next == nil && len(results) > 0 {
			coverage.MissingNextCacheDays++
		}
		for _, result := range results {
			code := t0ShortCodeFromResultCode(result.StockCode)
			var nextClose float64
			if next != nil {
				for _, bar := range next.Daily[code] {
					if bar.Date == nextDate {
						nextClose = bar.Close
						break
					}
				}
			}
			for _, hit := range result.RedRuleHits {
				coverage.Matched++
				if nextClose <= 0 {
					coverage.MissingT1++
					continue
				}
				coverage.Observed++
				samples = append(samples, gobRedStatSample{
					Date: date, StockCode: code, Hit: hit, NextClose: nextClose,
				})
				cases = append(cases, GobRedReplayCase{
					Date: date, StockCode: code, RuleKey: hit.Key, NextDate: nextDate,
					PnL: (nextClose/hit.EntryOpen - 1) * 100,
				})
			}
		}
	}
	dataVersion := fmt.Sprintf("gob-schema-%d-red-v1", t0DailyCacheSchemaVersion)
	if coverage.CompleteDays != coverage.Dates || coverage.MissingNextCacheDays > 0 {
		dataVersion = "gob-mixed-legacy-red-v1"
	}
	stats := aggregateGobRedRuleStats(samples, start, end, dataVersion, batchID)
	if coverage.CompleteDays != coverage.Dates {
		for i := range stats {
			stats[i].Status = "partial"
		}
	}
	return stats, cases, coverage, nil
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
	NextClose float64
}

func aggregateGobRedRuleStats(samples []gobRedStatSample, start, end, dataVersion, batchID string) []models.T0RedRuleStat {
	type scope struct {
		key, source, horizon string
		version              int
	}
	grouped := make(map[scope]*models.T0RedRuleStat)
	seen := make(map[string]struct{})
	for _, sample := range samples {
		hit := sample.Hit
		if sample.Date < start || sample.Date > end || hit.EntryOpen <= 0 || sample.NextClose <= 0 ||
			hit.ExitHorizon != "T+1_close" || hit.Key == "" || sample.StockCode == "" {
			continue
		}
		identity := sample.Date + ":" + sample.StockCode + ":" + hit.Key + ":" +
			strconv.Itoa(hit.Version) + ":" + hit.EntrySource + ":" + hit.ExitHorizon
		if _, exists := seen[identity]; exists {
			continue
		}
		seen[identity] = struct{}{}
		key := scope{hit.Key, hit.EntrySource, hit.ExitHorizon, hit.Version}
		stat := grouped[key]
		if stat == nil {
			stat = &models.T0RedRuleStat{
				RuleKey: hit.Key, RuleVersion: hit.Version, EntrySource: hit.EntrySource,
				ExitHorizon: hit.ExitHorizon, PeriodKey: "all", DataVersion: dataVersion,
				BatchID: batchID, Status: "verified",
			}
			grouped[key] = stat
		}
		pnl := (sample.NextClose/hit.EntryOpen - 1) * 100
		stat.SampleCount++
		stat.AvgPnL += pnl
		if pnl > 0 {
			stat.PositiveCount++
		}
		if sample.NextClose >= hit.EntryOpen*(1+2.5/100) {
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
		if stat.Status != "verified" || stat.SampleCount <= 0 {
			return fmt.Errorf("红策规则统计未核验，拒绝写入: %s", stat.RuleKey)
		}
	}
	for _, stat := range stats {
		stat.UpdatedAt = time.Now()
		if err := dao.Clauses(clause.OnConflict{
			Columns: []clause.Column{{Name: "rule_key"}, {Name: "rule_version"}, {Name: "entry_source"},
				{Name: "exit_horizon"}, {Name: "period_key"}, {Name: "data_version"}, {Name: "batch_id"}},
			DoUpdates: clause.AssignmentColumns([]string{"date_start", "date_end", "sample_count", "positive_count",
				"target_count", "profit_win_rate", "target_rate", "avg_pnl", "status", "updated_at"}),
		}).Create(&stat).Error; err != nil {
			return err
		}
	}
	return nil
}

// SaveGobRedRuleStats writes only the supplied aggregate table in the caller's
// explicitly selected database. Replay itself never writes a database.
func SaveGobRedRuleStats(dao *gorm.DB, stats []models.T0RedRuleStat) error {
	return saveGobRedRuleStats(dao, stats)
}

func enrichGobRedRuleStats(results []T0SelectionResult) []T0SelectionResult {
	if db.Dao == nil || len(results) == 0 {
		return results
	}
	keys := make(map[string]struct{})
	for _, result := range results {
		for _, hit := range result.RedRuleHits {
			keys[hit.Key] = struct{}{}
		}
	}
	if len(keys) == 0 {
		return results
	}
	ruleKeys := make([]string, 0, len(keys))
	for key := range keys {
		ruleKeys = append(ruleKeys, key)
	}
	var rows []models.T0RedRuleStat
	if err := db.Dao.Where("status = ? AND period_key = ? AND rule_key IN ?", "verified", "all", ruleKeys).
		Order("updated_at DESC, id DESC").Find(&rows).Error; err != nil {
		return results
	}
	type scope struct {
		key, source, horizon string
		version              int
	}
	latest := make(map[scope]models.T0RedRuleStat)
	for _, row := range rows {
		key := scope{row.RuleKey, row.EntrySource, row.ExitHorizon, row.RuleVersion}
		if _, exists := latest[key]; !exists && row.SampleCount > 0 {
			latest[key] = row
		}
	}
	out := append([]T0SelectionResult(nil), results...)
	for i := range out {
		out[i].RedRuleHits = append([]T0RedRuleHit(nil), results[i].RedRuleHits...)
		for j := range out[i].RedRuleHits {
			hit := &out[i].RedRuleHits[j]
			if hit.StatsStatus == "preopen" {
				continue
			}
			row, ok := latest[scope{hit.Key, hit.EntrySource, hit.ExitHorizon, hit.Version}]
			if !ok {
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
