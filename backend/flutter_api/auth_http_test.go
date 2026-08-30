package flutter_api

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestRequireAuthRejectsMissingToken(t *testing.T) {
	service := newTestAuthService(t)
	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		WriteJSON(w, map[string]string{"status": "ok"})
	})
	req := httptest.NewRequest(http.MethodGet, "/api/news", nil)
	rec := httptest.NewRecorder()

	RequireAuth(service.AuthService, next).ServeHTTP(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d, want 401", rec.Code)
	}
	if !strings.Contains(rec.Body.String(), "UNAUTHENTICATED") {
		t.Fatalf("body = %s, want auth code", rec.Body.String())
	}
}

func TestRequireAuthReadsCaseInsensitiveBearerAndStoresPrincipal(t *testing.T) {
	service := newTestAuthService(t)
	session, err := service.Register(context.Background(), RegisterInput{
		Phone:    "13800000000",
		Password: "secret123",
		Nickname: "A",
		DeviceID: "device-a",
	})
	if err != nil {
		t.Fatalf("register: %v", err)
	}

	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		principal, ok := PrincipalFromContext(r.Context())
		if !ok {
			t.Fatal("principal missing from context")
		}
		WriteJSON(w, map[string]string{
			"userId":    principal.UserID,
			"sessionId": principal.SessionID,
			"deviceId":  principal.DeviceID,
		})
	})
	req := httptest.NewRequest(http.MethodGet, "/api/news", nil)
	req.Header.Set("Authorization", "bEaReR "+session.AccessToken)
	rec := httptest.NewRecorder()

	RequireAuth(service.AuthService, next).ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200, body = %s", rec.Code, rec.Body.String())
	}

	var got map[string]string
	if err := json.Unmarshal(rec.Body.Bytes(), &got); err != nil {
		t.Fatalf("decode body: %v", err)
	}
	if got["userId"] != session.User.ID {
		t.Fatalf("user id = %q, want %q", got["userId"], session.User.ID)
	}
	if got["deviceId"] != "device-a" {
		t.Fatalf("device id = %q, want %q", got["deviceId"], "device-a")
	}
	if got["sessionId"] == "" {
		t.Fatal("session id should not be empty")
	}
}

func TestRequireAuthAllowsOptionsRequests(t *testing.T) {
	service := newTestAuthService(t)
	called := false
	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		called = true
		w.WriteHeader(http.StatusNoContent)
	})
	req := httptest.NewRequest(http.MethodOptions, "/api/news", nil)
	rec := httptest.NewRecorder()

	RequireAuth(service.AuthService, next).ServeHTTP(rec, req)

	if !called {
		t.Fatal("next handler was not called for OPTIONS")
	}
	if rec.Code != http.StatusNoContent {
		t.Fatalf("status = %d, want 204", rec.Code)
	}
}

func TestAuthHTTPRegisterLoginMeAndProfile(t *testing.T) {
	service := newTestAuthService(t)
	handler := newHTTPHandler(service.AuthService)

	registerReq := httptest.NewRequest(http.MethodPost, "/api/auth/register", strings.NewReader(`{"phone":"13800000000","password":"secret123","nickname":"Alice","deviceId":"device-a"}`))
	registerReq.Header.Set("Content-Type", "application/json")
	registerRec := httptest.NewRecorder()
	handler.ServeHTTP(registerRec, registerReq)

	if registerRec.Code != http.StatusCreated {
		t.Fatalf("register status = %d, want 201, body = %s", registerRec.Code, registerRec.Body.String())
	}

	var registerBody AuthSessionResponse
	if err := json.Unmarshal(registerRec.Body.Bytes(), &registerBody); err != nil {
		t.Fatalf("decode register body: %v", err)
	}
	if registerBody.AccessToken != "token-register" {
		t.Fatalf("register access token = %q, want %q", registerBody.AccessToken, "token-register")
	}
	if registerBody.User.Nickname != "Alice" {
		t.Fatalf("register nickname = %q, want %q", registerBody.User.Nickname, "Alice")
	}

	loginReq := httptest.NewRequest(http.MethodPost, "/api/auth/login", strings.NewReader(`{"phone":"13800000000","password":"secret123","deviceId":"device-b"}`))
	loginReq.Header.Set("Content-Type", "application/json")
	loginRec := httptest.NewRecorder()
	handler.ServeHTTP(loginRec, loginReq)

	if loginRec.Code != http.StatusOK {
		t.Fatalf("login status = %d, want 200, body = %s", loginRec.Code, loginRec.Body.String())
	}

	var loginBody AuthSessionResponse
	if err := json.Unmarshal(loginRec.Body.Bytes(), &loginBody); err != nil {
		t.Fatalf("decode login body: %v", err)
	}
	if loginBody.AccessToken != "token-login" {
		t.Fatalf("login access token = %q, want %q", loginBody.AccessToken, "token-login")
	}

	meReq := httptest.NewRequest(http.MethodGet, "/api/auth/me", nil)
	meReq.Header.Set("Authorization", "Bearer "+loginBody.AccessToken)
	meRec := httptest.NewRecorder()
	handler.ServeHTTP(meRec, meReq)

	if meRec.Code != http.StatusOK {
		t.Fatalf("me status = %d, want 200, body = %s", meRec.Code, meRec.Body.String())
	}

	var meBody struct {
		User      PublicUser `json:"user"`
		ExpiresAt string     `json:"expiresAt"`
	}
	if err := json.Unmarshal(meRec.Body.Bytes(), &meBody); err != nil {
		t.Fatalf("decode me body: %v", err)
	}
	if meBody.User.ID != registerBody.User.ID {
		t.Fatalf("me user id = %q, want %q", meBody.User.ID, registerBody.User.ID)
	}
	if meBody.User.Phone != "13800000000" {
		t.Fatalf("me phone = %q, want original phone", meBody.User.Phone)
	}
	if meBody.ExpiresAt == "" {
		t.Fatal("me expiresAt should not be empty")
	}

	profileReq := httptest.NewRequest(http.MethodPatch, "/api/auth/profile", strings.NewReader(`{"nickname":"Bob"}`))
	profileReq.Header.Set("Content-Type", "application/json")
	profileReq.Header.Set("Authorization", "Bearer "+loginBody.AccessToken)
	profileRec := httptest.NewRecorder()
	handler.ServeHTTP(profileRec, profileReq)

	if profileRec.Code != http.StatusOK {
		t.Fatalf("profile status = %d, want 200, body = %s", profileRec.Code, profileRec.Body.String())
	}

	var profileBody PublicUser
	if err := json.Unmarshal(profileRec.Body.Bytes(), &profileBody); err != nil {
		t.Fatalf("decode profile body: %v", err)
	}
	if profileBody.Nickname != "Bob" {
		t.Fatalf("updated nickname = %q, want %q", profileBody.Nickname, "Bob")
	}
}

func TestAuthHTTPOldTokenReturnsSessionReplaced(t *testing.T) {
	service := newTestAuthService(t)
	handler := newHTTPHandler(service.AuthService)

	registerReq := httptest.NewRequest(http.MethodPost, "/api/auth/register", strings.NewReader(`{"phone":"13800000000","password":"secret123","nickname":"Alice","deviceId":"device-a"}`))
	registerReq.Header.Set("Content-Type", "application/json")
	registerRec := httptest.NewRecorder()
	handler.ServeHTTP(registerRec, registerReq)
	if registerRec.Code != http.StatusCreated {
		t.Fatalf("register status = %d, want 201", registerRec.Code)
	}

	loginReq := httptest.NewRequest(http.MethodPost, "/api/auth/login", strings.NewReader(`{"phone":"13800000000","password":"secret123","deviceId":"device-b"}`))
	loginReq.Header.Set("Content-Type", "application/json")
	loginRec := httptest.NewRecorder()
	handler.ServeHTTP(loginRec, loginReq)
	if loginRec.Code != http.StatusOK {
		t.Fatalf("login status = %d, want 200", loginRec.Code)
	}

	meReq := httptest.NewRequest(http.MethodGet, "/api/auth/me", nil)
	meReq.Header.Set("Authorization", "Bearer token-register")
	meRec := httptest.NewRecorder()
	handler.ServeHTTP(meRec, meReq)

	if meRec.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d, want 401, body = %s", meRec.Code, meRec.Body.String())
	}
	if !strings.Contains(meRec.Body.String(), `"code":"SESSION_REPLACED"`) {
		t.Fatalf("body = %s, want SESSION_REPLACED", meRec.Body.String())
	}
}

func TestAuthHTTPHealthIsPublicAndBusinessRoutesAreProtected(t *testing.T) {
	service := newTestAuthService(t)
	handler := newHTTPHandler(service.AuthService)

	healthReq := httptest.NewRequest(http.MethodGet, "/api/health", nil)
	healthRec := httptest.NewRecorder()
	handler.ServeHTTP(healthRec, healthReq)
	if healthRec.Code != http.StatusOK {
		t.Fatalf("health status = %d, want 200, body = %s", healthRec.Code, healthRec.Body.String())
	}

	newsReq := httptest.NewRequest(http.MethodGet, "/api/news", nil)
	newsRec := httptest.NewRecorder()
	handler.ServeHTTP(newsRec, newsReq)
	if newsRec.Code != http.StatusUnauthorized {
		t.Fatalf("news status = %d, want 401, body = %s", newsRec.Code, newsRec.Body.String())
	}

	wsReq := httptest.NewRequest(http.MethodGet, "/ws", nil)
	wsRec := httptest.NewRecorder()
	handler.ServeHTTP(wsRec, wsReq)
	if wsRec.Code != http.StatusUnauthorized {
		t.Fatalf("ws status = %d, want 401, body = %s", wsRec.Code, wsRec.Body.String())
	}
}
