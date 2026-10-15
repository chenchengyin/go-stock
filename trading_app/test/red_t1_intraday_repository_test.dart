import 'package:dio/dio.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:trading_app/features/radar/data/red_t1_intraday_repository.dart';

void main() {
  test('空结果可重新请求，恢复后的有效分时会被缓存', () async {
    var requestCount = 0;
    final repository = RedT1IntradayRepository(
      fetchBatch: (date, codes, moduleCode) async {
        requestCount++;
        final code = codes.single;
        final hasData = requestCount > 1;
        return {
          'date': date,
          'previous_date': '2026-10-08',
          'items': {
            code: {
              'code': code,
              'date': date,
              'source': 'ths',
              'status': hasData ? 'ok' : 'no_intraday_data',
              'prevClose': 10,
              'bars': hasData
                  ? [
                      {'time': '09:30', 'price': 10.1},
                      {'time': '15:00', 'price': 10.2},
                    ]
                  : <Map<String, dynamic>>[],
            },
          },
        };
      },
    );

    final first = await repository.load('2026-10-08', ['000823.XSHE']);
    final second = await repository.load('2026-10-08', ['000823.XSHE']);
    final third = await repository.load('2026-10-08', ['000823.XSHE']);

    expect(first['000823.XSHE']!.bars, isEmpty);
    expect(second['000823.XSHE']!.bars, hasLength(2));
    expect(third['000823.XSHE']!.bars, hasLength(2));
    expect(requestCount, 2);
  });

  test('按交易日请求一只股票分时，传递模块权限并使用当日昨收与相邻交易日', () async {
    final requests = <RequestOptions>[];
    final dio = Dio(BaseOptions(baseUrl: 'https://example.test'));
    dio.interceptors.add(
      InterceptorsWrapper(
        onRequest: (options, handler) {
          requests.add(options);
          handler.resolve(
            Response(
              requestOptions: options,
              data: {
                'date': '2026-10-08',
                'previous_date': '2026-09-30',
                'next_date': '2026-10-09',
                'items': {
                  '000823.XSHE': {
                    'code': '000823.XSHE',
                    'date': '2026-10-08',
                    'prevClose': 9.8,
                    'bars': [
                      {'time': '09:30', 'price': 10.0},
                      {'time': '15:00', 'price': 10.3},
                    ],
                  },
                },
              },
            ),
          );
        },
      ),
    );
    final repository = RedT1IntradayRepository(dio: dio);
    final series = await repository.loadSession(
      '2026-10-08',
      '000823.XSHE',
      '2026-10-08',
      moduleCode: 'radar.red_shared_gob_strategy',
    );
    expect(requests, hasLength(1));
    expect(requests.single.path, '/api/t0-red/t1-intraday');
    expect(requests.single.queryParameters, {
      'date': '2026-10-08',
      'codes': '000823.XSHE',
      'module_code': 'radar.red_shared_gob_strategy',
      'session_date': '2026-10-08',
    });
    expect(series.prevClose, 9.8);
    expect(series.previousDate, '2026-09-30');
    expect(series.nextDate, '2026-10-09');
    expect(series.bars, hasLength(2));
  });

  test('无分时条目时保留日期边界但不伪造价格', () async {
    final dio = Dio(BaseOptions(baseUrl: 'https://example.test'));
    dio.interceptors.add(
      InterceptorsWrapper(
        onRequest: (options, handler) {
          handler.resolve(
            Response(
              requestOptions: options,
              data: {
                'date': '2026-10-09',
                'previous_date': '2026-10-08',
                'next_date': '',
                'items': <String, dynamic>{},
              },
            ),
          );
        },
      ),
    );
    final result = await RedT1IntradayRepository(
      dio: dio,
    ).loadSession('2026-10-08', '000823.XSHE', '2026-10-09');
    expect(result.date, '2026-10-09');
    expect(result.previousDate, '2026-10-08');
    expect(result.nextDate, '');
    expect(result.prevClose, 0);
    expect(result.bars, isEmpty);
  });
}
