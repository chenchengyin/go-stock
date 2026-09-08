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

  group('shouldTryTongHuaShunMacAppUri', () {
    test('defaults to web on macOS', () {
      expect(StockLauncher.defaultMacAppEnabled, isFalse);
      expect(
        StockLauncher.shouldTryTongHuaShunMacAppUri(
          platform: TargetPlatform.macOS,
          useMacApp: StockLauncher.defaultMacAppEnabled,
        ),
        isFalse,
      );
    });

    test('uses the Mac app only when the preference is enabled', () {
      expect(
        StockLauncher.shouldTryTongHuaShunMacAppUri(
          platform: TargetPlatform.macOS,
          useMacApp: true,
        ),
        isTrue,
      );
      expect(
        StockLauncher.shouldTryTongHuaShunMacAppUri(
          platform: TargetPlatform.windows,
          useMacApp: true,
        ),
        isFalse,
      );
      expect(
        StockLauncher.shouldTryTongHuaShunMacAppUri(
          platform: TargetPlatform.macOS,
          useMacApp: false,
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
    test('mac app uri opens the stock and keeps the market context', () {
      final uri = StockLauncher.buildTongHuaShunAppUri('601318');
      expect(
        uri.toString(),
        'hexinstock://action=jump&target=recently&stockcode=601318&market=17',
      );
    });

    test('app uri infers Shenzhen market for non-SSE stocks', () {
      final uri = StockLauncher.buildTongHuaShunAppUri('002558');
      expect(
        uri.toString(),
        'hexinstock://action=jump&target=recently&stockcode=002558&market=33',
      );
    });

    test('app uri accepts an explicit market override', () {
      final uri = StockLauncher.buildTongHuaShunAppUri(
        '601318',
        marketId: 'custom-market',
      );
      expect(
        uri.toString(),
        'hexinstock://action=jump&target=recently&stockcode=601318&market=custom-market',
      );
    });

    test('intent uri embeds scheme package stockcode and browser fallback', () {
      final uri = StockLauncher.buildTongHuaShunIntentUri('002558');
      final s = uri.toString();
      expect(s.startsWith('intent://'), isTrue);
      expect(s, contains('stockcode//=002558//'));
      expect(s, contains('scheme=amihexin'));
      expect(s, contains('package=com.hexin.plat.android'));
      expect(s, contains('S.browser_fallback_url='));
      expect(
        s,
        contains(
          Uri.encodeComponent('https://stockpage.10jqka.com.cn/002558/'),
        ),
      );
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
