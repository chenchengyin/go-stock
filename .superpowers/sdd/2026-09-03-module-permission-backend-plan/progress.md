# SDD ledger — plan: docs/superpowers/plans/2026-09-03-module-permission-backend-plan.md

## Setup

- Workspace: `/Users/vb/Projects/go-stock-worktrees/module-permission-management`
- Branch: `codex/module-permission-management`
- BASE: `fec4372c6ed14c0fd7b4f8f11165f3b0ada55599`
- Spec: `docs/superpowers/specs/2026-09-03-module-permission-management-design.md` (reachable and authoritative)

## Preflight conflict scan

| Scope | Producer / consumer | What was checked | Result |
| --- | --- | --- | --- |
| Task 1 ↔ Task 2 | models/registry → ModuleService | Registry codes and GORM models provide every service lookup and grant field named by Task 2. | Agrees; no conflict. |
| Task 2 ↔ Task 4 | ModuleService → module/auth/admin HTTP | Service methods used by handlers are defined before HTTP integration and preserve independent module sets. | Agrees; no conflict. |
| Task 2 ↔ Task 5 | access service → T0 authorization | T0 consumes stable module codes and `MODULE_FORBIDDEN`; no calculation or cache format change is requested. | Agrees; no conflict. |
| Task 3 ↔ Task 4 | database admin session → admin HTTP | Cookie session and CSRF behavior are available to all management routes; login response does not expose a bearer token. | Agrees; no conflict. |
| Task 4 ↔ Task 5 | module API/error contract → T0 handler | Explicit `module_code` is validated before results are read, and each module filters only its own result scope. | Agrees; no conflict. |
| Task 4 ↔ Task 6 | HTTP handlers → separate listener | User handler keeps ordinary APIs while the admin handler is mounted only on the configured admin listener. | Agrees; no conflict. |
| Task 5 ↔ Task 6 | T0 handler → server wiring | The authorized T0 handler can be injected into the user server without reintroducing admin routes on port 8080. | Agrees; no conflict. |
| Task 1 | registry/models/migration tests ↔ created files | Tests cover six definitions, unique grants, and migration models named in the task. | Self-consistent. |
| Task 2 | service tests ↔ ModuleService files | Tests cover public fallback, independent grants, transactional replacement, and reverse lookup. | Self-consistent. |
| Task 3 | admin auth/bootstrap tests ↔ session files/CLI | Tests cover bcrypt users, database sessions, Cookie attributes, CSRF, and explicit admin initialization. | Self-consistent. |
| Task 4 | HTTP tests ↔ user/admin routes | Tests cover module metadata, users, status, access replacement, reverse lookup, and user/admin boundary. | Self-consistent. |
| Task 5 | T0 tests ↔ handler/cache files | Tests cover explicit module code, independent filtering, forbidden access, and archive/prewarm authorization. | Self-consistent. |
| Task 6 | server/static tests ↔ listener files | Tests cover default/configured admin address, no admin routes on user server, and static SPA fallback. | Self-consistent. |
| Task 7 | PROJECT/docs ↔ completed backend behavior | Documentation and final verification refer to the APIs, ports, initialization, and test commands created by Tasks 1–6. | Self-consistent. |
| Backend ↔ admin-web plans | backend APIs/cookie/static listener → admin-web API/client/UI | Admin web depends on backend endpoints, HttpOnly session semantics, and production static hosting exactly as specified. | Intentional dependency; backend first. |
| Backend ↔ Flutter plan | `/api/auth/modules`, `MODULE_FORBIDDEN`, T0 `module_code` → Flutter repository/VM | Flutter consumes only the public API and explicit module-scoped business contract; no source-file overlap. | Intentional dependency; backend first. |
| Admin-web ↔ Flutter plans | independent web client vs Flutter client | No shared implementation files; both use the same backend module metadata contract. | No conflict. |

## Task status

- Backend Tasks 1–7 are complete and reviewed. The P2 documentation findings from the Task 7 review were corrected before the admin-web plan continued.

## Preflight baseline

- `PATH=/Users/vb/.cache/codex-runtimes/go1.26.0/bin:$PATH go test ./...` did not complete cleanly: existing unrelated failures include missing `frontend/dist` before setup, network-dependent data tests, and empty test database assumptions. The command was interrupted after those failures; feature work will use focused backend packages/tests.

- [x] Task 1: Add the module registry and database models
- [x] Task 2: Implement the per-user ModuleService
- [x] Task 3: Replace hardcoded admin authentication and add admin-init
- [x] Task 4: Expose the user and admin module HTTP APIs
- [x] Task 5: Enforce independent T0 strategy access and result scope
- [x] Task 6: Split the user server from the standalone admin server
- [x] Task 7: Update documentation and perform backend verification

Task 1: complete (commits fec4372..f3fceb3, review clean)

Task 2: complete (commits f3fceb3..ae3b4ac, review clean)

Task 3 review: spec ❌; three Important findings remain open: use the required plural development-origin environment variable, compare Origin scheme as well as host/port, and perform a dummy bcrypt comparison for missing/empty accounts. The reviewer also noted admin routes are still on the user handler, but that code is intentionally owned by Task 6.

Task 3 Ruling: defer the separate-listener finding to Task 6 — the plan explicitly assigns `server.go` listener separation and removal of admin routes from 8080 to Task 6, so changing it in the Task 3 fix would cross task boundaries; cost if wrong: admin routes could remain exposed on the user port until Task 6 completes.

Task 3: fix round 1/5 (3 addressed, 0 open; commits 4ba411b..4993aea)

Task 3 re-review: all three findings addressed; no new Critical/Important breakage.

Task 3: complete (commits ae3b4ac..4993aea, review clean)

Task 4 review: spec ⚠️ Needs fixes; Important finding: `NewAuthHTTPHandler` and `NewAdminHTTPHandler` use optional variadic ModuleService parameters and production `server.go` does not explicitly inject one. Minor finding deferred: repeated `user_ids` query keys are silently reduced to the first value.

Task 4: fix loop pending — fix the explicit ModuleService dependency injection; carry the deferred repeated-query observation to final review.

Task 4: fix round 1/5 (1 addressed, 0 open; commits 7a66e43..f6f8803)

Task 4 re-review: constructor injection and shared production wiring addressed; no new Critical/Important breakage. Deferred Minor repeated-query observation remains.

Task 4: complete (commits 4993aea..f6f8803, review clean)

Task 5 review: spec ⚠️ Partial compliance; Important finding: early-window prewarm historical results insert raw archive records and filter before legacy pattern enrichment, so older archives with empty pattern/signal fields can hide authorized purple/blue results. Minor finding deferred: add authorized purple/blue HTTP and prewarm scope coverage in final review triage.

Task 5: fix loop pending — restore enrichment before prewarm historical module filtering; retain the deferred test-coverage observation for final review.

Task 5: fix round 1/5 (1 addressed, 0 open; commits c7aac00..fe9b070)

Task 5 re-review: prewarm legacy enrichment addressed; no new Critical/Important breakage. Deferred Minor authorized-scope coverage remains.

Task 5: complete (commits f6f8803..fe9b070, review clean)

Task 6 implementer status: implementation committed as `013a9cb`, but the required broader regression is blocked because legacy admin assertions in `admin_http_test.go` and `auth_http_test.go` still call the now-user-only `newHTTPHandler`.

Task 6 Ruling: allow the smallest test-only scope expansion to update those legacy admin call sites to the separate admin handler/fixture and include the changed test files in the Task 6 fix commit. This is required to keep the existing admin contract tests valid after the planned listener split; cost if wrong: the Task 6 diff includes test plumbing beyond its original file list and can be reverted without changing production behavior.

Task 6: fix round 1/5 (legacy admin test handlers migrated; commit 5910faf)

Task 6 re-review: spec compliant; no Critical/Important/Minor findings. Focused user/admin/server/auth regression passed. The full package remains affected by the known unrelated T0/global DB nil-pointer path recorded in task-6-report.md.

Task 6: complete (commits fe9b070..5910faf, review clean)

Task 7: complete — `PROJECT.md` documents the separate 8080 user and configurable 18080 management listeners, admin web/static/origin configuration, bcrypt `admin-init`, smoke flow, and HTTPS/reverse-proxy deployment requirements. Known focused backend regression and `go build ./cmd/server ./cmd/admin-init` pass; full package retains the pre-existing T0/global DB nil-pointer risk. See `task-7-report.md`.
