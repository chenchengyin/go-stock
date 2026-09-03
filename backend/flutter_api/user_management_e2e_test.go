package flutter_api

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"go-stock/backend/data"

	"gorm.io/gorm"
)

type ownedDataSnapshot struct {
	FollowedStocks []string `json:"followedStocks"`
	Groups         []string `json:"groups"`
	GroupStocks    []string `json:"groupStocks"`
	TradingRecords []string `json:"tradingRecords"`
}

func TestUserManagementSingleDeviceHTTPFlow(t *testing.T) {
	dao := newUserDataTestDB(t)
	createLegacyUserOwnedTables(t, dao)

	legacyCounts := map[string]int64{
		"followed_stock":   countRows(t, dao, "followed_stock"),
		"stock_groups":     countRows(t, dao, "stock_groups"),
		"group_stock_info": countRows(t, dao, "group_stock_info"),
		"trading_records":  countRows(t, dao, "trading_records"),
	}
	if err := MigrateAuthTables(dao); err != nil {
		t.Fatalf("migrate auth and user data: %v", err)
	}
	for table, want := range legacyCounts {
		if got := countRows(t, dao, table); got != want {
			t.Fatalf("%s rows after migration = %d, want %d", table, got, want)
		}
	}

	authService := newDeterministicE2EAuthService(dao)
	handler := newUserManagementE2EHandler(authService, dao)

	deviceA := registerThroughHTTP(t, handler, "13800000000", "Alice", "device-a")
	if deviceA.AccessToken != "e2e-token-1" {
		t.Fatalf("device A token = %q, want deterministic first-login token", deviceA.AccessToken)
	}
	assertStoredDeviceSession(t, dao, deviceA.User.ID, "device-a", 1)

	protectedA := serveE2ERequest(t, handler, http.MethodGet, "/api/news", deviceA.AccessToken, "")
	assertHTTPStatus(t, protectedA, http.StatusOK)
	assertPrincipalResponse(t, protectedA, deviceA.User.ID, "device-a")

	deviceB := loginThroughHTTP(t, handler, "13800000000", "device-b")
	if deviceB.User.ID != deviceA.User.ID {
		t.Fatalf("device B user = %q, want same account %q", deviceB.User.ID, deviceA.User.ID)
	}
	if deviceB.AccessToken != "e2e-token-2" || deviceB.AccessToken == deviceA.AccessToken {
		t.Fatalf("device B token = %q, want new deterministic token", deviceB.AccessToken)
	}
	assertStoredDeviceSession(t, dao, deviceB.User.ID, "device-b", 1)

	replaced := serveE2ERequest(t, handler, http.MethodGet, "/api/news", deviceA.AccessToken, "")
	assertAuthHTTPError(t, replaced, http.StatusUnauthorized, "SESSION_REPLACED")

	protectedB := serveE2ERequest(t, handler, http.MethodGet, "/api/news", deviceB.AccessToken, "")
	assertHTTPStatus(t, protectedB, http.StatusOK)
	assertPrincipalResponse(t, protectedB, deviceB.User.ID, "device-b")

	otherAccount := registerThroughHTTP(t, handler, "13900000000", "Bob", "device-other")
	seedOwnedData(t, dao, deviceB.User.ID, otherAccount.User.ID)

	ownerSnapshot := requestOwnedData(t, handler, deviceB.AccessToken)
	assertOwnedDataSnapshot(t, ownerSnapshot, "owner-stock", "owner-group", "sz000001", "owner-trade")

	otherSnapshot := requestOwnedData(t, handler, otherAccount.AccessToken)
	assertOwnedDataSnapshot(t, otherSnapshot, "other-stock", "other-group", "sz000003", "other-trade")

	assertLegacyRowsRemainUnowned(t, dao, legacyCounts)

	logout := serveE2ERequest(t, handler, http.MethodPost, "/api/auth/logout", deviceB.AccessToken, "")
	assertHTTPStatus(t, logout, http.StatusOK)

	afterLogout := serveE2ERequest(t, handler, http.MethodGet, "/api/news", deviceB.AccessToken, "")
	assertAuthHTTPError(t, afterLogout, http.StatusUnauthorized, "UNAUTHENTICATED")

	otherStillActive := serveE2ERequest(t, handler, http.MethodGet, "/api/news", otherAccount.AccessToken, "")
	assertHTTPStatus(t, otherStillActive, http.StatusOK)
}

func newDeterministicE2EAuthService(dao *gorm.DB) *AuthService {
	service := NewAuthService(dao, nil)
	service.now = func() time.Time {
		return time.Date(2026, time.August, 30, 9, 0, 0, 0, time.UTC)
	}

	nextID := 0
	service.newID = func() (string, error) {
		nextID++
		return fmt.Sprintf("e2e-id-%d", nextID), nil
	}

	nextToken := 0
	service.newToken = func() (string, error) {
		nextToken++
		return fmt.Sprintf("e2e-token-%d", nextToken), nil
	}
	return service
}

func newUserManagementE2EHandler(authService *AuthService, dao *gorm.DB) http.Handler {
	protectedMux := http.NewServeMux()
	protectedMux.HandleFunc("/api/news", func(w http.ResponseWriter, r *http.Request) {
		principal, ok := PrincipalFromContext(r.Context())
		if !ok {
			WriteAuthError(w, newAuthError(http.StatusUnauthorized, "UNAUTHENTICATED", "未认证"))
			return
		}
		WriteJSON(w, map[string]string{
			"userId":   principal.UserID,
			"deviceId": principal.DeviceID,
		})
	})
	protectedMux.HandleFunc("/api/e2e/owned-data", ownedDataSnapshotHandler(NewUserDataService(dao)))

	root := http.NewServeMux()
	root.Handle("/api/auth/", NewAuthHTTPHandler(authService))
	root.Handle("/api/admin/", NewAdminHTTPHandler(NewAdminService(dao, nil)))
	root.Handle("/", RequireAuth(authService, protectedMux))
	return root
}

func ownedDataSnapshotHandler(userData *UserDataService) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		principal, ok := PrincipalFromContext(r.Context())
		if !ok {
			WriteAuthError(w, newAuthError(http.StatusUnauthorized, "UNAUTHENTICATED", "未认证"))
			return
		}

		followed, err := userData.ListFollowedStocks(r.Context(), principal.UserID, 0)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		snapshot := ownedDataSnapshot{
			FollowedStocks: make([]string, 0, len(followed)),
			Groups:         []string{},
			TradingRecords: []string{},
		}
		for _, stock := range followed {
			snapshot.FollowedStocks = append(snapshot.FollowedStocks, stock.Name)
		}

		groups, err := userData.ListGroups(r.Context(), principal.UserID)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		for _, group := range groups {
			snapshot.Groups = append(snapshot.Groups, group.Name)
			memberships, err := userData.ListGroupStocks(r.Context(), principal.UserID, group.ID)
			if err != nil {
				http.Error(w, err.Error(), http.StatusInternalServerError)
				return
			}
			for _, membership := range memberships {
				snapshot.GroupStocks = append(snapshot.GroupStocks, membership.StockCode)
			}
		}

		tradingRecords, err := userData.ListTradingRecords(r.Context(), principal.UserID)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		for _, record := range tradingRecords {
			snapshot.TradingRecords = append(snapshot.TradingRecords, record.StockName)
		}
		WriteJSON(w, snapshot)
	}
}

func registerThroughHTTP(t *testing.T, handler http.Handler, phone, nickname, deviceID string) AuthSessionResponse {
	t.Helper()
	body := fmt.Sprintf(`{"phone":%q,"password":"secret123","nickname":%q,"deviceId":%q}`, phone, nickname, deviceID)
	recorder := serveE2ERequest(t, handler, http.MethodPost, "/api/auth/register", "", body)
	assertHTTPStatus(t, recorder, http.StatusCreated)
	var registration RegisterResponse
	if err := json.Unmarshal(recorder.Body.Bytes(), &registration); err != nil {
		t.Fatalf("decode registration response: %v; body = %s", err, recorder.Body.String())
	}
	if registration.Status != authStatusDisabled {
		t.Fatalf("registration status = %q, want %q", registration.Status, authStatusDisabled)
	}
	if registration.Message != "注册成功，请等待管理员启用后再登录" {
		t.Fatalf("registration message = %q, want pending activation message", registration.Message)
	}
	responseJSON := recorder.Body.String()
	if strings.Contains(responseJSON, "accessToken") || strings.Contains(responseJSON, "expiresAt") {
		t.Fatalf("registration response contains session fields: %s", responseJSON)
	}

	adminToken := loginAdminThroughHTTP(t, handler)
	enable := serveE2ERequest(
		t,
		handler,
		http.MethodPatch,
		"/api/admin/users/"+registration.User.ID+"/status",
		adminToken,
		`{"status":"active"}`,
	)
	assertHTTPStatus(t, enable, http.StatusOK)
	return loginThroughHTTP(t, handler, phone, deviceID)
}

func loginThroughHTTP(t *testing.T, handler http.Handler, phone, deviceID string) AuthSessionResponse {
	t.Helper()
	body := fmt.Sprintf(`{"phone":%q,"password":"secret123","deviceId":%q}`, phone, deviceID)
	recorder := serveE2ERequest(t, handler, http.MethodPost, "/api/auth/login", "", body)
	assertHTTPStatus(t, recorder, http.StatusOK)
	return decodeSessionResponse(t, recorder)
}

func loginAdminThroughHTTP(t *testing.T, handler http.Handler) string {
	t.Helper()
	recorder := serveE2ERequest(
		t,
		handler,
		http.MethodPost,
		"/api/admin/login",
		"",
		`{"username":"admin","password":"admin"}`,
	)
	assertHTTPStatus(t, recorder, http.StatusOK)
	var response AdminSessionResponse
	if err := json.Unmarshal(recorder.Body.Bytes(), &response); err != nil {
		t.Fatalf("decode admin login: %v; body = %s", err, recorder.Body.String())
	}
	if response.AccessToken == "" {
		t.Fatal("admin login token is empty")
	}
	return response.AccessToken
}

func serveE2ERequest(t *testing.T, handler http.Handler, method, path, token, body string) *httptest.ResponseRecorder {
	t.Helper()
	request := httptest.NewRequest(method, path, strings.NewReader(body))
	if body != "" {
		request.Header.Set("Content-Type", "application/json")
	}
	if token != "" {
		request.Header.Set("Authorization", "Bearer "+token)
	}
	recorder := httptest.NewRecorder()
	handler.ServeHTTP(recorder, request)
	return recorder
}

func decodeSessionResponse(t *testing.T, recorder *httptest.ResponseRecorder) AuthSessionResponse {
	t.Helper()
	var response AuthSessionResponse
	if err := json.Unmarshal(recorder.Body.Bytes(), &response); err != nil {
		t.Fatalf("decode session response: %v; body = %s", err, recorder.Body.String())
	}
	return response
}

func assertHTTPStatus(t *testing.T, recorder *httptest.ResponseRecorder, want int) {
	t.Helper()
	if recorder.Code != want {
		t.Fatalf("status = %d, want %d; body = %s", recorder.Code, want, recorder.Body.String())
	}
}

func assertAuthHTTPError(t *testing.T, recorder *httptest.ResponseRecorder, wantStatus int, wantCode string) {
	t.Helper()
	assertHTTPStatus(t, recorder, wantStatus)
	var response map[string]string
	if err := json.Unmarshal(recorder.Body.Bytes(), &response); err != nil {
		t.Fatalf("decode auth error: %v; body = %s", err, recorder.Body.String())
	}
	if response["code"] != wantCode {
		t.Fatalf("auth error code = %q, want %q; body = %s", response["code"], wantCode, recorder.Body.String())
	}
}

func assertPrincipalResponse(t *testing.T, recorder *httptest.ResponseRecorder, wantUserID, wantDeviceID string) {
	t.Helper()
	var response map[string]string
	if err := json.Unmarshal(recorder.Body.Bytes(), &response); err != nil {
		t.Fatalf("decode principal response: %v; body = %s", err, recorder.Body.String())
	}
	if response["userId"] != wantUserID || response["deviceId"] != wantDeviceID {
		t.Fatalf("principal = (%q, %q), want (%q, %q)", response["userId"], response["deviceId"], wantUserID, wantDeviceID)
	}
}

func assertStoredDeviceSession(t *testing.T, dao *gorm.DB, userID, wantDeviceID string, wantActive int64) {
	t.Helper()
	var active []AuthSession
	if err := dao.Where("user_id = ? AND revoked_at IS NULL", userID).Find(&active).Error; err != nil {
		t.Fatalf("load active sessions: %v", err)
	}
	if int64(len(active)) != wantActive {
		t.Fatalf("active sessions = %d, want %d", len(active), wantActive)
	}
	if wantActive == 1 && active[0].DeviceID != wantDeviceID {
		t.Fatalf("active device = %q, want %q", active[0].DeviceID, wantDeviceID)
	}
}

func seedOwnedData(t *testing.T, dao *gorm.DB, ownerID, otherID string) {
	t.Helper()
	fixedTime := time.Date(2026, time.August, 30, 10, 0, 0, 0, time.UTC)
	for _, stock := range []data.FollowedStock{
		{UserID: &ownerID, StockCode: "sz000001", Name: "owner-stock", Time: fixedTime},
		{UserID: &otherID, StockCode: "sz000001", Name: "other-stock", Time: fixedTime},
	} {
		if err := dao.Create(&stock).Error; err != nil {
			t.Fatalf("create followed stock %q: %v", stock.Name, err)
		}
	}
	ownerGroup := data.Group{UserID: &ownerID, Name: "owner-group", Sort: 1}
	otherGroup := data.Group{UserID: &otherID, Name: "other-group", Sort: 1}
	for _, group := range []*data.Group{&ownerGroup, &otherGroup} {
		if err := dao.Create(group).Error; err != nil {
			t.Fatalf("create group %q: %v", group.Name, err)
		}
	}
	for _, membership := range []data.GroupStock{
		{UserID: &ownerID, GroupId: int(ownerGroup.ID), StockCode: "sz000001"},
		{UserID: &otherID, GroupId: int(ownerGroup.ID), StockCode: "sh600000"},
		{UserID: &otherID, GroupId: int(otherGroup.ID), StockCode: "sz000003"},
	} {
		if err := dao.Create(&membership).Error; err != nil {
			t.Fatalf("create group membership %q: %v", membership.StockCode, err)
		}
	}
	for _, record := range []data.TradingRecord{
		{UserID: &ownerID, StockCode: "sz000001", StockName: "owner-trade", Direction: "买入", Price: 10, Volume: 100, TradingTime: fixedTime},
		{UserID: &otherID, StockCode: "sz000001", StockName: "other-trade", Direction: "买入", Price: 11, Volume: 100, TradingTime: fixedTime},
	} {
		if err := dao.Create(&record).Error; err != nil {
			t.Fatalf("create trading record %q: %v", record.StockName, err)
		}
	}
}

func requestOwnedData(t *testing.T, handler http.Handler, token string) ownedDataSnapshot {
	t.Helper()
	recorder := serveE2ERequest(t, handler, http.MethodGet, "/api/e2e/owned-data", token, "")
	assertHTTPStatus(t, recorder, http.StatusOK)
	var snapshot ownedDataSnapshot
	if err := json.Unmarshal(recorder.Body.Bytes(), &snapshot); err != nil {
		t.Fatalf("decode owned data: %v; body = %s", err, recorder.Body.String())
	}
	return snapshot
}

func assertOwnedDataSnapshot(t *testing.T, snapshot ownedDataSnapshot, stock, group, groupStock, trade string) {
	t.Helper()
	if len(snapshot.FollowedStocks) != 1 || snapshot.FollowedStocks[0] != stock {
		t.Fatalf("followed stocks = %v, want [%s]", snapshot.FollowedStocks, stock)
	}
	if len(snapshot.Groups) != 1 || snapshot.Groups[0] != group {
		t.Fatalf("groups = %v, want [%s]", snapshot.Groups, group)
	}
	if len(snapshot.GroupStocks) != 1 || snapshot.GroupStocks[0] != groupStock {
		t.Fatalf("group stocks = %v, want [%s]", snapshot.GroupStocks, groupStock)
	}
	if len(snapshot.TradingRecords) != 1 || snapshot.TradingRecords[0] != trade {
		t.Fatalf("trading records = %v, want [%s]", snapshot.TradingRecords, trade)
	}
}

func assertLegacyRowsRemainUnowned(t *testing.T, dao *gorm.DB, originalCounts map[string]int64) {
	t.Helper()
	for table, want := range originalCounts {
		var unowned int64
		if err := dao.Table(table).Where("user_id IS NULL").Count(&unowned).Error; err != nil {
			t.Fatalf("count unowned %s rows: %v", table, err)
		}
		if unowned != want {
			t.Fatalf("unowned %s rows = %d, want preserved %d", table, unowned, want)
		}
	}
}
