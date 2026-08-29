import 'package:flutter/material.dart';
import 'package:flutter_test/flutter_test.dart';

import 'package:trading_app/main.dart';

void main() {
  testWidgets('App should display bottom navigation', (WidgetTester tester) async {
    await tester.pumpWidget(const TradingRadarApp());
    await tester.pump(const Duration(milliseconds: 500));

    // 验证底部导航存在
    expect(find.byType(BottomNavigationBar), findsOneWidget);
  });
}
