package flutter_api

import (
	"context"
	"errors"
	"net/http"
	"strings"
	"sync"
	"time"

	"gorm.io/gorm"
)

const (
	adminUsername    = "admin"
	adminPassword    = "admin"
	adminSessionTTL  = 12 * time.Hour
	adminRoleManaged = authRoleUser
)

type AdminLoginInput struct {
	Username string `json:"username"`
	Password string `json:"password"`
}

type AdminSessionResponse struct {
	AccessToken string    `json:"accessToken"`
	ExpiresAt   time.Time `json:"expiresAt"`
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

	mu       sync.RWMutex
	tokens   map[string]time.Time
	now      func() time.Time
	newToken func() (string, error)
}

func NewAdminService(dao *gorm.DB, onSessionsRevoked func(userID string, sessionIDs []string)) *AdminService {
	return &AdminService{
		dao:               dao,
		onSessionsRevoked: onSessionsRevoked,
		tokens:            make(map[string]time.Time),
		now:               time.Now,
		newToken:          newSecureHexToken,
	}
}

func (s *AdminService) Login(_ context.Context, input AdminLoginInput) (*AdminSessionResponse, error) {
	if strings.TrimSpace(input.Username) != adminUsername || input.Password != adminPassword {
		return nil, newAuthError(http.StatusUnauthorized, "INVALID_CREDENTIALS", "管理员账号或密码错误")
	}

	rawToken, err := s.newToken()
	if err != nil {
		return nil, err
	}

	now := s.now().UTC()
	expiresAt := now.Add(adminSessionTTL)
	s.mu.Lock()
	s.tokens[hashToken(rawToken)] = expiresAt
	s.mu.Unlock()

	return &AdminSessionResponse{
		AccessToken: rawToken,
		ExpiresAt:   expiresAt,
	}, nil
}

func (s *AdminService) Authenticate(rawToken string) error {
	rawToken = strings.TrimSpace(rawToken)
	if rawToken == "" {
		return newAuthError(http.StatusUnauthorized, "ADMIN_UNAUTHENTICATED", "管理员未登录")
	}

	tokenHash := hashToken(rawToken)
	now := s.now().UTC()
	s.mu.Lock()
	expiresAt, ok := s.tokens[tokenHash]
	if !ok || !now.Before(expiresAt) {
		delete(s.tokens, tokenHash)
		s.mu.Unlock()
		return newAuthError(http.StatusUnauthorized, "ADMIN_UNAUTHENTICATED", "管理员未登录")
	}
	s.mu.Unlock()
	return nil
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

		if err := tx.Model(&AuthUser{}).Where("id = ?", userID).Updates(map[string]any{
			"status":     status,
			"updated_at": now,
		}).Error; err != nil {
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
	return AdminUser{
		ID:        user.ID,
		Phone:     user.Phone,
		Nickname:  user.Nickname,
		Role:      user.Role,
		Status:    user.Status,
		CreatedAt: user.CreatedAt,
		UpdatedAt: user.UpdatedAt,
	}
}
