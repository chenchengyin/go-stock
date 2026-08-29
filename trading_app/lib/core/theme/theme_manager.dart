import 'package:flutter/material.dart';
import 'package:shared_preferences/shared_preferences.dart';
import 'app_colors.dart';
import 'app_theme.dart';

/// 主题管理 ViewModel — 通过 Provider 注入实现全局响应式主题切换
class ThemeManager extends ChangeNotifier {
  static const String prefsKey = 'app_theme_variant';

  ThemeManager({AppThemeVariant variant = AppThemeVariant.light})
    : _variant = variant {
    AppColors.applyVariant(variant);
  }

  AppThemeVariant _variant;
  AppThemeVariant get variant => _variant;

  AppColors get colors => AppColors.instance;

  ThemeData get themeData => AppTheme.current(variant);

  static AppThemeVariant parseStored(String? raw) {
    switch (raw) {
      case 'dark':
        return AppThemeVariant.dark;
      case 'grey':
        return AppThemeVariant.grey;
      default:
        return AppThemeVariant.light;
    }
  }

  static bool shouldApplyGreyFilter(AppThemeVariant variant) =>
      variant == AppThemeVariant.grey;

  Future<void> restore() async {
    try {
      final prefs = await SharedPreferences.getInstance();
      final next = parseStored(prefs.getString(prefsKey));
      if (next == _variant) {
        AppColors.applyVariant(next);
        return;
      }
      setVariant(next);
    } catch (e, st) {
      debugPrint('ThemeManager.restore failed: $e\n$st');
    }
  }

  void setVariant(AppThemeVariant variant) {
    if (_variant == variant) return;
    _variant = variant;
    AppColors.applyVariant(variant);
    notifyListeners();
    _persist(variant);
  }

  Future<void> _persist(AppThemeVariant variant) async {
    try {
      final prefs = await SharedPreferences.getInstance();
      await prefs.setString(prefsKey, variant.name);
    } catch (e, st) {
      debugPrint('ThemeManager persist failed: $e\n$st');
    }
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
