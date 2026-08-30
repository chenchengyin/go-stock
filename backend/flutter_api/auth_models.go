package flutter_api

import "time"

type AuthUser struct {
	ID           string    `gorm:"primaryKey;size:64" json:"id"`
	Phone        string    `gorm:"size:64;uniqueIndex:idx_auth_users_phone" json:"phone"`
	PasswordHash string    `gorm:"size:255;not null" json:"-"`
	Nickname     string    `gorm:"size:50;not null" json:"nickname"`
	Role         string    `gorm:"size:20;not null" json:"role"`
	Status       string    `gorm:"size:20;not null" json:"status"`
	CreatedAt    time.Time `json:"createdAt"`
	UpdatedAt    time.Time `json:"updatedAt"`
}

func (AuthUser) TableName() string { return "users" }

type AuthSession struct {
	ID           string     `gorm:"primaryKey;size:64" json:"id"`
	UserID       string     `gorm:"size:64;index;not null" json:"userId"`
	TokenHash    string     `gorm:"size:64;uniqueIndex:idx_auth_sessions_token_hash;not null" json:"-"`
	DeviceID     string     `gorm:"size:128;not null" json:"deviceId"`
	CreatedAt    time.Time  `json:"createdAt"`
	LastSeenAt   time.Time  `json:"lastSeenAt"`
	ExpiresAt    time.Time  `json:"expiresAt"`
	RevokedAt    *time.Time `json:"revokedAt,omitempty"`
	RevokeReason string     `gorm:"size:30" json:"-"`
}

func (AuthSession) TableName() string { return "user_sessions" }

type PublicUser struct {
	ID       string `json:"id"`
	Phone    string `json:"phone"`
	Nickname string `json:"nickname"`
	Role     string `json:"role"`
}

type RegisterInput struct {
	Phone    string `json:"phone"`
	Password string `json:"password"`
	Nickname string `json:"nickname"`
	DeviceID string `json:"deviceId"`
}

type LoginInput struct {
	Phone    string `json:"phone"`
	Password string `json:"password"`
	DeviceID string `json:"deviceId"`
}

type UpdateProfileInput struct {
	Nickname string `json:"nickname"`
}

type AuthPrincipal struct {
	UserID    string
	SessionID string
	DeviceID  string
}

type AuthSessionResponse struct {
	User        PublicUser `json:"user"`
	AccessToken string     `json:"accessToken"`
	ExpiresAt   time.Time  `json:"expiresAt"`
}

type AuthError struct {
	Code    string `json:"code"`
	Message string `json:"message"`
}
