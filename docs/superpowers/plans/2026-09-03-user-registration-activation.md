# Flutter 用户注册后管理员启用实施计划

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans when executing this plan.

**Goal:** 将现有 Flutter + Go 用户认证改为“注册即创建 disabled 用户，管理员启用后才能登录”；注册本身不创建会话、不返回 Token，也不能进入业务页面。

**Architecture:** 保留现有 users、user_sessions、AuthService、管理员状态接口、ApiAuthRepository、AuthGate 和业务依赖注入结构。Go 注册服务只创建 disabled 用户并返回注册结果，登录服务在密码校验通过后拒绝非 active 用户；Flutter 注册单独解析注册结果，不写入本地认证存储；AuthGate 只在已有有效用户和 Token 时挂载 AppShell；受保护请求遇到 401 或 403 ACCOUNT_DISABLED 时清理普通用户会话。

**Tech Stack:** Go、GORM、bcrypt、net/http、Go test；Flutter/Dart、Dio、Provider、SharedPreferences、Flutter test；现有 frontend 管理页面和 npm 构建链路。

**Spec:** [2026-08-30-user-management-single-device-auth-design.md](../../specs/2026-08-30-user-management-single-device-auth-design.md)

## Global Constraints

- 本计划是已确认设计的执行版本；不得恢复注册即登录、注册创建 user_sessions 或管理员启用后代替用户登录的行为。
- 新注册用户必须写入 users.status=disabled；只有管理员把状态改为 active 后，登录成功才允许创建会话。
- 注册成功响应不得包含 accessToken、expiresAt、sessionId 或设备信息。旧客户端发送的 deviceId 可以被服务端兼容接收，但注册不要求、不保存、不使用；登录仍要求有效 deviceId。
- 用户不存在、密码错误、无 Token、Token 失效或 disabled 用户均只能处于普通用户未登录态；不能挂载 AppShell、业务 Provider、业务 REST 请求或业务 WebSocket。
- 管理员登录和管理员状态接口是独立后台认证通道；复用现有后台页面和 PATCH /api/admin/users/{id}/status。
- 已有 active 用户不做批量重置，已有 disabled 用户不自动启用；不新增数据库表、字段或状态值。
- 保留现有单设备登录、密码哈希、会话替换、禁用撤销会话和用户数据隔离逻辑。
- 当前工作区已有与本需求无关的 DB、Flutter 构建产物、缓存和文档改动；只修改本计划列出的文件，不清理、不回滚、不提交无关改动。
- 每个任务先补失败测试，再实现最小改动，再运行任务级测试；提交信息沿用项目现有 conventional commits 风格。

---

## 1. 闭环与执行顺序

~~~~mermaid
flowchart TD
    START[Flutter 启动] --> RESTORE{本地 Token 能否通过 /api/auth/me}
    RESTORE -- 否 --> LOGIN[未登录入口 LoginPage]
    RESTORE -- 是 --> APP[挂载 AuthenticatedDependencies 与 AppShell]
    LOGIN --> REGISTER[提交 POST /api/auth/register]
    REGISTER --> CREATED[创建 users.status=disabled]
    CREATED --> RESULT[返回注册结果，无 Token，无会话]
    RESULT --> WAIT[提示等待管理员启用，回到登录页]
    WAIT --> ADMIN[管理员把 status 改为 active]
    ADMIN --> LOGIN2[用户再次提交登录]
    LOGIN2 --> PASSWORD{密码正确}
    PASSWORD -- 否 --> INVALID[401 INVALID_CREDENTIALS，保持未登录]
    PASSWORD -- 是 --> STATUS{status=active}
    STATUS -- 否 --> DISABLED[403 ACCOUNT_DISABLED，保持未登录]
    STATUS -- 是 --> SESSION[创建单设备会话并返回 Token]
    SESSION --> APP
    APP --> DISABLE[管理员把用户改为 disabled]
    DISABLE --> REVOKE[撤销会话或后续请求返回 403]
    REVOKE --> CLEAR[Flutter 清理会话并卸载业务依赖]
    CLEAR --> LOGIN
~~~~

状态不变量：

| 状态 | 普通用户会话 | Flutter 入口 | 业务访问 |
| --- | --- | --- | --- |
| 无用户或无本地 Token | 否 | LoginPage / RegisterPage | 否 |
| 注册成功但未启用 | 否 | 登录入口和等待启用提示 | 否 |
| disabled 用户尝试登录 | 否 | 登录页及 ACCOUNT_DISABLED 消息 | 否 |
| active 用户登录成功 | 是 | AppShell | 是 |
| 已登录后被管理员禁用 | 会话撤销或认证拒绝 | 登录入口 | 否 |

执行顺序固定为：后端领域契约和服务 → 后端 HTTP/管理员闭环 → Flutter 模型、仓库和 ViewModel → Flutter 页面与依赖门禁 → 全量验证。后一个任务依赖前一个任务的接口和测试结果。

## 2. 文件与责任边界

| 任务 | 修改文件 | 责任 |
| --- | --- | --- |
| 后端注册/登录服务 | backend/flutter_api/auth_models.go；backend/flutter_api/auth_service.go；backend/flutter_api/auth_service_test.go | 注册结果类型、disabled 默认值、无会话注册、密码后状态门禁 |
| 后端 HTTP/后台链路 | backend/flutter_api/auth_http.go；backend/flutter_api/auth_http_test.go；backend/flutter_api/admin_http_test.go；backend/flutter_api/admin_service_test.go；backend/flutter_api/server_auth_integration_test.go；backend/flutter_api/user_management_e2e_test.go | 注册响应、注册到启用再登录闭环、后台回归 |
| Flutter 认证数据层 | trading_app/lib/features/auth/domain/auth_models.dart；trading_app/lib/features/auth/data/auth_repository.dart；trading_app/lib/features/auth/presentation/auth_view_model.dart；trading_app/lib/core/network/api_client.dart；trading_app/lib/features/auth/data/session_controller.dart | 注册结果与登录会话分型、注册不持久化、认证失效清理 |
| Flutter 门禁与页面 | trading_app/lib/features/auth/presentation/register_page.dart；trading_app/lib/features/auth/presentation/auth_gate.dart；trading_app/lib/app/app_config.dart；对应 auth 测试 | 注册提示、未登录分支、业务 Provider 生命周期 |
| 集成验收 | 上述全部测试及 frontend 构建 | 证明无用户/未启用用户不能使用，启用后可以登录使用 |

## 3. Task 1：调整 Go 注册模型和 AuthService

**Files:**

- Modify: backend/flutter_api/auth_models.go
- Modify: backend/flutter_api/auth_service.go
- Modify: backend/flutter_api/auth_service_test.go

### Step 1: 先写服务层失败测试

在 auth_service_test.go 将旧的 TestAuthServiceRegisterCreatesUserAndSession 改为 TestAuthServiceRegisterCreatesDisabledUserWithoutSession，保留账号、默认昵称和 bcrypt 断言，并增加：

~~~~go
result, err := service.Register(context.Background(), RegisterInput{
    Phone: " 13800000000 ",
    Password: "secret123",
    Nickname: " ",
})
if err != nil {
    t.Fatalf("register: %v", err)
}
if result.Status != authStatusDisabled {
    t.Fatalf("status = %q, want %q", result.Status, authStatusDisabled)
}
if result.Message != "注册成功，请等待管理员启用后再登录" {
    t.Fatalf("message = %q, want pending activation message", result.Message)
}
assertActiveSessionCount(t, service, result.User.ID, 0)
~~~~

增加 TestAuthServiceDisabledUserCannotLogin：注册后使用正确密码登录，断言 403 ACCOUNT_DISABLED 且 active session 数仍为 0；增加 TestAuthServiceDisabledUserWrongPasswordIsInvalidCredentials：同一 disabled 用户使用错误密码，断言 401 INVALID_CREDENTIALS，不泄露账号状态。

保留现有 active 登录、并发禁用、单设备替换、注销和 Authenticate 测试，但把其前置步骤统一改为 Register → newTestAdminService(service.dao).UpdateUserStatus(..., authStatusActive) → Login。

### Step 2: 运行失败测试，确认测试锁定旧行为

执行：

~~~~sh
cd backend
go test ./flutter_api -run 'TestAuthService(Register|DisabledUser|Login|NewLogin|Logout|Authenticate|UpdateProfile)' -count=1
~~~~

预期当前实现失败：注册仍返回 AuthSessionResponse、状态仍为 active、注册仍创建一条会话。失败是预期结果，不要为了让旧测试通过而放宽断言。

### Step 3: 实现最小服务和模型改动

在 auth_models.go 新增只用于注册成功响应的 RegisterResponse，包含 User、Status、Message 三个 JSON 字段，不包含 accessToken、expiresAt、sessionId 或 deviceId。

在 auth_service.go 将 AuthService.Register 签名改为返回 *RegisterResponse，按以下顺序实现：

1. 保留 phone、password、nickname 校验和 bcrypt.GenerateFromPassword。
2. 保留 RegisterInput.DeviceID 以兼容旧请求，但删除注册时对 deviceID 非空的校验。
3. 创建 AuthUser 时把 Status 固定写为 authStatusDisabled。
4. 事务中只执行 tx.Create(&user)，删除 buildSession、revokeActiveSessionsTx 和 tx.Create(&session)。
5. 成功后返回 PublicUser、disabled 状态和固定中文等待启用消息；不得生成注册 Token，不调用 newToken。
6. 保留重复账号 ACCOUNT_EXISTS、非法参数和密码哈希行为。

在 Login 中先查用户、执行 bcrypt.CompareHashAndPassword，再根据 status 返回 ACCOUNT_DISABLED；disabled 账号的正确密码是 403，错误密码仍是 401。保持登录事务内的行锁、active 条件、旧会话撤销和新会话创建，避免管理员并发禁用后产生会话。

同步整理测试服务的 Token 序列：注册不消耗测试 Token；只有 Login 消耗 token-login 等测试 Token。

### Step 4: 运行服务层测试并检查数据库状态

执行：

~~~~sh
cd backend
gofmt -w flutter_api/auth_models.go flutter_api/auth_service.go flutter_api/auth_service_test.go
go test ./flutter_api -run 'TestAuthService(Register|DisabledUser|Login|NewLogin|Logout|Authenticate|UpdateProfile)' -count=1
~~~~

预期证明新用户 status=disabled、注册后 active session 数为 0、注册后直接登录无会话、启用后登录才有 1 条 active session。

### Step 5: 提交任务 1

~~~~sh
git add backend/flutter_api/auth_models.go backend/flutter_api/auth_service.go backend/flutter_api/auth_service_test.go
git commit -m "fix(auth): default new accounts to disabled"
~~~~

## 4. Task 2：同步 Go HTTP、管理员启用和后端集成测试

**Files:**

- Modify: backend/flutter_api/auth_http.go
- Modify: backend/flutter_api/auth_http_test.go
- Modify: backend/flutter_api/admin_http_test.go
- Modify: backend/flutter_api/admin_service_test.go
- Modify: backend/flutter_api/server_auth_integration_test.go
- Modify: backend/flutter_api/user_management_e2e_test.go

### Step 1: 先写 HTTP 闭环失败测试

在 auth_http_test.go 增加注册响应断言：POST /api/auth/register 返回 201，JSON 含 user、status=disabled、message，且不含 accessToken、expiresAt；请求可以省略 deviceId。

在现有 HTTP/integration 测试中增加完整链路：

1. 普通客户端注册。
2. 读取后台用户列表，确认新用户 status=disabled。
3. 普通客户端直接登录，断言 403 ACCOUNT_DISABLED。
4. 管理员使用现有管理员 Token 调用 PATCH /api/admin/users/{id}/status，提交 status=active。
5. 普通客户端再次登录，断言返回 accessToken 和 expiresAt。
6. 使用该 Token 请求 /api/auth/me 及一个现有业务接口，断言成功。
7. 未携带 Token 请求业务接口，断言 401；普通用户 Token 修改管理员状态，断言被拒绝。

更新 server_auth_integration_test.go、user_management_e2e_test.go 和 admin_http_test.go 中所有依赖注册响应 Token 的 helper：先保存注册返回的用户 ID，再通过管理员登录/状态更新能力启用用户，最后调用普通登录 helper 获取 Token。禁止直接写 users.status 或直接插入 user_sessions。

### Step 2: 运行后端 HTTP 测试，确认旧响应解析和 helper 失败

执行：

~~~~sh
cd backend
go test ./flutter_api -run 'Test(AuthHTTP|Admin|ServerAuth|UserManagement)' -count=1
~~~~

预期旧测试会在注册响应读取 accessToken、或在未启用时直接登录的位置失败；保留闭环断言后再改 handler 和 helper。

### Step 3: 实现 HTTP 响应和后台回归

在 auth_http.go 的 handleAuthRegister 中接收 RegisterResponse，并用 WriteJSONStatus(w, http.StatusCreated, result) 返回。登录 handler 继续接收 AuthSessionResponse，不复用注册类型。保持 register/login 公开、业务路径由 RequireAuth 保护。

核对 admin_service.go 的既有行为并只在测试发现缺口时修改：active/disabled 是唯一状态；启用只更新 users.status，不创建普通用户会话；禁用撤销该用户 active sessions 并触发现有 WebSocket 关闭回调。管理员启用接口不得返回或保存普通用户 Token。

确保 auth_middleware.go 的行为满足：

- 注册和登录公开可访问；
- /api/auth/me、/api/auth/profile、业务 REST 和 WebSocket 无有效普通用户会话时被拒绝；
- Authenticate 对 disabled 用户返回 403 ACCOUNT_DISABLED；
- 管理员接口仍只接受管理员认证。

### Step 4: 运行后端闭环和全量 Go 测试

执行：

~~~~sh
cd backend
gofmt -w flutter_api/auth_http.go flutter_api/auth_http_test.go flutter_api/admin_http_test.go flutter_api/admin_service_test.go flutter_api/server_auth_integration_test.go flutter_api/user_management_e2e_test.go
go test ./flutter_api -run 'Test(AuthHTTP|Admin|ServerAuth|UserManagement|AuthService)' -count=1
go test ./... -count=1
~~~~

预期注册 → 后台看到 disabled → 后台启用 → 用户登录 → 业务访问完整通过，未登录、未启用和禁用后的普通用户均不能进入业务链路。

### Step 5: 提交任务 2

~~~~sh
git add backend/flutter_api/auth_http.go backend/flutter_api/auth_http_test.go backend/flutter_api/admin_http_test.go backend/flutter_api/admin_service_test.go backend/flutter_api/server_auth_integration_test.go backend/flutter_api/user_management_e2e_test.go
git commit -m "test(auth): cover administrator activation flow"
~~~~

## 5. Task 3：拆分 Flutter 注册结果与登录会话

**Files:**

- Modify: trading_app/lib/features/auth/domain/auth_models.dart
- Modify: trading_app/lib/features/auth/data/auth_repository.dart
- Modify: trading_app/lib/features/auth/presentation/auth_view_model.dart
- Modify: trading_app/lib/core/network/api_client.dart
- Modify: trading_app/lib/features/auth/data/session_controller.dart
- Modify: trading_app/test/features/auth/auth_repository_test.dart
- Modify: trading_app/test/features/auth/auth_view_model_test.dart
- Modify: trading_app/test/features/auth/auth_gate_test.dart
- Modify: trading_app/test/features/auth/single_device_flow_test.dart

### Step 1: 先写 Flutter 数据层失败测试

在 auth_repository_test.dart 使用以下注册结果 JSON：

~~~~dart
const registrationJson = <String, dynamic>{
  'user': <String, dynamic>{
    'id': 'user-1',
    'phone': '13800000000',
    'nickname': 'Alice',
    'role': 'user',
  },
  'status': 'disabled',
  'message': '注册成功，请等待管理员启用后再登录',
};
~~~~

把原来期待 AuthSession 的注册测试改为 RegistrationResult，并断言注册后 auth:token、auth:user、auth:expiresAt 都为空；保留 device:id 不被清除。增加 Dio interceptor 测试：受保护请求返回 403 且 code=ACCOUNT_DISABLED 后，SessionController 清除当前 Token；skipAuth 请求不触发清理。

在 auth_view_model_test.dart 把注册即登录测试改为：register 返回 disabled 结果、user 为空、accessToken 为空、isLoggedIn=false、状态为 ready。增加登录返回 ACCOUNT_DISABLED 的测试。

single_device_flow_test.dart 的内存 API 改为：注册只返回 RegistrationResult；测试显式模拟管理员把用户设为 active；之后登录才返回 AuthSession，原有单设备替换断言继续保留。

### Step 2: 运行失败测试，确认类型和状态路径被锁定

执行：

~~~~sh
cd trading_app
flutter test test/features/auth/auth_repository_test.dart test/features/auth/auth_view_model_test.dart test/features/auth/auth_gate_test.dart test/features/auth/single_device_flow_test.dart
~~~~

预期当前代码因 AuthRepository.register 返回 Future<AuthSession>、注册响应按 session 解析、ViewModel 直接设置 user/token 而失败。

### Step 3: 实现模型、仓库和 ViewModel

在 auth_models.dart 新增 RegistrationResult，字段为 AppUser user、String status、String message，并实现 fromJson 解析 user/status/message。

将 AuthRepository.register 改为 Future<RegistrationResult>。ApiAuthRepository.register 继续通过 skipAuth 发送公开请求，解析 RegistrationResult，不再依赖 device:id、不再调用 getOrCreateDeviceId、不再调用 _persistSession。login 的 deviceId 获取、AuthSession 解析和持久化不变。

将 AuthViewModel.register 从 _authenticate 中拆出，返回 Future<RegistrationResult?>：设置 loading，调用 repository.register，成功后只恢复 ready 并返回结果，不写 user/accessToken；失败时设置 error 和后端 message，返回 null。_authenticate 只保留给 login。

保持 AuthGate 的 isLoggedIn 双条件：user 非空且 accessToken 非空。restoreSession 失败必须清理本地认证数据；/api/auth/me 对 disabled 用户返回 403 后，恢复流程进入 ready 的未登录分支，不创建业务依赖。

### Step 4: 实现认证失效清理

在 api_client.dart 的受保护请求 onError 中加入精确条件：

~~~~dart
final statusCode = error.response?.statusCode;
final code = error.response?.data is Map
    ? error.response?.data['code']
    : null;
final invalidatesDisabledAccount =
    statusCode == 403 && code == 'ACCOUNT_DISABLED';
if (!isAuthRequest &&
    (statusCode == 401 || invalidatesDisabledAccount)) {
  // use the existing clearIfCurrent path and request snapshot
}
~~~~

保留 clearIfCurrent 的并发保护和 SESSION_REPLACED、SESSION_EXPIRED 提示；clear 后不能残留 token、user 或 expiresAt。

### Step 5: 运行 Flutter 认证测试和静态检查

执行：

~~~~sh
cd trading_app
dart format lib/features/auth/domain/auth_models.dart lib/features/auth/data/auth_repository.dart lib/features/auth/presentation/auth_view_model.dart lib/core/network/api_client.dart lib/features/auth/data/session_controller.dart test/features/auth/auth_repository_test.dart test/features/auth/auth_view_model_test.dart test/features/auth/auth_gate_test.dart test/features/auth/single_device_flow_test.dart
flutter test test/features/auth/auth_repository_test.dart test/features/auth/auth_view_model_test.dart test/features/auth/auth_gate_test.dart test/features/auth/single_device_flow_test.dart
flutter analyze
~~~~

预期注册结果只存在于调用返回值和提示中，不会出现在认证存储；所有 disabled/无 Token 分支的 isLoggedIn 均为 false。

### Step 6: 提交任务 3

~~~~sh
git add trading_app/lib/features/auth/domain/auth_models.dart trading_app/lib/features/auth/data/auth_repository.dart trading_app/lib/features/auth/presentation/auth_view_model.dart trading_app/lib/core/network/api_client.dart trading_app/lib/features/auth/data/session_controller.dart trading_app/test/features/auth/auth_repository_test.dart trading_app/test/features/auth/auth_view_model_test.dart trading_app/test/features/auth/auth_gate_test.dart trading_app/test/features/auth/single_device_flow_test.dart
git commit -m "fix(auth): separate registration from login state"
~~~~

## 6. Task 4：修正注册页、AuthGate 和业务依赖生命周期

**Files:**

- Modify: trading_app/lib/features/auth/presentation/register_page.dart
- Modify: trading_app/lib/features/auth/presentation/auth_gate.dart
- Modify: trading_app/lib/app/app_config.dart
- Modify: trading_app/test/features/auth/registration_provider_scope_test.dart
- Modify: trading_app/test/features/auth/auth_gate_test.dart
- Modify: trading_app/test/features/auth/auth_view_model_test.dart

### Step 1: 先写页面和 Provider scope 失败测试

在 registration_provider_scope_test.dart 将成功注册后挂载 AppShell 的断言改为：

1. 提交成功后显示“注册成功，请等待管理员启用后再登录”。
2. 注册 ViewModel 仍是未登录状态。
3. AuthGate 不创建 AuthenticatedDependencies，不初始化业务 Provider，不发出业务远程加载。
4. 关闭提示后回到登录页。
5. 只有 fake 管理员启用并完成 login 后，authenticatedBuilder 才被调用。

在 auth_gate_test.dart 增加无 Token、restore 返回 null、restore 被 401/403 拒绝和清理后回到登录页的测试；用 test-only authenticatedBuilder 计数，证明 AppShell 分支没有被构造。

### Step 2: 运行页面测试，确认旧注册成功路径失败

执行：

~~~~sh
cd trading_app
flutter test test/features/auth/registration_provider_scope_test.dart test/features/auth/auth_gate_test.dart test/features/auth/auth_view_model_test.dart
~~~~

预期当前注册成功测试仍寻找自动登录或 AppShell，新的未登录闭环断言会失败。

### Step 3: 实现注册页等待启用交互

register_page.dart 提交按钮接收 register 返回值：

~~~~dart
final result = await auth.register(
  phone: _phoneController.text,
  password: _passwordController.text,
  nickname: _nicknameController.text,
);
if (!context.mounted || result == null) {
  return;
}
await _showMessage(result.message);
if (context.mounted && Navigator.canPop(context)) {
  Navigator.of(context).pop();
}
~~~~

将提示方法改成可等待的 Future<void>，避免弹窗尚未关闭就执行路由返回。成功只回到登录页；失败保留表单和错误消息，不自动返回，不调用 login，不构造业务页面。

### Step 4: 固化 AuthGate 和 AppConfig 门禁

AuthGate 先等待 restore 完成，再按 isLoggedIn 分支；未登录分支只构造 LoginPage 或 unauthenticatedBuilder，不能调用 AuthenticatedDependencies。业务 Provider 创建和远程加载只能位于已认证分支。

核对 app_config.dart 的 Provider 顺序：AuthStorage、SessionController、Dio、AuthRepository、AuthViewModel 可以在认证层创建；行情、资讯、雷达、策略等业务依赖只能由 AuthenticatedDependencies 创建。认证被清理时依赖树必须卸载。

### Step 5: 运行页面、认证和全量 Flutter 测试

执行：

~~~~sh
cd trading_app
dart format lib/features/auth/presentation/register_page.dart lib/features/auth/presentation/auth_gate.dart lib/app/app_config.dart test/features/auth/registration_provider_scope_test.dart test/features/auth/auth_gate_test.dart test/features/auth/auth_view_model_test.dart
flutter test test/features/auth
flutter test
flutter analyze
~~~~

预期注册成功停留在普通未登录流程，未启用用户不能看到 AppShell；启用后再登录才创建认证依赖和业务页面；已登录后被禁用会回到登录页。

### Step 6: 提交任务 4

~~~~sh
git add trading_app/lib/features/auth/presentation/register_page.dart trading_app/lib/features/auth/presentation/auth_gate.dart trading_app/lib/app/app_config.dart trading_app/test/features/auth/registration_provider_scope_test.dart trading_app/test/features/auth/auth_gate_test.dart trading_app/test/features/auth/auth_view_model_test.dart
git commit -m "fix(auth): keep pending users outside the app"
~~~~

## 7. Task 5：全链路验收与差异审查

### Step 1: 运行后端和 Flutter 全量验证

~~~~sh
cd backend
go test ./... -count=1

cd ../trading_app
flutter test
flutter analyze
~~~~

### Step 2: 构建现有后台页面

~~~~sh
cd frontend
npm run build
~~~~

验收后台页面：新注册用户在列表显示禁用/未启用，点击启用后显示启用；启用动作不返回普通用户 Token，用户必须回到 Flutter 登录页重新登录；现有禁用操作仍撤销会话。

### Step 3: 执行手工 API 闭环

使用测试数据库和现有管理员凭据验证：

1. POST /api/auth/register：201，status=disabled，含 user 和 message，不含 accessToken、expiresAt；该用户无 active session。
2. POST /api/auth/login：正确密码在启用前返回 403 ACCOUNT_DISABLED；错误密码返回 401 INVALID_CREDENTIALS。
3. GET /api/admin/users：能看到新用户为 disabled。
4. PATCH /api/admin/users/{id}/status，提交 active。
5. POST /api/auth/login：启用后返回 AuthSessionResponse；GET /api/auth/me 和业务接口成功。
6. 再次 PATCH status=disabled：会话被撤销，业务接口被拒绝；Flutter 清理后只能显示登录入口。
7. 无 Token 请求业务 REST/WebSocket：不能获得业务数据。

### Step 4: 审查差异、格式和提交边界

执行：

~~~~sh
git diff --check
git status --short
git diff --stat HEAD -- backend/flutter_api trading_app frontend/src
rg -n 'Register\\(.*AuthSession|register.*AuthSession|AccessToken.*register|accessToken.*register|创建.*session|注册即登录' backend/flutter_api trading_app frontend/src
~~~~

审查结果必须满足：

- 生产代码不存在注册成功后生成或持久化 Token 的路径；
- 测试中的注册 helper 不绕过管理员启用；
- 无关 DB、缓存、构建产物和历史文档不在本需求提交中；
- git diff --check 无空白错误；
- 提交信息与项目既有 fix(auth)、test(auth) 风格一致。

## 8. 最终验收标准

- 新用户注册后 users.status=disabled，user_sessions 没有有效记录，注册响应没有 accessToken 或 expiresAt。
- 未启用用户直接登录返回 403 ACCOUNT_DISABLED；错误密码不会泄露 disabled 状态。
- 管理员可以在现有后台用户管理页看到并启用新用户；启用不自动登录。
- 启用后的用户必须再次登录，成功后才能访问 /api/auth/me、行情、资讯、雷达、策略和 WebSocket。
- 无用户、无 Token、Token 无效、恢复失败和 disabled 用户在 Flutter 中均只能停留在未登录入口，不创建业务依赖。
- 已登录用户被管理员禁用后，后端撤销会话或返回 403 ACCOUNT_DISABLED；Flutter 清理本地认证并卸载业务依赖，回到登录页。
- 既有 active 用户、单设备会话替换、密码哈希、用户数据隔离、管理员认证和现有业务功能回归测试全部通过。
