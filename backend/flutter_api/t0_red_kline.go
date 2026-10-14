package flutter_api

import (
	"context"
	"encoding/json"
	"fmt"
	"github.com/go-resty/resty/v2"
	"go-stock/backend/data"
	"math"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"
)

const redKlineBefore = 60
const redKlineAfter = 5
const redKlineMaxCodes = 25
const redKlineFetchLimit = 800

var redKlineCodePattern = regexp.MustCompile(`^[0-9]{6}(\.(XSHG|XSHE|XBSE))?$`)

// Chart prices from different snapshots/providers may use different adjustment
// baselines. A window is always taken from one coherent series, never spliced.
type redKlineBar struct {
	Date  string             `json:"date"`
	Open  float64            `json:"open"`
	High  float64            `json:"high"`
	Low   float64            `json:"low"`
	Close float64            `json:"close"`
	MA    map[string]float64 `json:"ma,omitempty"`
}
type redKlineWindow struct {
	Code        string        `json:"code"`
	Date        string        `json:"date"`
	Bars        []redKlineBar `json:"bars"`
	AnchorIndex int           `json:"anchor_index"`
	BeforeCount int           `json:"before_count"`
	AfterCount  int           `json:"after_count"`
	Source      string        `json:"source"`
	Status      string        `json:"status"`
	Partial     bool          `json:"partial"`
}

type redKlineFetch func(code string, limit int, end string) *data.KLineSourceResult

var fetchRedKlineSeries redKlineFetch = func(code string, limit int, end string) *data.KLineSourceResult {
	if strings.TrimSpace(end) == "" {
		if bars := fetchRedKlineTencent(code, limit); len(bars) > 0 {
			return &data.KLineSourceResult{Data: &bars, Source: "tencent"}
		}
	}
	return data.FetchKLineWithFallback(code, "", "101", limit, end)
}

// Tencent may return metadata objects among candle rows, and newly listed
// stocks may have only unadjusted `day` rows. Decode the requested symbol and
// skip non-candle entries instead of rejecting the whole response. A failed
// request still falls back to the existing provider chain above.
func fetchRedKlineTencent(code string, limit int) []data.KLineData {
	if limit < 1 || limit > redKlineFetchLimit {
		limit = redKlineFetchLimit
	}
	if len(code) != 6 {
		return nil
	}
	symbol := ""
	switch code[0] {
	case '6':
		symbol = "sh" + code
	case '0', '3':
		symbol = "sz" + code
	case '8', '9':
		symbol = "bj" + code
	default:
		return nil
	}
	params := url.Values{"param": {fmt.Sprintf("%s,day,,,%d,qfq", symbol, limit)}}
	url := "https://web.ifzq.gtimg.cn/appstock/app/fqkline/get?" + params.Encode()
	ctx, cancel := context.WithTimeout(context.Background(), 8*time.Second)
	defer cancel()
	resp, err := resty.New().SetTimeout(8*time.Second).R().
		SetContext(ctx).
		SetHeader("User-Agent", "Mozilla/5.0").
		SetHeader("Referer", "https://gu.qq.com/").
		Get(url)
	if err != nil {
		return nil
	}
	if resp.StatusCode() != http.StatusOK {
		return nil
	}
	body := resp.Body()
	if len(body) > 2<<20 {
		return nil
	}
	return parseRedKlineTencentPayload(body, symbol)
}

func parseRedKlineTencentPayload(body []byte, symbol string) []data.KLineData {
	var result struct {
		Code int                        `json:"code"`
		Data map[string]json.RawMessage `json:"data"`
	}
	if json.Unmarshal(body, &result) != nil || result.Code != 0 {
		return nil
	}
	var stock struct {
		QFQDay []json.RawMessage `json:"qfqday"`
		Day    []json.RawMessage `json:"day"`
	}
	if json.Unmarshal(result.Data[symbol], &stock) != nil {
		return nil
	}
	rows := stock.QFQDay
	if len(rows) == 0 {
		rows = stock.Day
	}
	bars := make([]data.KLineData, 0, len(rows))
	for _, raw := range rows {
		var row []string
		if json.Unmarshal(raw, &row) != nil {
			continue
		}
		if len(row) < 5 {
			continue
		}
		bars = append(bars, data.KLineData{
			Day: row[0], Open: row[1], Close: row[2], High: row[3], Low: row[4],
		})
	}
	return bars
}

func validRedKlineBar(b dailyBar) bool {
	return len(b.Date) == 10 && b.Date[4] == '-' && b.Date[7] == '-' &&
		b.Open > 0 && b.Close > 0 && b.High > 0 && b.Low > 0 &&
		!math.IsNaN(b.Open) && !math.IsNaN(b.Close) && !math.IsNaN(b.High) && !math.IsNaN(b.Low) &&
		!math.IsInf(b.Open, 0) && !math.IsInf(b.Close, 0) && !math.IsInf(b.High, 0) && !math.IsInf(b.Low, 0) &&
		b.High >= math.Max(b.Open, b.Close) && b.Low <= math.Min(b.Open, b.Close) && b.High >= b.Low
}

func buildRedKlineWindow(code, date string, bars []dailyBar, source string) redKlineWindow {
	window := redKlineWindow{Code: code, Date: date, Bars: []redKlineBar{}, AnchorIndex: -1, Source: source, Status: "unavailable", Partial: true}
	byDate := make(map[string]dailyBar, len(bars))
	for _, bar := range bars {
		if validRedKlineBar(bar) && bar.Date <= time.Now().In(chinaLocation()).Format("2006-01-02") {
			byDate[bar.Date] = bar
		}
	}
	dates := make([]string, 0, len(byDate))
	for d := range byDate {
		dates = append(dates, d)
	}
	sort.Strings(dates)
	i := sort.SearchStrings(dates, date)
	start := i - redKlineBefore
	if start < 0 {
		start = 0
	}
	end := i
	if i < len(dates) && dates[i] == date {
		end = i + 1 + redKlineAfter
	} else {
		window.Status = "pending"
	}
	if end > len(dates) {
		end = len(dates)
	}
	if end < start {
		end = start
	}
	for index := start; index < end; index++ {
		d := dates[index]
		b := byDate[d]
		if d == date {
			window.AnchorIndex = len(window.Bars)
		}
		out := redKlineBar{Date: d, Open: b.Open, High: b.High, Low: b.Low, Close: b.Close}
		for _, period := range []int{5, 10, 20, 60} {
			if index+1 < period {
				continue
			}
			sum := 0.0
			for j := index + 1 - period; j <= index; j++ {
				sum += byDate[dates[j]].Close
			}
			if out.MA == nil {
				out.MA = make(map[string]float64, 4)
			}
			out.MA[strconv.Itoa(period)] = sum / float64(period)
		}
		window.Bars = append(window.Bars, out)
	}
	if window.AnchorIndex >= 0 {
		window.BeforeCount = window.AnchorIndex
		window.AfterCount = len(window.Bars) - window.AnchorIndex - 1
		window.Status = "ok"
	} else {
		window.BeforeCount = len(window.Bars)
	}
	if len(window.Bars) == 0 {
		window.Status = "unavailable"
	}
	// A partial window can be due to IPO, holiday, missing provider coverage, or
	// days that have not occurred yet. Report real counts, never pad candles.
	window.Partial = window.AnchorIndex < 0 || window.BeforeCount < redKlineBefore || window.AfterCount < redKlineAfter
	return window
}

func selectRedKlineWindow(code, date string, local, remote []dailyBar, remoteSource string) redKlineWindow {
	cached := buildRedKlineWindow(code, date, local, "cache")
	fetched := buildRedKlineWindow(code, date, remote, remoteSource)
	if fetched.AnchorIndex >= 0 && (cached.AnchorIndex < 0 ||
		(fetched.BeforeCount >= cached.BeforeCount && fetched.AfterCount >= cached.AfterCount &&
			(len(fetched.Bars) == 0 || len(cached.Bars) == 0 ||
				fetched.Bars[0].MA["60"] > 0 || cached.Bars[0].MA["60"] == 0))) {
		return fetched
	}
	return cached
}

func parseRedKlineData(result *data.KLineSourceResult) ([]dailyBar, string) {
	if result == nil || result.Data == nil {
		return nil, ""
	}
	bars := make([]dailyBar, 0, len(*result.Data))
	for _, kd := range *result.Data {
		if len(kd.Day) < 10 {
			continue
		}
		parse := func(value string) float64 {
			v, err := strconv.ParseFloat(strings.TrimSpace(value), 64)
			if err != nil {
				return 0
			}
			return v
		}
		b := dailyBar{Date: kd.Day[:10], Open: parse(kd.Open), High: parse(kd.High), Low: parse(kd.Low), Close: parse(kd.Close)}
		if validRedKlineBar(b) {
			bars = append(bars, b)
		}
	}
	return bars, result.Source
}

func loadRedKlineDaily(date string) map[string][]dailyBar {
	if cache, ok := readT0DailyCachePayload(date); ok {
		return cache.Daily
	}
	if cache, err := readGobRedReplayCache(t0DailyCachePath(date), date); err == nil {
		return cache.Daily
	}
	return nil
}

func preferRedKlineFollowup(date string, original, later []dailyBar) []dailyBar {
	oldWindow := buildRedKlineWindow("", date, original, "cache")
	newWindow := buildRedKlineWindow("", date, later, "cache")
	if newWindow.AnchorIndex >= 0 &&
		(newWindow.AfterCount > oldWindow.AfterCount || oldWindow.AnchorIndex < 0) &&
		newWindow.BeforeCount >= oldWindow.BeforeCount {
		return later
	}
	return original
}

// Choose a single newer daily snapshot instead of merging overlapping dates.
// A ten-calendar-day lead usually includes five subsequent trading sessions;
// holidays or missing snapshots simply leave the window partial.
func loadRedKlineFollowupDaily(date string) map[string][]dailyBar {
	anchor, err := time.Parse("2006-01-02", date)
	if err != nil {
		return nil
	}
	dir := filepath.Join(t0CacheRootPath, "t0", "daily")
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil
	}
	from := anchor.AddDate(0, 0, 10).Format("2006-01-02")
	until := anchor.AddDate(0, 0, 30).Format("2006-01-02")
	next := ""
	for _, entry := range entries {
		name := entry.Name()
		if entry.IsDir() || !strings.HasPrefix(name, "t0_daily_cache_") || !strings.HasSuffix(name, ".gob") {
			continue
		}
		candidate := strings.TrimSuffix(strings.TrimPrefix(name, "t0_daily_cache_"), ".gob")
		if candidate >= from && candidate <= until && (next == "" || candidate < next) {
			next = candidate
		}
	}
	if next == "" {
		return nil
	}
	return loadRedKlineDaily(next)
}

func redKlineWindows(date string, codes []string, daily map[string][]dailyBar, fetch redKlineFetch) map[string]redKlineWindow {
	out := make(map[string]redKlineWindow, len(codes))
	var mu sync.Mutex
	var wg sync.WaitGroup
	sem := make(chan struct{}, 4)
	today := time.Now().In(chinaLocation()).Format("2006-01-02")
	for _, code := range codes {
		wg.Add(1)
		go func(code string) {
			defer wg.Done()
			shortCode := t0ShortCodeFromResultCode(code)
			cached := buildRedKlineWindow(code, date, daily[shortCode], "cache")
			selected := cached
			// Even a cache with the anchor may lack later bars; ask for one coherent
			// up-to-date series, not a patchwork of differently adjusted daily Gobs.
			needMAWarmup := cached.BeforeCount >= redKlineBefore && len(cached.Bars) > 0 && cached.Bars[0].MA["60"] == 0
			if fetch != nil && (cached.AnchorIndex < 0 || cached.BeforeCount < redKlineBefore || needMAWarmup || (date < today && cached.AfterCount < redKlineAfter)) {
				sem <- struct{}{}
				recent, source := parseRedKlineData(fetch(shortCode, redKlineFetchLimit, ""))
				selected = selectRedKlineWindow(code, date, daily[shortCode], recent, source)
				// The rolling 800-day fallback does not cover indefinitely old archives.
				if selected.AnchorIndex < 0 || (selected.BeforeCount < redKlineBefore && len(recent) > 0 && recent[0].Date > date) ||
					(needMAWarmup && len(selected.Bars) > 0 && selected.Bars[0].MA["60"] == 0) {
					endDate := t0DailyFetchEndDate(date)
					historical, histSource := parseRedKlineData(fetch(shortCode, 160, endDate))
					selected = selectRedKlineWindow(code, date, daily[shortCode], historical, histSource)
				}
				<-sem
			}
			mu.Lock()
			out[code] = selected
			mu.Unlock()
		}(code)
	}
	wg.Wait()
	return out
}

func newRedKlineHandler() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}
		moduleCode := strings.TrimSpace(r.URL.Query().Get("module_code"))
		if moduleCode == "" {
			moduleCode = redT0StrategyModuleCode
		}
		if moduleCode != redT0StrategyModuleCode && moduleCode != sharedGobRedStrategyModuleCode {
			WriteAuthError(w, newAuthError(http.StatusBadRequest, "INVALID_ARGUMENT", "模块不存在"))
			return
		}
		date := strings.TrimSpace(r.URL.Query().Get("date"))
		parsed, err := time.Parse("2006-01-02", date)
		if err != nil || parsed.Format("2006-01-02") != date || date > time.Now().In(chinaLocation()).Format("2006-01-02") {
			WriteJSONStatus(w, http.StatusBadRequest, map[string]string{"error": "date must be a valid past or current YYYY-MM-DD"})
			return
		}
		raw := strings.Split(r.URL.Query().Get("codes"), ",")
		if len(raw) < 1 || len(raw) > redKlineMaxCodes {
			WriteJSONStatus(w, http.StatusBadRequest, map[string]string{"error": "codes must contain 1 to 25 stocks"})
			return
		}
		codes := make([]string, 0, len(raw))
		seen := make(map[string]bool, len(raw))
		for _, part := range raw {
			code := strings.ToUpper(strings.TrimSpace(part))
			if !redKlineCodePattern.MatchString(code) {
				WriteJSONStatus(w, http.StatusBadRequest, map[string]string{"error": fmt.Sprintf("invalid stock code: %s", part)})
				return
			}
			if !seen[code] {
				codes = append(codes, code)
				seen[code] = true
			}
		}
		if moduleCode == sharedGobRedStrategyModuleCode {
			shared, err := loadMainboardSharedGob()
			if err != nil {
				WriteJSONStatus(w, http.StatusServiceUnavailable, map[string]interface{}{"error": err.Error()})
				return
			}
			daily := make(map[string][]dailyBar, len(codes))
			archived := loadRedKlineDaily(date)
			for _, code := range codes {
				shortCode := t0ShortCodeFromResultCode(code)
				bars := shared.Daily[shortCode]
				if _, index := dailyBarOnDate(bars, date); index < 0 {
					if archivedBars := archived[shortCode]; len(archivedBars) > 0 {
						bars = archivedBars
					}
				}
				daily[shortCode] = bars
			}
			WriteJSON(w, map[string]interface{}{
				"date":  date,
				"items": redKlineWindows(date, codes, daily, fetchRedKlineSeries),
			})
			return
		}
		daily := loadRedKlineDaily(date)
		if date < time.Now().In(chinaLocation()).Format("2006-01-02") {
			followup := loadRedKlineFollowupDaily(date)
			for _, code := range codes {
				shortCode := t0ShortCodeFromResultCode(code)
				if len(followup[shortCode]) == 0 {
					continue
				}
				if daily == nil {
					daily = make(map[string][]dailyBar)
				}
				daily[shortCode] = preferRedKlineFollowup(date, daily[shortCode], followup[shortCode])
			}
		}
		WriteJSON(w, map[string]interface{}{"date": date, "items": redKlineWindows(date, codes, daily, fetchRedKlineSeries)})
	}
}
