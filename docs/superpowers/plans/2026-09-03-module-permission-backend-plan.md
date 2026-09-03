# Module Permission Backend Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (- [ ]) syntax for tracking.

**Goal:** 在 Go 服务中建立代码注册的模块目录、按用户独立授权的存储与接口、数据库管理员会话，并把「盘达」三个受控策略接口真正隔离。

**Architecture:** 以 ModuleDefinition 代码目录作为唯一模块来源，以 module_user_grants 保存用户白名单，以 ModuleService 集中处理可见模块、批量覆盖授权和接口鉴权。8080 只保留普通用户 API；同一个 Go 进程另外监听管理端口，提供数据库管理员登录、管理 API 和 admin-web/dist 静态文件。

**Tech Stack:** Go 1.26、net/http、GORM、SQLite、bcrypt、httptest、golang.org/x/term。

**Spec:** docs/superpowers/specs/2026-09-03-module-permission-management-design.md

## Global Constraints

- 首期范围：只配置 Flutter 用户端「盘达」内部 Tab。
- 公开模块：监控股票（自选）、自选异动、全市场，对所有已登录用户可见。
- 受控模块：紫策、主板策略、蓝策，以及未来新增的其他业务模块；默认不可见。
- 授权对象是具体用户，不使用用户组作为首期授权对象。
- 每个模块权限独立；授权或撤销一个模块不得改变其他模块。
- GET /api/auth/modules 只返回模块元数据，不返回股票结果。
- 受控业务接口必须在服务端校验模块编码，未授权返回 403 MODULE_FORBIDDEN。
- /api/t0-selection 返回策略结果或历史归档时必须携带一个明确的 module_code，并且只返回该模块对应结果。
- 管理后台默认监听 :18080，可由 GO_STOCK_ADMIN_ADDR 配置；8080 不注册管理 API。
- 管理员账号使用 users 表、role=admin 和 bcrypt；禁止写死 admin/admin。
- 管理浏览器会话使用数据库 admin_sessions、HttpOnly、SameSite=Lax、Path=/ Cookie；写操作校验同源 Origin 或等价 CSRF Token。
- 开发期写操作允许 GO_STOCK_ADMIN_DEV_ORIGINS 配置逗号分隔的 Vite 源，生产同源请求不依赖通配 CORS。
- 权限保存必须在一个事务内完成；moduleCodes 为空表示撤销选中用户的全部受控权限。
- 不修改现有 T0 选股规则、策略排序、缓存格式或归档文件。

## File Map

### Create

- backend/flutter_api/module_models.go — 模块元数据、权限快照、GORM 模型。
- backend/flutter_api/module_registry.go — 六个首期模块和稳定编码查找。
- backend/flutter_api/module_service.go — 可见模块、批量授权、反向查询、鉴权。
- backend/flutter_api/module_http.go — 用户端和管理端模块 API。
- backend/flutter_api/admin_bootstrap.go — 可测试的管理员创建服务函数。
- cmd/admin-init/main.go — 隐藏密码输入的初始化命令。
- backend/flutter_api/module_registry_test.go、module_service_test.go、module_http_test.go、admin_bootstrap_test.go — 对应测试。
- backend/flutter_api/module_http_test_helpers_test.go — 模块 HTTP 测试的管理员初始化、Cookie 请求和普通用户夹具。

### Modify

- backend/flutter_api/auth_models.go、auth_migrations.go — 新增权限和管理员会话模型/迁移。
- backend/flutter_api/admin_service.go、admin_http.go — 数据库管理员认证、Cookie、CSRF 和管理 API。
- backend/flutter_api/auth_http.go、auth_middleware.go — 权限清单和普通服务边界。
- backend/flutter_api/server.go — 用户服务与独立管理端口。
- backend/flutter_api/t0_selection.go — T0 module_code 校验和结果范围。
- backend/flutter_api/admin_service_test.go、admin_http_test.go、user_management_e2e_test.go — 改用数据库管理员、Cookie 会话和独立管理 Handler。
- backend/flutter_api/auth_http_test.go、server_auth_integration_test.go — 注入 ModuleService，并覆盖用户端不暴露管理 API 的边界。
- backend/flutter_api/t0_pattern_test.go、t0_selection_cache_test.go、t0_selection_aug4_sim_test.go — 给结果/归档请求增加明确 module_code 和认证测试上下文。
- go.mod、go.sum — 若需要，加入 golang.org/x/term 直接依赖。
- PROJECT.md — 更新运行和部署说明。

## Implementation Order

先完成本计划的后端接口，再执行 admin-web 和 Flutter 两份计划。后端内部顺序为：目录/迁移 → 权限服务 → 管理员会话 → HTTP API → T0 数据边界 → 独立监听 → 文档验证。

### Task 1: Add the module registry and database models

**Files:**

- Create: backend/flutter_api/module_models.go
- Create: backend/flutter_api/module_registry.go
- Create: backend/flutter_api/module_registry_test.go
- Modify: backend/flutter_api/auth_models.go
- Modify: backend/flutter_api/auth_migrations.go

**Interfaces:**

- ModuleAccessMode, ModuleDefinition, RegisteredModules() []ModuleDefinition, FindModule(code string) (ModuleDefinition, bool).
- ModuleUserGrant maps to module_user_grants.
- AdminSession maps to admin_sessions.
- MigrateAuthTables remains idempotent and creates both new tables.

- [ ] **Step 1: Write the failing registry and migration tests**

~~~go
func TestRegisteredModulesContainsCurrentRadarTabs(t *testing.T) {
  got := RegisteredModules()
  want := []string{
    "radar.monitored",
    "radar.purple_strategy",
    "radar.main_strategy",
    "radar.blue_strategy",
    "radar.watch_changes",
    "radar.all_changes",
  }
  if len(got) != len(want) {
    t.Fatalf("module count = %d, want %d", len(got), len(want))
  }
  seen := map[string]bool{}
  for index, module := range got {
    if module.Code != want[index] || seen[module.Code] {
      t.Fatalf("module[%d] = %+v", index, module)
    }
    seen[module.Code] = true
  }
}

func TestMigrateAuthTablesCreatesPermissionTablesAndIndexes(t *testing.T) {
  dao := newAuthTestDB(t)
  if err := MigrateAuthTables(dao); err != nil {
    t.Fatalf("first migration: %v", err)
  }
  if err := MigrateAuthTables(dao); err != nil {
    t.Fatalf("second migration: %v", err)
  }
  if !dao.Migrator().HasTable(&ModuleUserGrant{}) ||
    !dao.Migrator().HasTable(&AdminSession{}) {
    t.Fatal("permission tables were not created")
  }
  if !dao.Migrator().HasIndex(&ModuleUserGrant{},
    "idx_module_user_grants_unique") {
    t.Fatal("unique grant index was not created")
  }
}
~~~

- [ ] **Step 2: Run the focused tests to verify they fail**

Run: go test ./backend/flutter_api -run 'TestRegisteredModulesContainsCurrentRadarTabs|TestMigrateAuthTablesCreatesPermissionTablesAndIndexes' -count=1

Expected: FAIL because the registry, models, and indexes do not exist.

- [ ] **Step 3: Implement the registry and models**

Define ModuleAccessPublic as public and ModuleAccessAllowlist as user_allowlist. Register exactly these six definitions in sort order 10, 20, 30, 40, 50, 60: radar.monitored, radar.purple_strategy, radar.main_strategy, radar.blue_strategy, radar.watch_changes, radar.all_changes. Use flutter_web and radar_tab for the first phase and leave ParentCode nil. The ParentCode JSON field must not use omitempty so the user response serializes null as specified.

Add GORM tags to ModuleUserGrant for id, module_code, user_id, created_by, created_at, updated_at, a unique composite index named idx_module_user_grants_unique, and a user_id index. Add AdminSession fields id, user_id, token_hash, created_at, last_seen_at, expires_at, revoked_at; token_hash is unique and user_id is indexed. Extend MigrateAuthTables with both new models without changing existing user-owned migrations.

- [ ] **Step 4: Run the migration tests**

Run: go test ./backend/flutter_api -run 'TestRegisteredModules|TestMigrateAuthTables|TestAuthModels' -count=1

Expected: PASS.

- [ ] **Step 5: Commit**

~~~bash
git add backend/flutter_api/module_models.go backend/flutter_api/module_registry.go backend/flutter_api/module_registry_test.go backend/flutter_api/auth_models.go backend/flutter_api/auth_migrations.go
git commit -m "feat(admin): add module permission storage"
~~~

### Task 2: Implement the per-user ModuleService

**Files:**

- Create: backend/flutter_api/module_service.go
- Create: backend/flutter_api/module_service_test.go

**Interfaces:**

- NewModuleService(dao *gorm.DB) *ModuleService.
- ListVisibleModules(ctx context.Context, userID string) ([]ModuleDefinition, error).
- ListUserAccess(ctx context.Context, userIDs []string) ([]ModuleAccessSnapshot, error).
- ReplaceUserAccess(ctx context.Context, adminID string, userIDs []string, moduleCodes []string) error.
- ListModuleUsers(ctx context.Context, moduleCode string) (*AdminUserList, error).
- HasModuleAccess(ctx context.Context, userID, moduleCode string) (bool, error).

- [ ] **Step 1: Write failing service tests**

~~~go
func TestModuleServiceVisibleModulesUsesPublicAndDirectGrants(t *testing.T) {
  service := newTestModuleService(t)
  user := createActiveModuleUser(t, service.dao, "user-a")

  visible, err := service.ListVisibleModules(context.Background(), user.ID)
  if err != nil {
    t.Fatalf("list before grant: %v", err)
  }
  assertModuleCodes(t, visible,
    "radar.monitored", "radar.watch_changes", "radar.all_changes")

  if err := service.ReplaceUserAccess(context.Background(), "admin-a",
    []string{user.ID}, []string{"radar.main_strategy"}); err != nil {
    t.Fatalf("grant main: %v", err)
  }
  visible, err = service.ListVisibleModules(context.Background(), user.ID)
  if err != nil {
    t.Fatalf("list after grant: %v", err)
  }
  assertModuleCodes(t, visible,
    "radar.monitored", "radar.main_strategy",
    "radar.watch_changes", "radar.all_changes")
  if ok, err := service.HasModuleAccess(context.Background(),
    user.ID, "radar.purple_strategy"); err != nil || ok {
    t.Fatalf("purple access = %v, err = %v, want false", ok, err)
  }
}

func TestModuleServiceBatchReplaceIsAtomicAndEmptyRevokes(t *testing.T) {
  service := newTestModuleService(t)
  a := createActiveModuleUser(t, service.dao, "user-a")
  b := createActiveModuleUser(t, service.dao, "user-b")

  if err := service.ReplaceUserAccess(context.Background(), "admin-a",
    []string{a.ID, b.ID},
    []string{"radar.purple_strategy", "radar.blue_strategy"}); err != nil {
    t.Fatalf("initial grant: %v", err)
  }
  if err := service.ReplaceUserAccess(context.Background(), "admin-a",
    []string{a.ID, b.ID}, nil); err != nil {
    t.Fatalf("revoke: %v", err)
  }
  access, err := service.ListUserAccess(context.Background(), []string{a.ID, b.ID})
  if err != nil {
    t.Fatalf("read access: %v", err)
  }
  for _, item := range access {
    if len(item.ModuleCodes) != 0 {
      t.Fatalf("user %s still has grants: %#v", item.UserID, item.ModuleCodes)
    }
  }

  err = service.ReplaceUserAccess(context.Background(), "admin-a",
    []string{a.ID, b.ID},
    []string{"radar.main_strategy", "radar.not_registered"})
  if !IsAuthCode(err, "INVALID_ARGUMENT") {
    t.Fatalf("invalid batch error = %v, want INVALID_ARGUMENT", err)
  }
  access, err = service.ListUserAccess(context.Background(), []string{a.ID, b.ID})
  if err != nil {
    t.Fatalf("read after rollback: %v", err)
  }
  for _, item := range access {
    if len(item.ModuleCodes) != 0 {
      t.Fatalf("rollback left grants for user %s: %#v",
        item.UserID, item.ModuleCodes)
    }
  }
}
~~~

- [ ] **Step 2: Run the service tests to verify they fail**

Run: go test ./backend/flutter_api -run 'TestModuleService' -count=1

Expected: FAIL because ModuleService and its data types are not implemented.

- [ ] **Step 3: Implement allowlist semantics**

ListVisibleModules returns all public definitions plus directly granted allowlist definitions, ordered by Sort. HasModuleAccess returns true for a known public module and checks one grant row for a controlled module; unknown codes return INVALID_ARGUMENT.

ReplaceUserAccess trims and deduplicates IDs and codes, rejects an empty user list, validates every target as an ordinary user, rejects unknown/public module codes, and performs delete-plus-insert for all selected users inside one GORM transaction. The empty moduleCodes slice performs only the delete phase. Store adminID in created_by. ListUserAccess returns one snapshot per requested user, including an empty list. ListModuleUsers rejects unknown/public modules and returns ordinary users with a grant plus a matching total count.

Use this snapshot shape:

~~~go
type ModuleAccessSnapshot struct {
  UserID      string
  ModuleCodes []string
}
~~~

- [ ] **Step 4: Run focused and package tests**

Run: go test ./backend/flutter_api -run 'TestModuleService' -count=1

Expected: PASS for public fallback, each independent strategy grant, batch replacement, empty revoke, invalid input rollback, and reverse lookup.

Run: go test ./backend/flutter_api -count=1

Expected: PASS.

- [ ] **Step 5: Commit**

~~~bash
git add backend/flutter_api/module_service.go backend/flutter_api/module_service_test.go
git commit -m "feat(admin): add per-user module permission service"
~~~

### Task 3: Replace hardcoded admin authentication and add admin-init

**Files:**

- Create: backend/flutter_api/admin_bootstrap.go
- Create: backend/flutter_api/admin_bootstrap_test.go
- Create: cmd/admin-init/main.go
- Modify: backend/flutter_api/admin_service.go
- Modify: backend/flutter_api/admin_http.go
- Modify: backend/flutter_api/admin_service_test.go
- Modify: backend/flutter_api/admin_http_test.go
- Modify: backend/flutter_api/user_management_e2e_test.go
- Modify: go.mod and go.sum when the direct term dependency is required

**Interfaces:**

- AdminInitInput{Account, Nickname, Password string}.
- CreateAdmin(ctx context.Context, dao *gorm.DB, input AdminInitInput) error.
- AdminService.Login(ctx context.Context, input AdminLoginInput) (rawToken string, response *AdminSessionResponse, err error); AdminSessionResponse contains expiry metadata only and never serializes rawToken.
- AdminSessionResponse has only an ExpiresAt time.Time JSON field; the login JSON may include status: "ok" but never includes accessToken.
- AdminService.Authenticate(ctx context.Context, rawToken string) (AdminPrincipal, error).
- AdminService.Logout(ctx context.Context, rawToken string) error.
- RequireAdmin(service *AdminService, next http.Handler) reads Cookie go_stock_admin_session and stores AdminPrincipal in context.

- [ ] **Step 1: Write failing bootstrap and persistence tests**

~~~go
func TestCreateAdminHashesPasswordAndDoesNotOverwriteExistingAccount(t *testing.T) {
  service := newTestAuthService(t)
  input := AdminInitInput{
    Account: "13900000000", Nickname: "管理员一", Password: "secret123",
  }
  if err := CreateAdmin(context.Background(), service.dao, input); err != nil {
    t.Fatalf("create admin: %v", err)
  }
  var user AuthUser
  if err := service.dao.First(&user, "phone = ?", input.Account).Error; err != nil {
    t.Fatalf("load admin: %v", err)
  }
  if user.Role != "admin" ||
    bcrypt.CompareHashAndPassword([]byte(user.PasswordHash),
      []byte(input.Password)) != nil {
    t.Fatalf("invalid admin record: %+v", user)
  }
  oldHash := user.PasswordHash
  if err := CreateAdmin(context.Background(), service.dao, input);
    !IsAuthCode(err, "ACCOUNT_EXISTS") {
    t.Fatalf("duplicate error = %v", err)
  }
  if err := service.dao.First(&user, "phone = ?", input.Account).Error;
    err != nil || user.PasswordHash != oldHash {
    t.Fatalf("duplicate changed password: %v", err)
  }
}

func TestAdminSessionSurvivesServiceRecreationAndLogoutRevokesIt(t *testing.T) {
  service := newTestAuthService(t)
  err := CreateAdmin(context.Background(), service.dao, AdminInitInput{
    Account: "13900000000", Nickname: "管理员", Password: "secret123",
  })
  if err != nil {
    t.Fatalf("create admin: %v", err)
  }
  admin := NewAdminService(service.dao, nil)
  token, _, err := admin.Login(context.Background(),
    AdminLoginInput{Username: "13900000000", Password: "secret123"})
  if err != nil {
    t.Fatalf("login admin: %v", err)
  }
  admin = NewAdminService(service.dao, nil)
  if _, err := admin.Authenticate(context.Background(), token); err != nil {
    t.Fatalf("session did not persist: %v", err)
  }
  if err := admin.Logout(context.Background(), token); err != nil {
    t.Fatalf("logout: %v", err)
  }
  if _, err := admin.Authenticate(context.Background(), token);
    !IsAuthCode(err, "ADMIN_UNAUTHENTICATED") {
    t.Fatalf("revoked session error = %v", err)
  }
}
~~~

- [ ] **Step 2: Run tests to verify they fail**

Run: go test ./backend/flutter_api -run 'TestCreateAdmin|TestAdminSession' -count=1

Expected: FAIL because the current service only accepts admin/admin and stores tokens in memory.

- [ ] **Step 3: Implement database-backed sessions**

CreateAdmin trims Account, uses it as users.phone, defaults an empty nickname to Account, validates a non-empty password, hashes with bcrypt, sets role admin/status active, and creates exactly one row in a transaction. A duplicate maps to ACCOUNT_EXISTS and never updates an existing row.

AdminService.Login looks up users.phone, compares bcrypt, requires role admin and active status, creates a secure random token, stores only SHA-256 in admin_sessions, and returns the raw token only to the HTTP handler. AdminSessionResponse must omit AccessToken from its JSON representation; the handler writes rawToken to the HttpOnly cookie and returns only the expiry/status payload. Authenticate rejects missing/expired/revoked/disabled/non-admin sessions and updates last_seen_at. Logout marks the session revoked. Remove the hardcoded constants, in-memory token map, and session limit.

Set the session cookie with Name go_stock_admin_session, Path /, HttpOnly true, SameSite Lax, Expires response.ExpiresAt, and Secure when the request is HTTPS. Clear it with MaxAge -1 on logout. For POST, PUT, and PATCH, reject a present Origin that is not the request host or an explicitly configured development origin. Missing credentials, bad credentials, and unauthorized ordinary users must not reveal account existence.

The cmd/admin-init command uses standard flags -account and -nickname, reads and confirms the password twice with term.ReadPassword, calls db.Init(""), MigrateAuthTables, and CreateAdmin. It prints no password or hash and fails when the account already exists. Rewrite admin_service_test.go, admin_http_test.go, and user_management_e2e_test.go to create an admin fixture through CreateAdmin and send the session cookie; no admin test may depend on the old admin/admin credentials, bearer admin token, in-memory token map, or AccessToken field.

- [ ] **Step 4: Run tests and build the command**

Run: go test ./backend/flutter_api -run 'TestCreateAdmin|TestAdminSession|TestAdminHTTP' -count=1

Expected: PASS for multiple admins, bcrypt, duplicate protection, expiry/revocation, Cookie attributes, and status checks.

Run: go build ./cmd/admin-init

Expected: PASS.

- [ ] **Step 5: Commit**

~~~bash
git add backend/flutter_api/admin_bootstrap.go backend/flutter_api/admin_bootstrap_test.go cmd/admin-init/main.go backend/flutter_api/admin_service.go backend/flutter_api/admin_http.go backend/flutter_api/admin_service_test.go backend/flutter_api/admin_http_test.go backend/flutter_api/user_management_e2e_test.go go.mod go.sum
git commit -m "fix(admin): use database administrator sessions"
~~~

### Task 4: Expose the user and admin module HTTP APIs

**Files:**

- Create: backend/flutter_api/module_http.go
- Create: backend/flutter_api/module_http_test.go
- Create: backend/flutter_api/module_http_test_helpers_test.go
- Modify: backend/flutter_api/auth_http.go
- Modify: backend/flutter_api/admin_http.go
- Modify: backend/flutter_api/admin_http_test.go

**Interfaces:**

- GET /api/auth/modules returns version 1 and modules.
- POST /api/admin/login sets the session cookie and returns expiry/status metadata; POST /api/admin/logout revokes and clears it.
- GET /api/admin/me returns the authenticated admin profile and session expiry.
- GET /api/admin/modules returns all registered modules and authorizedUserCount.
- GET /api/admin/access?user_ids=user-a,user-b returns one access snapshot per requested user.
- GET /api/admin/modules/{module_code}/users returns items and total.
- PUT /api/admin/access accepts userIds and moduleCodes and returns status ok.
- PATCH /api/admin/users/{user_id}/status remains the status endpoint.
- NewAuthHTTPHandler(authService, moduleService) and NewAdminHTTPHandler(adminService, moduleService) are used by production and tests.

The test helper file defines these exact test-only helpers: newTestModuleService(t *testing.T) *ModuleService, createActiveModuleUser(t *testing.T, dao *gorm.DB, account string) *AuthUser, seedDatabaseAdmin(t *testing.T, dao *gorm.DB) *AuthUser, loginDatabaseAdminForTest(t *testing.T, handler http.Handler) http.Cookie, and newAdminCookieRequest(method, path string, cookie http.Cookie, body io.Reader) *http.Request. The admin helper calls CreateAdmin with a non-default fixture account and sends the returned Cookie, so the contract tests never rely on a bearer admin token or hardcoded admin/admin credentials.

- [ ] **Step 1: Write failing HTTP contract tests**

~~~go
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
    Version int
    Modules []ModuleDefinition
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

func TestAdminAccessBatchRejectsPublicModule(t *testing.T) {
  auth := newTestAuthService(t)
  seedDatabaseAdmin(t, auth.dao)
  user := createActiveModuleUser(t, auth.dao, "13800000000")
  handler := NewAdminHTTPHandler(
    NewAdminService(auth.dao, nil), NewModuleService(auth.dao))
  cookie := loginDatabaseAdminForTest(t, handler)

  req := newAdminCookieRequest(http.MethodPut, "/api/admin/access",
    cookie, strings.NewReader(
      "{\"userIds\":[\"" + user.ID +
      "\"],\"moduleCodes\":[\"radar.monitored\"]}"))
  req.Header.Set("Content-Type", "application/json")
  rec := httptest.NewRecorder()
  handler.ServeHTTP(rec, req)
  if rec.Code != http.StatusBadRequest ||
    !strings.Contains(rec.Body.String(), "INVALID_ARGUMENT") {
    t.Fatalf("response = %d %s", rec.Code, rec.Body.String())
  }
}
~~~

- [ ] **Step 2: Run the contract tests to verify they fail**

Run: go test ./backend/flutter_api -run 'TestAuthModulesReturnsOnlyPublicModulesWithoutGrants|TestAdminAccessBatchRejectsPublicModule' -count=1

Expected: FAIL because the routes and response handlers do not exist.

- [ ] **Step 3: Implement exact response and validation behavior**

The user response is:
~~~json
{"version":1,"modules":[{"code":"radar.main_strategy","name":"主板策略","client":"flutter_web","placement":"radar_tab","parentCode":null,"sort":30,"accessMode":"user_allowlist"}]}
~~~

The access response is:
~~~json
{"users":[
  {"userId":"user-a","moduleCodes":["radar.main_strategy"]},
  {"userId":"user-b","moduleCodes":[]}
]}
~~~

The login handler writes the raw service token to go_stock_admin_session and returns only expiry/status metadata; it never serializes AccessToken. GET /api/admin/me reads the AdminPrincipal from context and returns the admin profile plus expiry. Logout revokes the cookie session and clears the cookie with MaxAge -1. The save response is {"status":"ok"}. Reject malformed JSON, empty userIds, unknown users, unknown/duplicate module codes, and public module codes with INVALID_ARGUMENT. Empty moduleCodes is valid and revokes all controlled grants. Keep the user status endpoint behind RequireAdmin. The auth modules handler reads ordinary AuthPrincipal and never reads or returns business data.

- [ ] **Step 4: Run the HTTP package tests**

Run: go test ./backend/flutter_api -run 'Test(Auth|Admin|Server).*' -count=1

Expected: PASS for authentication, batch replacement, public lock, reverse lookup, and status enable/disable.

- [ ] **Step 5: Commit**

~~~bash
git add backend/flutter_api/module_http.go backend/flutter_api/module_http_test.go backend/flutter_api/module_http_test_helpers_test.go backend/flutter_api/auth_http.go backend/flutter_api/admin_http.go backend/flutter_api/admin_http_test.go
git commit -m "feat(admin): expose module permission APIs"
~~~

### Task 5: Enforce independent T0 strategy access and result scope

**Files:**

- Modify: backend/flutter_api/t0_selection.go
- Modify: backend/flutter_api/server.go
- Modify: backend/flutter_api/server_auth_integration_test.go
- Modify: backend/flutter_api/t0_pattern_test.go
- Modify: backend/flutter_api/t0_selection_cache_test.go
- Modify: backend/flutter_api/t0_selection_aug4_sim_test.go

**Interfaces:**

- Strategy result requests use module_code equal to one of the three strategy module codes.
- newT0SelectionHandler(moduleService *ModuleService) http.HandlerFunc performs the result/archive guard before calling the existing calculation code; server.go registers this handler at /api/t0-selection.
- filterPurpleT0Results and filterBlueT0Results return new slices and never mutate the shared calculation or archive slices.
- Missing/invalid module_code returns 400 INVALID_ARGUMENT for every T0 operation except list_dates=1, which returns dates only.
- Valid but ungranted module_code returns 403 MODULE_FORBIDDEN before calculation, prewarm candidates, or archive data is read.
- An authorized request returns only the selected module view.

- [ ] **Step 1: Write failing independent-access tests**

~~~go
func TestT0SelectionRequiresModuleCodeForResults(t *testing.T) {
  auth := newTestAuthService(t)
  user := authHTTPRegisterUser(t, auth, "13800000000", "Alice", "device-a")
  req := httptest.NewRequest(http.MethodGet,
    "/api/t0-selection?archived=1&date=2026-08-11", nil)
  req.Header.Set("Authorization", "Bearer "+user.AccessToken)
  rec := httptest.NewRecorder()
  newHTTPHandler(auth.AuthService).ServeHTTP(rec, req)
  if rec.Code != http.StatusBadRequest ||
    !strings.Contains(rec.Body.String(), "INVALID_ARGUMENT") {
    t.Fatalf("response = %d %s", rec.Code, rec.Body.String())
  }
}

func TestT0SelectionDoesNotCoupleMainPurpleAndBlue(t *testing.T) {
  auth := newTestAuthService(t)
  user := authHTTPRegisterUser(t, auth, "13800000000", "Alice", "device-a")
  modules := NewModuleService(auth.dao)
  if err := modules.ReplaceUserAccess(context.Background(), "admin-a",
    []string{user.User.ID}, []string{"radar.main_strategy"}); err != nil {
    t.Fatalf("grant main: %v", err)
  }

  for _, code := range []string{
    "radar.purple_strategy", "radar.blue_strategy",
  } {
    req := httptest.NewRequest(http.MethodGet,
      "/api/t0-selection?module_code="+url.QueryEscape(code)+
        "&archived=1&date=2026-08-11", nil)
    req.Header.Set("Authorization", "Bearer "+user.AccessToken)
    rec := httptest.NewRecorder()
    newHTTPHandler(auth.AuthService).ServeHTTP(rec, req)
    if rec.Code != http.StatusForbidden ||
      !strings.Contains(rec.Body.String(), "MODULE_FORBIDDEN") {
      t.Fatalf("%s response = %d %s", code, rec.Code, rec.Body.String())
    }
  }
}
~~~

- [ ] **Step 2: Run the tests to verify they fail**

Run: go test ./backend/flutter_api -run 'TestT0SelectionRequiresModuleCodeForResults|TestT0SelectionDoesNotCoupleMainPurpleAndBlue' -count=1

Expected: FAIL because the current handler ignores module_code and any authenticated user can reach the shared T0 route.

- [ ] **Step 3: Implement the request guard and pure result selector**

Read module_code from the request in newT0SelectionHandler, resolve it, obtain user ID from PrincipalFromContext, and call HasModuleAccess before any T0 operation other than list_dates=1. list_dates=1 is the only unscoped metadata response and must contain no result data. Prewarm responses that contain candidates or early-window historical results also require module_code and pass through the selected-module filter. Update the direct result/prewarm tests to go through an authenticated newHTTPHandler request; keep only pure cache/path tests free of HTTP auth. Use this selector:

~~~go
func selectT0ResultsForModule(
  moduleCode string, results []T0SelectionResult,
) ([]T0SelectionResult, error) {
  switch moduleCode {
  case "radar.main_strategy":
    return results, nil
  case "radar.purple_strategy":
    return filterPurpleT0Results(results), nil
  case "radar.blue_strategy":
    return filterBlueT0Results(results), nil
  default:
    return nil, newAuthError(http.StatusBadRequest,
      "INVALID_ARGUMENT", "模块不存在")
  }
}
~~~

Define the server-side purple selector with the current client rule `PatternT0N >= 2 && PatternWinPct > 40 && (100-PatternFailPct) > 60`, and the blue selector as `BuySignal == BuySignalBlue`; these predicates only move the existing presentation filters to the scoped response boundary. Apply the selector immediately before writing live and archived results. Do not alter calculation, sorting, cache, or archive formats. Keep the existing handler function’s response shapes for prewarm and date metadata, and ensure every result response contains only the selected module’s result list.

- [ ] **Step 4: Run T0 tests**

Run: go test ./backend/flutter_api -run 'TestT0Selection|Test.*T0.*(Sort|Pattern|Cache|Archive)' -count=1

Expected: PASS.

- [ ] **Step 5: Commit**

~~~bash
git add backend/flutter_api/t0_selection.go backend/flutter_api/server.go backend/flutter_api/server_auth_integration_test.go backend/flutter_api/t0_pattern_test.go backend/flutter_api/t0_selection_cache_test.go backend/flutter_api/t0_selection_aug4_sim_test.go
git commit -m "feat(admin): protect independent t0 strategy modules"
~~~

### Task 6: Split the user server from the standalone admin server

**Files:**

- Modify: backend/flutter_api/server.go, backend/flutter_api/auth_middleware.go, backend/flutter_api/admin_http.go
- Modify: http_server.go — document that StartHTTPServer launches the user listener and the standalone admin listener in the same process.
- Modify: backend/flutter_api/server_auth_integration_test.go

**Interfaces:**

- GO_STOCK_ADMIN_ADDR controls the admin listener; default :18080.
- GO_STOCK_ADMIN_WEB_DIR optionally points to admin-web/dist.
- GO_STOCK_ADMIN_DEV_ORIGINS is a comma-separated allowlist used only for development writes, with http://localhost:5174 as the documented Vite origin.
- newHTTPHandler contains no /api/admin/ registration.
- Admin listener serves the admin handler under /api/admin/ and SPA files under /.

- [ ] **Step 1: Write failing boundary and SPA tests**

~~~go
func TestUserHTTPHandlerDoesNotExposeAdminRoutes(t *testing.T) {
  auth := newTestAuthService(t)
  req := httptest.NewRequest(http.MethodGet, "/api/admin/users", nil)
  rec := httptest.NewRecorder()
  newHTTPHandler(auth.AuthService).ServeHTTP(rec, req)
  if rec.Code == http.StatusOK {
    t.Fatalf("admin response leaked from user server: %s", rec.Body.String())
  }
}

func TestAdminWebRootServesSpaFallback(t *testing.T) {
  root := t.TempDir()
  if err := os.WriteFile(filepath.Join(root, "index.html"),
    []byte("<html>admin</html>"), 0600); err != nil {
    t.Fatalf("write index: %v", err)
  }
  handler := newAdminWebHandlerForTest(root)
  req := httptest.NewRequest(http.MethodGet, "/users", nil)
  rec := httptest.NewRecorder()
  handler.ServeHTTP(rec, req)
  if rec.Code != http.StatusOK ||
    !strings.Contains(rec.Body.String(), "admin") {
    t.Fatalf("response = %d %s", rec.Code, rec.Body.String())
  }
}
~~~

- [ ] **Step 2: Run the tests to verify they fail**

Run: go test ./backend/flutter_api -run 'TestUserHTTPHandlerDoesNotExposeAdminRoutes|TestAdminWebRootServesSpaFallback' -count=1

Expected: FAIL because the current 8080 mux registers admin routes and has no admin static handler.

- [ ] **Step 3: Implement separate listener construction**

Construct one AuthService, ModuleService, and AdminService after MigrateAuthTables. Move admin route registration out of newHTTPHandler. Start the admin server in a goroutine, then keep the existing user listener and background jobs. Add newAdminWebHandlerForTest(root string) http.Handler around the production static-file builder so the SPA fallback test uses the same path resolver.

Use this resolver:

~~~go
const defaultAdminHTTPAddr = ":18080"

func adminHTTPAddr() string {
  if value := strings.TrimSpace(os.Getenv("GO_STOCK_ADMIN_ADDR"));
    value != "" {
    return value
  }
  return defaultAdminHTTPAddr
}
~~~

The static resolver checks GO_STOCK_ADMIN_WEB_DIR first, then project-root/admin-web/dist/index.html. If no build exists, keep /api/admin/ available for Vite development and log a warning; never serve Wails frontend/dist. Parse GO_STOCK_ADMIN_DEV_ORIGINS once for Origin validation; same-origin requests use the request host, and a mismatched origin receives 403 CSRF_INVALID. Remove the admin special-case from ordinary auth routing and keep user CORS separate from the admin handler.

- [ ] **Step 4: Run server and package tests**

Run: go test ./backend/flutter_api -run 'Test(UserHTTP|AdminWeb|Server|Auth|Admin)' -count=1

Expected: PASS with no admin API on 8080, cookie-authenticated admin handler, SPA fallback, and unchanged user API behavior.

Run: go test ./backend/flutter_api -count=1

Expected: PASS.

- [ ] **Step 5: Commit**

~~~bash
git add backend/flutter_api/server.go backend/flutter_api/auth_middleware.go backend/flutter_api/admin_http.go backend/flutter_api/server_auth_integration_test.go http_server.go
git commit -m "feat(admin): serve management web on separate port"
~~~

### Task 7: Update documentation and perform backend verification

**Files:**

- Modify: PROJECT.md

- [ ] **Step 1: Record the deployment smoke flow**

~~~text
go run ./cmd/admin-init -account 13900000000 -nickname 管理员
GO_STOCK_ADMIN_ADDR=:18080 go run ./cmd/server
浏览器打开 http://服务器地址:18080/
~~~

The smoke flow verifies admin login on 18080, ordinary user login through 8080, and exactly the three public modules before any grant.

- [ ] **Step 2: Run final verification**

Run: go test ./backend/flutter_api -count=1

Expected: PASS.

Run: go build ./cmd/server ./cmd/admin-init

Expected: PASS.

Run: git diff --check

Expected: no output and exit code 0.

- [ ] **Step 3: Update PROJECT.md**

Document 8080 as Flutter user API/Web, GO_STOCK_ADMIN_ADDR default :18080, GO_STOCK_ADMIN_WEB_DIR, admin-init behavior, and the requirement for HTTPS/reverse proxy before formal public deployment.

- [ ] **Step 4: Commit**

~~~bash
git add PROJECT.md
git commit -m "docs(admin): document standalone permission backend"
~~~
