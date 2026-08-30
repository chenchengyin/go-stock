package flutter_api

import "gorm.io/gorm"

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

	return nil
}
