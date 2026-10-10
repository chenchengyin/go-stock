package flutter_api

import (
	"fmt"
	"math"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"time"

	"go-stock/backend/data"
)

const redT1IntradayFetchLimit = 500

type redT1IntradayBar struct {
	Time         string  `json:"time"`
	Price        float64 `json:"price"`
	AveragePrice float64 `json:"averagePrice,omitempty"`
}

type redT1IntradaySeries struct {
	Code      string             `json:"code"`
	Date      string             `json:"date"`
	Source    string             `json:"source"`
	PrevClose float64            `json:"prevClose"`
	Bars      []redT1IntradayBar `json:"bars"`
}

func newRedT1IntradayHandler(moduleService *ModuleService) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}
		principal, ok := PrincipalFromContext(r.Context())
		if !ok {
			WriteAuthError(w, newAuthError(http.StatusUnauthorized, "UNAUTHENTICATED", "未认证"))
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
		allowed, err := moduleService.HasModuleAccess(r.Context(), principal.UserID, moduleCode)
		if err != nil {
			WriteAuthError(w, err)
			return
		}
		if !allowed {
			WriteAuthError(w, newAuthError(http.StatusForbidden, "MODULE_FORBIDDEN", "暂无该红策模块访问权限"))
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

		// Resolve from the selected T0 date so archived list entries can show
		// their historical T+1 session. Do not request a session later than today.
		t1Date, ok := resolveNextTradingDay(t0Date, today)
		if !ok {
			writeEmptyRedT1Intraday(w, "")
			return
		}

		items := redT1IntradayItems(t0Date, t1Date, codes)
		WriteJSON(w, map[string]interface{}{"date": t1Date, "items": items})
	}
}

func writeEmptyRedT1Intraday(w http.ResponseWriter, date string) {
	WriteJSON(w, map[string]interface{}{"date": date, "items": map[string]redT1IntradaySeries{}})
}

func redT1IntradayItems(t0Date, date string, codes []string) map[string]redT1IntradaySeries {
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
			var result *data.KLineSourceResult
			var prevClose float64
			var fetchWg sync.WaitGroup
			fetchWg.Add(2)
			go func() {
				defer fetchWg.Done()
				result = data.FetchIntradayKLineForDateWithFallback(
					shortCode, date, redT1IntradayFetchLimit,
				)
			}()
			go func() {
				defer fetchWg.Done()
				prevClose = redT1PreviousClose(shortCode, t0Date)
			}()
			fetchWg.Wait()
			series := parseRedT1IntradaySeries(code, date, result)
			if len(series.Bars) == 0 {
				return
			}
			series.PrevClose = prevClose
			if series.PrevClose <= 0 {
				return
			}
			mu.Lock()
			items[code] = series
			mu.Unlock()
		}(code)
	}
	wg.Wait()
	return items
}

func parseRedT1IntradaySeries(code, date string, result *data.KLineSourceResult) redT1IntradaySeries {
	series := redT1IntradaySeries{Code: code, Date: date, Bars: []redT1IntradayBar{}}
	if result == nil || result.Data == nil {
		return series
	}
	series.Source = result.Source
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

func redT1PreviousClose(shortCode, t0Date string) float64 {
	bars := fetchDailyKLineWithLimit(shortCode, t0Date, 5)
	bar, _ := dailyBarOnDate(bars, t0Date)
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
