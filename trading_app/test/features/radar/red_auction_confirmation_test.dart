import 'dart:async';

import 'package:dio/dio.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:trading_app/features/radar/presentation/radar_list/t0_strategy_view_model.dart';

const date = '2026-10-23';
final response = <String, dynamic>{
  'date': date,
  'confirmed': true,
  'count': 1,
  'results': [
    {'股票代码': '600000.XSHG', '股票名称': '浦发银行'},
  ],
};

void main() {
  test(
    'a forbidden response after logout preserves the request error without touching disposed state',
    () async {
      final pending = Completer<Map<String, dynamic>>();
      var revoked = false;
      final vm = T0StrategyViewModel(
        request: (_, _) => pending.future,
        onModuleForbidden: (_) => revoked = true,
      );
      final request = vm.fetchRedAuctionConfirmation(date);
      vm.dispose();
      final options = RequestOptions(path: '/api/t0-selection');
      final error = DioException(
        requestOptions: options,
        response: Response(
          requestOptions: options,
          statusCode: 403,
          data: {'code': 'MODULE_FORBIDDEN'},
        ),
      );
      final expectation = expectLater(request, throwsA(same(error)));
      pending.completeError(error);
      await expectation;
      expect(revoked, isFalse);
    },
  );

  test(
    'auction reminder fetches the dated red module and updates its list',
    () async {
      final queries = <Map<String, dynamic>>[];
      final vm = T0StrategyViewModel(
        now: () => DateTime.parse('2026-10-23T01:25:10Z'),
        fetchRealtimeQuotes: (_) async => {},
        request: (module, query) async {
          expect(module, t0SharedGobRedStrategyModuleCode);
          queries.add(query);
          return response;
        },
      );
      addTearDown(vm.dispose);
      final data = await vm.fetchRedAuctionConfirmation(date);
      vm.acceptRedAuctionConfirmation(data);
      expect(queries, [
        {'date': date, 'module_code': t0SharedGobRedStrategyModuleCode},
      ]);
      final state = vm.stateFor(t0SharedGobRedStrategyModuleCode);
      expect(state.results.single.stockName, '浦发银行');
      expect(state.selectedDate, date);
      expect(state.phase, T0StrategyPhase.confirmed);
      expect(state.loading, isFalse);
    },
  );

  test(
    'automatic confirmation preserves historical view until explicitly opened',
    () async {
      final vm = T0StrategyViewModel(
        now: () => DateTime.parse('2026-10-23T01:25:10Z'),
        fetchRealtimeQuotes: (_) async => {},
        request: (_, query) async => query['list_dates'] == '1'
            ? {
                'dates': ['2026-10-23', '2026-10-22'],
              }
            : {'date': '2026-10-22', 'archived': true, 'results': []},
      );
      addTearDown(vm.dispose);
      await vm.loadResults(
        moduleCode: t0SharedGobRedStrategyModuleCode,
        date: '2026-10-22',
        archived: true,
      );
      vm.acceptRedAuctionConfirmation(response);
      expect(
        vm.stateFor(t0SharedGobRedStrategyModuleCode).selectedDate,
        '2026-10-22',
      );
      vm.acceptRedAuctionConfirmation(response, show: true);
      expect(vm.stateFor(t0SharedGobRedStrategyModuleCode).selectedDate, date);
      expect(
        vm.resultsFor(t0SharedGobRedStrategyModuleCode).single.stockName,
        '浦发银行',
      );
    },
  );
}
