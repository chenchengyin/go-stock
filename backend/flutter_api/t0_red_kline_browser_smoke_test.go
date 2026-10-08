package flutter_api

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// Opt-in integration check using a test-only auth database and read-only local
// historical snapshots. The real authenticated K-line route and Flutter web
// static handler are used, without starting collectors or production DB work.
func TestRedKlineIsolatedHTTPIntegration(t *testing.T) {
	if os.Getenv("GO_STOCK_KLINE_HTTP_SMOKE") != "1" {
		t.Skip("opt-in local HTTP and Flutter asset smoke")
	}
	webRoot := os.Getenv("GO_STOCK_WEB_DIR")
	if _, err := os.Stat(filepath.Join(webRoot, "index.html")); err != nil {
		t.Fatalf("missing Flutter web build: %v", err)
	}
	cacheRoot, err := filepath.Abs(filepath.Join("..", "data", "cache"))
	if err != nil {
		t.Fatal(err)
	}
	oldCacheRoot := t0CacheRootPath
	t0CacheRootPath = cacheRoot
	defer func() { t0CacheRootPath = oldCacheRoot }()

	auth := newTestAuthService(t)
	auth.nowValue = time.Now()
	useGlobalTestDB(t, auth)
	user := authHTTPRegisterUser(t, auth, "13800998877", "Kline HTTP smoke", "smoke-device")
	modules := NewModuleService(auth.dao)
	if err := modules.ReplaceUserAccess(context.Background(), "smoke-admin", []string{user.User.ID}, []string{redT0StrategyModuleCode}); err != nil {
		t.Fatal(err)
	}
	realHandler := newHTTPHandler(auth.AuthService)
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/api/t0-selection" {
			if r.URL.Query().Get("list_dates") == "1" {
				WriteJSON(w, map[string]any{"dates": []string{"2026-09-30", "2026-09-16", "2024-01-11"}})
				return
			}
			date := r.URL.Query().Get("date")
			if date == "" {
				date = "2026-09-16"
			}
			raw, err := os.ReadFile(filepath.Join(cacheRoot, "t0", "selection", "t0_red_gob_"+date+".json"))
			if err != nil {
				WriteJSONStatus(w, http.StatusNotFound, map[string]string{"error": "snapshot unavailable"})
				return
			}
			var archive struct {
				Results []json.RawMessage `json:"results"`
			}
			if json.Unmarshal(raw, &archive) != nil {
				WriteJSONStatus(w, http.StatusInternalServerError, map[string]string{"error": "bad snapshot"})
				return
			}
			WriteJSON(w, map[string]any{"date": date, "display_date": date, "historical": true, "archived": true, "results": archive.Results})
			return
		}
		realHandler.ServeHTTP(w, r)
	})
	server := httptest.NewServer(handler)
	defer server.Close()
	client := &http.Client{Timeout: 35 * time.Second}
	static, err := client.Get(server.URL + "/")
	if err != nil {
		t.Fatal(err)
	}
	staticBody, _ := io.ReadAll(io.LimitReader(static.Body, 5000))
	static.Body.Close()
	if static.StatusCode != http.StatusOK || !bytes.Contains(staticBody, []byte("flutter_bootstrap")) {
		t.Fatalf("Flutter static page: %d %q", static.StatusCode, staticBody[:min(len(staticBody), 200)])
	}
	loginBody, _ := json.Marshal(map[string]string{"phone": "13800998877", "password": "secret123", "deviceId": "smoke-browser"})
	login, err := client.Post(server.URL+"/api/auth/login", "application/json", bytes.NewReader(loginBody))
	if err != nil {
		t.Fatal(err)
	}
	var session AuthSessionResponse
	if err := json.NewDecoder(login.Body).Decode(&session); err != nil {
		t.Fatal(err)
	}
	login.Body.Close()
	if login.StatusCode != http.StatusOK || session.AccessToken == "" {
		t.Fatalf("login status %d", login.StatusCode)
	}
	for _, sample := range []struct {
		date, code    string
		before, after int
	}{
		{"2026-09-16", "000823.XSHE", 60, 5},
		{"2026-09-30", "601091.XSHG", 8, 1},
		{"2024-01-11", "603595.XSHG", 60, 5},
	} {
		request, _ := http.NewRequest(http.MethodGet, server.URL+"/api/t0-red/kline-window?date="+sample.date+"&codes="+sample.code, nil)
		request.Header.Set("Authorization", "Bearer "+session.AccessToken)
		response, err := client.Do(request)
		if err != nil {
			t.Fatal(err)
		}
		var body struct {
			Items map[string]redKlineWindow `json:"items"`
		}
		err = json.NewDecoder(response.Body).Decode(&body)
		response.Body.Close()
		if err != nil {
			t.Fatal(err)
		}
		window := body.Items[sample.code]
		if response.StatusCode != http.StatusOK || window.BeforeCount != sample.before || window.AfterCount != sample.after || window.AnchorIndex != sample.before || len(window.Bars) <= window.AnchorIndex || window.AnchorIndex < 0 {
			t.Fatalf("%s %s: HTTP %d window=%+v", sample.date, sample.code, response.StatusCode, window)
		}
		if window.Bars[window.AnchorIndex].Date != sample.date {
			t.Fatalf("%s %s: wrong anchor %+v", sample.date, sample.code, window.Bars[window.AnchorIndex])
		}
		if sample.before >= 60 {
			for _, period := range []string{"5", "10", "20", "60"} {
				if window.Bars[window.AnchorIndex].MA[period] <= 0 {
					t.Fatalf("%s %s: MA%s missing at selected date", sample.date, sample.code, period)
				}
			}
		} else if window.Bars[window.AnchorIndex].MA["60"] != 0 {
			t.Fatalf("%s %s: IPO must not invent MA60", sample.date, sample.code)
		}
		t.Logf("%s %s: %d bars source=%s", sample.date, sample.code, len(window.Bars), window.Source)
	}
	list, err := client.Get(server.URL + "/api/t0-selection?list_dates=1&module_code=radar.red_strategy")
	if err != nil {
		t.Fatal(err)
	}
	data, _ := io.ReadAll(list.Body)
	list.Body.Close()
	if !strings.Contains(string(data), "2026-09-16") {
		t.Fatalf("date selector fixture: %s", data)
	}
}
