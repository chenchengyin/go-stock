import 'package:flutter/material.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:provider/provider.dart';
import 'package:shared_preferences/shared_preferences.dart';
import 'package:trading_app/core/theme/app_colors.dart';
import 'package:trading_app/core/theme/theme_manager.dart';
import 'package:trading_app/features/radar/data/radar_repository.dart';
import 'package:trading_app/features/radar/domain/voice_announcement_view_model.dart';
import 'package:trading_app/features/radar/presentation/radar_list/radar_page.dart';
import 'package:trading_app/features/radar/presentation/radar_list/radar_view_model.dart';
import 'package:trading_app/features/radar/presentation/radar_list/t0_strategy_view_model.dart';

void main() {
  TestWidgetsFlutterBinding.ensureInitialized();

  testWidgets('耀夜主题的雷达顶栏和 Tab 使用深色语义色', (tester) async {
    SharedPreferences.setMockInitialValues({'voice_announcement_asked': true});
    final themeManager = ThemeManager(variant: AppThemeVariant.dark);
    final radarVm = RadarViewModel(RadarRepositoryImpl());
    final strategyVm = T0StrategyViewModel();
    final voiceVm = VoiceAnnouncementViewModel();
    await tester.pumpWidget(
      AppColorsWidget(
        colors: themeManager.colors,
        variant: themeManager.variant,
        child: MultiProvider(
          providers: [
            ChangeNotifierProvider.value(value: radarVm),
            ChangeNotifierProvider.value(value: strategyVm),
            ChangeNotifierProvider.value(value: voiceVm),
          ],
          child: MaterialApp(
            theme: themeManager.themeData,
            home: const RadarPage(),
          ),
        ),
      ),
    );
    await tester.pump();

    final appBar = tester.widget<AppBar>(find.byType(AppBar).first);
    final tabBar = tester.widget<TabBar>(find.byType(TabBar));
    expect(appBar.backgroundColor, AppColors.appBarBg);
    expect(appBar.backgroundColor, isNot(Colors.white));
    expect(tabBar.labelColor, AppColors.brand);
    expect(tabBar.indicatorColor, AppColors.brand);

    await tester.pump(const Duration(milliseconds: 500));
    radarVm.dispose();
    strategyVm.dispose();
    voiceVm.dispose();
    AppColors.applyLight();
  });
}
