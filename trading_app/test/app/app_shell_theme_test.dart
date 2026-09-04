import 'package:flutter/material.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:provider/provider.dart';
import 'package:shared_preferences/shared_preferences.dart';
import 'package:trading_app/app/app_shell.dart';
import 'package:trading_app/core/theme/app_colors.dart';
import 'package:trading_app/core/theme/theme_manager.dart';
import 'package:trading_app/features/permissions/domain/module_definition.dart';
import 'package:trading_app/features/permissions/data/module_permission_repository.dart';
import 'package:trading_app/features/permissions/presentation/module_permission_controller.dart';
import 'package:trading_app/features/radar/data/radar_repository.dart';
import 'package:trading_app/features/radar/domain/voice_announcement_view_model.dart';
import 'package:trading_app/features/radar/presentation/radar_list/radar_view_model.dart';
import 'package:trading_app/features/radar/presentation/radar_list/t0_strategy_view_model.dart';

void main() {
  TestWidgetsFlutterBinding.ensureInitialized();

  testWidgets('耀夜主题的底部导航使用深色语义色', (tester) async {
    SharedPreferences.setMockInitialValues({'voice_announcement_asked': true});
    final themeManager = ThemeManager(variant: AppThemeVariant.dark);
    final radarVm = RadarViewModel(RadarRepositoryImpl());
    final strategyVm = T0StrategyViewModel();
    final voiceVm = _TestVoiceAnnouncementViewModel();
    await tester.pumpWidget(
      AppColorsWidget(
        colors: themeManager.colors,
        variant: themeManager.variant,
        child: MultiProvider(
          providers: [
            ChangeNotifierProvider.value(value: radarVm),
            ChangeNotifierProvider.value(value: strategyVm),
            ChangeNotifierProvider<VoiceAnnouncementViewModel>.value(
              value: voiceVm,
            ),
            ChangeNotifierProvider(
              create: (_) => ModulePermissionController.forTesting(
                publicModuleDefinitions,
              ),
            ),
          ],
          child: MaterialApp(
            theme: themeManager.themeData,
            home: const AppShell(),
          ),
        ),
      ),
    );
    await tester.pump(const Duration(milliseconds: 500));

    final navigationBar = tester.widget<BottomNavigationBar>(
      find.byType(BottomNavigationBar),
    );
    expect(navigationBar.backgroundColor, AppColors.cardBg);
    expect(navigationBar.selectedItemColor, AppColors.brand);
    expect(navigationBar.backgroundColor, isNot(Colors.white));

    radarVm.dispose();
    strategyVm.dispose();
    voiceVm.dispose();
    AppColors.applyLight();
  });

  testWidgets('回到前台时刷新模块权限', (tester) async {
    final source = _CountingModulePermissionSource();
    final permissions = ModulePermissionController(source);
    final radarVm = RadarViewModel(RadarRepositoryImpl());
    final strategyVm = T0StrategyViewModel();
    final voiceVm = _TestVoiceAnnouncementViewModel();

    await tester.pumpWidget(
      AppColorsWidget(
        colors: AppColors.instance,
        variant: AppThemeVariant.light,
        child: MultiProvider(
          providers: [
            ChangeNotifierProvider.value(value: radarVm),
            ChangeNotifierProvider.value(value: strategyVm),
            ChangeNotifierProvider<VoiceAnnouncementViewModel>.value(
              value: voiceVm,
            ),
            ChangeNotifierProvider<ModulePermissionController>.value(
              value: permissions,
            ),
          ],
          child: const MaterialApp(home: AppShell()),
        ),
      ),
    );
    await tester.pump(const Duration(milliseconds: 500));

    tester.binding.handleAppLifecycleStateChanged(AppLifecycleState.inactive);
    tester.binding.handleAppLifecycleStateChanged(AppLifecycleState.resumed);
    await tester.pump();

    expect(source.fetchCalls, 1);

    radarVm.dispose();
    strategyVm.dispose();
    voiceVm.dispose();
    permissions.dispose();
  });
}

class _CountingModulePermissionSource implements ModulePermissionSource {
  int fetchCalls = 0;

  @override
  Future<ModulePermissionSnapshot> fetchModules() async {
    fetchCalls++;
    return const ModulePermissionSnapshot(
      version: 1,
      modules: publicModuleDefinitions,
    );
  }
}

class _TestVoiceAnnouncementViewModel extends VoiceAnnouncementViewModel {
  @override
  bool get askedBefore => true;

  // The test does not install the flutter_tts platform channel.
  @override
  // ignore: must_call_super
  void dispose() {}
}
