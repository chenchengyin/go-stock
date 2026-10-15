package flutter_api

import (
	"testing"
	"time"
)

func TestFreshT0RealtimeQuoteCacheExpiresAndScopesTradeDate(t *testing.T) {
	const code = "600099"
	defer func() {
		t0RealtimeQuoteCacheMu.Lock()
		delete(t0RealtimeQuoteCache, code)
		t0RealtimeQuoteCacheMu.Unlock()
	}()

	base := time.Date(2026, 10, 9, 9, 25, 0, 0, chinaLocation())
	cacheT0RealtimeQuote(code, "2026-10-09", t0Realtime{Open: 10, Close: 10.1, PrevClose: 9.9, Source: "ths"}, base)

	if quote, ok := freshT0RealtimeQuote(code, "2026-10-09", base.Add(time.Second)); !ok || quote.Source != "ths" {
		t.Fatalf("fresh same-day quote was not returned: quote=%+v ok=%v", quote, ok)
	}
	if _, ok := freshT0RealtimeQuote(code, "2026-10-10", base.Add(time.Second)); ok {
		t.Fatal("quote from a different trade date must not be reused")
	}
	if _, ok := freshT0RealtimeQuote(code, "2026-10-09", base.Add(t0RealtimeQuoteCacheTTL)); ok {
		t.Fatal("expired quote must not be reused")
	}
}

func TestFreshT0RealtimeQuoteDiscardsPreAuctionCacheAtConfirmation(t *testing.T) {
	const code = "600098"
	before := time.Date(2026, 10, 14, 9, 24, 59, 0, chinaLocation())
	cacheT0RealtimeQuote(code, "2026-10-14", t0Realtime{Open: 10, PrevClose: 10}, before)
	if _, ok := freshT0RealtimeQuote(code, "2026-10-14", before.Add(time.Second)); ok {
		t.Fatal("pre-auction quote reused after 09:25")
	}
}
