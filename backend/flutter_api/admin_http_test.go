package flutter_api

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestAdminHTTPRejectsUnauthenticatedAndRegularUserTokens(t *testing.T) {
	auth := newTestAuthService(t)
	handler := newHTTPHandler(auth.AuthService)

	request := httptest.NewRequest(http.MethodGet, "/api/admin/users", nil)
	recorder := httptest.NewRecorder()
	handler.ServeHTTP(recorder, request)

	if recorder.Code != http.StatusUnauthorized {
		t.Fatalf("missing token status = %d, want 401", recorder.Code)
	}
	if !strings.Contains(recorder.Body.String(), "ADMIN_UNAUTHENTICATED") {
		t.Fatalf("missing token body = %s, want ADMIN_UNAUTHENTICATED", recorder.Body.String())
	}

	userSession, err := auth.Register(context.Background(), RegisterInput{
		Phone: "13800000000", Password: "secret123", Nickname: "Alice", DeviceID: "device-a",
	})
	if err != nil {
		t.Fatalf("register user: %v", err)
	}
	request = httptest.NewRequest(http.MethodGet, "/api/admin/users", nil)
	request.Header.Set("Authorization", "Bearer "+userSession.AccessToken)
	recorder = httptest.NewRecorder()
	handler.ServeHTTP(recorder, request)

	if recorder.Code != http.StatusUnauthorized {
		t.Fatalf("regular user token status = %d, want 401", recorder.Code)
	}
}

func TestAdminHTTPListsUsersAndFiltersKeyword(t *testing.T) {
	auth := newTestAuthService(t)
	handler := newHTTPHandler(auth.AuthService)

	createUserForAdminTest(t, auth, "13800000000", "Alice")
	createUserForAdminTest(t, auth, "13900000000", "Bob")
	if err := auth.dao.Create(&AuthUser{
		ID: "admin-record", Phone: "13700000000", PasswordHash: "unused", Nickname: "Internal", Role: "admin", Status: authStatusActive,
	}).Error; err != nil {
		t.Fatalf("create non-user role: %v", err)
	}
	token := loginAdminForTest(t, handler)

	request := httptest.NewRequest(http.MethodGet, "/api/admin/users?keyword=Alice", nil)
	request.Header.Set("Authorization", "Bearer "+token)
	recorder := httptest.NewRecorder()
	handler.ServeHTTP(recorder, request)

	if recorder.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", recorder.Code, recorder.Body.String())
	}
	if !strings.Contains(recorder.Body.String(), "Alice") || strings.Contains(recorder.Body.String(), "Bob") {
		t.Fatalf("body = %s, want only Alice", recorder.Body.String())
	}
	if strings.Contains(recorder.Body.String(), "passwordHash") || strings.Contains(recorder.Body.String(), "deviceId") {
		t.Fatalf("body contains private fields: %s", recorder.Body.String())
	}

	var body AdminUserList
	if err := json.Unmarshal(recorder.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode list: %v", err)
	}
	if body.Total != 1 || len(body.Items) != 1 || body.Items[0].Nickname != "Alice" {
		t.Fatalf("list body = %+v, want one Alice", body)
	}

	request = httptest.NewRequest(http.MethodGet, "/api/admin/users?keyword=13900000000", nil)
	request.Header.Set("Authorization", "Bearer "+token)
	recorder = httptest.NewRecorder()
	handler.ServeHTTP(recorder, request)
	if recorder.Code != http.StatusOK || !strings.Contains(recorder.Body.String(), "Bob") || strings.Contains(recorder.Body.String(), "Alice") || strings.Contains(recorder.Body.String(), "Internal") {
		t.Fatalf("phone-filtered body = %d %s, want only Bob", recorder.Code, recorder.Body.String())
	}
}

func TestAdminHTTPDisablesUserAndRevokesSession(t *testing.T) {
	auth := newTestAuthService(t)
	handler := newHTTPHandler(auth.AuthService)
	userSession, err := auth.Register(context.Background(), RegisterInput{
		Phone: "13800000000", Password: "secret123", Nickname: "Alice", DeviceID: "device-a",
	})
	if err != nil {
		t.Fatalf("register user: %v", err)
	}
	token := loginAdminForTest(t, handler)

	request := httptest.NewRequest(http.MethodPatch, "/api/admin/users/"+userSession.User.ID+"/status", strings.NewReader(`{"status":"disabled"}`))
	request.Header.Set("Content-Type", "application/json")
	request.Header.Set("Authorization", "Bearer "+token)
	recorder := httptest.NewRecorder()
	handler.ServeHTTP(recorder, request)

	if recorder.Code != http.StatusOK {
		t.Fatalf("status update status = %d, body = %s", recorder.Code, recorder.Body.String())
	}
	var user AdminUser
	if err := json.Unmarshal(recorder.Body.Bytes(), &user); err != nil {
		t.Fatalf("decode updated user: %v", err)
	}
	if user.Status != authStatusDisabled {
		t.Fatalf("user status = %q, want disabled", user.Status)
	}
	if _, err := auth.Authenticate(context.Background(), userSession.AccessToken); err == nil {
		t.Fatal("disabled user's existing session should be rejected")
	}
	if _, err := auth.Login(context.Background(), LoginInput{
		Phone: "13800000000", Password: "secret123", DeviceID: "device-b",
	}); !IsAuthCode(err, "ACCOUNT_DISABLED") {
		t.Fatalf("disabled user login err = %v, want ACCOUNT_DISABLED", err)
	}
}

func TestAdminHTTPEnablesUserAndAllowsLoginAgain(t *testing.T) {
	auth := newTestAuthService(t)
	handler := newHTTPHandler(auth.AuthService)
	userSession, err := auth.Register(context.Background(), RegisterInput{
		Phone: "13800000000", Password: "secret123", Nickname: "Alice", DeviceID: "device-a",
	})
	if err != nil {
		t.Fatalf("register user: %v", err)
	}
	adminToken := loginAdminForTest(t, handler)
	setAdminUserStatus(t, handler, adminToken, userSession.User.ID, authStatusDisabled)

	request := httptest.NewRequest(http.MethodPatch, "/api/admin/users/"+userSession.User.ID+"/status", strings.NewReader(`{"status":"active"}`))
	request.Header.Set("Content-Type", "application/json")
	request.Header.Set("Authorization", "Bearer "+adminToken)
	recorder := httptest.NewRecorder()
	handler.ServeHTTP(recorder, request)
	if recorder.Code != http.StatusOK {
		t.Fatalf("enable status = %d, body = %s", recorder.Code, recorder.Body.String())
	}

	if _, err := auth.Login(context.Background(), LoginInput{
		Phone: "13800000000", Password: "secret123", DeviceID: "device-b",
	}); err != nil {
		t.Fatalf("enabled user login: %v", err)
	}
}

func TestAdminHTTPRejectsUnknownUserAndInvalidStatus(t *testing.T) {
	auth := newTestAuthService(t)
	handler := newHTTPHandler(auth.AuthService)
	token := loginAdminForTest(t, handler)

	request := httptest.NewRequest(http.MethodPatch, "/api/admin/users/missing/status", strings.NewReader(`{"status":"disabled"}`))
	request.Header.Set("Content-Type", "application/json")
	request.Header.Set("Authorization", "Bearer "+token)
	recorder := httptest.NewRecorder()
	handler.ServeHTTP(recorder, request)
	if recorder.Code != http.StatusNotFound || !strings.Contains(recorder.Body.String(), "USER_NOT_FOUND") {
		t.Fatalf("unknown user response = %d %s, want 404 USER_NOT_FOUND", recorder.Code, recorder.Body.String())
	}

	request = httptest.NewRequest(http.MethodPatch, "/api/admin/users/missing/status", strings.NewReader(`{"status":"removed"}`))
	request.Header.Set("Content-Type", "application/json")
	request.Header.Set("Authorization", "Bearer "+token)
	recorder = httptest.NewRecorder()
	handler.ServeHTTP(recorder, request)
	if recorder.Code != http.StatusBadRequest || !strings.Contains(recorder.Body.String(), "INVALID_ARGUMENT") {
		t.Fatalf("invalid status response = %d %s, want 400 INVALID_ARGUMENT", recorder.Code, recorder.Body.String())
	}
}

func createUserForAdminTest(t *testing.T, auth *testAuthService, phone, nickname string) *AuthSessionResponse {
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

	request := httptest.NewRequest(http.MethodPost, "/api/admin/login", strings.NewReader(`{"username":"admin","password":"admin"}`))
	request.Header.Set("Content-Type", "application/json")
	recorder := httptest.NewRecorder()
	handler.ServeHTTP(recorder, request)
	if recorder.Code != http.StatusOK {
		t.Fatalf("admin login status = %d, body = %s", recorder.Code, recorder.Body.String())
	}

	var response AdminSessionResponse
	if err := json.Unmarshal(recorder.Body.Bytes(), &response); err != nil {
		t.Fatalf("decode admin login: %v", err)
	}
	if response.AccessToken == "" {
		t.Fatal("admin login token is empty")
	}
	return response.AccessToken
}

func setAdminUserStatus(t *testing.T, handler http.Handler, adminToken, userID, status string) {
	t.Helper()

	request := httptest.NewRequest(http.MethodPatch, "/api/admin/users/"+userID+"/status", strings.NewReader(`{"status":"`+status+`"}`))
	request.Header.Set("Content-Type", "application/json")
	request.Header.Set("Authorization", "Bearer "+adminToken)
	recorder := httptest.NewRecorder()
	handler.ServeHTTP(recorder, request)
	if recorder.Code != http.StatusOK {
		t.Fatalf("set user status = %d, body = %s", recorder.Code, recorder.Body.String())
	}
}
