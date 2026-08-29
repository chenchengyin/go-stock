import 'package:flutter/material.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:trading_app/core/theme/app_colors.dart';
import 'package:trading_app/main.dart';

void main() {
  TestWidgetsFlutterBinding.ensureInitialized();

  test('grey variant wraps the app in ColorFiltered', () {
    final wrapped = TradingRadarApp.wrapWithGreyFilter(
      AppThemeVariant.grey,
      const SizedBox(),
    );

    expect(wrapped, isA<ColorFiltered>());
  });

  test('light and dark variants do not use the grey filter', () {
    const child = SizedBox();

    expect(
      TradingRadarApp.wrapWithGreyFilter(AppThemeVariant.light, child),
      same(child),
    );
    expect(
      TradingRadarApp.wrapWithGreyFilter(AppThemeVariant.dark, child),
      same(child),
    );
  });
}
