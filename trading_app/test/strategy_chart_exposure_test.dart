import 'dart:async';
import 'package:flutter/material.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:visibility_detector/visibility_detector.dart';
import 'package:trading_app/features/radar/presentation/radar_list/strategy_chart_exposure.dart';

void main() {
  testWidgets(
    'a failed stale request cannot schedule retries for a replacement stock',
    (tester) async {
      final previousInterval =
          VisibilityDetectorController.instance.updateInterval;
      VisibilityDetectorController.instance.updateInterval = Duration.zero;
      final pending = Completer<int>();
      var secondLoads = 0;
      Widget page(String id) => MaterialApp(
        home: ListView(
          children: [
            StrategyChartExposure<int>(
              detectorId: id,
              load: () =>
                  id == 'old' ? pending.future : Future.value(++secondLoads),
              builder: (_, value, loading, error, waiting) =>
                  SizedBox(height: 200, child: Text('$id:$value')),
            ),
          ],
        ),
      );
      await tester.pumpWidget(page('old'));
      await tester.pump();
      await tester.pump(const Duration(milliseconds: 350));
      await tester.pumpWidget(page('new'));
      await tester.pump();
      await tester.pump(const Duration(milliseconds: 350));
      await tester.pumpAndSettle();
      expect(find.text('new:1'), findsOneWidget);
      pending.completeError(StateError('old failed'));
      await tester.pump();
      await tester.pump(const Duration(seconds: 2));
      await tester.pumpAndSettle();
      expect(secondLoads, 1);
      expect(find.text('new:1'), findsOneWidget);
      await tester.pumpWidget(const SizedBox.shrink());
      VisibilityDetectorController.instance.updateInterval = previousInterval;
    },
  );
}
