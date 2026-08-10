import 'package:flutter/material.dart';
import 'app_colors.dart';

/// Material 主题工厂 — 从 AppColors 静态字段读取颜色
class AppTheme {
  /// 基于当前 AppColors 静态值构建 ThemeData
  static ThemeData current() {
    // 手动构建 ColorScheme，将 AppColors 语义颜色映射进去
    // 这样组件中使用 Theme.of(context).colorScheme.error 即可自动跟随主题
    final colorScheme = ColorScheme(
      brightness: Brightness.light,
      primary: AppColors.brand,
      onPrimary: Colors.white,
      secondary: AppColors.brandLight,
      onSecondary: AppColors.textPrimary,
      error: AppColors.error,
      onError: Colors.white,
      surface: AppColors.cardBg,
      onSurface: AppColors.textPrimary,
    );

    return ThemeData(
      useMaterial3: true,
      colorScheme: colorScheme,
      scaffoldBackgroundColor: AppColors.scaffoldBg,
      appBarTheme: AppBarTheme(
        centerTitle: false,
        elevation: 0.5,
        backgroundColor: AppColors.appBarBg,
        foregroundColor: AppColors.appBarFg,
      ),
      cardTheme: CardThemeData(
        color: AppColors.cardBg,
        elevation: 0,
        shape: RoundedRectangleBorder(borderRadius: BorderRadius.circular(8)),
      ),
      dividerTheme: DividerThemeData(
        color: AppColors.divider,
        thickness: 0.5,
      ),
      inputDecorationTheme: InputDecorationTheme(
        filled: true,
        fillColor: AppColors.inputFill,
        border: OutlineInputBorder(
          borderRadius: BorderRadius.circular(12),
          borderSide: BorderSide(color: AppColors.border),
        ),
      ),
    );
  }
}
