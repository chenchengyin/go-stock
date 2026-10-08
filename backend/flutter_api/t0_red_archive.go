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
			if normalized, err := normalizeT0TradeDate(date); err == nil && normalized == date {
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
	if normalized, err := normalizeT0TradeDate(tradeDate); err != nil || normalized != tradeDate {
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
	if normalized, err := normalizeT0TradeDate(archive.Date); err != nil || normalized != archive.Date {
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

func archivedGobRedResults(tradeDate string) (
	[]T0SelectionResult, map[string][]dailyBar, string, bool,
) {
	if snapshot, ok := loadGobRedArchive(tradeDate); ok {
		state := snapshot.EntrySource
		if !snapshot.Complete {
			state = "incomplete"
		}
		return snapshot.Results, nil, state, !snapshot.Complete
	}
	cache, ok := loadT0DailyCache(tradeDate)
	legacyCache := false
	if !ok {
		var err error
		cache, err = readGobRedReplayCache(t0DailyCachePath(tradeDate), tradeDate)
		if err != nil {
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
