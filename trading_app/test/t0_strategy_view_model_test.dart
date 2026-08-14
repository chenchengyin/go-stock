import 'package:flutter_test/flutter_test.dart';
import 'package:trading_app/features/radar/presentation/radar_list/t0_strategy_view_model.dart';

Map<String, dynamic> _candidateReady({
  required String date,
  required List<Map<String, dynamic>> candidates,
}) {
  return {
    'prewarm': true,
    'status': 'ready',
    'phase': 'candidates',
    'date': date,
    'candidate_count': candidates.length,
    'candidates': candidates,
  };
}

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

  test('09:10 即使带 candidates 也不进入预览列表', () {
    final vm = T0StrategyViewModel(
      now: () => DateTime.utc(2026, 8, 12, 1, 10), // 上海 09:10
    );
    addTearDown(vm.dispose);
    vm.applyResponseForTest(_candidateReady(
      date: '2026-08-12',
      candidates: [
        {'股票代码': '600000.XSHG', '股票名称': '浦发银行'},
      ],
    ));

    expect(vm.phase, T0StrategyPhase.waiting);
    expect(vm.showingCandidatePreview, false);
    expect(vm.results, isEmpty);
    expect(vm.warmProgress?.candidateCount, 1);
  });

  test('09:20 进入候选预览并按实时涨幅降序，缺失行情沉底', () {
    final vm = T0StrategyViewModel(
      now: () => DateTime.utc(2026, 8, 12, 1, 20), // 上海 09:20
    );
    addTearDown(vm.dispose);
    vm.applyResponseForTest(_candidateReady(
      date: '2026-08-12',
      candidates: [
        {'股票代码': '600000.XSHG', '股票名称': '浦发银行'},
        {'股票代码': '000001.XSHE', '股票名称': '平安银行'},
        {'股票代码': '600519.XSHG', '股票名称': '贵州茅台'},
      ],
    ));
    expect(vm.phase, T0StrategyPhase.candidatePreview);
    expect(vm.showingCandidatePreview, true);
    expect(vm.results.length, 3);

    vm.applyQuotesForTest({
      '600000': {'code': '600000', 'changePercent': 1.2},
      '000001': {'code': '000001', 'changePercent': 2.5},
    });

    expect(vm.results.map((e) => e.rawCode).toList(), ['000001', '600000', '600519']);
    expect(vm.results.first.liveChangePercent, 2.5);
  });

  test('09:25 正式结果进入 confirmed 并停止行情轮询', () {
    final vm = T0StrategyViewModel(
      now: () => DateTime.utc(2026, 8, 12, 1, 25), // 上海 09:25
    );
    addTearDown(vm.dispose);
    vm.applyResponseForTest(_candidateReady(
      date: '2026-08-12',
      candidates: [
        {'股票代码': '600000.XSHG', '股票名称': '浦发银行'},
      ],
    ));
    expect(vm.showingCandidatePreview, false);

    vm.applyResponseForTest({
      'date': '2026-08-12',
      'results': [
        {'股票代码': '600000.XSHG', '股票名称': '浦发银行', 'T0开盘涨幅(%)': 1.1},
      ],
    });

    expect(vm.phase, T0StrategyPhase.confirmed);
    expect(vm.showingCandidatePreview, false);
    expect(vm.isQuotePolling, false);
    expect(vm.results.first.openGap, 1.1);
  });
}
