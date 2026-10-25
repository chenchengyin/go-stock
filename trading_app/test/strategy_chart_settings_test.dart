import 'package:flutter_test/flutter_test.dart';
import 'package:flutter/cupertino.dart';
import 'package:provider/provider.dart';
import 'package:trading_app/core/theme/theme_manager.dart';
import 'package:trading_app/features/auth/data/auth_repository.dart';
import 'package:trading_app/features/auth/presentation/auth_view_model.dart';
import 'package:trading_app/features/profile/presentation/system_settings_page.dart';
import 'package:shared_preferences/shared_preferences.dart';
import 'package:trading_app/core/storage/red_kline_settings.dart';

void main() {
  testWidgets('settings page exposes independent mainboard panels', (
    tester,
  ) async {
    SharedPreferences.setMockInitialValues({});
    final settings = StrategyChartSettings();
    await settings.restore();
    final auth = AuthViewModel(_UnusedAuthRepository());
    final theme = ThemeManager();
    addTearDown(() {
      settings.dispose();
      auth.dispose();
      theme.dispose();
    });
    await tester.pumpWidget(
      MultiProvider(
        providers: [
          ChangeNotifierProvider<RedKlineSettings>.value(value: settings),
          ChangeNotifierProvider<AuthViewModel>.value(value: auth),
          ChangeNotifierProvider<ThemeManager>.value(value: theme),
        ],
        child: const CupertinoApp(home: SystemSettingsPage()),
      ),
    );
    await tester.ensureVisible(find.text('主板K线图'));
    await tester.tap(find.text('主板K线图'));
    await tester.pumpAndSettle();
    expect(settings.enabledFor('radar.main_strategy'), isFalse);
    expect(settings.enabled, isTrue);
    await tester.ensureVisible(find.text('主板T+1分时图'));
    await tester.tap(find.text('主板T+1分时图'));
    await tester.pumpAndSettle();
    expect(settings.intradayEnabledFor('radar.main_strategy'), isFalse);
    expect(settings.t1IntradayEnabled, isTrue);
    await tester.pumpWidget(const SizedBox.shrink());
  });

  test(
    'mainboard chart switches persist independently and preserve red preferences',
    () async {
      SharedPreferences.setMockInitialValues({
        'show_red_strategy_kline': false,
        'show_red_strategy_t1_intraday': true,
      });
      final settings = StrategyChartSettings();
      await settings.restore();
      expect(settings.enabledFor('radar.main_strategy'), isTrue);
      expect(settings.intradayEnabledFor('radar.main_strategy'), isTrue);
      expect(settings.enabled, isFalse);
      expect(settings.enabledFor('radar.red_shared_gob_strategy'), isFalse);
      await settings.setEnabledFor('radar.main_strategy', false);
      await settings.setIntradayEnabledFor('radar.main_strategy', false);
      expect(settings.t1IntradayEnabled, isTrue);
      final restored = RedKlineSettings();
      await restored.restore();
      expect(restored.enabledFor('radar.main_strategy'), isFalse);
      expect(restored.intradayEnabledFor('radar.main_strategy'), isFalse);
      expect(restored.enabled, isFalse);
      expect(restored.t1IntradayEnabled, isTrue);
      await restored.setEnabled(true);
      expect(restored.enabledFor('radar.red_shared_gob_strategy'), isTrue);
      expect(restored.enabledFor('radar.main_strategy'), isFalse);
      settings.dispose();
      restored.dispose();
    },
  );
}

class _UnusedAuthRepository implements AuthRepository {
  @override
  dynamic noSuchMethod(Invocation invocation) => super.noSuchMethod(invocation);
}
