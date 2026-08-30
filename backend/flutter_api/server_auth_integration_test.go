package flutter_api

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/gorilla/websocket"
	"gorm.io/gorm"
)

func principalProbe(w http.ResponseWriter, r *http.Request) {
	principal, ok := PrincipalFromContext(r.Context())
	if !ok {
		WriteAuthError(w, &AuthError{Status: http.StatusUnauthorized, Code: "UNAUTHENTICATED", Message: "missing principal"})
		return
	}
	WriteJSON(w, map[string]string{"userId": principal.UserID})
}

func TestProtectedRoutesRejectUnauthenticatedRequestsBeforeBusinessHandlers(t *testing.T) {
	service := newTestAuthService(t)
	var calls atomic.Int32
	probe := func(w http.ResponseWriter, r *http.Request) {
		calls.Add(1)
		principalProbe(w, r)
	}
	handler := newHTTPHandler(service.AuthService, serverHandlerOverrides{
		News:       probe,
		FollowList: probe,
		Upload:     probe,
		WebSocket:  probe,
	})

	for _, path := range []string{"/api/news", "/api/follow-list", "/api/upload", "/uploads/private.png", "/ws"} {
		t.Run(path, func(t *testing.T) {
			rec := httptest.NewRecorder()
			handler.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, path, nil))
			if rec.Code != http.StatusUnauthorized {
				t.Fatalf("status = %d, want 401, body = %s", rec.Code, rec.Body.String())
			}
		})
	}
	if got := calls.Load(); got != 0 {
		t.Fatalf("business handler calls = %d, want 0", got)
	}
}

func TestProtectedRoutePassesAuthenticatedPrincipalToBusinessHandler(t *testing.T) {
	service := newTestAuthService(t)
	session := authHTTPRegisterUser(t, service, "13800000000", "Alice", "device-a")
	handler := newHTTPHandler(service.AuthService, serverHandlerOverrides{News: principalProbe})

	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, authHTTPRequest(http.MethodGet, "/api/news", session.AccessToken, ""))
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200, body = %s", rec.Code, rec.Body.String())
	}

	var got map[string]string
	if err := json.Unmarshal(rec.Body.Bytes(), &got); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if got["userId"] != session.User.ID {
		t.Fatalf("principal user = %q, want %q", got["userId"], session.User.ID)
	}
}

func TestProtectedRouterKeepsRegisterLoginAndHealthPublic(t *testing.T) {
	service := newTestAuthService(t)
	handler := newHTTPHandler(service.AuthService)

	healthRec := httptest.NewRecorder()
	handler.ServeHTTP(healthRec, httptest.NewRequest(http.MethodGet, "/api/health", nil))
	if healthRec.Code != http.StatusOK {
		t.Fatalf("health status = %d, want 200, body = %s", healthRec.Code, healthRec.Body.String())
	}

	registerRec := httptest.NewRecorder()
	registerReq := httptest.NewRequest(http.MethodPost, "/api/auth/register", strings.NewReader(`{"phone":"13800000000","password":"secret123","nickname":"Alice","deviceId":"device-a"}`))
	registerReq.Header.Set("Content-Type", "application/json")
	handler.ServeHTTP(registerRec, registerReq)
	if registerRec.Code != http.StatusCreated {
		t.Fatalf("register status = %d, want 201, body = %s", registerRec.Code, registerRec.Body.String())
	}

	loginRec := httptest.NewRecorder()
	loginReq := httptest.NewRequest(http.MethodPost, "/api/auth/login", strings.NewReader(`{"phone":"13800000000","password":"secret123","deviceId":"device-b"}`))
	loginReq.Header.Set("Content-Type", "application/json")
	handler.ServeHTTP(loginRec, loginReq)
	if loginRec.Code != http.StatusOK {
		t.Fatalf("login status = %d, want 200, body = %s", loginRec.Code, loginRec.Body.String())
	}
}

func TestStrategyIdentityIgnoresForgedUserAndNickname(t *testing.T) {
	service := newTestAuthService(t)
	useGlobalTestDB(t, service)
	migrateStrategyTables(t, service.dao)
	handler := newHTTPHandler(service.AuthService)

	userA := authHTTPRegisterUser(t, service, "13800000000", "Alice", "device-a")
	userB := authHTTPRegisterUser(t, service, "13900000000", "Bob", "device-b")

	createRec := httptest.NewRecorder()
	createBody := fmt.Sprintf(`{"action":"create_post","userId":%q,"nickname":"Mallory","title":"owned","content":"body","images":[]}`, userB.User.ID)
	handler.ServeHTTP(createRec, authHTTPRequest(http.MethodPost, "/api/strategy", userA.AccessToken, createBody))
	if createRec.Code != http.StatusOK {
		t.Fatalf("create status = %d, want 200, body = %s", createRec.Code, createRec.Body.String())
	}

	var post StrategyPost
	if err := json.Unmarshal(createRec.Body.Bytes(), &post); err != nil {
		t.Fatalf("decode post: %v", err)
	}
	if post.UserID != userA.User.ID || post.Nickname != "Alice" {
		t.Fatalf("post identity = (%q, %q), want (%q, %q)", post.UserID, post.Nickname, userA.User.ID, "Alice")
	}

	pointsRec := httptest.NewRecorder()
	path := "/api/strategy?action=points&userId=" + userB.User.ID
	handler.ServeHTTP(pointsRec, authHTTPRequest(http.MethodGet, path, userA.AccessToken, ""))
	if pointsRec.Code != http.StatusOK {
		t.Fatalf("points status = %d, want 200, body = %s", pointsRec.Code, pointsRec.Body.String())
	}
	var points StrategyUser
	if err := json.Unmarshal(pointsRec.Body.Bytes(), &points); err != nil {
		t.Fatalf("decode points: %v", err)
	}
	if points.UserID != userA.User.ID {
		t.Fatalf("points user = %q, want authenticated user %q", points.UserID, userA.User.ID)
	}
}

func TestStrategyIdentityCannotDeleteAnotherUsersPostOrComment(t *testing.T) {
	service := newTestAuthService(t)
	useGlobalTestDB(t, service)
	migrateStrategyTables(t, service.dao)
	handler := newHTTPHandler(service.AuthService)

	userA := authHTTPRegisterUser(t, service, "13800000000", "Alice", "device-a")
	userB := authHTTPRegisterUser(t, service, "13900000000", "Bob", "device-b")
	api := NewStrategyAPI()
	post, err := api.CreatePost(userB.User.ID, "Bob", "B post", "body", nil)
	if err != nil {
		t.Fatalf("create B post: %v", err)
	}
	comment, _, _, err := api.CreateComment(post.ID, nil, userB.User.ID, "Bob", "B comment", nil, nil, nil)
	if err != nil {
		t.Fatalf("create B comment: %v", err)
	}

	deleteCommentRec := httptest.NewRecorder()
	deleteCommentBody := fmt.Sprintf(`{"action":"delete_comment","userId":%q,"commentId":%d}`, userB.User.ID, comment.ID)
	handler.ServeHTTP(deleteCommentRec, authHTTPRequest(http.MethodPost, "/api/strategy", userA.AccessToken, deleteCommentBody))
	if !strings.Contains(deleteCommentRec.Body.String(), "error") {
		t.Fatalf("delete comment body = %s, want owner error", deleteCommentRec.Body.String())
	}
	var commentCount int64
	if err := service.dao.Unscoped().Model(&StrategyComment{}).Where("id = ? AND user_id = ?", comment.ID, userB.User.ID).Count(&commentCount).Error; err != nil {
		t.Fatalf("count B comment: %v", err)
	}
	if commentCount != 1 {
		t.Fatalf("B comment count = %d, want 1", commentCount)
	}

	deletePostRec := httptest.NewRecorder()
	deletePostBody := fmt.Sprintf(`{"action":"delete_post","userId":%q,"postId":%d}`, userB.User.ID, post.ID)
	handler.ServeHTTP(deletePostRec, authHTTPRequest(http.MethodPost, "/api/strategy", userA.AccessToken, deletePostBody))
	if !strings.Contains(deletePostRec.Body.String(), "error") {
		t.Fatalf("delete post body = %s, want owner error", deletePostRec.Body.String())
	}
	var postCount int64
	if err := service.dao.Unscoped().Model(&StrategyPost{}).Where("id = ? AND user_id = ?", post.ID, userB.User.ID).Count(&postCount).Error; err != nil {
		t.Fatalf("count B post: %v", err)
	}
	if postCount != 1 {
		t.Fatalf("B post count = %d, want 1", postCount)
	}
}

func TestWebSocketAuthAcceptsBearerHeaderAndQueryFallback(t *testing.T) {
	resetWebSocketClients(t)
	service := newTestAuthService(t)
	userA := authHTTPRegisterUser(t, service, "13800000000", "Alice", "device-a")
	userB := authHTTPRegisterUser(t, service, "13900000000", "Bob", "device-b")
	server := httptest.NewServer(newHTTPHandler(service.AuthService))
	defer server.Close()
	wsURL := "ws" + strings.TrimPrefix(server.URL, "http") + "/ws"

	header := http.Header{}
	header.Set("Authorization", "Bearer "+userA.AccessToken)
	headerConn, resp, err := websocket.DefaultDialer.Dial(wsURL, header)
	if err != nil {
		status := 0
		if resp != nil {
			status = resp.StatusCode
		}
		t.Fatalf("dial with bearer: status=%d err=%v", status, err)
	}
	defer headerConn.Close()

	queryConn, resp, err := websocket.DefaultDialer.Dial(wsURL+"?access_token="+userB.AccessToken, nil)
	if err != nil {
		status := 0
		if resp != nil {
			status = resp.StatusCode
		}
		t.Fatalf("dial with query fallback: status=%d err=%v", status, err)
	}
	defer queryConn.Close()

	waitForWebSocketClientCount(t, 2)
}

func TestWebSocketAuthRejectsMissingAndInvalidTokens(t *testing.T) {
	resetWebSocketClients(t)
	service := newTestAuthService(t)
	server := httptest.NewServer(newHTTPHandler(service.AuthService))
	defer server.Close()
	wsURL := "ws" + strings.TrimPrefix(server.URL, "http") + "/ws"

	for _, target := range []string{wsURL, wsURL + "?access_token=not-valid"} {
		conn, resp, err := websocket.DefaultDialer.Dial(target, nil)
		if conn != nil {
			conn.Close()
		}
		if err == nil {
			t.Fatalf("dial %q succeeded, want authentication failure", target)
		}
		if resp == nil || resp.StatusCode != http.StatusUnauthorized {
			status := 0
			if resp != nil {
				status = resp.StatusCode
			}
			t.Fatalf("dial %q status = %d, want 401", target, status)
		}
	}
}

func TestWebSocketAuthClosesOldSessionAfterReplacement(t *testing.T) {
	resetWebSocketClients(t)
	service := newTestAuthService(t)
	service.onSessionsReplaced = closeWebSocketsForUser
	first := authHTTPRegisterUser(t, service, "13800000000", "Alice", "device-a")
	server := httptest.NewServer(newHTTPHandler(service.AuthService))
	defer server.Close()
	wsURL := "ws" + strings.TrimPrefix(server.URL, "http") + "/ws"

	header := http.Header{}
	header.Set("Authorization", "Bearer "+first.AccessToken)
	oldConn, _, err := websocket.DefaultDialer.Dial(wsURL, header)
	if err != nil {
		t.Fatalf("dial old session: %v", err)
	}
	defer oldConn.Close()
	waitForWebSocketClientCount(t, 1)

	second, err := service.Login(context.Background(), LoginInput{
		Phone:    "13800000000",
		Password: "secret123",
		DeviceID: "device-b",
	})
	if err != nil {
		t.Fatalf("replacement login: %v", err)
	}

	if err := oldConn.SetReadDeadline(time.Now().Add(2 * time.Second)); err != nil {
		t.Fatalf("set read deadline: %v", err)
	}
	if _, _, err := oldConn.ReadMessage(); err == nil {
		t.Fatal("old session websocket remained open after replacement")
	}
	waitForWebSocketClientCount(t, 0)

	newHeader := http.Header{}
	newHeader.Set("Authorization", "Bearer "+second.AccessToken)
	newConn, _, err := websocket.DefaultDialer.Dial(wsURL, newHeader)
	if err != nil {
		t.Fatalf("dial replacement session: %v", err)
	}
	defer newConn.Close()
	waitForWebSocketClientCount(t, 1)
}

func migrateStrategyTables(t *testing.T, dao *gorm.DB) {
	t.Helper()
	if err := dao.AutoMigrate(
		&StrategyUser{},
		&StrategyPost{},
		&StrategyComment{},
		&StrategyLike{},
		&StrategyCheckIn{},
		&StrategyPointsLog{},
	); err != nil {
		t.Fatalf("migrate strategy tables: %v", err)
	}
}

func resetWebSocketClients(t *testing.T) {
	t.Helper()
	closeAll := func() {
		wsClientsMu.Lock()
		connections := make([]*websocket.Conn, 0, len(wsClients))
		for conn := range wsClients {
			connections = append(connections, conn)
			delete(wsClients, conn)
		}
		wsClientsMu.Unlock()
		for _, conn := range connections {
			_ = conn.Close()
		}
	}
	closeAll()
	t.Cleanup(closeAll)
}

func waitForWebSocketClientCount(t *testing.T, want int) {
	t.Helper()
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		wsClientsMu.RLock()
		got := len(wsClients)
		wsClientsMu.RUnlock()
		if got == want {
			return
		}
		time.Sleep(10 * time.Millisecond)
	}
	wsClientsMu.RLock()
	got := len(wsClients)
	wsClientsMu.RUnlock()
	t.Fatalf("websocket client count = %d, want %d", got, want)
}
