import 'package:flutter/material.dart';
import 'package:flutter/services.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:provider/provider.dart';
import 'package:shared_preferences/shared_preferences.dart';
import 'package:trading_app/features/radar/data/radar_repository.dart';
import 'package:trading_app/features/radar/domain/voice_announcement_view_model.dart';
import 'package:trading_app/features/radar/presentation/radar_list/radar_page.dart';
import 'package:trading_app/features/radar/presentation/radar_list/radar_view_model.dart';
import 'package:trading_app/features/radar/presentation/radar_list/t0_strategy_view_model.dart';
import 'package:trading_app/features/radar/domain/radar_models.dart';
import 'package:trading_app/features/permissions/domain/module_definition.dart';
import 'package:trading_app/features/permissions/presentation/module_permission_controller.dart';

final _allRadarModules = <ModuleDefinition>[
  ...publicModuleDefinitions,
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

ChangeNotifierProvider<ModulePermissionController> _permissionProvider() {
  return ChangeNotifierProvider(
    create: (_) => ModulePermissionController.forTesting(_allRadarModules),
  );
}

void main() {
  testWidgets('主板策略页内容支持文本选择', (tester) async {
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

    expect(find.byType(SelectionArea), findsOneWidget);

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

  testWidgets('涨停破板标签在 Flutter UI 中显示为石皮', (tester) async {
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

    expect(find.text('[石皮]'), findsOneWidget);
    expect(find.text('[涨停破板]'), findsNothing);
    disposeVms();
  });

  testWidgets('紫策只展示最近七个日期且不影响主板策略', (tester) async {
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
          '形态样本数': 2,
          '形态达标率(%)': 50,
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
          '形态样本数': 2,
          '形态达标率(%)': 50,
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
    expect(dateDropdown.items, hasLength(7));

    strategyVm.applyResponseForTest({
      'archived': true,
      'date': '2026-08-27',
      'results': [
        {
          '股票代码': '600001.XSHG',
          '股票名称': '紫策股',
          '形态样本数': 2,
          '形态达标率(%)': 50,
          '形态真亏率(%)': 30,
        },
      ],
    }, moduleCode: t0PurpleStrategyModuleCode);
    await tester.pump();
    dateDropdown = tester.widget<DropdownButton<String>>(
      find.byType(DropdownButton<String>),
    );
    expect(dateDropdown.value, isNull);
    expect(find.text('紫策仅支持最近七天，请选择日期'), findsOneWidget);

    strategyVm.applyResponseForTest({
      'archived': true,
      'date': '2026-08-28',
      'results': [
        {
          '股票代码': '600001.XSHG',
          '股票名称': '紫策股',
          '形态样本数': 2,
          '形态达标率(%)': 50,
          '形态真亏率(%)': 30,
        },
      ],
    }, moduleCode: t0PurpleStrategyModuleCode);
    await tester.pump();
    final purplePreviousButton = tester.widget<TextButton>(
      find.widgetWithText(TextButton, '前一天'),
    );
    expect(purplePreviousButton.onPressed, isNull);

    await tester.tap(find.text('主板策略(1)'));
    await tester.pumpAndSettle();
    dateDropdown = tester.widget<DropdownButton<String>>(
      find.byType(DropdownButton<String>),
    );
    expect(dateDropdown.items, hasLength(8));
    final mainPreviousButton = tester.widget<TextButton>(
      find.widgetWithText(TextButton, '前一天'),
    );
    expect(mainPreviousButton.onPressed, isNotNull);
    disposeVms();
  });

  testWidgets('紫策的涨停破板标签显示为皮', (tester) async {
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
          '形态样本数': 2,
          '形态达标率(%)': 50,
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
          '形态样本数': 2,
          '形态达标率(%)': 50,
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
          '形态样本数': 2,
          '形态达标率(%)': 50,
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
          '形态样本数': 2,
          '形态达标率(%)': 50,
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

  testWidgets('紫策位于主板策略左侧并只显示两个百分比均达标的股票', (tester) async {
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
          '形态样本数': 2,
          '形态达标率(%)': 40.1,
          '形态真亏率(%)': 39.9,
        },
        {'股票代码': '600002.XSHG', '股票名称': '非紫股', '形态达标率(%)': 40, '形态真亏率(%)': 20},
      ],
    });
    strategyVm.applyResponseForTest({
      'date': '2026-09-02',
      'results': [
        {
          '股票代码': '600001.XSHG',
          '股票名称': '紫股',
          '形态样本数': 2,
          '形态达标率(%)': 40.1,
          '形态真亏率(%)': 39.9,
        },
        {'股票代码': '600002.XSHG', '股票名称': '非紫股', '形态达标率(%)': 40, '形态真亏率(%)': 20},
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
