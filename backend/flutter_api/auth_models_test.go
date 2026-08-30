package flutter_api

import (
	"testing"

	"github.com/glebarez/sqlite"
	"gorm.io/gorm"
)

func newAuthTestDB(t *testing.T) *gorm.DB {
	t.Helper()

	dao, err := gorm.Open(sqlite.Open("file::memory:?cache=shared"), &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}

	sqlDB, err := dao.DB()
	if err != nil {
		t.Fatalf("get sql db: %v", err)
	}

	t.Cleanup(func() {
		_ = sqlDB.Close()
	})

	return dao
}

func TestMigrateAuthTablesCreatesUsersAndSessions(t *testing.T) {
	dao := newAuthTestDB(t)

	if dao.Migrator().HasTable(&AuthUser{}) {
		t.Fatal("users should not exist before migration")
	}

	if err := MigrateAuthTables(dao); err != nil {
		t.Fatalf("migrate auth tables: %v", err)
	}

	if !dao.Migrator().HasTable(&AuthUser{}) || !dao.Migrator().HasTable(&AuthSession{}) {
		t.Fatal("auth tables were not created")
	}

	if !dao.Migrator().HasIndex(&AuthUser{}, "idx_auth_users_phone") {
		t.Fatal("phone unique index was not created")
	}

	if !dao.Migrator().HasIndex(&AuthSession{}, "idx_auth_sessions_token_hash") {
		t.Fatal("token hash index was not created")
	}
}
