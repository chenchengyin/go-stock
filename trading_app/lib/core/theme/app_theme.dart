import 'package:flutter/material.dart';
import 'app_colors.dart';

/// Material 主题工厂 — 从 AppColors 静态字段读取颜色
class AppTheme {
  /// 基于主题变体和当前 AppColors 静态值构建 ThemeData
  static ThemeData current(AppThemeVariant variant) {
    final brightness = variant == AppThemeVariant.dark
        ? Brightness.dark
        : Brightness.light;
    final onPrimary = brightness == Brightness.dark
        ? const Color(0xffe0e0e0)
        : Colors.white;

    // 手动构建 ColorScheme，将 AppColors 语义颜色映射进去
    // 这样组件中使用 Theme.of(context).colorScheme.error 即可自动跟随主题
    final colorScheme = ColorScheme(
      brightness: brightness,
      primary: AppColors.brand,
      onPrimary: onPrimary,
      secondary: AppColors.brandLight,
      onSecondary: AppColors.textPrimary,
      error: AppColors.error,
      onError: onPrimary,
      surface: AppColors.cardBg,
      onSurface: AppColors.textPrimary,
    );

    return ThemeData(
      useMaterial3: true,
      brightness: brightness,
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
      dividerTheme: DividerThemeData(color: AppColors.divider, thickness: 0.5),
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
