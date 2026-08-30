# Task 1 实现报告

## 结果

Task 1 已完成：为 `backend/flutter_api` 建立了 Go 认证模型、独立迁移入口，以及对应的 SQLite/GORM 迁移测试。

## RED / GREEN 证据

### RED

先加入了失败测试 `[backend/flutter_api/auth_models_test.go](/Users/vb/Projects/go-stock/backend/flutter_api/auth_models_test.go)`，并运行：

```bash
export PATH=/Users/vb/.cache/codex-runtimes/go1.26.0/bin:$PATH
export GOTOOLCHAIN=local
export GOPATH=/Users/vb/go
go test ./backend/flutter_api -run TestMigrateAuthTablesCreatesUsersAndSessions -count=1
```

首次有效失败结果是：

```text
backend/flutter_api/auth_models_test.go:33:30: undefined: AuthUser
backend/flutter_api/auth_models_test.go:37:12: undefined: MigrateAuthTables
backend/flutter_api/auth_models_test.go:41:31: undefined: AuthUser
backend/flutter_api/auth_models_test.go:41:72: undefined: AuthSession
backend/flutter_api/auth_models_test.go:45:31: undefined: AuthUser
backend/flutter_api/auth_models_test.go:49:31: undefined: AuthSession
FAIL    go-stock/backend/flutter_api [build failed]
```

### GREEN

补齐实现后再次运行同一条测试，结果通过：

```text
ok  	go-stock/backend/flutter_api	0.386s
```

随后运行完整包测试：

```bash
export PATH=/Users/vb/.cache/codex-runtimes/go1.26.0/bin:$PATH
export GOTOOLCHAIN=local
export GOPATH=/Users/vb/go
go test ./backend/flutter_api -count=1 -timeout 10m
```

最终结果：

```text
ok  	go-stock/backend/flutter_api	381.158s
```

## 变更文件

- `[backend/flutter_api/auth_models.go](/Users/vb/Projects/go-stock/backend/flutter_api/auth_models.go)`
- `[backend/flutter_api/auth_migrations.go](/Users/vb/Projects/go-stock/backend/flutter_api/auth_migrations.go)`
- `[backend/flutter_api/auth_models_test.go](/Users/vb/Projects/go-stock/backend/flutter_api/auth_models_test.go)`

## 自检

- `AuthUser` 和 `AuthSession` 的表名分别固定为 `users`、`user_sessions`，符合 brief 要求。
- `phone` 和 `token_hash` 都带了对应唯一索引，测试覆盖了这两个索引是否被迁移创建。
- `MigrateAuthTables(dao *gorm.DB) error` 使用 `AutoMigrate` 加上 `CREATE UNIQUE INDEX IF NOT EXISTS`，重复执行是幂等的。
- `AuthSessionResponse` 只暴露 `PublicUser`、`AccessToken`、`ExpiresAt`，没有直接暴露数据库模型。

## 关注点

- 仓库里本来就有大量未跟踪的缓存文件和两个已修改的数据库文件，我没有动它们，也没有把它们加入本次提交。
- `backend/flutter_api` 的全包测试耗时较长，且包含不少联网/重型现有测试；本次验证已确认它们在当前改动下仍然通过。

## Round 1 修复记录

### 问题

Review 指出迁移测试覆盖不足，具体缺少：

- `users` / `user_sessions` 必需字段断言
- 手工创建的 `idx_auth_sessions_one_active` 过滤索引断言
- 第二次调用 `MigrateAuthTables` 的幂等性验证

### 修复

只增强了 `[backend/flutter_api/auth_models_test.go](/Users/vb/Projects/go-stock/backend/flutter_api/auth_models_test.go)`，没有修改生产迁移逻辑。

新增了：

- `users` 列断言：`id`, `phone`, `password_hash`, `nickname`, `role`, `status`, `created_at`, `updated_at`
- `user_sessions` 列断言：`id`, `user_id`, `token_hash`, `device_id`, `created_at`, `last_seen_at`, `expires_at`, `revoked_at`, `revoke_reason`
- `idx_auth_sessions_one_active` 的 SQL 过滤条件断言，验证包含 `WHERE revoked_at IS NULL`
- 第二次调用 `MigrateAuthTables(dao)` 的重复迁移断言

### 验证命令与输出

```bash
export PATH=/Users/vb/.cache/codex-runtimes/go1.26.0/bin:$PATH
export GOTOOLCHAIN=local
export GOPATH=/Users/vb/go
go test ./backend/flutter_api -run TestMigrateAuthTablesCreatesUsersAndSessions -count=1
```

输出：

```text
ok  	go-stock/backend/flutter_api	0.392s
```

```bash
export PATH=/Users/vb/.cache/codex-runtimes/go1.26.0/bin:$PATH
export GOTOOLCHAIN=local
export GOPATH=/Users/vb/go
go test ./backend/flutter_api -count=1 -timeout 10m
```

输出：

```text
ok  	go-stock/backend/flutter_api	427.221s
```

