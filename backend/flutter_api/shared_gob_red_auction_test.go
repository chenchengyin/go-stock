package flutter_api

import (
	"strings"
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

func TestTencentNonTradingMorningQuoteDoesNotBlockSharedHistory(t *testing.T) {
	now := time.Date(2026, 10, 19, 9, 30, 0, 0, chinaLocation())
	// Tencent keeps these non-trading quotes at 09:00 even after the auction.
	parts := strings.Split(`1~信达证券~601059~15.56~15.56~0.00~0~0~0~0.00~0~0.00~0~0.00~0~0.00~0~0.00~0~0.00~0~0.00~0~0.00~0~0.00~0~0.00~0~~20261019090000~0.00~0.00~0.00~0.00~15.56/0/0~0~0~0.00~27.60~D`, "~")
	if !validTencentT0QuoteFields(parts, now) {
		t.Fatal("same-day explicit non-trading quote was discarded before suspension verification")
	}
	for _, tc := range []struct {
		name  string
		index int
		value string
	}{
		{"ordinary pre-auction quote", 40, ""},
		{"old trading day", 30, "20261016090000"},
		{"before morning refresh", 30, "20261019000000"},
		{"opened stock", 5, "15.56"},
		{"traded stock", 6, "1"},
		{"missing previous close", 4, "0"},
		{"invalid open", 5, "bad"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			altered := append([]string(nil), parts...)
			altered[tc.index] = tc.value
			if validTencentT0QuoteFields(altered, now) {
				t.Fatal("unverified quote accepted")
			}
		})
	}
	if validTencentT0QuoteFields(parts[:30], now) {
		t.Fatal("short quote accepted")
	}
	parts[40] = ""
	parts[30] = "20261019092500"
	parts[5] = "15.56"
	if !validTencentT0QuoteFields(parts, now) {
		t.Fatal("normal post-auction quote rejected")
	}
}

func TestExplicitNonTradingStatusCanFinishPrewarmBeforeAuction(t *testing.T) {
	date := "2026-10-19"
	now := time.Date(2026, 10, 19, 9, 0, 0, 0, chinaLocation())
	quote := t0Realtime{TradeDate: date, PrevClose: 15.56, NonTrading: true}
	if !isConfirmedSharedGobSuspension(quote, date, now) {
		t.Fatal("explicit non-trading status blocked morning preparation")
	}
	quote.NonTrading = false
	if isConfirmedSharedGobSuspension(quote, date, now) {
		t.Fatal("ordinary pre-auction zero quote excluded")
	}
}

func TestSharedGobPriorSessionSuspensionNeedsClosingEvidence(t *testing.T) {
	date, prior := "2026-10-20", "2026-10-19"
	quote := t0Realtime{Source: "tencent", TradeDate: prior, QuoteTime: "20261019161450", PrevClose: 10.87}
	bars := []dailyBar{{Date: "2026-10-16", Close: 10.87}}
	if !isConfirmedSharedGobPriorSessionSuspension(quote, bars, date, prior) {
		t.Fatal("verified previous closing zero-trade quote should permit overnight prewarm")
	}
	cases := []struct {
		name   string
		change func(*t0Realtime)
	}{
		{"wrong source", func(q *t0Realtime) { q.Source = "ths" }},
		{"older date", func(q *t0Realtime) { q.TradeDate = "2026-10-16" }},
		{"wrong stamp date", func(q *t0Realtime) { q.QuoteTime = "20261016161450" }},
		{"missing stamp", func(q *t0Realtime) { q.QuoteTime = "" }},
		{"malformed stamp", func(q *t0Realtime) { q.QuoteTime = "20261019259999" }},
		{"morning zero", func(q *t0Realtime) { q.QuoteTime = "20261019092500" }},
		{"opened", func(q *t0Realtime) { q.Open = 10.87 }},
		{"traded", func(q *t0Realtime) { q.Volume = 1 }},
		{"missing prior close", func(q *t0Realtime) { q.PrevClose = 0 }},
		{"different historical close", func(q *t0Realtime) { q.PrevClose = 11 }},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			q := quote
			tc.change(&q)
			if isConfirmedSharedGobPriorSessionSuspension(q, bars, date, prior) {
				t.Fatal("unverified missing history excluded")
			}
		})
	}
	if isConfirmedSharedGobPriorSessionSuspension(quote, []dailyBar{{Date: prior, Close: 10.87}}, date, prior) {
		t.Fatal("existing prior-session bar must not be excluded")
	}
	if isConfirmedSharedGobPriorSessionSuspension(quote, nil, date, prior) {
		t.Fatal("missing history is not evidence of suspension")
	}
	if isConfirmedSharedGobPriorSessionSuspension(quote, bars, prior, prior) {
		t.Fatal("today's quote must not stand in for the preceding session")
	}
	quote.NonTrading = true
	quote.QuoteTime = "20261019090000"
	if !isConfirmedSharedGobPriorSessionSuspension(quote, bars, date, prior) {
		t.Fatal("explicit dated non-trading status should remain usable overnight")
	}
}
