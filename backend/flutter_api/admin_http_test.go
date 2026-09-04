package flutter_api

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"gorm.io/gorm"
)

func TestAdminHTTPLoginSetsSecureCookieWithoutSerializingRawToken(t *testing.T) {
	auth := newTestAuthService(t)
	createAdminForTest(t, auth.dao)
	handler := NewAdminHTTPHandler(NewAdminService(auth.dao, nil), NewModuleService(auth.dao))

	req := httptest.NewRequest(http.MethodPost, "https://example.com/api/admin/login", strings.NewReader(`{"username":"13900000000","password":"secret123"}`))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("login status = %d, body = %s", rec.Code, rec.Body.String())
	}
	if strings.Contains(rec.Body.String(), "accessToken") || strings.Contains(rec.Body.String(), "admin-token") {
		t.Fatalf("login response exposed token: %s", rec.Body.String())
	}
	var body AdminSessionResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil || body.ExpiresAt.IsZero() {
		t.Fatalf("login response = %+v, err = %v", body, err)
	}
	cookies := rec.Result().Cookies()
	if len(cookies) != 1 {
		t.Fatalf("cookies = %v, want one", cookies)
	}
	cookie := cookies[0]
	if cookie.Name != adminSessionCookieName || cookie.Path != "/" || !cookie.HttpOnly || !cookie.Secure || cookie.SameSite != http.SameSiteLaxMode || cookie.Expires.Unix() != body.ExpiresAt.Unix() {
		t.Fatalf("cookie = %+v", cookie)
	}
}

func TestAdminHTTPRejectsUnauthenticatedAndRegularUserSessions(t *testing.T) {
	auth := newTestAuthService(t)
	handler := NewAdminHTTPHandler(NewAdminService(auth.dao, nil), NewModuleService(auth.dao))

	req := httptest.NewRequest(http.MethodGet, "/api/admin/users", nil)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	assertAdminUnauthenticated(t, rec)

	userSession := authHTTPRegisterUser(t, auth, "13800000000", "Alice", "device-a")
	req = httptest.NewRequest(http.MethodGet, "/api/admin/users", nil)
	req.AddCookie(&http.Cookie{Name: adminSessionCookieName, Value: userSession.AccessToken})
	rec = httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	assertAdminUnauthenticated(t, rec)
}

func TestAdminHTTPListsUsersAndFiltersKeywordWithSessionCookie(t *testing.T) {
	auth := newTestAuthService(t)
	createAdminForTest(t, auth.dao)
	handler := NewAdminHTTPHandler(NewAdminService(auth.dao, nil), NewModuleService(auth.dao))
	createUserForAdminTest(t, auth, "13800000000", "Alice")
	createUserForAdminTest(t, auth, "13600000000", "Bob")
	if err := auth.dao.Create(&AuthUser{ID: "other-admin", Phone: "13700000000", PasswordHash: "unused", Nickname: "Internal", Role: authRoleAdmin, Status: authStatusActive}).Error; err != nil {
		t.Fatalf("create second admin: %v", err)
	}
	cookie := loginAdminCookieForTest(t, handler)

	req := httptest.NewRequest(http.MethodGet, "/api/admin/users?keyword=Alice", nil)
	req.AddCookie(cookie)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", rec.Code, rec.Body.String())
	}
	if strings.Contains(rec.Body.String(), "passwordHash") || strings.Contains(rec.Body.String(), "deviceId") {
		t.Fatalf("body contains private fields: %s", rec.Body.String())
	}
	var body AdminUserList
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode list: %v", err)
	}
	if body.Total != 1 || len(body.Items) != 1 || body.Items[0].Nickname != "Alice" {
		t.Fatalf("list body = %+v, want one Alice", body)
	}
}

func TestAdminHTTPMeReturnsProfileAndSessionExpiry(t *testing.T) {
	auth := newTestAuthService(t)
	createAdminForTest(t, auth.dao)
	handler := NewAdminHTTPHandler(NewAdminService(auth.dao, nil), NewModuleService(auth.dao))
	cookie := loginAdminCookieForTest(t, handler)

	req := httptest.NewRequest(http.MethodGet, "/api/admin/me", nil)
	req.AddCookie(cookie)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("me status = %d, body = %s", rec.Code, rec.Body.String())
	}
	var body struct {
		User      AdminUser `json:"user"`
		ExpiresAt string    `json:"expiresAt"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode me: %v", err)
	}
	if body.User.Phone != "13900000000" || body.User.Role != authRoleAdmin || body.ExpiresAt == "" {
		t.Fatalf("me body = %#v", body)
	}
}

func TestAdminHTTPRejectsCrossOriginMutationsAndClearsCookieOnLogout(t *testing.T) {
	auth := newTestAuthService(t)
	createAdminForTest(t, auth.dao)
	handler := NewAdminHTTPHandler(NewAdminService(auth.dao, nil), NewModuleService(auth.dao))
	cookie := loginAdminCookieForTest(t, handler)

	req := httptest.NewRequest(http.MethodPost, "http://example.com/api/admin/logout", nil)
	req.Header.Set("Origin", "https://evil.example")
	req.AddCookie(cookie)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	if rec.Code != http.StatusForbidden {
		t.Fatalf("cross-origin logout status = %d, body = %s", rec.Code, rec.Body.String())
	}

	req = httptest.NewRequest(http.MethodPost, "http://example.com/api/admin/logout", nil)
	req.Header.Set("Origin", "http://example.com")
	req.AddCookie(cookie)
	rec = httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("logout status = %d, body = %s", rec.Code, rec.Body.String())
	}
	cookies := rec.Result().Cookies()
	if len(cookies) != 1 || cookies[0].Name != adminSessionCookieName || cookies[0].MaxAge != -1 {
		t.Fatalf("logout cookies = %+v", cookies)
	}

	req = httptest.NewRequest(http.MethodGet, "http://example.com/api/admin/users", nil)
	req.AddCookie(cookie)
	rec = httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	assertAdminUnauthenticated(t, rec)
}

func TestAdminHTTPOriginRequiresSchemeAndUsesConfiguredPluralOrigins(t *testing.T) {
	t.Setenv("GO_STOCK_ADMIN_DEV_ORIGIN", "http://example.com")
	t.Setenv("GO_STOCK_ADMIN_DEV_ORIGINS", "http://localhost:5173, http://127.0.0.1:5173")

	request := httptest.NewRequest(http.MethodPatch, "https://example.com/api/admin/users/id/status", nil)
	request.Header.Set("Origin", "http://example.com")
	if adminRequestOriginAllowed(request) {
		t.Fatal("http origin must not match an https request")
	}

	request.Header.Set("Origin", "https://example.com")
	if !adminRequestOriginAllowed(request) {
		t.Fatal("same https origin should be allowed")
	}
	request.Header.Set("Origin", "https://example.com:443")
	if !adminRequestOriginAllowed(request) {
		t.Fatal("same https origin with effective port should be allowed")
	}
	request.Header.Set("Origin", "https://example.com:8443")
	if adminRequestOriginAllowed(request) {
		t.Fatal("different https port must not match")
	}

	request.Header.Set("Origin", "http://localhost:5173")
	if !adminRequestOriginAllowed(request) {
		t.Fatal("configured plural development origin should be allowed")
	}
}

func createAdminForTest(t *testing.T, dao *gorm.DB) {
	t.Helper()
	if err := CreateAdmin(context.Background(), dao, AdminInitInput{
		Account: "13900000000", Nickname: "管理员", Password: "secret123",
	}); err != nil {
		t.Fatalf("create admin: %v", err)
	}
}

func loginAdminCookieForTest(t *testing.T, handler http.Handler) *http.Cookie {
	t.Helper()
	req := httptest.NewRequest(http.MethodPost, "/api/admin/login", strings.NewReader(`{"username":"13900000000","password":"secret123"}`))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("admin login status = %d, body = %s", rec.Code, rec.Body.String())
	}
	cookies := rec.Result().Cookies()
	if len(cookies) != 1 || cookies[0].Value == "" {
		t.Fatalf("admin login cookies = %+v", cookies)
	}
	return cookies[0]
}

func assertAdminUnauthenticated(t *testing.T, rec *httptest.ResponseRecorder) {
	t.Helper()
	if rec.Code != http.StatusUnauthorized || !strings.Contains(rec.Body.String(), "ADMIN_UNAUTHENTICATED") {
		t.Fatalf("unauthenticated response = %d %s", rec.Code, rec.Body.String())
	}
}

func createUserForAdminTest(t *testing.T, auth *testAuthService, phone, nickname string) *RegisterResponse {
	t.Helper()
	result, err := auth.Register(context.Background(), RegisterInput{
		Phone: phone, Password: "secret123", Nickname: nickname, DeviceID: phone + "-device",
	})
	if err != nil {
		t.Fatalf("register %s: %v", phone, err)
	}
	return result
}

func loginAdminForTest(t *testing.T, handler http.Handler) string {
	t.Helper()
	return loginAdminCookieForTest(t, handler).Value
}

func setAdminUserStatus(t *testing.T, handler http.Handler, adminToken, userID, status string) {
	t.Helper()
	req := httptest.NewRequest(http.MethodPatch, "/api/admin/users/"+userID+"/status", strings.NewReader(`{"status":"`+status+`"}`))
	req.Header.Set("Content-Type", "application/json")
	req.AddCookie(&http.Cookie{Name: adminSessionCookieName, Value: adminToken})
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("set user status = %d, body = %s", rec.Code, rec.Body.String())
	}
}
