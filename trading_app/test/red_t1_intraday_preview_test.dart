import 'package:fl_chart/fl_chart.dart';
import 'package:flutter/material.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:trading_app/features/radar/data/red_t1_intraday_repository.dart';
import 'package:trading_app/features/radar/presentation/radar_list/red_t1_intraday_preview.dart';

void main() {
  testWidgets('T+1分时图按2%等距显示价格及涨跌幅刻度，长按涨跌幅在时间左侧', (tester) async {
    const series = RedT1IntradaySeries(
      code: '000823.XSHE',
      date: '2026-09-17',
      prevClose: 10,
      source: 'fixture',
      bars: [
        RedT1IntradayBar(time: '09:30', price: 10),
        RedT1IntradayBar(time: '10:30', price: 10.3),
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
    expect(chart().data.titlesData.rightTitles.sideTitles.interval, 2);
    expect(find.text('+2.0%'), findsOneWidget);
    expect(find.text('-2.0%'), findsOneWidget);

    final rect = tester.getRect(find.byType(LineChart));
    final gesture = await tester.startGesture(
      Offset(rect.left + 42 + (rect.width - 90) * .25, rect.center.dy),
    );
    await tester.pump(const Duration(milliseconds: 600));
    await tester.pump();
    final time = find.text('10:30');
    expect(time, findsOneWidget);
    final selectedChange = find.text('+3.0%');
    expect(selectedChange, findsOneWidget);
    expect(
      tester.getTopLeft(selectedChange).dx,
      lessThan(tester.getTopLeft(time).dx),
    );
    await gesture.moveTo(
      Offset(rect.left + 42 + (rect.width - 90) * .5, rect.center.dy),
    );
    await tester.pump();
    expect(find.text('-3.0%'), findsOneWidget);
    expect(find.text('11:30'), findsOneWidget);
    await gesture.up();
    await tester.pump();
    expect(find.text('11:30'), findsNothing);
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
}
