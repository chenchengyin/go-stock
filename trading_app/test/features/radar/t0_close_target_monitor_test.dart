import 'package:flutter_test/flutter_test.dart';
import 'package:trading_app/features/radar/domain/t0_close_target_monitor.dart';

void main() {
  test('目标价相等或更高时触发，低于目标及无效行情不触发', () {
    const monitor = T0CloseTargetMonitor(
      tradeDate: '2026-10-08',
      stockCode: '600001.XSHG',
      stockName: '测试股',
      entryClose: 10,
      targetPrice: 10.25,
    );

    expect(monitor.isReachedBy(10.249), isFalse);
    expect(monitor.isReachedBy(10.25), isTrue);
    expect(monitor.isReachedBy(10.30), isTrue);
    expect(monitor.isReachedBy(0), isFalse);
  });

  test('目标监控项 JSON 往返保留已触发状态', () {
    const monitor = T0CloseTargetMonitor(
      tradeDate: '2026-10-08',
      stockCode: '600001.XSHG',
      stockName: '测试股',
      entryClose: 10,
      targetPrice: 10.25,
      triggered: true,
    );

    final restored = T0CloseTargetMonitor.fromJson(monitor.toJson());
    expect(restored.tradeDate, monitor.tradeDate);
    expect(restored.targetPrice, monitor.targetPrice);
    expect(restored.triggered, isTrue);
  });
}
