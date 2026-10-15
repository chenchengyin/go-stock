import 'package:flutter/material.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:visibility_detector/visibility_detector.dart';
import 'package:trading_app/features/radar/data/red_t1_intraday_repository.dart';
import 'package:trading_app/features/radar/presentation/radar_list/red_t1_intraday_exposure.dart';

void main() {
  testWidgets('首次分时空响应时会在可见状态下自动重试一次', (tester) async {
    var requestCount = 0;
    final oldUpdateInterval =
        VisibilityDetectorController.instance.updateInterval;
    VisibilityDetectorController.instance.updateInterval = Duration.zero;
    addTearDown(() {
      VisibilityDetectorController.instance.updateInterval = oldUpdateInterval;
    });

    tester.view.physicalSize = const Size(400, 300);
    tester.view.devicePixelRatio = 1;
    addTearDown(tester.view.resetPhysicalSize);
    addTearDown(tester.view.resetDevicePixelRatio);

    await tester.pumpWidget(
      MaterialApp(
        home: RedT1IntradayExposure(
          detectorId: 'stock-retry',
          load: () async {
            requestCount++;
            return RedT1IntradaySeries(
              code: '000823',
              date: '2026-10-09',
              bars: requestCount == 1
                  ? const []
                  : const [
                      RedT1IntradayBar(time: '09:30', price: 10),
                      RedT1IntradayBar(time: '15:00', price: 10.2),
                    ],
              prevClose: 9.8,
              source: 'ths',
            );
          },
          builder: (context, series, loading, error, waiting) => SizedBox(
            height: 240,
            child: Text(series?.bars.isNotEmpty == true ? 'loaded' : 'empty'),
          ),
        ),
      ),
    );

    await tester.pump();
    await tester.pump(const Duration(milliseconds: 300));
    await tester.pump();
    expect(requestCount, 1);

    await tester.pump(const Duration(seconds: 2));
    await tester.pump();
    expect(requestCount, 2);
    expect(find.text('loaded'), findsOneWidget);
  });

  testWidgets('初次显示和滚动到卡片后都会触发分时请求', (tester) async {
    final requested = <String>[];
    final oldUpdateInterval =
        VisibilityDetectorController.instance.updateInterval;
    VisibilityDetectorController.instance.updateInterval = Duration.zero;
    addTearDown(() {
      VisibilityDetectorController.instance.updateInterval = oldUpdateInterval;
    });

    tester.view.physicalSize = const Size(400, 300);
    tester.view.devicePixelRatio = 1;
    addTearDown(tester.view.resetPhysicalSize);
    addTearDown(tester.view.resetDevicePixelRatio);

    await tester.pumpWidget(
      MaterialApp(
        home: ListView.builder(
          itemCount: 3,
          itemBuilder: (context, index) => RedT1IntradayExposure(
            detectorId: 'stock-$index',
            load: () async {
              requested.add('stock-$index');
              return RedT1IntradaySeries(
                code: '$index',
                date: '2026-10-09',
                bars: const [],
                prevClose: 0,
                source: '',
              );
            },
            builder: (context, series, loading, error, waiting) => SizedBox(
              height: 240,
              child: Text(series?.code ?? (waiting ? 'waiting' : 'loading')),
            ),
          ),
        ),
      ),
    );
    await tester.pump();
    await tester.pump(const Duration(milliseconds: 300));
    await tester.pump();
    expect(requested, contains('stock-0'));
    expect(requested, isNot(contains('stock-2')));

    await tester.drag(find.byType(ListView), const Offset(0, -500));
    await tester.pumpAndSettle();
    await tester.pump(const Duration(milliseconds: 300));
    await tester.pump();

    expect(requested, contains('stock-1'));
    expect(requested, contains('stock-2'));
  });
}
