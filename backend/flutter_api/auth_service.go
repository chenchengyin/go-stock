package flutter_api

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"net/http"
	"strings"
	"time"

	"golang.org/x/crypto/bcrypt"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

const (
	authRoleUser       = "user"
	authStatusActive   = "active"
	authStatusDisabled = "disabled"
	authSessionTTL     = 30 * 24 * time.Hour
)

type AuthService struct {
	dao                    *gorm.DB
	onSessionsReplaced     func(userID, newSessionID string, revokedSessionIDs []string)
	now                    func() time.Time
	newID                  func() (string, error)
	newToken               func() (string, error)
	beforeLoginTransaction func()
}

func NewAuthService(dao *gorm.DB, onSessionsReplaced func(userID, newSessionID string, revokedSessionIDs []string)) *AuthService {
	return &AuthService{
		dao:                dao,
		onSessionsReplaced: onSessionsReplaced,
		now:                time.Now,
		newID:              newSecureHexID,
		newToken:           newSecureHexToken,
	}
}

func (e *AuthError) Error() string { return e.Message }

func IsAuthCode(err error, code string) bool {
	var authErr *AuthError
	return errors.As(err, &authErr) && authErr.Code == code
}

func (s *AuthService) Register(ctx context.Context, input RegisterInput) (*AuthSessionResponse, error) {
	phone := strings.TrimSpace(input.Phone)
	if len(phone) < 5 || len(input.Password) < 6 {
		return nil, newAuthError(http.StatusBadRequest, "INVALID_ARGUMENT", "账号或密码格式不正确")
	}
	deviceID := strings.TrimSpace(input.DeviceID)
	if deviceID == "" {
		return nil, newAuthError(http.StatusBadRequest, "DEVICE_REQUIRED", "缺少设备标识")
	}
	nickname := strings.TrimSpace(input.Nickname)
	if nickname == "" {
		nickname = phone
	}

	passwordHash, err := bcrypt.GenerateFromPassword([]byte(input.Password), bcrypt.DefaultCost)
	if err != nil {
		return nil, err
	}

	now := s.now().UTC()
	userID, err := s.newID()
	if err != nil {
		return nil, err
	}
	session, rawToken, err := s.buildSession(userID, deviceID, now)
	if err != nil {
		return nil, err
	}

	user := AuthUser{
		ID:           userID,
		Phone:        phone,
		PasswordHash: string(passwordHash),
		Nickname:     nickname,
		Role:         authRoleUser,
		Status:       authStatusActive,
		CreatedAt:    now,
		UpdatedAt:    now,
	}

	if err := s.dao.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.Create(&user).Error; err != nil {
			if isUniqueConstraintError(err) {
				return newAuthError(http.StatusConflict, "ACCOUNT_EXISTS", "账号已存在")
			}
			return err
		}

		if _, err := revokeActiveSessionsTx(tx, user.ID, now, "new_login"); err != nil {
			return err
		}

		return tx.Create(&session).Error
	}); err != nil {
		return nil, err
	}

	return &AuthSessionResponse{
		User:        toPublicUser(user),
		AccessToken: rawToken,
		ExpiresAt:   session.ExpiresAt,
	}, nil
}

func (s *AuthService) Login(ctx context.Context, input LoginInput) (*AuthSessionResponse, error) {
	phone := strings.TrimSpace(input.Phone)
	if len(phone) < 5 || len(input.Password) < 6 {
		return nil, newAuthError(http.StatusUnauthorized, "INVALID_CREDENTIALS", "账号或密码错误")
	}
	deviceID := strings.TrimSpace(input.DeviceID)
	if deviceID == "" {
		return nil, newAuthError(http.StatusBadRequest, "DEVICE_REQUIRED", "缺少设备标识")
	}

	var user AuthUser
	if err := s.dao.WithContext(ctx).First(&user, "phone = ?", phone).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, newAuthError(http.StatusUnauthorized, "INVALID_CREDENTIALS", "账号或密码错误")
		}
		return nil, err
	}
	if user.Status != authStatusActive {
		return nil, newAuthError(http.StatusForbidden, "ACCOUNT_DISABLED", "账号已禁用")
	}
	if err := bcrypt.CompareHashAndPassword([]byte(user.PasswordHash), []byte(input.Password)); err != nil {
		return nil, newAuthError(http.StatusUnauthorized, "INVALID_CREDENTIALS", "账号或密码错误")
	}

	now := s.now().UTC()
	session, rawToken, err := s.buildSession(user.ID, deviceID, now)
	if err != nil {
		return nil, err
	}

	if s.beforeLoginTransaction != nil {
		s.beforeLoginTransaction()
	}

	var revokedSessionIDs []string
	if err := s.dao.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var currentUser AuthUser
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).
			Where("id = ? AND status = ?", user.ID, authStatusActive).
			First(&currentUser).Error; err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return newAuthError(http.StatusForbidden, "ACCOUNT_DISABLED", "账号已禁用")
			}
			return err
		}
		user = currentUser

		var err error
		revokedSessionIDs, err = revokeActiveSessionsTx(tx, user.ID, now, "new_login")
		if err != nil {
			return err
		}
		return tx.Create(&session).Error
	}); err != nil {
		return nil, err
	}

	if s.onSessionsReplaced != nil {
		s.onSessionsReplaced(user.ID, session.ID, revokedSessionIDs)
	}

	return &AuthSessionResponse{
		User:        toPublicUser(user),
		AccessToken: rawToken,
		ExpiresAt:   session.ExpiresAt,
	}, nil
}

func (s *AuthService) Authenticate(ctx context.Context, rawToken string) (AuthPrincipal, error) {
	token := strings.TrimSpace(rawToken)
	if token == "" {
		return AuthPrincipal{}, newAuthError(http.StatusUnauthorized, "UNAUTHENTICATED", "未认证")
	}

	var session AuthSession
	if err := s.dao.WithContext(ctx).First(&session, "token_hash = ?", hashToken(token)).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return AuthPrincipal{}, newAuthError(http.StatusUnauthorized, "UNAUTHENTICATED", "未认证")
		}
		return AuthPrincipal{}, err
	}

	if session.RevokedAt != nil {
		if session.RevokeReason == "new_login" {
			return AuthPrincipal{}, newAuthError(http.StatusUnauthorized, "SESSION_REPLACED", "账号已在其他设备登录，请重新登录")
		}
		if session.RevokeReason == "expired" {
			return AuthPrincipal{}, newAuthError(http.StatusUnauthorized, "SESSION_EXPIRED", "会话已过期")
		}
		return AuthPrincipal{}, newAuthError(http.StatusUnauthorized, "UNAUTHENTICATED", "未认证")
	}

	now := s.now().UTC()
	if !session.ExpiresAt.After(now) {
		if err := s.dao.WithContext(ctx).Model(&AuthSession{}).
			Where("id = ? AND revoked_at IS NULL", session.ID).
			Updates(map[string]any{
				"revoked_at":    now,
				"revoke_reason": "expired",
			}).Error; err != nil {
			return AuthPrincipal{}, err
		}
		return AuthPrincipal{}, newAuthError(http.StatusUnauthorized, "SESSION_EXPIRED", "会话已过期")
	}

	var user AuthUser
	if err := s.dao.WithContext(ctx).First(&user, "id = ?", session.UserID).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return AuthPrincipal{}, newAuthError(http.StatusUnauthorized, "UNAUTHENTICATED", "未认证")
		}
		return AuthPrincipal{}, err
	}
	if user.Status != authStatusActive {
		return AuthPrincipal{}, newAuthError(http.StatusForbidden, "ACCOUNT_DISABLED", "账号已禁用")
	}

	if err := s.dao.WithContext(ctx).Model(&AuthSession{}).
		Where("id = ?", session.ID).
		Update("last_seen_at", now).Error; err != nil {
		return AuthPrincipal{}, err
	}

	return AuthPrincipal{
		UserID:    session.UserID,
		SessionID: session.ID,
		DeviceID:  session.DeviceID,
	}, nil
}

func (s *AuthService) Logout(ctx context.Context, rawToken string) error {
	token := strings.TrimSpace(rawToken)
	if token == "" {
		return nil
	}

	now := s.now().UTC()
	var revokedSession AuthSession
	if err := s.dao.WithContext(ctx).
		First(&revokedSession, "token_hash = ? AND revoked_at IS NULL", hashToken(token)).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil
		}
		return err
	}

	result := s.dao.WithContext(ctx).Model(&AuthSession{}).
		Where("id = ? AND revoked_at IS NULL", revokedSession.ID).
		Updates(map[string]any{
			"revoked_at":    now,
			"revoke_reason": "logout",
		})
	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected == 0 {
		return nil
	}

	if s.onSessionsReplaced != nil {
		s.onSessionsReplaced(revokedSession.UserID, "", []string{revokedSession.ID})
	}
	return nil
}

func (s *AuthService) UpdateProfile(ctx context.Context, principal AuthPrincipal, input UpdateProfileInput) (*PublicUser, error) {
	nickname := strings.TrimSpace(input.Nickname)
	if nickname == "" {
		return nil, newAuthError(http.StatusBadRequest, "INVALID_ARGUMENT", "昵称不能为空")
	}

	now := s.now().UTC()
	if err := s.dao.WithContext(ctx).Model(&AuthUser{}).
		Where("id = ?", principal.UserID).
		Updates(map[string]any{
			"nickname":   nickname,
			"updated_at": now,
		}).Error; err != nil {
		return nil, err
	}

	var user AuthUser
	if err := s.dao.WithContext(ctx).First(&user, "id = ?", principal.UserID).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, newAuthError(http.StatusUnauthorized, "UNAUTHENTICATED", "未认证")
		}
		return nil, err
	}

	publicUser := toPublicUser(user)
	return &publicUser, nil
}

func (s *AuthService) buildSession(userID, deviceID string, now time.Time) (AuthSession, string, error) {
	sessionID, err := s.newID()
	if err != nil {
		return AuthSession{}, "", err
	}
	rawToken, err := s.newToken()
	if err != nil {
		return AuthSession{}, "", err
	}

	session := AuthSession{
		ID:           sessionID,
		UserID:       userID,
		TokenHash:    hashToken(rawToken),
		DeviceID:     deviceID,
		CreatedAt:    now,
		LastSeenAt:   now,
		ExpiresAt:    now.Add(authSessionTTL),
		RevokeReason: "",
	}

	return session, rawToken, nil
}

func revokeActiveSessionsTx(tx *gorm.DB, userID string, now time.Time, reason string) ([]string, error) {
	var sessionIDs []string
	if err := tx.Model(&AuthSession{}).
		Where("user_id = ? AND revoked_at IS NULL", userID).
		Order("created_at ASC").
		Pluck("id", &sessionIDs).Error; err != nil {
		return nil, err
	}
	if len(sessionIDs) == 0 {
		return sessionIDs, nil
	}
	if err := tx.Model(&AuthSession{}).
		Where("user_id = ? AND revoked_at IS NULL", userID).
		Updates(map[string]any{
			"revoked_at":    now,
			"revoke_reason": reason,
		}).Error; err != nil {
		return nil, err
	}
	return sessionIDs, nil
}

func hashToken(rawToken string) string {
	sum := sha256.Sum256([]byte(rawToken))
	return hex.EncodeToString(sum[:])
}

func toPublicUser(user AuthUser) PublicUser {
	return PublicUser{
		ID:       user.ID,
		Phone:    user.Phone,
		Nickname: user.Nickname,
		Role:     user.Role,
	}
}

func newSecureHexID() (string, error) {
	return randomHex(16)
}

func newSecureHexToken() (string, error) {
	return randomHex(32)
}

func randomHex(size int) (string, error) {
	buf := make([]byte, size)
	if _, err := rand.Read(buf); err != nil {
		return "", err
	}
	return hex.EncodeToString(buf), nil
}

func newAuthError(status int, code, message string) error {
	return &AuthError{
		Status:  status,
		Code:    code,
		Message: message,
	}
}

func isUniqueConstraintError(err error) bool {
	if err == nil {
		return false
	}
	message := strings.ToLower(err.Error())
	return strings.Contains(message, "unique constraint failed") ||
		strings.Contains(message, "duplicate key") ||
		strings.Contains(message, "unique failed")
}
