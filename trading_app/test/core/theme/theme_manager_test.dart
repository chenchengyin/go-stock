import 'package:flutter/material.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:shared_preferences/shared_preferences.dart';
import 'package:trading_app/core/theme/app_colors.dart';
import 'package:trading_app/core/theme/theme_manager.dart';

void main() {
  TestWidgetsFlutterBinding.ensureInitialized();

  setUp(() {
    SharedPreferences.setMockInitialValues({});
    AppColors.applyLight();
  });

  group('parseStored', () {
    test('null and unknown values fall back to light', () {
      expect(ThemeManager.parseStored(null), AppThemeVariant.light);
      expect(ThemeManager.parseStored(''), AppThemeVariant.light);
      expect(ThemeManager.parseStored('midnight'), AppThemeVariant.light);
    });

    test('parses all supported variants', () {
      expect(ThemeManager.parseStored('light'), AppThemeVariant.light);
      expect(ThemeManager.parseStored('dark'), AppThemeVariant.dark);
      expect(ThemeManager.parseStored('grey'), AppThemeVariant.grey);
    });
  });

  test('grey filter is enabled only for grey variant', () {
    expect(ThemeManager.shouldApplyGreyFilter(AppThemeVariant.grey), isTrue);
    expect(ThemeManager.shouldApplyGreyFilter(AppThemeVariant.dark), isFalse);
    expect(ThemeManager.shouldApplyGreyFilter(AppThemeVariant.light), isFalse);
  });

  test('setVariant persists and restore reads the stored variant', () async {
    final tm = ThemeManager();
    tm.setVariant(AppThemeVariant.dark);
    await Future<void>.delayed(Duration.zero);

    final prefs = await SharedPreferences.getInstance();
    expect(prefs.getString(ThemeManager.prefsKey), 'dark');

    final restored = ThemeManager();
    await restored.restore();
    expect(restored.variant, AppThemeVariant.dark);
  });

  test('restore unknown value falls back to light', () async {
    SharedPreferences.setMockInitialValues({ThemeManager.prefsKey: 'nope'});

    final tm = ThemeManager();
    await tm.restore();

    expect(tm.variant, AppThemeVariant.light);
  });

  test('dark theme data uses dark brightness', () {
    final tm = ThemeManager();
    tm.setVariant(AppThemeVariant.dark);

    expect(tm.themeData.brightness, Brightness.dark);
    expect(tm.themeData.colorScheme.brightness, Brightness.dark);
  });

  test('grey theme data keeps the light palette and brightness', () {
    final tm = ThemeManager();
    tm.setVariant(AppThemeVariant.grey);

    expect(tm.themeData.brightness, Brightness.light);
    expect(AppColors.cardBg, Colors.white);
    expect(AppColors.scaffoldBg, const Color(0xfff5f5f5));
  });
}
