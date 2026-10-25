import 'dart:async';

import 'package:dio/dio.dart';
import 'package:fl_chart/fl_chart.dart';
import 'package:flutter/material.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:provider/provider.dart';
import 'package:shared_preferences/shared_preferences.dart';
import 'package:visibility_detector/visibility_detector.dart';
import 'package:trading_app/core/storage/red_kline_settings.dart';
import 'package:trading_app/features/permissions/presentation/module_permission_controller.dart';
import 'package:trading_app/features/permissions/domain/module_definition.dart';
import 'package:trading_app/features/radar/data/radar_repository.dart';
import 'package:trading_app/features/radar/data/red_kline_repository.dart';
import 'package:trading_app/features/radar/data/red_t1_intraday_repository.dart';
import 'package:trading_app/features/radar/domain/voice_announcement_view_model.dart';
import 'package:trading_app/features/radar/presentation/radar_list/radar_page.dart';
import 'package:trading_app/features/radar/presentation/radar_list/radar_view_model.dart';
import 'package:trading_app/features/radar/presentation/radar_list/strategy_stock_charts.dart';
import 'package:trading_app/features/radar/presentation/radar_list/t0_strategy_view_model.dart';

Map<String, dynamic> window(String code, String date) => {
  'code': code,
  'date': date,
  'anchor_index': 1,
  'before_count': 1,
  'after_count': 1,
  'source': 'fixture',
  'status': 'ok',
  'partial': true,
  'bars': [
    {'date': '2026-09-23', 'open': 10, 'close': 11, 'high': 12, 'low': 9},
    {'date': date, 'open': 11, 'close': 12, 'high': 13, 'low': 10},
    {'date': '2026-09-25', 'open': 12, 'close': 13, 'high': 14, 'low': 11},
  ],
};

class _Harness {
  final settings = RedKlineSettings();
  final strategy = _Strategy();
  final radar = _Radar();
  final voice = _Voice();
  final requests = <RequestOptions>[];
  late final dio = Dio(BaseOptions(baseUrl: 'https://quotes.test'))
    ..interceptors.add(
      InterceptorsWrapper(
        onRequest: (options, handler) {
          requests.add(options);
          final date = options.queryParameters['date'] as String;
          final codes = (options.queryParameters['codes'] as String).split(',');
          handler.resolve(
            Response(
              requestOptions: options,
              data: {
                'date': options.path.endsWith('kline-window')
                    ? date
                    : '2026-09-25',
                'previous_date': '2026-09-24',
                'next_date': '',
                'items': {
                  for (final code in codes)
                    code: options.path.endsWith('kline-window')
                        ? window(code, date)
                        : {
                            'code': code,
                            'date': '2026-09-25',
                            'prevClose': 12,
                            'source': 'fixture',
                            'status': 'ok',
                            'bars': [
                              {'time': '09:30', 'price': 12.1},
                              {'time': '15:00', 'price': 13},
                            ],
                          },
                },
              },
            ),
          );
        },
      ),
    );
  late final kline = RedKlineRepository(dio: dio);
  late final intraday = RedT1IntradayRepository(dio: dio);
  final permissions = ModulePermissionController.forTesting([
    ...publicModuleDefinitions,
    const ModuleDefinition(
      code: t0MainStrategyModuleCode,
      name: '主板策略',
      client: 'flutter_web',
      placement: 'radar_tab',
      parentCode: null,
      sort: 30,
      accessMode: ModuleAccessMode.userAllowlist,
    ),
  ]);

  void show({int count = 1, String date = '2026-09-24'}) {
    strategy.applyResponseForTest({
      'date': date,
      'display_date': date,
      'archived': true,
      'results': [
        for (var i = 0; i < count; i++)
          {
            '股票代码': '600${i.toString().padLeft(3, '0')}.XSHG',
            '股票名称': '主板股$i',
            'T0开盘涨幅(%)': 1.23,
            'T0收盘涨幅(%)': 2.34,
            '形态': 'ZT|ZT|MYIN',
            '形态样本数': 123,
            '形态达标率(%)': 70,
            '形态真亏率(%)': 20,
          },
      ],
    });
  }

  Widget app() => MultiProvider(
    providers: [
      ChangeNotifierProvider<RedKlineSettings>.value(value: settings),
      ChangeNotifierProvider<T0StrategyViewModel>.value(value: strategy),
      ChangeNotifierProvider<RadarViewModel>.value(value: radar),
      ChangeNotifierProvider<VoiceAnnouncementViewModel>.value(value: voice),
      ChangeNotifierProvider<ModulePermissionController>.value(
        value: permissions,
      ),
      Provider<RedKlineRepository>.value(value: kline),
      Provider<RedT1IntradayRepository>.value(value: intraday),
    ],
    child: const MaterialApp(home: RadarPage()),
  );

  void dispose() {
    settings.dispose();
    strategy.dispose();
    radar.dispose();
    voice.dispose();
    permissions.dispose();
    dio.close();
  }
}

Future<void> expose(WidgetTester tester) async {
  await tester.pumpAndSettle();
  await tester.pump(const Duration(milliseconds: 450));
  await tester.pumpAndSettle();
}

void main() {
  setUp(() {
    SharedPreferences.setMockInitialValues({'voice_announcement_asked': true});
    VisibilityDetectorController.instance.updateInterval = Duration.zero;
  });
  tearDown(() {
    VisibilityDetectorController.instance.updateInterval = const Duration(
      milliseconds: 500,
    );
  });

  testWidgets(
    'mainboard displays charts with its module and preserves stock statistics; switches are independent',
    (tester) async {
      final h = _Harness();
      await h.settings.restore();
      h.show();
      await tester.pumpWidget(h.app());
      await tester.tap(find.text('主板策略(1)'));
      await expose(tester);
      expect(find.byType(CandlestickChart), findsOneWidget);
      expect(find.byType(LineChart), findsOneWidget);
      expect(find.text('70/80/123'), findsOneWidget);
      expect(find.text('重仓'), findsOneWidget);
      expect(find.byTooltip('复制股票代码'), findsOneWidget);
      expect(h.requests, hasLength(2));
      expect(
        h.requests.every(
          (r) => r.queryParameters['module_code'] == 'radar.main_strategy',
        ),
        isTrue,
      );
      await h.settings.setEnabled(false);
      await tester.pumpAndSettle();
      expect(find.byType(CandlestickChart), findsOneWidget);
      await h.settings.setEnabledFor('radar.main_strategy', false);
      await tester.pumpAndSettle();
      expect(find.byType(CandlestickChart), findsNothing);
      expect(find.byType(LineChart), findsOneWidget);
      await h.settings.setIntradayEnabledFor('radar.main_strategy', false);
      h.show(date: '2026-09-22');
      await expose(tester);
      expect(find.byType(LineChart), findsNothing);
      expect(h.requests, hasLength(2));
      await tester.pumpWidget(const SizedBox.shrink());
      h.dispose();
      await tester.pump(const Duration(milliseconds: 200));
    },
  );

  testWidgets('pulling mainboard list refreshes both chart panels', (
    tester,
  ) async {
    final h = _Harness();
    await h.settings.restore();
    h.show();
    await tester.pumpWidget(h.app());
    await tester.tap(find.text('主板策略(1)'));
    await expose(tester);
    expect(
      h.requests.where((r) => r.path.endsWith('kline-window')),
      hasLength(1),
    );
    await tester.drag(find.byType(ListView).last, const Offset(0, 500));
    await expose(tester);
    expect(
      h.requests.where((r) => r.path.endsWith('kline-window')),
      hasLength(2),
    );
    expect(
      h.requests.where((r) => r.path.endsWith('t1-intraday')),
      hasLength(2),
    );
    await tester.pumpWidget(const SizedBox.shrink());
    h.dispose();
    await tester.pump(const Duration(milliseconds: 200));
  });

  testWidgets(
    'mainboard long list fetches only exposed cards and loads more after scrolling',
    (tester) async {
      tester.view.physicalSize = const Size(390, 844);
      tester.view.devicePixelRatio = 1;
      addTearDown(tester.view.resetPhysicalSize);
      addTearDown(tester.view.resetDevicePixelRatio);
      final h = _Harness();
      await h.settings.restore();
      h.show(count: 30);
      await tester.pumpWidget(h.app());
      await tester.tap(find.text('主板策略(30)'));
      await expose(tester);
      final initial = h.requests.map((r) => r.queryParameters['codes']).toSet();
      expect(initial, contains('600000.XSHG'));
      expect(initial.length, lessThan(10));
      expect(initial, isNot(contains('600029.XSHG')));
      await tester.drag(find.byType(ListView).last, const Offset(0, -1400));
      await expose(tester);
      final later = h.requests.map((r) => r.queryParameters['codes']).toSet();
      expect(later.length, greaterThan(initial.length));
      expect(later.length, lessThan(30));
      expect(tester.takeException(), isNull);
      await tester.pumpWidget(const SizedBox.shrink());
      h.dispose();
      await tester.pump(const Duration(milliseconds: 200));
    },
  );

  testWidgets('failed daily chart does not hide available intraday panel', (
    tester,
  ) async {
    final h = _Harness();
    final failed = RedKlineRepository(
      fetchBatch: (_, __) async => throw StateError('offline'),
    );
    await tester.pumpWidget(
      MaterialApp(
        home: Scaffold(
          body: ListView(
            children: [
              StrategyStockCharts(
                moduleCode: 'radar.main_strategy',
                date: '2026-09-24',
                code: '600000.XSHG',
                showKline: true,
                showIntraday: true,
                klineRepository: failed,
                intradayRepository: h.intraday,
              ),
            ],
          ),
        ),
      ),
    );
    await expose(tester);
    expect(find.text('日K加载失败'), findsOneWidget);
    expect(find.byType(LineChart), findsOneWidget);
    await tester.pumpWidget(const SizedBox.shrink());
    h.dispose();
    await tester.pump(const Duration(milliseconds: 200));
  });

  testWidgets(
    'date changes discard previous daily results while new chart is shown',
    (tester) async {
      final old = Completer<Map<String, dynamic>>();
      final repository = RedKlineRepository(
        fetchBatch: (date, codes) async {
          if (date == '2026-09-24') return old.future;
          return {
            'items': {codes.single: window(codes.single, date)},
          };
        },
      );
      Widget page(String date) => MaterialApp(
        home: Scaffold(
          body: ListView(
            children: [
              StrategyStockCharts(
                moduleCode: 'radar.main_strategy',
                date: date,
                code: '600000.XSHG',
                showKline: true,
                showIntraday: false,
                klineRepository: repository,
              ),
            ],
          ),
        ),
      );
      await tester.pumpWidget(page('2026-09-24'));
      await tester.pump();
      await tester.pump(const Duration(milliseconds: 450));
      await tester.pumpWidget(page('2026-09-22'));
      await expose(tester);
      expect(find.text('2026-09-22 为选中日 · 右侧为事后走势'), findsOneWidget);
      old.complete({
        'items': {'600000.XSHG': window('600000.XSHG', '2026-09-24')},
      });
      await tester.pumpAndSettle();
      expect(find.text('2026-09-24 为选中日 · 右侧为事后走势'), findsNothing);
      expect(find.text('2026-09-22 为选中日 · 右侧为事后走势'), findsOneWidget);
      await tester.pumpWidget(const SizedBox.shrink());
    },
  );
}

class _Strategy extends T0StrategyViewModel {
  @override
  Future<void> warmUpIfNeeded({
    String moduleCode = t0MainStrategyModuleCode,
  }) async {}
  @override
  Future<void> loadAvailableDates({
    String moduleCode = t0MainStrategyModuleCode,
  }) async {}
  @override
  Future<void> loadResults({
    String moduleCode = t0MainStrategyModuleCode,
    String? date,
    bool archived = false,
  }) async {}
}

class _Radar extends RadarViewModel {
  _Radar() : super(RadarRepositoryImpl());
  @override
  Future<void> loadWatchChanges() async {}
  @override
  Future<void> loadAllChanges() async {}
  @override
  Future<void> loadMonitoredStocks() async {}
}

class _Voice extends VoiceAnnouncementViewModel {
  @override
  bool get askedBefore => true;
  @override
  // ignore: must_call_super
  void dispose() {}
}
