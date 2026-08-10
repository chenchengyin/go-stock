import 'dart:io';

import 'package:flutter/foundation.dart';
import 'package:home_widget/home_widget.dart';

/// 桌面股票组件数据管理器
///
/// 负责把 Flutter 端获取到的股票行情写入原生桌面组件可读取的存储区域，
/// 并触发组件刷新。当前仅在 Android 平台生效。
class StockWidgetManager {
  StockWidgetManager._();

  /// Android 端 Widget 的类名
  static const String _androidWidgetName = 'com.colin.trading.StockWidget';

  /// Widget 在 Flutter 侧注册的名字
  static const String _widgetName = 'StockWidget';

  /// 当前平台是否支持桌面组件
  static bool get _supported => !kIsWeb && Platform.isAndroid;

  /// 更新桌面组件显示的股票行情
  ///
  /// [name]   股票名称
  /// [price]  当前价格，建议保留两位小数
  /// [changeRate] 涨跌幅，如 2.35 表示 +2.35%
  static Future<void> updateStock({
    required String name,
    required double price,
    required double changeRate,
  }) async {
    if (!_supported) return;

    try {
      final priceStr = price.toStringAsFixed(2);
      final ratePrefix = changeRate >= 0 ? '+' : '';
      final rateStr = '$ratePrefix${changeRate.toStringAsFixed(2)}%';

      await Future.wait([
        HomeWidget.saveWidgetData('stock_name', name),
        HomeWidget.saveWidgetData('stock_price', priceStr),
        HomeWidget.saveWidgetData('stock_change_rate', rateStr),
      ]);

      await HomeWidget.updateWidget(
        name: _widgetName,
        androidName: _androidWidgetName,
      );
    } catch (e) {
      debugPrint('[StockWidgetManager] 更新桌面组件失败: $e');
    }
  }

  /// 清空桌面组件数据（例如监控列表为空时）
  static Future<void> clear() async {
    if (!_supported) return;

    try {
      await Future.wait([
        HomeWidget.saveWidgetData('stock_name', '--'),
        HomeWidget.saveWidgetData('stock_price', '--'),
        HomeWidget.saveWidgetData('stock_change_rate', '0.00%'),
      ]);
      await HomeWidget.updateWidget(
        name: _widgetName,
        androidName: _androidWidgetName,
      );
    } catch (e) {
      debugPrint('[StockWidgetManager] 清空桌面组件失败: $e');
    }
  }
}
