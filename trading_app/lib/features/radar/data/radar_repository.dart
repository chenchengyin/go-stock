import 'package:dio/dio.dart';
import 'package:trading_app/features/radar/domain/radar_models.dart';
import 'package:trading_app/core/network/api_client.dart';

abstract class RadarRepository {
  Future<List<MonitoredStock>> getMonitoredStocks();
  Future<String> addMonitoredStock(MonitoredStock stock);
  Future<String> removeMonitoredStock(String code);
  Future<List<StockChange>> getLatestChanges(List<String> codes);
  Future<List<Map<String, String>>> searchStocks(String keyword);
  Future<Map<String, Map<String, dynamic>>> fetchRealtimeQuotes(
    List<String> codes,
  );
}

class RadarRepositoryImpl implements RadarRepository {
  RadarRepositoryImpl({Dio? dio}) : _dio = dio ?? createApiClient();

  final Dio _dio;

  @override
  Future<List<MonitoredStock>> getMonitoredStocks() async {
    final response = await _dio.get('/api/follow-list');
    if (response.statusCode != 200 || response.data == null) return [];
    final list = response.data as List<dynamic>? ?? [];
    return list
        .map(
          (e) => MonitoredStock(
            code: e['stockCode'] as String? ?? '',
            name: e['name'] as String? ?? '',
            price: (e['price'] as num?)?.toDouble() ?? 0.0,
            changePercent: (e['changePercent'] as num?)?.toDouble() ?? 0.0,
            volume: (e['volume'] as num?)?.toInt() ?? 0,
            amount: (e['amount'] as num?)?.toDouble() ?? 0.0,
          ),
        )
        .toList();
  }

  @override
  Future<String> addMonitoredStock(MonitoredStock stock) async {
    final response = await _dio.post(
      '/api/follow',
      data: {'stockCode': stock.code},
    );
    if (response.statusCode != 200) return '关注失败';
    final data = response.data as Map<String, dynamic>?;
    return data?['result'] as String? ?? '关注失败';
  }

  @override
  Future<String> removeMonitoredStock(String code) async {
    final response = await _dio.post(
      '/api/unfollow',
      data: {'stockCode': code},
    );
    if (response.statusCode != 200) return '取消关注失败';
    final data = response.data as Map<String, dynamic>?;
    return data?['result'] as String? ?? '取消关注失败';
  }

  @override
  Future<List<StockChange>> getLatestChanges(List<String> codes) async {
    if (codes.isEmpty) return [];
    final codesParam = codes.join(',');
    final response = await _dio.get(
      '/api/stock-changes',
      queryParameters: {'codes': codesParam},
    );
    if (response.statusCode != 200) return [];
    final data = response.data as Map<String, dynamic>?;
    final list = data?['data'] as List<dynamic>? ?? [];
    return list
        .map(
          (e) => StockChange.fromJson(
            Map<String, dynamic>.from(e as Map<dynamic, dynamic>),
          ),
        )
        .toList();
  }

  @override
  Future<List<Map<String, String>>> searchStocks(String keyword) async {
    if (keyword.isEmpty) return [];
    final response = await _dio.get(
      '/api/stock-search',
      queryParameters: {'keyword': keyword},
    );
    if (response.statusCode != 200) return [];
    final list = response.data as List<dynamic>? ?? [];
    return list.map((e) {
      final map = Map<String, dynamic>.from(e as Map<dynamic, dynamic>);
      return {
        'code': map['stockCode'] as String? ?? '',
        'name': map['name'] as String? ?? '',
      };
    }).toList();
  }

  @override
  Future<Map<String, Map<String, dynamic>>> fetchRealtimeQuotes(
    List<String> codes,
  ) async {
    if (codes.isEmpty) return {};
    final response = await _dio.get(
      '/api/stock-realtime',
      queryParameters: {'codes': codes.join(',')},
    );
    if (response.statusCode != 200) return {};
    final list = response.data as List<dynamic>? ?? [];
    final map = <String, Map<String, dynamic>>{};
    for (final item in list) {
      final m = Map<String, dynamic>.from(item as Map<dynamic, dynamic>);
      final code = m['code'] as String? ?? '';
      if (code.isNotEmpty) {
        map[code] = m;
      }
    }
    return map;
  }
}
