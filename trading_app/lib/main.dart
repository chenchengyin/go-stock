import 'package:flutter/material.dart';

import 'app/app_config.dart';
import 'app/app_shell.dart';
import 'core/theme/app_theme.dart';
import 'features/radar/data/notification_util.dart';

void main() async {
  WidgetsFlutterBinding.ensureInitialized();
  await initNotifications();
  runApp(const TradingRadarApp());
}

class TradingRadarApp extends StatelessWidget {
  const TradingRadarApp({super.key});

  @override
  Widget build(BuildContext context) {
    return AppDependencies(
      child: MaterialApp(
        title: '交易雷达',
        debugShowCheckedModeBanner: false,
        color: Colors.white,
        theme: AppTheme.light(),
        home: const AppShell(),
      ),
    );
  }
}
