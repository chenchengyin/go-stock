import 'package:trading_app/features/radar/domain/radar_models.dart';

abstract class RadarRepository {
  Future<List<MonitoredStock>> getMonitoredStocks();
  Future<String> addMonitoredStock(MonitoredStock stock);
  Future<String> removeMonitoredStock(String code);
  Future<List<StockChange>> getLatestChanges(List<String> codes);
}

class RadarRepositoryImpl implements RadarRepository {
  RadarRepositoryImpl({required this.dio, required this.baseUrl});

  final dynamic dio;
  final String baseUrl;

  String get _baseUrl => '$baseUrl/api';

  @override
  Future<List<MonitoredStock>> getMonitoredStocks() async {
    final response = await dio.get('$_baseUrl/follow-list');
    if (response.statusCode != 200 || response.data == null) return [];
    final list = response.data as List<dynamic>? ?? [];
    return list
        .map((e) => MonitoredStock(
            code: e['stockCode'] as String? ?? '',
            name: e['name'] as String? ?? ''))
        .toList();
  }

  @override
  Future<String> addMonitoredStock(MonitoredStock stock) async {
    final response = await dio.post('$_baseUrl/follow', data: {
      'stockCode': stock.code,
    });
    if (response.statusCode != 200) return '关注失败';
    final data = response.data as Map<String, dynamic>?;
    return data?['result'] as String? ?? '关注失败';
  }

  @override
  Future<String> removeMonitoredStock(String code) async {
    final response = await dio.post('$_baseUrl/unfollow', data: {
      'stockCode': code,
    });
    if (response.statusCode != 200) return '取消关注失败';
    final data = response.data as Map<String, dynamic>?;
    return data?['result'] as String? ?? '取消关注失败';
  }

  @override
  Future<List<StockChange>> getLatestChanges(List<String> codes) async {
    if (codes.isEmpty) return [];
    final codesParam = codes.join(',');
    final response = await dio.get('$_baseUrl/stock-changes',
        queryParameters: {'codes': codesParam});
    if (response.statusCode != 200) return [];
    final data = response.data as Map<String, dynamic>?;
    final list = data?['data'] as List<dynamic>? ?? [];
    return list
        .map((e) => StockChange.fromJson(
            Map<String, dynamic>.from(e as Map<dynamic, dynamic>)))
        .toList();
  }
}
