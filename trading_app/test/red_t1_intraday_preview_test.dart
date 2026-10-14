import 'package:fl_chart/fl_chart.dart';
import 'package:flutter/material.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:trading_app/features/radar/data/red_t1_intraday_repository.dart';
import 'package:trading_app/features/radar/presentation/radar_list/red_t1_intraday_preview.dart';

void main() {
  testWidgets('T+1分时图仅在左轴按2%显示涨跌幅，长按涨跌幅在时间左侧', (tester) async {
    const series = RedT1IntradaySeries(
      code: '000823.XSHE',
      date: '2026-09-17',
      prevClose: 10,
      source: 'fixture',
      bars: [
        RedT1IntradayBar(time: '09:30', price: 10),
        RedT1IntradayBar(time: '10:30', price: 10.3, averagePrice: 10.1),
        RedT1IntradayBar(time: '11:30', price: 9.7),
        RedT1IntradayBar(time: '15:00', price: 10),
      ],
    );
    await tester.pumpWidget(
      const MaterialApp(
        home: Scaffold(
          body: SizedBox(
            width: 390,
            child: RedT1IntradayPreview(series: series),
          ),
        ),
      ),
    );
    LineChart chart() => tester.widget<LineChart>(find.byType(LineChart));
    expect(chart().data.minY, -4);
    expect(chart().data.maxY, 4);
    expect(chart().data.gridData.horizontalInterval, 2);
    expect(chart().data.titlesData.leftTitles.sideTitles.interval, 2);
    expect(chart().data.titlesData.rightTitles.sideTitles.showTitles, isFalse);
    expect(find.text('+2.0%'), findsOneWidget);
    expect(find.text('-2.0%'), findsOneWidget);
    expect(
      tester.getTopLeft(find.text('+2.0%')).dx,
      lessThan(tester.getCenter(find.byType(LineChart)).dx),
    );
    expect(find.text('价格'), findsNothing);
    expect(find.text('涨跌幅'), findsOneWidget);
    expect(find.textContaining('昨收'), findsNothing);
    expect(find.textContaining('均价 10.10'), findsNothing);
    expect(find.textContaining('10.30'), findsNothing);
    expect(find.text('10.00'), findsNothing);
    expect(find.text('9.70'), findsNothing);

    final rect = tester.getRect(find.byType(LineChart));
    final gesture = await tester.startGesture(
      Offset(rect.left + 48 + (rect.width - 48) * .25, rect.center.dy),
    );
    await tester.pump(const Duration(milliseconds: 600));
    await tester.pump();
    Finder selectedTime(String value) => find.byWidgetPredicate(
      (widget) =>
          widget is Text &&
          widget.data == value &&
          widget.style?.fontSize == 10,
    );
    final time = selectedTime('10:30');
    expect(time, findsOneWidget);
    final selectedChange = find.text('+3.0%');
    expect(selectedChange, findsOneWidget);
    expect(
      tester.getTopLeft(selectedChange).dx,
      lessThan(tester.getTopLeft(time).dx),
    );
    await gesture.moveTo(
      Offset(rect.left + 48 + (rect.width - 48) * .5, rect.center.dy),
    );
    await tester.pump();
    expect(find.text('-3.0%'), findsOneWidget);
    expect(selectedTime('11:30'), findsOneWidget);
    await gesture.up();
    await tester.pump();
    expect(selectedTime('11:30'), findsNothing);
  });

  testWidgets('T+1大幅波动时扩大图高以容纳全部2%刻度', (tester) async {
    const series = RedT1IntradaySeries(
      code: '000823.XSHE',
      date: '2026-09-17',
      prevClose: 10,
      source: 'fixture',
      bars: [
        RedT1IntradayBar(time: '09:30', price: 10),
        RedT1IntradayBar(time: '15:00', price: 11),
      ],
    );
    await tester.pumpWidget(
      const MaterialApp(
        home: Scaffold(
          body: SizedBox(
            width: 390,
            child: RedT1IntradayPreview(series: series),
          ),
        ),
      ),
    );
    final chart = tester.widget<LineChart>(find.byType(LineChart));
    expect(chart.data.minY, -12);
    expect(chart.data.maxY, 12);
    expect(
      tester.getSize(find.byType(LineChart)).height,
      greaterThanOrEqualTo(180),
    );
  });

  testWidgets('分时X轴每半小时标出时间且午休交界同时标出11:30与13:00', (tester) async {
    const series = RedT1IntradaySeries(
      code: '000823.XSHE',
      date: '2026-10-09',
      prevClose: 10,
      source: 'fixture',
      bars: [
        RedT1IntradayBar(time: '09:30', price: 10),
        RedT1IntradayBar(time: '15:00', price: 10.2),
      ],
    );
    await tester.pumpWidget(
      const MaterialApp(
        home: Scaffold(
          body: SizedBox(
            width: 320,
            child: RedT1IntradayPreview(series: series),
          ),
        ),
      ),
    );
    final chart = tester.widget<LineChart>(find.byType(LineChart));
    expect(chart.data.titlesData.bottomTitles.sideTitles.interval, 30);
    expect(chart.data.gridData.verticalInterval, 30);
    for (final time in [
      '09:30',
      '10:00',
      '10:30',
      '11:00',
      '11:30',
      '13:00',
      '13:30',
      '14:00',
      '14:30',
      '15:00',
    ]) {
      expect(find.text(time), findsWidgets, reason: '缺失半小时刻度 $time');
    }
    final labels = [
      '09:30',
      '10:00',
      '10:30',
      '11:00',
      '11:30',
      '13:30',
      '14:00',
      '14:30',
      '15:00',
    ];
    Rect axisRect(String time) => tester.getRect(
      find.byWidgetPredicate(
        (widget) =>
            widget is Text &&
            widget.data == time &&
            widget.style?.fontSize == 8,
      ),
    );
    for (var i = 1; i < labels.length; i++) {
      expect(
        axisRect(labels[i - 1]).overlaps(axisRect(labels[i])),
        isFalse,
        reason: '窄屏时间刻度互相遮挡：${labels[i - 1]} 和 ${labels[i]}',
      );
    }
    expect(axisRect('13:00').overlaps(axisRect('11:00')), isFalse);
    expect(axisRect('13:00').overlaps(axisRect('13:30')), isFalse);
    expect(tester.takeException(), isNull);
  });

  testWidgets('分时图宽度为原来的7/8，左右按钮按交易日切换并更新昨收', (tester) async {
    RedT1IntradaySeries day(
      String date,
      String? previous,
      String? next,
      double close,
    ) => RedT1IntradaySeries(
      code: '000823.XSHE',
      date: date,
      prevClose: close,
      previousDate: previous,
      nextDate: next,
      source: 'fixture',
      bars: const [
        RedT1IntradayBar(time: '09:30', price: 10),
        RedT1IntradayBar(time: '15:00', price: 10.3),
      ],
    );
    final base = day('2026-10-09', '2026-10-08', '2026-10-12', 10);
    final dates = <String>[];
    await tester.pumpWidget(
      MaterialApp(
        home: Scaffold(
          body: SizedBox(
            width: 400,
            child: RedT1IntradayPreview(
              series: base,
              loadSession: (date) async {
                dates.add(date);
                return switch (date) {
                  '2026-10-08' => day(date, '2026-09-30', '2026-10-09', 9),
                  '2026-10-09' => base,
                  _ => day(date, '2026-10-09', null, 11),
                };
              },
            ),
          ),
        ),
      ),
    );
    expect(tester.getSize(find.byType(LineChart)).width, closeTo(350, .01));
    for (final icon in [Icons.chevron_left, Icons.chevron_right]) {
      final bounds = tester.getRect(find.widgetWithIcon(IconButton, icon));
      expect(bounds.width, closeTo(25, .01));
      expect(
        bounds.height,
        closeTo(bounds.width, .01),
        reason: '分时切日按钮热区应与当前宽度一致，呈正方形',
      );
    }
    final prevRect = tester.getRect(
      find.widgetWithIcon(IconButton, Icons.chevron_left),
    );
    await tester.tapAt(Offset(prevRect.center.dx, prevRect.bottom + 3));
    await tester.pump();
    expect(dates, isEmpty, reason: '正方形外不应触发切日');
    await tester.tap(find.byTooltip('前一交易日分时'));
    await tester.pumpAndSettle();
    expect(dates, ['2026-10-08']);
    expect(find.text('分时 · 2026-10-08'), findsOneWidget);
    expect(find.text('+14.4%'), findsOneWidget);
    expect(find.textContaining('昨收'), findsNothing);
    await tester.tap(find.byTooltip('后一交易日分时'));
    await tester.pumpAndSettle();
    await tester.tap(find.byTooltip('后一交易日分时'));
    await tester.pumpAndSettle();
    expect(dates, ['2026-10-08', '2026-10-09', '2026-10-12']);
    expect(find.text('分时 · 2026-10-12'), findsOneWidget);
    expect(find.text('-6.4%'), findsOneWidget);
    expect(
      tester
          .widget<IconButton>(
            find.widgetWithIcon(IconButton, Icons.chevron_right),
          )
          .onPressed,
      isNull,
    );
    await tester.tap(find.byTooltip('前一交易日分时'));
    await tester.pumpAndSettle();
    expect(dates.last, '2026-10-09');
    expect(dates, isNot(contains('2026-10-10')));
    expect(dates, isNot(contains('2026-10-11')));
  });

  testWidgets('切到无数据交易日不沿用旧分时价格且仍可返回', (tester) async {
    const base = RedT1IntradaySeries(
      code: '000823.XSHE',
      date: '2026-10-09',
      prevClose: 10,
      previousDate: '2026-10-08',
      source: 'fixture',
      bars: [
        RedT1IntradayBar(time: '09:30', price: 10),
        RedT1IntradayBar(time: '15:00', price: 10.3),
      ],
    );
    await tester.pumpWidget(
      MaterialApp(
        home: Scaffold(
          body: SizedBox(
            width: 400,
            child: RedT1IntradayPreview(
              series: base,
              loadSession: (date) async => date == base.date
                  ? base
                  : const RedT1IntradaySeries(
                      code: '000823.XSHE',
                      date: '2026-10-08',
                      prevClose: 0,
                      nextDate: '2026-10-09',
                      source: '',
                      bars: [],
                    ),
            ),
          ),
        ),
      ),
    );
    await tester.tap(find.byTooltip('前一交易日分时'));
    await tester.pumpAndSettle();
    expect(find.textContaining('2026-10-08 暂无可用分时数据'), findsOneWidget);
    expect(find.byType(LineChart), findsNothing);
    await tester.tap(find.byTooltip('后一交易日分时'));
    await tester.pumpAndSettle();
    expect(find.byType(LineChart), findsOneWidget);
    expect(find.text('T+1 分时 · 2026-10-09'), findsOneWidget);
  });

  testWidgets('窄屏分时图两侧按钮不挤出图表', (tester) async {
    const series = RedT1IntradaySeries(
      code: '000823.XSHE',
      date: '2026-10-09',
      prevClose: 10,
      previousDate: '2026-10-08',
      source: 'fixture',
      bars: [
        RedT1IntradayBar(time: '09:30', price: 10),
        RedT1IntradayBar(time: '15:00', price: 10.2),
      ],
    );
    await tester.pumpWidget(
      const MaterialApp(
        home: Scaffold(
          body: SizedBox(
            width: 320,
            child: RedT1IntradayPreview(series: series),
          ),
        ),
      ),
    );
    expect(tester.getSize(find.byType(LineChart)).width, closeTo(280, .01));
    expect(find.byTooltip('前一交易日分时'), findsOneWidget);
    for (final icon in [Icons.chevron_left, Icons.chevron_right]) {
      final bounds = tester.getRect(find.widgetWithIcon(IconButton, icon));
      expect(bounds.width, closeTo(20, .01));
      expect(bounds.height, closeTo(20, .01));
    }
    expect(tester.takeException(), isNull);
  });

  testWidgets('切日请求失败时保留原图并允许重试', (tester) async {
    const series = RedT1IntradaySeries(
      code: '000823.XSHE',
      date: '2026-10-09',
      prevClose: 10,
      previousDate: '2026-10-08',
      source: 'fixture',
      bars: [
        RedT1IntradayBar(time: '09:30', price: 10),
        RedT1IntradayBar(time: '15:00', price: 10.2),
      ],
    );
    var calls = 0;
    await tester.pumpWidget(
      MaterialApp(
        home: Scaffold(
          body: SizedBox(
            width: 390,
            child: RedT1IntradayPreview(
              series: series,
              loadSession: (_) async {
                calls++;
                throw StateError('unavailable');
              },
            ),
          ),
        ),
      ),
    );
    await tester.tap(find.byTooltip('前一交易日分时'));
    await tester.pumpAndSettle();
    expect(find.text('分时切换失败，请重试'), findsOneWidget);
    expect(find.text('T+1 分时 · 2026-10-09'), findsOneWidget);
    await tester.tap(find.byTooltip('前一交易日分时'));
    await tester.pumpAndSettle();
    expect(calls, 2);
  });
}
