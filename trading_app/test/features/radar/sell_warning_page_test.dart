import 'package:flutter/material.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:provider/provider.dart';
import 'package:shared_preferences/shared_preferences.dart';
import 'package:trading_app/core/theme/app_colors.dart';
import 'package:trading_app/features/permissions/domain/module_definition.dart';
import 'package:trading_app/features/permissions/presentation/module_permission_controller.dart';
import 'package:trading_app/features/radar/data/radar_repository.dart';
import 'package:trading_app/features/radar/domain/radar_models.dart';
import 'package:trading_app/features/radar/domain/voice_announcement_view_model.dart';
import 'package:trading_app/features/radar/presentation/radar_list/radar_page.dart';
import 'package:trading_app/features/radar/presentation/radar_list/radar_view_model.dart';
import 'package:trading_app/features/radar/presentation/radar_list/t0_strategy_view_model.dart';

void main() {
  testWidgets('监控股票触发卖出信号后显示红色卖出标志', (tester) async {
    SharedPreferences.setMockInitialValues({'voice_announcement_asked': true});
    final radarVm = RadarViewModel(_SellWarningRepository());
    final strategyVm = _NoNetworkT0StrategyViewModel();
    final voiceVm = _TestVoiceAnnouncementViewModel();
    final permissions = ModulePermissionController.forTesting(
      publicModuleDefinitions,
    );
    var disposed = false;
    void disposeAll() {
      if (disposed) return;
      disposed = true;
      radarVm.dispose();
      strategyVm.dispose();
      voiceVm.dispose();
      permissions.dispose();
    }

    addTearDown(() {
      disposeAll();
    });

    await radarVm.loadMonitoredStocks();

    await tester.pumpWidget(
      AppColorsWidget(
        colors: AppColors.instance,
        variant: AppThemeVariant.light,
        child: MultiProvider(
          providers: [
            ChangeNotifierProvider.value(value: radarVm),
            ChangeNotifierProvider<T0StrategyViewModel>.value(
              value: strategyVm,
            ),
            ChangeNotifierProvider<VoiceAnnouncementViewModel>.value(
              value: voiceVm,
            ),
            ChangeNotifierProvider<ModulePermissionController>.value(
              value: permissions,
            ),
          ],
          child: const MaterialApp(home: RadarPage()),
        ),
      ),
    );
    await tester.pump();

    final badge = find.text('卖出');
    expect(badge, findsOneWidget);
    final badgeText = tester.widget<Text>(badge);
    expect(badgeText.style?.color, Colors.white);
    disposeAll();
  });
}

class _SellWarningRepository implements RadarRepository {
  @override
  Future<List<MonitoredStock>> getMonitoredStocks() async => [
    const MonitoredStock(code: 'sz000001', name: '测试股票', date: '2026-09-21'),
  ];

  @override
  Future<Map<String, Map<String, dynamic>>> fetchRealtimeQuotes(
    List<String> codes,
  ) async {
    return {
      'sz000001': {
        'code': 'sz000001',
        'name': '测试股票',
        'price': 9.8,
        'changePercent': -1.0,
        'open': 9.8,
        'preClose': 10.0,
        'high': 9.9,
        'low': 9.7,
        'date': '2026-09-21',
      },
    };
  }

  @override
  Future<List<StockChange>> getLatestChanges(List<String> codes) async => [];

  @override
  Future<List<StockChange>> getAllChanges() async => [];

  @override
  Future<String> addMonitoredStock(MonitoredStock stock) async => 'ok';

  @override
  Future<String> removeMonitoredStock(String code) async => 'ok';

  @override
  Future<List<Map<String, String>>> searchStocks(String keyword) async => [];
}

class _NoNetworkT0StrategyViewModel extends T0StrategyViewModel {
  @override
  Future<void> warmUpIfNeeded({
    String moduleCode = t0MainStrategyModuleCode,
  }) async {}
}

class _TestVoiceAnnouncementViewModel extends VoiceAnnouncementViewModel {
  @override
  bool get askedBefore => true;

  @override
  // ignore: must_call_super
  void dispose() {}
}
