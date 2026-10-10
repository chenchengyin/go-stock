package data

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"
)

const thsKLineAPIURL = "https://quota-h.10jqka.com.cn/fuyao/common_hq_aggr/quote/v1/single_kline"

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

// fetchFromTHS reads unadjusted K-lines from the same backend endpoint used by
// the 10jqka stock page. Unsupported markets/periods return an empty result so
// callers can continue through the existing provider fallback chain.
func fetchFromTHS(stockCode, klt string, limit int, end string) *KLineSourceResult {
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

	ctx, cancel := context.WithTimeout(context.Background(), 12*time.Second)
	defer cancel()
	client := CreateHTTPClientWithTimeout(12 * time.Second)
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
		SetBody(requestBody).
		Post(thsKLineAPIURL)
	if err != nil || resp == nil || resp.StatusCode() != http.StatusOK {
		return empty
	}

	var response thsKLineResponse
	if err := json.Unmarshal(resp.Body(), &response); err != nil || response.StatusCode != 0 {
		return empty
	}
	for _, quote := range response.Data.QuoteData {
		if quote.Code != code {
			continue
		}
		rows := parseTHSKLineValues(quote.DataFields, quote.Value, period, endTime)
		return &KLineSourceResult{Data: &rows}
	}
	return empty
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
