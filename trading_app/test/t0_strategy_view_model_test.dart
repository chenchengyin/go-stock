import 'package:flutter_test/flutter_test.dart';
import 'package:trading_app/features/radar/presentation/radar_list/t0_strategy_view_model.dart';

void main() {
  test('凌晨 ready 历史响应：解析列表并标记历史', () {
    final vm = T0StrategyViewModel();
    vm.applyResponseForTest({
      'prewarm': true,
      'status': 'ready',
      'historical': true,
      'display_date': '2026-08-11',
      'results': [
        {'股票代码': '600011.XSHG', '股票名称': '测试股', '标记': '前一天跌停'},
      ],
    });

    expect(vm.showingHistorical, true);
    expect(vm.displayDate, '2026-08-11');
    expect(vm.selectedDate, '2026-08-11');
    expect(vm.results.length, 1);
    expect(vm.results.first.tag, '前一天跌停');
    expect(vm.warmProgress, isNull);
  });

  test('archived 响应：设置 selectedDate 与历史标记', () {
    final vm = T0StrategyViewModel();
    vm.applyResponseForTest({
      'archived': true,
      'date': '2026-08-10',
      'results': [
        {'股票代码': '600010.XSHG', '股票名称': '测试股'},
      ],
    });

    expect(vm.showingHistorical, true);
    expect(vm.displayDate, '2026-08-10');
    expect(vm.selectedDate, '2026-08-10');
    expect(vm.results.length, 1);
  });

  test('当天正常结果：清空历史标记并保留 selectedDate', () {
    final vm = T0StrategyViewModel();
    vm.applyResponseForTest({
      'prewarm': true,
      'status': 'ready',
      'historical': true,
      'display_date': '2026-08-11',
      'results': [
        {'股票代码': '600011.XSHG', '股票名称': '测试股'},
      ],
    });
    vm.applyResponseForTest({
      'date': '2026-08-12',
      'results': [
        {'股票代码': '600000.XSHG', '股票名称': '浦发银行'},
      ],
    });

    expect(vm.showingHistorical, false);
    expect(vm.displayDate, isNull);
    expect(vm.selectedDate, '2026-08-12');
    expect(vm.results.first.stockCode, '600000.XSHG');
  });

  test('dropdownDates：今日不在归档列表时插入首位', () {
    final vm = T0StrategyViewModel();
    vm.applyAvailableDatesForTest(['2026-08-11', '2026-08-10']);
    vm.applyResponseForTest({
      'date': '2026-08-12',
      'results': [
        {'股票代码': '600000.XSHG', '股票名称': '浦发银行'},
      ],
    });

    expect(vm.dropdownDates, ['2026-08-12', '2026-08-11', '2026-08-10']);
    expect(vm.showDateSelector, true);
  });
}
