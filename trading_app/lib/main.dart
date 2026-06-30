import 'dart:async';

import 'package:flutter/material.dart';
import 'package:provider/provider.dart';

import 'app/app_config.dart';
import 'app/app_shell.dart';
import 'core/theme/app_colors.dart';
import 'core/theme/theme_manager.dart';
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

  @override
  Widget build(BuildContext context) {
    return AppDependencies(
      child: Consumer<ThemeManager>(
        builder: (context, tm, _) {
          final colors = tm.colors;
          return AppColorsWidget(
            colors: colors,
            child: MaterialApp(
              title: '交易雷达',
              debugShowCheckedModeBanner: false,
              color: Colors.white,
              theme: tm.themeData,
              home: const AppShell(),
            ),
          );
        },
      ),
    );
  }
}
