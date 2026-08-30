# 用户管理与单设备登录设计规格

**日期：** 2026-08-30
**状态：** 待规格评审
**适用范围：** 现有 `trading_app` 登录/注册模块、其使用的 `backend/flutter_api` `:8080` REST API 与 WebSocket

## 1. 背景与目标

当前 Flutter 端已经有登录、注册、编辑昵称页面，但使用的是 `MockAuthRepository`：账号没有服务端持久化，密码没有真正校验，Token 只是内存中的模拟值，业务接口也不验证用户身份。

本次改造把现有登录/注册模块升级为真实用户管理模块，目标是：

1. 用户可以使用手机号/账号、密码和昵称完成注册。
2. 用户可以使用手机号/账号和密码登录，登录态可在应用重启后恢复。
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
- 注册成功后直接创建会话并返回登录态，保持现有注册页注册成功后返回已登录状态的行为。

### 2.2 设备

- 客户端首次启动生成随机 `deviceId` 并持久化。
- 手机或桌面端按一次应用安装计算一台设备。
- Web 端按一个浏览器站点存储空间计算一台设备。
- 卸载、清除应用数据或清除 Web 站点数据后，新的 `deviceId` 视为新设备。
- `deviceId` 用于会话归属和提示，不作为单独的安全凭证；真正的认证依据是随机会话 Token。

### 2.3 鉴权边界

- 公开接口：注册、登录、健康检查。
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

建议新增到 `backend/flutter_api` 的认证模型中，并通过 API 层自动迁移：

| 字段 | 说明 |
| --- | --- |
| `id` | 随机字符串用户 ID，作为对外用户标识 |
| `phone` | 唯一登录名 |
| `password_hash` | 密码哈希，不保存明文 |
| `nickname` | 用户昵称 |
| `role` | 默认 `user` |
| `status` | 默认启用；禁用账号不能登录和访问业务接口 |
| `created_at` / `updated_at` | 审计时间 |

`phone` 建立唯一索引。对外返回的用户对象只包含 ID、手机号、昵称和角色，不返回密码哈希或会话内部字段。

### 4.2 `user_sessions`

每次登录创建一条会话记录，旧会话通过 `revoked_at` 失效：

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

有效会话定义为 `revoked_at IS NULL AND expires_at > now`。登录事务先撤销该用户所有有效会话，再插入新会话；为保证同一用户不产生两个有效会话，需要建立有效会话的唯一索引或等价的事务约束。

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

成功：创建用户和会话，返回用户对象、Token 和过期时间。

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

- 账号或密码错误：`401 INVALID_CREDENTIALS`
- 账号已禁用：`403 ACCOUNT_DISABLED`
- `deviceId` 缺失或不合法：`400 DEVICE_REQUIRED`

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
| `403` | `ACCOUNT_DISABLED` | 账号已禁用 |
| `409` | `ACCOUNT_EXISTS` | 账号已存在 |

## 6. 服务端请求链路

### 6.1 REST

`backend/flutter_api.Start` 注册公开认证路由，并将业务路由统一包装在认证中间件中。中间件执行：

1. 读取 `Authorization: Bearer <token>`。
2. 对 Token 做哈希并查询会话。
3. 检查会话是否存在、未撤销、未过期，以及账号处于启用状态。
4. 更新 `last_seen_at`。
5. 将用户 ID 写入请求上下文。
6. 调用具体 handler。

业务 handler 只从上下文读取用户 ID。请求参数中的 `userId` 和 `nickname` 即使存在也必须忽略或拒绝，避免身份冒用。

### 6.2 WebSocket

`/ws` 在握手阶段校验会话 Token。支持：

- 原生客户端通过 `Authorization` 请求头携带 Token。
- Web 客户端通过兼容浏览器的握手参数携带 Token；服务端不得把该参数写入普通请求日志。

无效会话拒绝升级；已连接会话被替换时，服务端关闭连接，客户端按会话失效处理。

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
- 响应错误拦截器：识别 `401` 和错误码。
- 单次失效处理锁：并发请求同时收到 `401` 时只执行一次清理和导航。
- 认证接口请求不附加旧 Token，避免登录/注册被旧会话干扰。

### 7.3 AuthViewModel 与启动门禁

- `MockAuthRepository` 替换为真实 `ApiAuthRepository`。
- `restore()` 先恢复本地凭证，再调用 `/api/auth/me`。
- Token 无效、过期或被替换时清理本地会话。
- 新增会话失效状态，区分普通过期和被其他设备登录替换。
- 增加 `AuthGate` 或等价启动门禁：认证状态确定前不加载业务 ViewModel 的远程数据；未登录时显示登录页，已登录时显示 `AppShell`。
- 登录/注册成功后让现有页面行为保持不变。

### 7.4 业务调用改造

- `StrategyRepository` 不再接收或发送 `userId`、`nickname` 参数。
- 其他用户数据请求不从页面层拼接用户 ID。
- 所有业务仓库继续复用同一个 Dio 实例，确保认证拦截器覆盖完整。
- 业务接口收到 `SESSION_REPLACED` 后停止刷新、清理当前用户和本地会话，并回到登录页。

## 8. 数据迁移与兼容

迁移遵循“保留旧数据、不自动分配”的策略：

1. 创建 `users`、`user_sessions` 表。
2. 为用户数据表增加可为空的 `user_id` 字段和必要索引。
3. 现有无归属行保留为 `user_id = NULL`，不返回给任何新用户。
4. 迁移过程不得把旧数据绑定到第一个注册用户，也不得静默删除旧行。
5. 新注册用户创建后，只能读写自己的新数据。
6. 策略社区已有的匿名或模拟用户数据不自动映射到新账号。

如果迁移失败，服务启动应记录清晰错误并停止提供受影响的认证/数据服务，不能以部分迁移状态继续写入。

## 9. 错误处理与安全要求

- 密码使用成熟的密码哈希算法保存，禁止明文和可逆加密。
- Token 使用密码学安全随机源生成，数据库只保存哈希。
- 日志不得输出密码、完整 Token 或包含 Token 的 WebSocket URL。
- 认证失败不泄露账号是否存在；登录失败统一使用账号或密码错误提示。
- 用户数据查询必须带当前用户范围，不能仅依赖前端隐藏入口。
- 访问其他用户资源时按不存在处理，避免泄露资源归属。
- 业务接口统一使用 JSON 错误结构，避免部分接口返回 200 + `error`、部分接口返回 HTTP 错误导致客户端无法统一处理。
- 生产环境使用 HTTPS/WSS；本期不改变现有 CORS 业务，但认证请求必须允许 `Authorization` Header。

## 10. 测试与验收

### 10.1 Go 测试

认证服务和中间件测试覆盖：

- 注册成功、重复账号、非法参数。
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
- `401` 并发响应只触发一次清理和登录跳转。
- `/me` 验证完成前不发起业务数据加载。
- `SESSION_REPLACED` 显示明确提示并清除登录态。
- 登录和注册服务端错误能展示给用户。
- 应用重启后有效会话能进入 `AppShell`，无效会话进入登录页。

### 10.3 端到端验收

1. 设备 A 注册并登录，能正常请求行情、资讯、雷达和策略接口。
2. 设备 B 登录同一账号后，设备 A 的下一次数据请求失败并自动退出。
3. 设备 B 继续正常请求数据。
4. 账号 A 看不到账号 B 的自选股、分组和交易记录。
5. 账号 A 退出后，旧 Token 不能继续请求数据。
6. 未登录用户不能访问业务 REST 接口和 WebSocket。
7. 旧的无归属数据仍保留在数据库中，但不会出现在新用户结果中。

## 11. 预期改动边界

后端重点涉及：

- `backend/flutter_api` 认证模型、服务、中间件、路由和测试。
- `backend/data` / 用户数据模型的 `user_id` 与范围查询改造。
- API 启动时的自动迁移注册。

Flutter 重点涉及：

- `trading_app/lib/features/auth` 的仓库、模型和 ViewModel。
- `trading_app/lib/core/network/api_client.dart`。
- `trading_app/lib/core/storage/local_cache.dart` 或等价持久化实现。
- 应用启动门禁和业务 Provider 初始化顺序。
- 策略仓库及相关用户数据调用。

不增加短信、邮箱、找回密码、第三方登录、Redis 或独立账号后台；不改变公共行情/资讯数据的计算逻辑。
