# Backend Task 7 Report

Date: 2026-09-04
Worktree: `/Users/vb/Projects/go-stock-worktrees/module-permission-management`
Branch: `codex/module-permission-management`

## Scope

Task 7 was limited to documenting the completed standalone permission backend and recording the backend verification state. No product code was changed. The existing dirty T0 cache, `data/stock.db`, and frontend lock/workspace files were left untouched and unstaged.

## Documentation

`PROJECT.md` now documents:

- the Flutter user REST/WebSocket listener on `:8080`, with no `/api/admin/` routes;
- the same-process standalone management listener, controlled by `GO_STOCK_ADMIN_ADDR` and defaulting to `:18080`;
- `GO_STOCK_ADMIN_WEB_DIR` precedence and the default `admin-web/dist` lookup;
- comma-separated development origins through `GO_STOCK_ADMIN_DEV_ORIGINS`, including the Vite example `http://localhost:5174`;
- `admin-init` hidden password confirmation, bcrypt-backed `users` administrator creation, duplicate-account protection, and the absence of hardcoded credentials;
- the required admin web startup/smoke flow and the three public modules before any grant;
- HTTPS/reverse-proxy, firewall, Host/Origin, WebSocket upgrade, and HTTPS-aware Cookie requirements before public deployment.

Documented smoke flow:

```text
go run ./cmd/admin-init -account 13900000000 -nickname 管理员
GO_STOCK_ADMIN_ADDR=:18080 go run ./cmd/server
浏览器打开 http://服务器地址:18080/
```

## Verification results

Per the user's instruction, the potentially long full package check was not rerun in this turn. The following known results are recorded for this worktree:

- Focused backend regression: PASS.

  ```text
  /Users/vb/.cache/codex-runtimes/go1.26.0/bin/go test ./backend/flutter_api -run 'Test(UserHTTP|AdminWeb|Server|Auth|Admin)' -count=1
  ok go-stock/backend/flutter_api 4.588s
  ```

- Backend builds: PASS, exit code 0 with no output.

  ```text
  /Users/vb/.cache/codex-runtimes/go1.26.0/bin/go build ./cmd/server ./cmd/admin-init
  ```

- `git diff --check`: PASS, exit code 0 with no output in this turn.

- Full backend package:

  ```text
  go test ./backend/flutter_api -count=1
  ```

  This remains a known pre-existing failure risk outside Task 7: the T0 prewarm path can reach `backend/data.GetSettingConfig` with a nil `*gorm.DB` and panic in `gorm.(*DB).Model`. No T0, data, or cache behavior was changed to address it.

## Staging boundary

Only `PROJECT.md`, this Task 7 report, and the SDD `progress.md` ledger entry are intended for the Task 7 commit. Existing dirty data/cache and frontend lock files must remain unstaged.

## Review follow-up

The documentation review identified three P2 issues. The ledger's live checklist and stale active-agent text were synchronized, the bcrypt wording now states that only the hash is written to `users.password_hash` while plaintext/hash output is not exposed, and the smoke flow now states how to register and enable an ordinary user on a clean database.
