# Red Auction Reminder Implementation Plan

> **For agentic workers:** Use superpowers:executing-plans to implement these tasks inline.

**Goal:** 已打开的浏览器在 09:25:10 获取红策，每 5 秒重试至 09:26，确认后刷新、提醒并支持点击打开。

**Architecture:** 独立 Dart reminder 管理时钟和请求生命周期；现有 T0 view model 提供确认请求和安全应用结果；浏览器桥接负责权限、每日去重和点击回调；RadarPage 绑定权限、页面生命周期和导航。

**Tech Stack:** Flutter/Dart、现有 Dio、浏览器 Notification/localStorage，无新增依赖。

**Spec:** docs/superpowers/specs/2026-10-24-red-auction-reminder-design.md

## Global Constraints

- 上海时间；首次 09:25:10；重试间隔 5 秒；停止新请求 09:26:00。
- 复用 radar.red_shared_gob_strategy；仅获授权用户；一天一次；网页保持打开。
- 不修改服务端策略及后台任务，不触碰已有缓存/数据库变更。

## Review Focus

- 已禁用、销毁或日期变化后的在途响应不得产生通知。
- 历史列表不得被自动请求切走，点击通知应切回红策和盘达。
- 空确认名单应提醒；未确认、日期错误、数据不完整不得误报成功。
- 超时和周末不得继续请求；跨日要重新排期。
- 不安全来源、拒绝授权、存储不可用和页面重载不得抛出未处理错误。

### Task 1: Reminder scheduling and selection state

**Files:** Create red_auction_reminder.dart and its test under trading_app/lib/features/radar/domain and trading_app/test/features/radar; modify t0_strategy_view_model.dart and its tests.

**Interfaces:** Reminder consumes request(date), onConfirmed(response), onTimeout(); view model exposes fetchRedAuctionConfirmation(date) and acceptRedAuctionConfirmation(response, show: false).

- [x] Write behavior tests for first request, five-second retries, no overlap, valid/empty confirmations, stop time, revocation, weekends and rollover; run to see missing implementation fail.
- [x] Implement reminder and the view model methods; test that background updates preserve historical selection and request the correct module/date.
- [x] Run focused Flutter tests; expected all pass.

### Task 2: Browser notification and application integration

**Files:** Create web/red_auction_notifications.js, Dart conditional bridge and JavaScript behavior test; modify web/index.html, radar_page.dart and app_shell.dart.

**Interfaces:** Bridge exposes permission(), requestPermission() and show(date, userId, count, body, onTap); RadarPage accepts onShowRadar callback from AppShell.

- [x] Add executable JavaScript tests for permission, deduplication, unsupported/insecure contexts and notification click; run to see missing implementation fail.
- [x] Implement browser bridge, permission button and binding to authorized RadarPage lifecycle; click focuses browser, selects red list and switches AppShell to radar.
- [x] Run focused tests, Flutter suite, scoped static analysis and web build. Review diff and report any unavailable/manual browser checks.

## Implementation and verification

- Reminder/view-model tests: 9 passed. JavaScript browser-bridge behavior tests: 4 passed. Tests cover the selected five-second interval, stopping, malformed date/incomplete response handling, empty confirmed results, stale authorization, weekend rollover, historical selection, permission support, reload deduplication and click callbacks.
- Full Flutter suite: 191 passed / 9 failed. The untouched HEAD was extracted to an isolated temporary directory and tested using the same resolved local SDK dependencies: 182 passed / the identical 9 failed. No additional failures were introduced.
- Scoped static analysis: 11 existing warnings/info; the same 11 are present in untouched HEAD. New reminder, bridge and test files have no analysis findings.
- Release Web build passed. Legacy notification_util_web.dart still prevents an optional Wasm build; the JavaScript Web release compiles successfully.
- Independent review identified a permission-refresh race: transient loading must retain a previously granted permission. Reminder enablement now follows canView; actual revocation/failure still invalidates in-flight results. Background date refresh is skipped while reading a historical selection.
- Minor browser limitation: simultaneous tabs use the same notification tag with renotify=false and localStorage deduplication. The storage check is not atomic across tabs; storage-disabled browsers retain only in-page deduplication. Browser timer throttling/sleep can delay the target time.
- Initial tests used controlled browser boundaries. A manual test button and onshow/onerror receipt diagnostics were subsequently added; all 6 JavaScript behavior tests passed, and the release Web build passed again. Production deployment has not been exercised.
- Existing data/cache/database changes were left untouched. Automatic local dependency-resolution churn was reverted from pubspec.lock; no dependencies were added. Tracked generated Web build files are restored after retaining a separate verified build artifact.

### macOS notification investigation, 2026-10-24

- User reported no visible banner after granting browser permission; the page received Notification.onshow. Browser permission alone therefore did not establish visible desktop presentation.
- With explicit user authorization to use AppleScript, inspected macOS 14.5 Notifications and Control Center. Google Chrome and its notification helper were enabled; Chrome initially selected banners, and Focus was not active. No definitive cause for the earlier missing banner was established.
- Observed an actual desktop notification titled “红策测试通知”, originating from localhost:8080, in the upper-right corner at approximately 20:03. It was also visible in Notification Center afterward.
- Changed only Google Chrome notification style from banners to persistent alerts (“提醒”). A fresh UI read returned notifications enabled and the third style button selected; a screenshot confirmed the “提醒” selection.
- A complete native click-to-red-list smoke check was not confirmed: foreground windows changed repeatedly during inspection. The existing automated notification callback tests remain the evidence for click handling. No unrelated notification or privacy settings were changed.

### Existing failing tests

- test/features/radar/radar_module_definitions_test.dart: radar catalog has nine independent stable modules in server order
- test/radar_page_test.dart: 切换进入蓝策也只初始化一次
- test/radar_page_test.dart: 命中显示条件的股票名称在四个策略页均显示为红色
- test/radar_page_test.dart: 紫策位于主板策略左侧并只显示严格阈值达标的股票
- test/radar_page_test.dart: 紫策隐藏开盘涨幅且主板策略仍显示
- test/radar_page_test.dart: 自选异动下拉刷新仍调用自选异动接口
- test/radar_page_test.dart: 蓝策位于主板策略右侧并显示蓝灯数量
- test/red_t1_intraday_preview_test.dart: 切到无数据交易日不沿用旧分时价格且仍可返回
- test/red_t1_intraday_preview_test.dart: 切日请求失败时保留原图并允许重试
