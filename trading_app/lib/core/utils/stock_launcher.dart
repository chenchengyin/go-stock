import 'package:flutter/foundation.dart';
import 'package:url_launcher/url_launcher.dart';

class StockLauncher {
  const StockLauncher._();

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
      if (await _tryLaunchPreferLaunch(
        buildTongHuaShunIntentUri(normalizedCode),
      )) {
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

  static String inferTongHuaShunMarketId(String code) {
    final normalizedCode = normalizeStockCode(code);

    if (normalizedCode.startsWith('688') || normalizedCode.startsWith('689')) {
      return '20';
    }
    if (normalizedCode.startsWith('6')) {
      return '17';
    }
    if (normalizedCode.startsWith('30')) {
      return '36';
    }
    if (normalizedCode.startsWith('8') ||
        normalizedCode.startsWith('4') ||
        normalizedCode.startsWith('9')) {
      return '151';
    }
    return '33';
  }

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
    final fallbackUrl = Uri.encodeComponent(
      buildTongHuaShunWebUri(normalizedCode).toString(),
    );
    return Uri.parse(
      'intent://command//=XXXX//'
      '&action//=GGFS//'
      '&stockcode//=$normalizedCode//'
      '&applicationScheme//=XXXX//'
      '#Intent;scheme=amihexin;package=com.hexin.plat.android;'
      'S.browser_fallback_url=$fallbackUrl;end',
    );
  }

  static Uri buildTongHuaShunWebUri(String normalizedCode) {
    return Uri.parse('https://stockpage.10jqka.com.cn/$normalizedCode/');
  }

  /// Web Intent：跳过 canLaunchUrl（Flutter Web 对 intent:// 常误报不可用）
  static Future<bool> _tryLaunchPreferLaunch(Uri uri) async {
    try {
      return await launchUrl(
        uri,
        mode: LaunchMode.externalApplication,
        webOnlyWindowName: kIsWeb ? '_self' : null,
      );
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
      return launchUrl(
        uri,
        mode: LaunchMode.externalApplication,
        webOnlyWindowName: kIsWeb ? '_self' : null,
      );
    } catch (error) {
      debugPrint('Launch stock url failed: $uri, $error');
      return false;
    }
  }
}
