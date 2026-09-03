package flutter_api

import (
	"context"
	"testing"
	"time"

	"gorm.io/gorm"
)

func TestAdminServiceLoginAcceptsFixedCredentials(t *testing.T) {
	auth := newTestAuthService(t)
	service := newTestAdminService(auth.dao)

	got, err := service.Login(context.Background(), AdminLoginInput{
		Username: "admin",
		Password: "admin",
	})
	if err != nil {
		t.Fatalf("login: %v", err)
	}
	if got.AccessToken != "admin-token" {
		t.Fatalf("token = %q, want admin-token", got.AccessToken)
	}
	if got.ExpiresAt != service.now().Add(adminSessionTTL) {
		t.Fatalf("expires at = %v, want %v", got.ExpiresAt, service.now().Add(adminSessionTTL))
	}
	if err := service.Authenticate(got.AccessToken); err != nil {
		t.Fatalf("authenticate: %v", err)
	}
}

func TestAdminServiceLoginRejectsWrongCredentials(t *testing.T) {
	auth := newTestAuthService(t)
	service := newTestAdminService(auth.dao)

	for _, input := range []AdminLoginInput{
		{Username: "root", Password: "admin"},
		{Username: "admin", Password: "wrong"},
	} {
		if _, err := service.Login(context.Background(), input); !IsAuthCode(err, "INVALID_CREDENTIALS") {
			t.Fatalf("login with %+v err = %v, want INVALID_CREDENTIALS", input, err)
		}
	}
}

func TestAdminServiceAuthenticateRejectsExpiredToken(t *testing.T) {
	auth := newTestAuthService(t)
	service := newTestAdminService(auth.dao)

	if _, err := service.Login(context.Background(), AdminLoginInput{Username: "admin", Password: "admin"}); err != nil {
		t.Fatalf("login: %v", err)
	}
	service.nowValue = service.nowValue.Add(adminSessionTTL + time.Second)

	err := service.Authenticate("admin-token")
	assertAuthError(t, err, 401, "ADMIN_UNAUTHENTICATED", "管理员未登录")
}

func TestAdminServiceLoginPrunesExpiredTokensAndCapsActiveTokens(t *testing.T) {
	auth := newTestAuthService(t)
	service := NewAdminService(auth.dao, nil)
	nowValue := auth.nowValue
	service.now = func() time.Time {
		return nowValue
	}
	nextToken := 0
	service.newToken = func() (string, error) {
		nextToken++
		return "admin-token-" + string(rune('a'+nextToken)), nil
	}

	for i := 0; i < adminSessionLimit+1; i++ {
		if _, err := service.Login(context.Background(), AdminLoginInput{Username: "admin", Password: "admin"}); err != nil {
			t.Fatalf("login %d: %v", i, err)
		}
	}

	nowValue = nowValue.Add(adminSessionTTL + time.Second)
	if _, err := service.Login(context.Background(), AdminLoginInput{Username: "admin", Password: "admin"}); err != nil {
		t.Fatalf("login after expiration: %v", err)
	}

	service.mu.RLock()
	activeTokens := len(service.tokens)
	service.mu.RUnlock()
	if activeTokens != 1 {
		t.Fatalf("stored admin tokens = %d, want only the current token after pruning", activeTokens)
	}
}

func TestAdminServiceDisableRevokesSessionsAndNotifies(t *testing.T) {
	auth := newTestAuthService(t)
	registered, err := auth.Register(context.Background(), RegisterInput{
		Phone: "13800000000", Password: "secret123", Nickname: "A", DeviceID: "device-a",
	})
	if err != nil {
		t.Fatalf("register: %v", err)
	}
	if _, err := newTestAdminService(auth.dao).UpdateUserStatus(
		context.Background(),
		registered.User.ID,
		authStatusActive,
	); err != nil {
		t.Fatalf("enable: %v", err)
	}
	if _, err := auth.Login(context.Background(), LoginInput{
		Phone: "13800000000", Password: "secret123", DeviceID: "device-a",
	}); err != nil {
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

	if callbackUserID != registered.User.ID {
		t.Fatalf("callback user id = %q, want %q", callbackUserID, registered.User.ID)
	}
	if len(callbackSessionIDs) != 1 || callbackSessionIDs[0] != "id-2" {
		t.Fatalf("callback session ids = %v, want [id-2]", callbackSessionIDs)
	}
}

type testAdminService struct {
	*AdminService
	nowValue time.Time
}

func newTestAdminService(dao *gorm.DB) *testAdminService {
	testService := &testAdminService{
		nowValue: time.Date(2026, time.August, 31, 9, 0, 0, 0, time.UTC),
	}

	service := NewAdminService(dao, nil)
	service.now = func() time.Time {
		return testService.nowValue
	}
	service.newToken = func() (string, error) {
		return "admin-token", nil
	}

	testService.AdminService = service
	return testService
}
