# Module Permission Flutter Client Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (- [ ]) syntax for tracking.

**Goal:** 让 Flutter 用户端读取服务端模块清单，按模块编码动态构建「盘达」Tab，在权限撤销或业务接口返回 403 时安全移除当前入口，并为未来底部导航复用同一权限状态。

**Architecture:** 新增模块权限领域模型、Dio 仓库和 ChangeNotifier 控制器。RadarPage 使用本地六个 Tab 定义与服务端模块编码求交集，运行时重建 TabController，按编码保留当前 Tab 和懒加载状态；T0 ViewModel 按策略模块保存结果状态，向共享接口传递显式 module_code，避免只授权一个策略时把其他策略数据当成已授权数据使用。

**Tech Stack:** Flutter/Dart、Provider、Dio、WidgetsBindingObserver、Flutter widget tests。

**Spec:** docs/superpowers/specs/2026-09-03-module-permission-management-design.md

## Global Constraints

- 首期只配置「盘达」内部 Tab；当前实际是六个 Tab：监控股票（自选）、紫策、主板策略、蓝策、自选异动、全市场。
- 公开模块始终可见：radar.monitored、radar.watch_changes、radar.all_changes。
- 受控模块 radar.purple_strategy、radar.main_strategy、radar.blue_strategy 默认隐藏，服务端授权后才显示。
- 模块权限按稳定编码处理，不使用中文名称或固定 Tab 下标。
- 首次加载和权限请求失败时只展示公开模块；失败不退出普通登录态，并提供重试入口。
- 登录恢复、应用回前台和手动刷新都重新请求 GET /api/auth/modules。
- 当前 Tab 被撤权时停止对应加载/刷新，并回到第一个可见 Tab。
- 业务接口收到 403 MODULE_FORBIDDEN 时撤销本地对应模块可见状态，不推断或连带撤销其他模块。
- /api/auth/modules 只返回模块元数据；T0 业务请求使用 module_code 请求参数，权限模块之间没有耦合。
- 首期不修改 AppShell 底部导航；模块模型保留 placement、parentCode、sort 以兼容未来扩展。

## File Map

### Create

- trading_app/lib/features/permissions/domain/module_definition.dart — 服务端模块元数据和解析。
- trading_app/lib/features/permissions/data/module_permission_repository.dart — /api/auth/modules 请求。
- trading_app/lib/features/permissions/presentation/module_permission_controller.dart — 权限状态、失败回退、撤销和刷新。
- trading_app/lib/features/radar/presentation/radar_list/radar_module_definitions.dart — 六个盘达 Tab 的编码、标题和排序。
- trading_app/test/features/permissions/module_permission_controller_test.dart — 权限加载和状态测试。
- trading_app/test/features/permissions/module_permission_repository_test.dart — JSON/Dio 契约测试。
- trading_app/test/features/permissions/module_permission_test_fixtures.dart — 权限控制器测试源和模块夹具。
- trading_app/test/features/radar/radar_module_definitions_test.dart — 六个 Tab 编码、顺序和公开策略测试。
- trading_app/test/features/radar/radar_page_test_fixtures.dart — Radar widget 测试的 Provider、模块和控制器夹具。

### Modify

- trading_app/lib/app/app_config.dart — 注入权限仓库和控制器；让 T0 ViewModel 能回调权限撤销。
- trading_app/lib/app/app_shell.dart — 监听前后台恢复并触发权限刷新。
- trading_app/lib/features/radar/presentation/radar_list/radar_page.dart — 从固定六个下标改为动态模块 Tab。
- trading_app/lib/features/radar/presentation/radar_list/t0_strategy_view_model.dart — 按策略模块保存结果、日期、加载和错误状态，并携带 module_code。
- trading_app/test/radar_page_test.dart — 动态 Tab、隐藏中间 Tab、撤权回退和失败回退测试。
- trading_app/test/radar_theme_chrome_test.dart — 为 RadarPage 测试补充权限控制器。
- trading_app/test/t0_strategy_view_model_test.dart — 模块范围请求和独立状态测试。

## Dependency

先完成 2026-09-03-module-permission-backend-plan.md 的 /api/auth/modules、MODULE_FORBIDDEN 和 T0 module_code 契约；后端完成后，本计划可与 admin-web 计划并行。Flutter 内部先实现权限基础层，再改 RadarPage。以下 Flutter 命令均以 trading_app/ 为工作目录。

### Task 1: Add the permission domain model and repository

**Files:**

- Create: trading_app/lib/features/permissions/domain/module_definition.dart
- Create: trading_app/lib/features/permissions/data/module_permission_repository.dart
- Create: trading_app/test/features/permissions/module_permission_repository_test.dart

**Interfaces:**

- ModuleDefinition.fromJson(Map<String, dynamic>) parses code, name, client, placement, nullable parentCode, sort, and accessMode.
- ModulePermissionSnapshot.fromJson(Map<String, dynamic>) parses version and modules.
- ModulePermissionSource is an abstract interface with fetchModules() returning Future<ModulePermissionSnapshot>.
- ModulePermissionRepository.fetchModules() returns Future<ModulePermissionSnapshot> through the injected Dio.
- publicModuleDefinitions is the immutable three-module fallback: monitored, watch_changes, all_changes.

- [ ] **Step 1: Write the failing parser and repository tests**

~~~dart
import 'package:dio/dio.dart';

test('parses module metadata and nullable parent code', () {
  final snapshot = ModulePermissionSnapshot.fromJson({
    'version': 1,
    'modules': [
      {
        'code': 'radar.main_strategy',
        'name': '主板策略',
        'client': 'flutter_web',
        'placement': 'radar_tab',
        'parentCode': null,
        'sort': 30,
        'accessMode': 'user_allowlist',
      },
    ],
  });

  expect(snapshot.version, 1);
  expect(snapshot.modules.single.code, 'radar.main_strategy');
  expect(snapshot.modules.single.parentCode, isNull);
  expect(snapshot.modules.single.accessMode,
      ModuleAccessMode.userAllowlist);
});

test('repository requests the authenticated module endpoint', () async {
  final dio = Dio(BaseOptions(baseUrl: 'http://test'));
  String? requestedPath;
  dio.interceptors.add(InterceptorsWrapper(
    onRequest: (options, handler) {
      requestedPath = options.path;
      handler.resolve(Response(
        requestOptions: options,
        data: {'version': 1, 'modules': <dynamic>[]},
      ));
    },
  ));
  final snapshot = await ModulePermissionRepository(dio).fetchModules();
  expect(snapshot.version, 1);
  expect(requestedPath, '/api/auth/modules');
});
~~~

- [ ] **Step 2: Run the repository tests to verify they fail**

Run: flutter test test/features/permissions/module_permission_repository_test.dart

Expected: FAIL because the permission model and repository do not exist.

- [ ] **Step 3: Implement the parser and Dio repository**

Use immutable value objects. Reject a response whose version is not 1 or whose module entry has an empty code. Sort parsed modules by sort while retaining the server response as the visibility source. Do not put stock results or other business data in this repository.

- [ ] **Step 4: Run focused tests and formatting**

Run: dart format lib/features/permissions test/features/permissions

Run: flutter test test/features/permissions/module_permission_repository_test.dart

Expected: PASS.

- [ ] **Step 5: Commit**

~~~bash
git add trading_app/lib/features/permissions/domain/module_definition.dart trading_app/lib/features/permissions/data/module_permission_repository.dart trading_app/test/features/permissions/module_permission_repository_test.dart
git commit -m "feat(radar): add module permission repository"
~~~

### Task 2: Add fail-closed permission state and lifecycle refresh

**Files:**

- Create: trading_app/lib/features/permissions/presentation/module_permission_controller.dart
- Create: trading_app/test/features/permissions/module_permission_controller_test.dart
- Modify: trading_app/lib/app/app_config.dart
- Modify: trading_app/lib/app/app_shell.dart

**Interfaces:**

- ModulePermissionController extends ChangeNotifier.
- ModulePermissionController.forTesting(Iterable<ModuleDefinition>) seeds a ready state for widget tests.
- load({bool force = false}) returns Future<void>.
- refresh() returns Future<void>.
- canView(String moduleCode) returns bool.
- visibleModules returns List<ModuleDefinition>.
- revoke(String moduleCode) removes only one controlled module.
- state is initial, loading, ready, or failure.
- The controller constructor accepts ModulePermissionSource; the fixture file supplies FakeModulePermissionSource(error: ...) and modulesFor(Iterable<String>).

- [ ] **Step 1: Write failing state tests**

~~~dart
test('failure falls back to the three public modules', () async {
  final controller = ModulePermissionController(
    FakeModulePermissionSource(error: StateError('offline')),
  );

  await controller.load();

  expect(controller.state, ModulePermissionState.failure);
  expect(controller.visibleModules.map((m) => m.code), [
    'radar.monitored',
    'radar.watch_changes',
    'radar.all_changes',
  ]);
  expect(controller.canView('radar.main_strategy'), isFalse);
});

test('revoke removes only the selected controlled module', () async {
  final controller = ModulePermissionController(
    FakeModulePermissionSource(
      modules: modulesFor([
        'radar.main_strategy',
        'radar.blue_strategy',
      ]),
    ),
  );
  await controller.load();

  controller.revoke('radar.main_strategy');

  expect(controller.canView('radar.main_strategy'), isFalse);
  expect(controller.canView('radar.blue_strategy'), isTrue);
});
~~~

- [ ] **Step 2: Run the state tests to verify they fail**

Run: flutter test test/features/permissions/module_permission_controller_test.dart

Expected: FAIL because the controller and local public fallback do not exist.

- [ ] **Step 3: Implement the controller and providers**

Use publicModuleDefinitions as the fail-closed fallback. On success, retain only entries with client flutter_web and placement radar_tab; RadarPage further intersects the result with its local six-entry builder catalog, so an unknown server code cannot create a route. On failure, set failure, expose public modules, and allow refresh without clearing ordinary auth state. Deduplicate concurrent loads and make revoke notify immediately.

In AuthenticatedDependencies, create ModulePermissionRepository from the app-wide Dio, then create ModulePermissionController and call load. Construct T0StrategyViewModel after the controller and pass onModuleForbidden: controller.revoke.

Make _AppShellState implement WidgetsBindingObserver, add/remove itself in initState/dispose, and call the controller refresh method when AppLifecycleState.resumed. When state is failure, show a one-line retry message without forcing logout.

- [ ] **Step 4: Run permission, auth, and AppShell tests**

Run: flutter test test/features/permissions test/features/auth test/app/app_shell_theme_test.dart

Expected: PASS; ordinary auth restore remains unchanged and lifecycle refresh does not create a second request while one is active.

- [ ] **Step 5: Commit**

~~~bash
git add trading_app/lib/features/permissions/presentation/module_permission_controller.dart trading_app/test/features/permissions/module_permission_controller_test.dart trading_app/test/features/permissions/module_permission_test_fixtures.dart trading_app/lib/app/app_config.dart trading_app/lib/app/app_shell.dart
git commit -m "feat(radar): load user module permissions"
~~~

### Task 3: Define the six Radar tabs by stable module code

**Files:**

- Create: trading_app/lib/features/radar/presentation/radar_list/radar_module_definitions.dart
- Create: trading_app/test/features/radar/radar_module_definitions_test.dart

**Interfaces:**

- radarModuleDefinitions contains exactly six RadarModuleDefinition values in server sort order.
- Each definition contains code, name, placement radar_tab, sort, and a RadarContentKind enum value: monitored, purpleStrategy, mainStrategy, blueStrategy, watchChanges, or allChanges.
- The three public definitions are marked public; three strategy definitions are marked user_allowlist.

- [ ] **Step 1: Write the failing catalog test**

~~~dart
test('radar catalog has six independent stable modules in server order', () {
  expect(radarModuleDefinitions.map((item) => item.code), [
    'radar.monitored',
    'radar.purple_strategy',
    'radar.main_strategy',
    'radar.blue_strategy',
    'radar.watch_changes',
    'radar.all_changes',
  ]);
  expect(
    radarModuleDefinitions
        .where((item) => item.accessMode == ModuleAccessMode.public),
    hasLength(3),
  );
  expect(
    radarModuleDefinitions.where(
      (item) => item.accessMode == ModuleAccessMode.userAllowlist,
    ),
    hasLength(3),
  );
});
~~~

- [ ] **Step 2: Run the catalog test to verify it fails**

Run: flutter test test/features/radar/radar_module_definitions_test.dart

Expected: FAIL because the six-tab catalog is currently embedded in RadarPage as fixed list positions.

- [ ] **Step 3: Implement the catalog without changing visible content**

Move only identity/order metadata and child selection into RadarModuleDefinition. Keep current stock, change, and strategy widget builders in RadarPage during this task so content and labels remain unchanged. Use the exact six backend codes and keep ParentCode null for first-phase Tab entries.

- [ ] **Step 4: Run the catalog test and format**

Run: dart format lib/features/radar/presentation/radar_list/radar_module_definitions.dart test/features/radar/radar_module_definitions_test.dart

Run: flutter test test/features/radar/radar_module_definitions_test.dart

Expected: PASS; RadarPage widget fixtures are updated and executed in Task 5.

- [ ] **Step 5: Commit**

~~~bash
git add trading_app/lib/features/radar/presentation/radar_list/radar_module_definitions.dart trading_app/test/features/radar/radar_module_definitions_test.dart
git commit -m "feat(radar): register tab module definitions"
~~~

### Task 4: Refactor T0 state and requests to carry independent module scopes

**Files:**

- Modify: trading_app/lib/features/radar/presentation/radar_list/t0_strategy_view_model.dart
- Modify: trading_app/test/t0_strategy_view_model_test.dart

**Interfaces:**

- T0ModuleState stateFor(String moduleCode) stores loading, error, warmup, display date, available dates, selected date, and polling state for one module.
- resultsFor(String moduleCode) returns List<T0StrategyStock> for one module.
- datesFor(String moduleCode), showDateSelector(String moduleCode), canGoPreviousArchive(String moduleCode), and canGoNextArchive(String moduleCode) read only that module's state.
- warmUpIfNeeded({required String moduleCode}) returns Future<void>.
- loadAvailableDates({required String moduleCode}) returns Future<void>.
- loadResults({required String moduleCode, String? date, bool archived = false}) returns Future<void>.
- selectDate(String moduleCode, String date) returns Future<void>.
- selectPreviousArchive(String moduleCode) and selectNextArchive(String moduleCode) retain the existing date-navigation behavior per module.
- T0Request is Future<Map<String, dynamic>> Function(String moduleCode, Map<String, dynamic> query); the ViewModel constructor accepts an optional request for deterministic tests and uses createApiClient in production.
- Constructor accepts void Function(String moduleCode)? onModuleForbidden.
- @visibleForTesting applyResponseForTest(String moduleCode, Map<String, dynamic> data) updates only the named module; existing parsing/sorting tests must pass the module code matching the response.

- [ ] **Step 1: Write failing request-scope tests**

~~~dart
import 'package:dio/dio.dart';

test('result request carries its explicit module code', () async {
  Map<String, dynamic>? seenQuery;
  final vm = T0StrategyViewModel(
    request: (moduleCode, query) async {
      seenQuery = Map<String, dynamic>.from(query);
      return {'results': <dynamic>[]};
    },
  );

  await vm.loadResults(
    moduleCode: 'radar.main_strategy',
    date: '2026-08-11',
    archived: true,
  );

  expect(seenQuery?['module_code'], 'radar.main_strategy');
  expect(vm.resultsFor('radar.purple_strategy'), isEmpty);
});

test('module forbidden callback names only the rejected module', () async {
  String? rejected;
  final vm = T0StrategyViewModel(
    request: (moduleCode, query) async {
      final request = RequestOptions(path: '/api/t0-selection');
      throw DioException(
        requestOptions: request,
        response: Response(
          requestOptions: request,
          statusCode: 403,
          data: {'code': 'MODULE_FORBIDDEN'},
        ),
      );
    },
    onModuleForbidden: (code) => rejected = code,
  );

  await vm.loadResults(moduleCode: 'radar.blue_strategy');

  expect(rejected, 'radar.blue_strategy');
});
~~~

- [ ] **Step 2: Run T0 tests to verify they fail**

Run: flutter test test/t0_strategy_view_model_test.dart

Expected: FAIL because the current ViewModel has one unscoped result list and sends no module_code. The request callback above is the test seam; production continues to use the existing /api/t0-selection URL.

- [ ] **Step 3: Implement per-module T0 state**

Replace the single result/error/loading state with a map keyed by the three strategy module codes. Keep date options shared only when their response is identical; result response, archived response, loading flag, warmup state, selected date, and error belong to the requested module. Add module_code to every T0 request, including prewarm and date metadata, and pass it through existing loading, history, warmup, and quote-update paths.

Keep results as a compatibility delegate to resultsFor('radar.main_strategy'), but make purpleResults and blueResults delegates to their own scoped entries rather than filters over the main list. Update RadarPage and existing ViewModel tests to call resultsFor with an explicit code; preserve the current purple predicate and blue signal presentation in the selected result view through the server-scoped response. Do not infer authorization from a result list.

Catch a Dio 403 whose body code is MODULE_FORBIDDEN, clear only that module state, stop that module's polling, invoke onModuleForbidden(moduleCode), and leave other module states untouched. For network/other errors, set only the requested module error. Update the existing purple/blue predicate tests to exercise their scoped result delegates or pure filters without reading the main module's list.

- [ ] **Step 4: Run T0 tests and format**

Run: dart format lib/features/radar/presentation/radar_list/t0_strategy_view_model.dart test/t0_strategy_view_model_test.dart

Run: flutter test test/t0_strategy_view_model_test.dart

Expected: PASS for query parameters, per-module data, existing date navigation, warmup, quotes, and 403 callback behavior.

- [ ] **Step 5: Commit**

~~~bash
git add trading_app/lib/features/radar/presentation/radar_list/t0_strategy_view_model.dart trading_app/test/t0_strategy_view_model_test.dart
git commit -m "feat(radar): scope t0 data by module"
~~~

### Task 5: Make RadarPage build and reload tabs dynamically

**Files:**

- Modify: trading_app/lib/features/radar/presentation/radar_list/radar_page.dart
- Modify: trading_app/test/radar_page_test.dart
- Create/modify: trading_app/test/features/radar/radar_page_test_fixtures.dart — provide radarTestApp, publicRadarModules, radarModulesFor, and a ready ModulePermissionController fixture while reusing the current Radar/T0/voice fakes.

**Interfaces:**

- visibleRadarModules is derived from ModulePermissionController.visibleModules and the local radarModuleDefinitions catalog.
- TabController length equals visibleRadarModules.length and is never hardcoded to 6.
- loadedModuleCodes is a Set<String>; lazy loading is keyed by module code.
- radarTestApp({List<ModuleDefinition>? modules, ModulePermissionController? permissionController}) returns the widget tree with all providers RadarPage currently requires; omitted modules default to publicRadarModules.

- [ ] **Step 1: Write failing widget tests for the important combinations**

~~~dart
testWidgets('only public modules render three tabs', (tester) async {
  await tester.pumpWidget(radarTestApp(modules: publicRadarModules));
  await tester.pumpAndSettle();

  expect(find.text('监控股票(自选)'), findsOneWidget);
  expect(find.text('自选异动'), findsOneWidget);
  expect(find.text('全市场'), findsOneWidget);
  expect(find.text('紫策'), findsNothing);
  expect(find.text('主板策略'), findsNothing);
  expect(find.text('蓝策'), findsNothing);
});

testWidgets('hiding middle strategy keeps remaining tab identities',
    (tester) async {
  await tester.pumpWidget(radarTestApp(
    modules: [
      ...publicRadarModules,
      ...radarModulesFor([
        'radar.purple_strategy',
        'radar.blue_strategy',
      ]),
    ],
  ));
  await tester.pumpAndSettle();

  expect(find.text('紫策'), findsOneWidget);
  expect(find.text('主板策略'), findsNothing);
  expect(find.text('蓝策'), findsOneWidget);
});

testWidgets('revoking current module returns to first visible tab',
    (tester) async {
  final permissions = ModulePermissionController.forTesting(
    [
      ...publicRadarModules,
      ...radarModulesFor(['radar.main_strategy']),
    ],
  );
  await tester.pumpWidget(radarTestApp(permissionController: permissions));
  await tester.pumpAndSettle();
  await tester.tap(find.text('主板策略'));
  await tester.pump();

  permissions.revoke('radar.main_strategy');
  await tester.pumpAndSettle();

  expect(find.text('主板策略'), findsNothing);
  expect(find.text('监控股票(自选)'), findsOneWidget);
});
~~~

- [ ] **Step 2: Run Radar widget tests to verify they fail**

Run: flutter test test/radar_page_test.dart

Expected: FAIL because RadarPage currently creates TabController length 6, uses fixed indices 1/2/3/4/5, and always renders all six children.

- [ ] **Step 3: Implement code-keyed dynamic Tab construction**

Create the full local tab specification once, filter it by ModulePermissionController.canView(code), and sort by catalog sort. Initialize with public modules while permission state is initial/loading/failure. Add a refresh IconButton to the 盘达 app bar that calls controller.refresh(). When visible code list changes, capture current code, dispose the old controller/listener, create a controller with the new length and an index resolved from the old code or zero, then rebuild TabBar and TabBarView from the same list. Keep at least the three public modules so controller length never reaches zero.

Replace lazy loading by index with lazy loading by module code. Strategy codes call scoped T0 methods; radar.watch_changes and radar.all_changes call the existing RadarViewModel loaders. Store loaded codes, sort flags, and refresh ownership by code. Count labels use t0Vm.resultsFor(code).length. Remove DefaultTabController length 6 and every fixed child/index assumption.

A MODULE_FORBIDDEN callback or permission refresh must cause the same visible-list rebuild and current-code fallback. If the controller changes while a tab is loading, cancel or ignore the old request result when its module code is no longer visible.

- [ ] **Step 4: Run Radar widget and theme tests**

Run: dart format lib/features/radar/presentation/radar_list/radar_page.dart test/radar_page_test.dart

Run: flutter test test/radar_page_test.dart test/radar_theme_chrome_test.dart

Expected: PASS for three public tabs, each single controlled module, all combinations, six-tab ordering, hidden middle tabs, lazy-load mapping, current-tab revoke, and permission failure.

- [ ] **Step 5: Commit**

~~~bash
git add trading_app/lib/features/radar/presentation/radar_list/radar_page.dart trading_app/test/radar_page_test.dart trading_app/test/features/radar/radar_page_test_fixtures.dart
git commit -m "feat(radar): render tabs from module permissions"
~~~

### Task 6: Finish client integration and run complete Flutter verification

**Files:**

- Modify: trading_app/lib/app/app_config.dart, trading_app/lib/app/app_shell.dart, trading_app/lib/features/radar/presentation/radar_list/radar_page.dart — finish provider and lifecycle integration.
- Modify: trading_app/test/features/auth/auth_gate_test.dart, trading_app/test/app/app_shell_theme_test.dart, trading_app/test/radar_theme_chrome_test.dart, trading_app/test/widget_test.dart — add the permission controller to authenticated widget trees.

- [ ] **Step 1: Add provider-aware auth and Radar fixtures**

Update authenticated widget builders so they provide ModulePermissionController before RadarPage. Unauthenticated tests must not instantiate the permission repository or call /api/auth/modules. Test doubles should return public modules unless a test explicitly supplies controlled modules.

- [ ] **Step 2: Run the complete Flutter test suite**

Run: flutter test

Expected: PASS with auth, theme, Radar, T0, news, and strategy tests. No test requires a real network permission response.

- [ ] **Step 3: Run static analysis**

Run: flutter analyze

Expected: no new analyzer errors for provider order, nullable TabController lifecycle, removed fixed-index helpers, or unused result getters. Keep unrelated existing warnings outside this feature unchanged.

- [ ] **Step 4: Build Flutter web and verify the public fallback**

Run: flutter build web --release --dart-define=API_BASE_URL=http://localhost:8080

Expected: PASS. With a test account that has no controlled grants, the built app shows exactly the three public 盘达 tabs. After granting main and refreshing or returning to foreground, only main is added. After revoking it, the tab disappears on the next permission refresh or immediately through the 403 callback.

- [ ] **Step 5: Commit the final Flutter integration**

~~~bash
git add trading_app/lib/app/app_config.dart trading_app/lib/app/app_shell.dart trading_app/lib/features/radar/presentation/radar_list/radar_page.dart trading_app/test/features/auth/auth_gate_test.dart trading_app/test/app/app_shell_theme_test.dart trading_app/test/radar_theme_chrome_test.dart trading_app/test/widget_test.dart
git commit -m "test(radar): verify module permission integration"
~~~
