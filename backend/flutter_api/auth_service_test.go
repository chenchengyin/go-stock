package flutter_api

import (
	"context"
	"errors"
	"fmt"
	"testing"
	"time"
)

func TestAuthServiceRegisterCreatesUserAndSession(t *testing.T) {
	service := newTestAuthService(t)

	got, err := service.Register(context.Background(), RegisterInput{
		Phone:    " 13800000000 ",
		Password: "secret123",
		Nickname: " ",
		DeviceID: "device-a",
	})
	if err != nil {
		t.Fatalf("register: %v", err)
	}

	if got.AccessToken != "token-register" {
		t.Fatalf("access token = %q, want %q", got.AccessToken, "token-register")
	}
	if got.ExpiresAt != service.now().Add(authSessionTTL) {
		t.Fatalf("expires at = %v, want %v", got.ExpiresAt, service.now().Add(authSessionTTL))
	}
	if got.User.Phone != "13800000000" {
		t.Fatalf("phone = %q, want trimmed phone", got.User.Phone)
	}
	if got.User.Nickname != "13800000000" {
		t.Fatalf("nickname = %q, want default nickname", got.User.Nickname)
	}
	if got.User.Role != authRoleUser {
		t.Fatalf("role = %q, want %q", got.User.Role, authRoleUser)
	}

	principal, err := service.Authenticate(context.Background(), got.AccessToken)
	if err != nil {
		t.Fatalf("authenticate registered token: %v", err)
	}
	if principal.UserID != got.User.ID {
		t.Fatalf("principal user = %q, want %q", principal.UserID, got.User.ID)
	}
	if principal.DeviceID != "device-a" {
		t.Fatalf("principal device = %q, want %q", principal.DeviceID, "device-a")
	}

	var user AuthUser
	if err := service.dao.WithContext(context.Background()).First(&user, "id = ?", got.User.ID).Error; err != nil {
		t.Fatalf("load user: %v", err)
	}
	if user.PasswordHash == "secret123" || user.PasswordHash == "" {
		t.Fatalf("password hash = %q, want bcrypt hash", user.PasswordHash)
	}

	assertActiveSessionCount(t, service, got.User.ID, 1)
}

func TestAuthServiceRegisterRejectsDuplicateAccount(t *testing.T) {
	service := newTestAuthService(t)

	if _, err := service.Register(context.Background(), RegisterInput{
		Phone:    "13800000000",
		Password: "secret123",
		Nickname: "A",
		DeviceID: "device-a",
	}); err != nil {
		t.Fatalf("first register: %v", err)
	}

	_, err := service.Register(context.Background(), RegisterInput{
		Phone:    "13800000000",
		Password: "secret123",
		Nickname: "B",
		DeviceID: "device-b",
	})
	if !IsAuthCode(err, "ACCOUNT_EXISTS") {
		t.Fatalf("duplicate register err = %v, want ACCOUNT_EXISTS", err)
	}
}

func TestAuthServiceRegisterReturnsStructuredValidationError(t *testing.T) {
	service := newTestAuthService(t)

	_, err := service.Register(context.Background(), RegisterInput{
		Phone:    " 1234 ",
		Password: "secret123",
		Nickname: "A",
		DeviceID: "device-a",
	})

	assertAuthError(t, err, 400, "INVALID_ARGUMENT", "账号或密码格式不正确")
}

func TestAuthServiceLoginRejectsInvalidCredentials(t *testing.T) {
	service := newTestAuthService(t)

	if _, err := service.Register(context.Background(), RegisterInput{
		Phone:    "13800000000",
		Password: "secret123",
		Nickname: "A",
		DeviceID: "device-a",
	}); err != nil {
		t.Fatalf("register: %v", err)
	}

	_, wrongPasswordErr := service.Login(context.Background(), LoginInput{
		Phone:    "13800000000",
		Password: "bad-pass",
		DeviceID: "device-b",
	})
	assertAuthError(t, wrongPasswordErr, 401, "INVALID_CREDENTIALS", "账号或密码错误")

	_, missingUserErr := service.Login(context.Background(), LoginInput{
		Phone:    "13900000000",
		Password: "secret123",
		DeviceID: "device-b",
	})
	assertAuthError(t, missingUserErr, 401, "INVALID_CREDENTIALS", "账号或密码错误")
}

func TestAuthServiceLoginPreservesPasswordWhitespace(t *testing.T) {
	service := newTestAuthService(t)

	registerResult, err := service.Register(context.Background(), RegisterInput{
		Phone:    " 13800000000 ",
		Password: " secret123 ",
		Nickname: "A",
		DeviceID: "device-a",
	})
	if err != nil {
		t.Fatalf("register: %v", err)
	}

	loggedIn, err := service.Login(context.Background(), LoginInput{
		Phone:    "13800000000",
		Password: " secret123 ",
		DeviceID: "device-b",
	})
	if err != nil {
		t.Fatalf("login with exact password: %v", err)
	}
	if loggedIn.User.ID != registerResult.User.ID {
		t.Fatalf("logged in user id = %q, want %q", loggedIn.User.ID, registerResult.User.ID)
	}

	_, err = service.Login(context.Background(), LoginInput{
		Phone:    "13800000000",
		Password: "secret123",
		DeviceID: "device-c",
	})
	assertAuthError(t, err, 401, "INVALID_CREDENTIALS", "账号或密码错误")
}

func TestAuthServiceNewLoginReplacesOldDevice(t *testing.T) {
	service := newTestAuthService(t)

	first, err := service.Register(context.Background(), RegisterInput{
		Phone: "13800000000", Password: "secret123", Nickname: "A", DeviceID: "device-a",
	})
	if err != nil {
		t.Fatalf("register: %v", err)
	}

	second, err := service.Login(context.Background(), LoginInput{
		Phone: "13800000000", Password: "secret123", DeviceID: "device-b",
	})
	if err != nil {
		t.Fatalf("login from device b: %v", err)
	}

	err = service.mustAuthenticateErr(context.Background(), first.AccessToken)
	if !IsAuthCode(err, "SESSION_REPLACED") {
		t.Fatalf("old token error = %v, want SESSION_REPLACED", err)
	}
	assertAuthError(t, err, 401, "SESSION_REPLACED", "账号已在其他设备登录，请重新登录")

	principal, err := service.Authenticate(context.Background(), second.AccessToken)
	if err != nil || principal.DeviceID != "device-b" {
		t.Fatalf("new session = %+v, err = %v", principal, err)
	}

	assertActiveSessionCount(t, service, second.User.ID, 1)

	var sessions []AuthSession
	if err := service.dao.Order("created_at asc").Find(&sessions, "user_id = ?", second.User.ID).Error; err != nil {
		t.Fatalf("load sessions: %v", err)
	}
	if len(sessions) != 2 {
		t.Fatalf("session count = %d, want 2", len(sessions))
	}
	if sessions[0].RevokedAt == nil || sessions[0].RevokeReason != "new_login" {
		t.Fatalf("replaced session = %+v, want revoked with new_login", sessions[0])
	}
	if sessions[1].RevokedAt != nil {
		t.Fatalf("new session revoked_at = %v, want nil", sessions[1].RevokedAt)
	}
	if len(service.replacedCalls) != 1 {
		t.Fatalf("replacement callbacks = %d, want 1", len(service.replacedCalls))
	}
	callback := service.replacedCalls[0]
	if callback.userID != second.User.ID || callback.newSessionID != sessions[1].ID {
		t.Fatalf("replacement callback = %+v, want user %q and new session %q", callback, second.User.ID, sessions[1].ID)
	}
	if len(callback.revokedSessionIDs) != 1 || callback.revokedSessionIDs[0] != sessions[0].ID {
		t.Fatalf("revoked session ids = %v, want [%s]", callback.revokedSessionIDs, sessions[0].ID)
	}
}

func TestAuthServiceNewLoginDoesNotRevokeOtherUsers(t *testing.T) {
	service := newTestAuthService(t)

	first, err := service.Register(context.Background(), RegisterInput{
		Phone: "13800000000", Password: "secret123", Nickname: "A", DeviceID: "device-a",
	})
	if err != nil {
		t.Fatalf("register first: %v", err)
	}
	second, err := service.Register(context.Background(), RegisterInput{
		Phone: "13900000000", Password: "secret123", Nickname: "B", DeviceID: "device-b",
	})
	if err != nil {
		t.Fatalf("register second: %v", err)
	}

	if _, err := service.Login(context.Background(), LoginInput{
		Phone: "13800000000", Password: "secret123", DeviceID: "device-c",
	}); err != nil {
		t.Fatalf("login: %v", err)
	}

	if _, err := service.Authenticate(context.Background(), second.AccessToken); err != nil {
		t.Fatalf("authenticate second user token: %v", err)
	}
	if _, err := service.Authenticate(context.Background(), first.AccessToken); !IsAuthCode(err, "SESSION_REPLACED") {
		t.Fatalf("authenticate first user old token err = %v, want SESSION_REPLACED", err)
	}

	assertActiveSessionCount(t, service, first.User.ID, 1)
	assertActiveSessionCount(t, service, second.User.ID, 1)
}

func TestAuthServiceLoginDoesNotCreateSessionAfterConcurrentDisable(t *testing.T) {
	service := newTestAuthService(t)
	registered, err := service.Register(context.Background(), RegisterInput{
		Phone: "13800000000", Password: "secret123", Nickname: "A", DeviceID: "device-a",
	})
	if err != nil {
		t.Fatalf("register: %v", err)
	}

	loginPaused := make(chan struct{})
	releaseLogin := make(chan struct{})
	service.beforeLoginTransaction = func() {
		close(loginPaused)
		<-releaseLogin
	}

	loginResult := make(chan error, 1)
	go func() {
		_, loginErr := service.Login(context.Background(), LoginInput{
			Phone: "13800000000", Password: "secret123", DeviceID: "device-b",
		})
		loginResult <- loginErr
	}()
	<-loginPaused

	admin := newTestAdminService(service.dao)
	disableResult := make(chan error, 1)
	go func() {
		_, disableErr := admin.UpdateUserStatus(context.Background(), registered.User.ID, authStatusDisabled)
		disableResult <- disableErr
	}()

	if disableErr := <-disableResult; disableErr != nil {
		t.Fatalf("disable user: %v", disableErr)
	}
	close(releaseLogin)

	if loginErr := <-loginResult; !IsAuthCode(loginErr, "ACCOUNT_DISABLED") {
		t.Fatalf("login after committed disable err = %v, want ACCOUNT_DISABLED", loginErr)
	}
	assertActiveSessionCount(t, service, registered.User.ID, 0)
}

func TestAuthServiceLogoutRevokesSessionIdempotently(t *testing.T) {
	service := newTestAuthService(t)

	session, err := service.Register(context.Background(), RegisterInput{
		Phone: "13800000000", Password: "secret123", Nickname: "A", DeviceID: "device-a",
	})
	if err != nil {
		t.Fatalf("register: %v", err)
	}
	principal, err := service.Authenticate(context.Background(), session.AccessToken)
	if err != nil {
		t.Fatalf("authenticate before logout: %v", err)
	}

	if err := service.Logout(context.Background(), session.AccessToken); err != nil {
		t.Fatalf("first logout: %v", err)
	}
	if err := service.Logout(context.Background(), session.AccessToken); err != nil {
		t.Fatalf("second logout: %v", err)
	}

	if _, err := service.Authenticate(context.Background(), session.AccessToken); !IsAuthCode(err, "UNAUTHENTICATED") {
		t.Fatalf("authenticate after logout err = %v, want UNAUTHENTICATED", err)
	}
	if len(service.replacedCalls) != 1 {
		t.Fatalf("session callbacks = %d, want 1", len(service.replacedCalls))
	}
	call := service.replacedCalls[0]
	if call.userID != session.User.ID || call.newSessionID != "" {
		t.Fatalf("logout callback = %+v, want user %q and no replacement session", call, session.User.ID)
	}
	if len(call.revokedSessionIDs) != 1 || call.revokedSessionIDs[0] != principal.SessionID {
		t.Fatalf("revoked session IDs = %v, want [%s]", call.revokedSessionIDs, principal.SessionID)
	}

	if err := service.Logout(context.Background(), "unknown-token"); err != nil {
		t.Fatalf("unknown logout: %v", err)
	}
	if len(service.replacedCalls) != 1 {
		t.Fatalf("session callbacks after no-op logouts = %d, want 1", len(service.replacedCalls))
	}

	assertActiveSessionCount(t, service, session.User.ID, 0)
}

func TestAuthServiceAuthenticateRejectsExpiredSession(t *testing.T) {
	service := newTestAuthService(t)

	session, err := service.Register(context.Background(), RegisterInput{
		Phone: "13800000000", Password: "secret123", Nickname: "A", DeviceID: "device-a",
	})
	if err != nil {
		t.Fatalf("register: %v", err)
	}

	service.nowValue = service.nowValue.Add(authSessionTTL + time.Minute)

	err = service.mustAuthenticateErr(context.Background(), session.AccessToken)
	if !IsAuthCode(err, "SESSION_EXPIRED") {
		t.Fatalf("authenticate expired err = %v, want SESSION_EXPIRED", err)
	}
	assertAuthError(t, err, 401, "SESSION_EXPIRED", "会话已过期")

	assertActiveSessionCount(t, service, session.User.ID, 0)
}

func TestAuthServiceUpdateProfileChangesNickname(t *testing.T) {
	service := newTestAuthService(t)

	session, err := service.Register(context.Background(), RegisterInput{
		Phone: "13800000000", Password: "secret123", Nickname: "Old", DeviceID: "device-a",
	})
	if err != nil {
		t.Fatalf("register: %v", err)
	}

	principal, err := service.Authenticate(context.Background(), session.AccessToken)
	if err != nil {
		t.Fatalf("authenticate: %v", err)
	}

	got, err := service.UpdateProfile(context.Background(), principal, UpdateProfileInput{Nickname: "  New Name  "})
	if err != nil {
		t.Fatalf("update profile: %v", err)
	}
	if got.Nickname != "New Name" {
		t.Fatalf("nickname = %q, want %q", got.Nickname, "New Name")
	}
	if got.Phone != "13800000000" {
		t.Fatalf("phone = %q, want original phone", got.Phone)
	}
}

type testAuthService struct {
	*AuthService
	nowValue      time.Time
	nextID        int
	nextToken     int
	replacedCalls []sessionsReplacedCall
}

type sessionsReplacedCall struct {
	userID            string
	newSessionID      string
	revokedSessionIDs []string
}

func newTestAuthService(t *testing.T) *testAuthService {
	t.Helper()

	dao := newAuthTestDB(t)
	if err := MigrateAuthTables(dao); err != nil {
		t.Fatalf("migrate auth tables: %v", err)
	}

	testService := &testAuthService{
		nowValue: time.Date(2026, time.August, 30, 9, 0, 0, 0, time.UTC),
	}

	service := NewAuthService(dao, func(userID, newSessionID string, revokedSessionIDs []string) {
		testService.replacedCalls = append(testService.replacedCalls, sessionsReplacedCall{
			userID:            userID,
			newSessionID:      newSessionID,
			revokedSessionIDs: append([]string(nil), revokedSessionIDs...),
		})
	})
	service.now = func() time.Time {
		return testService.nowValue
	}
	service.newID = func() (string, error) {
		testService.nextID++
		return fmt.Sprintf("id-%d", testService.nextID), nil
	}
	service.newToken = func() (string, error) {
		testService.nextToken++
		switch testService.nextToken {
		case 1:
			return "token-register", nil
		case 2:
			return "token-login", nil
		default:
			return fmt.Sprintf("token-extra-%d", testService.nextToken), nil
		}
	}

	testService.AuthService = service
	return testService
}

func assertActiveSessionCount(t *testing.T, service *testAuthService, userID string, want int64) {
	t.Helper()

	var got int64
	if err := service.dao.Model(&AuthSession{}).
		Where("user_id = ? AND revoked_at IS NULL", userID).
		Count(&got).Error; err != nil {
		t.Fatalf("count active sessions: %v", err)
	}
	if got != want {
		t.Fatalf("active sessions = %d, want %d", got, want)
	}
}

func (s *testAuthService) mustAuthenticateErr(ctx context.Context, token string) error {
	_, err := s.Authenticate(ctx, token)
	return err
}

func assertAuthError(t *testing.T, err error, wantStatus int, wantCode, wantMessage string) {
	t.Helper()

	var authErr *AuthError
	if !errors.As(err, &authErr) {
		t.Fatalf("error = %v, want *AuthError", err)
	}
	if authErr.Status != wantStatus {
		t.Fatalf("status = %d, want %d", authErr.Status, wantStatus)
	}
	if authErr.Code != wantCode {
		t.Fatalf("code = %q, want %q", authErr.Code, wantCode)
	}
	if authErr.Message != wantMessage {
		t.Fatalf("message = %q, want %q", authErr.Message, wantMessage)
	}
	if !IsAuthCode(err, wantCode) {
		t.Fatalf("IsAuthCode(%q) = false, want true", wantCode)
	}
}
