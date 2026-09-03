package flutter_api

import (
	"context"
	"testing"

	"golang.org/x/crypto/bcrypt"
)

func TestCreateAdminHashesPasswordAndDoesNotOverwriteExistingAccount(t *testing.T) {
	service := newTestAuthService(t)
	input := AdminInitInput{
		Account: "13900000000", Nickname: "管理员一", Password: "secret123",
	}
	if err := CreateAdmin(context.Background(), service.dao, input); err != nil {
		t.Fatalf("create admin: %v", err)
	}
	var user AuthUser
	if err := service.dao.First(&user, "phone = ?", input.Account).Error; err != nil {
		t.Fatalf("load admin: %v", err)
	}
	if user.Role != "admin" ||
		bcrypt.CompareHashAndPassword([]byte(user.PasswordHash),
			[]byte(input.Password)) != nil {
		t.Fatalf("invalid admin record: %+v", user)
	}
	oldHash := user.PasswordHash
	if err := CreateAdmin(context.Background(), service.dao, input); !IsAuthCode(err, "ACCOUNT_EXISTS") {
		t.Fatalf("duplicate error = %v", err)
	}
	if err := service.dao.First(&user, "phone = ?", input.Account).Error; err != nil || user.PasswordHash != oldHash {
		t.Fatalf("duplicate changed password: %v", err)
	}
}

func TestAdminSessionSurvivesServiceRecreationAndLogoutRevokesIt(t *testing.T) {
	service := newTestAuthService(t)
	err := CreateAdmin(context.Background(), service.dao, AdminInitInput{
		Account: "13900000000", Nickname: "管理员", Password: "secret123",
	})
	if err != nil {
		t.Fatalf("create admin: %v", err)
	}
	admin := NewAdminService(service.dao, nil)
	token, _, err := admin.Login(context.Background(),
		AdminLoginInput{Username: "13900000000", Password: "secret123"})
	if err != nil {
		t.Fatalf("login admin: %v", err)
	}
	admin = NewAdminService(service.dao, nil)
	if _, err := admin.Authenticate(context.Background(), token); err != nil {
		t.Fatalf("session did not persist: %v", err)
	}
	if err := admin.Logout(context.Background(), token); err != nil {
		t.Fatalf("logout: %v", err)
	}
	if _, err := admin.Authenticate(context.Background(), token); !IsAuthCode(err, "ADMIN_UNAUTHENTICATED") {
		t.Fatalf("revoked session error = %v", err)
	}
}
