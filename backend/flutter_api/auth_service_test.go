package flutter_api

import (
	"context"
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
	if !IsAuthCode(wrongPasswordErr, "INVALID_CREDENTIALS") {
		t.Fatalf("wrong password err = %v, want INVALID_CREDENTIALS", wrongPasswordErr)
	}

	_, missingUserErr := service.Login(context.Background(), LoginInput{
		Phone:    "13900000000",
		Password: "secret123",
		DeviceID: "device-b",
	})
	if !IsAuthCode(missingUserErr, "INVALID_CREDENTIALS") {
		t.Fatalf("missing user err = %v, want INVALID_CREDENTIALS", missingUserErr)
	}
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

	if _, err := service.Authenticate(context.Background(), first.AccessToken); !IsAuthCode(err, "SESSION_REPLACED") {
		t.Fatalf("old token error = %v, want SESSION_REPLACED", err)
	}

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
	if service.replacedCalls[0] != second.User.ID+":"+sessions[1].ID {
		t.Fatalf("replacement callback = %q, want %q", service.replacedCalls[0], second.User.ID+":"+sessions[1].ID)
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

func TestAuthServiceLogoutRevokesSessionIdempotently(t *testing.T) {
	service := newTestAuthService(t)

	session, err := service.Register(context.Background(), RegisterInput{
		Phone: "13800000000", Password: "secret123", Nickname: "A", DeviceID: "device-a",
	})
	if err != nil {
		t.Fatalf("register: %v", err)
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

	if _, err := service.Authenticate(context.Background(), session.AccessToken); !IsAuthCode(err, "SESSION_EXPIRED") {
		t.Fatalf("authenticate expired err = %v, want SESSION_EXPIRED", err)
	}

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
	replacedCalls []string
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

	service := NewAuthService(dao, func(userID, newSessionID string) {
		testService.replacedCalls = append(testService.replacedCalls, userID+":"+newSessionID)
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
