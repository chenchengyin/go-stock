package flutter_api

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"go-stock/backend/data"
	"go-stock/backend/db"
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

func TestAuthHTTPServesUnauthenticatedFlutterStaticResources(t *testing.T) {
	webRoot := t.TempDir()
	if err := os.WriteFile(filepath.Join(webRoot, "index.html"), []byte("login shell"), 0644); err != nil {
		t.Fatalf("write index: %v", err)
	}
	t.Setenv("GO_STOCK_WEB_DIR", webRoot)

	service := newTestAuthService(t)
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	rec := httptest.NewRecorder()

	newHTTPHandler(service.AuthService).ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("static status = %d, want 200, body = %s", rec.Code, rec.Body.String())
	}
	if rec.Body.String() != "login shell" {
		t.Fatalf("static body = %q, want login shell", rec.Body.String())
	}
}

func TestAuthHTTPUnauthenticatedProtectedResponseIncludesCORSHeaders(t *testing.T) {
	service := newTestAuthService(t)
	req := httptest.NewRequest(http.MethodGet, "/api/news", nil)
	req.Header.Set("Origin", "https://app.example")
	rec := httptest.NewRecorder()

	newHTTPHandler(service.AuthService).ServeHTTP(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d, want 401, body = %s", rec.Code, rec.Body.String())
	}
	if got := rec.Header().Get("Access-Control-Allow-Origin"); got != "*" {
		t.Fatalf("allow-origin = %q, want %q", got, "*")
	}
}

func useGlobalTestDB(t *testing.T, service *testAuthService) {
	t.Helper()

	old := db.Dao
	db.Dao = service.dao
	t.Cleanup(func() {
		db.Dao = old
	})
}

func authHTTPRegisterUser(t *testing.T, service *testAuthService, phone, nickname, deviceID string) *AuthSessionResponse {
	t.Helper()

	session, err := service.Register(context.Background(), RegisterInput{
		Phone:    phone,
		Password: "secret123",
		Nickname: nickname,
		DeviceID: deviceID,
	})
	if err != nil {
		t.Fatalf("register user %s: %v", phone, err)
	}
	return session
}

func authHTTPRequest(method, path, token, body string) *http.Request {
	req := httptest.NewRequest(method, path, strings.NewReader(body))
	if body != "" {
		req.Header.Set("Content-Type", "application/json")
	}
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	return req
}

func TestAuthHTTPFollowCreatesOwnedRowWithoutAdoptingLegacyNullRow(t *testing.T) {
	service := newTestAuthService(t)
	useGlobalTestDB(t, service)
	handler := newHTTPHandler(service.AuthService)

	userA := authHTTPRegisterUser(t, service, "13800000000", "Alice", "device-a")
	stubUserDataQuoteFetcher(t, func(stockCodes ...string) (*[]data.StockInfo, error) {
		return &[]data.StockInfo{{
			Code:  stockCodes[0],
			Name:  "PingAn",
			Price: "12.34",
		}}, nil
	})

	if err := service.dao.Create(&data.FollowedStock{
		StockCode: "sz000001",
		Name:      "legacy",
		Time:      time.Now(),
	}).Error; err != nil {
		t.Fatalf("create legacy followed stock: %v", err)
	}

	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, authHTTPRequest(http.MethodPost, "/api/follow", userA.AccessToken, `{"stockCode":"sz000001"}`))

	if rec.Code != http.StatusOK {
		t.Fatalf("follow status = %d, want 200, body = %s", rec.Code, rec.Body.String())
	}

	var bodyResp map[string]string
	if err := json.Unmarshal(rec.Body.Bytes(), &bodyResp); err != nil {
		t.Fatalf("decode follow body: %v", err)
	}
	if bodyResp["result"] != "关注成功" {
		t.Fatalf("follow result = %q, want %q", bodyResp["result"], "关注成功")
	}

	var ownedCount int64
	if err := service.dao.Model(&data.FollowedStock{}).
		Where("user_id = ? AND stock_code = ? AND is_del = ?", userA.User.ID, "sz000001", 0).
		Count(&ownedCount).Error; err != nil {
		t.Fatalf("count owned followed stock: %v", err)
	}
	if ownedCount != 1 {
		t.Fatalf("owned followed stock count = %d, want 1", ownedCount)
	}

	var legacyCount int64
	if err := service.dao.Model(&data.FollowedStock{}).
		Where("user_id IS NULL AND stock_code = ? AND is_del = ?", "sz000001", 0).
		Count(&legacyCount).Error; err != nil {
		t.Fatalf("count legacy followed stock: %v", err)
	}
	if legacyCount != 1 {
		t.Fatalf("legacy followed stock count = %d, want 1", legacyCount)
	}
}

func TestAuthHTTPFollowListIsScopedToAuthenticatedUser(t *testing.T) {
	service := newTestAuthService(t)
	useGlobalTestDB(t, service)
	handler := newHTTPHandler(service.AuthService)

	userA := authHTTPRegisterUser(t, service, "13800000000", "Alice", "device-a")
	userB := authHTTPRegisterUser(t, service, "13900000000", "Bob", "device-b")

	aID := userA.User.ID
	bID := userB.User.ID
	if err := service.dao.Create(&data.FollowedStock{StockCode: "sh600000", Name: "legacy", Time: time.Now()}).Error; err != nil {
		t.Fatalf("create legacy row: %v", err)
	}
	if err := service.dao.Create(&data.FollowedStock{UserID: &aID, StockCode: "sh600001", Name: "A-only", Time: time.Now()}).Error; err != nil {
		t.Fatalf("create user A row: %v", err)
	}
	if err := service.dao.Create(&data.FollowedStock{UserID: &bID, StockCode: "sh600002", Name: "B-only", Time: time.Now()}).Error; err != nil {
		t.Fatalf("create user B row: %v", err)
	}

	recA := httptest.NewRecorder()
	handler.ServeHTTP(recA, authHTTPRequest(http.MethodGet, "/api/follow-list", userA.AccessToken, ""))
	if recA.Code != http.StatusOK {
		t.Fatalf("user A list status = %d, want 200, body = %s", recA.Code, recA.Body.String())
	}

	var itemsA []map[string]any
	if err := json.Unmarshal(recA.Body.Bytes(), &itemsA); err != nil {
		t.Fatalf("decode user A list: %v", err)
	}
	if len(itemsA) != 1 || itemsA[0]["name"] != "A-only" {
		t.Fatalf("user A items = %+v, want only A-owned row", itemsA)
	}

	recB := httptest.NewRecorder()
	handler.ServeHTTP(recB, authHTTPRequest(http.MethodGet, "/api/follow-list", userB.AccessToken, ""))
	if recB.Code != http.StatusOK {
		t.Fatalf("user B list status = %d, want 200, body = %s", recB.Code, recB.Body.String())
	}

	var itemsB []map[string]any
	if err := json.Unmarshal(recB.Body.Bytes(), &itemsB); err != nil {
		t.Fatalf("decode user B list: %v", err)
	}
	if len(itemsB) != 1 || itemsB[0]["name"] != "B-only" {
		t.Fatalf("user B items = %+v, want only B-owned row", itemsB)
	}
}

func TestAuthHTTPUnfollowOnlyDeletesAuthenticatedUsersRecord(t *testing.T) {
	service := newTestAuthService(t)
	useGlobalTestDB(t, service)
	handler := newHTTPHandler(service.AuthService)

	userA := authHTTPRegisterUser(t, service, "13800000000", "Alice", "device-a")
	userB := authHTTPRegisterUser(t, service, "13900000000", "Bob", "device-b")
	aID := userA.User.ID
	bID := userB.User.ID

	if err := service.dao.Create(&data.FollowedStock{UserID: &aID, StockCode: "sh600000", Name: "A", Time: time.Now()}).Error; err != nil {
		t.Fatalf("create user A row: %v", err)
	}
	if err := service.dao.Create(&data.FollowedStock{UserID: &bID, StockCode: "sh600000", Name: "B", Time: time.Now()}).Error; err != nil {
		t.Fatalf("create user B row: %v", err)
	}

	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, authHTTPRequest(http.MethodPost, "/api/unfollow", userA.AccessToken, `{"stockCode":"sh600000"}`))

	if rec.Code != http.StatusOK {
		t.Fatalf("unfollow status = %d, want 200, body = %s", rec.Code, rec.Body.String())
	}

	var bodyResp map[string]string
	if err := json.Unmarshal(rec.Body.Bytes(), &bodyResp); err != nil {
		t.Fatalf("decode unfollow body: %v", err)
	}
	if bodyResp["result"] != "取消关注成功" {
		t.Fatalf("unfollow result = %q, want %q", bodyResp["result"], "取消关注成功")
	}

	var aCount int64
	if err := service.dao.Model(&data.FollowedStock{}).
		Where("user_id = ? AND stock_code = ? AND is_del = ?", userA.User.ID, "sh600000", 0).
		Count(&aCount).Error; err != nil {
		t.Fatalf("count user A rows: %v", err)
	}
	if aCount != 0 {
		t.Fatalf("user A active rows = %d, want 0", aCount)
	}

	var bCount int64
	if err := service.dao.Model(&data.FollowedStock{}).
		Where("user_id = ? AND stock_code = ? AND is_del = ?", userB.User.ID, "sh600000", 0).
		Count(&bCount).Error; err != nil {
		t.Fatalf("count user B rows: %v", err)
	}
	if bCount != 1 {
		t.Fatalf("user B active rows = %d, want 1", bCount)
	}
}
