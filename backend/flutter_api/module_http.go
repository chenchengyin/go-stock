package flutter_api

import (
	"errors"
	"net/http"
	"strings"

	"gorm.io/gorm"
)

type moduleSummary struct {
	ModuleDefinition
	AuthorizedUserCount int64 `json:"authorizedUserCount"`
}

type moduleAccessResponse struct {
	Users []moduleAccessSnapshot `json:"users"`
}

type moduleAccessSnapshot struct {
	UserID      string   `json:"userId"`
	ModuleCodes []string `json:"moduleCodes"`
}

func handleAuthModules(moduleService *ModuleService) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}
		principal, ok := PrincipalFromContext(r.Context())
		if !ok {
			WriteAuthError(w, newAuthError(http.StatusUnauthorized, "UNAUTHENTICATED", "未认证"))
			return
		}
		modules, err := moduleService.ListVisibleModules(r.Context(), principal.UserID)
		if err != nil {
			WriteAuthError(w, err)
			return
		}
		WriteJSON(w, struct {
			Version int                `json:"version"`
			Modules []ModuleDefinition `json:"modules"`
		}{Version: 1, Modules: modules})
	}
}

func handleAdminModules(moduleService *ModuleService) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}

		var counts []struct {
			ModuleCode          string
			AuthorizedUserCount int64
		}
		if err := moduleService.dao.WithContext(r.Context()).Model(&ModuleUserGrant{}).
			Select("module_code, COUNT(*) AS authorized_user_count").
			Group("module_code").Scan(&counts).Error; err != nil {
			WriteAuthError(w, err)
			return
		}
		countByCode := make(map[string]int64, len(counts))
		for _, count := range counts {
			countByCode[count.ModuleCode] = count.AuthorizedUserCount
		}
		modules := RegisteredModules()
		items := make([]moduleSummary, 0, len(modules))
		for _, module := range modules {
			items = append(items, moduleSummary{ModuleDefinition: module, AuthorizedUserCount: countByCode[module.Code]})
		}
		WriteJSON(w, map[string]any{"modules": items})
	}
}

func handleAdminAccess(moduleService *ModuleService) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		switch r.Method {
		case http.MethodGet:
			handleAdminAccessList(moduleService, w, r)
		case http.MethodPut:
			handleAdminAccessReplace(moduleService, w, r)
		default:
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		}
	}
}

func handleAdminAccessList(moduleService *ModuleService, w http.ResponseWriter, r *http.Request) {
	userIDs, err := normalizeModuleRequestStrings(strings.Split(r.URL.Query().Get("user_ids"), ","), false)
	if err != nil {
		WriteAuthError(w, err)
		return
	}
	if err := validateModuleRequestUsers(r, moduleService, userIDs); err != nil {
		WriteAuthError(w, err)
		return
	}
	access, err := moduleService.ListUserAccess(r.Context(), userIDs)
	if err != nil {
		WriteAuthError(w, moduleHTTPArgumentError(err))
		return
	}
	users := make([]moduleAccessSnapshot, 0, len(access))
	for _, snapshot := range access {
		users = append(users, moduleAccessSnapshot{UserID: snapshot.UserID, ModuleCodes: snapshot.ModuleCodes})
	}
	WriteJSON(w, moduleAccessResponse{Users: users})
}

func handleAdminAccessReplace(moduleService *ModuleService, w http.ResponseWriter, r *http.Request) {
	var input struct {
		UserIDs     []string `json:"userIds"`
		ModuleCodes []string `json:"moduleCodes"`
	}
	if err := decodeJSONBody(r, &input); err != nil {
		WriteAuthError(w, err)
		return
	}
	userIDs, err := normalizeModuleRequestStrings(input.UserIDs, false)
	if err != nil {
		WriteAuthError(w, err)
		return
	}
	moduleCodes, err := normalizeModuleRequestStrings(input.ModuleCodes, true)
	if err != nil {
		WriteAuthError(w, err)
		return
	}
	if err := validateModuleRequestUsers(r, moduleService, userIDs); err != nil {
		WriteAuthError(w, err)
		return
	}
	for _, code := range moduleCodes {
		module, ok := FindModule(code)
		if !ok || module.AccessMode != ModuleAccessAllowlist {
			WriteAuthError(w, newAuthError(http.StatusBadRequest, "INVALID_ARGUMENT", "模块不存在"))
			return
		}
	}
	principal, ok := AdminPrincipalFromContext(r.Context())
	if !ok {
		WriteAuthError(w, newAuthError(http.StatusUnauthorized, "ADMIN_UNAUTHENTICATED", "管理员未登录"))
		return
	}
	if err := moduleService.ReplaceUserAccess(r.Context(), principal.UserID, userIDs, moduleCodes); err != nil {
		WriteAuthError(w, moduleHTTPArgumentError(err))
		return
	}
	WriteJSON(w, map[string]string{"status": "ok"})
}

func handleAdminModuleUsers(moduleService *ModuleService) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}
		const prefix = "/api/admin/modules/"
		parts := strings.Split(strings.Trim(strings.TrimPrefix(r.URL.Path, prefix), "/"), "/")
		if len(parts) != 2 || parts[0] == "" || parts[1] != "users" {
			http.NotFound(w, r)
			return
		}
		users, err := moduleService.ListModuleUsers(r.Context(), parts[0])
		if err != nil {
			WriteAuthError(w, moduleHTTPArgumentError(err))
			return
		}
		WriteJSON(w, users)
	}
}

func normalizeModuleRequestStrings(values []string, allowEmpty bool) ([]string, error) {
	if values == nil {
		return nil, newAuthError(http.StatusBadRequest, "INVALID_ARGUMENT", "请求参数不正确")
	}
	if len(values) == 0 {
		if allowEmpty {
			return []string{}, nil
		}
		return nil, newAuthError(http.StatusBadRequest, "INVALID_ARGUMENT", "用户不存在")
	}
	seen := make(map[string]bool, len(values))
	result := make([]string, 0, len(values))
	for _, value := range values {
		value = strings.TrimSpace(value)
		if value == "" || seen[value] {
			return nil, newAuthError(http.StatusBadRequest, "INVALID_ARGUMENT", "请求参数不正确")
		}
		seen[value] = true
		result = append(result, value)
	}
	return result, nil
}

func validateModuleRequestUsers(r *http.Request, moduleService *ModuleService, userIDs []string) error {
	for _, userID := range userIDs {
		if _, err := moduleService.mustGetOrdinaryUser(r.Context(), userID); err != nil {
			return moduleHTTPArgumentError(err)
		}
	}
	return nil
}

func moduleHTTPArgumentError(err error) error {
	var authErr *AuthError
	if errors.As(err, &authErr) && authErr.Code == "USER_NOT_FOUND" {
		return newAuthError(http.StatusBadRequest, "INVALID_ARGUMENT", "用户不存在")
	}
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return newAuthError(http.StatusBadRequest, "INVALID_ARGUMENT", "请求参数不正确")
	}
	return err
}
