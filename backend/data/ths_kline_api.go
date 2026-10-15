package data

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"sort"
	"strconv"
	"strings"
	"time"

	"go-stock/backend/logger"
)

var thsKLineAPIURL = "https://quota-h.10jqka.com.cn/fuyao/common_hq_aggr/quote/v1/single_kline"

const (
	thsKLineMaxAttempts = 2
	thsKLineRetryDelay  = 250 * time.Millisecond
)

// Exposure-triggered cards can start together. Share this gate across all
// callers so concurrent HTTP handlers cannot create an unbounded THS burst.
var thsKLineRequestSlots = make(chan struct{}, 2)

type thsKLineRequest struct {
	CodeList   []thsKLineCode `json:"code_list"`
	TradeClass string         `json:"trade_class"`
	TimePeriod string         `json:"time_period"`
	TradeDate  int            `json:"trade_date"`
	BeginTime  int            `json:"begin_time"`
	EndTime    int64          `json:"end_time"`
	AdjustType string         `json:"adjust_type"`
	GPID       int            `json:"gpid"`
}

type thsKLineCode struct {
	Codes  []string `json:"codes"`
	Market string   `json:"market"`
}

type thsKLineResponse struct {
	StatusCode int    `json:"status_code"`
	StatusMsg  string `json:"status_msg"`
	Data       struct {
		QuoteData []struct {
			Code       string              `json:"code"`
			DataFields []string            `json:"data_fields"`
			Value      [][]json.RawMessage `json:"value"`
		} `json:"quote_data"`
	} `json:"data"`
}

type THSRealtimeDailyQuote struct {
	Date      string
	PrevDate  string
	Open      float64
	Price     float64
	PrevClose float64
}

func FetchTHSRealtimeDailyQuote(stockCode string, now time.Time) (THSRealtimeDailyQuote, bool) {
	loc := time.FixedZone("Asia/Shanghai", 8*60*60)
	now = now.In(loc)
	result := fetchFromTHSWithTimeout(stockCode, "101", 3, now.Format("2006-01-02 15:04:05"), 2*time.Second)
	if result == nil || result.Data == nil {
		return THSRealtimeDailyQuote{}, false
	}
	return parseTHSRealtimeDailyQuote(*result.Data, now.Format("2006-01-02"))
}

func parseTHSRealtimeDailyQuote(rows []KLineData, tradeDate string) (THSRealtimeDailyQuote, bool) {
	rows = append([]KLineData(nil), rows...)
	sort.SliceStable(rows, func(i, j int) bool { return rows[i].Day < rows[j].Day })
	for i, row := range rows {
		if row.Day != tradeDate || i == 0 || rows[i-1].Day >= tradeDate {
			continue
		}
		open, openErr := strconv.ParseFloat(row.Open, 64)
		price, priceErr := strconv.ParseFloat(row.Close, 64)
		prevClose, prevCloseErr := strconv.ParseFloat(rows[i-1].Close, 64)
		if openErr != nil || prevCloseErr != nil || open <= 0 || prevClose <= 0 {
			return THSRealtimeDailyQuote{}, false
		}
		if priceErr != nil || price <= 0 {
			price = open
		}
		return THSRealtimeDailyQuote{Date: tradeDate, PrevDate: rows[i-1].Day, Open: open, Price: price, PrevClose: prevClose}, true
	}
	return THSRealtimeDailyQuote{}, false
}

// fetchFromTHS reads unadjusted K-lines from the same backend endpoint used by
// the 10jqka stock page. Unsupported markets/periods return an empty result so
// callers can continue through the existing provider fallback chain.
func fetchFromTHS(stockCode, klt string, limit int, end string) *KLineSourceResult {
	return fetchFromTHSWithTimeout(stockCode, klt, limit, end, 12*time.Second)
}

func fetchFromTHSWithTimeout(stockCode, klt string, limit int, end string, timeout time.Duration) *KLineSourceResult {
	empty := &KLineSourceResult{Data: &[]KLineData{}}
	code, market, ok := thsCodeAndMarket(stockCode)
	if !ok || limit <= 0 {
		return empty
	}
	period, ok := thsTimePeriod(klt)
	if !ok {
		return empty
	}
	endTime, ok := thsEndTime(end)
	if !ok {
		return empty
	}

	requestBody := thsKLineRequest{
		CodeList:   []thsKLineCode{{Codes: []string{code}, Market: market}},
		TradeClass: "intraday",
		TimePeriod: period,
		TradeDate:  -1,
		BeginTime:  -limit,
		EndTime:    endTime,
		AdjustType: "actual",
		GPID:       1,
	}

	if timeout <= 0 {
		timeout = 12 * time.Second
	}
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()
	for attempt := 1; attempt <= thsKLineMaxAttempts; attempt++ {
		startedAt := time.Now()
		body, httpStatus, err := postTHSKLineRequest(ctx, timeout, requestBody)
		if err != nil {
			logger.SugaredLogger.Warnf(
				"THS K线请求失败: code=%s end=%s attempt=%d/%d http=%d elapsed=%s err=%v",
				code, end, attempt, thsKLineMaxAttempts, httpStatus, time.Since(startedAt), err,
			)
			if ctx.Err() != nil || attempt == thsKLineMaxAttempts || !waitTHSKLineRetry(ctx, attempt) {
				break
			}
			continue
		}
		if httpStatus != http.StatusOK {
			logger.SugaredLogger.Warnf(
				"THS K线 HTTP状态异常: code=%s end=%s attempt=%d/%d http=%d elapsed=%s",
				code, end, attempt, thsKLineMaxAttempts, httpStatus, time.Since(startedAt),
			)
			if (httpStatus != http.StatusTooManyRequests && httpStatus < http.StatusInternalServerError) ||
				attempt == thsKLineMaxAttempts || !waitTHSKLineRetry(ctx, attempt) {
				break
			}
			continue
		}

		var response thsKLineResponse
		if err := json.Unmarshal(body, &response); err != nil {
			logger.SugaredLogger.Warnf(
				"THS K线响应解析失败: code=%s end=%s attempt=%d/%d http=%d bytes=%d elapsed=%s err=%v",
				code, end, attempt, thsKLineMaxAttempts, httpStatus, len(body), time.Since(startedAt), err,
			)
			if attempt == thsKLineMaxAttempts || !waitTHSKLineRetry(ctx, attempt) {
				break
			}
			continue
		}
		if response.StatusCode != 0 {
			logger.SugaredLogger.Warnf(
				"THS K线业务状态异常: code=%s end=%s attempt=%d/%d http=%d api_status=%d api_msg=%q elapsed=%s",
				code, end, attempt, thsKLineMaxAttempts, httpStatus, response.StatusCode, response.StatusMsg, time.Since(startedAt),
			)
			if attempt == thsKLineMaxAttempts || !waitTHSKLineRetry(ctx, attempt) {
				break
			}
			continue
		}

		for _, quote := range response.Data.QuoteData {
			if quote.Code != code {
				continue
			}
			rows := parseTHSKLineValues(quote.DataFields, quote.Value, period, endTime)
			if len(rows) > 0 {
				if attempt > 1 {
					logger.SugaredLogger.Infof("THS K线重试后恢复: code=%s end=%s attempt=%d rows=%d", code, end, attempt, len(rows))
				}
				return &KLineSourceResult{Data: &rows}
			}
		}
		logger.SugaredLogger.Warnf(
			"THS K线未返回目标数据: code=%s end=%s attempt=%d/%d http=%d api_status=%d api_msg=%q quote_count=%d elapsed=%s",
			code, end, attempt, thsKLineMaxAttempts, httpStatus, response.StatusCode, response.StatusMsg,
			len(response.Data.QuoteData), time.Since(startedAt),
		)
		if attempt == thsKLineMaxAttempts || !waitTHSKLineRetry(ctx, attempt) {
			break
		}
	}
	return empty
}

func postTHSKLineRequest(ctx context.Context, timeout time.Duration, body thsKLineRequest) ([]byte, int, error) {
	select {
	case thsKLineRequestSlots <- struct{}{}:
		defer func() { <-thsKLineRequestSlots }()
	case <-ctx.Done():
		return nil, 0, ctx.Err()
	}

	client := CreateHTTPClientWithTimeout(timeout)
	// The page's static business headers work without session-bound X-Fuyao-Auth/sw8.
	resp, err := client.R().
		SetContext(ctx).
		SetHeader("Content-Type", "application/json").
		SetHeader("Origin", "https://stockpage.10jqka.com.cn").
		SetHeader("Referer", "https://stockpage.10jqka.com.cn/").
		SetHeader("User-Agent", "Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/152.0.0.0 Safari/537.36").
		SetHeader("X-Auth-ProgId", "7047").
		SetHeader("X-Auth-Type", "ths").
		SetHeader("X-Auth-Version", "1.0").
		SetHeader("X-Auth-AppName", "AINVEST").
		SetHeader("Source-Id", "hxkline-NEWS_appNewsFlowHome_Page").
		SetHeader("Platform", "hxkline").
		SetBody(body).
		Post(thsKLineAPIURL)
	if resp == nil {
		return nil, 0, err
	}
	return resp.Body(), resp.StatusCode(), err
}

func waitTHSKLineRetry(ctx context.Context, attempt int) bool {
	timer := time.NewTimer(time.Duration(attempt) * thsKLineRetryDelay)
	defer timer.Stop()
	select {
	case <-timer.C:
		return ctx.Err() == nil
	case <-ctx.Done():
		return false
	}
}

func thsCodeAndMarket(stockCode string) (code, market string, ok bool) {
	value := strings.ToUpper(strings.TrimSpace(stockCode))
	explicitMarket := ""
	if strings.Contains(value, ".") {
		parts := strings.Split(value, ".")
		if len(parts) != 2 {
			return "", "", false
		}
		value, explicitMarket = parts[0], parts[1]
	} else if len(value) > 6 && (strings.HasPrefix(value, "SH") || strings.HasPrefix(value, "SZ") || strings.HasPrefix(value, "BJ")) {
		explicitMarket, value = value[:2], value[2:]
	}

	if len(value) != 6 {
		return "", "", false
	}
	for _, char := range value {
		if char < '0' || char > '9' {
			return "", "", false
		}
	}

	switch explicitMarket {
	case "SH", "SS", "17":
		return value, "17", true
	case "SZ", "33":
		return value, "33", true
	case "BJ", "74":
		return "", "", false
	case "":
		switch value[0] {
		case '5', '6', '9':
			return value, "17", true
		case '0', '1', '2', '3':
			return value, "33", true
		default:
			return "", "", false
		}
	default:
		return "", "", false
	}
}

func thsTimePeriod(klt string) (string, bool) {
	switch strings.TrimSpace(klt) {
	case "1", "2", "3", "5", "10", "15", "30", "45", "60", "120", "180", "240":
		return "min_" + strings.TrimSpace(klt), true
	case "101":
		return "day_1", true
	case "102":
		return "week_1", true
	case "103":
		return "month_1", true
	case "104":
		return "quarter_1", true
	case "106":
		return "year_1", true
	default:
		return "", false
	}
}

func thsEndTime(end string) (int64, bool) {
	value := strings.TrimSpace(end)
	if value == "" {
		return 0, true
	}
	loc := time.FixedZone("Asia/Shanghai", 8*60*60)
	for _, layout := range []string{"2006-01-02", "20060102"} {
		if parsed, err := time.ParseInLocation(layout, value, loc); err == nil {
			return parsed.Add(24*time.Hour - time.Millisecond).UnixMilli(), true
		}
	}
	for _, layout := range []string{"2006-01-02 15:04:05", "20060102150405", "2006-01-02 15:04"} {
		if parsed, err := time.ParseInLocation(layout, value, loc); err == nil {
			return parsed.UnixMilli(), true
		}
	}
	if millis, err := strconv.ParseInt(value, 10, 64); err == nil && millis > 0 {
		return millis, true
	}
	return 0, false
}

func parseTHSKLineValues(fields []string, rows [][]json.RawMessage, period string, endTime int64) []KLineData {
	indices := make(map[string]int, len(fields))
	for i, field := range fields {
		indices[field] = i
	}
	result := make([]KLineData, 0, len(rows))
	minute := strings.HasPrefix(period, "min_")
	for _, row := range rows {
		timestamp, ok := thsRowNumber(row, indices, "1")
		if !ok || timestamp <= 0 || (endTime > 0 && int64(timestamp) > endTime) {
			continue
		}
		open, openOK := thsRowNumber(row, indices, "7")
		high, highOK := thsRowNumber(row, indices, "8")
		low, lowOK := thsRowNumber(row, indices, "9")
		closeValue, closeOK := thsRowNumber(row, indices, "11")
		volume, volumeOK := thsRowNumber(row, indices, "13")
		amount, amountOK := thsRowNumber(row, indices, "19")
		if !openOK || !highOK || !lowOK || !closeOK || !volumeOK || !amountOK {
			continue
		}
		at := time.UnixMilli(int64(timestamp)).In(time.FixedZone("Asia/Shanghai", 8*60*60))
		dayFormat := "2006-01-02"
		if minute {
			dayFormat = "2006-01-02 15:04:05"
		}
		// THS field 13 is shares while KLineData.Volume is hands; field 19 is yuan.
		result = append(result, KLineData{
			Day:    at.Format(dayFormat),
			Open:   fmt.Sprintf("%.2f", open),
			High:   fmt.Sprintf("%.2f", high),
			Low:    fmt.Sprintf("%.2f", low),
			Close:  fmt.Sprintf("%.2f", closeValue),
			Volume: strconv.FormatFloat(volume/100, 'f', -1, 64),
			Amount: fmt.Sprintf("%.2f", amount),
		})
	}
	for i := 1; i < len(result); i++ {
		previousClose, _ := strconv.ParseFloat(result[i-1].Close, 64)
		closeValue, _ := strconv.ParseFloat(result[i].Close, 64)
		high, _ := strconv.ParseFloat(result[i].High, 64)
		low, _ := strconv.ParseFloat(result[i].Low, 64)
		if previousClose <= 0 {
			continue
		}
		result[i].ChangePercent = fmt.Sprintf("%.2f", (closeValue-previousClose)/previousClose*100)
		result[i].ChangeValue = fmt.Sprintf("%.2f", closeValue-previousClose)
		result[i].Amplitude = fmt.Sprintf("%.2f", (high-low)/previousClose*100)
	}
	return result
}

func thsRowNumber(row []json.RawMessage, indices map[string]int, field string) (float64, bool) {
	index, ok := indices[field]
	if !ok || index < 0 || index >= len(row) {
		return 0, false
	}
	var text string
	if err := json.Unmarshal(row[index], &text); err != nil {
		text = strings.TrimSpace(string(row[index]))
	}
	value, err := strconv.ParseFloat(strings.Trim(text, `"`), 64)
	return value, err == nil
}
