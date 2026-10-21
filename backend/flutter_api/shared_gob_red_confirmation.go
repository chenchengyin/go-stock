package flutter_api

import (
	"encoding/json"
	"fmt"
	"go-stock/backend/analysis/t0reference"
	"go-stock/backend/data"
	"math"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

var sharedGobRedConfirmMu sync.Mutex
var sharedGobPriorDateFetchMu sync.Mutex
var sharedGobPriorDateCache = struct {
	sync.Mutex
	dates    map[string]string
	excluded map[string]map[string]string
}{dates: make(map[string]string), excluded: make(map[string]map[string]string)}

// A fresh benchmark is the trading-date witness. Cached pool bars cannot
// establish their own freshness: the entire GOB may have missed a session.
var sharedGobPriorDateFetcher = func(tradeDate string) (string, error) {
	day, err := time.Parse("2006-01-02", tradeDate)
	if err != nil {
		return "", err
	}
	end := day.AddDate(0, 0, -1).Format("2006-01-02")
	latest := ""
	for _, code := range []string{"600000", "000001", "600519"} {
		bars := histBarsBeforeTradeDate(fetchDailyKLineWithLimit(code, end, 20), tradeDate)
		if len(bars) >= 2 && bars[len(bars)-1].Date > latest {
			latest = bars[len(bars)-1].Date
		}
	}
	if latest != "" {
		return latest, nil
	}
	return "", fmt.Errorf("无法确认%s的前一交易日，等待日线数据恢复", tradeDate)
}

func sharedGobRequiredPriorDate(tradeDate string) (string, error) {
	sharedGobPriorDateCache.Lock()
	date := sharedGobPriorDateCache.dates[tradeDate]
	sharedGobPriorDateCache.Unlock()
	if date != "" {
		return date, nil
	}
	sharedGobPriorDateFetchMu.Lock()
	defer sharedGobPriorDateFetchMu.Unlock()
	sharedGobPriorDateCache.Lock()
	date = sharedGobPriorDateCache.dates[tradeDate]
	sharedGobPriorDateCache.Unlock()
	if date != "" {
		return date, nil
	}
	date, err := sharedGobPriorDateFetcher(tradeDate)
	if err != nil {
		return "", err
	}
	if date == "" || date >= tradeDate {
		return "", fmt.Errorf("前一交易日无效: %s", date)
	}
	sharedGobPriorDateCache.Lock()
	sharedGobPriorDateCache.dates[tradeDate] = date
	sharedGobPriorDateCache.Unlock()
	return date, nil
}

func sharedGobHistoryCurrent(bars []dailyBar, tradeDate, priorDate string) bool {
	hist := histBarsBeforeTradeDate(bars, tradeDate)
	return len(hist) > 0 && hist[len(hist)-1].Date == priorDate
}

func sharedGobStockHistoryCurrent(shared *mainboardSharedGobData, code string, bars []dailyBar, tradeDate, priorDate string) bool {
	return sharedGobHistoryCurrent(bars, tradeDate, priorDate) && shared.Snapshot.ClosedDailyDates[code] >= priorDate
}

func copySharedGobClosedDates(dates map[string]string) map[string]string {
	copied := make(map[string]string, len(dates))
	for code, date := range dates {
		copied[code] = date
	}
	return copied
}

func isConfirmedSharedGobSuspension(quote t0Realtime, tradeDate string, now time.Time) bool {
	return (quote.NonTrading || !isBeforeT0AuctionCutoff(now, tradeDate)) &&
		quote.TradeDate == tradeDate && quote.PrevClose > 0 &&
		quote.Open == 0 && quote.Volume == 0
}

// Overnight, the feed may still contain the previous session's closing quote.
// It can prove that session had no trades, but never supplies today's auction.
func isConfirmedSharedGobPriorSessionSuspension(quote t0Realtime, bars []dailyBar, tradeDate, priorDate string) bool {
	if quote.Source != "tencent" || quote.TradeDate != priorDate || quote.PrevClose <= 0 ||
		quote.Open != 0 || quote.Volume != 0 || priorDate >= tradeDate {
		return false
	}
	stamp, err := time.ParseInLocation("20060102150405", quote.QuoteTime, chinaLocation())
	if err != nil || stamp.Format("2006-01-02") != priorDate ||
		(!quote.NonTrading && stamp.Hour()*60+stamp.Minute() < 15*60) {
		return false
	}
	hist := histBarsBeforeTradeDate(bars, tradeDate)
	return len(hist) > 0 && hist[len(hist)-1].Date < priorDate &&
		math.Abs(hist[len(hist)-1].Close-quote.PrevClose) < 0.005
}

// A missing bar is only a valid exclusion when today's quotes independently
// prove suspension, or a fresh THS tail and Tencent agree the previous traded
// session was earlier. Ordinary download failures remain incomplete.
var sharedGobNonTradingHistoryVerifier = func(stocks []t0Stock, daily map[string][]dailyBar, tradeDate, priorDate string) map[string]string {
	excluded := map[string]string{}
	now := time.Now().In(chinaLocation())
	quotes := fetchT0RealtimeTencent(stocks)
	for _, stock := range stocks {
		rt, ok := quotes[stock.ShortCode]
		if !ok || rt.TradeDate != tradeDate || rt.PrevClose <= 0 {
			continue
		}
		if rt.Open == 0 && rt.Volume == 0 {
			if isConfirmedSharedGobSuspension(rt, tradeDate, now) {
				excluded[stock.ShortCode] = "当日停牌，无竞价开盘"
			}
			continue
		}
		quote, ok := data.FetchTHSRealtimeDailyQuote(stock.ShortCode, now)
		hist := histBarsBeforeTradeDate(daily[stock.ShortCode], tradeDate)
		if ok && quote.PrevDate < priorDate && len(hist) > 0 && hist[len(hist)-1].Date == quote.PrevDate &&
			math.Abs(quote.PrevClose-rt.PrevClose) < 0.005 && math.Abs(hist[len(hist)-1].Close-rt.PrevClose) < 0.005 {
			excluded[stock.ShortCode] = "前一交易日无成交日线，连续形态不适用"
		}
	}
	if len(excluded) < len(stocks) && isBeforeT0AuctionCutoff(now, tradeDate) {
		prior, err := time.ParseInLocation("2006-01-02", priorDate, chinaLocation())
		if err == nil {
			pending := make([]t0Stock, 0, len(stocks)-len(excluded))
			for _, stock := range stocks {
				if excluded[stock.ShortCode] == "" {
					pending = append(pending, stock)
				}
			}
			priorQuotes := fetchT0RealtimeTencentAt(pending, prior.Add(24*time.Hour-time.Second))
			for _, stock := range pending {
				if isConfirmedSharedGobPriorSessionSuspension(priorQuotes[stock.ShortCode], daily[stock.ShortCode], tradeDate, priorDate) {
					excluded[stock.ShortCode] = "前一交易日无成交日线，连续形态不适用"
				}
			}
		}
	}
	return excluded
}

func sharedGobRedHistoryReady(tradeDate string) bool {
	sharedGobPriorDateCache.Lock()
	prior := sharedGobPriorDateCache.dates[tradeDate]
	excluded := sharedGobPriorDateCache.excluded[tradeDate]
	sharedGobPriorDateCache.Unlock()
	if prior == "" {
		return false
	}
	shared, err := loadMainboardSharedGob()
	if err != nil {
		return false
	}
	stocks := sharedGobT0SelectionPool(shared, tradeDate)
	if len(stocks) == 0 {
		return false
	}
	for _, stock := range stocks {
		if excluded[stock.ShortCode] != "" || sharedGobVerifiedNoTradingDate(shared, stock.ShortCode, prior) {
			continue
		}
		if !sharedGobStockHistoryCurrent(shared, stock.ShortCode, shared.Daily[stock.ShortCode], tradeDate, prior) {
			return false
		}
	}
	return true
}

func sharedGobRedExcludedStocks(tradeDate string) map[string]string {
	sharedGobPriorDateCache.Lock()
	defer sharedGobPriorDateCache.Unlock()
	return sharedGobPriorDateCache.excluded[tradeDate]
}

type sharedGobRedConfirmation struct {
	SchemaVersion  int                   `json:"schema_version"`
	Date           string                `json:"date"`
	ConfirmedAt    string                `json:"confirmed_at"`
	HistoryDate    string                `json:"history_date"`
	GobGeneratedAt string                `json:"gob_generated_at"`
	RuleSetVersion string                `json:"rule_set_version"`
	Quotes         map[string]t0Realtime `json:"auction_quotes"`
	ExcludedStocks map[string]string     `json:"excluded_stocks,omitempty"`
	Response       json.RawMessage       `json:"response"`
}

func sharedGobRedConfirmationPath(date string) string {
	return filepath.Join(t0CacheRootPath, "t0", "selection", "t0_red_confirmed_"+date+".json")
}

func loadSharedGobRedConfirmation(date string) (map[string]interface{}, bool, error) {
	parsed, err := time.Parse("2006-01-02", date)
	if err != nil || parsed.Format("2006-01-02") != date {
		return nil, false, fmt.Errorf("无效确认日期: %s", date)
	}
	data, err := os.ReadFile(sharedGobRedConfirmationPath(date))
	if os.IsNotExist(err) {
		return nil, false, nil
	}
	if err != nil {
		return nil, false, err
	}
	var snapshot sharedGobRedConfirmation
	if err := json.Unmarshal(data, &snapshot); err != nil {
		return nil, false, err
	}
	if snapshot.SchemaVersion != 1 || snapshot.Date != date || snapshot.ConfirmedAt == "" || snapshot.HistoryDate == "" {
		return nil, false, fmt.Errorf("红策当日确认文件无效: %s", date)
	}
	var response map[string]interface{}
	if err := json.Unmarshal(snapshot.Response, &response); err != nil {
		return nil, false, err
	}
	if response["date"] != date || response["confirmed"] != true || response["results"] == nil {
		return nil, false, fmt.Errorf("红策确认结果无效: %s", date)
	}
	refreshConfirmedRedRuleDescriptions(response)
	return response, true, nil
}

// Refresh display metadata only; never rewrite confirmed prices, admissions,
// statistics, rule identities, or the snapshot on disk.
func refreshConfirmedRedRuleDescriptions(response map[string]interface{}) {
	gobRules := make(map[gobRedRuleStatScope]redGobRule)
	for _, rule := range approvedGobRedRules() {
		gobRules[gobRedRuleStatScope{ruleKey: rule.Key, ruleVersion: rule.Version}] = rule
	}
	referenceRules := make(map[string]t0reference.RuleDefinition)
	for _, definition := range t0reference.DefaultRuleDefinitions() {
		if !definition.RedEntry {
			continue
		}
		if compiled, err := t0reference.CompileRule(definition); err == nil {
			referenceRules[compiled.RuleKey] = definition
		}
	}
	results, _ := response["results"].([]interface{})
	for _, rawResult := range results {
		result, ok := rawResult.(map[string]interface{})
		if !ok {
			continue
		}
		hits, _ := result["红策独立形态命中"].([]interface{})
		for _, rawHit := range hits {
			hit, ok := rawHit.(map[string]interface{})
			if !ok {
				continue
			}
			key, _ := hit["rule_key"].(string)
			version, _ := hit["rule_version"].(float64)
			if rule, ok := gobRules[gobRedRuleStatScope{ruleKey: key, ruleVersion: int(version)}]; ok {
				hit["name"], hit["description"] = rule.Name, rule.Description
			}
		}
		referenceHits, _ := result["T0参考形态命中"].([]interface{})
		for _, rawHit := range referenceHits {
			hit, ok := rawHit.(map[string]interface{})
			if !ok {
				continue
			}
			key, _ := hit["rule_key"].(string)
			if definition, ok := referenceRules[key]; ok {
				hit["description"] = definition.Description
			}
		}
	}
}

// Hard-link an fsynced temporary file so even two processes cannot replace
// the first confirmed result. Empty, complete selections are snapshots too.
func saveSharedGobRedConfirmation(snapshot sharedGobRedConfirmation) (map[string]interface{}, error) {
	if err := ensureT0CacheDirs(); err != nil {
		return nil, err
	}
	data, err := json.MarshalIndent(snapshot, "", "  ")
	if err != nil {
		return nil, err
	}
	path := sharedGobRedConfirmationPath(snapshot.Date)
	f, err := os.CreateTemp(filepath.Dir(path), "t0_red_confirmed_*.tmp")
	if err != nil {
		return nil, err
	}
	defer os.Remove(f.Name())
	if _, err = f.Write(data); err == nil {
		err = f.Sync()
	}
	closeErr := f.Close()
	if err != nil {
		return nil, err
	}
	if closeErr != nil {
		return nil, closeErr
	}
	if err := os.Link(f.Name(), path); err != nil && !os.IsExist(err) {
		return nil, err
	}
	response, ok, err := loadSharedGobRedConfirmation(snapshot.Date)
	if err != nil {
		return nil, err
	}
	if !ok {
		return nil, fmt.Errorf("红策确认文件未保存")
	}
	return response, nil
}

func handleSharedGobRedSelection(w http.ResponseWriter, requestedDate string) {
	handleSharedGobRedSelectionAt(w, requestedDate, time.Now())
}

func handleSharedGobRedSelectionAt(w http.ResponseWriter, requestedDate string, now time.Time) {
	now = now.In(chinaLocation())
	today := now.Format("2006-01-02")
	date := strings.TrimSpace(requestedDate)
	if date == "" {
		date = today
	}
	if isT0WeekendDate(date) {
		WriteJSON(w, map[string]interface{}{"date": date, "no_data": true, "count": 0, "results": []T0SelectionResult{}, "source": "shared_gob"})
		return
	}
	if date == today && isBeforeT0AuctionCutoff(now, today) {
		WriteJSON(w, map[string]interface{}{"date": today, "status": "awaiting_auction", "count": 0, "results": []T0SelectionResult{}, "source": "shared_gob"})
		return
	}
	if date == today {
		sharedGobRedConfirmMu.Lock()
		defer sharedGobRedConfirmMu.Unlock()
	}
	if date != "" && date <= today && !isBeforeT0AuctionCutoff(now, date) {
		response, found, err := loadSharedGobRedConfirmation(date)
		if err != nil {
			WriteJSONStatus(w, http.StatusServiceUnavailable, map[string]interface{}{"date": date, "error": err.Error()})
			return
		}
		if found {
			if date < today {
				response["archived"] = true
			}
			WriteJSON(w, response)
			return
		}
	}
	if date == today && !isBeforeT0AuctionCutoff(now, today) && !sharedGobRedHistoryReady(today) {
		t0PrewarmStarter(today)
		WriteJSON(w, buildPrewarmProgressResponse(today, getT0WarmProgress(today)))
		return
	}
	handleSharedGobRedSelectionUnconfirmed(w, date)
}

type sharedGobRedTickWriter struct{}

func (sharedGobRedTickWriter) Header() http.Header         { return make(http.Header) }
func (sharedGobRedTickWriter) Write(b []byte) (int, error) { return len(b), nil }
func (sharedGobRedTickWriter) WriteHeader(int)             {}

func runSharedGobRedConfirmationTick(now time.Time) {
	local := now.In(chinaLocation())
	date := local.Format("2006-01-02")
	if isT0WeekendDate(date) || isBeforeT0AuctionCutoff(local, date) || local.Hour()*60+local.Minute() >= 15*60+5 {
		return
	}
	handleSharedGobRedSelectionAt(sharedGobRedTickWriter{}, date, local)
}
