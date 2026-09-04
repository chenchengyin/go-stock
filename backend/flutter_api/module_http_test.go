package flutter_api

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestHTTPHandlerConstructorsRequireModuleService(t *testing.T) {
	t.Helper()
	var _ func(*AuthService, *ModuleService) http.Handler = NewAuthHTTPHandler
	var _ func(*AdminService, *ModuleService) http.Handler = NewAdminHTTPHandler
}

func TestAuthModulesReturnsOnlyPublicModulesWithoutGrants(t *testing.T) {
	auth := newTestAuthService(t)
	user := authHTTPRegisterUser(t, auth, "13800000000", "Alice", "device-a")
	modules := NewModuleService(auth.dao)
	handler := RequireAuth(auth.AuthService,
		NewAuthHTTPHandler(auth.AuthService, modules))

	req := httptest.NewRequest(http.MethodGet, "/api/auth/modules", nil)
	req.Header.Set("Authorization", "Bearer "+user.AccessToken)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", rec.Code, rec.Body.String())
	}
	var body struct {
		Version int                `json:"version"`
		Modules []ModuleDefinition `json:"modules"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if body.Version != 1 ||
		!sameCodes(body.Modules,
			"radar.monitored", "radar.watch_changes", "radar.all_changes") {
		t.Fatalf("modules = %#v", body.Modules)
	}
}

func TestAuthModulesIncludesGrantedAllowlistMetadataWithoutBusinessResults(t *testing.T) {
	auth := newTestAuthService(t)
	user := authHTTPRegisterUser(t, auth, "13800000000", "Alice", "device-a")
	modules := NewModuleService(auth.dao)
	if err := modules.ReplaceUserAccess(context.Background(), "admin-a", []string{user.User.ID}, []string{"radar.main_strategy"}); err != nil {
		t.Fatalf("grant module: %v", err)
	}
	handler := RequireAuth(auth.AuthService, NewAuthHTTPHandler(auth.AuthService, modules))

	req := httptest.NewRequest(http.MethodGet, "/api/auth/modules", nil)
	req.Header.Set("Authorization", "Bearer "+user.AccessToken)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", rec.Code, rec.Body.String())
	}
	var body map[string]json.RawMessage
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if _, ok := body["results"]; ok {
		t.Fatalf("modules response leaked business results: %s", rec.Body.String())
	}
	var modulesBody struct {
		Modules []ModuleDefinition `json:"modules"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &modulesBody); err != nil {
		t.Fatalf("decode modules: %v", err)
	}
	if !sameCodes(modulesBody.Modules, "radar.monitored", "radar.main_strategy", "radar.watch_changes", "radar.all_changes") {
		t.Fatalf("modules = %#v", modulesBody.Modules)
	}
}

func TestAdminAccessBatchRejectsPublicModule(t *testing.T) {
	auth := newTestAuthService(t)
	seedDatabaseAdmin(t, auth.dao)
	user := createActiveModuleUser(t, auth.dao, "13800000000")
	handler := NewAdminHTTPHandler(
		NewAdminService(auth.dao, nil), NewModuleService(auth.dao))
	cookie := loginDatabaseAdminForTest(t, handler)

	req := newAdminCookieRequest(http.MethodPut, "/api/admin/access",
		cookie, strings.NewReader(
			"{\"userIds\":[\""+user.ID+
				"\"],\"moduleCodes\":[\"radar.monitored\"]}"))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	if rec.Code != http.StatusBadRequest ||
		!strings.Contains(rec.Body.String(), "INVALID_ARGUMENT") {
		t.Fatalf("response = %d %s", rec.Code, rec.Body.String())
	}
}

func TestAdminModuleHTTPListsModulesAccessAndGrantedUsers(t *testing.T) {
	auth := newTestAuthService(t)
	seedDatabaseAdmin(t, auth.dao)
	a := createActiveModuleUser(t, auth.dao, "user-a")
	b := createActiveModuleUser(t, auth.dao, "user-b")
	modules := NewModuleService(auth.dao)
	if err := modules.ReplaceUserAccess(context.Background(), "admin-a", []string{a.ID, b.ID}, []string{"radar.main_strategy"}); err != nil {
		t.Fatalf("grant main strategy: %v", err)
	}
	handler := NewAdminHTTPHandler(NewAdminService(auth.dao, nil), modules)
	cookie := loginDatabaseAdminForTest(t, handler)

	modulesReq := newAdminCookieRequest(http.MethodGet, "/api/admin/modules", cookie, nil)
	modulesRec := httptest.NewRecorder()
	handler.ServeHTTP(modulesRec, modulesReq)
	if modulesRec.Code != http.StatusOK {
		t.Fatalf("modules status = %d, body = %s", modulesRec.Code, modulesRec.Body.String())
	}
	var modulesBody struct {
		Modules []struct {
			Code                string `json:"code"`
			AccessMode          string `json:"accessMode"`
			AuthorizedUserCount int64  `json:"authorizedUserCount"`
		} `json:"modules"`
	}
	if err := json.Unmarshal(modulesRec.Body.Bytes(), &modulesBody); err != nil {
		t.Fatalf("decode modules: %v", err)
	}
	if len(modulesBody.Modules) != len(RegisteredModules()) || modulesBody.Modules[2].Code != "radar.main_strategy" || modulesBody.Modules[2].AuthorizedUserCount != 2 || modulesBody.Modules[0].AuthorizedUserCount != 0 {
		t.Fatalf("modules = %#v", modulesBody.Modules)
	}

	accessReq := newAdminCookieRequest(http.MethodGet, "/api/admin/access?user_ids="+a.ID+","+b.ID, cookie, nil)
	accessRec := httptest.NewRecorder()
	handler.ServeHTTP(accessRec, accessReq)
	if accessRec.Code != http.StatusOK {
		t.Fatalf("access status = %d, body = %s", accessRec.Code, accessRec.Body.String())
	}
	var accessBody struct {
		Users []struct {
			UserID      string   `json:"userId"`
			ModuleCodes []string `json:"moduleCodes"`
		} `json:"users"`
	}
	if err := json.Unmarshal(accessRec.Body.Bytes(), &accessBody); err != nil {
		t.Fatalf("decode access: %v", err)
	}
	if len(accessBody.Users) != 2 || accessBody.Users[0].UserID != a.ID || !sameStrings(accessBody.Users[0].ModuleCodes, "radar.main_strategy") || accessBody.Users[1].UserID != b.ID || !sameStrings(accessBody.Users[1].ModuleCodes, "radar.main_strategy") {
		t.Fatalf("access = %#v", accessBody.Users)
	}

	usersReq := newAdminCookieRequest(http.MethodGet, "/api/admin/modules/radar.main_strategy/users", cookie, nil)
	usersRec := httptest.NewRecorder()
	handler.ServeHTTP(usersRec, usersReq)
	if usersRec.Code != http.StatusOK {
		t.Fatalf("module users status = %d, body = %s", usersRec.Code, usersRec.Body.String())
	}
	var usersBody AdminUserList
	if err := json.Unmarshal(usersRec.Body.Bytes(), &usersBody); err != nil {
		t.Fatalf("decode module users: %v", err)
	}
	if usersBody.Total != 2 || len(usersBody.Items) != 2 || usersBody.Items[0].ID != a.ID || usersBody.Items[1].ID != b.ID {
		t.Fatalf("module users = %#v", usersBody)
	}
}

func TestAdminAccessBatchEmptyModuleCodesRevokesAllControlledGrants(t *testing.T) {
	auth := newTestAuthService(t)
	seedDatabaseAdmin(t, auth.dao)
	user := createActiveModuleUser(t, auth.dao, "user-a")
	modules := NewModuleService(auth.dao)
	if err := modules.ReplaceUserAccess(context.Background(), "admin-a", []string{user.ID}, []string{"radar.main_strategy"}); err != nil {
		t.Fatalf("grant module: %v", err)
	}
	handler := NewAdminHTTPHandler(NewAdminService(auth.dao, nil), modules)
	cookie := loginDatabaseAdminForTest(t, handler)

	req := newAdminCookieRequest(http.MethodPut, "/api/admin/access", cookie, strings.NewReader(`{"userIds":["user-a"],"moduleCodes":[]}`))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK || strings.TrimSpace(rec.Body.String()) != `{"status":"ok"}` {
		t.Fatalf("revoke response = %d %s", rec.Code, rec.Body.String())
	}
	access, err := modules.ListUserAccess(context.Background(), []string{user.ID})
	if err != nil {
		t.Fatalf("load access: %v", err)
	}
	if len(access) != 1 || len(access[0].ModuleCodes) != 0 {
		t.Fatalf("access after revoke = %#v", access)
	}
}

func TestAdminAccessBatchRejectsInvalidArguments(t *testing.T) {
	auth := newTestAuthService(t)
	seedDatabaseAdmin(t, auth.dao)
	user := createActiveModuleUser(t, auth.dao, "user-a")
	handler := NewAdminHTTPHandler(NewAdminService(auth.dao, nil), NewModuleService(auth.dao))
	cookie := loginDatabaseAdminForTest(t, handler)

	for name, body := range map[string]string{
		"malformed json":   `{"userIds":`,
		"empty user ids":   `{"userIds":[],"moduleCodes":[]}`,
		"unknown user":     `{"userIds":["missing"],"moduleCodes":[]}`,
		"unknown module":   `{"userIds":["` + user.ID + `"],"moduleCodes":["radar.unknown"]}`,
		"duplicate module": `{"userIds":["` + user.ID + `"],"moduleCodes":["radar.main_strategy","radar.main_strategy"]}`,
		"public module":    `{"userIds":["` + user.ID + `"],"moduleCodes":["radar.monitored"]}`,
	} {
		t.Run(name, func(t *testing.T) {
			req := newAdminCookieRequest(http.MethodPut, "/api/admin/access", cookie, strings.NewReader(body))
			req.Header.Set("Content-Type", "application/json")
			rec := httptest.NewRecorder()
			handler.ServeHTTP(rec, req)
			if rec.Code != http.StatusBadRequest || !strings.Contains(rec.Body.String(), "INVALID_ARGUMENT") {
				t.Fatalf("response = %d %s", rec.Code, rec.Body.String())
			}
		})
	}
}

func sameCodes(got []ModuleDefinition, want ...string) bool {
	if len(got) != len(want) {
		return false
	}
	for i, module := range got {
		if module.Code != want[i] {
			return false
		}
	}
	return true
}

func sameStrings(got []string, want ...string) bool {
	if len(got) != len(want) {
		return false
	}
	for i, value := range got {
		if value != want[i] {
			return false
		}
	}
	return true
}
