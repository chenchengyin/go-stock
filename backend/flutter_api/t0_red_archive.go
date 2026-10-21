package flutter_api

import (
	"encoding/json"
	"fmt"
	"math"
	"net/http"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"sync"
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
	addSharedGobRedSelectionDates(seen)
	dates := make([]string, 0, len(seen))
	for date := range seen {
		dates = append(dates, date)
	}
	sort.Sort(sort.Reverse(sort.StringSlice(dates)))
	return dates
}

type gobRedRuleHistoryOption struct {
	RuleSource      string `json:"rule_source"`
	RuleKey         string `json:"rule_key"`
	RuleVersion     int    `json:"rule_version"`
	Name            string `json:"name"`
	Description     string `json:"description,omitempty"`
	MatchCount      int    `json:"match_count"`
	MatchCountKnown bool   `json:"match_count_known"`
}

type gobRedRuleHistoryIndex struct {
	rules         map[redHistoryIdentity]*gobRedRuleHistoryOption
	matchesByRule map[redHistoryIdentity]map[string]T0SelectionResult
	options       []gobRedRuleHistoryOption
	incomplete    bool
}

var gobRedRuleHistoryIndexCache struct {
	sync.Mutex
	key   string
	value *gobRedRuleHistoryIndex
}

func listGobRedHistoryDatesDescending() []string {
	seen := gobRedHistoryCacheDates()
	addSharedGobRedSelectionDates(seen)
	return gobRedHistoryDatesDescending(seen)
}

func listGobRedHistoryCacheDatesDescending() []string {
	return gobRedHistoryDatesDescending(gobRedHistoryCacheDates())
}

func gobRedHistoryCacheDates() map[string]struct{} {
	seen := make(map[string]struct{})
	for _, source := range []struct{ dir, prefix, suffix string }{
		{filepath.Join(t0CacheRootPath, "t0", "daily"), "t0_daily_cache_", ".gob"},
		{filepath.Dir(gobRedArchivePath("")), "t0_red_gob_", ".json"},
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
	return seen
}

func gobRedHistoryDatesDescending(seen map[string]struct{}) []string {
	dates := make([]string, 0, len(seen))
	for date := range seen {
		dates = append(dates, date)
	}
	sort.Sort(sort.Reverse(sort.StringSlice(dates)))
	return dates
}

func addSharedGobRedSelectionDates(seen map[string]struct{}) {
	shared, err := loadMainboardSharedGob()
	if err != nil {
		return
	}
	for date, stocks := range shared.SelectionPools {
		if len(stocks) == 0 || isT0WeekendDate(date) {
			continue
		}
		if normalized, err := normalizeT0TradeDate(date); err == nil && normalized == date {
			seen[date] = struct{}{}
		}
	}
}

func handleGobRedRuleHistory(w http.ResponseWriter, ruleSource, ruleKey, rawRuleVersion string) {
	ruleVersion := 0
	if ruleKey != "" {
		ruleSource = strings.TrimSpace(ruleSource)
		if ruleSource == "" {
			// Keep older clients compatible; they historically queried only GOB rules.
			ruleSource = redHistorySourceGob
		}
		if ruleSource != redHistorySourceGob && ruleSource != redHistorySourceReference {
			WriteJSONStatus(w, http.StatusBadRequest, map[string]interface{}{"error": "rule_source无效"})
			return
		}
		parsed, err := strconv.Atoi(rawRuleVersion)
		if err != nil || parsed < 1 {
			WriteJSONStatus(w, http.StatusBadRequest, map[string]interface{}{"error": "rule_version无效"})
			return
		}
		ruleVersion = parsed
	}

	runtimes := loadT0ReferenceRuleRuntimes()
	replay := redHistoryReplayArchiveOnly
	if ruleKey != "" && (ruleSource == redHistorySourceReference || ruleKey == redHistoryMediumBearKey) {
		var err error
		replay, err = redHistoryReplayForRuntimes(runtimes)
		if err != nil {
			WriteJSONStatus(w, http.StatusServiceUnavailable, map[string]interface{}{"error": err.Error()})
			return
		}
		if ruleSource == redHistorySourceReference && replay == redHistoryReplayWithoutDatabase {
			WriteJSONStatus(w, http.StatusServiceUnavailable, map[string]interface{}{
				"error": "参考形态历史数据不可用，暂时无法回放；这不代表没有历史命中", "data_incomplete": true,
			})
			return
		}
	}
	var index *gobRedRuleHistoryIndex
	if ruleKey == "" {
		index = gobRedRuleHistoryArchiveIndexFor(runtimes)
	} else {
		index = gobRedRuleHistoryIndexFor(runtimes, replay)
	}

	if ruleKey != "" {
		identity := redHistoryIdentity{source: ruleSource, key: ruleKey, version: ruleVersion}
		if _, exists := index.rules[identity]; !exists {
			WriteJSONStatus(w, http.StatusNotFound, map[string]interface{}{"error": "红策规则不存在或已停用"})
			return
		}
		results := redHistoryResults(index.matchesByRule[identity])
		if ruleSource == redHistorySourceGob {
			results = enrichGobRedRuleStats(results)
		}
		var daily map[string][]dailyBar
		if shared, err := loadMainboardSharedGob(); err == nil {
			daily = shared.Daily
		}
		pricedResults := redHistoryResultsWithCachedPrices(results, daily, loadRedKlineDaily)
		WriteJSON(w, map[string]interface{}{
			"rule_source": ruleSource, "rule_key": ruleKey, "rule_version": ruleVersion,
			"count": len(results), "results": pricedResults, "data_incomplete": index.incomplete,
		})
		return
	}

	WriteJSON(w, map[string]interface{}{"rules": index.options, "data_incomplete": index.incomplete})
}

type redHistoryPriceResult struct {
	T0SelectionResult
	T0IntradayChangePP *float64 `json:"t0_intraday_change_pp"`
}

func redHistoryResultsWithCachedPrices(results []T0SelectionResult, daily map[string][]dailyBar, loadDaily func(string) map[string][]dailyBar) []redHistoryPriceResult {
	out := redHistoryResultsWithPrices(results, daily)
	missingByDate := make(map[string][]int)
	for i, item := range out {
		if item.T0IntradayChangePP == nil {
			missingByDate[item.Time] = append(missingByDate[item.Time], i)
		}
	}
	// Match the chart's local-cache fallback, loading each missing date only once.
	for date, indices := range missingByDate {
		archivedDaily := loadDaily(date)
		for _, i := range indices {
			out[i] = redHistoryResultsWithPrices(results[i:i+1], archivedDaily)[0]
		}
	}
	return out
}

// Project prices from one daily series: archived GOB results can contain a zero
// close-return placeholder. Never interpret that placeholder as an actual close.
func redHistoryResultsWithPrices(results []T0SelectionResult, daily map[string][]dailyBar) []redHistoryPriceResult {
	out := make([]redHistoryPriceResult, 0, len(results))
	for _, result := range results {
		item := redHistoryPriceResult{T0SelectionResult: result}
		bars := daily[t0ShortCodeFromResultCode(result.StockCode)]
		bar, index := dailyBarOnDate(bars, result.Time)
		if index > 0 && bar.Open > 0 && bar.Close > 0 && bars[index-1].Close > 0 &&
			!math.IsInf(bar.Open, 0) && !math.IsInf(bar.Close, 0) && !math.IsInf(bars[index-1].Close, 0) {
			previousClose := bars[index-1].Close
			item.OpenGap = round2((bar.Open/previousClose - 1) * 100)
			item.CloseRet = round2((bar.Close/previousClose - 1) * 100)
			change := round2(item.CloseRet - item.OpenGap)
			item.T0IntradayChangePP = &change
		}
		out = append(out, item)
	}
	return out
}

func gobRedRuleHistoryIndexFor(
	runtimes []t0ReferenceRuleRuntime,
	replay *redHistoryReplay,
) *gobRedRuleHistoryIndex {
	return gobRedRuleHistoryIndexForDates(listGobRedHistoryDatesDescending(), replay, runtimes)
}

func gobRedRuleHistoryArchiveIndexFor(runtimes []t0ReferenceRuleRuntime) *gobRedRuleHistoryIndex {
	return gobRedRuleHistoryIndexForDates(
		listGobRedHistoryCacheDatesDescending(), redHistoryReplayArchiveOnly, runtimes,
	)
}

func gobRedRuleHistoryIndexForDates(
	dates []string,
	replay *redHistoryReplay,
	runtimes []t0ReferenceRuleRuntime,
) *gobRedRuleHistoryIndex {
	key := gobRedRuleHistoryIndexKey(dates, replay, runtimes)

	gobRedRuleHistoryIndexCache.Lock()
	defer gobRedRuleHistoryIndexCache.Unlock()
	if gobRedRuleHistoryIndexCache.key == key && gobRedRuleHistoryIndexCache.value != nil {
		return gobRedRuleHistoryIndexCache.value
	}

	index := buildGobRedRuleHistoryIndex(dates, replay, runtimes)
	gobRedRuleHistoryIndexCache.key = key
	gobRedRuleHistoryIndexCache.value = index
	return index
}

func gobRedRuleHistoryIndexKey(
	dates []string,
	replay *redHistoryReplay,
	runtimes []t0ReferenceRuleRuntime,
) string {
	parts := []string{currentGobRedRuleSetVersion(), fmt.Sprintf("replay:%p", replay)}
	for _, rule := range approvedGobRedRules() {
		parts = append(parts, fmt.Sprintf("rule:%s@%d:%s:%s", rule.Key, rule.Version, rule.Name, rule.Description))
	}
	for _, runtime := range runtimes {
		if runtime.rule.RuleKey == "" || !runtime.redEntry {
			continue
		}
		parts = append(parts, fmt.Sprintf("reference:%s/%s/%s/%d/%.4f/%.4f/%.4f/%d/%s",
			runtime.rule.RuleKey, runtime.rule.Definition.Name, runtime.researchTier, runtime.sampleCount,
			runtime.strictWinRate, runtime.targetRate, runtime.earnRate,
			runtime.rule.Definition.ManualRank, runtime.rule.Definition.Description))
	}
	for _, date := range dates {
		parts = append(parts, date)
		for _, path := range []string{
			gobRedArchivePath(date),
			filepath.Join(t0CacheRootPath, "t0", "daily", "t0_daily_cache_"+date+".gob"),
		} {
			info, err := os.Stat(path)
			if err != nil {
				parts = append(parts, path+":missing")
				continue
			}
			parts = append(parts, fmt.Sprintf("%s:%d:%d", path, info.Size(), info.ModTime().UnixNano()))
		}
	}
	if info, err := os.Stat(mainboardSharedGobPath()); err == nil {
		parts = append(parts, fmt.Sprintf("shared:%d:%d", info.Size(), info.ModTime().UnixNano()))
	}
	return strings.Join(parts, "|")
}

func buildGobRedRuleHistoryIndex(
	dates []string,
	replay *redHistoryReplay,
	runtimes []t0ReferenceRuleRuntime,
) *gobRedRuleHistoryIndex {
	rules := make(map[redHistoryIdentity]*gobRedRuleHistoryOption)
	matchesByRule := make(map[redHistoryIdentity]map[string]T0SelectionResult)
	upgradedOpeningMatches := make(map[string]T0SelectionResult)
	openingRule, hasOpeningRule := gobRedRuleByKey("gob_zt_zt_pb_t1_amplitude_6_10_shadow_le_1_v3")
	for _, rule := range approvedGobRedRules() {
		identity := redHistoryIdentity{source: redHistorySourceGob, key: rule.Key, version: rule.Version}
		rules[identity] = &gobRedRuleHistoryOption{
			RuleSource: redHistorySourceGob, RuleKey: rule.Key, RuleVersion: rule.Version,
			Name: rule.Name, Description: rule.Description,
		}
		matchesByRule[identity] = make(map[string]T0SelectionResult)
	}
	for _, runtime := range runtimes {
		if runtime.rule.RuleKey == "" || !runtime.redEntry {
			continue
		}
		identity := redHistoryIdentity{source: redHistorySourceReference, key: runtime.rule.RuleKey, version: 1}
		rules[identity] = &gobRedRuleHistoryOption{
			RuleSource: redHistorySourceReference, RuleKey: runtime.rule.RuleKey,
			RuleVersion: 1, Name: runtime.rule.Definition.Name,
			Description: runtime.rule.Definition.Description,
		}
		matchesByRule[identity] = make(map[string]T0SelectionResult)
	}
	incomplete := replay.incomplete
	if len(dates) == 0 {
		incomplete = true
	}
	for _, date := range dates {
		archive, ok := loadGobRedArchive(date)
		dateResults := []T0SelectionResult(nil)
		dateIncomplete := true
		if ok {
			// 历史归档即使因日线覆盖或开盘价来源被标为不完整，也已经包含
			// 当时可确认的红策命中。直接使用快照并标记覆盖不完整，避免每次
			// 打开条件列表都重新解码该日期的大型 GOB 日线缓存。
			dateResults = archive.Results
			dateIncomplete = !archive.Complete || archive.RuleSetVersion != currentGobRedRuleSetVersion()
		} else if replay.archiveOnly {
			incomplete = true
			continue
		} else {
			var sourceStatus string
			dateResults, _, sourceStatus, dateIncomplete = archivedGobRedResults(date)
			if sourceStatus == "unavailable" {
				incomplete = true
				continue
			}
		}
		if dateIncomplete {
			incomplete = true
		}
		for _, result := range dateResults {
			for _, hit := range result.RedRuleHits {
				identity := redHistoryIdentity{source: redHistorySourceGob, key: hit.Key, version: hit.Version}
				if hit.Key == "" || hit.Version < 1 {
					continue
				}
				if _, exists := rules[identity]; !exists {
					// A retired rule stays queryable under its original identity.
					// Its matches must never be silently assigned to the current version.
					description := hit.Description
					if hit.Key == "gob_zt_zt_pb_t1_amplitude_6_10_shadow_le_1_v2" && hit.Version == 2 {
						description = legacyGobZTztPbV2Description
					}
					rules[identity] = &gobRedRuleHistoryOption{
						RuleSource: redHistorySourceGob, RuleKey: hit.Key, RuleVersion: hit.Version,
						Name: fmt.Sprintf("%s（历史 v%d）", hit.Name, hit.Version), Description: description,
					}
					matchesByRule[identity] = make(map[string]T0SelectionResult)
				}
				matched := result
				matched.RedRuleHits = []T0RedRuleHit{hit}
				matched.DisplayRuleHits = []string{hit.Name}
				key := matched.Time + ":" + matched.StockCode
				if _, exists := matchesByRule[identity][key]; !exists {
					matchesByRule[identity][key] = matched
				}
				if hasOpeningRule {
					if upgraded, ok := gobRedHistoryV3OpeningHit(openingRule, result, hit); ok {
						matched.RedRuleHits = []T0RedRuleHit{upgraded}
						matched.DisplayRuleHits = []string{upgraded.Name}
						upgradedOpeningMatches[key] = matched
					}
				}
			}
		}
	}
	if hasOpeningRule {
		identity := redHistoryIdentity{source: redHistorySourceGob, key: openingRule.Key, version: openingRule.Version}
		for key, result := range upgradedOpeningMatches {
			if _, exists := matchesByRule[identity][key]; !exists {
				matchesByRule[identity][key] = result
			}
		}
	}
	for identity, byDate := range replay.results {
		if _, isCurrentRule := rules[identity]; !isCurrentRule {
			continue
		}
		for key, result := range byDate {
			if _, exists := matchesByRule[identity][key]; !exists {
				matchesByRule[identity][key] = result
			}
		}
	}

	options := make([]gobRedRuleHistoryOption, 0, len(rules))
	for identity, option := range rules {
		option.MatchCount = len(matchesByRule[identity])
		option.MatchCountKnown = !replay.archiveOnly ||
			(identity.source == redHistorySourceGob && identity.key != redHistoryMediumBearKey)
		options = append(options, *option)
	}
	sort.Slice(options, func(i, j int) bool {
		if options[i].MatchCountKnown != options[j].MatchCountKnown {
			return options[i].MatchCountKnown
		}
		if options[i].MatchCount != options[j].MatchCount {
			return options[i].MatchCount > options[j].MatchCount
		}
		if options[i].Name != options[j].Name {
			return options[i].Name < options[j].Name
		}
		if options[i].RuleVersion != options[j].RuleVersion {
			return options[i].RuleVersion < options[j].RuleVersion
		}
		if options[i].RuleSource != options[j].RuleSource {
			return options[i].RuleSource < options[j].RuleSource
		}
		return options[i].RuleKey < options[j].RuleKey
	})
	return &gobRedRuleHistoryIndex{
		rules: rules, matchesByRule: matchesByRule, options: options, incomplete: incomplete,
	}
}

// V3 retains V2's historical pattern and narrows only the opening range.
// Recheck stored prices rather than copying the old count or rounded gap.
// This is a read-time projection; the original V2 snapshot remains intact.
func gobRedHistoryV3OpeningHit(rule redGobRule, result T0SelectionResult, hit T0RedRuleHit) (T0RedRuleHit, bool) {
	if rule.Key != "gob_zt_zt_pb_t1_amplitude_6_10_shadow_le_1_v3" || rule.Version != 3 ||
		hit.Key != "gob_zt_zt_pb_t1_amplitude_6_10_shadow_le_1_v2" || hit.Version != 2 ||
		hit.EntryOpen <= 0 || result.PrevClose <= 0 || !rule.MatchOpen(hit.EntryOpen, result.PrevClose) {
		return T0RedRuleHit{}, false
	}
	hit.Key, hit.Version = rule.Key, rule.Version
	hit.Name, hit.Description = rule.Name, rule.Description
	hit.Stats, hit.StatsStatus = nil, "pending"
	return hit, true
}

func redHistoryResults(byDate map[string]T0SelectionResult) []T0SelectionResult {
	results := make([]T0SelectionResult, 0, len(byDate))
	for _, result := range byDate {
		results = append(results, result)
	}
	sort.Slice(results, func(i, j int) bool {
		if results[i].Time != results[j].Time {
			return results[i].Time > results[j].Time
		}
		return results[i].StockCode < results[j].StockCode
	})
	return results
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
					if hit.ExitHorizon == "T+1_close" || hit.ExitHorizon == "T+1_target_2_5" || hit.ExitHorizon == gobRedBodyHorizon {
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
