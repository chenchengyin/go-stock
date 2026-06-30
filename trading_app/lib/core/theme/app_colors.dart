import 'package:flutter/material.dart';

// ---------------------------------------------------------------------------
// 语义化颜色定义 — 支持 light / dark / grey 三种主题
// 使用方式：
//   AppColors.of(context).textPriceUp   — 需要 context（响应式）
//   AppColors.textPriceUp               — 不需要 context，直接全局访问
//   组件内部直接静态调用，ThemeManager 切换时自动更新
// ---------------------------------------------------------------------------

/// 主题变体
enum AppThemeVariant { light, dark, grey }

/// 全局颜色管理类 — 所有颜色字段均为 `static`，切换主题时自动更新。
class AppColors {
  AppColors._(); // 禁止实例化

  // ── 品牌 ──────────────────────────────────────────────
  static Color brand = _light.brand;
  static Color brandLight = _light.brandLight;

  /// 主按钮颜色（大按钮统一使用此常量）
  static Color get buttonPrimary => brand;

  // ── 背景 ──────────────────────────────────────────────
  static Color scaffoldBg = _light.scaffoldBg;
  static Color cardBg = _light.cardBg;
  static Color backgroundColor = _light.backgroundColor;
  static Color surfaceBg = _light.surfaceBg;

  // ── 文字 ──────────────────────────────────────────────
  static Color textPrimary = _light.textPrimary;
  static Color textSecondary = _light.textSecondary;
  static Color textTertiary = _light.textTertiary;

  // ── 价格涨跌 ──────────────────────────────────────────
  static Color textPriceUp = _light.textPriceUp;
  static Color textPriceDown = _light.textPriceDown;

  // ── 涨跌背景色块 ──────────────────────────────────────
  static Color stockUpBg = _light.stockUpBg;
  static Color stockDownBg = _light.stockDownBg;

  // ── 分割线 / 边框 / 输入框 ────────────────────────────
  static Color divider = _light.divider;
  static Color border = _light.border;
  static Color appBarBg = _light.appBarBg;
  static Color appBarFg = _light.appBarFg;
  static Color inputFill = _light.inputFill;

  // ── 语义色 ────────────────────────────────────────────
  static Color warning = _light.warning;
  static Color success = _light.success;
  static Color error = _light.error;
  static Color info = _light.info;
  static Color link = _light.link;

  // ── 交互状态 ──────────────────────────────────────────
  static Color disabled = _light.disabled;

  // ── 骨架屏 / 加载 ─────────────────────────────────────
  static Color shimmerBase = _light.shimmerBase;
  static Color shimmerHighlight = _light.shimmerHighlight;

  // ── 蒙层 ──────────────────────────────────────────────
  static Color overlay = _light.overlay;

  // ── 标签色（异动类型用） ─────────────────────────────
  static Color tagRed = _light.tagRed;
  static Color tagOrange = _light.tagOrange;
  static Color tagGreen = _light.tagGreen;
  static Color tagBlue = _light.tagBlue;
  static Color tagPurple = _light.tagPurple;

  // ── 主题切换 ──────────────────────────────────────────
  /// 应用浅色主题
  static void applyLight() => _apply(_light);

  /// 应用深色主题
  static void applyDark() => _apply(_dark);

  /// 应用灰色主题
  static void applyGrey() => _apply(_grey);

  static void applyVariant(AppThemeVariant v) {
    switch (v) {
      case AppThemeVariant.light:
        applyLight();
      case AppThemeVariant.dark:
        applyDark();
      case AppThemeVariant.grey:
        applyGrey();
    }
  }

  static void _apply(_Palette p) {
    brand = p.brand;
    brandLight = p.brandLight;
    scaffoldBg = p.scaffoldBg;
    cardBg = p.cardBg;
    backgroundColor = p.backgroundColor;
    surfaceBg = p.surfaceBg;
    textPrimary = p.textPrimary;
    textSecondary = p.textSecondary;
    textTertiary = p.textTertiary;
    textPriceUp = p.textPriceUp;
    textPriceDown = p.textPriceDown;
    stockUpBg = p.stockUpBg;
    stockDownBg = p.stockDownBg;
    divider = p.divider;
    border = p.border;
    appBarBg = p.appBarBg;
    appBarFg = p.appBarFg;
    inputFill = p.inputFill;
    warning = p.warning;
    success = p.success;
    error = p.error;
    info = p.info;
    link = p.link;
    disabled = p.disabled;
    shimmerBase = p.shimmerBase;
    shimmerHighlight = p.shimmerHighlight;
    overlay = p.overlay;
    tagRed = p.tagRed;
    tagOrange = p.tagOrange;
    tagGreen = p.tagGreen;
    tagBlue = p.tagBlue;
    tagPurple = p.tagPurple;
  }

  // ── 快捷获取（响应式） ────────────────────────────────
  /// 从 InheritedWidget 获取当前颜色（响应式跟随主题切换）
  static AppColors of(BuildContext context) {
    return maybeOf(context) ?? AppColors.instance;
  }

  static AppColors? maybeOf(BuildContext context) {
    return context
        .dependOnInheritedWidgetOfExactType<AppColorsWidget>()
        ?.colors;
  }

  /// 单例，用于 of(context) 的后备
  static final AppColors instance = AppColors._();

  // ═══════════════════════════════════════════════════════
  // 预置调色板（私有）
  // ═══════════════════════════════════════════════════════

  static final _Palette _light = _Palette(
    brand: const Color(0xff4a90d9),
    brandLight: const Color(0xfff0f5ff),
    scaffoldBg: const Color(0xfff5f5f5),
    cardBg: Colors.white,
    backgroundColor: const Color(0xfffafafa),
    surfaceBg: const Color(0xfff6f8fb),
    textPrimary: const Color(0xff172033),
    textSecondary: const Color(0xff5f6368),
    textTertiary: const Color(0xff9e9e9e),
    textPriceUp: const Color(0xffe53935),
    textPriceDown: const Color(0xff0d904f),
    stockUpBg: const Color(0xfffce8e6),
    stockDownBg: const Color(0xffe6f4ea),
    divider: const Color(0xffe0e0e0),
    border: const Color(0xffeeeeee),
    appBarBg: const Color(0xfff6f8fb),
    appBarFg: const Color(0xff172033),
    inputFill: const Color(0xfff5f5f5),
    warning: const Color(0xfff9a825),
    success: const Color(0xff43a047),
    error: const Color(0xffe53935),
    info: const Color(0xff1e88e5),
    link: const Color(0xff1a73e8),
    disabled: const Color(0xffbdbdbd),
    shimmerBase: const Color(0xffe0e0e0),
    shimmerHighlight: const Color(0xfff5f5f5),
    overlay: const Color(0x66000000),
    tagRed: const Color(0xffe91e63),
    tagOrange: const Color(0xffff5722),
    tagGreen: const Color(0xff4caf50),
    tagBlue: const Color(0xff2196f3),
    tagPurple: const Color(0xff9c27b0),
  );

  static final _Palette _dark = _Palette(
    brand: const Color.fromARGB(255, 26, 124, 228),
    brandLight: const Color.fromARGB(255, 9, 65, 121),
    scaffoldBg: const Color(0xff121212),
    cardBg: const Color(0xff1e1e1e),
    backgroundColor: const Color(0xff3a3a3a),
    surfaceBg: const Color(0xff252525),
    textPrimary: const Color(0xffe0e0e0),
    textSecondary: const Color(0xffaaaaaa),
    textTertiary: const Color(0xff777777),
    textPriceUp: const Color(0xffef5350),
    textPriceDown: const Color(0xff66bb6a),
    stockUpBg: const Color(0xff3a1a1a),
    stockDownBg: const Color(0xff1a3a1a),
    divider: const Color(0xff333333),
    border: const Color(0xff333333),
    appBarBg: const Color(0xff1e1e1e),
    appBarFg: const Color(0xffe0e0e0),
    inputFill: const Color(0xff2a2a2a),
    warning: const Color(0xfffdd835),
    success: const Color(0xff66bb6a),
    error: const Color(0xffef5350),
    info: const Color(0xff42a5f5),
    link: const Color(0xff64b5f6),
    disabled: const Color(0xff616161),
    shimmerBase: const Color(0xff333333),
    shimmerHighlight: const Color(0xff444444),
    overlay: const Color(0x80000000),
    tagRed: const Color(0xffef5350),
    tagOrange: const Color(0xffff7043),
    tagGreen: const Color(0xff66bb6a),
    tagBlue: const Color(0xff42a5f5),
    tagPurple: const Color(0xffab47bc),
  );

  static final _Palette _grey = _Palette(
    brand: const Color(0xff888888),
    brandLight: const Color(0xfff0f0f0),
    scaffoldBg: const Color(0xfff5f5f5),
    cardBg: Colors.white,
    backgroundColor: const Color(0xfff8f8f8),
    surfaceBg: const Color(0xfff6f8fb),
    textPrimary: const Color(0xff333333),
    textSecondary: const Color(0xff888888),
    textTertiary: const Color(0xffaaaaaa),
    textPriceUp: const Color(0xff888888),
    textPriceDown: const Color(0xff888888),
    stockUpBg: const Color(0xfff5f5f5),
    stockDownBg: const Color(0xfff5f5f5),
    divider: const Color(0xffe0e0e0),
    border: const Color(0xffeeeeee),
    appBarBg: const Color(0xfff6f8fb),
    appBarFg: const Color(0xff333333),
    inputFill: const Color(0xfff5f5f5),
    warning: const Color(0xff999999),
    success: const Color(0xff999999),
    error: const Color(0xff999999),
    info: const Color(0xff999999),
    link: const Color(0xff888888),
    disabled: const Color(0xffcccccc),
    shimmerBase: const Color(0xffe0e0e0),
    shimmerHighlight: const Color(0xfff5f5f5),
    overlay: const Color(0x66000000),
    tagRed: const Color(0xff888888),
    tagOrange: const Color(0xff888888),
    tagGreen: const Color(0xff888888),
    tagBlue: const Color(0xff888888),
    tagPurple: const Color(0xff888888),
  );
}

/// 调色板数据类（私有，仅用于预设）
class _Palette {
  const _Palette({
    required this.brand,
    required this.brandLight,
    required this.scaffoldBg,
    required this.cardBg,
    required this.backgroundColor,
    required this.surfaceBg,
    required this.textPrimary,
    required this.textSecondary,
    required this.textTertiary,
    required this.textPriceUp,
    required this.textPriceDown,
    required this.stockUpBg,
    required this.stockDownBg,
    required this.divider,
    required this.border,
    required this.appBarBg,
    required this.appBarFg,
    required this.inputFill,
    required this.warning,
    required this.success,
    required this.error,
    required this.info,
    required this.link,
    required this.disabled,
    required this.shimmerBase,
    required this.shimmerHighlight,
    required this.overlay,
    required this.tagRed,
    required this.tagOrange,
    required this.tagGreen,
    required this.tagBlue,
    required this.tagPurple,
  });

  final Color brand;
  final Color brandLight;
  final Color scaffoldBg;
  final Color cardBg;
  final Color backgroundColor;
  final Color surfaceBg;
  final Color textPrimary;
  final Color textSecondary;
  final Color textTertiary;
  final Color textPriceUp;
  final Color textPriceDown;
  final Color stockUpBg;
  final Color stockDownBg;
  final Color divider;
  final Color border;
  final Color appBarBg;
  final Color appBarFg;
  final Color inputFill;
  final Color warning;
  final Color success;
  final Color error;
  final Color info;
  final Color link;
  final Color disabled;
  final Color shimmerBase;
  final Color shimmerHighlight;
  final Color overlay;
  final Color tagRed;
  final Color tagOrange;
  final Color tagGreen;
  final Color tagBlue;
  final Color tagPurple;
}

// ═══════════════════════════════════════════════════════
// InheritedWidget — 用于 context 传递
// ═══════════════════════════════════════════════════════
class AppColorsWidget extends InheritedWidget {
  const AppColorsWidget({
    super.key,
    required this.colors,
    required super.child,
  });

  final AppColors colors;

  @override
  bool updateShouldNotify(AppColorsWidget old) => old.colors != colors;
}
