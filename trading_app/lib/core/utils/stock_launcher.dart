import 'package:flutter/foundation.dart';
import 'package:shared_preferences/shared_preferences.dart';
import 'package:url_launcher/url_launcher.dart';

class StockLauncher {
  const StockLauncher._();

  static const String macAppEnabledPreferenceKey =
      'stock_launcher_mac_app_enabled';
  static const bool defaultMacAppEnabled = false;

  static bool _macAppEnabled = defaultMacAppEnabled;
  static Future<void>? _initializeFuture;

  static bool get macAppEnabled => _macAppEnabled;

  /// Loads the Mac launch preference before the app becomes interactive.
  static Future<void> initialize() {
    return _initializeFuture ??= _loadPreference();
  }

  static Future<void> _loadPreference() async {
    try {
      final prefs = await SharedPreferences.getInstance();
      _macAppEnabled =
          prefs.getBool(macAppEnabledPreferenceKey) ?? defaultMacAppEnabled;
    } catch (error, stackTrace) {
      debugPrint('StockLauncher preference load failed: $error\n$stackTrace');
    }
  }

  static Future<bool> setMacAppEnabled(bool enabled) async {
    await initialize();
    final previous = _macAppEnabled;
    _macAppEnabled = enabled;

    try {
      final prefs = await SharedPreferences.getInstance();
      final saved = await prefs.setBool(macAppEnabledPreferenceKey, enabled);
      if (!saved) {
        _macAppEnabled = previous;
      }
      return saved;
    } catch (error, stackTrace) {
      _macAppEnabled = previous;
      debugPrint('StockLauncher preference save failed: $error\n$stackTrace');
      return false;
    }
  }

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

    final resolvedMarketId =
        marketId ?? inferTongHuaShunMarketId(normalizedCode);

    final isWeb = isWebOverride ?? kIsWeb;
    final platform = platformOverride ?? defaultTargetPlatform;
    final useMacApp = shouldTryTongHuaShunMacAppUri(
      platform: platform,
      useMacApp: _macAppEnabled,
    );

    if (shouldUseAndroidWebIntent(isWeb: isWeb, platform: platform)) {
      if (await _tryLaunchPreferLaunch(
        buildTongHuaShunIntentUri(normalizedCode),
      )) {
        return true;
      }
      return _tryLaunch(buildTongHuaShunWebUri(normalizedCode));
    }

    if (platform == TargetPlatform.macOS && !useMacApp) {
      return _tryLaunch(buildTongHuaShunWebUri(normalizedCode));
    }

    if (isWeb && useMacApp) {
      if (await _tryLaunchPreferLaunch(
        buildTongHuaShunAppUri(normalizedCode, marketId: resolvedMarketId),
      )) {
        return true;
      }
    }

    if (isWeb) {
      return _tryLaunch(buildTongHuaShunWebUri(normalizedCode));
    }

    if (await _tryLaunch(
      buildTongHuaShunAppUri(normalizedCode, marketId: resolvedMarketId),
    )) {
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

  static bool shouldTryTongHuaShunMacAppUri({
    required TargetPlatform platform,
    required bool useMacApp,
  }) {
    return useMacApp && platform == TargetPlatform.macOS;
  }

  static bool isAndroidMobileUserAgent(String userAgent) {
    return userAgent.toLowerCase().contains('android');
  }

  /// Opens a stock in the macOS Tonghuashun app.
  ///
  /// Tonghuashun keeps the last-used chart period, so this intentionally only
  /// navigates to the stock and does not send an undocumented page parameter.
  static Uri buildTongHuaShunAppUri(String normalizedCode, {String? marketId}) {
    final resolvedMarketId =
        marketId ?? inferTongHuaShunMarketId(normalizedCode);
    return Uri.parse(
      'hexinstock://action=jump'
      '&target=recently'
      '&stockcode=$normalizedCode'
      '&market=$resolvedMarketId',
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
        webOnlyWindowName: kIsWeb ? '_blank' : null,
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
      return await launchUrl(
        uri,
        mode: LaunchMode.externalApplication,
        webOnlyWindowName: kIsWeb ? '_blank' : null,
      );
    } catch (error) {
      debugPrint('Launch stock url failed: $uri, $error');
      return false;
    }
  }
}
