import 'dart:convert';

import 'package:shared_preferences/shared_preferences.dart';
import 'package:trading_app/features/radar/domain/radar_models.dart';

abstract class RadarRepository {
  Future<List<MonitoredStock>> getMonitoredStocks();
  Future<void> addMonitoredStock(MonitoredStock stock);
  Future<void> removeMonitoredStock(String code);
  Future<List<StockChange>> getLatestChanges(List<String> codes);
}

class RadarRepositoryImpl implements RadarRepository {
  RadarRepositoryImpl({required this.dio, required this.baseUrl});

  final dynamic dio;
  final String baseUrl;

  String get _endpoint => '$baseUrl/stock-changes';
  String get _prefsKey => 'monitored_stocks';

  Future<SharedPreferences> _getPrefs() => SharedPreferences.getInstance();

  @override
  Future<List<MonitoredStock>> getMonitoredStocks() async {
    final prefs = await _getPrefs();
    final stocksJson = prefs.getStringList(_prefsKey) ?? [];
    return stocksJson
        .map((e) => MonitoredStock.fromJson(
            jsonDecode(e) as Map<String, dynamic>))
        .toList();
  }

  @override
  Future<void> addMonitoredStock(MonitoredStock stock) async {
    final prefs = await _getPrefs();
    final stocksJson = prefs.getStringList(_prefsKey) ?? [];
    stocksJson.add(jsonEncode(stock.toJson()));
    await prefs.setStringList(_prefsKey, stocksJson);
  }

  @override
  Future<void> removeMonitoredStock(String code) async {
    final prefs = await _getPrefs();
    final stocksJson = prefs.getStringList(_prefsKey) ?? [];
    stocksJson.removeWhere(
        (e) => (e as Map<dynamic, dynamic>)['code'] == code);
    await prefs.setStringList(_prefsKey, stocksJson);
  }

  @override
  Future<List<StockChange>> getLatestChanges(List<String> codes) async {
    if (codes.isEmpty) return [];
    final codesParam = codes.join(',');
    final response = await dio.get(
      _endpoint,
      queryParameters: {'codes': codesParam},
    );
    if (response.statusCode != 200) return [];
    final data = response.data as Map<String, dynamic>?;
    final list = data?['data'] as List<dynamic>? ?? [];
    return list
        .map((e) => StockChange.fromJson(
            Map<String, dynamic>.from(e as Map<dynamic, dynamic>)))
        .toList();
  }
}
