package data

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func trendFixture(code, market, date string) map[string]any {
	at, _ := time.ParseInLocation("2006-01-02 15:04", date+" 09:30", time.FixedZone("CST", 28800))
	return map[string]any{"status_code": 0, "data": map[string]any{"quote_data": []any{
		map[string]any{"code": code, "market": market, "base_price": 10,
			"data_fields": []string{"1", "10", "13", "19"},
			"value":       [][]any{{at.UnixMilli(), 11, 100, 1100}, {at.Add(time.Minute).UnixMilli(), 12, 300, 3500}}},
	}}}
}

func TestTHSTrendUsesHistoricalDateAndConvertsCumulativeTurnover(t *testing.T) {
	old := thsTrendAPIURL
	count := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var body map[string]any
		_ = json.NewDecoder(r.Body).Decode(&body)
		if body["trade_date"] != float64(20260723) || body["trade_class"] != "intraday" || body["end_time"] != nil {
			t.Errorf("wrong trend request: %v", body)
		}
		count++
		_ = json.NewEncoder(w).Encode(trendFixture("600498", "17", "2026-07-23"))
	}))
	defer server.Close()
	thsTrendAPIURL = server.URL
	defer func() { thsTrendAPIURL = old }()
	result := fetchTHSTrend("600498", "2026-07-23")
	if result == nil || result.Data == nil || len(*result.Data) != 2 || result.PrevClose != 10 || count != 1 {
		t.Fatalf("unexpected result: %+v requests=%d", result, count)
	}
	rows := *result.Data
	if rows[0].Volume != "1" || rows[1].Volume != "2" || rows[1].Amount != "2400" || rows[1].Close != "12.00" {
		t.Fatalf("cumulative turnover treated as minute increments: %+v", rows)
	}
}

func TestTHSTrendRejectsWrongDateCodeMarketAndDuplicateTime(t *testing.T) {
	for _, kind := range []string{"date", "code", "market", "duplicate", "price"} {
		t.Run(kind, func(t *testing.T) {
			fixture := trendFixture("600498", "17", "2026-07-23")
			quote := fixture["data"].(map[string]any)["quote_data"].([]any)[0].(map[string]any)
			switch kind {
			case "date":
				fixture = trendFixture("600498", "17", "2026-08-10")
			case "code":
				quote["code"] = "000823"
			case "market":
				quote["market"] = "33"
			case "duplicate":
				quote["value"].([][]any)[1][0] = quote["value"].([][]any)[0][0]
			case "price":
				quote["value"].([][]any)[1][1] = 0
			}
			raw, _ := json.Marshal(fixture)
			result := parseTHSTrend(raw, "600498", "17", "2026-07-23")
			if result != nil {
				t.Fatalf("accepted %s: %+v", kind, result)
			}
		})
	}
}

func TestTHSTrendRetriesTransientEmptyResponse(t *testing.T) {
	old := thsTrendAPIURL
	count := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		count++
		if count == 1 {
			_ = json.NewEncoder(w).Encode(map[string]any{"status_code": 0, "data": map[string]any{"quote_data": []any{}}})
			return
		}
		_ = json.NewEncoder(w).Encode(trendFixture("000823", "33", "2024-01-03"))
	}))
	defer server.Close()
	thsTrendAPIURL = server.URL
	defer func() { thsTrendAPIURL = old }()
	result := fetchTHSTrend("000823", "2024-01-03")
	if count != 2 || result == nil || result.Data == nil || len(*result.Data) != 2 {
		t.Fatalf("retry failed: count=%d result=%+v", count, result)
	}
}
