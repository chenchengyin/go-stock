package flutter_api

import (
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

type gobRedArchive struct {
	Date           string              `json:"date"`
	SavedAt        string              `json:"saved_at"`
	EntrySource    string              `json:"entry_source"`
	RuleSetVersion string              `json:"rule_set_version,omitempty"`
	Complete       bool                `json:"complete"`
	Results        []T0SelectionResult `json:"results"`
}

type gobRedSellMonitor struct {
	TradeDate   string  `json:"trade_date"`
	StockCode   string  `json:"stock_code"`
	StockName   string  `json:"stock_name"`
	EntryClose  float64 `json:"entry_close"`
	TargetPrice float64 `json:"target_price"`
}

func gobRedSellMonitorsForTradeDate(tradeDate string) []gobRedSellMonitor {
	entryDate := ""
	for _, date := range listGobRedAvailableDates() {
		if date < tradeDate {
			entryDate = date
			break
		}
	}
	if entryDate == "" {
		return nil
	}
	archive, ok := loadGobRedArchive(entryDate)
	if !ok || !archive.Complete || archive.RuleSetVersion != currentGobRedRuleSetVersion() {
		return nil
	}
	monitors := make([]gobRedSellMonitor, 0)
	for _, result := range archive.Results {
		for _, hit := range result.RedRuleHits {
			if hit.Key != "gob_zt_t1_bear_t0_bear_close_entry_target_2_5_v1" ||
				hit.Version != 1 || hit.EntryClose <= 0 || hit.TargetPrice <= 0 {
				continue
			}
			monitors = append(monitors, gobRedSellMonitor{
				TradeDate: entryDate, StockCode: result.StockCode, StockName: result.StockName,
				EntryClose: hit.EntryClose, TargetPrice: hit.TargetPrice,
			})
		}
	}
	return monitors
}

func gobRedArchivePath(tradeDate string) string {
	return filepath.Join(t0CacheRootPath, "t0", "selection", "t0_red_gob_"+tradeDate+".json")
}

func listGobRedAvailableDates() []string {
	seen := make(map[string]struct{})
	for _, date := range listSelectionArchiveDates() {
		seen[date] = struct{}{}
	}
	for _, source := range []struct{ dir, prefix, suffix string }{
		{filepath.Join(t0CacheRootPath, "t0", "daily"), "t0_daily_cache_", ".gob"},
		{filepath.Join(t0CacheRootPath, "t0", "selection"), "t0_red_gob_", ".json"},
	} {
		entries, err := os.ReadDir(source.dir)
		if err != nil {
			continue
		}
		for _, entry := range entries {
			name := entry.Name()
			if entry.IsDir() || !strings.HasPrefix(name, source.prefix) || !strings.HasSuffix(name, source.suffix) {
				continue
			}
			date := strings.TrimSuffix(strings.TrimPrefix(name, source.prefix), source.suffix)
			if normalized, err := normalizeT0TradeDate(date); err == nil && normalized == date && !isT0WeekendDate(date) {
				seen[date] = struct{}{}
			}
		}
	}
	dates := make([]string, 0, len(seen))
	for date := range seen {
		dates = append(dates, date)
	}
	sort.Sort(sort.Reverse(sort.StringSlice(dates)))
	return dates
}

func loadGobRedArchive(tradeDate string) (*gobRedArchive, bool) {
	if normalized, err := normalizeT0TradeDate(tradeDate); err != nil || normalized != tradeDate || isT0WeekendDate(tradeDate) {
		return nil, false
	}
	data, err := os.ReadFile(gobRedArchivePath(tradeDate))
	if err != nil {
		return nil, false
	}
	var archive gobRedArchive
	if err := json.Unmarshal(data, &archive); err != nil || archive.Date != tradeDate {
		return nil, false
	}
	return &archive, true
}

func saveGobRedArchive(archive *gobRedArchive) error {
	if archive == nil {
		return fmt.Errorf("红策独立归档为空")
	}
	if normalized, err := normalizeT0TradeDate(archive.Date); err != nil || normalized != archive.Date || isT0WeekendDate(archive.Date) {
		return fmt.Errorf("红策独立归档日期无效: %q", archive.Date)
	}
	if old, ok := loadGobRedArchive(archive.Date); ok {
		if old.RuleSetVersion == currentGobRedRuleSetVersion() {
			if old.Complete {
				merged, changed := mergeGobRedArchiveResults(old.Results, archive.Results)
				if !changed {
					return nil
				}
				copyArchive := *old
				copyArchive.Results = merged
				archive = &copyArchive
			} else if !archive.Complete && len(old.Results) >= len(archive.Results) {
				return nil
			}
		}
	}
	if err := ensureT0CacheDirs(); err != nil {
		return err
	}
	copyArchive := *archive
	if copyArchive.RuleSetVersion == "" {
		copyArchive.RuleSetVersion = currentGobRedRuleSetVersion()
	}
	if copyArchive.SavedAt == "" {
		copyArchive.SavedAt = time.Now().Format(time.RFC3339)
	}
	if copyArchive.Results == nil {
		copyArchive.Results = []T0SelectionResult{}
	}
	data, err := json.MarshalIndent(&copyArchive, "", "  ")
	if err != nil {
		return err
	}
	dir := filepath.Dir(gobRedArchivePath(archive.Date))
	f, err := os.CreateTemp(dir, "t0_red_gob_*.tmp")
	if err != nil {
		return err
	}
	defer os.Remove(f.Name())
	if _, err := f.Write(data); err != nil {
		f.Close()
		return err
	}
	if err := f.Close(); err != nil {
		return err
	}
	return os.Rename(f.Name(), gobRedArchivePath(archive.Date))
}

func mergeGobRedArchiveResults(oldResults, newResults []T0SelectionResult) ([]T0SelectionResult, bool) {
	merged := append([]T0SelectionResult(nil), oldResults...)
	byCode := make(map[string]int, len(merged))
	for i, result := range merged {
		byCode[result.StockCode] = i
	}
	changed := false
	for _, result := range newResults {
		i, found := byCode[result.StockCode]
		if !found {
			byCode[result.StockCode] = len(merged)
			merged = append(merged, result)
			changed = true
			continue
		}
		for _, hit := range result.RedRuleHits {
			foundHit := false
			for j, existing := range merged[i].RedRuleHits {
				if existing.Key != hit.Key || existing.Version != hit.Version {
					continue
				}
				foundHit = true
				if hit.EntryClose > 0 && (existing.EntryClose != hit.EntryClose || existing.TargetPrice != hit.TargetPrice) {
					merged[i].RedRuleHits[j] = hit
					changed = true
				}
				break
			}
			if !foundHit {
				merged[i].RedRuleHits = append(merged[i].RedRuleHits, hit)
				changed = true
			}
		}
		if changed {
			merged[i].DisplayRuleHits = mergeT0DisplayRuleHits(merged[i].DisplayRuleHits, result.DisplayRuleHits)
		}
	}
	return merged, changed
}

type GobRedArchiveBackfillReport struct {
	Dates                 int            `json:"dates"`
	CompleteDays          int            `json:"complete_days"`
	IncompleteDays        int            `json:"incomplete_days"`
	SkippedDays           int            `json:"skipped_days"`
	ResultStockDays       int            `json:"result_stock_days"`
	RuleHits              int            `json:"rule_hits"`
	RuleHitsByKey         map[string]int `json:"rule_hits_by_key"`
	PreservedCompleteDays int            `json:"preserved_complete_days"`
}

// BackfillGobRedArchives writes historical daily_open snapshots from local Gob
// caches. Incomplete inputs remain explicitly incomplete; confirmed snapshots
// are preserved by saveGobRedArchive.
func BackfillGobRedArchives(cacheRoot, start, end string) (GobRedArchiveBackfillReport, error) {
	var report GobRedArchiveBackfillReport
	if normalized, err := normalizeT0TradeDate(start); err != nil || normalized != start {
		return report, fmt.Errorf("开始日期无效: %q", start)
	}
	if normalized, err := normalizeT0TradeDate(end); err != nil || normalized != end || end < start ||
		end >= time.Now().In(chinaLocation()).Format("2006-01-02") {
		return report, fmt.Errorf("结束日期必须是早于今天且不早于开始日期的有效日期")
	}

	oldRoot := t0CacheRootPath
	if cacheRoot != "" {
		abs, err := filepath.Abs(cacheRoot)
		if err != nil {
			return report, err
		}
		t0CacheRootPath = abs
	}
	defer func() { t0CacheRootPath = oldRoot }()

	dir := filepath.Join(t0CacheRootPath, "t0", "daily")
	entries, err := os.ReadDir(dir)
	if err != nil {
		return report, err
	}
	const prefix, suffix = "t0_daily_cache_", ".gob"
	dates := make([]string, 0, len(entries))
	for _, entry := range entries {
		name := entry.Name()
		if entry.IsDir() || !strings.HasPrefix(name, prefix) || !strings.HasSuffix(name, suffix) {
			continue
		}
		date := strings.TrimSuffix(strings.TrimPrefix(name, prefix), suffix)
		if normalized, err := normalizeT0TradeDate(date); err == nil && normalized == date &&
			!isT0WeekendDate(date) && date >= start && date <= end {
			dates = append(dates, date)
		}
	}
	sort.Strings(dates)
	report.RuleHitsByKey = make(map[string]int)
	stockDays := make(map[string]struct{})
	for _, date := range dates {
		archive, exists := loadGobRedArchive(date)
		if exists && archive.Complete && archive.RuleSetVersion == currentGobRedRuleSetVersion() {
			report.PreservedCompleteDays++
		} else {
			results, _, sourceStatus, incomplete := archivedGobRedResults(date)
			if sourceStatus == "unavailable" || sourceStatus == "stale_snapshot" {
				report.SkippedDays++
				continue
			}
			archive = &gobRedArchive{
				Date: date, EntrySource: "daily_open", RuleSetVersion: currentGobRedRuleSetVersion(),
				Complete: !incomplete, Results: results,
			}
			if err := saveGobRedArchive(archive); err != nil {
				return report, fmt.Errorf("保存 %s 红策快照失败: %w", date, err)
			}
			archive, _ = loadGobRedArchive(date)
		}
		if archive == nil {
			report.SkippedDays++
			continue
		}
		report.Dates++
		if archive.Complete {
			report.CompleteDays++
		} else {
			report.IncompleteDays++
		}
		for _, result := range archive.Results {
			stockDays[date+":"+result.StockCode] = struct{}{}
			for _, hit := range result.RedRuleHits {
				report.RuleHits++
				report.RuleHitsByKey[hit.Key]++
			}
		}
	}
	report.ResultStockDays = len(stockDays)
	return report, nil
}

type GobRedDailyCacheRefreshReport struct {
	Dates           int `json:"dates"`
	SkippedDays     int `json:"skipped_days"`
	CandidateStocks int `json:"candidate_stocks"`
	RequestedStocks int `json:"requested_stocks"`
	RefreshedStocks int `json:"refreshed_stocks"`
	FailedStocks    int `json:"failed_stocks"`
	T0BarsAdded     int `json:"t0_bars_added"`
	T1BarsAdded     int `json:"t1_bars_added"`
}

// RefreshGobRedDailyCaches fills missing historical bars using each date's
// cached point-in-time stock pool. It never replaces that pool with today's list.
func RefreshGobRedDailyCaches(cacheRoot, start, end string) (GobRedDailyCacheRefreshReport, error) {
	var report GobRedDailyCacheRefreshReport
	if normalized, err := normalizeT0TradeDate(start); err != nil || normalized != start {
		return report, fmt.Errorf("开始日期无效: %q", start)
	}
	if normalized, err := normalizeT0TradeDate(end); err != nil || normalized != end || end < start ||
		end >= time.Now().In(chinaLocation()).Format("2006-01-02") {
		return report, fmt.Errorf("结束日期必须是早于今天且不早于开始日期的有效日期")
	}

	oldRoot := t0CacheRootPath
	if cacheRoot != "" {
		abs, err := filepath.Abs(cacheRoot)
		if err != nil {
			return report, err
		}
		t0CacheRootPath = abs
	}
	defer func() { t0CacheRootPath = oldRoot }()

	dir := filepath.Join(t0CacheRootPath, "t0", "daily")
	entries, err := os.ReadDir(dir)
	if err != nil {
		return report, err
	}
	const prefix, suffix = "t0_daily_cache_", ".gob"
	dates := make([]string, 0, len(entries))
	for _, entry := range entries {
		name := entry.Name()
		if entry.IsDir() || !strings.HasPrefix(name, prefix) || !strings.HasSuffix(name, suffix) {
			continue
		}
		date := strings.TrimSuffix(strings.TrimPrefix(name, prefix), suffix)
		if normalized, err := normalizeT0TradeDate(date); err == nil && normalized == date &&
			!isT0WeekendDate(date) && date >= start && date <= end {
			dates = append(dates, date)
		}
	}
	sort.Strings(dates)
	for _, date := range dates {
		cache, err := readGobRedReplayCache(filepath.Join(dir, prefix+date+suffix), date)
		if err != nil || cache.DataSchemaVersion != t0DailyCacheSchemaVersion || !cache.StockPoolComplete {
			report.SkippedDays++
			continue
		}
		report.Dates++
		stocksByCode := make(map[string]t0Stock, len(cache.Stocks))
		for _, stock := range cache.Stocks {
			stocksByCode[stock.ShortCode] = stock
		}
		priorDate := latestGobPriorDate(date, cache.Daily)
		targets := make(map[string]t0Stock)
		candidateCodes := make(map[string]struct{})
		for _, stock := range cache.Stocks {
			bars := cache.Daily[stock.ShortCode]
			latest := ""
			for _, bar := range bars {
				if bar.Date < date && bar.Date > latest {
					latest = bar.Date
				}
			}
			if latest != priorDate {
				targets[stock.ShortCode] = stock
			}
		}

		addMissingCandidateBars := func() {
			candidates := buildGobRedPreopenCandidates(date, cache.Stocks, cache.Daily, approvedGobRedRules())
			nextDate := firstDailyDateAfter(cache.Daily, date)
			for _, candidate := range candidates {
				stockCode := t0ShortCodeFromResultCode(candidate.StockCode)
				if _, seen := candidateCodes[stockCode]; !seen {
					report.CandidateStocks++
					candidateCodes[stockCode] = struct{}{}
				}
				stock, ok := stocksByCode[stockCode]
				if !ok {
					continue
				}
				_, t0Index := dailyBarOnDate(cache.Daily[stockCode], date)
				needsT1 := false
				for _, hit := range candidate.RedRuleHits {
					if hit.ExitHorizon == "T+1_close" {
						needsT1 = true
						break
					}
				}
				_, t1Index := dailyBarOnDate(cache.Daily[stockCode], nextDate)
				if t0Index < 0 || (needsT1 && (nextDate == "" || t1Index < 0)) {
					targets[stockCode] = stock
				}
			}
		}
		addMissingCandidateBars()

		refreshTargets := func() bool {
			if len(targets) == 0 {
				return false
			}
			refreshStocks := make([]t0Stock, 0, len(targets))
			for _, stock := range targets {
				refreshStocks = append(refreshStocks, stock)
			}
			sort.Slice(refreshStocks, func(i, j int) bool {
				return refreshStocks[i].ShortCode < refreshStocks[j].ShortCode
			})
			targets = make(map[string]t0Stock)
			report.RequestedStocks += len(refreshStocks)
			fetched := fetchAllDailyKLine(refreshStocks, t0DailyFetchEndDate(date), nil)
			changed := false
			for _, stock := range refreshStocks {
				bars, ok := fetched[stock.ShortCode]
				if !ok {
					report.FailedStocks++
					continue
				}
				oldBars := cache.Daily[stock.ShortCode]
				_, oldT0Index := dailyBarOnDate(oldBars, date)
				nextDate := firstDailyDateAfter(cache.Daily, date)
				_, oldT1Index := dailyBarOnDate(oldBars, nextDate)
				merged := make(map[string]dailyBar, len(oldBars)+len(bars))
				for _, bar := range oldBars {
					merged[bar.Date] = bar
				}
				for _, bar := range bars {
					merged[bar.Date] = bar
				}
				allBars := make([]dailyBar, 0, len(merged))
				for _, bar := range merged {
					allBars = append(allBars, bar)
				}
				sort.Slice(allBars, func(i, j int) bool { return allBars[i].Date < allBars[j].Date })
				cache.Daily[stock.ShortCode] = allBars
				if _, index := dailyBarOnDate(allBars, date); oldT0Index < 0 && index >= 0 {
					report.T0BarsAdded++
				}
				if nextDate != "" {
					if _, index := dailyBarOnDate(allBars, nextDate); oldT1Index < 0 && index >= 0 {
						report.T1BarsAdded++
					}
				} else {
					for _, bar := range allBars {
						if bar.Date > date {
							if oldT1Index < 0 {
								report.T1BarsAdded++
							}
							break
						}
					}
				}
				report.RefreshedStocks++
				changed = true
			}
			return changed
		}

		changed := refreshTargets()
		addMissingCandidateBars()
		changed = refreshTargets() || changed
		if changed {
			if err := saveT0DailyCache(date, cache.Stocks, cache.Daily); err != nil {
				return report, fmt.Errorf("保存 %s 红策日线缓存失败: %w", date, err)
			}
		}
	}
	return report, nil
}

func archivedGobRedResults(tradeDate string) (
	[]T0SelectionResult, map[string][]dailyBar, string, bool,
) {
	snapshot, hasSnapshot := loadGobRedArchive(tradeDate)
	if hasSnapshot && snapshot.Complete && snapshot.RuleSetVersion == currentGobRedRuleSetVersion() {
		return snapshot.Results, nil, snapshot.EntrySource, false
	}
	cache, ok := loadT0DailyCache(tradeDate)
	legacyCache := false
	if !ok {
		var err error
		cache, err = readGobRedReplayCache(t0DailyCachePath(tradeDate), tradeDate)
		if err != nil {
			if hasSnapshot {
				return snapshot.Results, nil, "stale_snapshot", true
			}
			return nil, nil, "unavailable", true
		}
		legacyCache = true
	}
	candidates := buildGobRedPreopenCandidates(
		tradeDate, cache.Stocks, cache.Daily, approvedGobRedRules())
	quotes, source, _ := buildGobRedOpeningQuotes(
		tradeDate, resultsToT0Stocks(candidates), cache.Daily)
	results, matchStatus := matchGobRedRules(
		tradeDate, resultsToT0Stocks(candidates), cache.Daily,
		quotes, source, approvedGobRedRules())
	incomplete := legacyCache || buildT0DailyCacheWarning(cache.Stocks, cache.Daily) != nil ||
		matchStatus.MissingOpen > 0 || matchStatus.UnknownLimit > 0
	if incomplete {
		return results, cache.Daily, "incomplete", true
	}
	return results, cache.Daily, "daily_open", false
}

func handleArchivedGobRedSelection(w http.ResponseWriter, tradeDate string) {
	independent, gobDaily, gobState, gobIncomplete := archivedGobRedResults(tradeDate)
	ctx := &t0ModuleSelectionContext{
		TradeDate: tradeDate, Daily: gobDaily, Independent: independent,
	}
	legacyState := "unavailable"
	var legacy []T0SelectionResult
	var savedAt string
	if archive, ok := loadT0SelectionArchive(tradeDate); ok && !t0ArchiveNeedsBackfill(archive) {
		savedAt = archive.SavedAt
		if len(archive.Results) == 0 {
			legacyState = "main_archive"
		} else {
			daily := gobDaily
			if len(missingT0ResultDailyCodes(archive.Results, daily)) > 0 {
				if fetched, ok := loadT0DailyCacheForResults(tradeDate, archive.Results); ok {
					daily = fetched
				}
			}
			if len(missingT0ResultDailyCodes(archive.Results, daily)) == 0 {
				legacy = archive.Results
				legacyState = "main_archive"
				if ctx.Daily == nil {
					ctx.Daily = daily
				} else {
					for code, bars := range daily {
						ctx.Daily[code] = bars
					}
				}
			}
		}
	}
	response := map[string]interface{}{
		"date": tradeDate, "archived": true, "saved_at": savedAt,
		"count": len(legacy), "results": legacy,
		"source_status": map[string]string{"legacy": legacyState, "gob": gobState},
	}
	if gobIncomplete || legacyState == "unavailable" {
		response["data_incomplete"] = true
	}
	writeScopedT0ResponseWithContext(w, redT0StrategyModuleCode, response, ctx, "results")
}
