package flutter_api

import "time"

type ModuleAccessMode string

const (
	ModuleAccessPublic    ModuleAccessMode = "public"
	ModuleAccessAllowlist ModuleAccessMode = "user_allowlist"
)

type ModuleDefinition struct {
	Code       string           `json:"code"`
	Name       string           `json:"name"`
	Client     string           `json:"client"`
	Placement  string           `json:"placement"`
	ParentCode *string          `json:"parentCode"`
	Sort       int              `json:"sort"`
	AccessMode ModuleAccessMode `json:"accessMode"`
}

type ModuleUserGrant struct {
	ID         string    `gorm:"primaryKey;size:64" json:"id"`
	ModuleCode string    `gorm:"size:128;not null;uniqueIndex:idx_module_user_grants_unique" json:"moduleCode"`
	UserID     string    `gorm:"size:64;not null;uniqueIndex:idx_module_user_grants_unique;index:idx_module_user_grants_user_id" json:"userId"`
	CreatedBy  string    `gorm:"size:64;not null" json:"createdBy"`
	CreatedAt  time.Time `json:"createdAt"`
	UpdatedAt  time.Time `json:"updatedAt"`
}

func (ModuleUserGrant) TableName() string { return "module_user_grants" }
