# 红策策略 Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** 在现有雷达中新增排在紫策之前的红策，复用主板策略结果并用同日期 Gob 日线执行可扩展的服务端二次过滤。

**Architecture:** 后端新增 `radar.red_strategy` 模块。主板当天结果继续由现有选股流程生成并归档，红策在同一请求上下文中复用该批结果和已解码的 Gob 日线 Map；历史日期读取主板 JSON 和对应 Gob。前端只增加模块入口并复用现有 T0 列表，不执行红策业务过滤。

**Tech Stack:** Go HTTP API、Gob 日线缓存、JSON 选股归档、SQLite 形态统计、Flutter/Dart Provider ViewModel。

**Spec:** `docs/superpowers/specs/2026-09-09-red-strategy-design.md`

## Global Constraints

- 红策模块编码固定为 `radar.red_strategy`，显示名称为“红策”，排序位于紫策之前。
- 红策复用主板策略同日期的 `t0_selection_<date>.json` 结果，不生成红策专用 JSON。
- Gob 中使用该股票实际最早一根有效日线；不足 60 根不剔除，T-1 收盘价等于基准价时保留。
- 真赚率保留条件为 `100 - PatternFailPct >= 50`；达标率保留条件为 `PatternWinPct >= 20`。
- 红策请求中同一日期 Gob 只解码一次；日线 Map 不进入 Flutter 状态或 HTTP 响应。
- 红策复用通用三条标红规则，标签使用内存日线重新计算并显示 `P`、`足`、`月`。
- 遵循现有项目命名、排序、访问控制和测试风格；除非用户明确要求，不提交 Git commit。

## File Map

- Create: `backend/flutter_api/t0_red_strategy.go` — 红策上下文、命名过滤链、红策标签重算。
- Create: `backend/flutter_api/t0_red_strategy_test.go` — 红策过滤边界、日线基准和标签测试。
- Modify: `backend/flutter_api/t0_selection.go` — 让当前选股、历史归档、预热响应共享红策所需的 Gob Map，并保持旧入口兼容。
- Modify: `backend/flutter_api/module_registry.go` — 注册红策模块。
- Modify: `backend/flutter_api/module_registry_test.go` — 校验模块顺序。
- Modify: `trading_app/lib/features/radar/presentation/radar_list/radar_module_definitions.dart` — 增加 Flutter 红策目录项。
- Modify: `trading_app/lib/features/radar/presentation/radar_list/t0_strategy_view_model.dart` — 增加红策状态、请求常量和日期切换清理。
- Modify: `trading_app/lib/features/radar/presentation/radar_list/radar_page.dart` — 增加红策 Tab 分支，复用现有列表卡片、标红和标签显示。
- Modify: `trading_app/test/features/radar/radar_module_definitions_test.dart` — 更新目录数量和红策排序断言。
- Modify: `trading_app/test/t0_strategy_view_model_test.dart` — 验证红策请求和日期状态行为。
- Modify: `trading_app/test/radar_page_test.dart` — 验证红策页面复用策略列表展示。

### Task 1: 建立红策纯过滤链

**Files:**
- Create: `backend/flutter_api/t0_red_strategy.go`
- Create: `backend/flutter_api/t0_red_strategy_test.go`
- Modify: `backend/flutter_api/t0_selection.go:407-430`
- Test: `backend/flutter_api/t0_red_strategy_test.go`

**Interfaces:**
- Consumes: `T0SelectionResult`, `dailyBar`, `histBarsBeforeTradeDate`, `t0ShortCodeFromResultCode`, `prevDayRetsFromHist`。
- Produces:
  ```go
  type t0ModuleSelectionContext struct {
      TradeDate string
      Daily     map[string][]dailyBar
  }

  func selectT0ResultsForModule(
      moduleCode string,
      results []T0SelectionResult,
      ctx *t0ModuleSelectionContext,
  ) ([]T0SelectionResult, error)

  func filterRedT0Results(
      results []T0SelectionResult,
      ctx *t0ModuleSelectionContext,
  ) []T0SelectionResult
  ```

- [ ] **Step 1: Add failing unit tests for the price-baseline filter.** Build a `Daily` fixture with 3–5 ascending dates and another fixture with fewer than 60 dates. Assert that `PrevClose < daily[0].Close` is removed, `PrevClose == daily[0].Close` is retained, and a shorter series uses its actual first bar instead of requiring 60 bars.
- [ ] **Step 2: Run the focused test to verify it fails.**

  Run: `./.tools/go/bin/go test ./backend/flutter_api -run 'TestFilterRedT0Results' -count=1`

  Expected: FAIL because the red filter and context do not exist yet.

- [ ] **Step 3: Add failing tests for percentage boundaries and missing data.** Include results with `PatternFailPct` values producing earn rates of `49.99`, `50`, and `50.01`, plus `PatternWinPct` values `19.99`, `20`, and `20.01`. Assert boundary values remain and lower values are removed. Include a result without a Gob daily series and assert it is removed without affecting other results.
- [ ] **Step 4: Implement the named red filter chain.** Add three ordered rules: actual earliest close baseline, earn rate, and win rate. Use copied result slices so filtering and tag updates never mutate the main JSON result slice. The selector should pass `nil` context for main, purple, and blue modules, while red requires the context and returns only filtered results.
- [ ] **Step 5: Add the red tag recalculation to the same result-copy path.** For retained results, use `histBarsBeforeTradeDate(ctx.Daily[shortCode], ctx.TradeDate)` and the existing `pickPrevDayTag` inputs to set `Tag`; do not read the JSON `Tag` value. Keep the existing `P`/`足`/`月` names as the frontend display mapping.
- [ ] **Step 6: Run the focused tests to verify the filter chain passes.**

  Run: `./.tools/go/bin/go test ./backend/flutter_api -run 'Test(FilterRedT0Results|RedT0Tags)' -count=1`

  Expected: PASS, including all boundary and short-history cases.

### Task 2: Share one Gob Map through current, historical, and prewarm responses

**Files:**
- Modify: `backend/flutter_api/t0_selection.go:514-548`
- Modify: `backend/flutter_api/t0_selection.go:638-704`
- Modify: `backend/flutter_api/t0_selection.go:1870-1895`
- Modify: `backend/flutter_api/t0_selection.go:2000-2150`
- Create: `backend/flutter_api/t0_red_selection_test.go`
- Test: `backend/flutter_api/t0_red_selection_test.go`

**Interfaces:**
- Consumes: `t0ModuleSelectionContext`, `filterRedT0Results`, existing `loadT0DailyCache`, `t0DailyCachePayload`。
- Produces:
  ```go
  func runT0SelectionWithDaily(
      tradeDate string,
  ) ([]T0SelectionResult, map[string][]dailyBar, error)

  func RunT0Selection(tradeDate string) ([]T0SelectionResult, error)

  func writeScopedT0ResponseWithDaily(
      w http.ResponseWriter,
      moduleCode string,
      response map[string]interface{},
      daily map[string][]dailyBar,
      fields ...string,
  )
  ```

- [ ] **Step 1: Add a regression test that the legacy `RunT0Selection` signature remains usable.** Keep an existing backtest-style call to `RunT0Selection(date)` and assert it still returns results without exposing daily data to callers.
- [ ] **Step 2: Refactor the existing implementation behind `runT0SelectionWithDaily`.** Move the current `RunT0Selection` body into the new helper, retain the loaded `dailyCache` through result assembly, and return it only from the helper. Make `RunT0Selection` call the helper and discard the Map before returning.
- [ ] **Step 3: Add response-context support without changing non-red behavior.** Keep `writeScopedT0Response` as the compatibility wrapper. Add `writeScopedT0ResponseWithDaily` so red responses use `enrichT0ResultsForDisplayWithDaily` with the already loaded Map, while main, purple, and blue retain the existing lazy Gob read path.
- [ ] **Step 4: Update the current-day handler for red.** When `moduleCode == "radar.red_strategy"`, call `runT0SelectionWithDaily`, save the same main result slice to the existing main archive, create `t0ModuleSelectionContext`, filter/tag the result copy, and pass the same `daily` Map to response display enrichment. Do not call the main HTTP handler or run main selection a second time.
- [ ] **Step 5: Update the historical handler for red.** Read the existing main archive, load the matching Gob once, create the context, then filter/tag and display-enrich the copied results. Preserve existing `ARCHIVE_INCOMPLETE` behavior and do not trigger historical upstream fetches. If the matching Gob is unavailable or invalid, return the established data-not-ready/error response instead of using unverified values.
- [ ] **Step 6: Thread the same cache through the ready prewarm path.** Refactor `buildPrewarmReadyResponseAt` around a helper that accepts an optional already-loaded `*t0DailyCachePayload`. For red, pass that payload to candidate filtering and display enrichment; do not make `writeScopedT0Response` decode the same Gob again. For the pre-open historical response, use the response `display_date` to select its archive context and release all local payload references when the response is complete.
- [ ] **Step 7: Add endpoint-level tests using a temporary cache root.** Create a main archive containing three results and a matching Gob containing one short history, one price-filter failure, and one result that passes all filters. Request `archived=1&module_code=radar.red_strategy`; assert the response count, result identity, recalculated `标记`, and `命中条件`. Add a ready-prewarm test that asserts red candidates are filtered before serialization.
- [ ] **Step 8: Run backend red and archive tests.**

  Run: `./.tools/go/bin/go test ./backend/flutter_api -run 'Test(Red|SelectT0ResultsForModule|PatternBuySignalArchivedSmoke)' -count=1`

  Expected: PASS, with existing main/purple/blue behavior unchanged.

### Task 3: Register the backend module and preserve access control

**Files:**
- Modify: `backend/flutter_api/module_registry.go:1-45`
- Modify: `backend/flutter_api/module_registry_test.go:5-28`
- Test: `backend/flutter_api/module_registry_test.go`

**Interfaces:**
- Consumes: existing `ModuleDefinition` registry and allowlist access mode。
- Produces: `RegisteredModules()` containing `radar.red_strategy` between monitored and purple modules, with `Name: "红策"`, `Sort: 15`, and `ModuleAccessAllowlist`。

- [ ] **Step 1: Extend the registry test expected order.** Insert `radar.red_strategy` between `radar.monitored` and `radar.purple_strategy`, and update the expected module count.
- [ ] **Step 2: Run the registry test to verify it fails.**

  Run: `./.tools/go/bin/go test ./backend/flutter_api -run 'TestRegisteredModulesContainsCurrentRadarTabs' -count=1`

  Expected: FAIL because the registry does not yet contain red.

- [ ] **Step 3: Add the red module definition.** Use the existing allowlist access mode and `Sort: 15`; do not add a database migration or automatic grants. Existing administration and permission APIs remain responsible for granting access.
- [ ] **Step 4: Run module registry and permission tests.**

  Run: `./.tools/go/bin/go test ./backend/flutter_api -run 'Test(RegisteredModules|Module|MigrateAuthTables)' -count=1`

  Expected: PASS.

### Task 4: Add the Flutter red strategy tab using the shared T0 UI

**Files:**
- Modify: `trading_app/lib/features/radar/presentation/radar_list/radar_module_definitions.dart:5-75`
- Modify: `trading_app/lib/features/radar/presentation/radar_list/t0_strategy_view_model.dart:1-145`
- Modify: `trading_app/lib/features/radar/presentation/radar_list/t0_strategy_view_model.dart:430-640`
- Modify: `trading_app/lib/features/radar/presentation/radar_list/radar_page.dart:55-60`
- Modify: `trading_app/lib/features/radar/presentation/radar_list/radar_page.dart:350-425`
- Modify: `trading_app/lib/features/radar/presentation/radar_list/radar_page.dart:900-925`
- Modify: `trading_app/lib/features/radar/presentation/radar_list/radar_page.dart:1110-1215`
- Test: `trading_app/test/features/radar/radar_module_definitions_test.dart`
- Test: `trading_app/test/t0_strategy_view_model_test.dart`
- Test: `trading_app/test/radar_page_test.dart`

**Interfaces:**
- Consumes: backend module code `radar.red_strategy` and existing `T0StrategyStock` JSON fields.
- Produces: a visible red tab using `RadarContentKind.redStrategy`, a dedicated T0 module state, and the existing card display behavior.

- [ ] **Step 1: Add failing Flutter catalog assertions.** Expect seven modules, with order `监控股票、红策、紫策、主板策略、蓝策、自选异动、全市场`; assert four allowlist modules and the red code/name/access mode.
- [ ] **Step 2: Run the focused catalog test to verify it fails.**

  Run: `cd trading_app && flutter test test/features/radar/radar_module_definitions_test.dart`

  Expected: FAIL because red is absent.

- [ ] **Step 3: Add the red module definition and ViewModel state.** Add `RadarContentKind.redStrategy`, `t0RedStrategyModuleCode`, and a red entry to the per-module state map. Keep request parsing unchanged because the server returns the existing `T0StrategyStock` fields.
- [ ] **Step 4: Add the red page branch.** Extend `_StrategyListKind` with `red`, route red through `resultsFor(module.code)`, use the empty text `暂无符合红策条件的股票`, and treat red like main for date navigation, P tag rendering, percentages, and red-name display. Do not add client-side red filtering.
- [ ] **Step 5: Clear old T0 result references on date changes.** In the shared date-selection path, clear the selected module’s `results`, `candidates`, display date, and stale error before starting a new-date request. Keep timers stopped/restarted through existing lifecycle methods so the old module state cannot keep displaying the previous date while a new Gob-backed response is loading.
- [ ] **Step 6: Add ViewModel tests for red routing and date reset.** Inject a request function, return a red JSON result, assert the request contains `module_code: radar.red_strategy`, assert the parsed result is exposed by `resultsFor`, and assert changing from one historical date to another clears the prior list before the new response is applied.
- [ ] **Step 7: Add a widget assertion for red presentation.** Build the existing radar test fixture with red permission and a red T0 result containing `命中条件` and `标记: 涨停破板`; assert the red tab is ordered before purple, the stock name uses the existing red highlight, and the tag is rendered as `[P]`.
- [ ] **Step 8: Format and run focused Flutter tests.**

  Run: `cd trading_app && dart format lib/features/radar/presentation/radar_list/radar_module_definitions.dart lib/features/radar/presentation/radar_list/t0_strategy_view_model.dart lib/features/radar/presentation/radar_list/radar_page.dart test/features/radar/radar_module_definitions_test.dart test/t0_strategy_view_model_test.dart test/radar_page_test.dart`

  Then run: `flutter test test/features/radar/radar_module_definitions_test.dart test/t0_strategy_view_model_test.dart test/radar_page_test.dart`

  Expected: PASS.

### Task 5: Full focused verification and review

**Files:**
- Verify: `backend/flutter_api/t0_red_strategy.go`
- Verify: `backend/flutter_api/t0_selection.go`
- Verify: `backend/flutter_api/module_registry.go`
- Verify: `trading_app/lib/features/radar/presentation/radar_list/radar_module_definitions.dart`
- Verify: `trading_app/lib/features/radar/presentation/radar_list/t0_strategy_view_model.dart`
- Verify: `trading_app/lib/features/radar/presentation/radar_list/radar_page.dart`

**Interfaces:**
- Consumes: completed backend and Flutter tasks.
- Produces: verified red strategy with no unrelated cache, database, generated build, or deployment changes.

- [ ] **Step 1: Run the complete backend package tests.**

  Run: `./.tools/go/bin/go test ./backend/flutter_api -count=1`

  Expected: PASS; investigate only failures caused by the red strategy changes.

- [ ] **Step 2: Run the complete Flutter test suite.**

  Run: `cd trading_app && flutter test`

  Expected: PASS; do not modify unrelated tests or generated cache files to mask failures.

- [ ] **Step 3: Inspect the diff for data-lifecycle regressions.** Confirm no red path stores `*t0DailyCachePayload` or `map[string][]dailyBar` in a global variable, ViewModel state, JSON response, or long-lived timer closure; confirm the same loaded Map is passed to filtering, tag calculation, and display-rule enrichment.
- [ ] **Step 4: Inspect module and JSON boundaries.** Confirm red reads the main archive/result pipeline, does not write a red archive, keeps response `count` equal to filtered results, and leaves main/purple/blue selection behavior unchanged.
- [ ] **Step 5: Report the final changed files and verification commands.** Do not commit, deploy, or rebuild production artifacts unless the user separately requests that follow-up.
