package flutter_api

import (
	"context"
	"errors"
	"net/http"
	"slices"
	"strings"
	"time"

	"gorm.io/gorm"
)

type ModuleAccessSnapshot struct {
	UserID      string
	ModuleCodes []string
}

type ModuleService struct {
	dao   *gorm.DB
	now   func() time.Time
	newID func() (string, error)
}

func NewModuleService(dao *gorm.DB) *ModuleService {
	return &ModuleService{
		dao:   dao,
		now:   time.Now,
		newID: newSecureHexID,
	}
}

func (s *ModuleService) ListVisibleModules(ctx context.Context, userID string) ([]ModuleDefinition, error) {
	grants, err := s.listGrantedCodes(ctx, []string{strings.TrimSpace(userID)})
	if err != nil {
		return nil, err
	}

	allowed := grants[strings.TrimSpace(userID)]
	modules := make([]ModuleDefinition, 0, len(registeredModules))
	for _, module := range RegisteredModules() {
		if module.AccessMode == ModuleAccessPublic || allowed[module.Code] {
			modules = append(modules, module)
		}
	}
	slices.SortFunc(modules, func(a, b ModuleDefinition) int {
		return a.Sort - b.Sort
	})
	return modules, nil
}

func (s *ModuleService) ListUserAccess(ctx context.Context, userIDs []string) ([]ModuleAccessSnapshot, error) {
	normalized := normalizeUniqueStrings(userIDs)
	if len(normalized) == 0 {
		return []ModuleAccessSnapshot{}, nil
	}

	grants, err := s.listGrantedCodes(ctx, normalized)
	if err != nil {
		return nil, err
	}

	snapshots := make([]ModuleAccessSnapshot, 0, len(normalized))
	for _, userID := range normalized {
		codes := make([]string, 0, len(grants[userID]))
		for _, module := range RegisteredModules() {
			if grants[userID][module.Code] {
				codes = append(codes, module.Code)
			}
		}
		snapshots = append(snapshots, ModuleAccessSnapshot{
			UserID:      userID,
			ModuleCodes: codes,
		})
	}
	return snapshots, nil
}

func (s *ModuleService) ReplaceUserAccess(ctx context.Context, adminID string, userIDs []string, moduleCodes []string) error {
	targetUserIDs := normalizeUniqueStrings(userIDs)
	if len(targetUserIDs) == 0 {
		return newAuthError(http.StatusBadRequest, "INVALID_ARGUMENT", "用户不存在")
	}

	controlledCodes, err := validateControlledModuleCodes(moduleCodes)
	if err != nil {
		return err
	}

	adminID = strings.TrimSpace(adminID)
	now := s.now().UTC()

	return s.dao.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := s.validateOrdinaryUsersTx(tx, targetUserIDs); err != nil {
			return err
		}

		if err := tx.Where("user_id IN ?", targetUserIDs).Delete(&ModuleUserGrant{}).Error; err != nil {
			return err
		}
		if len(controlledCodes) == 0 {
			return nil
		}

		grants := make([]ModuleUserGrant, 0, len(targetUserIDs)*len(controlledCodes))
		for _, userID := range targetUserIDs {
			for _, moduleCode := range controlledCodes {
				grantID, err := s.newID()
				if err != nil {
					return err
				}
				grants = append(grants, ModuleUserGrant{
					ID:         grantID,
					ModuleCode: moduleCode,
					UserID:     userID,
					CreatedBy:  adminID,
					CreatedAt:  now,
					UpdatedAt:  now,
				})
			}
		}
		return tx.Create(&grants).Error
	})
}

func (s *ModuleService) ListModuleUsers(ctx context.Context, moduleCode string) (*AdminUserList, error) {
	module, err := requireControlledModule(moduleCode)
	if err != nil {
		return nil, err
	}

	query := s.dao.WithContext(ctx).
		Model(&AuthUser{}).
		Joins("JOIN module_user_grants ON module_user_grants.user_id = users.id").
		Where("users.role = ? AND module_user_grants.module_code = ?", authRoleUser, module.Code)

	var total int64
	if err := query.Count(&total).Error; err != nil {
		return nil, err
	}

	var users []AuthUser
	if err := query.Order("users.created_at ASC").Order("users.id ASC").Find(&users).Error; err != nil {
		return nil, err
	}

	items := make([]AdminUser, 0, len(users))
	for _, user := range users {
		items = append(items, toAdminUser(user))
	}
	return &AdminUserList{Items: items, Total: total}, nil
}

func (s *ModuleService) HasModuleAccess(ctx context.Context, userID, moduleCode string) (bool, error) {
	module, ok := FindModule(strings.TrimSpace(moduleCode))
	if !ok {
		return false, newAuthError(http.StatusBadRequest, "INVALID_ARGUMENT", "模块不存在")
	}
	if module.AccessMode == ModuleAccessPublic {
		return true, nil
	}

	var count int64
	if err := s.dao.WithContext(ctx).Model(&ModuleUserGrant{}).
		Where("user_id = ? AND module_code = ?", strings.TrimSpace(userID), module.Code).
		Count(&count).Error; err != nil {
		return false, err
	}
	return count > 0, nil
}

func (s *ModuleService) validateOrdinaryUsersTx(tx *gorm.DB, userIDs []string) error {
	var count int64
	if err := tx.Model(&AuthUser{}).
		Where("role = ? AND id IN ?", authRoleUser, userIDs).
		Count(&count).Error; err != nil {
		return err
	}
	if count != int64(len(userIDs)) {
		return newAuthError(http.StatusNotFound, "USER_NOT_FOUND", "用户不存在")
	}
	return nil
}

func (s *ModuleService) listGrantedCodes(ctx context.Context, userIDs []string) (map[string]map[string]bool, error) {
	result := make(map[string]map[string]bool, len(userIDs))
	if len(userIDs) == 0 {
		return result, nil
	}
	for _, userID := range userIDs {
		result[userID] = map[string]bool{}
	}

	var grants []ModuleUserGrant
	if err := s.dao.WithContext(ctx).
		Where("user_id IN ?", userIDs).
		Find(&grants).Error; err != nil {
		return nil, err
	}
	for _, grant := range grants {
		codes, ok := result[grant.UserID]
		if !ok {
			continue
		}
		codes[grant.ModuleCode] = true
	}
	return result, nil
}

func normalizeUniqueStrings(items []string) []string {
	seen := make(map[string]bool, len(items))
	result := make([]string, 0, len(items))
	for _, item := range items {
		trimmed := strings.TrimSpace(item)
		if trimmed == "" || seen[trimmed] {
			continue
		}
		seen[trimmed] = true
		result = append(result, trimmed)
	}
	return result
}

func validateControlledModuleCodes(moduleCodes []string) ([]string, error) {
	normalized := normalizeUniqueStrings(moduleCodes)
	for _, code := range normalized {
		module, err := requireControlledModule(code)
		if err != nil {
			return nil, err
		}
		if module.AccessMode != ModuleAccessAllowlist {
			return nil, newAuthError(http.StatusBadRequest, "INVALID_ARGUMENT", "模块不存在")
		}
	}
	return normalized, nil
}

func requireControlledModule(moduleCode string) (ModuleDefinition, error) {
	module, ok := FindModule(strings.TrimSpace(moduleCode))
	if !ok || module.AccessMode == ModuleAccessPublic {
		return ModuleDefinition{}, newAuthError(http.StatusBadRequest, "INVALID_ARGUMENT", "模块不存在")
	}
	return module, nil
}

func (s *ModuleService) mustGetOrdinaryUser(ctx context.Context, userID string) (*AuthUser, error) {
	var user AuthUser
	if err := s.dao.WithContext(ctx).
		Where("id = ? AND role = ?", strings.TrimSpace(userID), authRoleUser).
		First(&user).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, newAuthError(http.StatusNotFound, "USER_NOT_FOUND", "用户不存在")
		}
		return nil, err
	}
	return &user, nil
}
