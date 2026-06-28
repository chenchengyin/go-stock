import 'dart:convert';
import 'package:flutter/foundation.dart';
import 'package:flutter/material.dart';
import 'package:trading_app/core/network/api_client.dart';
import 'package:trading_app/features/radar/data/radar_repository.dart';
import 'package:trading_app/features/radar/domain/radar_models.dart';

class RadarViewModel extends ChangeNotifier {
  RadarViewModel(this._repository);

  final RadarRepository _repository;

  List<MonitoredStock> monitoredStocks = [];
  List<StockChange> latestChanges = [];
  String searchKeyword = '';
  bool isSearching = false;

  /// 加载监控股票列表
  Future<void> loadMonitoredStocks() async {
    monitoredStocks = await _repository.getMonitoredStocks();
    notifyListeners();
  }

  /// 添加监控股票（只检查本地是否已存在，不弹提示）
  Future<bool> addMonitoredStock(MonitoredStock stock) async {
    if (monitoredStocks.any((s) => s.code == stock.code)) {
      return false;
    }
    final result = await _repository.addMonitoredStock(stock);
    if (result == '关注成功') {
      await loadMonitoredStocks();
      return true;
    }
    return false;
  }

  /// 移除监控股票
  Future<void> removeMonitoredStock(String code) async {
    await _repository.removeMonitoredStock(code);
    await loadMonitoredStocks();
  }

  /// 加载最新异动数据
  Future<void> loadLatestChanges() async {
    final codes = monitoredStocks.map((s) => s.code).toList();
    latestChanges = await _repository.getLatestChanges(codes);
    notifyListeners();
  }

  /// 搜索股票
  Future<List<Map<String, String>>> searchStocks(String keyword) async {
    if (keyword.isEmpty) return [];
    try {
      final response = await createApiClient().get(
        '/security_depth',
        queryParameters: {
          'secid': _toSecid(keyword),
          'fields': 'f1,f2,f3,f4,f5,f6',
        },
      );
      if (response.statusCode == 200) {
        final data = response.data;
        if (data is String && data.contains('{')) {
          final jsonStr = data.substring(
              data.indexOf('{'), data.lastIndexOf('}') + 1);
          final json = Map<String, dynamic>.from(
              jsonDecode(jsonStr) as Map<String, dynamic>);
          final result = json['data']?['result'] as List<dynamic>? ?? [];
          return result
              .take(10)
              .map((e) => {
                    'code': e['sec_code'] as String? ?? '',
                    'name': e['sec_name'] as String? ?? '',
                  })
              .toList();
        }
      }
    } catch (e) {
      if (kDebugMode) print('搜索股票失败: $e');
    }
    return [];
  }

  String _toSecid(String code) {
    if (code.startsWith('6')) return '1.$code';
    if (code.startsWith('0') || code.startsWith('3')) return '0.$code';
    return '0.$code';
  }
}
