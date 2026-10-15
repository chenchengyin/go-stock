package data

import (
	"context"
	"encoding/json"
	"math"
	"net/http"
	"sort"
	"strconv"
	"time"

	"go-stock/backend/logger"
)

var thsTrendAPIURL = "https://quota-h.10jqka.com.cn/fuyao/common_hq_aggr/quote/v1/single_trend"

// Historical trends use a calendar date, not a minute-K-line end timestamp.
func fetchTHSTrend(stockCode, targetDate string) *KLineSourceResult {
	code, market, ok := thsCodeAndMarket(stockCode)
	date, err := time.Parse("2006-01-02", targetDate)
	if !ok || err != nil || date.Format("2006-01-02") != targetDate {
		return nil
	}
	tradeDate, _ := strconv.Atoi(date.Format("20060102"))
	body := map[string]any{"code_list": []thsKLineCode{{Codes: []string{code}, Market: market}}, "trade_class": "intraday", "trade_date": tradeDate, "gpid": 1}
	ctx, cancel := context.WithTimeout(context.Background(), 12*time.Second)
	defer cancel()
	for attempt := 1; attempt <= thsKLineMaxAttempts; attempt++ {
		raw, status, err := postTHSTrendRequest(ctx, body)
		if err == nil && status == http.StatusOK {
			if result := parseTHSTrend(raw, code, market, targetDate); result != nil {
				return result
			}
		}
		logger.SugaredLogger.Warnf("THS 历史分时未取得有效数据: code=%s date=%s attempt=%d http=%d err=%v", code, targetDate, attempt, status, err)
		if ctx.Err() != nil || (status >= 400 && status < 500 && status != 429) || attempt == thsKLineMaxAttempts || !waitTHSKLineRetry(ctx, attempt) {
			break
		}
	}
	return nil
}

func postTHSTrendRequest(ctx context.Context, body any) ([]byte, int, error) {
	select {
	case thsKLineRequestSlots <- struct{}{}:
		defer func() { <-thsKLineRequestSlots }()
	case <-ctx.Done():
		return nil, 0, ctx.Err()
	}
	response, err := CreateHTTPClientWithTimeout(12*time.Second).R().SetContext(ctx).
		SetHeader("Content-Type", "application/json").
		SetHeader("Origin", "https://stockpage.10jqka.com.cn").
		SetHeader("Referer", "https://stockpage.10jqka.com.cn/").
		SetHeader("User-Agent", "Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/152.0.0.0 Safari/537.36").
		SetHeader("X-Auth-ProgId", "7047").SetHeader("X-Auth-Type", "ths").SetHeader("X-Auth-Version", "1.0").
		SetHeader("X-Auth-AppName", "AINVEST").SetHeader("Source-Id", "hxkline-NEWS_appNewsFlowHome_Page").SetHeader("Platform", "hxkline").
		SetBody(body).Post(thsTrendAPIURL)
	if response == nil {
		return nil, 0, err
	}
	return response.Body(), response.StatusCode(), err
}

func parseTHSTrend(raw []byte, code, market, date string) *KLineSourceResult {
	var response struct {
		StatusCode int `json:"status_code"`
		Data       struct {
			QuoteData []struct {
				Code      string              `json:"code"`
				Market    string              `json:"market"`
				BasePrice float64             `json:"base_price"`
				Fields    []string            `json:"data_fields"`
				Values    [][]json.RawMessage `json:"value"`
			} `json:"quote_data"`
		} `json:"data"`
	}
	if json.Unmarshal(raw, &response) != nil || response.StatusCode != 0 {
		return nil
	}
	for _, q := range response.Data.QuoteData {
		if q.Code != code || q.Market != market || len(q.Values) < 2 {
			continue
		}
		indices := map[string]int{}
		for i, field := range q.Fields {
			indices[field] = i
		}
		type point struct {
			at                    int64
			price, volume, amount float64
		}
		points := make([]point, 0, len(q.Values))
		for _, row := range q.Values {
			at, atOK := thsRowNumber(row, indices, "1")
			price, priceOK := thsRowNumber(row, indices, "10")
			if !atOK || !priceOK || math.IsNaN(at) || math.IsInf(at, 0) || at <= 0 || math.IsNaN(price) || math.IsInf(price, 0) || price <= 0 {
				return nil
			}
			local := time.UnixMilli(int64(at)).In(time.FixedZone("Asia/Shanghai", 28800))
			clock := local.Format("15:04")
			if local.Format("2006-01-02") != date || !(clock >= "09:30" && clock <= "11:30" || clock >= "13:00" && clock <= "15:00") {
				return nil
			}
			volume, _ := thsRowNumber(row, indices, "13")
			amount, _ := thsRowNumber(row, indices, "19")
			if math.IsNaN(volume) || math.IsInf(volume, 0) || volume < 0 {
				volume = 0
			}
			if math.IsNaN(amount) || math.IsInf(amount, 0) || amount < 0 {
				amount = 0
			}
			points = append(points, point{int64(at), price, volume, amount})
		}
		sort.Slice(points, func(i, j int) bool { return points[i].at < points[j].at })
		rows := make([]KLineData, 0, len(points))
		var prevVolume, prevAmount float64
		for i, p := range points {
			if i > 0 && p.at == points[i-1].at {
				return nil
			}
			// single_trend returns cumulative shares/yuan; downstream expects minute hands/yuan.
			volume, amount := p.volume-prevVolume, p.amount-prevAmount
			if volume < 0 || amount < 0 {
				return nil
			}
			prevVolume, prevAmount = p.volume, p.amount
			price := strconv.FormatFloat(p.price, 'f', 2, 64)
			rows = append(rows, KLineData{Day: time.UnixMilli(p.at).In(time.FixedZone("Asia/Shanghai", 28800)).Format("2006-01-02 15:04:05"), Open: price, High: price, Low: price, Close: price, Volume: strconv.FormatFloat(volume/100, 'f', -1, 64), Amount: strconv.FormatFloat(amount, 'f', -1, 64)})
		}
		base := q.BasePrice
		if math.IsNaN(base) || math.IsInf(base, 0) || base < 0 {
			base = 0
		}
		return &KLineSourceResult{Data: &rows, Source: "ths", PrevClose: base}
	}
	return nil
}
