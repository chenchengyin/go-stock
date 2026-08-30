package flutter_api

import (
	"database/sql"
	"fmt"
	"strings"
	"testing"

	"github.com/glebarez/sqlite"
	"gorm.io/gorm"
)

func newAuthTestDB(t *testing.T) *gorm.DB {
	t.Helper()

	dsn := fmt.Sprintf("file:%s?mode=memory&cache=shared", strings.ReplaceAll(t.Name(), "/", "_"))
	dao, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{})
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

func mustHaveColumns(t *testing.T, dao *gorm.DB, table string, want []string) {
	t.Helper()

	cols, err := dao.Migrator().ColumnTypes(table)
	if err != nil {
		t.Fatalf("column types for %s: %v", table, err)
	}

	got := make(map[string]bool, len(cols))
	for _, col := range cols {
		name := col.Name()
		got[name] = true
	}

	for _, column := range want {
		if !got[column] {
			t.Fatalf("missing column %s on table %s", column, table)
		}
	}
}

func mustHaveFilteredIndex(t *testing.T, dao *gorm.DB, indexName, wantWhere string) {
	t.Helper()

	var sqlText sql.NullString
	if err := dao.Raw(`SELECT sql FROM sqlite_master WHERE type = 'index' AND name = ?`, indexName).Scan(&sqlText).Error; err != nil {
		t.Fatalf("query index %s: %v", indexName, err)
	}
	if !sqlText.Valid {
		t.Fatalf("index %s was not created", indexName)
	}
	if !strings.Contains(sqlText.String, wantWhere) {
		t.Fatalf("index %s did not include filter %q: %s", indexName, wantWhere, sqlText.String)
	}
}

func TestMigrateAuthTablesCreatesUsersAndSessions(t *testing.T) {
	dao := newAuthTestDB(t)

	if dao.Migrator().HasTable(&AuthUser{}) {
		t.Fatal("users should not exist before migration")
	}

	if err := MigrateAuthTables(dao); err != nil {
		t.Fatalf("migrate auth tables: %v", err)
	}

	if err := MigrateAuthTables(dao); err != nil {
		t.Fatalf("second migrate auth tables: %v", err)
	}

	if !dao.Migrator().HasTable(&AuthUser{}) || !dao.Migrator().HasTable(&AuthSession{}) {
		t.Fatal("auth tables were not created")
	}

	mustHaveColumns(t, dao, "users", []string{
		"id",
		"phone",
		"password_hash",
		"nickname",
		"role",
		"status",
		"created_at",
		"updated_at",
	})
	mustHaveColumns(t, dao, "user_sessions", []string{
		"id",
		"user_id",
		"token_hash",
		"device_id",
		"created_at",
		"last_seen_at",
		"expires_at",
		"revoked_at",
		"revoke_reason",
	})

	if !dao.Migrator().HasIndex(&AuthUser{}, "idx_auth_users_phone") {
		t.Fatal("phone unique index was not created")
	}

	if !dao.Migrator().HasIndex(&AuthSession{}, "idx_auth_sessions_token_hash") {
		t.Fatal("token hash index was not created")
	}

	mustHaveFilteredIndex(t, dao, "idx_auth_sessions_one_active", "WHERE revoked_at IS NULL")
}
