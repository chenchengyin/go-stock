import 'package:flutter/material.dart';
import 'package:provider/provider.dart';

import 'app/app_config.dart';
import 'app/app_shell.dart';
import 'core/theme/app_colors.dart';
import 'core/theme/theme_manager.dart';
import 'features/auth/presentation/auth_gate.dart';
import 'features/radar/data/notification_util.dart';

void main() async {
  WidgetsFlutterBinding.ensureInitialized();
  await initNotifications(onTap: _handleNotificationTap);
  runApp(const TradingRadarApp());
}

void _handleNotificationTap(Map<String, dynamic> data) {
  final type = data['type'] as String?;
  if (type == 'stock_change_detail') {
    final stockCode = data['stockCode'] as String?;
    final stockName = data['stockName'] as String?;
    if (stockCode != null && stockName != null) {
      _navigateToStockDetail(stockCode, stockName);
    }
  }
}

void _navigateToStockDetail(String stockCode, String stockName) {
  final navigatorKey = AppShell.navigatorKey;
  if (navigatorKey.currentState != null) {
    AppShell.navigateToStockDetail(stockCode, stockName);
  } else {
    setPendingDeepLink(stockCode, stockName);
  }
}

class TradingRadarApp extends StatelessWidget {
  const TradingRadarApp({super.key});

  /// 灰度滤镜矩阵 — 将所有颜色转为灰度
  static const _grayScaleMatrix = <double>[
    0.2126,
    0.7152,
    0.0722,
    0,
    0,
    0.2126,
    0.7152,
    0.0722,
    0,
    0,
    0.2126,
    0.7152,
    0.0722,
    0,
    0,
    0,
    0,
    0,
    1,
    0,
  ];

  static Widget wrapWithGreyFilter(AppThemeVariant variant, Widget child) {
    if (!ThemeManager.shouldApplyGreyFilter(variant)) {
      return child;
    }
    return ColorFiltered(
      colorFilter: const ColorFilter.matrix(_grayScaleMatrix),
      child: child,
    );
  }

  @override
  Widget build(BuildContext context) {
    return AppDependencies(
      child: Consumer<ThemeManager>(
        builder: (context, tm, _) {
          final colors = tm.colors;
          Widget app = AppColorsWidget(
            colors: colors,
            variant: tm.variant,
            child: MaterialApp(
              title: '交易雷达',
              debugShowCheckedModeBanner: false,
              color: AppColors.scaffoldBg,
              theme: tm.themeData,
              home: const AuthGate(),
            ),
          );

          // 灰色主题：全局加灰度滤镜，所有组件（包括硬编码红色）统一变灰
          app = wrapWithGreyFilter(tm.variant, app);

          return app;
        },
      ),
    );
  }
}
