import 'package:flutter/foundation.dart';
import 'package:url_launcher/url_launcher.dart';

class StockLauncher {
  const StockLauncher._();

  static Future<bool> openTongHuaShun({
    required String code,
    String? marketId,
  }) async {
    final normalizedCode = _normalizeCode(code);
    if (normalizedCode.isEmpty) {
      return false;
    }

    final resolvedMarketId =
        marketId ?? inferTongHuaShunMarketId(normalizedCode);
    final appUri = Uri.parse(
      'amihexin://command//=XXXX//'
      '&action//=GGFS//'
      '&stockcode//=$normalizedCode//'
      '&applicationScheme//=XXXX//',
      // '&marketId//=$resolvedMarketId',
    );

    if (await _tryLaunch(appUri)) {
      return true;
    }

    return _tryLaunch(
      Uri.parse('https://stockpage.10jqka.com.cn/$normalizedCode/'),
    );
  }

  static String inferTongHuaShunMarketId(String code) {
    final normalizedCode = _normalizeCode(code);

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

  static String _normalizeCode(String code) {
    return code.replaceAll(RegExp(r'[^0-9]'), '');
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
