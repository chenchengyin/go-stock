import 'package:flutter/material.dart';

/// Material 主题（Cupertino 组件不需要 ThemeData, 这里保持 minimal）
class AppTheme {
  static ThemeData light() {
    return ThemeData(
      useMaterial3: true,
      colorScheme: ColorScheme.fromSeed(
        seedColor: const Color(0xff2364aa),
        brightness: Brightness.light,
      ),
      scaffoldBackgroundColor: const Color(0xfff6f8fb),
      appBarTheme: const AppBarTheme(
        centerTitle: false,
        elevation: 0,
        backgroundColor: Color(0xfff6f8fb),
        foregroundColor: Color(0xff172033),
      ),
      cardTheme: CardThemeData(
        color: Colors.white,
        elevation: 0,
        shape: RoundedRectangleBorder(borderRadius: BorderRadius.circular(8)),
      ),
    );
  }
}
