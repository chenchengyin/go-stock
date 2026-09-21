import 'package:flutter_test/flutter_test.dart';
import 'package:trading_app/features/radar/domain/radar_models.dart';
import 'package:trading_app/features/radar/domain/sell_warning.dart';

MonitoredStock quote({
  required double open,
  required double preClose,
  required double changePercent,
  required double high,
  double previousHigh = 0,
  String date = '2026-09-21',
}) {
  return MonitoredStock(
    code: 'sz000001',
    name: '测试股票',
    price: 10,
    open: open,
    preClose: preClose,
    changePercent: changePercent,
    high: high,
    previousHigh: previousHigh,
    date: date,
  );
}

void main() {
  test('低开且当前涨跌幅达到负一时产生卖出信号', () {
    final result = evaluateSellWarning(
      quote(open: 9.8, preClose: 10, changePercent: -1.0, high: 9.9),
    );

    expect(result?.type, SellWarningType.lowOpen);
    expect(result?.label, '卖出');
  });

  test('低开但当前涨跌幅高于负一时不产生卖出信号', () {
    final result = evaluateSellWarning(
      quote(open: 9.8, preClose: 10, changePercent: -0.99, high: 9.9),
    );

    expect(result, isNull);
  });

  test('高开且今日最高价触及前一交易日最高价时产生卖出信号', () {
    final result = evaluateSellWarning(
      quote(
        open: 10.2,
        preClose: 10,
        changePercent: 3,
        high: 12,
        previousHigh: 12,
      ),
    );

    expect(result?.type, SellWarningType.highOpen);
    expect(result?.label, '卖出');
  });

  test('高开但今日最高价未触及前一交易日最高价时不产生卖出信号', () {
    final result = evaluateSellWarning(
      quote(
        open: 10.2,
        preClose: 10,
        changePercent: 1,
        high: 11.99,
        previousHigh: 12,
      ),
    );

    expect(result, isNull);
  });

  test('平开不触发低开或高开卖出信号', () {
    final result = evaluateSellWarning(
      quote(
        open: 10,
        preClose: 10,
        changePercent: -1,
        high: 12,
        previousHigh: 12,
      ),
    );

    expect(result, isNull);
  });

  test('监控行情可以解析后端返回的前一交易日最高价', () {
    final stock = MonitoredStock.fromJson({
      'code': 'sz000001',
      'name': '测试股票',
      'prevHigh': 12.34,
    });

    expect(stock.previousHigh, 12.34);
  });

  test('卖出信号在同一交易日保持，跨交易日重新判断', () {
    final tracker = SellWarningTracker();
    final triggered = quote(
      open: 9.8,
      preClose: 10,
      changePercent: -1,
      high: 9.9,
    );
    tracker.observe([triggered]);
    expect(tracker.contains(triggered.code), isTrue);

    tracker.observe([
      quote(open: 9.8, preClose: 10, changePercent: 0, high: 10),
    ]);
    expect(tracker.contains(triggered.code), isTrue);

    tracker.observe([
      quote(
        open: 10,
        preClose: 10,
        changePercent: 0,
        high: 10,
        date: '2026-09-22',
      ),
    ]);
    expect(tracker.contains(triggered.code), isFalse);
  });
}
