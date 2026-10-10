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
  });

  final String code;
  final String date;
  final List<RedT1IntradayBar> bars;
  final double prevClose;
  final String source;

  factory RedT1IntradaySeries.fromJson(Map<String, dynamic> json) {
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
    );
  }
}

typedef RedT1IntradayBatchFetcher =
    Future<Map<String, dynamic>> Function(
      String date,
      List<String> codes,
      String moduleCode,
    );

/// Fetches current T+1 minute series without retaining completed responses.
class RedT1IntradayRepository {
  RedT1IntradayRepository({Dio? dio, RedT1IntradayBatchFetcher? fetchBatch})
    : _dio = dio,
      _fetchBatch = fetchBatch;

  final Dio? _dio;
  final RedT1IntradayBatchFetcher? _fetchBatch;

  Future<Map<String, RedT1IntradaySeries>> load(
    String t0Date,
    List<String> codes, {
    String moduleCode = 'radar.red_strategy',
  }) async {
    final unique = codes.where((code) => code.isNotEmpty).toSet().toList()
      ..sort();
    if (t0Date.isEmpty || unique.isEmpty) return {};
    final items = <String, RedT1IntradaySeries>{};
    for (var start = 0; start < unique.length; start += 25) {
      final end = (start + 25).clamp(0, unique.length);
      final batch = unique.sublist(start, end);
      final response = _fetchBatch != null
          ? await _fetchBatch(t0Date, batch, moduleCode)
          : await _request(t0Date, batch, moduleCode);
      final rawItems = response['items'];
      if (rawItems is! Map) continue;
      for (final entry in rawItems.entries) {
        if (entry.value is! Map) continue;
        items[entry.key.toString()] = RedT1IntradaySeries.fromJson(
          Map<String, dynamic>.from(entry.value as Map),
        );
      }
    }
    return items;
  }

  Future<Map<String, dynamic>> _request(
    String date,
    List<String> codes,
    String moduleCode,
  ) async {
    final dio = _dio;
    if (dio == null) throw StateError('Intraday API client is missing');
    final response = await dio.get<dynamic>(
      '/api/t0-red/t1-intraday',
      queryParameters: {
        'date': date,
        'codes': codes.join(','),
        'module_code': moduleCode,
      },
    );
    if (response.data is! Map) {
      throw const FormatException('Invalid T+1 intraday response');
    }
    return Map<String, dynamic>.from(response.data as Map);
  }
}
