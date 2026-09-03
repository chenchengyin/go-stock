package flutter_api

import (
	"context"
	"fmt"
	"net/http"
	"testing"
	"time"

	"gorm.io/gorm"
)

func TestModuleServiceVisibleModulesUsesPublicAndDirectGrants(t *testing.T) {
	service := newTestModuleService(t)
	user := createActiveModuleUser(t, service.dao, "user-a")

	visible, err := service.ListVisibleModules(context.Background(), user.ID)
	if err != nil {
		t.Fatalf("list before grant: %v", err)
	}
	assertModuleCodes(t, visible,
		"radar.monitored", "radar.watch_changes", "radar.all_changes")

	if err := service.ReplaceUserAccess(context.Background(), "admin-a",
		[]string{user.ID}, []string{"radar.main_strategy"}); err != nil {
		t.Fatalf("grant main: %v", err)
	}

	visible, err = service.ListVisibleModules(context.Background(), user.ID)
	if err != nil {
		t.Fatalf("list after grant: %v", err)
	}
	assertModuleCodes(t, visible,
		"radar.monitored", "radar.main_strategy",
		"radar.watch_changes", "radar.all_changes")

	if ok, err := service.HasModuleAccess(context.Background(),
		user.ID, "radar.purple_strategy"); err != nil || ok {
		t.Fatalf("purple access = %v, err = %v, want false", ok, err)
	}
	if ok, err := service.HasModuleAccess(context.Background(),
		user.ID, "radar.watch_changes"); err != nil || !ok {
		t.Fatalf("watch changes access = %v, err = %v, want true", ok, err)
	}
	if _, err := service.HasModuleAccess(context.Background(),
		user.ID, "radar.not_registered"); !IsAuthCode(err, "INVALID_ARGUMENT") {
		t.Fatalf("unknown module error = %v, want INVALID_ARGUMENT", err)
	}
}

func TestModuleServiceBatchReplaceIsAtomicAndEmptyRevokes(t *testing.T) {
	service := newTestModuleService(t)
	a := createActiveModuleUser(t, service.dao, "user-a")
	b := createActiveModuleUser(t, service.dao, "user-b")

	if err := service.ReplaceUserAccess(context.Background(), "admin-a",
		[]string{a.ID, b.ID},
		[]string{"radar.purple_strategy", "radar.blue_strategy"}); err != nil {
		t.Fatalf("initial grant: %v", err)
	}
	if err := service.ReplaceUserAccess(context.Background(), "admin-a",
		[]string{a.ID, b.ID}, nil); err != nil {
		t.Fatalf("revoke: %v", err)
	}

	access, err := service.ListUserAccess(context.Background(), []string{a.ID, b.ID})
	if err != nil {
		t.Fatalf("read access: %v", err)
	}
	for _, item := range access {
		if len(item.ModuleCodes) != 0 {
			t.Fatalf("user %s still has grants: %#v", item.UserID, item.ModuleCodes)
		}
	}

	err = service.ReplaceUserAccess(context.Background(), "admin-a",
		[]string{a.ID, b.ID},
		[]string{"radar.main_strategy", "radar.not_registered"})
	if !IsAuthCode(err, "INVALID_ARGUMENT") {
		t.Fatalf("invalid batch error = %v, want INVALID_ARGUMENT", err)
	}
	access, err = service.ListUserAccess(context.Background(), []string{a.ID, b.ID})
	if err != nil {
		t.Fatalf("read after rollback: %v", err)
	}
	for _, item := range access {
		if len(item.ModuleCodes) != 0 {
			t.Fatalf("rollback left grants for user %s: %#v",
				item.UserID, item.ModuleCodes)
		}
	}
}

func TestModuleServiceListUserAccessKeepsIndependentModuleSets(t *testing.T) {
	service := newTestModuleService(t)
	a := createActiveModuleUser(t, service.dao, "user-a")
	b := createActiveModuleUser(t, service.dao, "user-b")
	c := createActiveModuleUser(t, service.dao, "user-c")

	if err := service.ReplaceUserAccess(context.Background(), "admin-a",
		[]string{" " + a.ID + " ", a.ID}, []string{"radar.purple_strategy", "radar.purple_strategy"}); err != nil {
		t.Fatalf("grant purple: %v", err)
	}
	if err := service.ReplaceUserAccess(context.Background(), "admin-a",
		[]string{b.ID}, []string{"radar.blue_strategy"}); err != nil {
		t.Fatalf("grant blue: %v", err)
	}

	access, err := service.ListUserAccess(context.Background(), []string{a.ID, b.ID, c.ID})
	if err != nil {
		t.Fatalf("list access: %v", err)
	}
	if len(access) != 3 {
		t.Fatalf("snapshot count = %d, want 3", len(access))
	}

	assertUserAccessSnapshot(t, access[0], a.ID, "radar.purple_strategy")
	assertUserAccessSnapshot(t, access[1], b.ID, "radar.blue_strategy")
	assertUserAccessSnapshot(t, access[2], c.ID)
}

func TestModuleServiceReplaceAndReverseLookupValidateTargets(t *testing.T) {
	service := newTestModuleService(t)
	userA := createActiveModuleUser(t, service.dao, "user-a")
	userB := createActiveModuleUser(t, service.dao, "user-b")

	if err := service.ReplaceUserAccess(context.Background(), "admin-a",
		[]string{userA.ID, userB.ID},
		[]string{"radar.main_strategy"}); err != nil {
		t.Fatalf("grant main: %v", err)
	}

	list, err := service.ListModuleUsers(context.Background(), "radar.main_strategy")
	if err != nil {
		t.Fatalf("list module users: %v", err)
	}
	if list.Total != 2 || len(list.Items) != 2 {
		t.Fatalf("module users = %+v, want total and items of 2", list)
	}
	if list.Items[0].ID != userA.ID || list.Items[1].ID != userB.ID {
		t.Fatalf("module users order = %+v", list.Items)
	}

	if err := service.ReplaceUserAccess(context.Background(), "admin-a",
		[]string{"   "}, []string{"radar.blue_strategy"}); !IsAuthCode(err, "INVALID_ARGUMENT") {
		t.Fatalf("blank user ids err = %v, want INVALID_ARGUMENT", err)
	}

	admin := seedModuleAdminUser(t, service.dao, "admin-record")
	if err := service.ReplaceUserAccess(context.Background(), "admin-a",
		[]string{admin.ID}, []string{"radar.blue_strategy"}); !IsAuthCode(err, "USER_NOT_FOUND") {
		t.Fatalf("admin target err = %v, want USER_NOT_FOUND", err)
	}

	if err := service.ReplaceUserAccess(context.Background(), "admin-a",
		[]string{userA.ID}, []string{"radar.watch_changes"}); !IsAuthCode(err, "INVALID_ARGUMENT") {
		t.Fatalf("public module err = %v, want INVALID_ARGUMENT", err)
	}

	if _, err := service.ListModuleUsers(context.Background(), "radar.watch_changes"); !IsAuthCode(err, "INVALID_ARGUMENT") {
		t.Fatalf("public reverse lookup err = %v, want INVALID_ARGUMENT", err)
	}
	if _, err := service.ListModuleUsers(context.Background(), "radar.not_registered"); !IsAuthCode(err, "INVALID_ARGUMENT") {
		t.Fatalf("unknown reverse lookup err = %v, want INVALID_ARGUMENT", err)
	}
}

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

func seedModuleAdminUser(t *testing.T, dao *gorm.DB, id string) *AuthUser {
	t.Helper()

	now := time.Date(2026, time.September, 3, 9, 0, 0, 0, time.UTC)
	user := &AuthUser{
		ID:           id,
		Phone:        "13700000000",
		PasswordHash: "unused",
		Nickname:     "Admin",
		Role:         "admin",
		Status:       authStatusActive,
		CreatedAt:    now,
		UpdatedAt:    now,
	}
	if err := dao.Create(user).Error; err != nil {
		t.Fatalf("create admin user: %v", err)
	}
	return user
}

func assertModuleCodes(t *testing.T, got []ModuleDefinition, want ...string) {
	t.Helper()

	if len(got) != len(want) {
		t.Fatalf("module count = %d, want %d", len(got), len(want))
	}
	for i, module := range got {
		if module.Code != want[i] {
			t.Fatalf("module[%d] = %q, want %q", i, module.Code, want[i])
		}
	}
}

func assertUserAccessSnapshot(t *testing.T, got ModuleAccessSnapshot, wantUserID string, wantCodes ...string) {
	t.Helper()

	if got.UserID != wantUserID {
		t.Fatalf("snapshot user = %q, want %q", got.UserID, wantUserID)
	}
	if len(got.ModuleCodes) != len(wantCodes) {
		t.Fatalf("snapshot codes = %#v, want %#v", got.ModuleCodes, wantCodes)
	}
	for i, code := range wantCodes {
		if got.ModuleCodes[i] != code {
			t.Fatalf("snapshot code[%d] = %q, want %q", i, got.ModuleCodes[i], code)
		}
	}
}

func TestModuleServiceErrorContractDocumentsModuleNotFound(t *testing.T) {
	service := newTestModuleService(t)
	user := createActiveModuleUser(t, service.dao, "user-a")

	_, err := service.HasModuleAccess(context.Background(), user.ID, "radar.unknown")
	assertAuthError(t, err, http.StatusBadRequest, "INVALID_ARGUMENT", "模块不存在")
}
