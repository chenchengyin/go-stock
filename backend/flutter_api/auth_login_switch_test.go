package flutter_api

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func TestAdminSystemSettingsDefaultsToOffAndRequiresAdmin(t *testing.T) {
	dao := newAuthTestDB(t)
	if err := MigrateAuthTables(dao); err != nil {
		t.Fatal(err)
	}
	createAdminForTest(t, dao)
	handler := NewAdminHTTPHandler(NewAdminService(dao, nil), NewModuleService(dao))
	for _, method := range []string{http.MethodGet, http.MethodPatch} {
		req := httptest.NewRequest(method, "/api/admin/settings", strings.NewReader(`{"singleDeviceLogin":true}`))
		rec := httptest.NewRecorder()
		handler.ServeHTTP(rec, req)
		assertAdminUnauthenticated(t, rec)
	}
	regular := registerAndLoginAuthTest(t, &testAuthService{AuthService: NewAuthService(dao, nil)}, "13800000000", "A", "device-a")
	regularCookie := &http.Cookie{Name: adminSessionCookieName, Value: regular.AccessToken}
	assertAdminUnauthenticated(t, settingsRequest(t, handler, regularCookie, http.MethodPatch, `{"singleDeviceLogin":true}`))
	cookie := loginAdminCookieForTest(t, handler)
	response := settingsRequest(t, handler, cookie, http.MethodGet, "")
	assertSingleDeviceSetting(t, response, false)
	for _, body := range []string{`{}`, `{"singleDeviceLogin":null}`, `{"singleDeviceLogin":"false"}`} {
		rec := settingsRequest(t, handler, cookie, http.MethodPatch, body)
		if rec.Code != http.StatusBadRequest {
			t.Fatalf("invalid body %s: status %d, %s", body, rec.Code, rec.Body.String())
		}
	}
	req := httptest.NewRequest(http.MethodPatch, "/api/admin/settings", strings.NewReader(`{"singleDeviceLogin":true}`))
	req.AddCookie(cookie)
	req.Header.Set("Origin", "https://untrusted.example")
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	if rec.Code != http.StatusForbidden {
		t.Fatalf("cross-origin update status = %d", rec.Code)
	}
}

func TestAdminSystemSettingsSwitchAppliesWithoutRestart(t *testing.T) {
	auth := newTestAuthService(t)
	createAdminForTest(t, auth.dao)
	var notified []string
	admin := NewAdminService(auth.dao, func(_ string, ids []string) { notified = append(notified, ids...) })
	handler := NewAdminHTTPHandler(admin, NewModuleService(auth.dao))
	cookie := loginAdminCookieForTest(t, handler)
	assertSingleDeviceSetting(t, settingsRequest(t, handler, cookie, http.MethodPatch, `{"singleDeviceLogin":false}`), false)
	first := registerAndLoginAuthTest(t, auth, "13800000000", "A", "device-a")
	auth.nowValue = auth.nowValue.Add(time.Second)
	second, err := auth.Login(context.Background(), LoginInput{Phone: "13800000000", Password: "secret123", DeviceID: "device-b"})
	if err != nil {
		t.Fatal(err)
	}
	firstPrincipal, err := auth.Authenticate(context.Background(), first.AccessToken)
	if err != nil {
		t.Fatalf("first device with switch off: %v", err)
	}
	assertActiveSessionCount(t, auth, first.User.ID, 2)
	assertSingleDeviceSetting(t, settingsRequest(t, handler, cookie, http.MethodPatch, `{"singleDeviceLogin":true}`), true)
	if len(notified) != 1 || notified[0] != firstPrincipal.SessionID {
		t.Fatalf("revocation notifications = %v", notified)
	}
	if _, err := auth.Authenticate(context.Background(), first.AccessToken); !IsAuthCode(err, "SESSION_REPLACED") {
		t.Fatalf("older session after enabling: %v", err)
	}
	if _, err := auth.Authenticate(context.Background(), second.AccessToken); err != nil {
		t.Fatalf("latest session after enabling: %v", err)
	}
	if err := MigrateAuthTables(auth.dao); err != nil {
		t.Fatal(err)
	}
	assertSingleDeviceSetting(t, settingsRequest(t, handler, cookie, http.MethodGet, ""), true)
	third, err := auth.Login(context.Background(), LoginInput{Phone: "13800000000", Password: "secret123", DeviceID: "device-c"})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := auth.Authenticate(context.Background(), second.AccessToken); !IsAuthCode(err, "SESSION_REPLACED") {
		t.Fatalf("second session after new login: %v", err)
	}
	assertSingleDeviceSetting(t, settingsRequest(t, handler, cookie, http.MethodPatch, `{"singleDeviceLogin":false}`), false)
	if err := MigrateAuthTables(auth.dao); err != nil {
		t.Fatal(err)
	}
	assertSingleDeviceSetting(t, settingsRequest(t, handler, cookie, http.MethodGet, ""), false)
	if _, err := auth.Login(context.Background(), LoginInput{Phone: "13800000000", Password: "secret123", DeviceID: "device-d"}); err != nil {
		t.Fatal(err)
	}
	if _, err := auth.Authenticate(context.Background(), third.AccessToken); err != nil {
		t.Fatalf("third device after disabling: %v", err)
	}
	assertActiveSessionCount(t, auth, first.User.ID, 2)
}

func settingsRequest(t *testing.T, handler http.Handler, cookie *http.Cookie, method, body string) *httptest.ResponseRecorder {
	t.Helper()
	req := httptest.NewRequest(method, "/api/admin/settings", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.AddCookie(cookie)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	return rec
}

func assertSingleDeviceSetting(t *testing.T, rec *httptest.ResponseRecorder, want bool) {
	t.Helper()
	if rec.Code != http.StatusOK {
		t.Fatalf("settings status = %d: %s", rec.Code, rec.Body.String())
	}
	var body map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatal(err)
	}
	if body["singleDeviceLogin"] != want {
		t.Fatalf("settings = %v, want singleDeviceLogin=%v", body, want)
	}
}
