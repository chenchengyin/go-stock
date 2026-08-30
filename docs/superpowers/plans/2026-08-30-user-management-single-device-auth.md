# 用户管理与单设备登录实施计划

> **For agentic workers:** REQUIRED SUB-SKILL: Use `superpowers:subagent-driven-development` (recommended) or `superpowers:executing-plans` to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** 将现有 Flutter Mock 登录/注册升级为服务端真实账号体系，并在所有 `:8080` 业务 REST/WebSocket 请求中实现单设备会话和用户数据隔离。

**Architecture:** 使用 Go + SQLite 保存 `users` 和 `user_sessions`，服务端通过随机不透明 Token、Token 哈希和统一认证中间件校验身份；同一用户的新登录会在事务中撤销旧会话。Flutter 使用持久化 Token、持久化 `deviceId`、Dio 全局拦截器和认证启动门禁，确保旧设备请求收到 `401 SESSION_REPLACED` 后自动清理登录态。

**Tech Stack:** Go 1.26、GORM、SQLite、`golang.org/x/crypto/bcrypt`、`net/http`、Gorilla WebSocket、Flutter/Dart、Dio、Provider、SharedPreferences、Go `testing`、Flutter `flutter_test`。

**Spec:** `docs/superpowers/specs/2026-08-30-user-management-single-device-auth-design.md`

## Global Constraints

- `phone` 是唯一登录名；输入去首尾空格；密码至少 6 位；昵称为空时使用账号作为默认昵称。
- 注册成功立即创建会话；登录成功返回用户、Token 和过期时间；Token 默认有效期 30 天。
- 客户端首次启动生成并持久化随机 `deviceId`；卸载、清除应用数据或清除 Web 站点数据视为新设备。
- 公开接口只有注册、登录和健康检查；其他 `/api` 数据接口、上传接口和 `/ws` 必须认证。
- 业务 handler 不信任请求中的 `userId`、`nickname` 或其他身份字段，身份只来自认证上下文。
- 密码只保存哈希；服务端只保存 Token 哈希；日志不得打印密码、完整 Token 或包含 Token 的 WebSocket URL。
- 账号之间的自选股、分组、交易记录和策略社区数据必须隔离；行情、资讯、股票基础资料和选股缓存继续共享。
- 历史无归属数据保留为无用户归属，不自动分配给第一个注册用户，不静默删除。
- 不增加短信验证码、邮箱验证、找回密码、第三方登录、Redis 或独立账号后台。
- 每个任务完成自己的测试后单独提交，提交信息遵循项目现有 Conventional Commits 风格。
- 不触碰当前工作区已有的 `data/stock.db`、`backend/data/stock.db`、T0 缓存和其他无关未跟踪文件。

## 文件与职责地图

| 文件 | 职责 |
| --- | --- |
| `backend/flutter_api/auth_models.go` | 用户、会话、认证输入输出、认证错误和请求上下文所需的纯数据类型 |
| `backend/flutter_api/auth_migrations.go` | 认证表、用户归属字段和索引的幂等迁移 |
| `backend/flutter_api/auth_service.go` | 密码哈希、注册、登录、Token 校验、退出和资料修改 |
| `backend/flutter_api/auth_middleware.go` | Bearer Token 提取、认证上下文和统一 HTTP 认证错误 |
| `backend/flutter_api/auth_http.go` | `/api/auth/*` 路由和 JSON 错误响应 |
| `backend/flutter_api/user_data_service.go` | 面向 HTTP API 的用户范围自选股/分组/交易数据访问 |
| `backend/flutter_api/server.go` | 路由包装、用户范围 handler 接入和 WebSocket 会话注册 |
| `backend/flutter_api/strategy.go` | 将策略社区操作改为使用认证上下文中的用户身份 |
| `backend/data/stock_data_api.go` | 为既有用户数据模型增加可为空的 `user_id` |
| `backend/data/stock_group_api.go` | 为分组模型增加可为空的 `user_id` |
| `trading_app/lib/features/auth/domain/auth_models.dart` | Flutter 用户和会话 JSON 模型 |
| `trading_app/lib/features/auth/data/auth_storage.dart` | Token、用户资料和设备 ID 的持久化抽象与实现 |
| `trading_app/lib/features/auth/data/auth_repository.dart` | 真实认证 API 仓库，替换 Mock 仓库 |
| `trading_app/lib/features/auth/presentation/auth_view_model.dart` | 登录态恢复、登录、注册、资料修改和强制退出状态 |
| `trading_app/lib/features/auth/presentation/auth_gate.dart` | 认证完成前阻止业务页面和远程加载 |
| `trading_app/lib/core/network/api_client.dart` | 自动添加 Bearer Token 和一次性处理 `401` |
| `trading_app/lib/app/app_config.dart` | 认证 Provider 与已认证业务 Provider 的生命周期分层 |
| `trading_app/lib/main.dart` | 使用 `AuthGate` 作为应用首页入口 |

---

### Task 1: 建立 Go 认证模型与独立迁移入口

**Files:**
- Create: `backend/flutter_api/auth_models.go`
- Create: `backend/flutter_api/auth_migrations.go`
- Create: `backend/flutter_api/auth_models_test.go`

**Interfaces:**
- Produces `AuthUser`, `AuthSession`, `RegisterInput`, `LoginInput`, `UpdateProfileInput`, `AuthPrincipal`, `AuthSessionResponse` and `AuthError` for later backend tasks.
- Produces `MigrateAuthTables(dao *gorm.DB) error` for the auth service and server startup.

- [ ] **Step 1: Write the failing migration test**

在 `auth_models_test.go` 创建内存 SQLite GORM 数据库，先断言迁移前没有认证表，再调用迁移入口并断言表、字段和索引存在：

```go
func TestMigrateAuthTablesCreatesUsersAndSessions(t *testing.T) {

	dao := newAuthTestDB(t)

	if dao.Migrator().HasTable(&AuthUser{}) {
		t.Fatal("users should not exist before migration")
	}

	if err := MigrateAuthTables(dao); err != nil {
		t.Fatalf("migrate auth tables: %v", err)
	}

	if !dao.Migrator().HasTable(&AuthUser{}) || !dao.Migrator().HasTable(&AuthSession{}) {
		t.Fatal("auth tables were not created")
	}

	if !dao.Migrator().HasIndex(&AuthUser{}, "idx_auth_users_phone") {
		t.Fatal("phone unique index was not created")
	}

	if !dao.Migrator().HasIndex(&AuthSession{}, "idx_auth_sessions_token_hash") {
		t.Fatal("token hash index was not created")
	}
}
```

测试辅助函数使用 `sqlite.Open("file::memory:?cache=shared")`、`gorm.Open` 和测试结束时关闭底层 `sql.DB`；不得调用全局 `db.Dao`。

- [ ] **Step 2: Run the migration test and verify it fails**

Run: `go test ./backend/flutter_api -run TestMigrateAuthTablesCreatesUsersAndSessions -count=1`

Expected: FAIL because `AuthUser`, `AuthSession` or `MigrateAuthTables` does not yet exist.

- [ ] **Step 3: Add the model types and idempotent migration**

在 `auth_models.go` 定义：

```go
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

type AuthSession struct {
	ID            string     `gorm:"primaryKey;size:64" json:"id"`
	UserID        string     `gorm:"size:64;index;not null" json:"userId"`
	TokenHash     string     `gorm:"size:64;uniqueIndex:idx_auth_sessions_token_hash;not null" json:"-"`
	DeviceID      string     `gorm:"size:128;not null" json:"deviceId"`
	CreatedAt     time.Time  `json:"createdAt"`
	LastSeenAt    time.Time  `json:"lastSeenAt"`
	ExpiresAt     time.Time  `json:"expiresAt"`
	RevokedAt     *time.Time `json:"revokedAt,omitempty"`
	RevokeReason  string     `gorm:"size:30" json:"-"`
}
```

给两种模型增加 `TableName()`，分别返回 `users` 和 `user_sessions`。定义输入和输出类型时，`AuthSessionResponse` 只包含 `PublicUser`、明文 `AccessToken` 和 `ExpiresAt`；数据库模型不直接作为响应暴露。

`MigrateAuthTables` 使用 `AutoMigrate` 创建两张表，并执行：

```sql
CREATE UNIQUE INDEX IF NOT EXISTS idx_auth_sessions_one_active
ON user_sessions(user_id)
WHERE revoked_at IS NULL;
```

函数必须可重复执行，并在任一步失败时返回错误。

- [ ] **Step 4: Run the migration test and verify it passes**

Run: `go test ./backend/flutter_api -run TestMigrateAuthTablesCreatesUsersAndSessions -count=1`

Expected: PASS.

- [ ] **Step 5: Commit the model and migration seam**

```bash
git add backend/flutter_api/auth_models.go backend/flutter_api/auth_migrations.go backend/flutter_api/auth_models_test.go
git commit -m "feat(auth): add user and session models"
```

### Task 2: 实现密码哈希、注册、登录和会话服务

**Files:**
- Create: `backend/flutter_api/auth_service.go`
- Create: `backend/flutter_api/auth_service_test.go`
- Modify: `go.mod`
- Modify: `go.sum`

**Interfaces:**
- Consumes `AuthUser`, `AuthSession`, `AuthSessionResponse`, `AuthError` and `MigrateAuthTables` from Task 1.
- Produces `NewAuthService(dao *gorm.DB, onSessionsReplaced func(userID, newSessionID string)) *AuthService`.
- Produces `Register(ctx context.Context, input RegisterInput) (*AuthSessionResponse, error)`.
- Produces `Login(ctx context.Context, input LoginInput) (*AuthSessionResponse, error)`.
- Produces `Authenticate(ctx context.Context, rawToken string) (AuthPrincipal, error)`.
- Produces `Logout(ctx context.Context, rawToken string) error`.
- Produces `UpdateProfile(ctx context.Context, principal AuthPrincipal, input UpdateProfileInput) (*PublicUser, error)`.

- [ ] **Step 1: Write failing service tests**

在 `auth_service_test.go` 使用 Task 1 的独立内存数据库，覆盖注册、重复账号、错误密码、单设备替换、退出、过期和昵称修改。单设备核心断言如下：

```go
func TestAuthServiceNewLoginReplacesOldDevice(t *testing.T) {

	service := newTestAuthService(t)

	first, err := service.Register(context.Background(), RegisterInput{
		Phone: "13800000000", Password: "secret123", Nickname: "A", DeviceID: "device-a",
	})
	if err != nil {
		t.Fatalf("register: %v", err)
	}

	second, err := service.Login(context.Background(), LoginInput{
		Phone: "13800000000", Password: "secret123", DeviceID: "device-b",
	})
	if err != nil {
		t.Fatalf("login from device b: %v", err)
	}

	if _, err := service.Authenticate(context.Background(), first.AccessToken); !IsAuthCode(err, "SESSION_REPLACED") {
		t.Fatalf("old token error = %v, want SESSION_REPLACED", err)
	}

	principal, err := service.Authenticate(context.Background(), second.AccessToken)
	if err != nil || principal.DeviceID != "device-b" {
		t.Fatalf("new session = %+v, err = %v", principal, err)
	}
}
```

另加断言确认数据库中只有一个 `revoked_at IS NULL` 的会话；不同账号的会话互不影响。

- [ ] **Step 2: Run the service tests and verify they fail**

Run: `go test ./backend/flutter_api -run 'TestAuthService' -count=1`

Expected: FAIL because `AuthService` and its methods do not yet exist.

- [ ] **Step 3: Add bcrypt and implement deterministic service seams**

添加直接依赖 `golang.org/x/crypto`，在 `AuthService` 内注入 `now`、ID 生成器和 Token 生成器，生产环境使用 `time.Now`、密码学安全随机字节和 `bcrypt.GenerateFromPassword`，测试使用固定时钟和固定 Token。

服务实现必须遵循：

1. `Register` trim `Phone`，校验账号长度至少 5、密码至少 6、`DeviceID` 非空；昵称为空时使用 trim 后的账号。
2. 注册事务内创建用户，再撤销该用户已有有效会话并插入新会话；唯一索引冲突转换成 `ACCOUNT_EXISTS`。
3. `Login` 用 `bcrypt.CompareHashAndPassword` 校验密码；账号不存在、密码错误统一返回 `INVALID_CREDENTIALS`。
4. 登录事务先撤销当前用户的有效会话并写入 `RevokeReason = "new_login"`，再创建新会话；提交事务成功后调用 `onSessionsReplaced`。
5. `Authenticate` 通过 Token SHA-256 哈希查找会话，先判断 `RevokeReason`，再判断过期时间和用户状态；有效时更新 `LastSeenAt` 并返回 `AuthPrincipal`。
6. 被 `new_login` 撤销返回 `SESSION_REPLACED`；过期返回 `SESSION_EXPIRED`；其他无效令牌返回 `UNAUTHENTICATED`。
7. `Logout` 按当前 Token 撤销会话，重复退出保持幂等；`UpdateProfile` 只允许修改昵称并重新读取用户资料。

使用以下固定错误结构，不让 service 返回裸字符串：

```go
type AuthError struct {
	Status  int
	Code    string
	Message string
}

func (e *AuthError) Error() string { return e.Message }

func IsAuthCode(err error, code string) bool {
	var authErr *AuthError
	return errors.As(err, &authErr) && authErr.Code == code
}
```

- [ ] **Step 4: Run focused and race tests**

Run: `go test ./backend/flutter_api -run 'TestAuthService' -count=1`

Expected: PASS.

Run: `go test -race ./backend/flutter_api -run 'TestAuthServiceNewLoginReplacesOldDevice' -count=1`

Expected: PASS with no race report.

- [ ] **Step 5: Commit the auth service**

```bash
git add backend/flutter_api/auth_service.go backend/flutter_api/auth_service_test.go go.mod go.sum
git commit -m "feat(auth): add account and session service"
```

### Task 3: 增加认证 HTTP API 和统一 REST 中间件

**Files:**
- Create: `backend/flutter_api/auth_middleware.go`
- Create: `backend/flutter_api/auth_http.go`
- Create: `backend/flutter_api/auth_http_test.go`
- Modify: `backend/flutter_api/server.go`

**Interfaces:**
- Consumes `AuthService` and `AuthError` from Task 2.
- Produces `PrincipalFromContext(ctx context.Context) (AuthPrincipal, bool)`.
- Produces `RequireAuth(service *AuthService, next http.Handler) http.Handler`.
- Produces `NewAuthHTTPHandler(service *AuthService) http.Handler`.
- Produces `WriteAuthError(w http.ResponseWriter, err error)`.

- [ ] **Step 1: Write failing middleware and endpoint tests**

在 `auth_http_test.go` 创建 `httptest.NewServer` 或 `httptest.NewRecorder`，先验证以下行为：

```go
func TestRequireAuthRejectsMissingToken(t *testing.T) {

	service := newTestAuthService(t)
	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		WriteJSON(w, map[string]string{"status": "ok"})
	})
	req := httptest.NewRequest(http.MethodGet, "/api/news", nil)
	rec := httptest.NewRecorder()

	RequireAuth(service, next).ServeHTTP(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d, want 401", rec.Code)
	}
	if !strings.Contains(rec.Body.String(), "UNAUTHENTICATED") {
		t.Fatalf("body = %s, want auth code", rec.Body.String())
	}
}
```

同时写注册成功、登录成功、`/me` 返回用户、旧 Token 返回 `SESSION_REPLACED`、`/api/health` 公开和认证请求能从上下文读取用户的测试。

- [ ] **Step 2: Run the HTTP tests and verify they fail**

Run: `go test ./backend/flutter_api -run 'TestRequireAuth|TestAuthHTTP' -count=1`

Expected: FAIL because the middleware、HTTP handler 和 auth routes 尚不存在。

- [ ] **Step 3: Implement auth context, error response and auth routes**

在 `auth_middleware.go` 使用私有 context key 保存 `AuthPrincipal`。Token 提取只接受大小写不敏感的 `Bearer ` 前缀，并拒绝空 Token。

在 `auth_http.go` 实现：

- `POST /api/auth/register`：解析输入，调用 `Register`，成功返回 `201`。
- `POST /api/auth/login`：解析输入，调用 `Login`，成功返回 `200`。
- `GET /api/auth/me`：从上下文读取 principal，返回用户和会话过期时间。
- `POST /api/auth/logout`：调用 `Logout`，成功返回 `200`，重复退出幂等。
- `PATCH /api/auth/profile`：只接受昵称，调用 `UpdateProfile`。

统一写出：

```json
{
  "code": "SESSION_REPLACED",
  "message": "账号已在其他设备登录，请重新登录"
}
```

`WriteAuthError` 必须先设置 HTTP 状态再编码 JSON；不能返回 `200` + `error`。

- [ ] **Step 4: Wrap routes without changing public health behavior**

在 `server.go` 抽出一个只负责注册路由的 `newHTTPHandler(authService *AuthService) http.Handler`，让 `Start()` 继续负责后台任务和监听端口。把公开路径限制为：

```go
var publicPaths = map[string]bool{
	"/api/auth/register": true,
	"/api/auth/login":    true,
	"/api/health":        true,
}
```

认证中间件对 `OPTIONS` 直接交给 CORS 中间件处理；其他路径包括 `/api/auth/me`、`/api/auth/logout`、`/api/auth/profile`、所有业务 `/api`、`/uploads/` 和 `/ws` 均必须认证。`Start()` 在监听前调用 `MigrateAuthTables(db.Dao)`，迁移失败记录错误并停止启动 HTTP 服务。

- [ ] **Step 5: Run HTTP tests and full Go package tests**

Run: `go test ./backend/flutter_api -run 'TestRequireAuth|TestAuthHTTP' -count=1`

Expected: PASS.

Run: `go test ./backend/flutter_api -count=1`

Expected: PASS.

- [ ] **Step 6: Commit the HTTP auth layer**

```bash
git add backend/flutter_api/auth_middleware.go backend/flutter_api/auth_http.go backend/flutter_api/auth_http_test.go backend/flutter_api/server.go
git commit -m "feat(auth): protect flutter api routes"
```

### Task 4: 增加用户数据归属字段和用户范围数据服务

**Files:**
- Create: `backend/flutter_api/user_data_service.go`
- Create: `backend/flutter_api/user_data_service_test.go`
- Modify: `backend/flutter_api/auth_migrations.go`
- Modify: `backend/data/stock_data_api.go`
- Modify: `backend/data/stock_group_api.go`

**Interfaces:**
- Consumes `AuthPrincipal` and `MigrateAuthTables` from earlier tasks.
- Produces `MigrateUserOwnedData(dao *gorm.DB) error`.
- Produces `NewUserDataService(dao *gorm.DB) *UserDataService`.
- Produces `ListFollowedStocks(ctx context.Context, userID string, groupID uint) ([]data.FollowedStock, error)`.
- Produces `Follow(ctx context.Context, userID, stockCode string) (string, error)`.
- Produces `Unfollow(ctx context.Context, userID, stockCode string) (string, error)`.
- Produces `OwnsGroup(ctx context.Context, userID string, groupID uint) (bool, error)`.

- [ ] **Step 1: Write failing ownership and legacy-data tests**

在 `user_data_service_test.go` 为两个用户创建相同股票代码的自选记录，断言每个用户只能读到自己的记录；插入 `user_id = NULL` 的历史记录，断言任何新用户都读不到：

```go
func TestUserDataServiceDoesNotCrossUserBoundary(t *testing.T) {

	dao := newUserDataTestDB(t)
	if err := MigrateUserOwnedData(dao); err != nil {
		t.Fatalf("migrate user data: %v", err)
	}

	userA := "user-a"
	userB := "user-b"
	legacy := data.FollowedStock{StockCode: "sh600000", Name: "legacy"}
	ownedA := data.FollowedStock{UserID: &userA, StockCode: "sh600000", Name: "A"}
	ownedB := data.FollowedStock{UserID: &userB, StockCode: "sh600000", Name: "B"}
	if err := dao.Create(&legacy).Error; err != nil {
		t.Fatal(err)
	}
	if err := dao.Create(&ownedA).Error; err != nil {
		t.Fatal(err)
	}
	if err := dao.Create(&ownedB).Error; err != nil {
		t.Fatal(err)
	}

	service := NewUserDataService(dao)
	items, err := service.ListFollowedStocks(context.Background(), userA, 0)
	if err != nil || len(items) != 1 || items[0].Name != "A" {
		t.Fatalf("user A items = %+v, err = %v", items, err)
	}
}
```

补充测试组归属、删除他人记录返回不存在、用户 + 股票代码唯一索引和迁移前后旧记录数量不变。

- [ ] **Step 2: Run ownership tests and verify they fail**

Run: `go test ./backend/flutter_api -run 'TestUserDataService' -count=1`

Expected: FAIL because user ownership fields、migration and scoped service do not yet exist。

- [ ] **Step 3: Add nullable ownership fields without changing legacy desktop semantics**

在 `data.FollowedStock`、`data.Group`、`data.GroupStock` 和 `data.TradingRecord` 增加：

```go
UserID *string `json:"userId,omitempty" gorm:"index"`
```

保持现有 Wails 直接绑定方法签名不变；这些方法继续代表本地单用户行为，未显式设置用户 ID 的旧写入使用 `NULL`，不会被新 Flutter HTTP 范围查询读取。

在 `MigrateUserOwnedData` 中 `AutoMigrate` 四个模型，创建以下幂等索引：

```sql
CREATE UNIQUE INDEX IF NOT EXISTS idx_followed_stock_user_code
ON followed_stock(user_id, stock_code)
WHERE user_id IS NOT NULL;

CREATE INDEX IF NOT EXISTS idx_stock_groups_user_sort
ON stock_groups(user_id, sort);

CREATE INDEX IF NOT EXISTS idx_group_stock_group_code
ON group_stock_info(group_id, stock_code);

CREATE INDEX IF NOT EXISTS idx_trading_records_user_time
ON trading_records(user_id, trading_time);
```

迁移只添加字段和索引，不更新、删除或重新归属历史行；用迁移前后 `COUNT(*)` 断言保留旧数据。

- [ ] **Step 4: Implement scoped service operations**

`UserDataService` 的查询都显式带 `user_id = userID`，分组查询同时通过所属用户校验。`Follow` 复用现有行情获取逻辑生成自选记录，但写入 `UserID: &userID`；`Unfollow` 只删除当前用户的记录；空结果使用“未找到当前用户记录”而不是跨用户回退查询。

在 `auth_migrations.go` 增加组合迁移入口，使 `MigrateUserOwnedData` 可单独测试并可由服务器重复执行。公共行情查询不得增加 `user_id` 过滤。

- [ ] **Step 5: Run ownership tests and related existing tests**

Run: `go test ./backend/flutter_api -run 'TestUserDataService' -count=1`

Expected: PASS.

Run: `go test ./backend/data -run 'Test.*Stock|Test.*Trading' -count=1`

Expected: PASS or report no matching tests without changing existing test behavior.

- [ ] **Step 6: Commit user data isolation**

```bash
git add backend/flutter_api/auth_migrations.go backend/flutter_api/user_data_service.go backend/flutter_api/user_data_service_test.go backend/data/stock_data_api.go backend/data/stock_group_api.go
git commit -m "feat(auth): scope user data by account"
```

### Task 5: 接入业务 handler、策略身份和 WebSocket 会话

**Files:**
- Create: `backend/flutter_api/server_auth_integration_test.go`
- Modify: `backend/flutter_api/server.go`
- Modify: `backend/flutter_api/strategy.go`
- Modify: `backend/flutter_api/user_data_service.go`

**Interfaces:**
- Consumes `PrincipalFromContext`, `RequireAuth`, `UserDataService` and `AuthService` from Tasks 2–4.
- Produces an HTTP router where all protected route categories enforce auth.
- Produces WebSocket clients associated with `UserID` and `SessionID` so old connections close after session replacement.

- [ ] **Step 1: Write failing integration tests for protected routes and identity source**

在 `server_auth_integration_test.go` 使用 `newHTTPHandler` 和内存认证服务，验证：

- 未登录访问 `/api/news`、`/api/follow-list`、`/api/upload` 和 `/ws` 不会进入业务 handler。
- 登录后的请求进入业务 handler，并能从 context 读取真实用户。
- `/api/health` 和认证路由保持公开。
- 策略请求体伪造 `userId = "other-user"` 时，服务端仍使用 Token 对应用户。

测试探针使用一个仅记录 principal 的 handler，不调用外部行情服务：

```go
func principalProbe(w http.ResponseWriter, r *http.Request) {

	principal, ok := PrincipalFromContext(r.Context())
	if !ok {
		WriteAuthError(w, &AuthError{Status: http.StatusUnauthorized, Code: "UNAUTHENTICATED", Message: "missing principal"})
		return
	}
	WriteJSON(w, map[string]string{"userId": principal.UserID})
}
```

- [ ] **Step 2: Run integration tests and verify they fail**

Run: `go test ./backend/flutter_api -run 'TestProtected|TestStrategyIdentity|TestWebSocketAuth' -count=1`

Expected: FAIL because existing handlers are not yet wrapped and strategy still trusts request fields。

- [ ] **Step 3: Replace follow handlers with scoped service calls**

修改 `handleFollow`、`handleUnfollow` 和 `handleGetFollowList`：

1. 先从 `PrincipalFromContext` 获取 `userID`。
2. 将股票代码和分组参数交给 `UserDataService`。
3. 不再调用无用户范围的 `data.NewStockDataApi().Follow`、`UnFollow`、`GetFollowList`。
4. 保留裸代码 `sh`/`sz` 重试行为，但每次重试都使用同一个当前用户 ID。

修改 `handleStrategy` 的所有 `points`、`checkin_status`、`liked`、`today_reply_points` 和 POST 操作：

- GET 的 `userId` 查询参数只保留兼容解析，不参与身份选择。
- POST 的 `userId`、`nickname` 字段忽略，改用 principal 的 `UserID` 和 `Nickname`。
- 帖子、评论、点赞、删除和积分操作在数据库查询层继续检查 owner。

- [ ] **Step 4: Associate and close WebSocket sessions**

将现有 `map[*websocket.Conn]bool` 改为保存以下结构：

```go
type wsClient struct {
	conn      *websocket.Conn
	userID    string
	sessionID string
}
```

WebSocket 握手从 `Authorization` Header 读取 Token；为浏览器兼容性支持 `access_token` 查询参数，但禁止把完整 URL 写入日志。握手通过 `AuthService.Authenticate` 后才升级连接。

新增 `closeWebSocketsForUser(userID, exceptSessionID string)`，登录事务提交后的 `onSessionsReplaced` 回调调用它，关闭同一用户旧 session 的连接。关闭动作使用现有 `wsClientsMu`，不得在读锁下删除 map 项。

- [ ] **Step 5: Run route, strategy and WebSocket tests**

Run: `go test ./backend/flutter_api -run 'TestProtected|TestStrategyIdentity|TestWebSocketAuth' -count=1`

Expected: PASS.

Run: `go test ./backend/flutter_api -count=1`

Expected: PASS.

- [ ] **Step 6: Commit protected business integration**

```bash
git add backend/flutter_api/server.go backend/flutter_api/server_auth_integration_test.go backend/flutter_api/strategy.go backend/flutter_api/user_data_service.go
git commit -m "feat(auth): bind api actions to authenticated users"
```

### Task 6: 替换 Flutter Mock 仓库并持久化会话和设备 ID

**Files:**
- Create: `trading_app/lib/features/auth/data/auth_storage.dart`
- Create: `trading_app/test/features/auth/auth_repository_test.dart`
- Modify: `trading_app/lib/features/auth/domain/auth_models.dart`
- Modify: `trading_app/lib/features/auth/data/auth_repository.dart`
- Modify: `trading_app/lib/core/storage/local_cache.dart`

**Interfaces:**
- Produces `AuthStorage` with `read`, `write`, `clearAuth` and `getOrCreateDeviceId`.
- Produces `ApiAuthRepository({required Dio dio, required AuthStorage storage})` implementing the existing `AuthRepository` interface.
- Keeps `MemoryLocalCache` available for unrelated cache tests; authentication tests use an in-memory `AuthStorage` fake.

- [ ] **Step 1: Write failing Dart model and repository tests**

在 `auth_repository_test.dart` 覆盖模型解析、设备 ID 稳定性、登录请求字段、注册请求字段和会话持久化。测试要断言同一个 storage 两次取得的设备 ID 相同：

```dart
test('device id is generated once and reused', () async {
  final storage = MemoryAuthStorage();
  final first = await storage.getOrCreateDeviceId();
  final second = await storage.getOrCreateDeviceId();

  expect(first, isNotEmpty);
  expect(second, first);
});
```

Repository 测试使用 Dio 的测试 adapter 返回固定 JSON，断言登录请求包含 `phone`、`password` 和持久化的 `deviceId`，成功后 `auth:token`、用户信息和 `auth:expiresAt` 已写入 storage。

- [ ] **Step 2: Run Flutter auth tests and verify they fail**

Run: `cd trading_app && flutter test test/features/auth/auth_repository_test.dart`

Expected: FAIL because `ApiAuthRepository`、`AuthStorage`、JSON parsing and device persistence do not yet exist。

- [ ] **Step 3: Add session models and storage implementation**

给 `AppUser` 和 `AuthSession` 增加 `fromJson` / `toJson`；`AuthSession` 增加 `expiresAt`，并继续保留 `accessToken` 和 `user`。

在 `auth_storage.dart` 定义：

```dart
abstract class AuthStorage {
  Future<String?> read(String key);
  Future<void> write(String key, String value);
  Future<void> clearAuth();
  Future<String> getOrCreateDeviceId();
}
```

生产实现使用 `SharedPreferences.getInstance()`，设备 ID 使用 `Random.secure()` 生成 32 个十六进制字符；测试实现使用 Map。`clearAuth` 只清理 `auth:*`，不得清理 `device:id`。

- [ ] **Step 4: Implement `ApiAuthRepository`**

在现有 `auth_repository.dart` 中保留 `AuthRepository` 抽象，删除生产路径对 `MockAuthRepository` 的依赖并实现：

- `restoreSession`：读取 Token；没有 Token 返回 null；有 Token 调用 `/api/auth/me`；失败时清理 auth 数据并重新抛出认证异常。
- `login`：调用 `/api/auth/login`，自动添加 `deviceId`，保存响应并返回 `AuthSession`。
- `register`：调用 `/api/auth/register`，自动添加 `deviceId`，保存响应并返回 `AuthSession`。
- `updateNickname`：调用 `PATCH /api/auth/profile`，更新本地用户资料。
- `logout`：最佳努力调用 `/api/auth/logout`，无论网络结果如何都清理本地 auth 数据。

将服务端错误解析为带 `code`、`message` 和 `statusCode` 的 `AuthApiException`，界面层显示服务端 `message`，不显示 Dio 内部堆栈。

- [ ] **Step 5: Run Flutter auth tests and analyze**

Run: `cd trading_app && flutter test test/features/auth/auth_repository_test.dart`

Expected: PASS.

Run: `cd trading_app && flutter analyze`

Expected: no analyzer errors caused by the auth model/repository changes.

- [ ] **Step 6: Commit the Flutter auth repository**

```bash
git add trading_app/lib/features/auth/domain/auth_models.dart trading_app/lib/features/auth/data/auth_storage.dart trading_app/lib/features/auth/data/auth_repository.dart trading_app/lib/core/storage/local_cache.dart trading_app/test/features/auth/auth_repository_test.dart
git commit -m "feat(auth): connect flutter auth to api"
```

### Task 7: 增加 Dio 会话拦截器、认证状态和启动门禁

**Files:**
- Create: `trading_app/lib/features/auth/presentation/auth_gate.dart`
- Create: `trading_app/test/features/auth/auth_gate_test.dart`
- Modify: `trading_app/lib/core/network/api_client.dart`
- Modify: `trading_app/lib/features/auth/presentation/auth_view_model.dart`
- Modify: `trading_app/lib/app/app_config.dart`
- Modify: `trading_app/lib/main.dart`

**Interfaces:**
- Produces `SessionInvalidationReason` with `replaced`, `expired` and `revoked` values.
- Produces a session controller or equivalent observable used by both `AuthViewModel` and the Dio interceptors.
- Changes `createApiClient` to accept the session controller while preserving `baseUrl` override and `resetApiClientForTesting`.

- [ ] **Step 1: Write failing interceptor and gate tests**

测试至少覆盖：

- 普通请求自动带 `Authorization: Bearer token`。
- 认证接口通过 `Options.extra['skipAuth'] = true` 不携带旧 Token。
- 401 `SESSION_REPLACED` 清理 auth 数据并通知一次。
- `AuthGate` 在 restore loading 时不构建 `AppShell`，restore 成功后才构建 `AppShell`，失败后构建 `LoginPage`。

会话失效通知次数测试使用两条同时失败的请求，并断言清理回调只发生一次。

- [ ] **Step 2: Run gate/interceptor tests and verify they fail**

Run: `cd trading_app && flutter test test/features/auth/auth_gate_test.dart`

Expected: FAIL because the client has no auth interceptor, invalidation controller or AuthGate。

- [ ] **Step 3: Implement observable session controller**

在 auth 数据层或 core auth 支持层定义一个 ChangeNotifier 兼容的会话控制器，至少提供：

```dart
enum SessionInvalidationReason { replaced, expired, revoked }

abstract class SessionController {
  Future<String?> readToken();
  Future<void> save(AuthSession session);
  Future<void> clear({SessionInvalidationReason? reason});
  SessionInvalidationReason? get lastInvalidationReason;
  bool get isInvalidating;
}
```

`clear` 使用异步锁保证并发 401 只清理一次，并保留最近一次失效原因供 `AuthViewModel` 显示提示。

- [ ] **Step 4: Implement Dio request/error interceptors**

修改 `_buildApiClient`：

1. `onRequest` 检查 `options.extra['skipAuth'] != true`，读取 Token 后设置 `Authorization`。
2. `ApiAuthRepository` 的 login/register/me/logout/profile 请求按需要设置 `skipAuth`；`me` 在恢复时使用当前 Token，其他 auth 请求不携带旧 Token。
3. `onError` 仅对 HTTP 401 解析 JSON `code`；`SESSION_REPLACED`、`SESSION_EXPIRED` 和未认证错误调用 session controller 的 `clear`。
4. 不自动重试失效请求，直接把原始 Dio 错误交给调用方。

- [ ] **Step 5: Add AuthGate and defer business Providers**

调整 `AuthViewModel` 监听 session controller：

- `restore` 期间状态为 loading。
- `/me` 成功后写入用户和 Token，状态 ready。
- `SESSION_REPLACED` 显示“账号已在其他设备登录，请重新登录”，清理用户并进入未登录状态。
- 普通过期显示“登录状态已失效，请重新登录”。

创建 `AuthGate`：

```dart
class AuthGate extends StatelessWidget {
  const AuthGate({super.key});

  @override
  Widget build(BuildContext context) {
    final auth = context.watch<AuthViewModel>();
    if (auth.state.isLoading || auth.state.status == ViewStatus.idle) {
      return const Center(child: CircularProgressIndicator());
    }
    if (!auth.isLoggedIn) {
      return const LoginPage();
    }
    return const AuthenticatedDependencies(child: AppShell());
  }
}
```

将 `AppDependencies` 拆为两层：外层只创建 session controller、Dio、`ApiAuthRepository` 和 `AuthViewModel`; 内层 `AuthenticatedDependencies` 才创建 `RadarViewModel`、`ShortTermEmotionViewModel`、`NewsViewModel`、`T0StrategyViewModel` 和 `StrategyViewModel`。把当前 app config 中的 `.loadMonitoredStocks()` 和短线情绪 `.load()` 移入已认证层；`RadarViewModel` 的周期刷新只能在已认证层存在。

修改 `main.dart` 将 `home` 改为 `AuthGate`，保留主题 Provider 在认证门禁外层。

- [ ] **Step 6: Run Flutter gate tests and current suite**

Run: `cd trading_app && flutter test test/features/auth/auth_gate_test.dart`

Expected: PASS.

Run: `cd trading_app && flutter test`

Expected: PASS.

- [ ] **Step 7: Commit session interception and startup gating**

```bash
git add trading_app/lib/core/network/api_client.dart trading_app/lib/features/auth/presentation/auth_view_model.dart trading_app/lib/features/auth/presentation/auth_gate.dart trading_app/lib/app/app_config.dart trading_app/lib/main.dart trading_app/test/features/auth/auth_gate_test.dart
git commit -m "feat(auth): gate flutter data behind session"
```

### Task 8: 更新登录界面、资料操作和策略调用契约

**Files:**
- Create: `trading_app/test/features/auth/auth_view_model_test.dart`
- Modify: `trading_app/lib/features/auth/presentation/login_page.dart`
- Modify: `trading_app/lib/features/auth/presentation/register_page.dart`
- Modify: `trading_app/lib/features/auth/presentation/auth_view_model.dart`
- Modify: `trading_app/lib/features/profile/presentation/profile_page.dart`
- Modify: `trading_app/lib/features/profile/presentation/edit_profile_page.dart`
- Modify: `trading_app/lib/features/profile/presentation/system_settings_page.dart`
- Modify: `trading_app/lib/features/strategy/data/strategy_repository.dart`
- Modify: `trading_app/lib/features/strategy/presentation/strategy_view_model.dart`
- Modify: `trading_app/lib/features/strategy/presentation/strategy_page.dart`
- Modify: `trading_app/lib/features/strategy/presentation/create_post_page.dart`

**Interfaces:**
- Consumes `ApiAuthRepository`, session controller and `AuthGate` from Tasks 6–7.
- Changes strategy repository methods to omit `userId` and `nickname`; the server owns identity.
- Produces UI behavior that shows backend validation messages and never ships demo credentials.

- [ ] **Step 1: Write failing UI/state tests**

在 `auth_view_model_test.dart` 覆盖：

- 登录成功后 `user` 和 `accessToken` 有值。
- `ACCOUNT_EXISTS` 显示服务端注册错误。
- `SESSION_REPLACED` 清空用户并记录替换提示。
- 退出登录后用户、Token 和业务登录态为空。

增加一个 Widget 测试，确认登录页的手机号和密码输入框默认为空，不再预填示例账号。

- [ ] **Step 2: Run the tests and verify they fail**

Run: `cd trading_app && flutter test test/features/auth/auth_view_model_test.dart`

Expected: FAIL because the existing ViewModel does not consume remote auth errors or session invalidation reasons, and login currently预填示例账号。

- [ ] **Step 3: Update ViewModel and auth UI copy**

修改 `AuthViewModel`：

- 捕获 `AuthApiException` 时使用其 `message`。
- 将 session controller 的失效原因映射为用户可读提示。
- `logout` 先调用仓库再清理本地状态，网络失败也必须清理。
- `updateNickname` 成功后同步本地用户，失败时不关闭编辑页。

修改登录页：

- 移除 `TextEditingController(text: ...)` 的示例账号和密码。
- 登录按钮只在字段通过基础校验且不 loading 时启用。
- 直接展示服务端的账号不存在、密码错误和会话错误消息。

修改注册页：

- 保留手机号/账号、密码、昵称字段。
- 客户端基础校验与服务端规则一致：账号至少 5、密码至少 6。
- 注册成功保持返回已登录状态。

- [ ] **Step 4: Remove client-supplied identity from strategy calls**

修改 `StrategyRepository`：

- `viewPost(postId)`、`checkIn()`、`getUserPoints()`、`hasCheckedIn()`、`toggleLike(postId)`、`addComment(postId, ...)`、`deleteComment(commentId)` 和 `deletePost(postId)` 不再接收 `userId`。
- `createPost(title, content, images)` 不再接收 `userId`、`nickname`。
- 请求 body 不再写入这些字段；Dio 从统一客户端附加 Token。

修改 `StrategyViewModel`、策略页和发帖页的调用，页面只依赖 `AuthViewModel.isLoggedIn` 和当前用户展示昵称。登录态被替换后，策略操作不再吞掉 `SESSION_REPLACED`，让全局拦截器完成退出。

资料页和系统设置页继续使用 `AuthViewModel.logout()`；成功退出后由 `AuthGate` 销毁已认证业务 Provider，停止定时刷新。

- [ ] **Step 5: Run focused UI tests and analyze**

Run: `cd trading_app && flutter test test/features/auth/auth_view_model_test.dart`

Expected: PASS.

Run: `cd trading_app && flutter analyze`

Expected: no analyzer errors and no calls to strategy repository with user identity parameters。

- [ ] **Step 6: Commit auth UI and strategy contract changes**

```bash
git add trading_app/lib/features/auth trading_app/lib/features/profile trading_app/lib/features/strategy trading_app/test/features/auth/auth_view_model_test.dart
git commit -m "feat(auth): finish login and identity flow"
```

### Task 9: 完成迁移验证、端到端验收和交付检查

**Files:**
- Create: `backend/flutter_api/user_management_e2e_test.go`
- Create: `trading_app/test/features/auth/single_device_flow_test.dart`

**Interfaces:**
- Consumes the complete backend and Flutter auth flows from Tasks 1–8.
- Produces reproducible two-device acceptance coverage and final verification evidence.

- [ ] **Step 1: Write the end-to-end two-device test**

使用同一个测试数据库和 HTTP handler，完成以下序列：

1. Device A 注册并保存 Token A。
2. Device A 请求受保护数据成功。
3. Device B 使用相同账号登录并保存 Token B。
4. Token A 请求返回 `401 SESSION_REPLACED`。
5. Token B 请求成功。
6. 账号 B 的用户范围查询看不到账号 A 的自选股、分组和交易记录。
7. 退出 Token B 后，Token B 再请求返回 `401`。

Flutter 测试使用两个独立 `MemoryAuthStorage` 和两个 Dio client，确认 Device A 的 session controller 清理而 Device B 保持登录。

- [ ] **Step 2: Run backend and Flutter verification**

Run: `go test ./backend/flutter_api -count=1`

Expected: PASS.

Run: `go test ./... -count=1`

Expected: PASS, with any pre-existing unrelated failures recorded rather than hidden。

Run: `cd trading_app && flutter test`

Expected: PASS.

Run: `cd trading_app && flutter analyze`

Expected: no analyzer errors.

- [ ] **Step 3: Manually verify the real API flow**

启动后端：

```bash
go run ./cmd/server
```

用两个独立客户端或两个浏览器存储空间调用：

```bash
curl -i -X POST http://localhost:8080/api/auth/register \
  -H 'Content-Type: application/json' \
  -d '{"phone":"13800000000","password":"secret123","nickname":"设备A","deviceId":"device-a"}'

curl -i -X POST http://localhost:8080/api/auth/login \
  -H 'Content-Type: application/json' \
  -d '{"phone":"13800000000","password":"secret123","deviceId":"device-b"}'

curl -i http://localhost:8080/api/news \
  -H 'Authorization: Bearer <token-a>'
```

Expected: Device B 登录后，最后一个请求返回 `401` 和 `SESSION_REPLACED`；用 Token B 请求同一接口成功。

- [ ] **Step 4: Check migration safety and workspace scope**

确认：

- 旧用户数据行数量未减少。
- 旧 `user_id = NULL` 行不会被新账号查询返回。
- `git diff --stat` 只包含本功能代码、测试和计划/规格文件。
- `git status --short` 中已有数据库和缓存变更仍未被暂存。

- [ ] **Step 5: Commit the end-to-end verification tests**

端到端测试通过后，使用：

```bash
git add backend/flutter_api/user_management_e2e_test.go trading_app/test/features/auth/single_device_flow_test.dart
git commit -m "test(auth): cover single-device session flow"
```

不要提交数据库、缓存、构建产物或测试生成文件。
