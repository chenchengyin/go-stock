package flutter_api

import (
	"testing"
	"time"
)

func TestSharedGobRedAuctionRequiresTimestampVerifiedTencentQuotes(t *testing.T) {
	tradeDate := time.Now().In(chinaLocation()).Format("2006-01-02")
	oldFetcher := sharedGobRedLiveQuoteFetcher
	sharedGobRedLiveQuoteFetcher = func([]t0Stock) map[string]t0Realtime {
		return map[string]t0Realtime{
			"600000": {Open: 10, PrevClose: 10, Source: "tencent", TradeDate: tradeDate},
			"600001": {Open: 11, PrevClose: 10, Source: "ths", TradeDate: tradeDate},
			"600002": {Open: 12, PrevClose: 10, Source: "tencent", TradeDate: "2026-10-13"},
		}
	}
	t.Cleanup(func() { sharedGobRedLiveQuoteFetcher = oldFetcher })
	stocks := []t0Stock{{ShortCode: "600000"}, {ShortCode: "600001"}, {ShortCode: "600002"}}
	quotes, source, warning := buildSharedGobRedAuctionQuotes(tradeDate, stocks, nil)
	if source != "realtime_open" || len(quotes) != 1 || quotes["600000"].Open != 10 {
		t.Fatalf("unverified quote admitted: source=%s quotes=%v", source, quotes)
	}
	if warning == nil || warning.Missing != 2 {
		t.Fatalf("missing verified quotes should keep confirmation waiting: %+v", warning)
	}
}

func TestSharedGobSuspensionCannotBeConfirmedBeforeAuctionEnds(t *testing.T) {
	date := "2026-10-15"
	quote := t0Realtime{TradeDate: date, PrevClose: 10, Open: 0, Volume: 0}
	before := time.Date(2026, 10, 15, 9, 24, 59, 0, chinaLocation())
	after := time.Date(2026, 10, 15, 9, 25, 0, 0, chinaLocation())
	if isConfirmedSharedGobSuspension(quote, date, before) {
		t.Fatal("a pre-auction zero quote was treated as a suspension")
	}
	if !isConfirmedSharedGobSuspension(quote, date, after) {
		t.Fatal("a post-auction zero quote was not treated as a suspension")
	}
	quote.Open = 10
	if isConfirmedSharedGobSuspension(quote, date, after) {
		t.Fatal("an opened stock was treated as a suspension")
	}
}

func TestTencentAuctionTimestampBoundary(t *testing.T) {
	before := time.Date(2026, 10, 15, 9, 24, 59, 0, chinaLocation())
	after := time.Date(2026, 10, 15, 9, 25, 0, 0, chinaLocation())
	if !validTencentT0QuoteTimestamp("20261015092459", before) {
		t.Fatal("same-day pre-auction quote should be available to preview")
	}
	if validTencentT0QuoteTimestamp("20261015092459", after) ||
		validTencentT0QuoteTimestamp("20261014092500", after) ||
		validTencentT0QuoteTimestamp("20261015", after) {
		t.Fatal("stale or undated quote entered the confirmed auction")
	}
	if !validTencentT0QuoteTimestamp("20261015092500", after) {
		t.Fatal("confirmed auction quote was rejected")
	}
}
