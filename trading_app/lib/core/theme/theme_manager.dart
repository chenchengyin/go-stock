import 'package:flutter/material.dart';
import 'app_colors.dart';
import 'app_theme.dart';

/// 主题管理 ViewModel — 通过 Provider 注入实现全局响应式主题切换
class ThemeManager extends ChangeNotifier {
  ThemeManager({AppThemeVariant variant = AppThemeVariant.light})
      : _variant = variant {
    AppColors.applyVariant(variant);
  }

  AppThemeVariant _variant;
  AppThemeVariant get variant => _variant;

  AppColors get colors => AppColors.instance;

  ThemeData get themeData => AppTheme.current();

  void setVariant(AppThemeVariant variant) {
    if (_variant == variant) return;
    _variant = variant;
    AppColors.applyVariant(variant);
    notifyListeners();
  }

  void cycle() {
    switch (_variant) {
      case AppThemeVariant.light:
        setVariant(AppThemeVariant.dark);
      case AppThemeVariant.dark:
        setVariant(AppThemeVariant.grey);
      case AppThemeVariant.grey:
        setVariant(AppThemeVariant.light);
    }
  }
}
