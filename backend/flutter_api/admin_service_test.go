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
