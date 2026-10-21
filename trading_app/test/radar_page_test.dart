import 'package:flutter/material.dart';
import 'package:flutter/services.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:provider/provider.dart';
import 'package:shared_preferences/shared_preferences.dart';
import 'package:trading_app/core/theme/app_colors.dart';
import 'package:trading_app/features/radar/data/radar_repository.dart';
import 'package:trading_app/features/radar/domain/voice_announcement_view_model.dart';
import 'package:trading_app/features/radar/presentation/radar_list/radar_page.dart';
import 'package:trading_app/features/radar/presentation/radar_list/radar_view_model.dart';
import 'package:trading_app/features/radar/presentation/radar_list/stock_rating_store.dart';
import 'package:trading_app/features/radar/presentation/radar_list/t0_strategy_view_model.dart';
import 'package:trading_app/features/radar/domain/radar_models.dart';
import 'package:trading_app/features/permissions/domain/module_definition.dart';
import 'package:trading_app/features/permissions/presentation/module_permission_controller.dart';

final _allRadarModules = <ModuleDefinition>[
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
  const ModuleDefinition(
    code: 'radar.red_shared_gob_strategy',
    name: '红策',
    client: 'flutter_web',
    placement: 'radar_tab',
    parentCode: null,
    sort: 14,
    accessMode: ModuleAccessMode.userAllowlist,
  ),
  const ModuleDefinition(
    code: 'radar.purple_strategy',
    name: '紫策',
    client: 'flutter_web',
    placement: 'radar_tab',
    parentCode: null,
    sort: 20,
    accessMode: ModuleAccessMode.userAllowlist,
  ),
  const ModuleDefinition(
    code: 'radar.main_strategy',
    name: '主板策略',
    client: 'flutter_web',
    placement: 'radar_tab',
    parentCode: null,
    sort: 30,
    accessMode: ModuleAccessMode.userAllowlist,
  ),
  const ModuleDefinition(
    code: 'radar.blue_strategy',
    name: '蓝策',
    client: 'flutter_web',
    placement: 'radar_tab',
    parentCode: null,
    sort: 40,
    accessMode: ModuleAccessMode.userAllowlist,
  ),
];

ChangeNotifierProvider<ModulePermissionController> _permissionProvider({
  bool includeRegression = false,
}) {
  return ChangeNotifierProvider(
    create: (_) => ModulePermissionController.forTesting([
      ..._allRadarModules,
      if (includeRegression)
        const ModuleDefinition(
          code: 'radar.red_strategy_regression',
          name: '策略回归',
          client: 'flutter_web',
          placement: 'radar_tab',
          parentCode: null,
          sort: 13,
          accessMode: ModuleAccessMode.userAllowlist,
        ),
    ]),
  );
}

void main() {
  for (final size in [const Size(800, 600), const Size(360, 480)]) {
    testWidgets('红策组合切换不显示常驻说明卡 $size', (tester) async {
      tester.view.physicalSize = size;
      tester.view.devicePixelRatio = 1;
      addTearDown(tester.view.resetPhysicalSize);
      addTearDown(tester.view.resetDevicePixelRatio);
      SharedPreferences.setMockInitialValues({
        'voice_announcement_asked': true,
      });
      final radarVm = _NoNetworkRadarViewModel();
      final strategyVm = _RuleHistoryT0StrategyViewModel();
      final voiceVm = _TestVoiceAnnouncementViewModel();
      var disposed = false;
      void disposeVms() {
        if (disposed) return;
        disposed = true;
        radarVm.dispose();
        strategyVm.dispose();
        voiceVm.dispose();
      }

      addTearDown(disposeVms);
      await tester.pumpWidget(
        MultiProvider(
          providers: [
            ChangeNotifierProvider<RadarViewModel>.value(value: radarVm),
            ChangeNotifierProvider<T0StrategyViewModel>.value(
              value: strategyVm,
            ),
            ChangeNotifierProvider<VoiceAnnouncementViewModel>.value(
              value: voiceVm,
            ),
            _permissionProvider(includeRegression: true),
          ],
          child: const MaterialApp(home: RadarPage()),
        ),
      );
      await tester.pump();
      await tester.tap(find.text('策略回归'));
      await tester.pumpAndSettle();
      final dropdownRect = tester.getRect(
        find.byType(DropdownButtonFormField<String>),
      );
      final sortRect = tester.getRect(find.byTooltip('T0日内浮盈：默认排序'));
      expect(sortRect.left, greaterThanOrEqualTo(dropdownRect.right));
      for (final label in ['默认排序', '高到低', '低到高']) {
        await tester.tap(find.byTooltip('T0日内浮盈：$label'));
        await tester.pumpAndSettle();
      }
      expect(strategyVm.redRuleHistorySort, RedRuleHistorySort.original);
      expect(find.byTooltip('T0日内浮盈：默认排序'), findsOneWidget);
      expect(find.text('组合说明'), findsNothing);
      await tester.tap(find.byType(DropdownButtonFormField<String>));
      await tester.pumpAndSettle();
      await tester.tap(find.text('四组阴线 · 0次').last);
      await tester.pumpAndSettle();
      expect(strategyVm.selectedRedRuleHistoryId, strategyVm.options.first.id);
      List<String> visibleStockOrder() {
        final names = ['平开差三', '低开差二', '高开差四'];
        names.sort(
          (a, b) => tester
              .getTopLeft(find.text(a))
              .dy
              .compareTo(tester.getTopLeft(find.text(b)).dy),
        );
        return names;
      }

      expect(visibleStockOrder(), ['平开差三', '低开差二', '高开差四']);
      await tester.tap(find.byTooltip('T0日内浮盈：默认排序'));
      await tester.pumpAndSettle();
      expect(visibleStockOrder(), ['高开差四', '平开差三', '低开差二']);
      await tester.tap(find.byTooltip('T0日内浮盈：高到低'));
      await tester.pumpAndSettle();
      expect(visibleStockOrder(), ['低开差二', '平开差三', '高开差四']);
      await tester.tap(find.byTooltip('T0日内浮盈：低到高'));
      await tester.pumpAndSettle();
      expect(visibleStockOrder(), ['平开差三', '低开差二', '高开差四']);
      expect(find.text(strategyVm.options.first.description), findsNothing);
      expect(tester.takeException(), isNull);
      await tester.tap(find.byType(DropdownButtonFormField<String>));
      await tester.pumpAndSettle();
      expect(find.text('另一组合 · 0次'), findsNothing);
      await tester.tap(find.text('另一组合 · 待加载').last);
      await tester.pumpAndSettle();
      expect(find.text(strategyVm.options.first.description), findsNothing);
      expect(strategyVm.selectedRedRuleHistoryId, strategyVm.options.last.id);
      expect(find.text(strategyVm.options.last.description), findsNothing);
      expect(tester.takeException(), isNull);
      disposeVms();
    });
  }

  testWidgets('周末策略状态显示无数据', (tester) async {
    SharedPreferences.setMockInitialValues({'voice_announcement_asked': true});
    final radarVm = RadarViewModel(RadarRepositoryImpl());
    final strategyVm = _NoNetworkT0StrategyViewModel();
    final voiceVm = _TestVoiceAnnouncementViewModel();
    var disposed = false;
    void disposeVms() {
      if (disposed) return;
      disposed = true;
      radarVm.dispose();
      strategyVm.dispose();
      voiceVm.dispose();
    }

    addTearDown(() {
      disposeVms();
    });

    strategyVm.applyResponseForTest({
      'date': '2026-09-12',
      'no_data': true,
      'results': <dynamic>[],
    });

    await tester.pumpWidget(
      MultiProvider(
        providers: [
          ChangeNotifierProvider.value(value: radarVm),
          ChangeNotifierProvider<T0StrategyViewModel>.value(value: strategyVm),
          ChangeNotifierProvider<VoiceAnnouncementViewModel>.value(
            value: voiceVm,
          ),
          _permissionProvider(),
        ],
        child: MaterialApp(home: const RadarPage()),
      ),
    );
    await tester.pump();
    await tester.tap(find.text('主板策略'));
    await tester.pumpAndSettle();

    expect(find.text('无数据'), findsOneWidget);
    disposeVms();
  });

  testWidgets('主板策略页内容不支持文本选择', (tester) async {
    SharedPreferences.setMockInitialValues({'voice_announcement_asked': true});
    final radarVm = RadarViewModel(RadarRepositoryImpl());
    final strategyVm = T0StrategyViewModel();
    final voiceVm = _TestVoiceAnnouncementViewModel();

    await tester.pumpWidget(
      MultiProvider(
        providers: [
          ChangeNotifierProvider.value(value: radarVm),
          ChangeNotifierProvider.value(value: strategyVm),
          ChangeNotifierProvider<VoiceAnnouncementViewModel>.value(
            value: voiceVm,
          ),
          _permissionProvider(),
        ],
        child: MaterialApp(home: const RadarPage()),
      ),
    );
    await tester.tap(find.byType(Tab).at(2));
    await tester.pumpAndSettle();

    expect(find.byType(SelectionArea), findsNothing);

    // RadarViewModel 启动了周期刷新；测试结束前取消定时器，避免测试框架报未处理定时器。
    radarVm.dispose();
    strategyVm.dispose();
    voiceVm.dispose();
  });

  testWidgets('主板策略条目复制按钮复制纯数字股票代码', (tester) async {
    String? copiedText;
    final messenger =
        TestDefaultBinaryMessengerBinding.instance.defaultBinaryMessenger;
    messenger.setMockMethodCallHandler(SystemChannels.platform, (call) async {
      if (call.method == 'Clipboard.setData') {
        copiedText =
            (call.arguments as Map<dynamic, dynamic>)['text'] as String?;
      }
      if (call.method == 'Clipboard.getData') {
        return {'text': copiedText};
      }
      return null;
    });
    addTearDown(() {
      messenger.setMockMethodCallHandler(SystemChannels.platform, null);
    });

    await tester.pumpWidget(
      MaterialApp(
        home: const Scaffold(body: StockCodeCopyButton(code: '600519.XSHG')),
      ),
    );

    final copyButton = find.byTooltip('复制股票代码', skipOffstage: false);
    expect(copyButton, findsOneWidget);
    await tester.tap(copyButton);
    await tester.pump();

    final clipboard = await Clipboard.getData('text/plain');
    expect(clipboard?.text, '600519');
    expect(find.text('股票代码已复制'), findsOneWidget);
  });

  testWidgets('涨停破板标签在 Flutter UI 中显示为 P', (tester) async {
    SharedPreferences.setMockInitialValues({'voice_announcement_asked': true});
    final radarVm = RadarViewModel(RadarRepositoryImpl());
    final strategyVm = _NoNetworkT0StrategyViewModel();
    final voiceVm = _TestVoiceAnnouncementViewModel();
    var disposed = false;
    void disposeVms() {
      if (disposed) return;
      disposed = true;
      radarVm.dispose();
      strategyVm.dispose();
      voiceVm.dispose();
    }

    addTearDown(() {
      disposeVms();
    });

    strategyVm.applyResponseForTest({
      'date': '2026-09-02',
      'results': [
        {'股票代码': '600001.XSHG', '股票名称': '破板股', '标记': '涨停破板'},
      ],
    });

    await tester.pumpWidget(
      MultiProvider(
        providers: [
          ChangeNotifierProvider.value(value: radarVm),
          ChangeNotifierProvider<T0StrategyViewModel>.value(value: strategyVm),
          ChangeNotifierProvider<VoiceAnnouncementViewModel>.value(
            value: voiceVm,
          ),
          _permissionProvider(),
        ],
        child: MaterialApp(home: const RadarPage()),
      ),
    );
    await tester.pump();

    await tester.tap(find.text('主板策略(1)'));
    await tester.pumpAndSettle();

    expect(find.text('[P]'), findsOneWidget);
    expect(find.text('[涨停破板]'), findsNothing);
    disposeVms();
  });

  testWidgets('命中显示条件的股票名称在四个策略页均显示为红色', (tester) async {
    SharedPreferences.setMockInitialValues({'voice_announcement_asked': true});
    final radarVm = RadarViewModel(RadarRepositoryImpl());
    final strategyVm = _NoNetworkT0StrategyViewModel();
    final voiceVm = _TestVoiceAnnouncementViewModel();
    var disposed = false;
    void disposeVms() {
      if (disposed) return;
      disposed = true;
      radarVm.dispose();
      strategyVm.dispose();
      voiceVm.dispose();
    }

    addTearDown(disposeVms);

    Map<String, dynamic> result() => {
      '股票代码': '600001.XSHG',
      '股票名称': '三连形态股',
      '形态': 'ZT|ZT|PB',
      '命中条件': ['涨停＋涨停＋阳线破板'],
      '形态样本数': 3,
      '形态达标率(%)': 70,
      '形态真亏率(%)': 20,
      '买入信号': 'blue',
    };

    strategyVm.applyResponseForTest({
      'date': '2026-09-02',
      'results': [result()],
    });
    strategyVm.applyResponseForTest({
      'date': '2026-09-02',
      'results': [result()],
    }, moduleCode: t0RedStrategyModuleCode);
    strategyVm.applyResponseForTest({
      'date': '2026-09-02',
      'results': [result()],
    }, moduleCode: t0PurpleStrategyModuleCode);
    strategyVm.applyResponseForTest({
      'date': '2026-09-02',
      'results': [result()],
    }, moduleCode: t0BlueStrategyModuleCode);

    await tester.pumpWidget(
      MultiProvider(
        providers: [
          ChangeNotifierProvider.value(value: radarVm),
          ChangeNotifierProvider<T0StrategyViewModel>.value(value: strategyVm),
          ChangeNotifierProvider<VoiceAnnouncementViewModel>.value(
            value: voiceVm,
          ),
          _permissionProvider(),
        ],
        child: MaterialApp(home: const RadarPage()),
      ),
    );
    await tester.pump();

    for (final tabLabel in ['红策(1)', '紫策(1)', '主板策略(1)', '蓝策(1)']) {
      await tester.tap(find.text(tabLabel));
      await tester.pumpAndSettle();
      final stockName = tester.widget<Text>(find.text('三连形态股'));
      expect(stockName.style?.color, AppColors.error);
    }

    expect(find.byTooltip('命中：涨停＋涨停＋阳线破板'), findsNothing);
    disposeVms();
  });

  testWidgets('满足强势连板条件的股票名称显示为蓝色', (tester) async {
    SharedPreferences.setMockInitialValues({'voice_announcement_asked': true});
    final radarVm = RadarViewModel(RadarRepositoryImpl());
    final strategyVm = _NoNetworkT0StrategyViewModel();
    final voiceVm = _TestVoiceAnnouncementViewModel();
    var disposed = false;
    void disposeVms() {
      if (disposed) return;
      disposed = true;
      radarVm.dispose();
      strategyVm.dispose();
      voiceVm.dispose();
    }

    addTearDown(disposeVms);
    strategyVm.applyResponseForTest({
      'date': '2026-09-02',
      'results': [
        {
          '股票代码': '600001.XSHG',
          '股票名称': '强势连板股',
          '命中条件': [],
          '强势连板': true,
          '重点标红': true,
          'T0参考形态命中': [
            {
              'rule_key': 'v1:strong-continuation',
              'name': '强势连板',
              'research_tier': 'A',
              'strict_win_rate': 70,
              'target_rate': 55,
              'earn_rate': 80,
              'sample_count': 20,
              'manual_rank': 5,
            },
          ],
          '买入信号': 'red',
        },
      ],
    }, moduleCode: t0SharedGobRedStrategyModuleCode);

    await tester.pumpWidget(
      MultiProvider(
        providers: [
          ChangeNotifierProvider.value(value: radarVm),
          ChangeNotifierProvider<T0StrategyViewModel>.value(value: strategyVm),
          ChangeNotifierProvider<VoiceAnnouncementViewModel>.value(
            value: voiceVm,
          ),
          _permissionProvider(),
        ],
        child: MaterialApp(home: const RadarPage()),
      ),
    );
    await tester.pump();

    await tester.tap(find.text('红策(1)'));
    await tester.pumpAndSettle();

    final stockName = tester.widget<Text>(find.text('强势连板股'));
    expect(stockName.style?.color, AppColors.info);
    expect(find.text('强势连板 55/80/20'), findsOneWidget);
    expect(
      find.byTooltip(
        'T0参考形态：强势连板\n'
        '评判：T0开盘→T0收盘（当日价格表现）\n'
        '达标率：55%\n'
        '不亏率（含持平）：80%\n'
        '正收益率：70%\n'
        '达标为收益≥2.5%，正收益为收益>0%\n'
        '样本数：20',
      ),
      findsOneWidget,
    );
    disposeVms();
  });

  testWidgets('红策T+1统计支持实时开盘复用日线开盘统计且不借用旧T0胜率', (tester) async {
    SharedPreferences.setMockInitialValues({'voice_announcement_asked': true});
    final radarVm = RadarViewModel(RadarRepositoryImpl());
    final strategyVm = _NoNetworkT0StrategyViewModel();
    final voiceVm = _TestVoiceAnnouncementViewModel();
    var disposed = false;
    void disposeVms() {
      if (disposed) return;
      disposed = true;
      radarVm.dispose();
      strategyVm.dispose();
      voiceVm.dispose();
    }

    addTearDown(disposeVms);
    strategyVm.applyResponseForTest({
      'date': '2026-09-14',
      'data_incomplete': true,
      'results': [
        {
          '股票代码': '600101.XSHG',
          '股票名称': '双命中股',
          '命中条件': ['中阴→跌停→跌停后低开', '另一形态'],
          '形态达标率(%)': 88,
          'T0参考形态命中': [
            {
              'rule_key': 'old_t0',
              'name': '旧T0形态',
              'description': 'T-3、T-2收盘涨停；T0开盘涨幅0.01%～3%',
              'research_tier': 'A',
              'strict_win_rate': 70,
              'target_rate': 55,
              'earn_rate': 80,
              'sample_count': 20,
              'manual_rank': 1,
            },
          ],
          '红策独立形态命中': [
            {
              'rule_key': 'gob_myin_dt_dt_v1',
              'rule_version': 1,
              'name': '中阴→跌停→跌停后低开',
              'description': 'T-3中阴、T-2和T-1跌停；T0低开区间',
              'entry_source': 'daily_open',
              'exit_horizon': 'T+1_close',
              'stats_status': 'pending',
            },
            {
              'rule_key': 'test_second_v1',
              'rule_version': 1,
              'name': '另一形态',
              'entry_source': 'realtime_open',
              'exit_horizon': 'T+1_close',
              'stats_status': 'verified',
              'stats': {
                'sample_count': 12,
                'profit_win_rate': 75,
                'target_rate': 50,
                'date_start': '2024-01-02',
                'date_end': '2026-09-21',
                'entry_source': 'daily_open',
                'exit_horizon': 'T+1_close',
              },
            },
            {
              'rule_key':
                  'gob_t7_drop_le_22_25_gap_m4_m2_t1_amp_ge_7_1_t0_high_v2',
              'rule_version': 2,
              'name': '七日深跌后低开',
              'entry_source': 'daily_open',
              'exit_horizon': 'T0_high',
              'stats_status': 'verified',
              'stats': {
                'sample_count': 10,
                'profit_win_rate': 90,
                'target_rate': 60,
                'date_start': '2024-01-02',
                'date_end': '2026-09-21',
                'entry_source': 'daily_open',
                'exit_horizon': 'T0_high',
              },
            },
            {
              'rule_key':
                  'gob_t7_drop_le_22_25_gap_m4_m2_t1_amp_ge_7_1_close_pos_le_80_body_v3',
              'rule_version': 3,
              'name': '七日深跌后低开（T−1收盘位置≤80%）',
              'description': 'T-1收盘位置=(收盘−最低)/(最高−最低)≤80%。',
              'entry_source': 'daily_open',
              'exit_horizon': 'T0_T1_body',
              'stats_status': 'verified',
              'stats': {
                'sample_count': 71,
                'profit_win_rate': 50,
                'target_rate': 71.83,
                't0_body_target_rate': 39.44,
                't1_body_target_rate': 42.25,
                'date_start': '2026-01-19',
                'date_end': '2026-09-18',
                'entry_source': 'daily_open',
                'exit_horizon': 'T0_T1_body',
              },
            },
            {
              'rule_key': 'body_without_breakdown',
              'rule_version': 3,
              'name': '实体分项缺失',
              'entry_source': 'daily_open',
              'exit_horizon': 'T0_T1_body',
              'stats_status': 'verified',
              'stats': {
                'sample_count': 71,
                'profit_win_rate': 99,
                'target_rate': 99,
                'entry_source': 'daily_open',
                'exit_horizon': 'T0_T1_body',
              },
            },
          ],
        },
      ],
    }, moduleCode: t0SharedGobRedStrategyModuleCode);

    await tester.pumpWidget(
      MultiProvider(
        providers: [
          ChangeNotifierProvider.value(value: radarVm),
          ChangeNotifierProvider<T0StrategyViewModel>.value(value: strategyVm),
          ChangeNotifierProvider<VoiceAnnouncementViewModel>.value(
            value: voiceVm,
          ),
          _permissionProvider(),
        ],
        child: MaterialApp(home: const RadarPage()),
      ),
    );
    await tester.pump();
    await tester.tap(find.text('红策(1)'));
    await tester.pumpAndSettle();

    expect(find.text('双命中股'), findsOneWidget);
    expect(find.text('中阴→跌停→跌停后低开 · T+1收盘 统计待核验'), findsOneWidget);
    expect(find.text('另一形态 · T+1收盘 真赚75.0% · 达标50.0% · 样本12'), findsOneWidget);
    expect(
      find.text('七日深跌后低开 · T0盘中最高价 浮盈触及90.0% · 目标触及(>2.5%)60.0% · 样本10'),
      findsOneWidget,
    );
    expect(find.textContaining('T0盘中最高价 真赚'), findsNothing);
    expect(
      find.text(
        '七日深跌后低开（T−1收盘位置≤80%） · T0/T+1阳线实体 T0达标39.4% · T1达标42.3% · 任一天71.8% · 样本71',
      ),
      findsOneWidget,
    );
    expect(find.text('实体分项缺失 · T0/T+1阳线实体 统计待核验'), findsOneWidget);
    expect(find.textContaining('T0/T+1阳线实体 真赚'), findsNothing);
    expect(
      find.byWidgetPredicate(
        (widget) =>
            widget is Tooltip &&
            widget.message!.contains('严格>2.5%') &&
            widget.message!.contains('T0实体达标率：39.4%') &&
            widget.message!.contains('任一天实体达标率：71.8%'),
      ),
      findsOneWidget,
    );
    expect(find.text('旧T0形态 55/80/20'), findsOneWidget);
    expect(
      find.byTooltip(
        'T0参考形态：旧T0形态\n'
        '条件说明：T-3、T-2收盘涨停；T0开盘涨幅0.01%～3%\n'
        '评判：T0开盘→T0收盘（当日价格表现）\n'
        '达标率：55%\n'
        '不亏率（含持平）：80%\n'
        '正收益率：70%\n'
        '达标为收益≥2.5%，正收益为收益>0%\n'
        '样本数：20',
      ),
      findsOneWidget,
    );
    expect(
      find.byTooltip(
        '形态：中阴→跌停→跌停后低开\n'
        '条件说明：T-3中阴、T-2和T-1跌停；T0低开区间\n'
        '评判：T+1收盘\n'
        '入场：日线开盘代理\n'
        '统计待核验',
      ),
      findsOneWidget,
    );
    expect(find.text('红策数据不完整，仅显示已确认命中；列表数量不是完整统计'), findsOneWidget);
    disposeVms();
  });

  testWidgets('紫策可浏览两个月内归档交易日且不影响主板策略', (tester) async {
    SharedPreferences.setMockInitialValues({'voice_announcement_asked': true});
    final radarVm = RadarViewModel(RadarRepositoryImpl());
    final strategyVm = _NoNetworkT0StrategyViewModel();
    final voiceVm = _TestVoiceAnnouncementViewModel();
    var disposed = false;
    void disposeVms() {
      if (disposed) return;
      disposed = true;
      radarVm.dispose();
      strategyVm.dispose();
      voiceVm.dispose();
    }

    addTearDown(disposeVms);

    strategyVm.applyAvailableDatesForTest([
      '2026-09-03',
      '2026-09-02',
      '2026-09-01',
      '2026-08-31',
      '2026-08-30',
      '2026-08-29',
      '2026-08-28',
      '2026-08-27',
    ]);
    strategyVm.applyAvailableDatesForTest([
      '2026-09-03',
      '2026-09-02',
      '2026-09-01',
      '2026-08-31',
      '2026-08-30',
      '2026-08-29',
      '2026-08-28',
      '2026-08-27',
    ], moduleCode: t0PurpleStrategyModuleCode);
    strategyVm.applyResponseForTest({
      'date': '2026-09-03',
      'results': [
        {
          '股票代码': '600001.XSHG',
          '股票名称': '紫策股',
          '形态样本数': 3,
          '形态达标率(%)': 70,
          '形态真亏率(%)': 30,
        },
      ],
    });
    strategyVm.applyResponseForTest({
      'date': '2026-09-03',
      'results': [
        {
          '股票代码': '600001.XSHG',
          '股票名称': '紫策股',
          '形态样本数': 3,
          '形态达标率(%)': 70,
          '形态真亏率(%)': 30,
        },
      ],
    }, moduleCode: t0PurpleStrategyModuleCode);

    await tester.pumpWidget(
      MultiProvider(
        providers: [
          ChangeNotifierProvider.value(value: radarVm),
          ChangeNotifierProvider<T0StrategyViewModel>.value(value: strategyVm),
          ChangeNotifierProvider<VoiceAnnouncementViewModel>.value(
            value: voiceVm,
          ),
          _permissionProvider(),
        ],
        child: MaterialApp(home: const RadarPage()),
      ),
    );
    await tester.pump();

    await tester.tap(find.text('紫策(1)'));
    await tester.pumpAndSettle();
    var dateDropdown = tester.widget<DropdownButton<String>>(
      find.byType(DropdownButton<String>),
    );
    expect(dateDropdown.items, hasLength(6));

    strategyVm.applyResponseForTest({
      'archived': true,
      'date': '2026-08-27',
      'results': [
        {
          '股票代码': '600001.XSHG',
          '股票名称': '紫策股',
          '形态样本数': 3,
          '形态达标率(%)': 70,
          '形态真亏率(%)': 30,
        },
      ],
    }, moduleCode: t0PurpleStrategyModuleCode);
    await tester.pump();
    dateDropdown = tester.widget<DropdownButton<String>>(
      find.byType(DropdownButton<String>),
    );
    expect(dateDropdown.value, '2026-08-27');
    expect(find.text('紫策仅支持最近七天，请选择日期'), findsNothing);
    expect(find.text('紫策股'), findsOneWidget);

    strategyVm.applyResponseForTest({
      'archived': true,
      'date': '2026-08-28',
      'results': [
        {
          '股票代码': '600001.XSHG',
          '股票名称': '紫策股',
          '形态样本数': 3,
          '形态达标率(%)': 70,
          '形态真亏率(%)': 30,
        },
      ],
    }, moduleCode: t0PurpleStrategyModuleCode);
    await tester.pump();
    final purplePreviousButton = tester.widget<TextButton>(
      find.widgetWithText(TextButton, '前一天'),
    );
    expect(purplePreviousButton.onPressed, isNotNull);

    await tester.tap(find.text('主板策略(1)'));
    await tester.pumpAndSettle();
    dateDropdown = tester.widget<DropdownButton<String>>(
      find.byType(DropdownButton<String>),
    );
    expect(dateDropdown.items, hasLength(8));
    final mainPreviousButton = tester.widget<TextButton>(
      find.widgetWithText(TextButton, '前一天'),
    );
    final mainNextButton = tester.widget<TextButton>(
      find.widgetWithText(TextButton, '后一天'),
    );
    expect(mainPreviousButton.onPressed, isNotNull);
    final previousLabel = tester.widget<Text>(
      find.descendant(
        of: find.widgetWithText(TextButton, '前一天'),
        matching: find.text('前一天'),
      ),
    );
    final nextLabel = tester.widget<Text>(
      find.descendant(
        of: find.widgetWithText(TextButton, '后一天'),
        matching: find.text('后一天'),
      ),
    );
    expect(previousLabel.style?.fontSize, 16);
    expect(nextLabel.style?.fontSize, 16);
    expect(
      mainPreviousButton.style?.padding?.resolve({}),
      const EdgeInsets.symmetric(horizontal: 10),
    );
    expect(
      mainNextButton.style?.padding?.resolve({}),
      const EdgeInsets.symmetric(horizontal: 10),
    );
    expect(tester.widget<Text>(find.text('当前显示')).style?.fontSize, 16);
    expect(dateDropdown.style?.fontSize, 17);
    expect(tester.widget<Text>(find.text('选股结果')).style?.fontSize, 16);
    disposeVms();
  });

  testWidgets('紫策的涨停破板标签与形态统计在 Flutter UI 中正确显示', (tester) async {
    SharedPreferences.setMockInitialValues({'voice_announcement_asked': true});
    final radarVm = RadarViewModel(RadarRepositoryImpl());
    final strategyVm = _NoNetworkT0StrategyViewModel();
    final voiceVm = _TestVoiceAnnouncementViewModel();
    var disposed = false;
    void disposeVms() {
      if (disposed) return;
      disposed = true;
      radarVm.dispose();
      strategyVm.dispose();
      voiceVm.dispose();
    }

    addTearDown(disposeVms);

    strategyVm.applyResponseForTest({
      'date': '2026-09-03',
      'results': [
        {
          '股票代码': '600001.XSHG',
          '股票名称': '紫策破板股',
          '标记': '涨停破板',
          '形态': 'ZT|ZT|MYIN',
          '形态样本数': 3,
          '形态达标率(%)': 70,
          '形态真亏率(%)': 30,
        },
      ],
    });
    strategyVm.applyResponseForTest({
      'date': '2026-09-03',
      'results': [
        {
          '股票代码': '600001.XSHG',
          '股票名称': '紫策破板股',
          '标记': '涨停破板',
          '形态': 'ZT|ZT|MYIN',
          '形态样本数': 3,
          '形态达标率(%)': 70,
          '形态真亏率(%)': 30,
        },
      ],
    }, moduleCode: t0PurpleStrategyModuleCode);

    await tester.pumpWidget(
      MultiProvider(
        providers: [
          ChangeNotifierProvider.value(value: radarVm),
          ChangeNotifierProvider<T0StrategyViewModel>.value(value: strategyVm),
          ChangeNotifierProvider<VoiceAnnouncementViewModel>.value(
            value: voiceVm,
          ),
          _permissionProvider(),
        ],
        child: MaterialApp(home: const RadarPage()),
      ),
    );
    await tester.pump();

    await tester.tap(find.text('紫策(1)'));
    await tester.pumpAndSettle();

    expect(find.text('[皮]'), findsOneWidget);
    expect(find.text('[石皮]'), findsNothing);
    expect(find.text('70/70/3'), findsOneWidget);
    expect(
      find.byTooltip('形态：ZT|ZT|MYIN（涨停＋涨停＋中阴）\n达标率：70%\n赚率：70%\n样本数：3'),
      findsOneWidget,
    );
    disposeVms();
  });

  testWidgets('策略股票根据真赚率自动显示仓位评级', (tester) async {
    SharedPreferences.setMockInitialValues({
      'voice_announcement_asked': true,
      StockRatingStore.storageKey: '{"600001":"avoid"}',
    });
    final radarVm = RadarViewModel(RadarRepositoryImpl());
    final strategyVm = _NoNetworkT0StrategyViewModel();
    final voiceVm = _TestVoiceAnnouncementViewModel();
    var disposed = false;
    void disposeVms() {
      if (disposed) return;
      disposed = true;
      radarVm.dispose();
      strategyVm.dispose();
      voiceVm.dispose();
    }

    addTearDown(disposeVms);

    strategyVm.applyResponseForTest({
      'date': '2026-09-03',
      'results': [
        {
          '股票代码': '600001.XSHG',
          '股票名称': '评级测试股',
          '形态': 'ZT|ZT|MYIN',
          '形态样本数': 3,
          '形态达标率(%)': 70,
          '形态真亏率(%)': 20,
        },
      ],
    });

    await tester.pumpWidget(
      MultiProvider(
        providers: [
          ChangeNotifierProvider.value(value: radarVm),
          ChangeNotifierProvider<T0StrategyViewModel>.value(value: strategyVm),
          ChangeNotifierProvider<VoiceAnnouncementViewModel>.value(
            value: voiceVm,
          ),
          _permissionProvider(),
        ],
        child: MaterialApp(home: const RadarPage()),
      ),
    );
    await tester.pump();
    await tester.ensureVisible(find.text('主板策略(1)'));
    await tester.tap(find.text('主板策略(1)'));
    await tester.pumpAndSettle();

    expect(find.text('70/80/3'), findsOneWidget);
    expect(find.text('重仓'), findsOneWidget);
    expect(
      find.byTooltip('评级：重仓\n真赚率：80%\n规则：重仓≥75%，轻仓≥55%，不买<55%'),
      findsOneWidget,
    );
    disposeVms();
  });

  testWidgets('窄屏长股票名下评级紧跟形态统计且不溢出', (tester) async {
    await tester.binding.setSurfaceSize(const Size(390, 844));
    addTearDown(() => tester.binding.setSurfaceSize(null));
    SharedPreferences.setMockInitialValues({
      'voice_announcement_asked': true,
      StockRatingStore.storageKey: '{"600001":"light"}',
    });
    final radarVm = RadarViewModel(RadarRepositoryImpl());
    final strategyVm = _NoNetworkT0StrategyViewModel();
    final voiceVm = _TestVoiceAnnouncementViewModel();
    var disposed = false;
    void disposeVms() {
      if (disposed) return;
      disposed = true;
      radarVm.dispose();
      strategyVm.dispose();
      voiceVm.dispose();
    }

    addTearDown(disposeVms);
    strategyVm.applyResponseForTest({
      'date': '2026-09-03',
      'results': [
        {
          '股票代码': '600001.XSHG',
          '股票名称': '这是一个非常长的股票名称',
          'T0开盘涨幅(%)': 1.23,
          'T0收盘涨幅(%)': 2.34,
          '形态': 'ZT|ZT|MYIN',
          '形态样本数': 123,
          '形态达标率(%)': 70,
          '形态真亏率(%)': 30,
        },
      ],
    });

    await tester.pumpWidget(
      MultiProvider(
        providers: [
          ChangeNotifierProvider.value(value: radarVm),
          ChangeNotifierProvider<T0StrategyViewModel>.value(value: strategyVm),
          ChangeNotifierProvider<VoiceAnnouncementViewModel>.value(
            value: voiceVm,
          ),
          _permissionProvider(),
        ],
        child: MaterialApp(home: const RadarPage()),
      ),
    );
    await tester.pump();
    await tester.ensureVisible(find.text('主板策略(1)'));
    await tester.tap(find.text('主板策略(1)'));
    await tester.pumpAndSettle();

    expect(tester.takeException(), isNull);
    expect(
      tester.getTopLeft(find.text('轻仓')).dx,
      greaterThan(tester.getTopRight(find.text('70/70/123')).dx),
    );
    disposeVms();
  });

  testWidgets('紫策隐藏开盘涨幅且主板策略仍显示', (tester) async {
    SharedPreferences.setMockInitialValues({'voice_announcement_asked': true});
    final radarVm = RadarViewModel(RadarRepositoryImpl());
    final strategyVm = _NoNetworkT0StrategyViewModel();
    final voiceVm = _TestVoiceAnnouncementViewModel();
    var disposed = false;
    void disposeVms() {
      if (disposed) return;
      disposed = true;
      radarVm.dispose();
      strategyVm.dispose();
      voiceVm.dispose();
    }

    addTearDown(disposeVms);

    strategyVm.applyResponseForTest({
      'date': '2026-09-03',
      'results': [
        {
          '股票代码': '600001.XSHG',
          '股票名称': '紫策涨幅股',
          'T0开盘涨幅(%)': 1.23,
          'T0收盘涨幅(%)': 2.34,
          '形态样本数': 3,
          '形态达标率(%)': 70,
          '形态真亏率(%)': 30,
        },
      ],
    });
    strategyVm.applyResponseForTest({
      'date': '2026-09-03',
      'results': [
        {
          '股票代码': '600001.XSHG',
          '股票名称': '紫策涨幅股',
          'T0开盘涨幅(%)': 1.23,
          'T0收盘涨幅(%)': 2.34,
          '形态样本数': 3,
          '形态达标率(%)': 70,
          '形态真亏率(%)': 30,
        },
      ],
    }, moduleCode: t0PurpleStrategyModuleCode);

    await tester.pumpWidget(
      MultiProvider(
        providers: [
          ChangeNotifierProvider.value(value: radarVm),
          ChangeNotifierProvider<T0StrategyViewModel>.value(value: strategyVm),
          ChangeNotifierProvider<VoiceAnnouncementViewModel>.value(
            value: voiceVm,
          ),
          _permissionProvider(),
        ],
        child: MaterialApp(home: const RadarPage()),
      ),
    );
    await tester.pump();

    await tester.tap(find.text('紫策(1)'));
    await tester.pumpAndSettle();
    expect(find.text('开盘+1.23%'), findsNothing);

    await tester.tap(find.text('主板策略(1)'));
    await tester.pumpAndSettle();
    expect(find.text('开盘+1.23%'), findsOneWidget);
    disposeVms();
  });

  testWidgets('蓝策位于主板策略右侧并显示蓝灯数量', (tester) async {
    SharedPreferences.setMockInitialValues({'voice_announcement_asked': true});
    final radarVm = RadarViewModel(RadarRepositoryImpl());
    final strategyVm = _NoNetworkT0StrategyViewModel();
    final voiceVm = _TestVoiceAnnouncementViewModel();
    var disposed = false;
    void disposeVms() {
      if (disposed) return;
      disposed = true;
      radarVm.dispose();
      strategyVm.dispose();
      voiceVm.dispose();
    }

    addTearDown(disposeVms);

    strategyVm.applyResponseForTest({
      'date': '2026-09-02',
      'results': [
        {'股票代码': '600001.XSHG', '股票名称': '蓝股', '买入信号': 'blue'},
        {'股票代码': '600002.XSHG', '股票名称': '绿股', '买入信号': 'green'},
      ],
    });
    strategyVm.applyResponseForTest({
      'date': '2026-09-02',
      'results': [
        {'股票代码': '600001.XSHG', '股票名称': '蓝股', '买入信号': 'blue'},
        {'股票代码': '600002.XSHG', '股票名称': '绿股', '买入信号': 'green'},
      ],
    }, moduleCode: t0BlueStrategyModuleCode);

    await tester.pumpWidget(
      MultiProvider(
        providers: [
          ChangeNotifierProvider.value(value: radarVm),
          ChangeNotifierProvider<T0StrategyViewModel>.value(value: strategyVm),
          ChangeNotifierProvider<VoiceAnnouncementViewModel>.value(
            value: voiceVm,
          ),
          _permissionProvider(),
        ],
        child: MaterialApp(home: const RadarPage()),
      ),
    );
    await tester.pump();

    final tabs = tester.widgetList<Tab>(find.byType(Tab)).toList();
    expect(tabs.map((tab) => tab.text).toList(), [
      '监控股票(自选)',
      '紫策',
      '主板策略(2)',
      '蓝策(1)',
      '自选异动',
      '全市场',
    ]);
    disposeVms();
  });

  testWidgets('紫策位于主板策略左侧并只显示严格阈值达标的股票', (tester) async {
    SharedPreferences.setMockInitialValues({'voice_announcement_asked': true});
    final radarVm = RadarViewModel(RadarRepositoryImpl());
    final strategyVm = _NoNetworkT0StrategyViewModel();
    final voiceVm = _TestVoiceAnnouncementViewModel();
    var disposed = false;
    void disposeVms() {
      if (disposed) return;
      disposed = true;
      radarVm.dispose();
      strategyVm.dispose();
      voiceVm.dispose();
    }

    addTearDown(disposeVms);

    strategyVm.applyResponseForTest({
      'date': '2026-09-02',
      'results': [
        {
          '股票代码': '600001.XSHG',
          '股票名称': '紫股',
          '形态样本数': 3,
          '形态达标率(%)': 70,
          '形态真亏率(%)': 39.9,
        },
        {
          '股票代码': '600002.XSHG',
          '股票名称': '非紫股',
          '形态达标率(%)': 29.9,
          '形态真亏率(%)': 20,
        },
      ],
    });
    strategyVm.applyResponseForTest({
      'date': '2026-09-02',
      'results': [
        {
          '股票代码': '600001.XSHG',
          '股票名称': '紫股',
          '形态样本数': 3,
          '形态达标率(%)': 70,
          '形态真亏率(%)': 39.9,
        },
        {
          '股票代码': '600002.XSHG',
          '股票名称': '非紫股',
          '形态达标率(%)': 29.9,
          '形态真亏率(%)': 20,
        },
      ],
    }, moduleCode: t0PurpleStrategyModuleCode);

    await tester.pumpWidget(
      MultiProvider(
        providers: [
          ChangeNotifierProvider.value(value: radarVm),
          ChangeNotifierProvider<T0StrategyViewModel>.value(value: strategyVm),
          ChangeNotifierProvider<VoiceAnnouncementViewModel>.value(
            value: voiceVm,
          ),
          _permissionProvider(),
        ],
        child: MaterialApp(home: const RadarPage()),
      ),
    );
    await tester.pump();

    final tabs = tester.widgetList<Tab>(find.byType(Tab)).toList();
    expect(tabs.map((tab) => tab.text).toList(), [
      '监控股票(自选)',
      '紫策(1)',
      '主板策略(2)',
      '蓝策',
      '自选异动',
      '全市场',
    ]);

    await tester.tap(find.text('紫策(1)'));
    await tester.pumpAndSettle();

    expect(strategyVm.loadAvailableDatesCalls, 1);
    expect(strategyVm.loadResultsCalls, 1);
    expect(find.text('紫股'), findsOneWidget);
    expect(find.text('非紫股'), findsNothing);
    disposeVms();
  });

  testWidgets('蓝策只显示蓝灯股票并保留策略卡片行为', (tester) async {
    SharedPreferences.setMockInitialValues({'voice_announcement_asked': true});
    final radarVm = RadarViewModel(RadarRepositoryImpl());
    final strategyVm = _NoNetworkT0StrategyViewModel();
    final voiceVm = _TestVoiceAnnouncementViewModel();
    var disposed = false;
    void disposeVms() {
      if (disposed) return;
      disposed = true;
      radarVm.dispose();
      strategyVm.dispose();
      voiceVm.dispose();
    }

    addTearDown(disposeVms);

    strategyVm.applyResponseForTest({
      'date': '2026-09-02',
      'results': [
        {'股票代码': '600001.XSHG', '股票名称': '蓝股', '买入信号': 'blue'},
        {'股票代码': '600002.XSHG', '股票名称': '绿股', '买入信号': 'green'},
      ],
    });
    strategyVm.applyResponseForTest({
      'date': '2026-09-02',
      'results': [
        {'股票代码': '600001.XSHG', '股票名称': '蓝股', '买入信号': 'blue'},
        {'股票代码': '600002.XSHG', '股票名称': '绿股', '买入信号': 'green'},
      ],
    }, moduleCode: t0BlueStrategyModuleCode);

    await tester.pumpWidget(
      MultiProvider(
        providers: [
          ChangeNotifierProvider.value(value: radarVm),
          ChangeNotifierProvider<T0StrategyViewModel>.value(value: strategyVm),
          ChangeNotifierProvider<VoiceAnnouncementViewModel>.value(
            value: voiceVm,
          ),
          _permissionProvider(),
        ],
        child: MaterialApp(home: const RadarPage()),
      ),
    );
    await tester.pump();

    await tester.tap(find.text('蓝策(1)'));
    await tester.pumpAndSettle();

    expect(strategyVm.loadAvailableDatesCalls, 1);
    expect(strategyVm.loadResultsCalls, 1);
    expect(find.text('蓝股'), findsOneWidget);
    expect(find.text('绿股'), findsNothing);
    expect(find.byTooltip('复制股票代码'), findsOneWidget);

    await tester.tap(find.text('主板策略(2)'));
    await tester.pumpAndSettle();
    await tester.tap(find.text('蓝策(1)'));
    await tester.pumpAndSettle();
    expect(strategyVm.loadAvailableDatesCalls, 2);
    expect(strategyVm.loadResultsCalls, 2);
    disposeVms();
  });

  testWidgets('切换进入蓝策也只初始化一次', (tester) async {
    SharedPreferences.setMockInitialValues({'voice_announcement_asked': true});
    final radarVm = RadarViewModel(RadarRepositoryImpl());
    final strategyVm = _NoNetworkT0StrategyViewModel();
    final voiceVm = _TestVoiceAnnouncementViewModel();
    var disposed = false;
    void disposeVms() {
      if (disposed) return;
      disposed = true;
      radarVm.dispose();
      strategyVm.dispose();
      voiceVm.dispose();
    }

    addTearDown(disposeVms);

    strategyVm.applyResponseForTest({
      'date': '2026-09-02',
      'results': [
        {'股票代码': '600001.XSHG', '股票名称': '蓝股', '买入信号': 'blue'},
      ],
    });
    strategyVm.applyResponseForTest({
      'date': '2026-09-02',
      'results': [
        {'股票代码': '600001.XSHG', '股票名称': '蓝股', '买入信号': 'blue'},
      ],
    }, moduleCode: t0BlueStrategyModuleCode);

    await tester.pumpWidget(
      MultiProvider(
        providers: [
          ChangeNotifierProvider.value(value: radarVm),
          ChangeNotifierProvider<T0StrategyViewModel>.value(value: strategyVm),
          ChangeNotifierProvider<VoiceAnnouncementViewModel>.value(
            value: voiceVm,
          ),
          _permissionProvider(),
        ],
        child: MaterialApp(home: const RadarPage()),
      ),
    );
    await tester.pump();

    final tabBarView = find.byType(TabBarView);
    tester.widget<TabBarView>(tabBarView).controller!.animateTo(3);
    await tester.pumpAndSettle();

    expect(find.text('蓝股'), findsOneWidget);
    expect(strategyVm.loadAvailableDatesCalls, 1);
    expect(strategyVm.loadResultsCalls, 1);
    disposeVms();
  });

  testWidgets('自选异动下拉刷新仍调用自选异动接口', (tester) async {
    SharedPreferences.setMockInitialValues({'voice_announcement_asked': true});
    final radarVm = _NoNetworkRadarViewModel();
    final strategyVm = _NoNetworkT0StrategyViewModel();
    final voiceVm = _TestVoiceAnnouncementViewModel();
    var disposed = false;
    void disposeVms() {
      if (disposed) return;
      disposed = true;
      radarVm.dispose();
      strategyVm.dispose();
      voiceVm.dispose();
    }

    addTearDown(disposeVms);

    radarVm.watchChanges = List.generate(
      20,
      (index) => StockChange(
        id: index + 1,
        changeTime: '09:${(30 + index).toString().padLeft(2, '0')}:00',
        changeDate: '2026-09-02',
        stockCode: 'sh600001',
        stockName: '测试股',
        changeType: 1,
        typeName: '测试异动',
        price: 10,
        changeRate: 1,
        volume: 100,
        amount: 1000,
      ),
    );

    await tester.pumpWidget(
      MultiProvider(
        providers: [
          ChangeNotifierProvider<RadarViewModel>.value(value: radarVm),
          ChangeNotifierProvider<T0StrategyViewModel>.value(value: strategyVm),
          ChangeNotifierProvider<VoiceAnnouncementViewModel>.value(
            value: voiceVm,
          ),
          _permissionProvider(),
        ],
        child: MaterialApp(home: const RadarPage()),
      ),
    );
    await tester.pump();

    await tester.tap(find.text('自选异动'));
    await tester.pumpAndSettle();
    final refreshIndicator = tester.widget<RefreshIndicator>(
      find.byType(RefreshIndicator).first,
    );
    await refreshIndicator.onRefresh();
    await tester.pumpAndSettle();

    expect(radarVm.watchChangesCalls, 2);
    expect(radarVm.allChangesCalls, 0);
    disposeVms();
  });
}

class _TestVoiceAnnouncementViewModel extends VoiceAnnouncementViewModel {
  @override
  bool get askedBefore => true;

  // The test does not install the flutter_tts platform channel.
  @override
  // ignore: must_call_super
  void dispose() {}
}

class _NoNetworkT0StrategyViewModel extends T0StrategyViewModel {
  _NoNetworkT0StrategyViewModel({super.request});

  int loadAvailableDatesCalls = 0;
  int loadResultsCalls = 0;

  @override
  Future<void> warmUpIfNeeded({
    String moduleCode = t0MainStrategyModuleCode,
  }) async {}

  @override
  Future<void> loadAvailableDates({
    String moduleCode = t0MainStrategyModuleCode,
  }) async {
    loadAvailableDatesCalls++;
  }

  @override
  Future<void> loadResults({
    String moduleCode = t0MainStrategyModuleCode,
    String? date,
    bool archived = false,
  }) async {
    loadResultsCalls++;
  }
}

class _RuleHistoryT0StrategyViewModel extends _NoNetworkT0StrategyViewModel {
  _RuleHistoryT0StrategyViewModel()
    : super(
        request: (_, _) async => {
          'results': [
            {
              '时间': '2026-09-10',
              '股票代码': '600001.XSHG',
              '股票名称': '平开差三',
              'T0开盘涨幅(%)': 0,
              'T0收盘涨幅(%)': 3,
              't0_intraday_change_pp': 3,
            },
            {
              '时间': '2026-09-10',
              '股票代码': '600002.XSHG',
              '股票名称': '低开差二',
              'T0开盘涨幅(%)': -3,
              'T0收盘涨幅(%)': -1,
              't0_intraday_change_pp': 2,
            },
            {
              '时间': '2026-09-10',
              '股票代码': '600003.XSHG',
              '股票名称': '高开差四',
              'T0开盘涨幅(%)': 5,
              'T0收盘涨幅(%)': 9,
              't0_intraday_change_pp': 4,
            },
          ],
        },
      );

  final options = [
    T0RedRuleHistoryOption(
      ruleSource: 'gob',
      ruleKey: 'four_groups',
      ruleVersion: 1,
      name: '四组阴线',
      description:
          '形态条件\n${List.filled(20, 'T-2收盘涨停，T-1收阴；第3组实体>0%且<3%。').join('\n')}\n\nT0开盘\n0.01%～3%，包含两端。\n\n统计口径\n按T+1收盘评判。',
      matchCount: 0,
    ),
    const T0RedRuleHistoryOption(
      ruleSource: 'gob',
      ruleKey: 'another',
      ruleVersion: 1,
      name: '另一组合',
      description: '形态条件\nT-1振幅≥7.1%。\n\nT0开盘\n−4%≤涨幅<−2%。\n\n统计口径\n仅计算盘中浮盈触及。',
      matchCount: 0,
      matchCountKnown: false,
    ),
  ];
  String? _selectedId;

  @override
  List<T0RedRuleHistoryOption> get redRuleHistoryOptions => options;

  @override
  String? get selectedRedRuleHistoryId => _selectedId;

  @override
  Future<void> loadRedRuleHistoryOptions({bool force = false}) async {}

  @override
  Future<void> loadRedRuleHistoryStocks(T0RedRuleHistoryOption option) async {
    _selectedId = option.id;
    await super.loadRedRuleHistoryStocks(option);
  }
}

class _NoNetworkRadarViewModel extends RadarViewModel {
  _NoNetworkRadarViewModel() : super(RadarRepositoryImpl());

  int watchChangesCalls = 0;
  int allChangesCalls = 0;

  @override
  Future<void> loadWatchChanges() async {
    watchChangesCalls++;
  }

  @override
  Future<void> loadAllChanges() async {
    allChangesCalls++;
  }
}
