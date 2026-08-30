import 'package:flutter/material.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:shared_preferences/shared_preferences.dart';

import 'package:trading_app/features/auth/presentation/login_page.dart';
import 'package:trading_app/main.dart';

void main() {
  testWidgets('App gates business navigation until authentication', (
    WidgetTester tester,
  ) async {
    SharedPreferences.setMockInitialValues(<String, Object>{});

    await tester.pumpWidget(const TradingRadarApp());
    await tester.pump(const Duration(milliseconds: 500));

    expect(find.byType(LoginPage), findsOneWidget);
    expect(find.byType(BottomNavigationBar), findsNothing);
  });
}
