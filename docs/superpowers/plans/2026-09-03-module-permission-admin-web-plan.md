# Module Permission Admin Web Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (- [ ]) syntax for tracking.

**Goal:** 创建独立的 Vue 管理网页，提供功能管理、用户管理、用户多选和模块权限弹窗，并完全移除 Wails 内嵌后台入口。

**Architecture:** admin-web 是独立 Vite Vue 应用，生产产物由 Go 的 :18080 管理服务托管，所有 API 通过同源 /api/admin/* 访问。前端只使用服务端 HttpOnly Cookie，不保存管理员 Token；权限编辑以“选中用户集合 + 受控模块最终集合”一次性提交。

**Tech Stack:** Vue 3、Vite 7、Vue Router 4、Naive UI、Vitest、@vue/test-utils、Fetch API。

**Spec:** docs/superpowers/specs/2026-09-03-module-permission-management-design.md

## Global Constraints

- 后台必须是独立网页，不引用 Wails 运行时，不依赖 frontend/dist。
- 功能管理展示代码注册模块；管理员不能凭空创建客户端不存在的页面。
- 用户管理支持账号/昵称搜索、启用/禁用、多选和“配置模块权限”。
- 公开模块监控股票（自选）、自选异动、全市场显示为锁定，不允许勾选修改。
- 受控模块紫策、主板策略、蓝策显示复选框；权限按具体用户独立生效。
- 多用户权限弹窗使用全选、未选、半选三态；保存覆盖所有选中用户的受控集合。
- moduleCodes 为空表示撤销选中用户的全部受控模块权限。
- 使用管理员 HttpOnly Cookie，不使用 localStorage 保存 Token，不显示或预填 admin/admin。
- 生产网页与 API 同源；开发通过 Vite proxy 访问 GO_STOCK_ADMIN_ADDR，不在生产开放通配 CORS。
- 保存成功后重新查询用户权限；错误时保留当前选择并展示后端错误消息。

## File Map

### Create

- admin-web/package.json — 独立依赖和脚本。
- admin-web/index.html — 管理网页入口。
- admin-web/vite.config.js — Vue 插件、开发端口和 /api 代理。
- admin-web/vitest.config.js — jsdom 测试环境。
- admin-web/src/main.js — Vue、Router、Naive UI provider 挂载。
- admin-web/src/router.js — 登录、用户管理、功能管理路由。
- admin-web/src/api.js — 同源 Fetch、Cookie 会话和统一错误转换。
- admin-web/src/App.vue — 后台布局、导航和全局会话状态。
- admin-web/src/views/LoginView.vue — 管理员登录表单。
- admin-web/src/views/UserManagementView.vue — 用户列表、搜索、状态和批量权限入口。
- admin-web/src/views/FunctionManagementView.vue — 模块目录和授权人数/反向查看。
- admin-web/src/components/ModulePermissionDialog.vue — 多用户模块复选弹窗。
- admin-web/src/permission-selection.js — 三态计算和批量选择纯函数。
- admin-web/src/styles.css — 独立网页布局样式。
- admin-web/test/api.test.js、login.test.js、permission-selection.test.js、ModulePermissionDialog.test.js、FunctionManagementView.test.js — API 和界面行为测试。

### Delete/Modify

- Delete: frontend/src/components/admin.vue — 移除 Wails 内嵌后台。
- Modify: frontend/src/router/router.js — 删除 /admin 路由和 import。
- Modify: frontend/src/App.vue — 删除“后台管理”菜单入口。

## Dependency

先完成 2026-09-03-module-permission-backend-plan.md 的 API、Cookie 和静态目录任务，再执行本计划。后端返回字段以设计规格为准：items/total 用户列表、users[] 权限查询、modules[] 模块清单、{status: "ok"} 批量保存结果。

### Task 1: Scaffold the standalone Vue app and API client

**Files:**

- Create: admin-web/package.json
- Create: admin-web/index.html
- Create: admin-web/vite.config.js
- Create: admin-web/vitest.config.js
- Create: admin-web/src/main.js
- Create: admin-web/src/api.js
- Create: admin-web/src/styles.css
- Create: admin-web/test/api.test.js

**Interfaces:**

- apiRequest(path, options = {}) returns decoded JSON or throws AdminApiError with code, message, and status.
- adminApi.login(username, password) calls POST /api/admin/login with credentials include.
- adminApi.logout(), me(), listUsers(keyword), updateUserStatus(userId, status), listModules(), getAccess(userIds), replaceAccess(userIds, moduleCodes), and listModuleUsers(moduleCode) each map to one backend route.

- [ ] **Step 1: Write failing API client tests**

~~~js
import { afterEach, expect, test, vi } from 'vitest'
import { adminApi, apiRequest } from '../src/api.js'

afterEach(() => vi.restoreAllMocks())

test('api client sends cookie credentials and never stores a token', async () => {
  global.fetch = vi.fn().mockResolvedValue(new Response(
    JSON.stringify({ user: { id: 'admin-a' } }),
    { status: 200, headers: { 'Content-Type': 'application/json' } },
  ))

  await adminApi.me()

  expect(fetch).toHaveBeenCalledWith('/api/admin/me', expect.objectContaining({
    credentials: 'include',
  }))
  expect(localStorage.getItem('adminAccessToken')).toBeNull()
})

test('api client converts an expired session into a structured error', async () => {
  global.fetch = vi.fn().mockResolvedValue(new Response(
    JSON.stringify({
      code: 'ADMIN_UNAUTHENTICATED',
      message: '管理员未登录',
    }),
    { status: 401, headers: { 'Content-Type': 'application/json' } },
  ))

  await expect(apiRequest('/api/admin/me')).rejects.toMatchObject({
    code: 'ADMIN_UNAUTHENTICATED',
    status: 401,
  })
})
~~~

- [ ] **Step 2: Run the tests to verify they fail**

Run: pnpm --dir admin-web test -- --run test/api.test.js

Expected: FAIL because the independent app and API client do not exist.

- [ ] **Step 3: Create the minimal app and Fetch wrapper**

Use a minimal package containing vue, vue-router, naive-ui, vite, @vitejs/plugin-vue, vitest, jsdom, and @vue/test-utils. Configure Vite with a 5174 development port and a proxy from /api to process.env.VITE_ADMIN_API_TARGET or http://localhost:18080. Set Vitest environment to jsdom.

apiRequest sets credentials: include, sets Content-Type only when a body exists, JSON-encodes request bodies, parses JSON responses, and converts non-2xx responses to AdminApiError. It never reads or writes an access token. A 401 or 403 is returned to the view layer so the app can redirect to login and clear in-memory state.

- [ ] **Step 4: Run the API tests and build**

Run: pnpm --dir admin-web test -- --run test/api.test.js

Expected: PASS.

Run: pnpm --dir admin-web build

Expected: PASS and create admin-web/dist/index.html.

- [ ] **Step 5: Commit**

~~~bash
git add admin-web/package.json admin-web/index.html admin-web/vite.config.js admin-web/vitest.config.js admin-web/src/main.js admin-web/src/api.js admin-web/src/styles.css admin-web/test/api.test.js
git commit -m "feat(admin-web): scaffold standalone management app"
~~~

### Task 2: Add session layout and administrator login

**Files:**

- Create: admin-web/src/router.js
- Create: admin-web/src/App.vue
- Create: admin-web/src/views/LoginView.vue
- Create: admin-web/test/login.test.js
- Modify: admin-web/src/main.js

**Interfaces:**

- Routes are /login, /users, and /functions; / redirects to /users after session check.
- App state tracks currentAdmin and sessionChecked in memory only.
- Login calls adminApi.login, then adminApi.me, then routes to /users.
- Logout calls adminApi.logout, clears memory, and routes to /login.

- [ ] **Step 1: Write the failing login and route test**

~~~js
import { flushPromises, mount } from '@vue/test-utils'
import { expect, test, vi } from 'vitest'
import App from '../src/App.vue'
import { adminApi } from '../src/api.js'
import router from '../src/router.js'

test('unauthenticated app renders login without default credentials', async () => {
  vi.spyOn(adminApi, 'me').mockRejectedValue({
    status: 401,
    code: 'ADMIN_UNAUTHENTICATED',
  })
  const wrapper = mount(App, { global: { plugins: [router] } })
  await router.isReady()
  await flushPromises()

  expect(wrapper.text()).toContain('后台管理')
  expect(wrapper.text()).toContain('登录')
  expect(wrapper.text()).not.toContain('admin/admin')
})
~~~

- [ ] **Step 2: Run the focused test to verify it fails**

Run: pnpm --dir admin-web test -- --run test/login.test.js

Expected: FAIL because routes, App layout, and LoginView are not present.

- [ ] **Step 3: Implement the session gate and layout**

Create a left navigation with 用户管理 and 功能管理, a top bar with the current admin nickname/account and 退出. On first mount call adminApi.me; a 401 redirects to /login. Login inputs are empty by default, use username and password, and never mention the legacy hardcoded account. Login relies on the server Set-Cookie response and does not process an accessToken field. Logout is best effort and always clears in-memory state and routes to /login.

- [ ] **Step 4: Run login/build checks**

Run: pnpm --dir admin-web test -- --run test/login.test.js

Expected: PASS.

Run: pnpm --dir admin-web build

Expected: PASS.

- [ ] **Step 5: Commit**

~~~bash
git add admin-web/src/main.js admin-web/src/router.js admin-web/src/App.vue admin-web/src/views/LoginView.vue admin-web/test/login.test.js
git commit -m "feat(admin-web): add administrator login shell"
~~~

### Task 3: Implement user management and the multi-user permission dialog

**Files:**

- Create: admin-web/src/views/UserManagementView.vue
- Create: admin-web/src/components/ModulePermissionDialog.vue
- Create: admin-web/src/permission-selection.js
- Create: admin-web/test/permission-selection.test.js
- Create: admin-web/test/ModulePermissionDialog.test.js

**Interfaces:**

- moduleCheckState(accessByUser, moduleCode) returns checked, unchecked, or indeterminate.
- selectedControlledCodes(modules, selectedCodes) excludes public modules.
- adminApi.getAccess(selectedUserIds) returns users with userId and moduleCodes.
- adminApi.replaceAccess(selectedUserIds, moduleCodes) replaces the selected users' controlled grants.

- [ ] **Step 1: Write failing pure-function and dialog tests**

~~~js
import { expect, test } from 'vitest'
import {
  moduleCheckState,
  selectedControlledCodes,
} from '../src/permission-selection.js'

const modules = [
  { code: 'radar.monitored', accessMode: 'public' },
  { code: 'radar.main_strategy', accessMode: 'user_allowlist' },
  { code: 'radar.blue_strategy', accessMode: 'user_allowlist' },
]

test('module state is checked only when every selected user has the grant', () => {
  expect(moduleCheckState([
    { userId: 'a', moduleCodes: ['radar.main_strategy'] },
    { userId: 'b', moduleCodes: ['radar.main_strategy'] },
  ], 'radar.main_strategy')).toBe('checked')

  expect(moduleCheckState([
    { userId: 'a', moduleCodes: ['radar.main_strategy'] },
    { userId: 'b', moduleCodes: [] },
  ], 'radar.main_strategy')).toBe('indeterminate')

  expect(moduleCheckState([
    { userId: 'a', moduleCodes: [] },
    { userId: 'b', moduleCodes: [] },
  ], 'radar.main_strategy')).toBe('unchecked')
})

test('public modules cannot enter the save payload', () => {
  expect(selectedControlledCodes(modules, [
    'radar.monitored',
    'radar.blue_strategy',
  ])).toEqual(['radar.blue_strategy'])
})
~~~

- [ ] **Step 2: Run the selection tests to verify they fail**

Run: pnpm --dir admin-web test -- --run test/permission-selection.test.js test/ModulePermissionDialog.test.js

Expected: FAIL because the selection functions and dialog do not exist.

- [ ] **Step 3: Implement user table selection and status actions**

Load adminApi.listUsers(keyword) on mount and on search submit. Render account, nickname, role, status, registration time, and status action. Keep selected user IDs in a Set, expose selectedCount, and disable 配置模块权限 when the count is zero. PATCH /api/admin/users/{id}/status toggles only active/disabled, then reloads the list; errors leave the row unchanged and show message.error.

- [ ] **Step 4: Implement the permission dialog and exact batch semantics**

When opened, call listModules and getAccess(selectedUserIds). Render every module sorted by backend sort; public modules show a locked tag and disabled checkbox. For every controlled module, calculate checked, unchecked, or indeterminate from all selected users. A half-selected checkbox must support an explicit click to checked or unchecked.

On 完成, send exactly the selected user IDs and final controlled checkbox set. An empty set sends moduleCodes: [] and revokes every controlled module. Disable save while pending; success closes the dialog, reloads users/access state, and shows success; failure keeps the dialog open and shows the backend message. Never infer one module from another.

- [ ] **Step 5: Run dialog tests and build**

Run: pnpm --dir admin-web test -- --run test/permission-selection.test.js test/ModulePermissionDialog.test.js

Expected: PASS for all/none/partial selection, public lock, empty revoke, multi-user save, and error retention.

Run: pnpm --dir admin-web build

Expected: PASS.

- [ ] **Step 6: Commit**

~~~bash
git add admin-web/src/views/UserManagementView.vue admin-web/src/components/ModulePermissionDialog.vue admin-web/src/permission-selection.js admin-web/test/permission-selection.test.js admin-web/test/ModulePermissionDialog.test.js
git commit -m "feat(admin-web): add batch module permission management"
~~~

### Task 4: Implement function management and reverse authorization lookup

**Files:**

- Create: admin-web/src/views/FunctionManagementView.vue
- Create: admin-web/test/FunctionManagementView.test.js
- Modify: admin-web/src/router.js, admin-web/src/App.vue

**Interfaces:**

- GET /api/admin/modules populates the module table.
- GET /api/admin/modules/{module_code}/users populates the selected module authorized-user panel.

- [ ] **Step 1: Write the failing function-management test**

~~~js
import { flushPromises, mount } from '@vue/test-utils'
import { expect, test, vi } from 'vitest'
import FunctionManagementView from '../src/views/FunctionManagementView.vue'
import { adminApi } from '../src/api.js'

test('function management locks public modules and shows controlled counts', async () => {
  vi.spyOn(adminApi, 'listModules').mockResolvedValue({
    modules: [
      {
        code: 'radar.monitored',
        name: '监控股票（自选）',
        accessMode: 'public',
        authorizedUserCount: 0,
      },
      {
        code: 'radar.main_strategy',
        name: '主板策略',
        accessMode: 'user_allowlist',
        authorizedUserCount: 2,
      },
    ],
  })
  const wrapper = mount(FunctionManagementView)
  await flushPromises()

  expect(wrapper.text()).toContain('公开模块')
  expect(wrapper.text()).toContain('2')
  expect(wrapper.get(
    '[data-module-code="radar.monitored"] [data-role="public-lock"]',
  ).exists()).toBe(true)
})
~~~

- [ ] **Step 2: Run the test to verify it fails**

Run: pnpm --dir admin-web test -- --run test/FunctionManagementView.test.js

Expected: FAIL because the function-management view does not exist.

- [ ] **Step 3: Implement the module table and reverse lookup panel**

Display module name, code, client, placement, access policy, and authorized user count. Render each row with data-module-code; public rows show 公开模块 and a data-role="public-lock" indicator. Controlled rows show 用户授权模块 and a 查看用户 action that calls the reverse endpoint and renders account, nickname, and status. Keep editing in the user-management dialog; function management is the directory and reverse-view surface. Use a native table for the module rows so the behavior test observes the actual lock marker rather than a test-only component stub.

- [ ] **Step 4: Run the view test and build**

Run: pnpm --dir admin-web test -- --run test/FunctionManagementView.test.js

Expected: PASS.

Run: pnpm --dir admin-web build

Expected: PASS.

- [ ] **Step 5: Commit**

~~~bash
git add admin-web/src/views/FunctionManagementView.vue admin-web/src/router.js admin-web/src/App.vue admin-web/test/FunctionManagementView.test.js
git commit -m "feat(admin-web): add function management view"
~~~

### Task 5: Remove the embedded Wails admin and verify the standalone build

**Files:**

- Delete: frontend/src/components/admin.vue
- Modify: frontend/src/router/router.js
- Modify: frontend/src/App.vue
- Modify: PROJECT.md

- [ ] **Step 1: Write the repository boundary check**

Run this search after the removal:

~~~bash
rg -n "admin\\.vue|path: '/admin'|name: 'admin'|adminAccessToken|admin/admin" frontend admin-web PROJECT.md
~~~

Expected: no embedded Wails admin route, localStorage token, or hardcoded credentials remains. The new standalone login labels and URL may remain.

- [ ] **Step 2: Remove only the embedded admin surface**

Delete admin.vue, its import and /admin route from frontend/src/router/router.js, and only the 后台管理 menu item from frontend/src/App.vue. Keep unrelated Wails pages and business APIs unchanged. The backend plan owns the matching PROJECT.md deployment instructions.

- [ ] **Step 3: Run both frontend builds and all admin-web tests**

Run: pnpm --dir admin-web test -- --run

Expected: PASS.

Run: pnpm --dir admin-web build

Expected: PASS.

Run: pnpm --dir frontend build

Expected: PASS with no admin route import errors.

- [ ] **Step 4: Commit**

~~~bash
git add frontend/src/components/admin.vue frontend/src/router/router.js frontend/src/App.vue
git commit -m "refactor(admin): remove embedded management page"
~~~

### Task 6: Run the standalone browser smoke test

**Files:**

- None. A smoke-test defect must be fixed in the owning implementation task with a regression test before this verification is rerun.

- [ ] **Step 1: Build and start the backend**

Run: pnpm --dir admin-web build

Run: GO_STOCK_ADMIN_ADDR=:18080 go run ./cmd/server

Expected: the Go process exposes the user API/Web on 8080 and management web/API on 18080; 8080 does not expose management data.

- [ ] **Step 2: Verify the browser workflow**

Open http://服务器地址:18080/ and verify:

~~~text
管理员登录
  → 用户管理
  → 搜索并勾选多个普通用户
  → 配置模块权限
  → 公开模块锁定，受控模块支持半选
  → 完成
  → 刷新后权限保持
  → 功能管理查看模块和授权人数
  → 查看主板策略已授权用户
~~~

Expected: the browser has no visible Token storage, a regular user cannot load admin data, and selecting only main does not select purple or blue.

- [ ] **Step 3: Run final repository checks**

Run: git status --short

Expected: no unrelated workspace files are staged.

Run: git diff --check

Expected: no output and exit code 0.
