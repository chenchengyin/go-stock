package flutter_api

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"gorm.io/gorm"
)

const moduleAdminAccount = "module-admin-13900000001"

func newTestModuleService(t *testing.T) *ModuleService {
	t.Helper()

	dao := newAuthTestDB(t)
	if err := MigrateAuthTables(dao); err != nil {
		t.Fatalf("migrate auth tables: %v", err)
	}
	return NewModuleService(dao)
}

func createActiveModuleUser(t *testing.T, dao *gorm.DB, account string) *AuthUser {
	t.Helper()

	now := time.Date(2026, time.September, 3, 9, 0, 0, 0, time.UTC)
	phoneSuffix := 0
	for _, ch := range account {
		phoneSuffix += int(ch)
	}
	user := &AuthUser{
		ID:           account,
		Phone:        fmt.Sprintf("138%08d", phoneSuffix),
		PasswordHash: "unused",
		Nickname:     account,
		Role:         authRoleUser,
		Status:       authStatusActive,
		CreatedAt:    now,
		UpdatedAt:    now,
	}
	if err := dao.Create(user).Error; err != nil {
		t.Fatalf("create user %s: %v", account, err)
	}
	return user
}

func seedDatabaseAdmin(t *testing.T, dao *gorm.DB) *AuthUser {
	t.Helper()

	if err := CreateAdmin(context.Background(), dao, AdminInitInput{
		Account: moduleAdminAccount, Nickname: "模块管理员", Password: "secret123",
	}); err != nil {
		t.Fatalf("create admin: %v", err)
	}
	var admin AuthUser
	if err := dao.Where("phone = ?", moduleAdminAccount).First(&admin).Error; err != nil {
		t.Fatalf("load admin: %v", err)
	}
	return &admin
}

func loginDatabaseAdminForTest(t *testing.T, handler http.Handler) http.Cookie {
	t.Helper()

	req := httptest.NewRequest(http.MethodPost, "/api/admin/login",
		strings.NewReader(`{"username":"`+moduleAdminAccount+`","password":"secret123"}`))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("admin login status = %d, body = %s", rec.Code, rec.Body.String())
	}
	cookies := rec.Result().Cookies()
	if len(cookies) != 1 || cookies[0].Name != adminSessionCookieName || cookies[0].Value == "" {
		t.Fatalf("admin login cookies = %+v", cookies)
	}
	return *cookies[0]
}

func newAdminCookieRequest(method, path string, cookie http.Cookie, body io.Reader) *http.Request {
	req := httptest.NewRequest(method, path, body)
	req.AddCookie(&cookie)
	return req
}

func seedModuleAdminUser(t *testing.T, dao *gorm.DB, id string) *AuthUser {
	t.Helper()

	now := time.Date(2026, time.September, 3, 9, 0, 0, 0, time.UTC)
	user := &AuthUser{
		ID:           id,
		Phone:        "13700000000",
		PasswordHash: "unused",
		Nickname:     "Admin",
		Role:         authRoleAdmin,
		Status:       authStatusActive,
		CreatedAt:    now,
		UpdatedAt:    now,
	}
	if err := dao.Create(user).Error; err != nil {
		t.Fatalf("create admin user: %v", err)
	}
	return user
}
