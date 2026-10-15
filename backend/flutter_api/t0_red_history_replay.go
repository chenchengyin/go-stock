package flutter_api

import (
	"fmt"
	"sort"
	"strings"
	"sync"

	"go-stock/backend/analysis/t0reference"
	"go-stock/backend/db"
	"go-stock/backend/models"
)

const (
	redHistorySourceGob       = "gob"
	redHistorySourceReference = "reference"
	redHistoryMediumBearKey   = "gob_zt_t1_bear_open_close_body_4combos_v1"
)

type redHistoryIdentity struct {
	source  string
	key     string
	version int
}

type redHistoryReplay struct {
	results     map[redHistoryIdentity]map[string]T0SelectionResult
	incomplete  bool
	archiveOnly bool
}

type redHistoryReplayMemo struct {
	sync.Mutex
	key   string
	value *redHistoryReplay
}

var redHistoryReplayCache redHistoryReplayMemo
var redHistoryReplayArchiveOnly = &redHistoryReplay{archiveOnly: true}
var redHistoryReplayWithoutDatabase = &redHistoryReplay{
	results:    map[redHistoryIdentity]map[string]T0SelectionResult{},
	incomplete: true,
}

type t0ReferenceHistoryWatermark struct {
	MaxID uint  `gorm:"column:max_id"`
	Count int64 `gorm:"column:row_count"`
}

type t0ReferenceHistoryJoinRow struct {
	ObservationID uint    `gorm:"column:observation_id"`
	TradeDate     string  `gorm:"column:trade_date"`
	StockCode     string  `gorm:"column:stock_code"`
	T0PrevClose   float64 `gorm:"column:t0_prev_close"`
	T0Open        float64 `gorm:"column:t0_open"`
	T0Close       float64 `gorm:"column:t0_close"`
	T0AmountYi    float64 `gorm:"column:t0_amount_yi"`
	T0OpenGap     float64 `gorm:"column:t0_open_gap"`
	Offset        int     `gorm:"column:bar_offset"`
	BarDate       string  `gorm:"column:bar_date"`
	PrevClose     float64 `gorm:"column:bar_prev_close"`
	Open          float64 `gorm:"column:bar_open"`
	High          float64 `gorm:"column:bar_high"`
	Low           float64 `gorm:"column:bar_low"`
	Close         float64 `gorm:"column:bar_close"`
	Volume        float64 `gorm:"column:bar_volume"`
	AmountYi      float64 `gorm:"column:bar_amount_yi"`
}

type t0ReferenceHistoryObservation struct {
	id          uint
	tradeDate   string
	stockCode   string
	t0PrevClose float64
	t0Open      float64
	t0Close     float64
	t0AmountYi  float64
	t0OpenGap   float64
	barCount    int
	barSeen     [3]bool
	bars        [3]t0reference.BarSnapshot
}

func redHistoryReplayForRuntimes(runtimes []t0ReferenceRuleRuntime) (*redHistoryReplay, error) {
	if db.Dao == nil {
		return redHistoryReplayWithoutDatabase, nil
	}
	var watermark t0ReferenceHistoryWatermark
	if err := db.Dao.Model(&models.T0ReferenceObservation{}).
		Select("COALESCE(MAX(id), 0) AS max_id, COUNT(*) AS row_count").
		Where("is_active = ? AND data_status = ? AND entry_source = ?", true, "complete", "daily_open").
		Scan(&watermark).Error; err != nil {
		return nil, fmt.Errorf("读取红策历史事实版本失败: %w", err)
	}
	if watermark.Count == 0 {
		return redHistoryReplayWithoutDatabase, nil
	}
	key := redHistoryReplayKey(runtimes, watermark)
	redHistoryReplayCache.Lock()
	defer redHistoryReplayCache.Unlock()
	if redHistoryReplayCache.key == key && redHistoryReplayCache.value != nil {
		return redHistoryReplayCache.value, nil
	}
	replay, err := buildRedHistoryReplay(runtimes, watermark)
	if err != nil {
		return nil, err
	}
	redHistoryReplayCache.key = key
	redHistoryReplayCache.value = replay
	return replay, nil
}

func redHistoryReplayKey(runtimes []t0ReferenceRuleRuntime, watermark t0ReferenceHistoryWatermark) string {
	dates := listGobRedHistoryDatesDescending()
	dateVersion := ""
	if len(dates) > 0 {
		dateVersion = strings.Join(dates, ",")
	}
	parts := []string{currentGobRedRuleSetVersion(), fmt.Sprintf("%d/%d", watermark.MaxID, watermark.Count), dateVersion}
	for _, runtime := range runtimes {
		if runtime.rule.RuleKey == "" || !runtime.redEntry {
			continue
		}
		parts = append(parts, fmt.Sprintf("%s/%s/%s/%d/%.4f/%.4f/%.4f/%d/%s",
			runtime.rule.RuleKey, runtime.rule.Definition.Name, runtime.researchTier, runtime.sampleCount,
			runtime.strictWinRate, runtime.targetRate, runtime.earnRate,
			runtime.rule.Definition.ManualRank, runtime.rule.Definition.Description))
	}
	sort.Strings(parts[3:])
	return strings.Join(parts, "|")
}

func buildRedHistoryReplay(
	runtimes []t0ReferenceRuleRuntime,
	watermark t0ReferenceHistoryWatermark,
) (*redHistoryReplay, error) {
	replay := &redHistoryReplay{
		results: make(map[redHistoryIdentity]map[string]T0SelectionResult),
	}
	for _, runtime := range runtimes {
		if runtime.rule.RuleKey == "" || !runtime.redEntry {
			continue
		}
		identity := redHistoryIdentity{source: redHistorySourceReference, key: runtime.rule.RuleKey, version: 1}
		replay.results[identity] = make(map[string]T0SelectionResult)
	}
	mediumBearRule, hasMediumBearRule := gobRedRuleByKey(redHistoryMediumBearKey)
	mediumBearIdentity := redHistoryIdentity{source: redHistorySourceGob, key: redHistoryMediumBearKey, version: 1}
	if hasMediumBearRule {
		replay.results[mediumBearIdentity] = make(map[string]T0SelectionResult)
	}

	rows, err := db.Dao.Table("t0_reference_observations AS o").
		Select(`o.id AS observation_id, o.trade_date AS trade_date, o.stock_code AS stock_code,
			o.t0_prev_close AS t0_prev_close, o.t0_open AS t0_open,
			o.t0_close AS t0_close, o.t0_amount_yi AS t0_amount_yi,
			o.t0_open_gap AS t0_open_gap, b.offset AS bar_offset, b.bar_date AS bar_date,
			b.prev_close AS bar_prev_close, b.open AS bar_open, b.high AS bar_high,
			b.low AS bar_low, b.close AS bar_close, b.volume AS bar_volume,
			b.amount_yi AS bar_amount_yi`).
		Joins("JOIN t0_reference_bars AS b ON b.observation_id = o.id AND b.offset IN (-3, -2, -1)").
		Where("o.is_active = ? AND o.data_status = ? AND o.entry_source = ?", true, "complete", "daily_open").
		Order("o.id ASC, b.offset ASC").Rows()
	if err != nil {
		return nil, fmt.Errorf("读取红策历史日线事实失败: %w", err)
	}
	defer rows.Close()

	compiledRedRules := make([]t0reference.CompiledRule, 0, len(runtimes))
	runtimeByRuleKey := make(map[string]t0ReferenceRuleRuntime, len(runtimes))
	for _, runtime := range runtimes {
		if runtime.rule.RuleKey != "" && runtime.redEntry {
			compiledRedRules = append(compiledRedRules, runtime.rule)
			runtimeByRuleKey[runtime.rule.RuleKey] = runtime
		}
	}
	seenDates := make(map[string]struct{})
	processed := int64(0)
	var current *t0ReferenceHistoryObservation
	process := func(observation *t0ReferenceHistoryObservation) {
		if observation == nil {
			return
		}
		if observation.barCount != 3 {
			replay.incomplete = true
			return
		}
		processed++
		seenDates[observation.tradeDate] = struct{}{}
		hist := referenceObservationDailyBars(observation)
		if len(hist) != 4 || observation.t0PrevClose <= 0 || observation.t0Open <= 0 || observation.t0Close <= 0 ||
			hist[2].Close <= 0 || hist[3].Close <= 0 {
			replay.incomplete = true
			return
		}
		base := T0SelectionResult{
			Time:         observation.tradeDate,
			StockCode:    redHistoryExchangeCode(observation.stockCode),
			StockName:    observation.stockCode,
			OpenGap:      round2(observation.t0OpenGap),
			CloseRet:     round2((observation.t0Close/observation.t0PrevClose - 1) * 100),
			PrevClose:    round2(observation.t0PrevClose),
			PrevCloseRet: round2((hist[3].Close/hist[2].Close - 1) * 100),
			AmountYi:     round2(observation.t0AmountYi),
			BuySignal:    BuySignalRed,
		}

		view, viewOK := preT0ViewFromDaily(hist)
		if !viewOK {
			replay.incomplete = true
			return
		}
		for _, hit := range t0reference.MatchReferenceRules(
			view, t0reference.EntryView{Gap: base.OpenGap}, compiledRedRules,
		) {
			runtime, ok := runtimeByRuleKey[hit.RuleKey]
			if !ok {
				continue
			}
			identity := redHistoryIdentity{source: redHistorySourceReference, key: runtime.rule.RuleKey, version: 1}
			matched := base
			matched.DisplayRuleHits = []string{runtime.rule.Definition.Name}
			matched.T0ReferenceHits = []T0ReferenceHit{{
				RuleKey:       runtime.rule.RuleKey,
				Name:          runtime.rule.Definition.Name,
				Description:   runtime.rule.Definition.Description,
				ResearchTier:  runtime.researchTier,
				StrictWinRate: runtime.strictWinRate,
				TargetRate:    runtime.targetRate,
				EarnRate:      runtime.earnRate,
				SampleCount:   runtime.sampleCount,
				ManualRank:    runtime.rule.Definition.ManualRank,
			}}
			if runtime.rule.Definition.Name == t0reference.RuleNameStrongContinuation {
				matched.StrongContinuationDisplayRuleHit = true
				matched.TechBlueDisplayRuleHit = true
			}
			addRedHistoryReplayResult(replay, identity, matched)
		}

		if hasMediumBearRule && mediumBearRule.MatchHistory(hist[1:]) &&
			mediumBearRule.MatchOpen(observation.t0Open, observation.t0PrevClose) {
			matched := base
			matched.DisplayRuleHits = []string{mediumBearRule.Name}
			matched.RedRuleHits = []T0RedRuleHit{{
				Key:         mediumBearRule.Key,
				Version:     mediumBearRule.Version,
				Name:        mediumBearRule.Name,
				Description: mediumBearRule.Description,
				ExitHorizon: gobRedRuleExitHorizon(mediumBearRule),
				EntrySource: "daily_open",
				EntryOpen:   observation.t0Open,
				StatsStatus: "pending",
			}}
			addRedHistoryReplayResult(replay, mediumBearIdentity, matched)
		}
	}

	for rows.Next() {
		var row t0ReferenceHistoryJoinRow
		if err := rows.Scan(
			&row.ObservationID, &row.TradeDate, &row.StockCode,
			&row.T0PrevClose, &row.T0Open, &row.T0Close, &row.T0AmountYi, &row.T0OpenGap,
			&row.Offset, &row.BarDate, &row.PrevClose, &row.Open,
			&row.High, &row.Low, &row.Close, &row.Volume, &row.AmountYi,
		); err != nil {
			return nil, fmt.Errorf("解析红策历史日线事实失败: %w", err)
		}
		if current == nil || current.id != row.ObservationID {
			process(current)
			current = &t0ReferenceHistoryObservation{
				id: row.ObservationID, tradeDate: row.TradeDate, stockCode: row.StockCode,
				t0PrevClose: row.T0PrevClose, t0Open: row.T0Open,
				t0Close: row.T0Close, t0AmountYi: row.T0AmountYi,
				t0OpenGap: row.T0OpenGap,
			}
		}
		barIndex := row.Offset + 3
		if barIndex < 0 || barIndex >= len(current.bars) {
			replay.incomplete = true
			continue
		}
		if !current.barSeen[barIndex] {
			current.barSeen[barIndex] = true
			current.barCount++
		}
		current.bars[barIndex] = t0reference.BarSnapshot{
			Date: row.BarDate, PrevClose: row.PrevClose, Open: row.Open, High: row.High,
			Low: row.Low, Close: row.Close, Volume: row.Volume, AmountYi: row.AmountYi,
		}
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("遍历红策历史日线事实失败: %w", err)
	}
	process(current)
	if processed != watermark.Count {
		replay.incomplete = true
	}
	for _, date := range listGobRedHistoryDatesDescending() {
		if _, ok := seenDates[date]; !ok {
			replay.incomplete = true
			break
		}
	}
	loadRedHistoryStockNames(replay.results)
	return replay, nil
}

func referenceObservationDailyBars(observation *t0ReferenceHistoryObservation) []dailyBar {
	if observation == nil {
		return nil
	}
	bars := make([]dailyBar, 0, 4)
	first := observation.bars[0]
	// preT0ViewFromDaily needs the close immediately before T-3; the fact row
	// stores it on the T-3 snapshot, so use a synthetic leading bar for that value.
	bars = append(bars, dailyBar{Date: first.Date, Close: first.PrevClose})
	for _, bar := range observation.bars {
		bars = append(bars, dailyBar{
			Date: bar.Date, Open: bar.Open, High: bar.High, Low: bar.Low,
			Close: bar.Close, Volume: bar.Volume, AmountYi: bar.AmountYi,
		})
	}
	return bars
}

func addRedHistoryReplayResult(replay *redHistoryReplay, identity redHistoryIdentity, result T0SelectionResult) {
	byDate := replay.results[identity]
	if byDate == nil {
		byDate = make(map[string]T0SelectionResult)
		replay.results[identity] = byDate
	}
	key := result.Time + ":" + result.StockCode
	byDate[key] = result
}

func gobRedRuleByKey(key string) (redGobRule, bool) {
	for _, rule := range approvedGobRedRules() {
		if rule.Key == key {
			return rule, true
		}
	}
	return redGobRule{}, false
}

func redHistoryExchangeCode(stockCode string) string {
	code := strings.TrimSpace(stockCode)
	if strings.Contains(code, ".") {
		return code
	}
	suffix := ".XSHE"
	if strings.HasPrefix(code, "6") {
		suffix = ".XSHG"
	} else if strings.HasPrefix(code, "4") || strings.HasPrefix(code, "8") {
		suffix = ".XBSE"
	}
	return code + suffix
}

func loadRedHistoryStockNames(results map[redHistoryIdentity]map[string]T0SelectionResult) {
	if db.Dao == nil {
		return
	}
	var rows []struct {
		Symbol string `gorm:"column:symbol"`
		Name   string `gorm:"column:name"`
	}
	if err := db.Dao.Table("tushare_stock_basic").Select("symbol, name").Find(&rows).Error; err != nil {
		return
	}
	names := make(map[string]string, len(rows))
	for _, row := range rows {
		if row.Symbol != "" && row.Name != "" {
			names[row.Symbol] = row.Name
		}
	}
	for _, byDate := range results {
		for key, result := range byDate {
			if name := names[t0ShortCodeFromResultCode(result.StockCode)]; name != "" {
				result.StockName = name
				byDate[key] = result
			}
		}
	}
}
