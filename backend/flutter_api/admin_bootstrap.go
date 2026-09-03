package flutter_api

import (
	"context"
	"net/http"
	"strings"
	"time"

	"golang.org/x/crypto/bcrypt"
	"gorm.io/gorm"
)

const authRoleAdmin = "admin"

const adminDummyPasswordHash = "$2a$10$N9qo8uLOickgx2ZMRZoMyeIjZAgcfl7p92ldGxad68LJZdL17lhWy"

type AdminInitInput struct {
	Account  string
	Nickname string
	Password string
}

func CreateAdmin(ctx context.Context, dao *gorm.DB, input AdminInitInput) error {
	account := strings.TrimSpace(input.Account)
	if account == "" || input.Password == "" {
		return newAuthError(http.StatusBadRequest, "INVALID_ARGUMENT", "管理员账号和密码不能为空")
	}
	nickname := strings.TrimSpace(input.Nickname)
	if nickname == "" {
		nickname = account
	}

	passwordHash, err := bcrypt.GenerateFromPassword([]byte(input.Password), bcrypt.DefaultCost)
	if err != nil {
		return err
	}
	id, err := newSecureHexID()
	if err != nil {
		return err
	}
	now := time.Now().UTC()
	user := AuthUser{ID: id, Phone: account, PasswordHash: string(passwordHash), Nickname: nickname, Role: authRoleAdmin, Status: authStatusActive, CreatedAt: now, UpdatedAt: now}
	return dao.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.Create(&user).Error; err != nil {
			if isUniqueConstraintError(err) {
				return newAuthError(http.StatusConflict, "ACCOUNT_EXISTS", "账号已存在")
			}
			return err
		}
		return nil
	})
}
