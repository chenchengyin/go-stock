package flutter_api

import (
	"math"
	"strconv"
	"strings"
	"sync"

	"go-stock/backend/data"
)

type previousHighCacheEntry struct {
	value float64
	ok    bool
}

var previousHighCache = struct {
	sync.RWMutex
	entries map[string]previousHighCacheEntry
}{
	entries: make(map[string]previousHighCacheEntry),
}

// selectPreviousHigh returns the highest-dated valid daily bar strictly before quoteDate.
func selectPreviousHigh(bars []data.KLineData, quoteDate string) (float64, bool) {
	quoteDay := normalizeMarketDate(quoteDate)
	if quoteDay == "" {
		return 0, false
	}

	var bestDay string
	var bestHigh float64
	for _, bar := range bars {
		day := normalizeMarketDate(bar.Day)
		if day == "" || day >= quoteDay || day <= bestDay {
			continue
		}
		high, err := strconv.ParseFloat(strings.TrimSpace(bar.High), 64)
		if err != nil || high <= 0 || math.IsNaN(high) || math.IsInf(high, 0) {
			continue
		}
		bestDay = day
		bestHigh = high
	}
	if bestDay == "" {
		return 0, false
	}
	return bestHigh, true
}

func getPreviousHigh(stockCode, quoteDate string) float64 {
	day := normalizeMarketDate(quoteDate)
	if strings.TrimSpace(stockCode) == "" || day == "" {
		return 0
	}

	cacheKey := strings.ToLower(strings.TrimSpace(stockCode)) + "|" + day
	previousHighCache.RLock()
	cached, ok := previousHighCache.entries[cacheKey]
	previousHighCache.RUnlock()
	if ok {
		if cached.ok {
			return cached.value
		}
		return 0
	}

	result := data.FetchKLineWithFallback(stockCode, "", "101", 10, day)
	var value float64
	if result != nil && result.Data != nil {
		if high, found := selectPreviousHigh(*result.Data, day); found {
			value = high
		}
	}

	previousHighCache.Lock()
	if len(previousHighCache.entries) >= 2048 {
		previousHighCache.entries = make(map[string]previousHighCacheEntry)
	}
	previousHighCache.entries[cacheKey] = previousHighCacheEntry{
		value: value,
		ok:    value > 0,
	}
	previousHighCache.Unlock()
	return value
}

func normalizeMarketDate(value string) string {
	value = strings.TrimSpace(value)
	if len(value) >= 10 && value[4] == '-' && value[7] == '-' {
		return value[:10]
	}
	if len(value) >= 8 {
		compact := value[:8]
		if _, err := strconv.Atoi(compact); err == nil {
			return compact[:4] + "-" + compact[4:6] + "-" + compact[6:8]
		}
	}
	return ""
}
