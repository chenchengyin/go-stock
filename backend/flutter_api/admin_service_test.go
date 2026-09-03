package flutter_api

import (
	"context"
	"errors"
	"testing"
	"time"

	"gorm.io/gorm"
)

func TestAdminServiceMissingAccountsStillRunDummyPasswordComparison(t *testing.T) {
	auth := newTestAuthService(t)
	service := newTestAdminService(auth.dao)
	var comparedHashes []string
	service.comparePassword = func(hash, _ []byte) error {
		comparedHashes = append(comparedHashes, string(hash))
		return errors.New("invalid password")
	}

	for _, username := range []string{"", "missing-account"} {
		if _, _, err := service.Login(context.Background(), AdminLoginInput{
			Username: username, Password: "secret123",
		}); !IsAuthCode(err, "INVALID_CREDENTIALS") {
			t.Fatalf("login username %q err = %v, want INVALID_CREDENTIALS", username, err)
		}
	}
	if len(comparedHashes) != 2 {
		t.Fatalf("password comparisons = %d, want 2", len(comparedHashes))
	}
	if comparedHashes[0] != adminDummyPasswordHash || comparedHashes[1] != adminDummyPasswordHash {
		t.Fatalf("compared hashes = %q, want fixed dummy hash", comparedHashes)
	}
}

func TestAdminServiceLoginRejectsMissingWrongAndOrdinaryCredentialsIdentically(t *testing.T) {
	auth := newTestAuthService(t)
	if _, err := auth.Register(context.Background(), RegisterInput{
		Phone: "13800000000", Password: "secret123", Nickname: "User",
	}); err != nil {
		t.Fatalf("register ordinary user: %v", err)
	}
	if err := CreateAdmin(context.Background(), auth.dao, AdminInitInput{
		Account: "13900000000", Password: "secret123",
	}); err != nil {
		t.Fatalf("create admin: %v", err)
	}
	service := newTestAdminService(auth.dao)

	for _, input := range []AdminLoginInput{
		{Username: "missing", Password: "secret123"},
		{Username: "13900000000", Password: "wrong"},
		{Username: "13800000000", Password: "secret123"},
	} {
		if _, _, err := service.Login(context.Background(), input); !IsAuthCode(err, "INVALID_CREDENTIALS") {
			t.Fatalf("login with %+v err = %v, want INVALID_CREDENTIALS", input, err)
		}
	}
}

func TestAdminServiceStoresOnlyTokenHashAndRejectsExpiredOrDisabledSessions(t *testing.T) {
	auth := newTestAuthService(t)
	if err := CreateAdmin(context.Background(), auth.dao, AdminInitInput{
		Account: "13900000000", Password: "secret123",
	}); err != nil {
		t.Fatalf("create admin: %v", err)
	}
	service := newTestAdminService(auth.dao)
	token, response, err := service.Login(context.Background(), AdminLoginInput{
		Username: "13900000000", Password: "secret123",
	})
	if err != nil {
		t.Fatalf("login: %v", err)
	}
	if token != "admin-token" || response.ExpiresAt != service.now().Add(adminSessionTTL) {
		t.Fatalf("login response = %q %+v", token, response)
	}
	var session AdminSession
	if err := auth.dao.First(&session, "token_hash = ?", hashToken(token)).Error; err != nil {
		t.Fatalf("load hashed session: %v", err)
	}
	if session.TokenHash == token {
		t.Fatal("raw token was persisted")
	}

	service.nowValue = service.nowValue.Add(adminSessionTTL + time.Second)
	if _, err := service.Authenticate(context.Background(), token); !IsAuthCode(err, "ADMIN_UNAUTHENTICATED") {
		t.Fatalf("expired session error = %v", err)
	}

	service.nowValue = service.nowValue.Add(-adminSessionTTL - time.Second)
	token, _, err = service.Login(context.Background(), AdminLoginInput{Username: "13900000000", Password: "secret123"})
	if err != nil {
		t.Fatalf("second login: %v", err)
	}
	if err := auth.dao.Model(&AuthUser{}).Where("phone = ?", "13900000000").Update("status", authStatusDisabled).Error; err != nil {
		t.Fatalf("disable admin: %v", err)
	}
	if _, err := service.Authenticate(context.Background(), token); !IsAuthCode(err, "ADMIN_UNAUTHENTICATED") {
		t.Fatalf("disabled session error = %v", err)
	}
}

func TestAdminServiceDisableRevokesOrdinaryUserSessionsAndNotifies(t *testing.T) {
	auth := newTestAuthService(t)
	registered, err := auth.Register(context.Background(), RegisterInput{
		Phone: "13800000000", Password: "secret123", Nickname: "A", DeviceID: "device-a",
	})
	if err != nil {
		t.Fatalf("register: %v", err)
	}
	if _, err := newTestAdminService(auth.dao).UpdateUserStatus(context.Background(), registered.User.ID, authStatusActive); err != nil {
		t.Fatalf("enable: %v", err)
	}
	if _, err := auth.Login(context.Background(), LoginInput{Phone: "13800000000", Password: "secret123", DeviceID: "device-a"}); err != nil {
		t.Fatalf("login: %v", err)
	}

	var callbackUserID string
	var callbackSessionIDs []string
	service := NewAdminService(auth.dao, func(userID string, sessionIDs []string) {
		callbackUserID = userID
		callbackSessionIDs = append([]string(nil), sessionIDs...)
	})
	if _, err := service.UpdateUserStatus(context.Background(), registered.User.ID, authStatusDisabled); err != nil {
		t.Fatalf("disable: %v", err)
	}
	if callbackUserID != registered.User.ID || len(callbackSessionIDs) != 1 || callbackSessionIDs[0] != "id-2" {
		t.Fatalf("callback = %q %v, want %q [id-2]", callbackUserID, callbackSessionIDs, registered.User.ID)
	}
}

type testAdminService struct {
	*AdminService
	nowValue time.Time
}

func newTestAdminService(dao *gorm.DB) *testAdminService {
	testService := &testAdminService{nowValue: time.Date(2026, time.August, 31, 9, 0, 0, 0, time.UTC)}
	service := NewAdminService(dao, nil)
	service.now = func() time.Time { return testService.nowValue }
	nextSession := 0
	service.newID = func() (string, error) {
		nextSession++
		if nextSession == 1 {
			return "admin-session-id", nil
		}
		return "admin-session-id-" + string(rune('0'+nextSession)), nil
	}
	service.newToken = func() (string, error) {
		if nextSession == 0 {
			return "admin-token", nil
		}
		return "admin-token-" + string(rune('0'+nextSession+1)), nil
	}
	testService.AdminService = service
	return testService
}
