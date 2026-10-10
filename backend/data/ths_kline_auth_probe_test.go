package data

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"os"
	"strings"
	"testing"
	"time"
)

const thsKLineProbeURL = "https://quota-h.10jqka.com.cn/fuyao/common_hq_aggr/quote/v1/single_kline"

type thsKLineProbeResponse struct {
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

// TestTHSKLineAuthProbe compares direct backend requests with and without the
// page's auth headers. Set THS_KLINE_LIVE_PROBE=1 to run it explicitly.
func TestTHSKLineAuthProbe(t *testing.T) {
	if os.Getenv("THS_KLINE_LIVE_PROBE") != "1" {
		t.Skip("set THS_KLINE_LIVE_PROBE=1 to call the live THS endpoint")
	}

	body, err := json.Marshal(map[string]any{
		"code_list":   []any{map[string]any{"codes": []string{"000409"}, "market": "33"}},
		"trade_class": "intraday",
		"time_period": "day_1",
		"trade_date":  -1,
		"begin_time":  -400,
		"end_time":    0,
		"adjust_type": "forward",
		"gpid":        1,
	})
	if err != nil {
		t.Fatal(err)
	}

	type headerSet struct {
		name    string
		headers map[string]string
	}
	baseHeaders := map[string]string{
		"Content-Type": "application/json",
		"Origin":       "https://stockpage.10jqka.com.cn",
		"Referer":      "https://stockpage.10jqka.com.cn/",
		"User-Agent":   "Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/152.0.0.0 Safari/537.36",
	}
	appHeaders := map[string]string{
		"X-Auth-ProgId":  "7047",
		"X-Auth-Type":    "ths",
		"X-Auth-Version": "1.0",
		"X-Auth-AppName": "AINVEST",
		"Source-Id":      "hxkline-NEWS_appNewsFlowHome_Page",
		"Platform":       "hxkline",
	}
	clone := func(src map[string]string) map[string]string {
		out := make(map[string]string, len(src))
		for key, value := range src {
			out[key] = value
		}
		return out
	}
	withAppHeaders := clone(baseHeaders)
	for key, value := range appHeaders {
		withAppHeaders[key] = value
	}
	sets := []headerSet{
		{name: "origin_referer_only", headers: baseHeaders},
		{name: "app_headers_without_fuyao_auth", headers: withAppHeaders},
	}
	if token := strings.TrimSpace(os.Getenv("THS_FUYAO_AUTH")); token != "" {
		fuyaoOnly := clone(baseHeaders)
		fuyaoOnly["X-Fuyao-Auth"] = token
		sets = append(sets, headerSet{name: "x_fuyao_auth_only", headers: fuyaoOnly})

		fullHeaders := clone(withAppHeaders)
		fullHeaders["X-Fuyao-Auth"] = token
		if sw8 := strings.TrimSpace(os.Getenv("THS_SW8")); sw8 != "" {
			fullHeaders["sw8"] = sw8
		}
		sets = append(sets, headerSet{name: "full_page_auth_headers", headers: fullHeaders})
	} else {
		t.Log("THS_FUYAO_AUTH is unset; full_page_auth_headers case is omitted")
	}

	client := &http.Client{Timeout: 10 * time.Second}
	anySuccess := false
	for _, set := range sets {
		set := set
		t.Run(set.name, func(t *testing.T) {
			ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
			defer cancel()
			req, err := http.NewRequestWithContext(ctx, http.MethodPost, thsKLineProbeURL, bytes.NewReader(body))
			if err != nil {
				t.Fatal(err)
			}
			for key, value := range set.headers {
				req.Header.Set(key, value)
			}

			resp, err := client.Do(req)
			if err != nil {
				t.Fatalf("request failed: %v", err)
			}
			defer resp.Body.Close()
			responseBody, err := io.ReadAll(io.LimitReader(resp.Body, 2<<20))
			if err != nil {
				t.Fatalf("read response: %v", err)
			}

			var parsed thsKLineProbeResponse
			if err := json.Unmarshal(responseBody, &parsed); err != nil {
				t.Logf("HTTP %d returned non-JSON or unexpected body (%d bytes)", resp.StatusCode, len(responseBody))
				return
			}
			barCount := 0
			for _, quote := range parsed.Data.QuoteData {
				if quote.Code == "000409" {
					barCount += len(quote.Value)
					start := len(quote.Value) - 5
					if start < 0 {
						start = 0
					}
					sample, _ := json.Marshal(quote.Value[start:])
					t.Logf("code=%s data_fields=%v latest_rows=%s", quote.Code, quote.DataFields, sample)
				}
			}
			ok := resp.StatusCode == http.StatusOK && parsed.StatusCode == 0 && barCount > 0
			t.Logf("HTTP=%d api_status=%d msg=%q bars=%d", resp.StatusCode, parsed.StatusCode, parsed.StatusMsg, barCount)
			if ok {
				anySuccess = true
			}
		})
	}
	if !anySuccess {
		t.Fatal("no request variant returned K-line bars; inspect the per-case status logs")
	}
}
