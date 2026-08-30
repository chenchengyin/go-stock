package flutter_api

import (
	"context"
	"fmt"
	"strings"
	"testing"
	"time"

	"go-stock/backend/data"

	"github.com/glebarez/sqlite"
	"gorm.io/gorm"
)

func newUserDataTestDB(t *testing.T) *gorm.DB {
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

func mustHaveIndex(t *testing.T, dao *gorm.DB, model any, indexName string) {
	t.Helper()

	if !dao.Migrator().HasIndex(model, indexName) {
		t.Fatalf("index %s was not created", indexName)
	}
}

func createLegacyUserOwnedTables(t *testing.T, dao *gorm.DB) {
	t.Helper()

	statements := []string{
		`CREATE TABLE followed_stock (
			stock_code TEXT,
			name TEXT,
			sort INTEGER,
			time DATETIME,
			is_del INTEGER DEFAULT 0
		);`,
		`CREATE TABLE stock_groups (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			created_at DATETIME,
			updated_at DATETIME,
			deleted_at DATETIME,
			name TEXT,
			sort INTEGER
		);`,
		`CREATE TABLE group_stock_info (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			created_at DATETIME,
			updated_at DATETIME,
			deleted_at DATETIME,
			stock_code TEXT,
			group_id INTEGER
		);`,
		`CREATE TABLE trading_records (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			stock_code TEXT,
			stock_name TEXT,
			direction TEXT,
			price REAL,
			volume INTEGER,
			trading_time DATETIME,
			created_at DATETIME,
			updated_at DATETIME
		);`,
		`INSERT INTO followed_stock (stock_code, name, sort, time, is_del) VALUES ('sh600000', 'legacy', 1, '2026-08-30 09:30:00', 0);`,
		`INSERT INTO stock_groups (name, sort, created_at, updated_at) VALUES ('legacy-group', 1, '2026-08-30 09:30:00', '2026-08-30 09:30:00');`,
		`INSERT INTO group_stock_info (stock_code, group_id, created_at, updated_at) VALUES ('sh600000', 1, '2026-08-30 09:30:00', '2026-08-30 09:30:00');`,
		`INSERT INTO trading_records (stock_code, stock_name, direction, price, volume, trading_time, created_at, updated_at) VALUES ('sh600000', 'legacy', '买入', 10, 100, '2026-08-30 09:30:00', '2026-08-30 09:30:00', '2026-08-30 09:30:00');`,
	}

	for _, stmt := range statements {
		if err := dao.Exec(stmt).Error; err != nil {
			t.Fatalf("exec legacy schema statement failed: %v", err)
		}
	}
}

func countRows(t *testing.T, dao *gorm.DB, table string) int64 {
	t.Helper()

	var count int64
	if err := dao.Table(table).Count(&count).Error; err != nil {
		t.Fatalf("count %s: %v", table, err)
	}
	return count
}

func stubUserDataQuoteFetcher(t *testing.T, fn func(stockCodes ...string) (*[]data.StockInfo, error)) {
	t.Helper()

	old := userDataQuoteFetcher
	userDataQuoteFetcher = fn
	t.Cleanup(func() {
		userDataQuoteFetcher = old
	})
}

func TestUserDataServiceDoesNotCrossUserBoundary(t *testing.T) {
	dao := newUserDataTestDB(t)
	if err := MigrateUserOwnedData(dao); err != nil {
		t.Fatalf("migrate user data: %v", err)
	}

	userA := "user-a"
	userB := "user-b"
	legacy := data.FollowedStock{StockCode: "sh600000", Name: "legacy"}
	ownedA := data.FollowedStock{UserID: &userA, StockCode: "sh600000", Name: "A", Time: time.Now()}
	ownedB := data.FollowedStock{UserID: &userB, StockCode: "sh600000", Name: "B", Time: time.Now()}
	if err := dao.Create(&legacy).Error; err != nil {
		t.Fatal(err)
	}
	if err := dao.Create(&ownedA).Error; err != nil {
		t.Fatal(err)
	}
	if err := dao.Create(&ownedB).Error; err != nil {
		t.Fatal(err)
	}

	service := NewUserDataService(dao)

	itemsA, err := service.ListFollowedStocks(context.Background(), userA, 0)
	if err != nil {
		t.Fatalf("list user A followed stocks: %v", err)
	}
	if len(itemsA) != 1 || itemsA[0].Name != "A" {
		t.Fatalf("user A items = %+v", itemsA)
	}

	itemsB, err := service.ListFollowedStocks(context.Background(), userB, 0)
	if err != nil {
		t.Fatalf("list user B followed stocks: %v", err)
	}
	if len(itemsB) != 1 || itemsB[0].Name != "B" {
		t.Fatalf("user B items = %+v", itemsB)
	}
}

func TestUserDataServiceScopesGroupQueriesToOwner(t *testing.T) {
	dao := newUserDataTestDB(t)
	if err := MigrateUserOwnedData(dao); err != nil {
		t.Fatalf("migrate user data: %v", err)
	}

	userA := "user-a"
	userB := "user-b"
	groupA := data.Group{Name: "A", Sort: 1, UserID: &userA}
	groupB := data.Group{Name: "B", Sort: 1, UserID: &userB}
	stockA := data.FollowedStock{UserID: &userA, StockCode: "sz000001", Name: "PingAn", Time: time.Now()}
	stockB := data.FollowedStock{UserID: &userB, StockCode: "sz000001", Name: "Other", Time: time.Now()}
	if err := dao.Create(&groupA).Error; err != nil {
		t.Fatal(err)
	}
	if err := dao.Create(&groupB).Error; err != nil {
		t.Fatal(err)
	}
	if err := dao.Create(&stockA).Error; err != nil {
		t.Fatal(err)
	}
	if err := dao.Create(&stockB).Error; err != nil {
		t.Fatal(err)
	}
	if err := dao.Create(&data.GroupStock{UserID: &userA, GroupId: int(groupA.ID), StockCode: stockA.StockCode}).Error; err != nil {
		t.Fatal(err)
	}
	if err := dao.Create(&data.GroupStock{UserID: &userB, GroupId: int(groupB.ID), StockCode: stockB.StockCode}).Error; err != nil {
		t.Fatal(err)
	}

	service := NewUserDataService(dao)

	ownsA, err := service.OwnsGroup(context.Background(), userA, groupA.ID)
	if err != nil {
		t.Fatalf("check owner for group A: %v", err)
	}
	if !ownsA {
		t.Fatal("user A should own group A")
	}

	ownsB, err := service.OwnsGroup(context.Background(), userA, groupB.ID)
	if err != nil {
		t.Fatalf("check owner for group B: %v", err)
	}
	if ownsB {
		t.Fatal("user A must not own group B")
	}

	items, err := service.ListFollowedStocks(context.Background(), userA, groupA.ID)
	if err != nil {
		t.Fatalf("list grouped followed stocks: %v", err)
	}
	if len(items) != 1 || items[0].Name != "PingAn" {
		t.Fatalf("grouped items = %+v", items)
	}

	items, err = service.ListFollowedStocks(context.Background(), userA, groupB.ID)
	if err != nil {
		t.Fatalf("list other user's group: %v", err)
	}
	if len(items) != 0 {
		t.Fatalf("other user's group should be invisible, got %+v", items)
	}
}

func TestUserDataServiceListGroupsAndMembershipsAreScopedToOwner(t *testing.T) {
	dao := newUserDataTestDB(t)
	if err := MigrateUserOwnedData(dao); err != nil {
		t.Fatalf("migrate user data: %v", err)
	}

	userA := "user-a"
	userB := "user-b"
	groupA := data.Group{Name: "A", Sort: 1, UserID: &userA}
	groupB := data.Group{Name: "B", Sort: 1, UserID: &userB}
	legacyGroup := data.Group{Name: "legacy", Sort: 2}
	for _, group := range []*data.Group{&groupA, &groupB, &legacyGroup} {
		if err := dao.Create(group).Error; err != nil {
			t.Fatalf("create group %q: %v", group.Name, err)
		}
	}

	memberships := []data.GroupStock{
		{UserID: &userA, GroupId: int(groupA.ID), StockCode: "sz000001"},
		{UserID: &userB, GroupId: int(groupA.ID), StockCode: "sh600000"},
		{GroupId: int(groupA.ID), StockCode: "sz000002"},
		{UserID: &userB, GroupId: int(groupB.ID), StockCode: "sz000003"},
	}
	for _, membership := range memberships {
		if err := dao.Create(&membership).Error; err != nil {
			t.Fatalf("create group membership %q: %v", membership.StockCode, err)
		}
	}

	service := NewUserDataService(dao)
	groups, err := service.ListGroups(context.Background(), userA)
	if err != nil {
		t.Fatalf("list user A groups: %v", err)
	}
	if len(groups) != 1 || groups[0].Name != "A" {
		t.Fatalf("user A groups = %+v, want only group A", groups)
	}

	groupStocks, err := service.ListGroupStocks(context.Background(), userA, groupA.ID)
	if err != nil {
		t.Fatalf("list user A group stocks: %v", err)
	}
	if len(groupStocks) != 1 || groupStocks[0].StockCode != "sz000001" {
		t.Fatalf("user A group stocks = %+v, want only owned membership", groupStocks)
	}

	foreignGroupStocks, err := service.ListGroupStocks(context.Background(), userA, groupB.ID)
	if err != nil {
		t.Fatalf("list foreign group stocks: %v", err)
	}
	if len(foreignGroupStocks) != 0 {
		t.Fatalf("foreign group stocks = %+v, want empty", foreignGroupStocks)
	}
}

func TestUserDataServiceListTradingRecordsIsScopedToOwner(t *testing.T) {
	dao := newUserDataTestDB(t)
	if err := MigrateUserOwnedData(dao); err != nil {
		t.Fatalf("migrate user data: %v", err)
	}

	userA := "user-a"
	userB := "user-b"
	fixedTime := time.Date(2026, time.August, 30, 10, 0, 0, 0, time.UTC)
	records := []data.TradingRecord{
		{UserID: &userA, StockCode: "sz000001", StockName: "A", TradingTime: fixedTime},
		{UserID: &userB, StockCode: "sh600000", StockName: "B", TradingTime: fixedTime},
		{StockCode: "sz000002", StockName: "legacy", TradingTime: fixedTime},
	}
	for _, record := range records {
		if err := dao.Create(&record).Error; err != nil {
			t.Fatalf("create trading record %q: %v", record.StockName, err)
		}
	}

	service := NewUserDataService(dao)
	userRecords, err := service.ListTradingRecords(context.Background(), userA)
	if err != nil {
		t.Fatalf("list user A trading records: %v", err)
	}
	if len(userRecords) != 1 || userRecords[0].StockName != "A" {
		t.Fatalf("user A trading records = %+v, want only A", userRecords)
	}
}

func TestUserDataServiceUnfollowDoesNotDeleteOtherUsersRecord(t *testing.T) {
	dao := newUserDataTestDB(t)
	if err := MigrateUserOwnedData(dao); err != nil {
		t.Fatalf("migrate user data: %v", err)
	}

	userA := "user-a"
	userB := "user-b"
	if err := dao.Create(&data.FollowedStock{UserID: &userA, StockCode: "sh600000", Name: "A", Time: time.Now()}).Error; err != nil {
		t.Fatal(err)
	}
	if err := dao.Create(&data.FollowedStock{UserID: &userB, StockCode: "sh600000", Name: "B", Time: time.Now()}).Error; err != nil {
		t.Fatal(err)
	}

	service := NewUserDataService(dao)

	result, err := service.Unfollow(context.Background(), userA, "sh600000")
	if err != nil {
		t.Fatalf("unfollow current user stock: %v", err)
	}
	if result != "取消关注成功" {
		t.Fatalf("unexpected unfollow result: %s", result)
	}

	itemsA, err := service.ListFollowedStocks(context.Background(), userA, 0)
	if err != nil {
		t.Fatalf("list user A after unfollow: %v", err)
	}
	if len(itemsA) != 0 {
		t.Fatalf("user A should have no followed stocks, got %+v", itemsA)
	}

	itemsB, err := service.ListFollowedStocks(context.Background(), userB, 0)
	if err != nil {
		t.Fatalf("list user B after user A unfollow: %v", err)
	}
	if len(itemsB) != 1 || itemsB[0].Name != "B" {
		t.Fatalf("user B items = %+v", itemsB)
	}

	result, err = service.Unfollow(context.Background(), userA, "sh600000")
	if err != nil {
		t.Fatalf("unfollow missing current user stock: %v", err)
	}
	if result != "未找到当前用户记录" {
		t.Fatalf("unexpected missing-record result: %s", result)
	}

	itemsB, err = service.ListFollowedStocks(context.Background(), userB, 0)
	if err != nil {
		t.Fatalf("list user B after second unfollow: %v", err)
	}
	if len(itemsB) != 1 {
		t.Fatalf("user B record should remain, got %+v", itemsB)
	}
}

func TestUserDataServiceEnforcesUserAndStockUniqueness(t *testing.T) {
	dao := newUserDataTestDB(t)
	if err := MigrateUserOwnedData(dao); err != nil {
		t.Fatalf("migrate user data: %v", err)
	}

	userA := "user-a"
	userB := "user-b"
	if err := dao.Create(&data.FollowedStock{StockCode: "sh600000", Name: "legacy"}).Error; err != nil {
		t.Fatal(err)
	}
	if err := dao.Create(&data.FollowedStock{UserID: &userA, StockCode: "sh600000", Name: "A"}).Error; err != nil {
		t.Fatal(err)
	}
	if err := dao.Create(&data.FollowedStock{UserID: &userB, StockCode: "sh600000", Name: "B"}).Error; err != nil {
		t.Fatal(err)
	}

	err := dao.Create(&data.FollowedStock{UserID: &userA, StockCode: "sh600000", Name: "A2"}).Error
	if err == nil {
		t.Fatal("expected duplicate current-user stock create to fail")
	}
	if !strings.Contains(err.Error(), "UNIQUE") && !strings.Contains(err.Error(), "idx_followed_stock_user_code") {
		t.Fatalf("expected unique index error, got %v", err)
	}
}

func TestUserDataServiceFollowScopesUniquenessPerUser(t *testing.T) {
	dao := newUserDataTestDB(t)
	if err := MigrateUserOwnedData(dao); err != nil {
		t.Fatalf("migrate user data: %v", err)
	}

	stubUserDataQuoteFetcher(t, func(stockCodes ...string) (*[]data.StockInfo, error) {
		return &[]data.StockInfo{{
			Code:  stockCodes[0],
			Name:  "PingAn",
			Price: "12.34",
		}}, nil
	})

	service := NewUserDataService(dao)
	userA := "user-a"
	userB := "user-b"

	result, err := service.Follow(context.Background(), userA, "sz000001")
	if err != nil {
		t.Fatalf("follow current user stock: %v", err)
	}
	if result != "关注成功" {
		t.Fatalf("unexpected follow result: %s", result)
	}

	result, err = service.Follow(context.Background(), userA, "sz000001")
	if err != nil {
		t.Fatalf("follow duplicate current user stock: %v", err)
	}
	if result != "已经关注了" {
		t.Fatalf("unexpected duplicate follow result: %s", result)
	}

	result, err = service.Follow(context.Background(), userB, "sz000001")
	if err != nil {
		t.Fatalf("follow same stock for different user: %v", err)
	}
	if result != "关注成功" {
		t.Fatalf("unexpected second-user follow result: %s", result)
	}

	itemsA, err := service.ListFollowedStocks(context.Background(), userA, 0)
	if err != nil {
		t.Fatalf("list user A followed stocks: %v", err)
	}
	itemsB, err := service.ListFollowedStocks(context.Background(), userB, 0)
	if err != nil {
		t.Fatalf("list user B followed stocks: %v", err)
	}
	if len(itemsA) != 1 || len(itemsB) != 1 {
		t.Fatalf("expected one visible record per user, got A=%+v B=%+v", itemsA, itemsB)
	}
}

func TestUserDataServiceFollowRestoresSoftDeletedCurrentUserRecord(t *testing.T) {
	dao := newUserDataTestDB(t)
	if err := MigrateUserOwnedData(dao); err != nil {
		t.Fatalf("migrate user data: %v", err)
	}

	stubUserDataQuoteFetcher(t, func(stockCodes ...string) (*[]data.StockInfo, error) {
		return &[]data.StockInfo{{
			Code:  stockCodes[0],
			Name:  "ReFollow",
			Price: "9.87",
		}}, nil
	})

	service := NewUserDataService(dao)
	userA := "user-a"

	if err := dao.Create(&data.FollowedStock{UserID: &userA, StockCode: "sh600000", Name: "old"}).Error; err != nil {
		t.Fatal(err)
	}
	if err := dao.Where("user_id = ? AND stock_code = ?", userA, "sh600000").Delete(&data.FollowedStock{}).Error; err != nil {
		t.Fatalf("soft delete followed stock: %v", err)
	}

	result, err := service.Follow(context.Background(), userA, "sh600000")
	if err != nil {
		t.Fatalf("refollow stock: %v", err)
	}
	if result != "关注成功" {
		t.Fatalf("unexpected refollow result: %s", result)
	}

	items, err := service.ListFollowedStocks(context.Background(), userA, 0)
	if err != nil {
		t.Fatalf("list user followed stocks after refollow: %v", err)
	}
	if len(items) != 1 || items[0].Name != "ReFollow" {
		t.Fatalf("unexpected refollowed items: %+v", items)
	}

	var total int64
	if err := dao.Unscoped().Model(&data.FollowedStock{}).Where("user_id = ? AND stock_code = ?", userA, "sh600000").Count(&total).Error; err != nil {
		t.Fatalf("count unscoped followed stocks: %v", err)
	}
	if total != 1 {
		t.Fatalf("expected refollow to restore existing row, got %d rows", total)
	}
}

func TestUserDataServiceMigrationPreservesLegacyRowsAndIndexes(t *testing.T) {
	dao := newUserDataTestDB(t)
	createLegacyUserOwnedTables(t, dao)

	beforeCounts := map[string]int64{
		"followed_stock":   countRows(t, dao, "followed_stock"),
		"stock_groups":     countRows(t, dao, "stock_groups"),
		"group_stock_info": countRows(t, dao, "group_stock_info"),
		"trading_records":  countRows(t, dao, "trading_records"),
	}

	if err := MigrateUserOwnedData(dao); err != nil {
		t.Fatalf("migrate user data: %v", err)
	}
	if err := MigrateUserOwnedData(dao); err != nil {
		t.Fatalf("second migrate user data: %v", err)
	}

	for table, want := range beforeCounts {
		if got := countRows(t, dao, table); got != want {
			t.Fatalf("%s rows changed after migration: got %d want %d", table, got, want)
		}
	}

	mustHaveColumns(t, dao, "followed_stock", []string{"stock_code", "user_id"})
	mustHaveColumns(t, dao, "stock_groups", []string{"id", "user_id"})
	mustHaveColumns(t, dao, "group_stock_info", []string{"id", "user_id"})
	mustHaveColumns(t, dao, "trading_records", []string{"id", "user_id"})

	mustHaveFilteredIndex(t, dao, "idx_followed_stock_user_code", "WHERE user_id IS NOT NULL")
	mustHaveIndex(t, dao, &data.Group{}, "idx_stock_groups_user_sort")
	mustHaveIndex(t, dao, &data.GroupStock{}, "idx_group_stock_group_code")
	mustHaveIndex(t, dao, &data.TradingRecord{}, "idx_trading_records_user_time")
}

func TestUserDataServiceMigrateAuthTablesAlsoMigratesOwnedData(t *testing.T) {
	dao := newUserDataTestDB(t)

	if err := MigrateAuthTables(dao); err != nil {
		t.Fatalf("migrate auth tables: %v", err)
	}

	mustHaveColumns(t, dao, "users", []string{"id"})
	mustHaveColumns(t, dao, "followed_stock", []string{"stock_code", "user_id"})
	mustHaveColumns(t, dao, "stock_groups", []string{"id", "user_id"})
}
