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
  test('sortStrategyStocksForDisplay：blue 排在实时涨幅更高的非 blue 之前', () {
    final green = T0StrategyStock(
      stockCode: '600000.XSHG',
      stockName: '浦发银行',
      openGap: 0,
      closeRet: 0,
      limitUpDates: '-',
      ma20: 0,
      amountYi: 0,
      prevClose: 0,
      prevCloseRet: 0,
      buySignal: 'green',
      liveChangePercent: 5.0,
    );
    final blue = T0StrategyStock(
      stockCode: '600519.XSHG',
      stockName: '贵州茅台',
      openGap: 0,
      closeRet: 0,
      limitUpDates: '-',
      ma20: 0,
      amountYi: 0,
      prevClose: 0,
      prevCloseRet: 0,
      buySignal: 'blue',
      liveChangePercent: 1.0,
    );
    final sorted = T0StrategyViewModel.sortStrategyStocksForDisplay(
      [green, blue],
      liveChangePercent: (s) => s.liveChangePercent,
      preview: true,
    );
    expect(sorted.first.buySignal, 'blue');
  });

  test('parses pattern buy signal fields', () {
    final s = T0StrategyStock.fromJson({
      '股票代码': '001203.XSHE',
      '股票名称': '大中矿业',
      'T0开盘涨幅(%)': 1.27,
      'T0收盘涨幅(%)': -1.73,
      '标记': '涨停破板',
      '形态': 'XY|ZT|ZT',
      '形态样本数': 56,
      '形态达标率(%)': 41.1,
      '形态真亏率(%)': 44.6,
      '买入信号': 'green',
    });
    expect(s.buySignal, 'green');
    expect(s.patternWinPct, 41.1);
    expect(s.patternFailPct, 44.6);
    expect(s.patternT0N, 56);
    expect(s.pattern, 'XY|ZT|ZT');
  });

  test('insufficient 仍解析并保留形态统计数字', () {
    final s = T0StrategyStock.fromJson({
      '股票代码': '002721.XSHE',
      '股票名称': '金一文化',
      '形态': 'ZT|MYIN|DT',
      '形态样本数': 9,
      '形态达标率(%)': 44.4,
      '形态真亏率(%)': 55.6,
      '买入信号': 'insufficient',
    });
    expect(s.buySignal, 'insufficient');
    expect(s.patternT0N, 9);
    expect(s.patternWinPct, closeTo(44.4, 0.01));
    expect(s.patternFailPct, closeTo(55.6, 0.01));
  });

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

  group('archive date navigation', () {
    T0StrategyViewModel vmWithDates({
      required List<String> available,
      required String selectedDate,
    }) {
      final vm = T0StrategyViewModel();
      vm.applyAvailableDatesForTest(available);
      vm.applyResponseForTest({
        'archived': true,
        'date': selectedDate,
        'results': [
          {'股票代码': '600000.XSHG', '股票名称': '浦发银行'},
        ],
      });
      return vm;
    }

    test('中间项：前后相邻日期均可导航', () {
      final vm = vmWithDates(
        available: ['2026-08-12', '2026-08-11', '2026-08-10'],
        selectedDate: '2026-08-11',
      );

      expect(vm.previousArchiveDate, '2026-08-10');
      expect(vm.nextArchiveDate, '2026-08-12');
      expect(vm.canGoPreviousArchive, true);
      expect(vm.canGoNextArchive, true);
    });

    test('最新项：仅可前往更早归档', () {
      final vm = vmWithDates(
        available: ['2026-08-12', '2026-08-11'],
        selectedDate: '2026-08-12',
      );

      expect(vm.nextArchiveDate, isNull);
      expect(vm.canGoNextArchive, false);
      expect(vm.previousArchiveDate, '2026-08-11');
      expect(vm.canGoPreviousArchive, true);
    });

    test('最旧项：仅可前往更新归档', () {
      final vm = vmWithDates(
        available: ['2026-08-12', '2026-08-11'],
        selectedDate: '2026-08-11',
      );

      expect(vm.previousArchiveDate, isNull);
      expect(vm.canGoPreviousArchive, false);
      expect(vm.nextArchiveDate, '2026-08-12');
      expect(vm.canGoNextArchive, true);
    });

    test('仅一项：两方向均不可导航', () {
      final vm = vmWithDates(
        available: ['2026-08-10'],
        selectedDate: '2026-08-10',
      );

      expect(vm.previousArchiveDate, isNull);
      expect(vm.nextArchiveDate, isNull);
      expect(vm.canGoPreviousArchive, false);
      expect(vm.canGoNextArchive, false);
    });

    test('今日不在 availableDates 时，前一天指向归档最新日', () {
      final vm = T0StrategyViewModel();
      vm.applyAvailableDatesForTest(['2026-08-11', '2026-08-10']);
      vm.applyResponseForTest({
        'date': '2026-08-12',
        'results': [
          {'股票代码': '600000.XSHG', '股票名称': '浦发银行'},
        ],
      });

      expect(vm.dropdownDates, ['2026-08-12', '2026-08-11', '2026-08-10']);
      expect(vm.selectedDate, '2026-08-12');
      expect(vm.previousArchiveDate, '2026-08-11');
      expect(vm.nextArchiveDate, isNull);
    });
  });
}
