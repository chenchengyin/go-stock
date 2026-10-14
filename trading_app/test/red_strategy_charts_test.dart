import 'dart:async';

import 'package:fl_chart/fl_chart.dart';
import 'package:flutter/material.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:trading_app/features/radar/data/red_kline_repository.dart';
import 'package:trading_app/features/radar/data/red_t1_intraday_repository.dart';
import 'package:trading_app/features/radar/presentation/radar_list/red_strategy_charts.dart';

RedKlineWindow _window(String anchor, List<String> dates) =>
    RedKlineWindow.fromJson({
      'code': '000823.XSHE',
      'date': anchor,
      'anchor_index': dates.indexOf(anchor),
      'before_count': dates.indexOf(anchor),
      'after_count': dates.length - dates.indexOf(anchor) - 1,
      'source': 'fixture',
      'status': 'ok',
      'partial': true,
      'bars': [
        for (var i = 0; i < dates.length; i++)
          {
            'date': dates[i],
            'open': 10.0 + i,
            'high': 12.0 + i,
            'low': 9.0 + i,
            'close': 11.0 + i,
          },
      ],
    });

RedT1IntradaySeries _series(String date, String? previous, String? next) =>
    RedT1IntradaySeries(
      code: '000823.XSHE',
      date: date,
      prevClose: 10,
      source: 'fixture',
      previousDate: previous,
      nextDate: next,
      bars: const [
        RedT1IntradayBar(time: '09:30', price: 10),
        RedT1IntradayBar(time: '15:00', price: 10.2),
      ],
    );

void main() {
  testWidgets('T+1分时异步加载后默认显示同日K线详情，长按松手恢复', (tester) async {
    final window = _window('2026-09-16', [
      '2026-09-15',
      '2026-09-16',
      '2026-09-17',
    ]);
    Widget page(RedT1IntradaySeries? series) => MaterialApp(
      home: Scaffold(
        body: SizedBox(
          width: 390,
          child: RedStrategyCharts(
            code: window.code,
            t0Date: window.date,
            showKline: true,
            window: window,
            showIntraday: true,
            series: series,
          ),
        ),
      ),
    );
    await tester.pumpWidget(page(null));
    final chartFinder = find.byType(CandlestickChart);
    CandlestickChart chart() => tester.widget(chartFinder);
    expect(chart().data.touchedPointIndicator, isNull);

    await tester.pumpWidget(page(_series('2026-09-17', '2026-09-16', null)));
    expect(chart().data.touchedPointIndicator?.x, 2);
    expect(find.textContaining('2026-09-17  开 12.00 (0.00%)'), findsOneWidget);
    expect(find.textContaining('收 13.00 (+8.33%)'), findsOneWidget);
    expect(find.textContaining('高 14.00 (+16.67%)'), findsOneWidget);
    expect(find.textContaining('低 11.00 (-8.33%)'), findsOneWidget);

    final rect = tester.getRect(chartFinder);
    final gesture = await tester.startGesture(
      Offset(rect.left + rect.width / 6, rect.center.dy),
    );
    await tester.pump(const Duration(milliseconds: 600));
    await tester.pump();
    expect(chart().data.touchedPointIndicator?.x, 0);
    expect(find.textContaining('2026-09-15  开 10.00 (--%)'), findsOneWidget);
    await gesture.up();
    await tester.pump();
    expect(chart().data.touchedPointIndicator?.x, 2);
    expect(find.textContaining('2026-09-17  开 12.00 (0.00%)'), findsOneWidget);
  });

  testWidgets('切换前后分时日期会持续选中同日K线，返回也同步', (tester) async {
    final dates = ['2026-09-15', '2026-09-16', '2026-09-17', '2026-09-18'];
    final window = _window('2026-09-16', dates);
    final series = _series('2026-09-17', '2026-09-16', '2026-09-18');
    await tester.pumpWidget(
      MaterialApp(
        home: Scaffold(
          body: SizedBox(
            width: 390,
            child: RedStrategyCharts(
              code: window.code,
              t0Date: window.date,
              showKline: true,
              window: window,
              showIntraday: true,
              series: series,
              loadSession: (date) async => _series(
                date,
                dates[dates.indexOf(date) - 1],
                dates.indexOf(date) == dates.length - 1
                    ? null
                    : dates[dates.indexOf(date) + 1],
              ),
            ),
          ),
        ),
      ),
    );
    CandlestickChart chart() => tester.widget(find.byType(CandlestickChart));
    expect(chart().data.touchedPointIndicator?.x, 2);
    expect(chart().data.touchedPointIndicator?.y, 13);
    expect(find.textContaining('2026-09-17  开 12.00 (0.00%)'), findsOneWidget);
    await tester.tap(find.byTooltip('后一交易日分时'));
    await tester.pumpAndSettle();
    expect(chart().data.touchedPointIndicator?.x, 3);
    expect(chart().data.touchedPointIndicator?.y, 14);
    expect(find.textContaining('2026-09-18  开 13.00 (0.00%)'), findsOneWidget);
    await tester.tap(find.byTooltip('前一交易日分时'));
    await tester.pumpAndSettle();
    expect(chart().data.touchedPointIndicator?.x, 2);
    expect(find.textContaining('2026-09-17  开 12.00 (0.00%)'), findsOneWidget);
    await tester.tap(find.byTooltip('前一交易日分时'));
    await tester.pumpAndSettle();
    expect(chart().data.touchedPointIndicator?.x, 1);
    expect(find.textContaining('2026-09-16  开 11.00 (0.00%)'), findsOneWidget);
  });

  testWidgets('切到窗口以外日期按需加载该日K线，缺失时不冒用邻近K线', (tester) async {
    final window = _window('2026-09-16', [
      '2026-09-15',
      '2026-09-16',
      '2026-09-17',
    ]);
    final requested = <String>[];
    await tester.pumpWidget(
      MaterialApp(
        home: Scaffold(
          body: SizedBox(
            width: 390,
            child: RedStrategyCharts(
              code: window.code,
              t0Date: window.date,
              showKline: true,
              window: window,
              showIntraday: true,
              series: _series('2026-09-17', '2026-09-16', '2026-09-18'),
              loadSession: (date) async => _series(
                date,
                '2026-09-17',
                date == '2026-09-18' ? '2026-09-19' : null,
              ),
              loadKlineWindow: (date) async {
                requested.add(date);
                return date == '2026-09-18'
                    ? _window(date, ['2026-09-16', '2026-09-17', '2026-09-18'])
                    : null;
              },
            ),
          ),
        ),
      ),
    );
    await tester.tap(find.byTooltip('后一交易日分时'));
    await tester.pumpAndSettle();
    expect(requested, ['2026-09-18']);
    expect(
      tester
          .widget<CandlestickChart>(find.byType(CandlestickChart))
          .data
          .touchedPointIndicator
          ?.x,
      2,
    );
    expect(find.textContaining('2026-09-18  开 12.00 (0.00%)'), findsOneWidget);
    await tester.tap(find.byTooltip('后一交易日分时'));
    await tester.pumpAndSettle();
    expect(requested, ['2026-09-18', '2026-09-19']);
    expect(
      tester
          .widget<CandlestickChart>(find.byType(CandlestickChart))
          .data
          .touchedPointIndicator,
      isNull,
    );
    expect(find.textContaining('2026-09-19 日K线暂无可用数据'), findsOneWidget);
  });

  testWidgets('切换股票或红策日期后忽略过期的日K请求', (tester) async {
    final pending = Completer<RedKlineWindow?>();
    final original = _window('2026-09-16', [
      '2026-09-15',
      '2026-09-16',
      '2026-09-17',
    ]);
    final replacement = _window('2026-09-20', ['2026-09-19', '2026-09-20']);
    Widget page(RedKlineWindow window, RedT1IntradaySeries series) =>
        MaterialApp(
          home: Scaffold(
            body: SizedBox(
              width: 390,
              child: RedStrategyCharts(
                code: window.code,
                t0Date: window.date,
                showKline: true,
                window: window,
                showIntraday: true,
                series: series,
                loadSession: (date) async => _series(date, '2026-09-17', null),
                loadKlineWindow: (date) => pending.future,
              ),
            ),
          ),
        );
    await tester.pumpWidget(
      page(original, _series('2026-09-17', '2026-09-16', '2026-09-18')),
    );
    await tester.tap(find.byTooltip('后一交易日分时'));
    await tester.pump();
    await tester.pumpWidget(
      page(replacement, _series('2026-09-21', '2026-09-20', null)),
    );
    pending.complete(_window('2026-09-18', ['2026-09-17', '2026-09-18']));
    await tester.pump();
    expect(
      tester
          .widget<CandlestickChart>(find.byType(CandlestickChart))
          .data
          .touchedPointIndicator,
      isNull,
    );
    expect(find.textContaining('2026-09-18  开'), findsNothing);
  });
}
