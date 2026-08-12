import 'package:flutter/foundation.dart';
import 'package:url_launcher/url_launcher.dart';

class StockLauncher {
  const StockLauncher._();

  static Future<bool> openTongHuaShun({
    required String code,
    String? marketId,
  }) async {
    final normalizedCode = normalizeStockCode(code);
    if (normalizedCode.isEmpty) {
      return false;
    }

    final resolvedMarketId =
        marketId ?? inferTongHuaShunMarketId(normalizedCode);
    final appUri = buildTongHuaShunAppUri(normalizedCode);
    // '&marketId//=$resolvedMarketId',

    if (await _tryLaunch(appUri)) {
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
}
