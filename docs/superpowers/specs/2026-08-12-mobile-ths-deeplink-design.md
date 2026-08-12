# 手机浏览器同花顺 Deep Link 设计

## 目标

用户在手机端浏览器打开 Flutter Web（例如 `http://118.178.19.165:8080/`）并点击同花顺图标时，尽量跳转到本机同花顺 App 的该股票页面；电脑浏览器保持直接打开同花顺网页的现状。

未安装同花顺或唤起失败时，打开网页：`https://stockpage.10jqka.com.cn/{纯数字代码}/`。

主要目标平台为 **Android 手机浏览器**。本设计不包含 iOS Universal Link / 应用商店跳转。

## 背景

现有实现集中在 `trading_app/lib/core/utils/stock_launcher.dart`：

1. 先尝试 `amihexin://...&stockcode//=...` Deep Link；
2. 失败则打开 `https://stockpage.10jqka.com.cn/{code}/`。

雷达列表、详情、系统设置等入口均调用 `StockLauncher.openTongHuaShun`。Android 原生 Manifest 已声明 `amihexin` scheme 与包名 `com.hexin.plat.android`。

在 Flutter Web 的 Android Chrome / 厂商浏览器上，裸自定义 scheme 唤起不稳定；电脑浏览器尝试唤起 App 也无意义。需要按运行环境分流。

## 行为矩阵

| 运行环境 | 行为 |
|---|---|
| Flutter Web · Android 手机浏览器 | ① Android Intent 唤起同花顺到该股票；② 失败 → 网页兜底 |
| Flutter Web · 非 Android（含电脑、iPhone） | 直接打开网页，不尝试唤起 App |
| 原生 App（Android / iOS） | 保持现状：`amihexin://` → 失败再网页 |

## 设备判断（仅 Web）

在 `kIsWeb == true` 时，用 User-Agent 判断是否为 Android 手机（匹配常见 Android 浏览器 / WebView UA）。

- 判定为 Android 手机 → Intent 优先路径
- 其余 Web → 直接网页

不引入复杂设备库；判断逻辑做成纯函数，便于单测。

## URL 格式

股票代码规范化：只保留数字（`002558.XSHE` → `002558`）。空代码直接返回失败。

`inferTongHuaShunMarketId` 保留，但唤起串与现状一致，**不强制附带 marketId**，避免改坏已验证的原生跳转。

### Android 手机 Web（Intent，优先）

将现有 Deep Link 的 path/query 片段嵌入 Intent URL，并指定 scheme 与包名：

```text
intent://command//=XXXX//&action//=GGFS//&stockcode//=002558//&applicationScheme//=XXXX//#Intent;scheme=amihexin;package=com.hexin.plat.android;end
```

`stockcode` 使用规范化后的纯数字代码。Intent 内容与现有 `amihexin://` 参数保持一致。

### 原生 App（Deep Link）

```text
amihexin://command//=XXXX//&action//=GGFS//&stockcode//=002558//&applicationScheme//=XXXX//
```

### 兜底 / 电脑 Web

```text
https://stockpage.10jqka.com.cn/002558/
```

## 流程

```text
openTongHuaShun(code)
  → normalize code（失败则 return false）
  → if kIsWeb && isAndroidMobileUA:
        try launch(Intent URL, externalApplication)
        if 失败 → launch(网页)
     else if kIsWeb:
        launch(网页)
     else:
        try amihexin:// → 失败则网页
```

失败判定：`canLaunchUrl` 为 false，或 `launchUrl` 抛错 / 返回 false。Intent 或 Deep Link 失败时静默改开网页，不新增「未安装同花顺」提示。仅当网页也失败时返回 false，调用方现有 SnackBar 继续生效。

## 代码边界

- 只改 `trading_app/lib/core/utils/stock_launcher.dart`（可同文件增加 UA 判断与 Intent 拼装的小函数）。
- 雷达、详情、设置等调用方不改。
- 不改后端、不改 AndroidManifest、不改 UI。

## 风险与预期

部分国产浏览器可能拦截 Intent。拦截或未安装 App 时落到同花顺网页，仍可用。本设计接受该降级，不保证所有 Android 浏览器 100% 唤起成功。

## 明确不做

- iOS Safari Universal Link / 专项唤起
- 跳转应用商店或下载页
- 修改各列表卡片 UI
- 依赖同花顺 H5 自行唤端

## 验收

1. Android 手机浏览器访问部署的 Web，已装同花顺 → 点击图标进入该股票页。
2. 未装同花顺或浏览器拦截 → 打开 `stockpage.10jqka.com.cn/{代码}/`。
3. 电脑浏览器 → 始终直接开网页，不尝试唤起。
4. 原生 App（若有）→ 与改前行为一致。

可选单测（纯函数）：

- 代码规范化
- `isAndroidMobileUA` 对典型 UA 的正负例
- Intent URL 拼装包含正确 `stockcode`、`scheme=amihexin`、`package=com.hexin.plat.android`
