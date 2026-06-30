import 'package:flutter/material.dart';
import 'package:trading_app/features/radar/domain/radar_models.dart';

/// 盘口语言标签类型
enum PanKouTag {
  /// 涨停
  zhangTing,
  /// 跌停
  dieTing,
  /// 对子顶：当前价 = 最高价
  duiZiDing,
  /// 对子底：当前价 = 最低价
  duiZiDi,
  /// 一字板：开盘 = 收盘 = 最高 = 最低
  yiZiBan,
}

/// 盘口语言分析器
class PanKouAnalyzer {
  /// 根据股票数据生成盘口语言标签
  static List<PanKouTag> analyzeTags(MonitoredStock stock) {
    if (stock.price <= 0 || stock.open <= 0 || stock.preClose <= 0) {
      return [];
    }

    final price = stock.price;
    final open = stock.open;
    final high = stock.high;
    final low = stock.low;
    final preClose = stock.preClose;

    // 一字板
    if (high > 0 && low > 0 && open == price && high == price && low == price) {
      return [PanKouTag.yiZiBan];
    }

    // 涨停
    if ((high - preClose * 1.1).abs() < 0.01 || (high - preClose * 1.2).abs() < 0.01) {
      return [PanKouTag.zhangTing];
    }
    // 跌停
    if ((low - preClose * 0.9).abs() < 0.01 || (low - preClose * 0.8).abs() < 0.01) {
      return [PanKouTag.dieTing];
    }
    // 对子顶：最高价小数点后两位数字相同，如 .00 .11 .22 .33 … .99
    if (high > 0) {
      final cents = (high * 100).round() % 100;
      if (cents ~/ 10 == cents % 10) {
        return [PanKouTag.duiZiDing];
      }
    }
    // 对子底：最低价小数点后两位数字相同
    if (low > 0) {
      final cents = (low * 100).round() % 100;
      if (cents ~/ 10 == cents % 10) {
        return [PanKouTag.duiZiDi];
      }
    }

    return [];
  }

  static String getTagName(PanKouTag tag) {
    switch (tag) {
      case PanKouTag.zhangTing:
        return '涨停';
      case PanKouTag.dieTing:
        return '跌停';
      case PanKouTag.duiZiDing:
        return '对子顶';
      case PanKouTag.duiZiDi:
        return '对子底';
      case PanKouTag.yiZiBan:
        return '一字板';
    }
  }

  static Color getTagColor(PanKouTag tag) {
    switch (tag) {
      case PanKouTag.zhangTing:
      case PanKouTag.duiZiDing:
        return const Color(0xffe53935);
      case PanKouTag.dieTing:
      case PanKouTag.duiZiDi:
        return const Color(0xff0d904f);
      case PanKouTag.yiZiBan:
        return const Color(0xff9c27b0);
    }
  }
}
