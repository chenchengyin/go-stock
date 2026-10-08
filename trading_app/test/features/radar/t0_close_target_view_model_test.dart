import 'dart:async';

import 'package:flutter_test/flutter_test.dart';
import 'package:fake_async/fake_async.dart';
import 'package:shared_preferences/shared_preferences.dart';
import 'package:trading_app/features/radar/presentation/radar_list/t0_strategy_view_model.dart';

void main() {
  TestWidgetsFlutterBinding.ensureInitialized();

  test('红策在上海时间14:30进入尾盘候选预览', () {
    SharedPreferences.setMockInitialValues({});
    final vm = T0StrategyViewModel(
      now: () => DateTime.utc(2026, 10, 8, 6, 30),
      fetchRealtimeQuotes: (_) async => {},
    );
    addTearDown(vm.dispose);

    vm.applyResponseForTest({
      'date': '2026-10-08',
      'prewarm': true,
      'status': 'ready',
      'candidates': [
        {'股票代码': '600001.XSHG', '股票名称': '有效候选', '前一交易日收盘': 10},
        {'股票代码': '600002.XSHG', '股票名称': '开盘缺口超范围', '前一交易日收盘': 10},
      ],
    }, moduleCode: t0RedStrategyModuleCode);

    expect(
      vm.stateFor(t0RedStrategyModuleCode).showingTailCandidatePreview,
      isTrue,
    );
    expect(vm.resultsFor(t0RedStrategyModuleCode), isEmpty);
    vm.applyQuotesForTest({
      '600001': {'price': 9.9, 'open': 10.1},
      '600002': {'price': 10.4, 'open': 10.5},
    }, moduleCode: t0RedStrategyModuleCode);
    expect(vm.resultsFor(t0RedStrategyModuleCode).map((s) => s.rawCode), [
      '600001',
    ]);
    vm.applyQuotesForTest({
      '600001': {'price': 10.2, 'open': 10.1},
      '600002': {'price': 10.4, 'open': 10.5},
    }, moduleCode: t0RedStrategyModuleCode);
    expect(vm.resultsFor(t0RedStrategyModuleCode), isEmpty);
  });

  test('已打开的红策列表会在14:30自动刷新尾盘候选', () {
    SharedPreferences.setMockInitialValues({});
    fakeAsync((async) {
      var selectionCalls = 0;
      final vm = T0StrategyViewModel(
        now: () => DateTime.utc(2026, 10, 8, 6, 29, 58).add(async.elapsed),
        fetchRealtimeQuotes: (_) async => {},
        request: (_, query) async {
          if (query['list_dates'] == '1') return {'dates': []};
          selectionCalls++;
          if (selectionCalls == 1) {
            return {'date': '2026-10-08', 'results': []};
          }
          return {
            'date': '2026-10-08',
            'prewarm': true,
            'status': 'ready',
            'candidates': [
              {'股票代码': '600001.XSHG', '股票名称': '测试股'},
            ],
          };
        },
      );
      unawaited(vm.loadResults(moduleCode: t0RedStrategyModuleCode));
      async.flushMicrotasks();
      expect(selectionCalls, 1);

      async.elapse(const Duration(seconds: 2));
      async.flushMicrotasks();

      expect(selectionCalls, 2);
      expect(
        vm.stateFor(t0RedStrategyModuleCode).showingTailCandidatePreview,
        isTrue,
      );
      vm.dispose();
    });
  });

  test('T+1达到目标价后红策列表展示卖出提醒', () {
    SharedPreferences.setMockInitialValues({});
    final vm = T0StrategyViewModel(
      now: () => DateTime.utc(2026, 10, 9, 2),
      fetchRealtimeQuotes: (_) async => {},
    );
    addTearDown(vm.dispose);

    vm.applyResponseForTest({
      'date': '2026-10-09',
      'results': [],
      '卖出监控': [
        {
          'trade_date': '2026-10-08',
          'stock_code': '600001.XSHG',
          'stock_name': '测试股',
          'entry_close': 10,
          'target_price': 10.25,
        },
      ],
    }, moduleCode: t0RedStrategyModuleCode);
    vm.applyQuotesForTest({
      '600001': {'price': 10.25},
    }, moduleCode: t0RedStrategyModuleCode);

    final reminders = vm
        .resultsFor(t0RedStrategyModuleCode)
        .where((stock) => stock.sellMonitor?.triggered == true);
    expect(reminders, hasLength(1));
  });

  test('目标触发状态写入本地并在重载后保留', () async {
    SharedPreferences.setMockInitialValues({});
    final response = {
      'date': '2026-10-09',
      'results': [],
      '卖出监控': [
        {
          'trade_date': '2026-10-08',
          'stock_code': '600001.XSHG',
          'stock_name': '测试股',
          'entry_close': 10,
          'target_price': 10.25,
        },
      ],
    };
    final vm = T0StrategyViewModel(
      now: () => DateTime.utc(2026, 10, 9, 2),
      fetchRealtimeQuotes: (_) async => {},
    );
    vm.applyResponseForTest(response, moduleCode: t0RedStrategyModuleCode);
    vm.applyQuotesForTest({
      '600001': {'price': 10.25},
    }, moduleCode: t0RedStrategyModuleCode);
    await Future<void>.delayed(const Duration(milliseconds: 20));

    final prefs = await SharedPreferences.getInstance();
    expect(
      prefs.getStringList('t0_close_target_triggered_2026-10-09'),
      contains('600001.XSHG'),
    );

    final reloaded = T0StrategyViewModel(
      now: () => DateTime.utc(2026, 10, 9, 2),
      fetchRealtimeQuotes: (_) async => {},
    );
    reloaded.applyResponseForTest(
      response,
      moduleCode: t0RedStrategyModuleCode,
    );
    await Future<void>.delayed(const Duration(milliseconds: 20));
    expect(
      reloaded
          .resultsFor(t0RedStrategyModuleCode)
          .single
          .sellMonitor
          ?.triggered,
      isTrue,
    );
    vm.dispose();
    reloaded.dispose();
  });
}
