import 'package:flutter/cupertino.dart';
import 'package:flutter/material.dart';
import 'package:fl_chart/fl_chart.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:shared_preferences/shared_preferences.dart';
import 'package:trading_app/core/storage/red_kline_settings.dart';
import 'package:trading_app/core/theme/theme_manager.dart';
import 'package:trading_app/features/auth/data/auth_repository.dart';
import 'package:trading_app/features/auth/presentation/auth_view_model.dart';
import 'package:trading_app/features/profile/presentation/system_settings_page.dart';
import 'package:trading_app/features/radar/data/red_kline_repository.dart';
import 'package:provider/provider.dart';
import 'package:trading_app/features/permissions/domain/module_definition.dart';
import 'package:trading_app/features/permissions/presentation/module_permission_controller.dart';
import 'package:trading_app/features/radar/data/radar_repository.dart';
import 'package:trading_app/features/radar/domain/voice_announcement_view_model.dart';
import 'package:trading_app/features/radar/presentation/radar_list/radar_page.dart';
import 'package:trading_app/features/radar/presentation/radar_list/radar_view_model.dart';
import 'package:trading_app/features/radar/presentation/radar_list/t0_strategy_view_model.dart';
import 'package:trading_app/features/radar/presentation/radar_list/red_kline_preview.dart';

void main() {
  testWidgets('红策蜡烛实体在卡片和放大图均增加2像素', (tester) async {
    for (final candleCount in [66, 10]) {
      final date = '2026-09-16';
      final window = RedKlineWindow.fromJson({
        'code': '000823.XSHE',
        'date': date,
        'anchor_index': candleCount - 1,
        'before_count': candleCount - 1,
        'after_count': 0,
        'source': 'fixture',
        'status': 'ok',
        'partial': true,
        'bars': [
          for (var i = 0; i < candleCount; i++)
            {
              'date': DateTime(
                2026,
                1,
                1,
              ).add(Duration(days: i)).toIso8601String().substring(0, 10),
              'open': 10.0,
              'high': 12.0,
              'low': 9.0,
              'close': 11.0,
            },
        ],
      });
      await tester.pumpWidget(
        MaterialApp(
          home: Scaffold(
            body: SizedBox(width: 390, child: RedKlinePreview(window: window)),
          ),
        ),
      );
      double bodyWidth() {
        final chart = tester.widget<CandlestickChart>(
          find.byType(CandlestickChart).last,
        );
        final painter =
            chart.data.candlestickPainter as DefaultCandlestickPainter;
        return painter
            .candlestickStyleProvider(chart.data.candlestickSpots.first, 0)
            .bodyWidth;
      }

      expect(bodyWidth(), candleCount < 20 ? 9 : 4.8);
      await tester.tap(find.byTooltip('放大K线'));
      await tester.pumpAndSettle();
      expect(bodyWidth(), candleCount < 20 ? 11 : 7);
      await tester.tap(find.byTooltip('关闭K线'));
      await tester.pumpAndSettle();
      await tester.pumpWidget(const SizedBox.shrink());
    }
  });

  testWidgets('红策K线叠加MA5/10/20/60并显示图例', (tester) async {
    final window = RedKlineWindow.fromJson({
      'code': '000823.XSHE',
      'date': '2026-09-16',
      'anchor_index': 1,
      'before_count': 1,
      'after_count': 0,
      'source': 'cache',
      'status': 'ok',
      'partial': true,
      'bars': [
        {
          'date': '2026-09-15',
          'open': 10,
          'high': 12,
          'low': 9,
          'close': 11,
          'ma': {'5': 10.5, '10': 10.2, '20': 10.1, '60': 10.0},
        },
        {
          'date': '2026-09-16',
          'open': 11,
          'high': 13,
          'low': 10,
          'close': 12,
          'ma': {'5': 11.0, '10': 10.5, '20': 10.2, '60': 10.1},
        },
      ],
    });
    await tester.pumpWidget(
      MaterialApp(
        home: Scaffold(
          body: SizedBox(width: 390, child: RedKlinePreview(window: window)),
        ),
      ),
    );
    expect(window.bars.last.ma[60], 10.1);
    for (final period in [5, 10, 20, 60]) {
      expect(find.text('MA$period'), findsOneWidget);
    }
    final chart = tester.widget<CandlestickChart>(
      find.byType(CandlestickChart),
    );
    expect(chart.data.candlestickPainter, isA<RedMovingAveragePainter>());
  });

  testWidgets('系统设置红策K线开关默认打开且关闭后持久化', (tester) async {
    SharedPreferences.setMockInitialValues({});
    final setting = RedKlineSettings();
    await setting.restore();
    final auth = AuthViewModel(_UnusedAuthRepository());
    final theme = ThemeManager();
    addTearDown(() {
      setting.dispose();
      auth.dispose();
      theme.dispose();
    });
    await tester.pumpWidget(
      MultiProvider(
        providers: [
          ChangeNotifierProvider<RedKlineSettings>.value(value: setting),
          ChangeNotifierProvider<AuthViewModel>.value(value: auth),
          ChangeNotifierProvider<ThemeManager>.value(value: theme),
        ],
        child: const CupertinoApp(home: SystemSettingsPage()),
      ),
    );
    expect(find.text('红策K线图'), findsOneWidget);
    final switchFinder = find.byType(CupertinoSwitch);
    expect(tester.widget<CupertinoSwitch>(switchFinder.last).value, isTrue);
    await tester.tap(switchFinder.last);
    await tester.pumpAndSettle();
    expect(setting.enabled, isFalse);
    expect(
      (await SharedPreferences.getInstance()).getBool(
        RedKlineSettings.preferenceKey,
      ),
      isFalse,
    );
    await tester.pumpWidget(const SizedBox.shrink());
  });

  testWidgets('红策列表切日、开关及12卡滚动只构建可见K线', (tester) async {
    tester.view.physicalSize = const Size(390, 844);
    tester.view.devicePixelRatio = 1;
    addTearDown(() {
      tester.view.resetPhysicalSize();
      tester.view.resetDevicePixelRatio();
    });
    SharedPreferences.setMockInitialValues({'voice_announcement_asked': true});
    final setting = RedKlineSettings();
    await setting.restore();
    final calls = <String>[];
    final charts = RedKlineRepository(
      fetchBatch: (date, codes) async {
        calls.add('$date:${codes.join(',')}');
        return {
          'date': date,
          'items': {
            for (final code in codes)
              code: {
                'code': code,
                'date': date,
                'anchor_index': 1,
                'before_count': 1,
                'after_count': 1,
                'source': 'fixture',
                'status': 'ok',
                'partial': true,
                'bars': [
                  {
                    'date': '2026-09-14',
                    'open': 10,
                    'high': 12,
                    'low': 9,
                    'close': 11,
                  },
                  {
                    'date': date,
                    'open': 11,
                    'high': 13,
                    'low': 10,
                    'close': 12,
                  },
                  {
                    'date': '2026-09-18',
                    'open': 12,
                    'high': 14,
                    'low': 11,
                    'close': 13,
                  },
                ],
              },
          },
        };
      },
    );
    final strategy = _NoNetworkRedViewModel();
    final radar = RadarViewModel(RadarRepositoryImpl());
    final voice = _SilentVoiceViewModel();
    final permissions = ModulePermissionController.forTesting([
      ...publicModuleDefinitions,
      const ModuleDefinition(
        code: 'radar.red_strategy',
        name: '红策',
        client: 'flutter_web',
        placement: 'radar_tab',
        parentCode: null,
        sort: 15,
        accessMode: ModuleAccessMode.userAllowlist,
      ),
    ]);
    var disposed = false;
    void disposeVms() {
      if (disposed) return;
      disposed = true;
      radar.dispose();
      strategy.dispose();
      voice.dispose();
      permissions.dispose();
      setting.dispose();
    }

    addTearDown(disposeVms);
    void showDate(String date) {
      strategy.applyResponseForTest({
        'date': date,
        'display_date': date,
        'archived': true,
        'results': [
          {'股票代码': '000823.XSHE', '股票名称': '验收股'},
        ],
      }, moduleCode: t0RedStrategyModuleCode);
    }

    showDate('2026-09-16');
    strategy.applyAvailableDatesForTest([
      '2026-09-16',
      '2026-09-17',
    ], moduleCode: t0RedStrategyModuleCode);
    await tester.pumpWidget(
      MultiProvider(
        providers: [
          ChangeNotifierProvider<RedKlineSettings>.value(value: setting),
          Provider<RedKlineRepository>.value(value: charts),
          ChangeNotifierProvider<T0StrategyViewModel>.value(value: strategy),
          ChangeNotifierProvider<RadarViewModel>.value(value: radar),
          ChangeNotifierProvider<VoiceAnnouncementViewModel>.value(
            value: voice,
          ),
          ChangeNotifierProvider<ModulePermissionController>.value(
            value: permissions,
          ),
        ],
        child: const MaterialApp(home: RadarPage()),
      ),
    );
    await tester.tap(find.text('红策(1)'));
    await tester.pumpAndSettle();
    expect(find.byType(CandlestickChart), findsOneWidget);
    expect(find.text('2026-09-16 为选中日 · 右侧为事后走势'), findsOneWidget);
    expect(calls, ['2026-09-16:000823.XSHE']);
    await setting.setEnabled(false);
    await tester.pumpAndSettle();
    expect(find.byType(CandlestickChart), findsNothing);
    showDate('2026-09-17');
    await tester.pumpAndSettle();
    expect(calls, hasLength(1));
    await setting.setEnabled(true);
    await tester.pumpAndSettle();
    expect(find.byType(CandlestickChart), findsOneWidget);
    expect(find.text('2026-09-17 为选中日 · 右侧为事后走势'), findsOneWidget);
    expect(calls, ['2026-09-16:000823.XSHE', '2026-09-17:000823.XSHE']);

    strategy.applyResponseForTest({
      'date': '2026-09-17',
      'display_date': '2026-09-17',
      'archived': true,
      'results': [
        for (var i = 0; i < 12; i++)
          {
            '股票代码': '600${(100 + i).toString().padLeft(3, '0')}.XSHG',
            '股票名称': '压力股 $i',
          },
      ],
    }, moduleCode: t0RedStrategyModuleCode);
    await tester.pumpAndSettle();
    expect(find.text('红策(12)'), findsOneWidget);
    expect(calls.last.split(',').length, 12);
    final initialCharts = find.byType(CandlestickChart).evaluate().length;
    expect(initialCharts, inInclusiveRange(1, 7));
    await tester.drag(find.byType(ListView).last, const Offset(0, -1500));
    await tester.pumpAndSettle();
    expect(find.byType(CandlestickChart), findsWidgets);
    expect(find.byType(CandlestickChart).evaluate().length, lessThan(12));
    expect(tester.takeException(), isNull);

    await tester.pumpWidget(const SizedBox.shrink());
    disposeVms();
  });

  testWidgets('新股短历史K线在卡片中仍有可辨识的实体宽度', (tester) async {
    final window = RedKlineWindow.fromJson({
      'code': '601091.XSHG',
      'date': '2026-09-30',
      'anchor_index': 8,
      'before_count': 8,
      'after_count': 1,
      'source': 'tencent',
      'status': 'ok',
      'partial': true,
      'bars': [
        for (var i = 0; i < 10; i++)
          {
            'date': '2026-09-${(17 + i).toString().padLeft(2, '0')}',
            'open': 10 + i,
            'high': 12 + i,
            'low': 9 + i,
            'close': 11 + i,
          },
      ],
    });
    await tester.pumpWidget(
      MaterialApp(
        home: Scaffold(
          body: SizedBox(width: 390, child: RedKlinePreview(window: window)),
        ),
      ),
    );
    final chart = tester.widget<CandlestickChart>(
      find.byType(CandlestickChart),
    );
    final painter = chart.data.candlestickPainter as DefaultCandlestickPainter;
    final style = painter.candlestickStyleProvider(
      chart.data.candlestickSpots.first,
      0,
    );
    expect(style.bodyWidth, greaterThanOrEqualTo(6));
  });

  testWidgets('网络失败和无K线数据使用不同提示', (tester) async {
    await tester.pumpWidget(
      const MaterialApp(home: Scaffold(body: RedKlinePreview(error: true))),
    );
    expect(find.text('日K加载失败'), findsOneWidget);
    await tester.pumpWidget(
      const MaterialApp(home: Scaffold(body: RedKlinePreview())),
    );
    expect(find.text('暂无可用日K线'), findsOneWidget);
  });

  testWidgets('红策K线显示日线和放大入口', (tester) async {
    final window = RedKlineWindow.fromJson({
      'code': '000823.XSHE',
      'date': '2026-09-16',
      'anchor_index': 1,
      'before_count': 1,
      'after_count': 1,
      'source': 'tencent',
      'status': 'ok',
      'partial': true,
      'bars': [
        {'date': '2026-09-15', 'open': 10, 'high': 12, 'low': 9, 'close': 11},
        {'date': '2026-09-16', 'open': 11, 'high': 13, 'low': 10, 'close': 12},
        {'date': '2026-09-18', 'open': 12, 'high': 14, 'low': 11, 'close': 13},
      ],
    });
    await tester.pumpWidget(
      MaterialApp(
        home: Scaffold(
          body: SizedBox(width: 360, child: RedKlinePreview(window: window)),
        ),
      ),
    );
    expect(find.byType(CandlestickChart), findsOneWidget);
    expect(find.textContaining('前1根'), findsOneWidget);
    await tester.tap(find.byTooltip('放大K线'));
    await tester.pumpAndSettle();
    expect(find.byType(CandlestickChart), findsNWidgets(2));
  });

  test('K线开关默认开启，关闭后可恢复', () async {
    SharedPreferences.setMockInitialValues({});
    final settings = RedKlineSettings();
    expect(settings.ready, isFalse);
    await settings.restore();
    expect(settings.ready, isTrue);
    expect(settings.enabled, isTrue);
    expect(await settings.setEnabled(false), isTrue);
    final restored = RedKlineSettings();
    await restored.restore();
    expect(restored.enabled, isFalse);
  });

  test('解析交易日窗口且不填充缺失K线', () {
    final window = RedKlineWindow.fromJson({
      'code': '000823.XSHE',
      'date': '2026-09-16',
      'anchor_index': 1,
      'before_count': 1,
      'after_count': 1,
      'source': 'tencent',
      'status': 'ok',
      'partial': true,
      'bars': [
        {'date': '2026-09-15', 'open': 10, 'high': 12, 'low': 9, 'close': 11},
        {'date': '2026-09-16', 'open': 11, 'high': 13, 'low': 10, 'close': 12},
        {'date': '2026-09-18', 'open': 12, 'high': 14, 'low': 11, 'close': 13},
      ],
    });
    expect(window.bars.map((e) => e.date).toList(), [
      '2026-09-15',
      '2026-09-16',
      '2026-09-18',
    ]);
    expect(window.anchorIndex, 1);
    expect(window.afterCount, 1);
  });

  test('当天数据过期后重新取数，较老归档继续复用', () async {
    var now = DateTime(2026, 10, 8, 14);
    var calls = 0;
    final repository = RedKlineRepository(
      now: () => now,
      fetchBatch: (date, codes) async {
        calls++;
        return {'date': date, 'items': <String, dynamic>{}};
      },
    );
    await repository.load('2026-10-08', ['000823.XSHE']);
    now = now.add(const Duration(minutes: 6));
    await repository.load('2026-10-08', ['000823.XSHE']);
    expect(calls, 2);
    await repository.load('2025-01-06', ['000823.XSHE']);
    now = now.add(const Duration(hours: 1));
    await repository.load('2025-01-06', ['000823.XSHE']);
    expect(calls, 3);
  });

  test('同一批股票只因列表重新排序不会重复请求', () async {
    var calls = 0;
    final repository = RedKlineRepository(
      fetchBatch: (date, codes) async {
        calls++;
        return {'date': date, 'items': <String, dynamic>{}};
      },
    );
    await repository.load('2026-09-16', ['600121.XSHG', '000823.XSHE']);
    await repository.load('2026-09-16', ['000823.XSHE', '600121.XSHG']);
    expect(calls, 1);
  });

  test('一批最多25只股票并复用同日期请求', () async {
    final calls = <List<String>>[];
    final repository = RedKlineRepository(
      fetchBatch: (date, codes) async {
        calls.add(codes);
        return {
          'date': date,
          'items': {
            for (final code in codes)
              code: {
                'code': code,
                'date': date,
                'bars': [],
                'anchor_index': -1,
                'before_count': 0,
                'after_count': 0,
                'source': 'cache',
                'status': 'unavailable',
                'partial': true,
              },
          },
        };
      },
    );
    final codes = List.generate(26, (i) => i.toString().padLeft(6, '0'));
    final first = await repository.load('2026-09-16', codes);
    final second = await repository.load('2026-09-16', codes);
    expect(first.length, 26);
    expect(second.length, 26);
    expect(calls.map((e) => e.length).toList(), [25, 1]);
  });
}

class _UnusedAuthRepository implements AuthRepository {
  @override
  dynamic noSuchMethod(Invocation invocation) => super.noSuchMethod(invocation);
}

class _NoNetworkRedViewModel extends T0StrategyViewModel {
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

class _SilentVoiceViewModel extends VoiceAnnouncementViewModel {
  @override
  bool get askedBefore => true;
  @override
  // ignore: must_call_super
  void dispose() {}
}
