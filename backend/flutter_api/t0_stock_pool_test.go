package flutter_api

import (
	"encoding/gob"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"

	"github.com/go-resty/resty/v2"

	"go-stock/backend/data"
)

type t0RoundTripFunc func(*http.Request) (*http.Response, error)

func (f t0RoundTripFunc) RoundTrip(r *http.Request) (*http.Response, error) {
	return f(r)
}

func sinaStockPageJSON(t *testing.T, start, count int) []byte {
	t.Helper()
	items := make([]map[string]any, 0, count)
	for i := 0; i < count; i++ {
		items = append(items, map[string]any{
			"code":   fmt.Sprintf("600%03d", start+i),
			"name":   fmt.Sprintf("测试股票%d", start+i),
			"nmc":    1000000,
			"mktcap": 1000000,
		})
	}
	body, err := json.Marshal(items)
	if err != nil {
		t.Fatal(err)
	}
	return body
}

func newSinaTestHTTPClient(roundTrip t0RoundTripFunc) *resty.Client {
	return resty.NewWithClient(&http.Client{Transport: roundTrip}).SetRetryCount(0)
}

func withSinaTestHTTPClient(t *testing.T, roundTrip t0RoundTripFunc) {
	t.Helper()
	orig := data.SharedHTTPClient
	data.SharedHTTPClient = newSinaTestHTTPClient(roundTrip)
	t.Cleanup(func() { data.SharedHTTPClient = orig })
}

func sinaHTTPResponse(status int, body []byte) *http.Response {
	return &http.Response{
		StatusCode: status,
		Body:       io.NopCloser(strings.NewReader(string(body))),
		Header:     make(http.Header),
	}
}

func TestFetchStockPoolFromSinaRetriesTransientPageFailure(t *testing.T) {
	page2Attempts := 0
	withSinaTestHTTPClient(t, func(r *http.Request) (*http.Response, error) {
		page, _ := strconv.Atoi(r.URL.Query().Get("page"))
		switch page {
		case 1:
			return sinaHTTPResponse(http.StatusOK, sinaStockPageJSON(t, 0, 100)), nil
		case 2:
			page2Attempts++
			if page2Attempts == 1 {
				return nil, fmt.Errorf("temporary upstream failure")
			}
			return sinaHTTPResponse(http.StatusOK, sinaStockPageJSON(t, 100, 1)), nil
		default:
			t.Fatalf("unexpected page request: %d", page)
			return nil, nil
		}
	})

	stocks := fetchStockPoolFromSina()
	if len(stocks) != 101 {
		t.Fatalf("transient page failure must be retried without losing the pool: got %d stocks", len(stocks))
	}
	if page2Attempts != 2 {
		t.Fatalf("page 2 attempts=%d, want 2", page2Attempts)
	}
}

func TestFetchStockPoolFromSinaRejectsPartialPoolAfterPageFailure(t *testing.T) {
	page2Attempts := 0
	withSinaTestHTTPClient(t, func(r *http.Request) (*http.Response, error) {
		page, _ := strconv.Atoi(r.URL.Query().Get("page"))
		switch page {
		case 1:
			return sinaHTTPResponse(http.StatusOK, sinaStockPageJSON(t, 0, 100)), nil
		case 2:
			page2Attempts++
			return nil, fmt.Errorf("upstream unavailable")
		default:
			t.Fatalf("unexpected page request: %d", page)
			return nil, nil
		}
	})

	stocks := fetchStockPoolFromSina()
	if len(stocks) != 0 {
		t.Fatalf("failed pagination must not return a partial pool: got %d stocks", len(stocks))
	}
	if page2Attempts < 2 {
		t.Fatalf("page 2 should be retried before failing, attempts=%d", page2Attempts)
	}
}

func TestLoadT0DailyCacheRejectsUnverifiedStockPool(t *testing.T) {
	orig := t0CacheRootPath
	t0CacheRootPath = t.TempDir()
	t.Cleanup(func() { t0CacheRootPath = orig })

	date := "2026-09-04"
	if err := ensureT0CacheDirs(); err != nil {
		t.Fatal(err)
	}
	f, err := os.Create(t0DailyCachePath(date))
	if err != nil {
		t.Fatal(err)
	}
	payload := struct {
		TradeDate         string
		Stocks            []t0Stock
		Daily             map[string][]dailyBar
		StockPoolComplete bool
	}{
		TradeDate: date,
		Stocks:    []t0Stock{{Code: "sh.600000", ShortCode: "600000", Name: "测试股票"}},
		Daily:     map[string][]dailyBar{"600000": {{Date: "2026-09-03", Close: 10}}},
	}
	if err := gob.NewEncoder(f).Encode(&payload); err != nil {
		_ = f.Close()
		t.Fatal(err)
	}
	if err := f.Close(); err != nil {
		t.Fatal(err)
	}

	if _, ok := loadT0DailyCache(date); ok {
		t.Fatal("cache without a verified complete stock pool must be rejected")
	}
	if isT0DailyCacheFilePresent(date) {
		t.Fatal("unverified cache must not be reported as present")
	}
}

func TestIsT0DailyCacheFilePresentRequiresReadableCache(t *testing.T) {
	orig := t0CacheRootPath
	t0CacheRootPath = t.TempDir()
	t.Cleanup(func() { t0CacheRootPath = orig })

	date := "2026-09-05"
	if err := os.MkdirAll(filepath.Dir(t0DailyCachePath(date)), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(t0DailyCachePath(date), []byte("not a gob"), 0o644); err != nil {
		t.Fatal(err)
	}

	if isT0DailyCacheFilePresent(date) {
		t.Fatal("corrupted cache must not be reported as present")
	}
}

func TestSaveT0DailyCacheMarksStockPoolComplete(t *testing.T) {
	orig := t0CacheRootPath
	t0CacheRootPath = t.TempDir()
	t.Cleanup(func() { t0CacheRootPath = orig })

	date := "2026-09-06"
	stocks := []t0Stock{{Code: "sh.600000", ShortCode: "600000", Name: "测试股票"}}
	daily := map[string][]dailyBar{"600000": {{Date: "2026-09-05", Close: 10}}}
	if err := saveT0DailyCache(date, stocks, daily); err != nil {
		t.Fatal(err)
	}

	f, err := os.Open(t0DailyCachePath(date))
	if err != nil {
		t.Fatal(err)
	}
	var payload struct {
		TradeDate         string
		Stocks            []t0Stock
		Daily             map[string][]dailyBar
		StockPoolComplete bool
	}
	if err := gob.NewDecoder(f).Decode(&payload); err != nil {
		_ = f.Close()
		t.Fatal(err)
	}
	if err := f.Close(); err != nil {
		t.Fatal(err)
	}
	if !payload.StockPoolComplete {
		t.Fatal("saveT0DailyCache must persist the complete stock-pool marker")
	}
}
