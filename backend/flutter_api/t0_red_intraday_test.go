package flutter_api

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestRedT1IntradayRouteDoesNotRequireLoginOrModuleAccess(t *testing.T) {
	auth := newTestAuthService(t)
	handler := newHTTPHandler(auth.AuthService)
	req := httptest.NewRequest(http.MethodGet,
		"/api/t0-red/t1-intraday?date=2020-01-02&codes=600000.XSHG&module_code=radar.red_shared_gob_strategy&session_date=2999-01-01", nil)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	if rec.Code != http.StatusBadRequest || !strings.Contains(rec.Body.String(), "session_date") {
		t.Fatalf("unauthenticated request did not reach session validation: %d %s", rec.Code, rec.Body.String())
	}
}

func TestResolveRedT1SessionNavigatesTradingDaysWithoutBorrowingPrices(t *testing.T) {
	trading := map[string]bool{
		"2026-09-30": true, "2026-10-08": true,
		"2026-10-09": true, "2026-10-12": true,
	}
	prev := func(date string) (string, bool) {
		var found string
		for day := range trading {
			if day < date && day > found {
				found = day
			}
		}
		return found, found != ""
	}
	next := func(date, through string) (string, bool) {
		var found string
		for day := range trading {
			if day > date && day <= through && (found == "" || day < found) {
				found = day
			}
		}
		return found, found != ""
	}
	for _, tc := range []struct {
		name, requested, today, want, wantPrev, wantNext string
		ok                                               bool
	}{
		{"default T+1", "", "2026-10-12", "2026-10-09", "2026-10-08", "2026-10-12", true},
		{"previous skips holidays", "2026-09-30", "2026-10-12", "2026-09-30", "", "2026-10-08", true},
		{"next skips weekend", "2026-10-12", "2026-10-12", "2026-10-12", "2026-10-09", "", true},
		{"future forbidden", "2026-10-12", "2026-10-09", "", "", "", false},
		{"non trading day forbidden", "2026-10-10", "2026-10-12", "", "", "", false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			got, previous, following, ok := resolveRedT1Session("2026-10-08", tc.requested, tc.today, func(date string) bool { return trading[date] }, next, prev)
			if got != tc.want || previous != tc.wantPrev || following != tc.wantNext || ok != tc.ok {
				t.Fatalf("got (%q,%q,%q,%v), want (%q,%q,%q,%v)", got, previous, following, ok, tc.want, tc.wantPrev, tc.wantNext, tc.ok)
			}
		})
	}
}
