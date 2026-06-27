import 'package:flutter/material.dart';

import 'app/app_config.dart';
import 'app/app_shell.dart';
import 'core/theme/app_theme.dart';

void main() {
  runApp(const TradingRadarApp());
}

class TradingRadarApp extends StatelessWidget {
  const TradingRadarApp({super.key});

  @override
  Widget build(BuildContext context) {
    return AppDependencies(
      child: MaterialApp(
        // title: '交易雷达',
        debugShowCheckedModeBanner: false,
        theme: AppTheme.light(),
        home: const AppShell(),
      ),
    );
  }
}
