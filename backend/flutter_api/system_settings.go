package flutter_api

import (
	"context"
	"net/http"
	"time"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

const singleDeviceLoginSettingKey = "single_device_login"

type SystemSetting struct {
	Key     string `gorm:"primaryKey;size:64"`
	Enabled bool   `gorm:"not null"`
}

type SystemSettings struct {
	SingleDeviceLogin bool `json:"singleDeviceLogin"`
}

func readSystemSettings(dao *gorm.DB) (SystemSettings, error) {
	var setting SystemSetting
	err := dao.First(&setting, "key = ?", singleDeviceLoginSettingKey).Error
	return SystemSettings{SingleDeviceLogin: setting.Enabled}, err
}

// Keep the database constraint and the login policy in the same transaction.
func applySingleDeviceLoginTx(tx *gorm.DB, enabled bool, now time.Time) ([]AuthSession, error) {
	if !enabled {
		return nil, tx.Exec(`DROP INDEX IF EXISTS idx_auth_sessions_one_active`).Error
	}
	var revoked []AuthSession
	if err := tx.Raw(`SELECT id, user_id FROM (
  SELECT id, user_id, ROW_NUMBER() OVER (PARTITION BY user_id ORDER BY created_at DESC, id DESC) AS position
  FROM user_sessions WHERE revoked_at IS NULL
 ) WHERE position > 1`).Scan(&revoked).Error; err != nil {
		return nil, err
	}
	if len(revoked) > 0 {
		ids := make([]string, 0, len(revoked))
		for _, session := range revoked {
			ids = append(ids, session.ID)
		}
		if err := tx.Model(&AuthSession{}).Where("id IN ?", ids).Updates(map[string]any{
			"revoked_at": now, "revoke_reason": "new_login",
		}).Error; err != nil {
			return nil, err
		}
	}
	err := tx.Exec(`CREATE UNIQUE INDEX IF NOT EXISTS idx_auth_sessions_one_active
  ON user_sessions(user_id) WHERE revoked_at IS NULL`).Error
	return revoked, err
}

func (s *AdminService) UpdateSystemSettings(ctx context.Context, settings SystemSettings) (*SystemSettings, error) {
	var revoked []AuthSession
	err := s.dao.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		setting := SystemSetting{Key: singleDeviceLoginSettingKey, Enabled: settings.SingleDeviceLogin}
		if err := tx.Clauses(clause.OnConflict{
			Columns: []clause.Column{{Name: "key"}}, DoUpdates: clause.AssignmentColumns([]string{"enabled"}),
		}).Create(&setting).Error; err != nil {
			return err
		}
		var err error
		revoked, err = applySingleDeviceLoginTx(tx, settings.SingleDeviceLogin, s.now().UTC())
		return err
	})
	if err != nil {
		return nil, err
	}
	if s.onSessionsRevoked != nil {
		byUser := make(map[string][]string)
		for _, session := range revoked {
			byUser[session.UserID] = append(byUser[session.UserID], session.ID)
		}
		for userID, ids := range byUser {
			s.onSessionsRevoked(userID, ids)
		}
	}
	return &settings, nil
}

func handleAdminSettings(service *AdminService) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		switch r.Method {
		case http.MethodGet:
			settings, err := readSystemSettings(service.dao.WithContext(r.Context()))
			if err != nil {
				WriteAuthError(w, err)
				return
			}
			WriteJSON(w, settings)
		case http.MethodPatch:
			var input struct {
				SingleDeviceLogin *bool `json:"singleDeviceLogin"`
			}
			if err := decodeJSONBody(r, &input); err != nil {
				WriteAuthError(w, err)
				return
			}
			if input.SingleDeviceLogin == nil {
				WriteAuthError(w, newAuthError(http.StatusBadRequest, "INVALID_ARGUMENT", "请提供单点登录开关状态"))
				return
			}
			settings, err := service.UpdateSystemSettings(r.Context(), SystemSettings{SingleDeviceLogin: *input.SingleDeviceLogin})
			if err != nil {
				WriteAuthError(w, err)
				return
			}
			WriteJSON(w, settings)
		default:
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		}
	}
}
