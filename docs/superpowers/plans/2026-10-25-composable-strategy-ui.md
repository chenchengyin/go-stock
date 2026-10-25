# Composable Strategy UI Implementation Plan

> **For agentic workers:** Use superpowers:executing-plans inline, task by task.

**Goal:** Upgrade mainboard to the red strategy chart experience through reusable, composable Flutter UI components.

**Architecture:** Extract presentational slots and stock chips from RadarPage. Reuse synchronized charts with a generic visibility loader and per-module settings; extend the existing Go quote endpoints for mainboard.

**Tech Stack:** Flutter/Dart, Provider, visibility_detector, existing fl_chart, Go net/http.

**Spec:** docs/superpowers/specs/2026-10-25-composable-strategy-ui-design.md

## Global Constraints

- Preserve mainboard selection, sorting and statistical meaning, and existing red behavior.
- Preserve pre-existing working tree edits; work in current dev checkout; no automatic commits or publication.
- Keep legacy red chart/settings entry points compatible. No new dependencies.
- Use radar.main_strategy for mainboard chart data and separate persistent switches.

## Review Focus

- Empty or failed Kline must not suppress available intraday data or stock actions.
- Date/stock/module changes must isolate pending async results and retries.
- Charts disabled must cause no corresponding network requests.
- Offscreen long-list cards must not eagerly fetch mainboard charts.
- Narrow screens and red regression view must retain header, chips and chart interactions.

### Task 1: Mainboard quote support

**Files:** backend/flutter_api/t0_red_kline.go, t0_red_intraday.go, new strategy_charts_test.go.
**Interfaces:** Existing HTTP schemas; admit radar.main_strategy; use the ordinary dated daily/followup path for mainboard.
- [x] Write route tests for mainboard Kline data and intraday session validation/response; unknown modules remain rejected.
- [x] Run Go tests and observe rejection before code change.
- [x] Extend module allowlists without changing red data branches.
- [x] Run focused backend chart tests.

### Task 2: Composable presentation and settings

**Files:** New strategy_card.dart, strategy_stock_card.dart, strategy_stock_chips.dart, stock_code_copy_button.dart, strategy_charts.dart, strategy_chart_settings.dart; legacy chart/settings adapters; RadarPage; SystemSettingsPage; new component/settings tests.
**Interfaces:** StrategyCard(header, sections, onTap), StrategyCardHeader(information, actions, wrap), StrategyStockCard(stock, variant, charts, ...); StrategyCharts retains existing callbacks. StrategyChartSettings exposes enabledFor, intradayEnabledFor, setEnabledFor, setIntradayEnabledFor plus legacy red getters.
- [x] Add tests for card slots/responsive action interaction and independent settings persistence.
- [x] Observe missing behavior, then extract UI and implement module settings.
- [x] Show independent mainboard switches in settings; preserve old red preferences and Provider type.
- [x] Run component, settings and existing radar/chart tests.

### Task 3: Lazy mainboard integration

**Files:** New strategy_chart_exposure.dart and strategy_stock_charts.dart; adapt red_t1_intraday_exposure.dart; RadarPage; new mainboard integration tests.
**Interfaces:** StrategyChartExposure<T>(detectorId, load, builder, shouldRetry, errorMessage); StrategyStockCharts(moduleCode, date, code, showKline, showIntraday, repositories, revision).
- [x] Add tests for mainboard chart display/module queries, independent switches, viewport-only loading, stale responses, failures and session linkage.
- [x] Observe failures, then wire lazy chart sections on mainboard; leave red batch loading intact.
- [x] Verify offscreen/disabled behavior and stale request isolation; run relevant Dart tests and analyzer.

### Task 4: Final validation

- [x] Run complete Flutter suite and backend/flutter_api suite; report unrelated pre-existing failures if any.
- [x] Build Flutter web and inspect local UI if available; no deployment or process replacement.
- [x] Fresh review of changed code, fix material issues with reproducing tests.
- [x] Record outcome and verification in local chat log.
