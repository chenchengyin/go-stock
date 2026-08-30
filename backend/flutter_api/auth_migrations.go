package flutter_api

import (
	"go-stock/backend/data"

	"gorm.io/gorm"
)

func MigrateAuthTables(dao *gorm.DB) error {
	if err := dao.AutoMigrate(&AuthUser{}, &AuthSession{}); err != nil {
		return err
	}

	if err := dao.Exec(`
		CREATE UNIQUE INDEX IF NOT EXISTS idx_auth_sessions_one_active
		ON user_sessions(user_id)
		WHERE revoked_at IS NULL;
	`).Error; err != nil {
		return err
	}

	return MigrateUserOwnedData(dao)
}

func MigrateUserOwnedData(dao *gorm.DB) error {
	if err := dao.AutoMigrate(
		&data.FollowedStock{},
		&data.Group{},
		&data.GroupStock{},
		&data.TradingRecord{},
	); err != nil {
		return err
	}

	statements := []string{
		`CREATE UNIQUE INDEX IF NOT EXISTS idx_followed_stock_user_code
		ON followed_stock(user_id, stock_code)
		WHERE user_id IS NOT NULL;`,
		`CREATE INDEX IF NOT EXISTS idx_stock_groups_user_sort
		ON stock_groups(user_id, sort);`,
		`CREATE INDEX IF NOT EXISTS idx_group_stock_group_code
		ON group_stock_info(group_id, stock_code);`,
		`CREATE INDEX IF NOT EXISTS idx_trading_records_user_time
		ON trading_records(user_id, trading_time);`,
	}

	for _, stmt := range statements {
		if err := dao.Exec(stmt).Error; err != nil {
			return err
		}
	}

	return nil
}
