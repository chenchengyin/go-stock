package flutter_api

import (
	"context"
	"errors"
	"net/http"
	"strings"
	"time"

	"golang.org/x/crypto/bcrypt"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

const (
	adminSessionTTL  = 12 * time.Hour
	adminRoleManaged = authRoleUser
)

type AdminLoginInput struct {
	Username string `json:"username"`
	Password string `json:"password"`
}

type AdminSessionResponse struct {
	ExpiresAt time.Time `json:"expiresAt"`
}

type AdminPrincipal struct {
	UserID    string
	SessionID string
}

type AdminUser struct {
	ID        string    `json:"id"`
	Phone     string    `json:"phone"`
	Nickname  string    `json:"nickname"`
	Role      string    `json:"role"`
	Status    string    `json:"status"`
	CreatedAt time.Time `json:"createdAt"`
	UpdatedAt time.Time `json:"updatedAt"`
}

type AdminUserList struct {
	Items []AdminUser `json:"items"`
	Total int64       `json:"total"`
}

type AdminService struct {
	dao               *gorm.DB
	onSessionsRevoked func(userID string, sessionIDs []string)
	now               func() time.Time
	newID             func() (string, error)
	newToken          func() (string, error)
	comparePassword   func(hash, password []byte) error
}

func NewAdminService(dao *gorm.DB, onSessionsRevoked func(userID string, sessionIDs []string)) *AdminService {
	return &AdminService{dao: dao, onSessionsRevoked: onSessionsRevoked, now: time.Now, newID: newSecureHexID, newToken: newSecureHexToken, comparePassword: bcrypt.CompareHashAndPassword}
}

func (s *AdminService) Login(ctx context.Context, input AdminLoginInput) (string, *AdminSessionResponse, error) {
	invalid := func() (string, *AdminSessionResponse, error) {
		return "", nil, newAuthError(http.StatusUnauthorized, "INVALID_CREDENTIALS", "管理员账号或密码错误")
	}
	username := strings.TrimSpace(input.Username)
	var user AuthUser
	passwordHash := adminDummyPasswordHash
	userFound := false
	if username != "" {
		if err := s.dao.WithContext(ctx).First(&user, "phone = ?", username).Error; err != nil {
			if !errors.Is(err, gorm.ErrRecordNotFound) {
				return "", nil, err
			}
		} else {
			passwordHash = user.PasswordHash
			userFound = true
		}
	}
	passwordErr := s.comparePassword([]byte(passwordHash), []byte(input.Password))
	if !userFound || passwordErr != nil || user.Role != authRoleAdmin || user.Status != authStatusActive {
		return invalid()
	}
	rawToken, err := s.newToken()
	if err != nil {
		return "", nil, err
	}
	sessionID, err := s.newID()
	if err != nil {
		return "", nil, err
	}
	now := s.now().UTC()
	session := AdminSession{ID: sessionID, UserID: user.ID, TokenHash: hashToken(rawToken), CreatedAt: now, LastSeenAt: now, ExpiresAt: now.Add(adminSessionTTL)}
	if err := s.dao.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var current AuthUser
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).Where("id = ? AND role = ? AND status = ?", user.ID, authRoleAdmin, authStatusActive).First(&current).Error; err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return newAuthError(http.StatusUnauthorized, "INVALID_CREDENTIALS", "管理员账号或密码错误")
			}
			return err
		}
		return tx.Create(&session).Error
	}); err != nil {
		return "", nil, err
	}
	return rawToken, &AdminSessionResponse{ExpiresAt: session.ExpiresAt}, nil
}

func (s *AdminService) Authenticate(ctx context.Context, rawToken string) (AdminPrincipal, error) {
	invalid := func() (AdminPrincipal, error) {
		return AdminPrincipal{}, newAuthError(http.StatusUnauthorized, "ADMIN_UNAUTHENTICATED", "管理员未登录")
	}
	token := strings.TrimSpace(rawToken)
	if token == "" {
		return invalid()
	}
	var session AdminSession
	if err := s.dao.WithContext(ctx).First(&session, "token_hash = ?", hashToken(token)).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return invalid()
		}
		return AdminPrincipal{}, err
	}
	if session.RevokedAt != nil {
		return invalid()
	}
	now := s.now().UTC()
	if !session.ExpiresAt.After(now) {
		if err := s.dao.WithContext(ctx).Model(&AdminSession{}).Where("id = ? AND revoked_at IS NULL", session.ID).Update("revoked_at", now).Error; err != nil {
			return AdminPrincipal{}, err
		}
		return invalid()
	}
	var user AuthUser
	if err := s.dao.WithContext(ctx).First(&user, "id = ?", session.UserID).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return invalid()
		}
		return AdminPrincipal{}, err
	}
	if user.Role != authRoleAdmin || user.Status != authStatusActive {
		return invalid()
	}
	if err := s.dao.WithContext(ctx).Model(&AdminSession{}).Where("id = ? AND revoked_at IS NULL", session.ID).Update("last_seen_at", now).Error; err != nil {
		return AdminPrincipal{}, err
	}
	return AdminPrincipal{UserID: session.UserID, SessionID: session.ID}, nil
}

func (s *AdminService) Logout(ctx context.Context, rawToken string) error {
	token := strings.TrimSpace(rawToken)
	if token == "" {
		return nil
	}
	return s.dao.WithContext(ctx).Model(&AdminSession{}).Where("token_hash = ? AND revoked_at IS NULL", hashToken(token)).Update("revoked_at", s.now().UTC()).Error
}

func (s *AdminService) ListUsers(ctx context.Context, keyword string) (*AdminUserList, error) {
	query := s.dao.WithContext(ctx).Model(&AuthUser{}).Where("role = ?", adminRoleManaged)
	keyword = strings.TrimSpace(keyword)
	if keyword != "" {
		like := "%" + keyword + "%"
		query = query.Where("phone LIKE ? OR nickname LIKE ?", like, like)
	}
	var total int64
	if err := query.Count(&total).Error; err != nil {
		return nil, err
	}
	var users []AuthUser
	if err := query.Order("created_at DESC").Find(&users).Error; err != nil {
		return nil, err
	}
	items := make([]AdminUser, 0, len(users))
	for _, user := range users {
		items = append(items, toAdminUser(user))
	}
	return &AdminUserList{Items: items, Total: total}, nil
}

func (s *AdminService) UpdateUserStatus(ctx context.Context, userID, status string) (*AdminUser, error) {
	status = strings.TrimSpace(status)
	if status != authStatusActive && status != authStatusDisabled {
		return nil, newAuthError(http.StatusBadRequest, "INVALID_ARGUMENT", "用户状态不正确")
	}
	var user AuthUser
	var revokedSessionIDs []string
	now := s.now().UTC()
	err := s.dao.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.Where("id = ? AND role = ?", userID, adminRoleManaged).First(&user).Error; err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return newAuthError(http.StatusNotFound, "USER_NOT_FOUND", "用户不存在")
			}
			return err
		}
		if err := tx.Model(&AuthUser{}).Where("id = ?", userID).Updates(map[string]any{"status": status, "updated_at": now}).Error; err != nil {
			return err
		}
		if status == authStatusDisabled {
			var err error
			revokedSessionIDs, err = revokeActiveSessionsTx(tx, userID, now, "admin_disabled")
			if err != nil {
				return err
			}
		}
		user.Status = status
		user.UpdatedAt = now
		return nil
	})
	if err != nil {
		return nil, err
	}
	if len(revokedSessionIDs) > 0 && s.onSessionsRevoked != nil {
		s.onSessionsRevoked(user.ID, revokedSessionIDs)
	}
	result := toAdminUser(user)
	return &result, nil
}

func toAdminUser(user AuthUser) AdminUser {
	return AdminUser{ID: user.ID, Phone: user.Phone, Nickname: user.Nickname, Role: user.Role, Status: user.Status, CreatedAt: user.CreatedAt, UpdatedAt: user.UpdatedAt}
}
