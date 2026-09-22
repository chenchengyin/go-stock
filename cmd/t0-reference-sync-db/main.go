package main

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"log"
	"path/filepath"
	"sort"
	"strings"

	"go-stock/backend/analysis/candlepattern"
	"go-stock/backend/analysis/t0reference"
	"go-stock/backend/db"
	"go-stock/backend/flutter_api"
	"go-stock/backend/models"
	"gorm.io/gorm"
)

const dailyCachePrefix = "t0_daily_cache_"

type SyncSummary struct {
	Stocks       int
	Complete     int
	Incomplete   int
	Observations []t0reference.Observation
}

type ruleRuntime struct {
	compiled t0reference.CompiledRule
	stats    *t0reference.ReferenceStatsAccumulator
}

func main() {
	cacheRoot := flag.String("cache-root", "", "T0 daily cache root (default backend/data/cache)")
	dbPath := flag.String("db", "data/stock.db", "SQLite database path")
	from := flag.String("from", "", "first trade date, inclusive (YYYY-MM-DD)")
	to := flag.String("to", "", "last trade date, inclusive (YYYY-MM-DD)")
	flag.Parse()

	root := *cacheRoot
	if root == "" {
		var err error
		root, err = candlepattern.ResolveCacheRoot(".")
		if err != nil {
			log.Fatal(err)
		}
	}
	dates, err := listCacheDates(root, *from, *to)
	if err != nil {
		log.Fatal(err)
	}
	if len(dates) == 0 {
		log.Fatal("no daily cache files found")
	}

	runtimes, err := compileDefaultRules()
	if err != nil {
		log.Fatal(err)
	}
	db.Init(*dbPath)
	flutter_api.AutoMigrate()

	var total SyncSummary
	var rangeFingerprint strings.Builder
	for _, date := range dates {
		cache, loadErr := candlepattern.LoadDailyCache(root, date)
		if loadErr != nil {
			log.Fatal(loadErr)
		}
		sourceBatch, universeBatch := cacheIdentity(cache)
		summary, syncErr := syncCache(db.Dao, cache, sourceBatch, universeBatch)
		if syncErr != nil {
			log.Fatalf("sync %s: %v", date, syncErr)
		}
		for _, observation := range summary.Observations {
			for i := range runtimes {
				runtimes[i].stats.Add(observation)
			}
		}
		total.Stocks += summary.Stocks
		total.Complete += summary.Complete
		total.Incomplete += summary.Incomplete
		rangeFingerprint.WriteString(sourceBatch)
		rangeFingerprint.WriteByte('\n')
		fmt.Printf("%s stocks=%d complete=%d incomplete=%d\n", date, summary.Stocks, summary.Complete, summary.Incomplete)
	}

	batchID := rangeBatchID(dates[0], dates[len(dates)-1], rangeFingerprint.String())
	statCount, tierCounts, err := persistRulesAndStats(db.Dao, runtimes, batchID, dates)
	if err != nil {
		log.Fatal(err)
	}
	fmt.Printf("backfill dates=%d range=%s..%s stocks=%d complete=%d incomplete=%d rules=%d stats=%d batch=%s tiers=%v\n",
		len(dates), dates[0], dates[len(dates)-1], total.Stocks, total.Complete, total.Incomplete,
		len(runtimes), statCount, batchID, tierCounts)
}

func compileDefaultRules() ([]ruleRuntime, error) {
	definitions := t0reference.DefaultRuleDefinitions()
	runtimes := make([]ruleRuntime, 0, len(definitions))
	for _, definition := range definitions {
		compiled, err := t0reference.CompileRule(definition)
		if err != nil {
			return nil, fmt.Errorf("compile default rule %q: %w", definition.Name, err)
		}
		runtimes = append(runtimes, ruleRuntime{
			compiled: compiled,
			stats:    t0reference.NewReferenceStatsAccumulator(compiled),
		})
	}
	return runtimes, nil
}

func syncCache(dao *gorm.DB, cache *candlepattern.DailyCache, sourceBatch, universeBatch string) (SyncSummary, error) {
	if cache == nil || cache.TradeDate == "" {
		return SyncSummary{}, errors.New("daily cache is incomplete")
	}
	if sourceBatch == "" || universeBatch == "" {
		computedSource, computedUniverse := cacheIdentity(cache)
		if sourceBatch == "" {
			sourceBatch = computedSource
		}
		if universeBatch == "" {
			universeBatch = computedUniverse
		}
	}
	codes := cacheStockCodes(cache)
	requests := make([]t0reference.ObservationVersion, 0, len(codes))
	summary := SyncSummary{Stocks: len(codes)}
	for _, code := range codes {
		bars := cache.Daily[code]
		if len(bars) == 0 {
			// Some historical caches key the map by the short code while the
			// metadata uses Code; cacheStockCodes has already normalized this
			// case, so an absent value is an incomplete sample.
			summary.Incomplete++
			continue
		}
		snapshots := snapshotsFromDailyBars(bars)
		t0Bar, ok := findDailyBar(bars, cache.TradeDate)
		if !ok {
			summary.Incomplete++
			continue
		}
		observation, err := t0reference.BuildObservation(code, cache.TradeDate, snapshots,
			t0reference.Entry{Price: t0Bar.Open, Source: t0reference.EntrySourceDailyOpen})
		if err != nil {
			summary.Incomplete++
			continue
		}
		summary.Complete++
		summary.Observations = append(summary.Observations, observation)
		requests = append(requests, t0reference.ObservationVersion{
			Observation:   observation,
			SourceBatch:   sourceBatch,
			SourceHash:    stockSourceHash(code, bars),
			UniverseBatch: universeBatch,
		})
	}
	if err := t0reference.PersistObservations(dao, requests); err != nil {
		return SyncSummary{}, err
	}
	return summary, nil
}

func snapshotsFromDailyBars(bars []candlepattern.DailyBar) []t0reference.BarSnapshot {
	ordered := append([]candlepattern.DailyBar(nil), bars...)
	sort.SliceStable(ordered, func(i, j int) bool { return ordered[i].Date < ordered[j].Date })
	snapshots := make([]t0reference.BarSnapshot, 0, len(ordered))
	for i, bar := range ordered {
		prevClose := 0.0
		if i > 0 {
			prevClose = ordered[i-1].Close
		}
		snapshots = append(snapshots, t0reference.BarSnapshot{
			Date: bar.Date, PrevClose: prevClose, Open: bar.Open, High: bar.High,
			Low: bar.Low, Close: bar.Close, Volume: bar.Volume, AmountYi: bar.AmountYi,
		})
	}
	return snapshots
}

func findDailyBar(bars []candlepattern.DailyBar, date string) (candlepattern.DailyBar, bool) {
	for _, bar := range bars {
		if bar.Date == date {
			return bar, true
		}
	}
	return candlepattern.DailyBar{}, false
}

func cacheStockCodes(cache *candlepattern.DailyCache) []string {
	seen := make(map[string]bool)
	codes := make([]string, 0, len(cache.Stocks))
	for _, stock := range cache.Stocks {
		code := stock.ShortCode
		if code == "" {
			code = stock.Code
		}
		if code == "" || seen[code] {
			continue
		}
		seen[code] = true
		codes = append(codes, code)
	}
	sort.Strings(codes)
	return codes
}

type cacheFingerprintStock struct {
	Code string
	Bars []candlepattern.DailyBar
}

type cacheFingerprint struct {
	TradeDate string
	Stocks    []candlepattern.StockMeta
	Daily     []cacheFingerprintStock
}

func cacheIdentity(cache *candlepattern.DailyCache) (sourceBatch, universeBatch string) {
	codes := cacheStockCodes(cache)
	stocks := make([]candlepattern.StockMeta, 0, len(codes))
	metaByCode := make(map[string]candlepattern.StockMeta, len(codes))
	for _, stock := range cache.Stocks {
		code := stock.ShortCode
		if code == "" {
			code = stock.Code
		}
		if code != "" {
			metaByCode[code] = stock
		}
	}
	for _, code := range codes {
		stocks = append(stocks, metaByCode[code])
	}
	daily := make([]cacheFingerprintStock, 0, len(codes))
	for _, code := range codes {
		bars := append([]candlepattern.DailyBar(nil), cache.Daily[code]...)
		sort.SliceStable(bars, func(i, j int) bool { return bars[i].Date < bars[j].Date })
		daily = append(daily, cacheFingerprintStock{Code: code, Bars: bars})
	}
	payload := cacheFingerprint{TradeDate: cache.TradeDate, Stocks: stocks, Daily: daily}
	encoded, _ := json.Marshal(payload)
	fullHash := sha256.Sum256(encoded)
	fullHex := hex.EncodeToString(fullHash[:])

	universeEncoded, _ := json.Marshal(struct {
		TradeDate string
		Codes     []string
	}{cache.TradeDate, codes})
	universeHash := sha256.Sum256(universeEncoded)
	universeHex := hex.EncodeToString(universeHash[:])
	return "gob:" + cache.TradeDate + ":" + fullHex[:16], "pool:" + cache.TradeDate + ":" + universeHex[:16]
}

func stockSourceHash(code string, bars []candlepattern.DailyBar) string {
	ordered := append([]candlepattern.DailyBar(nil), bars...)
	sort.SliceStable(ordered, func(i, j int) bool { return ordered[i].Date < ordered[j].Date })
	payload, _ := json.Marshal(cacheFingerprintStock{Code: code, Bars: ordered})
	hash := sha256.Sum256(payload)
	return hex.EncodeToString(hash[:])
}

func rangeBatchID(dateStart, dateEnd, fingerprint string) string {
	hash := sha256.Sum256([]byte(fingerprint))
	return "gob-range:" + dateStart + ":" + dateEnd + ":" + hex.EncodeToString(hash[:])[:16]
}

func listCacheDates(cacheRoot, from, to string) ([]string, error) {
	paths, err := filepath.Glob(filepath.Join(cacheRoot, "t0", "daily", dailyCachePrefix+"*.gob"))
	if err != nil {
		return nil, err
	}
	dates := make([]string, 0, len(paths))
	for _, path := range paths {
		name := filepath.Base(path)
		date := strings.TrimSuffix(strings.TrimPrefix(name, dailyCachePrefix), ".gob")
		if len(date) != len("2006-01-02") {
			continue
		}
		if from != "" && date < from {
			continue
		}
		if to != "" && date > to {
			continue
		}
		dates = append(dates, date)
	}
	sort.Strings(dates)
	return dates, nil
}

func persistRulesAndStats(dao *gorm.DB, runtimes []ruleRuntime, batchID string, dates []string) (int, map[string]int, error) {
	years := make([]string, 0)
	seenYears := make(map[string]bool)
	for _, date := range dates {
		if len(date) >= 4 && !seenYears[date[:4]] {
			seenYears[date[:4]] = true
			years = append(years, date[:4])
		}
	}
	sort.Strings(years)
	statCount := 0
	tierCounts := make(map[string]int)
	err := dao.Transaction(func(tx *gorm.DB) error {
		for _, name := range t0reference.DeprecatedRuleNames() {
			if err := tx.Model(&models.T0ReferenceRule{}).
				Where("name = ?", name).
				Update("enabled", false).Error; err != nil {
				return err
			}
		}
		for _, runtime := range runtimes {
			definition := runtime.compiled.Definition
			if err := tx.Model(&models.T0ReferenceRule{}).
				Where("name = ? AND rule_key <> ?", definition.Name, runtime.compiled.RuleKey).
				Update("enabled", false).Error; err != nil {
				return err
			}
			row := models.T0ReferenceRule{
				RuleKey:           runtime.compiled.RuleKey,
				RuleKind:          definition.RuleKind,
				Name:              definition.Name,
				LookbackStart:     -3,
				LookbackEnd:       -1,
				DefinitionVersion: definition.DefinitionVersion,
				ConditionJSON:     definition.ConditionJSON,
				EntryJSON:         definition.EntryJSON,
				ManualRank:        definition.ManualRank,
				MinSamples:        definition.MinSamples,
				DeepRed:           definition.DeepRed,
				RedEntry:          definition.RedEntry,
				Enabled:           true,
			}
			var existing models.T0ReferenceRule
			findErr := tx.Where("rule_key = ?", row.RuleKey).First(&existing).Error
			switch {
			case findErr == nil:
				row.ID = existing.ID
				if err := tx.Model(&existing).Updates(row).Error; err != nil {
					return err
				}
				// GORM omits false zero-values from struct Updates. The marker is
				// intentionally reversible when a candidate is removed from the
				// selected deep-red set, so write it explicitly.
				if err := tx.Model(&existing).Update("deep_red", row.DeepRed).Error; err != nil {
					return err
				}
				if err := tx.Model(&existing).Update("red_entry", row.RedEntry).Error; err != nil {
					return err
				}
			case errors.Is(findErr, gorm.ErrRecordNotFound):
				if err := tx.Create(&row).Error; err != nil {
					return err
				}
			default:
				return findErr
			}

			periods := []string{"all"}
			periods = append(periods, years...)
			for _, period := range periods {
				stat := runtime.stats.Stat(period)
				statRow := models.T0ReferenceRuleStat{
					RuleID: row.ID, BatchID: batchID, PeriodKey: stat.PeriodKey,
					DateStart: stat.DateStart, DateEnd: stat.DateEnd, SampleCount: stat.SampleCount,
					ProfitWinRate: stat.ProfitWinRate, TargetRate: stat.TargetRate,
					LossRate: stat.LossRate, AvgPnL: stat.AvgPnL, MedianPnL: stat.MedianPnL,
					ResearchTier: stat.ResearchTier,
				}
				var existingStat models.T0ReferenceRuleStat
				findStatErr := tx.Where("rule_id = ? AND batch_id = ? AND period_key = ?", row.ID, batchID, stat.PeriodKey).First(&existingStat).Error
				switch {
				case findStatErr == nil:
					statRow.ID = existingStat.ID
					if err := tx.Model(&existingStat).Updates(statRow).Error; err != nil {
						return err
					}
				case errors.Is(findStatErr, gorm.ErrRecordNotFound):
					if err := tx.Create(&statRow).Error; err != nil {
						return err
					}
				default:
					return findStatErr
				}
				statCount++
				tierCounts[stat.ResearchTier]++
			}
		}
		return nil
	})
	return statCount, tierCounts, err
}
