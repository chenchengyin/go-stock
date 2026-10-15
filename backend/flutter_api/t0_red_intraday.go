package flutter_api

import (
	"encoding/json"
	"fmt"
	"math"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"

	"go-stock/backend/data"
	"go-stock/backend/logger"
)

const redT1IntradayFetchLimit = 500

var redT1IntradayFetcher = data.FetchIntradayKLineForDateWithFallback

type redT1IntradayBar struct {
	Time         string  `json:"time"`
	Price        float64 `json:"price"`
	AveragePrice float64 `json:"averagePrice,omitempty"`
}

type redT1IntradaySeries struct {
	Code      string             `json:"code"`
	Date      string             `json:"date"`
	Source    string             `json:"source"`
	Status    string             `json:"status"`
	Message   string             `json:"message,omitempty"`
	PrevClose float64            `json:"prevClose"`
	Bars      []redT1IntradayBar `json:"bars"`
}

type redT1IntradayCacheRecord struct {
	Version int                 `json:"version"`
	Series  redT1IntradaySeries `json:"series"`
}

func redT1IntradayCachePath(code, date string) string {
	return filepath.Join(t0CacheRootPath, "t0", "intraday", date,
		t0ShortCodeFromResultCode(code)+".json")
}

func loadRedT1IntradayCache(code, date string) (*redT1IntradaySeries, bool) {
	contents, err := os.ReadFile(redT1IntradayCachePath(code, date))
	if err != nil {
		return nil, false
	}
	var record redT1IntradayCacheRecord
	if err := json.Unmarshal(contents, &record); err != nil || record.Version != 2 ||
		record.Series.Date != date || len(record.Series.Bars) < 2 {
		return nil, false
	}
	record.Series.Code = code
	return &record.Series, true
}

func saveRedT1IntradayCache(series redT1IntradaySeries) error {
	if series.Date == "" || len(series.Bars) < 2 || series.Source == "" {
		return nil
	}
	path := redT1IntradayCachePath(series.Code, series.Date)
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}
	contents, err := json.Marshal(redT1IntradayCacheRecord{Version: 2, Series: series})
	if err != nil {
		return err
	}
	temp, err := os.CreateTemp(dir, ".intraday-*.tmp")
	if err != nil {
		return err
	}
	defer os.Remove(temp.Name())
	if _, err := temp.Write(contents); err != nil {
		_ = temp.Close()
		return err
	}
	if err := temp.Close(); err != nil {
		return err
	}
	return os.Rename(temp.Name(), path)
}

func redT1SessionIsComplete(date string, now time.Time) bool {
	local := now.In(chinaLocation())
	today := local.Format("2006-01-02")
	if date < today {
		return true
	}
	return date == today && local.Hour()*60+local.Minute() >= 15*60+5
}

func newRedT1IntradayHandler() http.HandlerFunc {
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

		t0Date := strings.TrimSpace(r.URL.Query().Get("date"))
		parsed, err := time.Parse("2006-01-02", t0Date)
		today := time.Now().In(chinaLocation()).Format("2006-01-02")
		if err != nil || parsed.Format("2006-01-02") != t0Date || t0Date > today {
			WriteJSONStatus(w, http.StatusBadRequest, map[string]string{"error": "date must be a valid past or current YYYY-MM-DD"})
			return
		}
		rawCodes := strings.Split(r.URL.Query().Get("codes"), ",")
		if len(rawCodes) < 1 || len(rawCodes) > redKlineMaxCodes {
			WriteJSONStatus(w, http.StatusBadRequest, map[string]string{"error": "codes must contain 1 to 25 stocks"})
			return
		}
		codes := make([]string, 0, len(rawCodes))
		seen := make(map[string]bool, len(rawCodes))
		for _, raw := range rawCodes {
			code := strings.ToUpper(strings.TrimSpace(raw))
			if !redKlineCodePattern.MatchString(code) {
				WriteJSONStatus(w, http.StatusBadRequest, map[string]string{"error": fmt.Sprintf("invalid stock code: %s", raw)})
				return
			}
			if !seen[code] {
				codes = append(codes, code)
				seen[code] = true
			}
		}

		// The default is the selected T0's next trading session. Explicit
		// sessions let the chart move in either direction without guessing
		// holidays in the client or borrowing a different day's previous close.
		sessionDate, previousDate, nextDate, valid := resolveRedT1Session(
			t0Date, strings.TrimSpace(r.URL.Query().Get("session_date")), today,
			isValidTradingDay, resolveNextTradingDay, resolvePrevTradingDay,
		)
		if !valid {
			WriteJSONStatus(w, http.StatusBadRequest, map[string]string{"error": "session_date must be an existing trading day not later than today"})
			return
		}
		if sessionDate == "" {
			writeEmptyRedT1Intraday(w, codes)
			return
		}
		items := redT1IntradayItems(previousDate, sessionDate, codes)
		WriteJSON(w, map[string]interface{}{
			"date": sessionDate, "previous_date": previousDate,
			"next_date": nextDate, "items": items,
		})
	}
}

// Resolve chart navigation from actual exchange sessions. Keep the previous
// session separate: its daily close is the only valid percentage baseline.
func resolveRedT1Session(
	t0Date, requestedDate, today string,
	isTradingDay func(string) bool,
	next func(string, string) (string, bool),
	prev func(string) (string, bool),
) (sessionDate, previousDate, nextDate string, valid bool) {
	if requestedDate == "" {
		sessionDate, _ = next(t0Date, today)
		if sessionDate == "" {
			return "", "", "", true
		}
	} else {
		day, err := time.Parse("2006-01-02", requestedDate)
		if err != nil || day.Format("2006-01-02") != requestedDate || requestedDate > today || !isTradingDay(requestedDate) {
			return "", "", "", false
		}
		sessionDate = requestedDate
	}
	// The T+1 baseline is exactly the selected T0's close, regardless of
	// another stock's daily-calendar lookup being temporarily unavailable.
	if requestedDate == "" {
		previousDate = t0Date
	} else if t1, found := next(t0Date, today); found && t1 == sessionDate {
		previousDate = t0Date
	} else {
		previousDate, _ = prev(sessionDate)
	}
	nextDate, _ = next(sessionDate, today)
	return sessionDate, previousDate, nextDate, true
}

func writeEmptyRedT1Intraday(w http.ResponseWriter, codes []string) {
	items := make(map[string]redT1IntradaySeries, len(codes))
	for _, code := range codes {
		items[code] = redT1IntradaySeries{
			Code: code, Status: "no_session",
			Message: "所选日期之后暂无已发生的交易日，暂时没有 T+1 分时",
			Bars:    []redT1IntradayBar{},
		}
	}
	WriteJSON(w, map[string]interface{}{
		"date": "", "previous_date": "", "next_date": "", "items": items,
	})
}

func redT1IntradayItems(previousDate, date string, codes []string) map[string]redT1IntradaySeries {
	items := make(map[string]redT1IntradaySeries, len(codes))
	var mu sync.Mutex
	var wg sync.WaitGroup
	sem := make(chan struct{}, 4)
	for _, code := range codes {
		wg.Add(1)
		go func(code string) {
			defer wg.Done()
			sem <- struct{}{}
			defer func() { <-sem }()

			shortCode := t0ShortCodeFromResultCode(code)
			if cached, ok := loadRedT1IntradayCache(code, date); ok {
				if cached.PrevClose <= 0 && previousDate != "" {
					cached.PrevClose = redT1PreviousClose(shortCode, previousDate)
				}
				setRedT1IntradayStatus(cached)
				mu.Lock()
				items[code] = *cached
				mu.Unlock()
				return
			}

			var result *data.KLineSourceResult
			result = redT1IntradayFetcher(shortCode, date, redT1IntradayFetchLimit)
			series := parseRedT1IntradaySeries(code, date, result)
			if series.PrevClose <= 0 && len(series.Bars) >= 2 && previousDate != "" {
				series.PrevClose = redT1PreviousClose(shortCode, previousDate)
			}
			setRedT1IntradayStatus(&series)
			if redT1SessionIsComplete(date, time.Now()) && len(series.Bars) >= 2 {
				if err := saveRedT1IntradayCache(series); err != nil {
					logger.SugaredLogger.Warnf("保存分时缓存失败: code=%s date=%s err=%v", code, date, err)
				}
			}
			mu.Lock()
			items[code] = series
			mu.Unlock()
		}(code)
	}
	wg.Wait()
	return items
}

func setRedT1IntradayStatus(series *redT1IntradaySeries) {
	switch {
	case len(series.Bars) == 0:
		series.Status = "no_intraday_data"
		series.Message = "同花顺、东方财富、新浪、通达信均未返回该交易日的有效分钟线"
	case series.PrevClose <= 0:
		series.Status = "missing_previous_close"
		series.Message = "已取得分钟线，但未取得上一交易日收盘价，无法计算涨跌幅"
	case len(series.Bars) < 2:
		series.Status = "insufficient_data"
		series.Message = "有效分钟线不足 2 个点，无法绘制分时"
	default:
		series.Status = "ok"
		series.Message = ""
	}
}

func parseRedT1IntradaySeries(code, date string, result *data.KLineSourceResult) redT1IntradaySeries {
	series := redT1IntradaySeries{Code: code, Date: date, Bars: []redT1IntradayBar{}}
	if result == nil || result.Data == nil {
		return series
	}
	series.Source = result.Source
	series.PrevClose = result.PrevClose
	if math.IsNaN(series.PrevClose) || math.IsInf(series.PrevClose, 0) {
		series.PrevClose = 0
	}
	useTurnoverAverage := result.Source == "ths" || result.Source == "eastmoney"
	var cumulativeVolumeShares, cumulativeAmount float64
	for _, bar := range *result.Data {
		if redIntradayDate(bar.Day) != date {
			continue
		}
		clock := minuteClock(bar.Day)
		price, err := strconv.ParseFloat(strings.TrimSpace(bar.Close), 64)
		if clock == "" || err != nil || price <= 0 || math.IsNaN(price) || math.IsInf(price, 0) {
			continue
		}
		volume, _ := strconv.ParseFloat(strings.TrimSpace(bar.Volume), 64)
		amount, _ := strconv.ParseFloat(strings.TrimSpace(bar.Amount), 64)
		if math.IsNaN(volume) || math.IsInf(volume, 0) || volume < 0 {
			volume = 0
		}
		minute := redT1IntradayBar{Time: clock, Price: price}
		if useTurnoverAverage && volume > 0 && amount > 0 && !math.IsNaN(amount) && !math.IsInf(amount, 0) {
			// THS and EastMoney minute volumes are in hands; amounts are in yuan.
			cumulativeVolumeShares += volume * 100
			cumulativeAmount += amount
		}
		if useTurnoverAverage && cumulativeVolumeShares > 0 {
			minute.AveragePrice = cumulativeAmount / cumulativeVolumeShares
		}
		series.Bars = append(series.Bars, minute)
	}
	return series
}

func redT1PreviousClose(shortCode, previousDate string) float64 {
	if shared, err := loadMainboardSharedGob(); err == nil {
		if bar, _ := dailyBarOnDate(shared.Daily[shortCode], previousDate); bar.Close > 0 {
			return bar.Close
		}
	}
	if cached, ok := readT0DailyCachePayloadFromDisk(previousDate); ok {
		if bar, _ := dailyBarOnDate(cached.Daily[shortCode], previousDate); bar.Close > 0 {
			return bar.Close
		}
	}
	bars := fetchDailyKLineWithLimit(shortCode, previousDate, 5)
	bar, _ := dailyBarOnDate(bars, previousDate)
	if bar.Close <= 0 || math.IsNaN(bar.Close) || math.IsInf(bar.Close, 0) {
		return 0
	}
	return bar.Close
}

func redIntradayDate(value string) string {
	value = strings.TrimSpace(value)
	if len(value) < 10 {
		return ""
	}
	return strings.NewReplacer("/", "-", ".", "-").Replace(value[:10])
}

func minuteClock(value string) string {
	value = strings.TrimSpace(value)
	index := strings.IndexAny(value, " T")
	if index < 0 || index+1 >= len(value) {
		return ""
	}
	clock := value[index+1:]
	if len(clock) >= 5 && clock[2] == ':' {
		return clock[:5]
	}
	if len(clock) >= 4 && clock[:4] >= "0000" && clock[:4] <= "2359" {
		return clock[:2] + ":" + clock[2:4]
	}
	return ""
}
