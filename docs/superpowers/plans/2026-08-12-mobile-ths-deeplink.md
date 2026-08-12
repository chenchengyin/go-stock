# 手机浏览器同花顺 Deep Link Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** 让 Flutter Web 在 Android 手机浏览器点击同花顺图标时优先 Intent 唤起同花顺 App 到该股票页，失败则开网页；电脑等非 Android Web 直接开网页；原生 App 行为不变。

**Architecture:** 所有逻辑集中在 `StockLauncher`。抽出可单测的纯函数（代码规范化、Android Web Intent 判定、Intent/网页/amihexin URI 拼装），`openTongHuaShun` 按 `kIsWeb` + `defaultTargetPlatform`（Web 上由浏览器 UA 推导）分流。Android Web 的 Intent 路径不依赖 `canLaunchUrl`（Web 上常误报 false）。

**Tech Stack:** Flutter / Dart 3.8、`url_launcher`、`flutter_test`

## Global Constraints

- 主要目标平台：Android 手机浏览器；不做 iOS Universal Link / 应用商店跳转
- 调用方（雷达、详情、设置）不改；不改后端、AndroidManifest、UI
- 未装 App / 唤起失败 → `https://stockpage.10jqka.com.cn/{纯数字代码}/`
- 唤起串不强制附带 `marketId`（与现状一致）
- Intent 失败静默兜底网页，不新增「未安装」提示

---

## File Structure

| 文件 | 职责 |
|---|---|
| `trading_app/lib/core/utils/stock_launcher.dart` | 分流逻辑、URI 拼装、启动 |
| `trading_app/test/core/utils/stock_launcher_test.dart` | 纯函数单测 |

不新增其他源文件。

---

### Task 1: 纯函数 API + 单测

**Files:**
- Modify: `trading_app/lib/core/utils/stock_launcher.dart`
- Create: `trading_app/test/core/utils/stock_launcher_test.dart`

**Interfaces:**
- Consumes: 无
- Produces:
  - `static String normalizeStockCode(String code)` — 只保留数字；可公开或 `@visibleForTesting`
  - `static bool shouldUseAndroidWebIntent({required bool isWeb, required TargetPlatform platform})` — `isWeb && platform == TargetPlatform.android`
  - `static bool isAndroidMobileUserAgent(String userAgent)` — UA 小写含 `android`（与规格可选单测对齐；运行时主判定用 `shouldUseAndroidWebIntent`）
  - `static Uri buildTongHuaShunAppUri(String normalizedCode)` — `amihexin://...`
  - `static Uri buildTongHuaShunIntentUri(String normalizedCode)` — `intent://...#Intent;scheme=amihexin;package=com.hexin.plat.android;end`
  - `static Uri buildTongHuaShunWebUri(String normalizedCode)` — `https://stockpage.10jqka.com.cn/{code}/`

- [ ] **Step 1: 写失败单测**

创建 `trading_app/test/core/utils/stock_launcher_test.dart`：

```dart
import 'package:flutter/foundation.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:trading_app/core/utils/stock_launcher.dart';

void main() {
  group('normalizeStockCode', () {
    test('strips exchange suffix and non-digits', () {
      expect(StockLauncher.normalizeStockCode('002558.XSHE'), '002558');
      expect(StockLauncher.normalizeStockCode('sh600519'), '600519');
      expect(StockLauncher.normalizeStockCode(''), '');
    });
  });

  group('shouldUseAndroidWebIntent', () {
    test('true only for web + android', () {
      expect(
        StockLauncher.shouldUseAndroidWebIntent(
          isWeb: true,
          platform: TargetPlatform.android,
        ),
        isTrue,
      );
      expect(
        StockLauncher.shouldUseAndroidWebIntent(
          isWeb: true,
          platform: TargetPlatform.iOS,
        ),
        isFalse,
      );
      expect(
        StockLauncher.shouldUseAndroidWebIntent(
          isWeb: false,
          platform: TargetPlatform.android,
        ),
        isFalse,
      );
      expect(
        StockLauncher.shouldUseAndroidWebIntent(
          isWeb: true,
          platform: TargetPlatform.macOS,
        ),
        isFalse,
      );
    });
  });

  group('isAndroidMobileUserAgent', () {
    test('detects android browsers', () {
      expect(
        StockLauncher.isAndroidMobileUserAgent(
          'Mozilla/5.0 (Linux; Android 13; Pixel 7) AppleWebKit/537.36 '
          '(KHTML, like Gecko) Chrome/120.0.0.0 Mobile Safari/537.36',
        ),
        isTrue,
      );
      expect(
        StockLauncher.isAndroidMobileUserAgent(
          'Mozilla/5.0 (iPhone; CPU iPhone OS 17_0 like Mac OS X) '
          'AppleWebKit/605.1.15 (KHTML, like Gecko) Version/17.0 Mobile/15E148 Safari/604.1',
        ),
        isFalse,
      );
      expect(
        StockLauncher.isAndroidMobileUserAgent(
          'Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 '
          '(KHTML, like Gecko) Chrome/120.0.0.0 Safari/537.36',
        ),
        isFalse,
      );
    });
  });

  group('URI builders', () {
    test('app uri keeps amihexin scheme and stockcode', () {
      final uri = StockLauncher.buildTongHuaShunAppUri('601318');
      expect(uri.scheme, 'amihexin');
      expect(uri.toString(), contains('stockcode//=601318//'));
    });

    test('intent uri embeds scheme package and stockcode', () {
      final uri = StockLauncher.buildTongHuaShunIntentUri('002558');
      final s = uri.toString();
      expect(s.startsWith('intent://'), isTrue);
      expect(s, contains('stockcode//=002558//'));
      expect(s, contains('scheme=amihexin'));
      expect(s, contains('package=com.hexin.plat.android'));
      expect(s.endsWith(';end') || s.contains(';end'), isTrue);
    });

    test('web uri points to stockpage', () {
      expect(
        StockLauncher.buildTongHuaShunWebUri('600519').toString(),
        'https://stockpage.10jqka.com.cn/600519/',
      );
    });
  });
}
```

- [ ] **Step 2: 跑测确认失败**

Run:

```bash
cd trading_app && flutter test test/core/utils/stock_launcher_test.dart
```

Expected: FAIL（`normalizeStockCode` / `shouldUseAndroidWebIntent` / builders 未定义，或仍为 private `_normalizeCode`）

- [ ] **Step 3: 实现纯函数（先不改 openTongHuaShun 分流）**

在 `stock_launcher.dart` 增加（可把现有 `_normalizeCode` 改为公开的 `normalizeStockCode`，旧名委托到新名）：

```dart
import 'package:flutter/foundation.dart';
import 'package:url_launcher/url_launcher.dart';

class StockLauncher {
  const StockLauncher._();

  static String normalizeStockCode(String code) {
    return code.replaceAll(RegExp(r'[^0-9]'), '');
  }

  static bool shouldUseAndroidWebIntent({
    required bool isWeb,
    required TargetPlatform platform,
  }) {
    return isWeb && platform == TargetPlatform.android;
  }

  static bool isAndroidMobileUserAgent(String userAgent) {
    return userAgent.toLowerCase().contains('android');
  }

  static Uri buildTongHuaShunAppUri(String normalizedCode) {
    return Uri.parse(
      'amihexin://command//=XXXX//'
      '&action//=GGFS//'
      '&stockcode//=$normalizedCode//'
      '&applicationScheme//=XXXX//',
    );
  }

  static Uri buildTongHuaShunIntentUri(String normalizedCode) {
    return Uri.parse(
      'intent://command//=XXXX//'
      '&action//=GGFS//'
      '&stockcode//=$normalizedCode//'
      '&applicationScheme//=XXXX//'
      '#Intent;scheme=amihexin;package=com.hexin.plat.android;end',
    );
  }

  static Uri buildTongHuaShunWebUri(String normalizedCode) {
    return Uri.parse('https://stockpage.10jqka.com.cn/$normalizedCode/');
  }

  // 保留现有 openTongHuaShun / inferTongHuaShunMarketId / _tryLaunch；
  // 本 Task 仅让 openTongHuaShun 内部改用 normalizeStockCode + buildTongHuaShunAppUri/WebUri，
  // 分流逻辑留到 Task 2。
}
```

将现有 `openTongHuaShun` 中的拼接改为调用 `normalizeStockCode` + `buildTongHuaShunAppUri` / `buildTongHuaShunWebUri`，行为暂与改前一致（仍先 app 后 web）。

- [ ] **Step 4: 跑测确认通过**

Run:

```bash
cd trading_app && flutter test test/core/utils/stock_launcher_test.dart
```

Expected: PASS（全部用例）

- [ ] **Step 5: Commit**

```bash
git add trading_app/lib/core/utils/stock_launcher.dart \
  trading_app/test/core/utils/stock_launcher_test.dart
git commit -m "$(cat <<'EOF'
feat(web): extract Tonghuashun URI helpers and unit tests

EOF
)"
```

---

### Task 2: openTongHuaShun 按环境分流

**Files:**
- Modify: `trading_app/lib/core/utils/stock_launcher.dart`

**Interfaces:**
- Consumes: Task 1 全部 Produces
- Produces: 更新后的 `openTongHuaShun` 行为（对外签名保持 `openTongHuaShun({required String code, String? marketId})`；可增加 `@visibleForTesting` 可选覆盖参数便于测分流，不强制）

- [ ] **Step 1: 实现分流与 Web Intent 启动**

将 `openTongHuaShun` 改为：

```dart
static Future<bool> openTongHuaShun({
  required String code,
  String? marketId,
  @visibleForTesting bool? isWebOverride,
  @visibleForTesting TargetPlatform? platformOverride,
}) async {
  final normalizedCode = normalizeStockCode(code);
  if (normalizedCode.isEmpty) {
    return false;
  }

  // marketId 保留参数与 inferTongHuaShunMarketId，供未来使用；当前唤起串不强制附带
  final _ = marketId ?? inferTongHuaShunMarketId(normalizedCode);

  final isWeb = isWebOverride ?? kIsWeb;
  final platform = platformOverride ?? defaultTargetPlatform;

  if (shouldUseAndroidWebIntent(isWeb: isWeb, platform: platform)) {
    if (await _tryLaunchPreferLaunch(buildTongHuaShunIntentUri(normalizedCode))) {
      return true;
    }
    return _tryLaunch(buildTongHuaShunWebUri(normalizedCode));
  }

  if (isWeb) {
    return _tryLaunch(buildTongHuaShunWebUri(normalizedCode));
  }

  if (await _tryLaunch(buildTongHuaShunAppUri(normalizedCode))) {
    return true;
  }
  return _tryLaunch(buildTongHuaShunWebUri(normalizedCode));
}

/// Web Intent：跳过 canLaunchUrl（Flutter Web 对 intent:// 常误报不可用）
static Future<bool> _tryLaunchPreferLaunch(Uri uri) async {
  try {
    return await launchUrl(uri, mode: LaunchMode.externalApplication);
  } catch (error) {
    debugPrint('Launch stock url failed: $uri, $error');
    return false;
  }
}

static Future<bool> _tryLaunch(Uri uri) async {
  try {
    if (!await canLaunchUrl(uri)) {
      return false;
    }
    return launchUrl(uri, mode: LaunchMode.externalApplication);
  } catch (error) {
    debugPrint('Launch stock url failed: $uri, $error');
    return false;
  }
}
```

说明：若不想用 `_` 吃掉 marketId，可保留 `resolvedMarketId` 变量但不拼进 URI（与规格一致）。

- [ ] **Step 2: 补一条分流相关单测（可选但推荐）**

在 `stock_launcher_test.dart` 已有 `shouldUseAndroidWebIntent` 覆盖下，无需 mock `url_launcher`。若已有 override 参数，可不测 `openTongHuaShun` 异步启动。

确认 Task 1 测试仍全部通过即可。

- [ ] **Step 3: 跑测**

Run:

```bash
cd trading_app && flutter test test/core/utils/stock_launcher_test.dart
```

Expected: PASS

- [ ] **Step 4: Commit**

```bash
git add trading_app/lib/core/utils/stock_launcher.dart \
  trading_app/test/core/utils/stock_launcher_test.dart
git commit -m "$(cat <<'EOF'
feat(web): open Tonghuashun via Android Intent on mobile web

EOF
)"
```

---

### Task 3: Web 构建与手工验收清单

**Files:**
- 无源码改动（按需重建并部署 Flutter Web；部署步骤沿用仓库现有 `feat(web): serve flutter web build` 流程）

**Interfaces:**
- Consumes: Task 2 完成后的 `StockLauncher`
- Produces: 可在 `http://<host>:8080/` 验证的 Web 产物

- [ ] **Step 1: 本地分析**

Run:

```bash
cd trading_app && flutter analyze lib/core/utils/stock_launcher.dart
```

Expected: 无 error（允许项目既有无关 warning，但本文件不应引入新 error）

- [ ] **Step 2: 按需 rebuild web**

若当前部署依赖 `trading_app/build/web`（或仓库既定输出目录），执行项目惯用的 web build 命令，例如：

```bash
cd trading_app && flutter build web --release
```

然后将产物按现有方式挂到后端静态目录并重启/部署到目标机（与此前公网 IP 访问同一路径）。**具体部署命令以仓库现有脚本为准，本 Task 不发明新部署体系。**

- [ ] **Step 3: 手工验收（对照规格）**

1. Android 手机浏览器打开站点，已装同花顺 → 点图标进入该股票页  
2. 未装 / 拦截 → 打开 `stockpage.10jqka.com.cn/{代码}/`  
3. 电脑浏览器 → 直接开网页，不尝试唤起  
4. 原生 App（若有包）→ 仍先 `amihexin://` 再网页  

- [ ] **Step 4: Commit 构建产物（仅当仓库惯例需要提交 web build 时）**

若团队惯例不提交 `build/web`，跳过本步。若需要提交，只 add 实际变更的 web 产物路径后：

```bash
git commit -m "$(cat <<'EOF'
build(web): rebuild for Android Tonghuashun intent launch

EOF
)"
```

---

## Spec Coverage Self-Review

| 规格要求 | 对应 Task |
|---|---|
| Android 手机 Web Intent 优先 | Task 2 |
| 失败 / 未装 → stockpage 网页 | Task 2 |
| 电脑 Web 直接网页 | Task 2 `if (isWeb)` 分支 |
| 原生 App 保持 amihexin → 网页 | Task 2 非 web 分支 |
| 代码只留数字 | Task 1 `normalizeStockCode` |
| 不强制 marketId | Task 2 |
| 只改 StockLauncher / 调用方不改 | File Structure |
| Intent 跳过 canLaunchUrl 误报 | Task 2 `_tryLaunchPreferLaunch` |
| UA / Android 判定可测 | Task 1 `shouldUseAndroidWebIntent` + `isAndroidMobileUserAgent` |
| 手工验收 | Task 3 |

**Placeholder scan:** 无 TBD/TODO。部署命令写明「沿用现有流程」而非空步骤。

**Type consistency:** 全部静态方法挂在 `StockLauncher`；URI builders 均接收已规范化的 `normalizedCode` 字符串。
