import 'package:flutter/material.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:provider/provider.dart';
import 'package:shared_preferences/shared_preferences.dart';
import 'package:trading_app/features/radar/presentation/radar_list/red_empty_reminders.dart';
import 'package:trading_app/features/radar/presentation/radar_list/t0_strategy_view_model.dart';

String visibleReminder(WidgetTester tester) => tester
    .widgetList<Text>(find.byType(Text))
    .map((text) => text.data)
    .whereType<String>()
    .singleWhere(redEmptyReminders.contains);

void main() {
  setUp(() => SharedPreferences.setMockInitialValues({}));

  for (final noData in [true, false]) {
    testWidgets('空结果每次加载都换句且不随普通通知变化 noData=$noData', (tester) async {
      final vm = T0StrategyViewModel(
        request: (_, query) async => query['list_dates'] == '1'
            ? {'dates': <String>[]}
            : {'date': '2026-10-26', 'no_data': noData, 'results': <dynamic>[]},
      );
      var disposed = false;
      addTearDown(() {
        if (!disposed) vm.dispose();
      });
      await tester.pumpWidget(
        ChangeNotifierProvider<T0StrategyViewModel>.value(
          value: vm,
          child: MaterialApp(
            home: Consumer<T0StrategyViewModel>(
              builder: (_, model, _) => RedEmptyReminder(
                revision: model.resultsRevisionFor(
                  t0SharedGobRedStrategyModuleCode,
                ),
              ),
            ),
          ),
        ),
      );
      await tester.pumpAndSettle();
      var previous = visibleReminder(tester);
      vm.applyAvailableDatesForTest([
        '2026-10-26',
      ], moduleCode: t0SharedGobRedStrategyModuleCode);
      await tester.pumpAndSettle();
      expect(visibleReminder(tester), previous);

      for (var i = 0; i < 10; i++) {
        await vm.loadResults(moduleCode: t0SharedGobRedStrategyModuleCode);
        await tester.pumpAndSettle();
        final next = visibleReminder(tester);
        expect(next, isNot(previous));
        previous = next;
      }
      expect(tester.takeException(), isNull);
      await tester.pumpWidget(const SizedBox.shrink());
      vm.dispose();
      disposed = true;
    });
  }

  testWidgets('重新打开页面从本地记录排除上句', (tester) async {
    String? previous;
    for (var i = 0; i < 10; i++) {
      await tester.pumpWidget(
        MaterialApp(home: RedEmptyReminder(key: ValueKey(i), revision: 0)),
      );
      await tester.pumpAndSettle();
      final next = visibleReminder(tester);
      expect(next, isNot(previous));
      previous = next;
    }
    expect(tester.takeException(), isNull);
  });
}
