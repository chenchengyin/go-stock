# 用户管理与单设备登录设计规格

**日期：** 2026-08-30
**状态：** 待规格评审
**适用范围：** 现有 `trading_app` 登录/注册模块、其使用的 `backend/flutter_api` `:8080` REST API 与 WebSocket

> **当前版本说明：** 用户表、会话表、真实 API 认证、密码哈希、单设备会话和用户数据隔离基础能力已存在。本规格当前用于统一认证行为约束；本次新增的注册启用流程以 `docs/superpowers/plans/2026-09-03-user-registration-activation.md` 为实施依据，不重复建设基础认证设施。

## 1. 背景与目标

当前 Flutter 端已经通过 `ApiAuthRepository` 调用真实认证 API，使用持久化 Token 恢复登录态；Go 服务已通过 `AuthService`、bcrypt 和 `user_sessions` 提供账号与会话认证。本次只在现有基础上调整注册后的启用门禁，并保持启用账号的登录、单设备会话和用户数据隔离逻辑。

本次调整的目标是：

1. 用户可以使用手机号/账号、密码和昵称完成注册，注册后默认不可用。
2. 用户必须经管理员启用后，才能使用手机号/账号和密码登录；登录态可在应用重启后恢复。
3. 同一账号同时只保留一台设备的有效会话。
4. 新设备登录后，旧设备的会话立即失效；旧设备后续数据请求返回认证失败，客户端自动回到登录页。
5. 自选股、分组、交易记录和策略社区数据按账号隔离。
6. 行情、资讯、股票基础资料、选股缓存等公共数据继续共享。
7. 旧的无账号归属数据保留，但不自动分配给任何新用户。

本期不增加短信验证码、邮箱验证、找回密码和第三方登录；注册字段继续沿用现有页面的手机号/账号、密码、昵称。

## 2. 已确认的产品约束

### 2.1 账号

- `phone` 继续作为唯一登录名，服务端对输入执行去首尾空格处理。
- 密码至少 6 位，服务端必须再次校验，不能只依赖客户端校验。
- 昵称注册时允许为空；为空时使用账号作为默认昵称，保持现有页面行为。
- 注册成功后账号默认为 `disabled`，只返回注册结果，不创建会话、不返回 Token；必须由后台管理员改为 `active` 后，用户才能登录。

### 2.2 设备

- 客户端首次启动生成随机 `deviceId` 并持久化。
- 手机或桌面端按一次应用安装计算一台设备。
- Web 端按一个浏览器站点存储空间计算一台设备。
- 卸载、清除应用数据或清除 Web 站点数据后，新的 `deviceId` 视为新设备。
- `deviceId` 用于会话归属和提示，不作为单独的安全凭证；真正的认证依据是随机会话 Token。

### 2.3 鉴权边界

- 普通用户公开接口：注册、登录、健康检查；管理员登录 `/api/admin/login` 属于独立的后台认证通道。
- 受保护接口：所有其他 `/api` 数据接口、上传接口和 `/ws` WebSocket。
- 业务 handler 不再信任请求中的 `userId`、`nickname` 或其他身份字段。
- 服务端从认证上下文取得真实用户 ID，并以此执行所有用户数据读写。

## 3. 总体架构

采用“随机不透明会话令牌 + SQLite 会话表 + 统一认证中间件”的方案。

```text
Flutter Auth UI
    -> ApiAuthRepository
    -> Dio 认证拦截器
    -> /api/auth/login|register|me|logout|profile
    -> AuthService
    -> users + user_sessions

Flutter 业务请求
    -> Authorization: Bearer <token>
    -> AuthMiddleware
    -> AuthContext(userID)
    -> 用户范围查询 / 公共数据查询
```

服务端只保存 Token 的哈希值，不保存明文 Token。每次登录生成新的会话记录，并在同一事务中撤销该账号原有的有效会话。单个账号的有效会话数量由数据库约束和事务保证为最多一条。

当前服务是单个 Go API 进程使用现有 SQLite 数据库的部署形态。若未来扩展为多个 API 实例，需要把会话存储迁移到共享数据库或 Redis；本期不引入新的基础设施。

## 4. 数据模型

### 4.1 `users`

现有 `backend/flutter_api` 认证模型通过 API 层迁移并已投入使用；本次只改变新注册用户的初始 `status`，不新增字段：

| 字段 | 说明 |
| --- | --- |
| `id` | 随机字符串用户 ID，作为对外用户标识 |
| `phone` | 唯一登录名 |
| `password_hash` | 密码哈希，不保存明文 |
| `nickname` | 用户昵称 |
| `role` | 默认 `user` |
| `status` | 新注册用户默认 `disabled`；只有 `active` 用户可以登录和访问业务接口。`disabled` 同时覆盖“尚未启用”和“被管理员禁用” |
| `created_at` / `updated_at` | 审计时间 |

`phone` 建立唯一索引。对外返回的用户对象只包含 ID、手机号、昵称和角色，不返回密码哈希或会话内部字段。

### 4.2 `user_sessions`

每次成功登录创建一条会话记录，注册不会创建会话；旧会话通过 `revoked_at` 失效：

| 字段 | 说明 |
| --- | --- |
| `id` | 会话 ID |
| `user_id` | 所属用户 |
| `token_hash` | Token 哈希，唯一索引 |
| `device_id` | 客户端安装/浏览器设备标识 |
| `created_at` | 创建时间 |
| `last_seen_at` | 最近成功认证时间 |
| `expires_at` | 过期时间，默认登录后 30 天 |
| `revoked_at` | 会话失效时间，可为空 |
| `revoke_reason` | `logout`、`new_login`、`expired` 等原因 |

有效会话定义为 `revoked_at IS NULL AND expires_at > now`。只有登录成功才创建会话；登录事务先撤销该用户所有有效会话，再插入新会话；为保证同一用户不产生两个有效会话，需要建立有效会话的唯一索引或等价的事务约束。

### 4.3 用户数据归属

以下用户产生的数据增加 `user_id`，并在查询、修改、删除时强制使用当前用户范围：

- `followed_stock`
- `stock_groups`
- `group_stock_info`
- `trading_records`
- 策略社区的 `StrategyUser`、`StrategyPost`、`StrategyComment`、`StrategyLike`、签到和积分流水

必要的组合唯一索引包括“用户 + 股票代码”和“用户 + 分组股票关系”，防止不同用户之间互相覆盖。

策略社区已有 `userId` 字段，但其来源必须改为认证上下文；请求体中的同名字段不再作为身份依据。

股票主数据、行情、资讯、选股缓存和其他公共研究数据不增加账号过滤，只要求请求已认证。

## 5. 认证 API

### 5.1 注册

`POST /api/auth/register`

请求：

```json
{
  "phone": "13800000000",
  "password": "secret123",
  "nickname": "昵称",
  "deviceId": "client-installation-id"
}
```

成功：创建 `status=disabled` 的用户，不创建会话，返回注册结果：

```json
{
  "user": {
    "id": "<id>",
    "phone": "13800000000",
    "nickname": "昵称",
    "role": "user"
  },
  "status": "disabled",
  "message": "注册成功，请等待管理员启用后再登录"
}
```

注册响应不包含 `accessToken`、`expiresAt` 或设备信息。请求中的 `deviceId` 为旧客户端兼容字段，可继续提交但不再参与注册；登录仍必须提交有效 `deviceId`。

错误：

- 参数不合法：`400`
- 账号已存在：`409 ACCOUNT_EXISTS`

### 5.2 登录

`POST /api/auth/login`

请求：

```json
{
  "phone": "13800000000",
  "password": "secret123",
  "deviceId": "client-installation-id"
}
```

成功：撤销该账号旧会话，创建当前设备会话并返回登录响应。

错误：

- 账号不存在或密码错误：`401 INVALID_CREDENTIALS`
- 凭证正确但账号未启用或已禁用：`403 ACCOUNT_DISABLED`，提示用户等待管理员启用
- `deviceId` 缺失或不合法：`400 DEVICE_REQUIRED`

服务端先校验账号密码，再根据账号状态决定是否返回 `ACCOUNT_DISABLED`；不能因为错误密码直接暴露账号的启用状态。

### 5.3 当前用户

`GET /api/auth/me`

需要有效 Bearer Token。返回当前用户对象和会话过期时间，用于应用启动时恢复登录态。

### 5.4 退出

`POST /api/auth/logout`

需要有效 Bearer Token。撤销当前会话；重复退出可按幂等成功处理。

### 5.5 修改资料

`PATCH /api/auth/profile`

需要有效 Bearer Token。当前只支持修改昵称，服务端从认证上下文定位用户。

### 5.6 统一错误结构

```json
{
  "code": "SESSION_REPLACED",
  "message": "账号已在其他设备登录，请重新登录"
}
```

认证相关错误码：

| HTTP | 错误码 | 含义 |
| --- | --- | --- |
| `400` | `INVALID_ARGUMENT` | 参数缺失或格式错误 |
| `400` | `DEVICE_REQUIRED` | 缺少设备标识 |
| `401` | `UNAUTHENTICATED` | 未携带 Token |
| `401` | `INVALID_CREDENTIALS` | 账号或密码错误 |
| `401` | `SESSION_EXPIRED` | 会话过期 |
| `401` | `SESSION_REPLACED` | 会话被新设备登录替换 |
| `403` | `ACCOUNT_DISABLED` | 账号未启用或已禁用 |
| `409` | `ACCOUNT_EXISTS` | 账号已存在 |

## 6. 服务端请求链路

### 6.1 REST

`backend/flutter_api.Start` 注册公开认证路由，并将业务路由统一包装在认证中间件中。中间件执行：

1. 读取 `Authorization: Bearer <token>`。
2. 对 Token 做哈希并查询会话。
3. 检查会话是否存在、未撤销、未过期，以及账号处于 `active` 状态；其他状态不得进入业务上下文。
4. 更新 `last_seen_at`。
5. 将用户 ID 写入请求上下文。
6. 调用具体 handler。

业务 handler 只从上下文读取用户 ID。请求参数中的 `userId` 和 `nickname` 即使存在也必须忽略或拒绝，避免身份冒用。

### 6.2 WebSocket

`/ws` 在握手阶段校验会话 Token。支持：

- 原生客户端通过 `Authorization` 请求头携带 Token。
- Web 客户端通过兼容浏览器的握手参数携带 Token；服务端不得把该参数写入普通请求日志。

无效会话或非 `active` 用户拒绝升级；已连接会话被替换或管理员禁用时，服务端关闭连接，客户端按会话失效处理。

### 6.3 并发登录

同一账号的登录操作在数据库事务中完成：

1. 校验密码。
2. 撤销当前账号已有有效会话，原因设为 `new_login`。
3. 插入新设备会话。
4. 提交事务后才向客户端返回新 Token。

SQLite 写事务串行化，结合有效会话唯一约束，保证并发登录最终只有最后提交的会话有效。已经在新登录提交前开始执行的旧请求允许完成；新登录提交后到达的旧 Token 请求必须失败。

## 7. Flutter 客户端接入

### 7.1 会话存储

将现有 `MemoryLocalCache` 替换为基于本地持久化的实现，至少保存：

- `auth:token`
- `auth:phone`
- `auth:nickname`
- `auth:role`
- `auth:expiresAt`
- `device:id`

Token 仅作为访问凭证保存；密码不在本地保存。认证存储通过抽象接口提供，测试可以继续使用内存实现。

### 7.2 API 客户端

`trading_app/lib/core/network/api_client.dart` 增加：

- 请求拦截器：从会话存储读取 Token 并添加 Bearer Header。
- 响应错误拦截器：识别 `401` 和 `403 ACCOUNT_DISABLED`。
- 单次失效处理锁：并发受保护请求同时收到 `401` 或 `403 ACCOUNT_DISABLED` 时只执行一次清理和导航。
- 认证接口请求不附加旧 Token，避免登录/注册被旧会话干扰。

### 7.3 AuthViewModel 与启动门禁

- 继续使用现有真实 `ApiAuthRepository`，仅将注册响应从 `AuthSession` 改为注册结果。
- `restore()` 先恢复本地凭证，再调用 `/api/auth/me`。
- Token 无效、过期、被替换或用户被禁用时清理本地会话。
- 新增会话失效状态，区分普通过期、被其他设备登录替换和账号不可用。
- 增加 `AuthGate` 或等价启动门禁：认证状态确定前不加载业务 ViewModel 的远程数据；无用户、无有效 Token、Token 恢复失败或用户非 `active` 时只能显示登录/注册入口，不能显示 `AppShell`。
- 注册成功后显示“等待管理员启用”提示并回到登录页；只有管理员启用后的下一次登录才进入 `AppShell`。
- 受保护请求收到 `401` 或 `403 ACCOUNT_DISABLED` 时清理本地会话并卸载业务依赖，确保用户只能回到未登录态。

### 7.4 业务调用改造

- `StrategyRepository` 不再接收或发送 `userId`、`nickname` 参数。
- 其他用户数据请求不从页面层拼接用户 ID。
- 所有业务仓库继续复用同一个 Dio 实例，确保认证拦截器覆盖完整。
- 业务接口收到 `SESSION_REPLACED` 后停止刷新、清理当前用户和本地会话，并回到登录页。

## 8. 数据迁移与兼容

现有迁移已经完成用户、会话和用户数据归属基础建设；本次不新增表、不修改用户数据表结构，继续遵循“保留旧数据、不自动分配”的策略：

1. 新注册用户明确写入 `status=disabled`，不创建 `user_sessions`。
2. 已有 `active` 用户状态不做批量重置；已有 `disabled` 用户仍需管理员启用。
3. 现有无归属行保留为 `user_id = NULL`，不返回给任何新用户。
4. 不得把旧数据绑定到第一个注册用户，也不得静默删除旧行。
5. 新注册用户创建后，只能读写自己的新数据。
6. 策略社区已有的匿名或模拟用户数据不自动映射到新账号。

## 9. 错误处理与安全要求

- 密码使用成熟的密码哈希算法保存，禁止明文和可逆加密。
- Token 使用密码学安全随机源生成，数据库只保存哈希。
- 日志不得输出密码、完整 Token 或包含 Token 的 WebSocket URL。
- 账号不存在或密码错误统一返回 `INVALID_CREDENTIALS`，不泄露账号是否存在；服务端先完成密码校验，凭证正确但账号非 `active` 时按本期产品要求返回 `403 ACCOUNT_DISABLED` 和等待启用提示，客户端不得将其视为已登录。
- 用户数据查询必须带当前用户范围，不能仅依赖前端隐藏入口。
- 访问其他用户资源时按不存在处理，避免泄露资源归属。
- 业务接口统一使用 JSON 错误结构，避免部分接口返回 200 + `error`、部分接口返回 HTTP 错误导致客户端无法统一处理。
- 生产环境使用 HTTPS/WSS；本期不改变现有 CORS 业务，但认证请求必须允许 `Authorization` Header。

## 10. 测试与验收

### 10.1 Go 测试

认证服务和中间件测试覆盖：

- 注册成功、重复账号、非法参数。
- 注册成功后状态为 `disabled`、无会话且响应不含 Token；未启用前登录失败，管理员启用后登录成功。
- 登录成功、错误密码、禁用账号。
- `/me` 恢复有效会话。
- 登出后 Token 立即失效。
- 会话过期后返回 `SESSION_EXPIRED`。
- 设备 A 登录后设备 B 登录，设备 A 后续请求返回 `SESSION_REPLACED`，设备 B 请求成功。
- 不同账号之间会话互不影响。
- 未认证访问每个业务路由类别均返回 `401`。
- 并发登录最终只有一个有效会话。
- 用户数据查询、写入和删除均按 `user_id` 隔离。
- `user_id = NULL` 的历史数据不会返回给新用户。

### 10.2 Flutter 测试

- Token、用户信息和 `deviceId` 能持久化和恢复。
- Dio 业务请求自动附加 Bearer Token。
- 受保护请求的 `401` 或 `403 ACCOUNT_DISABLED` 并发响应只触发一次清理和登录跳转。
- `/me` 验证完成前不发起业务数据加载。
- `SESSION_REPLACED` 显示明确提示并清除登录态。
- 登录和注册服务端错误能展示给用户；`ACCOUNT_DISABLED` 会清理业务会话并回到登录页。
- 应用重启后仅有效且对应 `active` 用户的会话能进入 `AppShell`；无用户、无效会话或非 `active` 用户进入登录页。

### 10.3 端到端验收

1. 设备 A 注册后保持未登录，管理员启用账号后，设备 A 登录并能正常请求行情、资讯、雷达和策略接口。
2. 设备 B 登录同一账号后，设备 A 的下一次数据请求失败并自动退出。
3. 设备 B 继续正常请求数据。
4. 账号 A 看不到账号 B 的自选股、分组和交易记录。
5. 账号 A 退出后，旧 Token 不能继续请求数据。
6. 未登录用户、未启用用户和无效会话不能访问业务 REST 接口和 WebSocket；已登录后收到 `403 ACCOUNT_DISABLED` 时客户端自动回到登录页。
7. 旧的无归属数据仍保留在数据库中，但不会出现在新用户结果中。

## 11. 预期改动边界

本次后端重点涉及：

- `backend/flutter_api` 注册响应、注册服务、登录状态校验和相关测试。
- 管理员启用/禁用回归测试，以及会话撤销后的认证行为。

本次 Flutter 重点涉及：

- `trading_app/lib/features/auth` 的注册模型、仓库、ViewModel 和注册页。
- `trading_app/lib/core/network/api_client.dart` 的 `401/403` 会话清理。
- 应用启动门禁和已认证业务 Provider 的生命周期测试。

不增加短信、邮箱、找回密码、第三方登录、Redis 或独立账号后台；不改变公共行情/资讯数据的计算逻辑。
