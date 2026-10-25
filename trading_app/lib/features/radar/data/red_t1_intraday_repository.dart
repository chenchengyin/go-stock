import 'package:dio/dio.dart';

class RedT1IntradayBar {
  const RedT1IntradayBar({
    required this.time,
    required this.price,
    this.averagePrice = 0,
  });

  final String time;
  final double price;
  final double averagePrice;
}

class RedT1IntradaySeries {
  const RedT1IntradaySeries({
    required this.code,
    required this.date,
    required this.bars,
    required this.prevClose,
    required this.source,
    this.status = '',
    this.message = '',
    this.previousDate,
    this.nextDate,
  });

  final String code;
  final String date;
  final List<RedT1IntradayBar> bars;
  final double prevClose;
  final String source;
  final String status;
  final String message;
  final String? previousDate;
  final String? nextDate;

  factory RedT1IntradaySeries.fromJson(
    Map<String, dynamic> json, {
    String? previousDate,
    String? nextDate,
  }) {
    final rawBars = json['bars'];
    final bars = <RedT1IntradayBar>[];
    if (rawBars is List) {
      for (final raw in rawBars) {
        if (raw is! Map) continue;
        final item = Map<String, dynamic>.from(raw);
        final time = item['time'];
        final price = item['price'];
        if (time is! String || time.isEmpty || price is! num) continue;
        final value = price.toDouble();
        if (!value.isFinite || value <= 0) continue;
        final rawAveragePrice = item['averagePrice'];
        final averagePrice = rawAveragePrice is num
            ? rawAveragePrice.toDouble()
            : 0.0;
        bars.add(
          RedT1IntradayBar(
            time: time,
            price: value,
            averagePrice: averagePrice.isFinite && averagePrice > 0
                ? averagePrice
                : 0,
          ),
        );
      }
    }
    return RedT1IntradaySeries(
      code: json['code'] as String? ?? '',
      date: json['date'] as String? ?? '',
      bars: List.unmodifiable(bars),
      prevClose: (json['prevClose'] as num?)?.toDouble() ?? 0,
      source: json['source'] as String? ?? '',
      status: json['status'] as String? ?? '',
      message: json['message'] as String? ?? '',
      previousDate: previousDate,
      nextDate: nextDate,
    );
  }
}

typedef RedT1IntradayBatchFetcher =
    Future<Map<String, dynamic>> Function(
      String date,
      List<String> codes,
      String moduleCode,
    );

/// Fetches T+1 minute series and coalesces each date/code batch in memory.
class RedT1IntradayRepository {
  RedT1IntradayRepository({Dio? dio, RedT1IntradayBatchFetcher? fetchBatch})
    : _dio = dio,
      _fetchBatch = fetchBatch;

  final Dio? _dio;
  final RedT1IntradayBatchFetcher? _fetchBatch;
  final Map<String, Future<Map<String, RedT1IntradaySeries>>> _pending = {};
  final Map<String, Map<String, RedT1IntradaySeries>> _successfulCache = {};

  Future<Map<String, RedT1IntradaySeries>> load(
    String t0Date,
    List<String> codes, {
    String moduleCode = 'radar.red_strategy',
  }) {
    final unique = codes.where((code) => code.isNotEmpty).toSet().toList()
      ..sort();
    if (t0Date.isEmpty || unique.isEmpty) {
      return Future.value(<String, RedT1IntradaySeries>{});
    }
    final key = '$moduleCode:$t0Date:${unique.join(',')}';
    final cached = _successfulCache[key];
    if (cached != null) return Future.value(cached);
    final pending = _pending[key];
    if (pending != null) return pending;

    late final Future<Map<String, RedT1IntradaySeries>> request;
    request =
        (() async {
              final items = <String, RedT1IntradaySeries>{};
              for (var start = 0; start < unique.length; start += 25) {
                final end = (start + 25).clamp(0, unique.length);
                final batch = unique.sublist(start, end);
                final response = _fetchBatch != null
                    ? await _fetchBatch(t0Date, batch, moduleCode)
                    : await _request(t0Date, batch, moduleCode);
                final rawItems = response['items'];
                if (rawItems is! Map) {
                  throw const FormatException('分时接口响应缺少 items');
                }
                for (final code in batch) {
                  final raw = rawItems[code];
                  items[code] = RedT1IntradaySeries.fromJson(
                    raw is Map
                        ? Map<String, dynamic>.from(raw)
                        : {
                            'code': code,
                            'date': response['date'] as String? ?? '',
                            'status': 'missing_series',
                            'message': '分时接口未返回该股票的数据条目',
                          },
                    previousDate: response['previous_date'] as String?,
                    nextDate: response['next_date'] as String?,
                  );
                }
              }
              return items;
            })()
            .then((items) {
              // A cleared/replaced request may still resolve to its original
              // caller, but must never repopulate the refreshed cache.
              if (identical(_pending[key], request) &&
                  items.values.every(_isCacheable)) {
                final stableItems =
                    Map<String, RedT1IntradaySeries>.unmodifiable(items);
                _successfulCache[key] = stableItems;
                return stableItems;
              }
              return items;
            })
            .whenComplete(() {
              if (identical(_pending[key], request)) _pending.remove(key);
            });
    _pending[key] = request;
    return request;
  }

  bool _isCacheable(RedT1IntradaySeries series) =>
      series.bars.length >= 2 && series.prevClose > 0;

  void clearCache({String? moduleCode}) {
    final keys = {..._pending.keys, ..._successfulCache.keys}
        .where((key) => moduleCode == null || key.startsWith('$moduleCode:'))
        .toList();
    for (final key in keys) {
      _pending.remove(key);
      _successfulCache.remove(key);
    }
  }

  /// Fetch one requested trading session for one card, without retaining it.
  Future<RedT1IntradaySeries> loadSession(
    String t0Date,
    String code,
    String sessionDate, {
    String moduleCode = 'radar.red_strategy',
  }) async {
    final response = await _request(
      t0Date,
      [code],
      moduleCode,
      sessionDate: sessionDate,
    );
    final raw = response['items'] is Map
        ? (response['items'] as Map)[code]
        : null;
    return RedT1IntradaySeries.fromJson(
      raw is Map
          ? Map<String, dynamic>.from(raw)
          : {
              'code': code,
              'date': response['date'] ?? sessionDate,
              'status': 'missing_series',
              'message': '分时接口未返回该股票的数据条目',
            },
      previousDate: response['previous_date'] as String?,
      nextDate: response['next_date'] as String?,
    );
  }

  Future<Map<String, dynamic>> _request(
    String date,
    List<String> codes,
    String moduleCode, {
    String? sessionDate,
  }) async {
    final dio = _dio;
    if (dio == null) throw StateError('Intraday API client is missing');
    final response = await dio.get<dynamic>(
      '/api/t0-red/t1-intraday',
      queryParameters: {
        'date': date,
        'codes': codes.join(','),
        'module_code': moduleCode,
        if (sessionDate != null) 'session_date': sessionDate,
      },
    );
    if (response.data is! Map) {
      throw const FormatException('Invalid T+1 intraday response');
    }
    return Map<String, dynamic>.from(response.data as Map);
  }
}

String redT1IntradayErrorMessage(Object error) {
  if (error is DioException) {
    final data = error.response?.data;
    if (data is Map) {
      final message = data['message'] ?? data['error'];
      if (message is String && message.trim().isNotEmpty) {
        return message.trim();
      }
    }
    final statusCode = error.response?.statusCode;
    if (statusCode != null) return '分时接口请求失败（HTTP $statusCode）';
    if (error.type == DioExceptionType.connectionTimeout ||
        error.type == DioExceptionType.sendTimeout ||
        error.type == DioExceptionType.receiveTimeout) {
      return '分时接口请求超时';
    }
    if (error.type == DioExceptionType.connectionError) {
      return '无法连接分时数据服务';
    }
    return '分时接口请求失败';
  }
  if (error is FormatException) return error.message;
  return '分时加载失败（${error.runtimeType}）';
}
