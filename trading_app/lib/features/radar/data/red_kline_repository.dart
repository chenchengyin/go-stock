import 'package:dio/dio.dart';

class RedKlineBar {
  const RedKlineBar({
    required this.date,
    required this.open,
    required this.high,
    required this.low,
    required this.close,
    this.ma = const {},
  });

  final String date;
  final double open;
  final double high;
  final double low;
  final double close;

  /// Only complete windows are present; never substitute a short average.
  final Map<int, double> ma;

  factory RedKlineBar.fromJson(Map<String, dynamic> json) {
    final rawMa = json['ma'];
    final ma = <int, double>{};
    if (rawMa is Map) {
      for (final period in [5, 10, 20, 60]) {
        final value = rawMa['$period'];
        if (value is num && value.isFinite && value > 0) {
          ma[period] = value.toDouble();
        }
      }
    }
    return RedKlineBar(
      date: json['date'] as String? ?? '',
      open: (json['open'] as num).toDouble(),
      high: (json['high'] as num).toDouble(),
      low: (json['low'] as num).toDouble(),
      close: (json['close'] as num).toDouble(),
      ma: Map.unmodifiable(ma),
    );
  }
}

class RedKlineWindow {
  const RedKlineWindow({
    required this.code,
    required this.date,
    required this.bars,
    required this.anchorIndex,
    required this.beforeCount,
    required this.afterCount,
    required this.source,
    required this.status,
    required this.partial,
  });

  final String code;
  final String date;
  final List<RedKlineBar> bars;
  final int anchorIndex;
  final int beforeCount;
  final int afterCount;
  final String source;
  final String status;
  final bool partial;

  factory RedKlineWindow.fromJson(Map<String, dynamic> json) {
    final raw = json['bars'] as List<dynamic>? ?? [];
    return RedKlineWindow(
      code: json['code'] as String? ?? '',
      date: json['date'] as String? ?? '',
      bars: List.unmodifiable(
        raw.whereType<Map>().map(
          (item) => RedKlineBar.fromJson(Map<String, dynamic>.from(item)),
        ),
      ),
      anchorIndex: (json['anchor_index'] as num?)?.toInt() ?? -1,
      beforeCount: (json['before_count'] as num?)?.toInt() ?? 0,
      afterCount: (json['after_count'] as num?)?.toInt() ?? 0,
      source: json['source'] as String? ?? '',
      status: json['status'] as String? ?? 'unavailable',
      partial: json['partial'] as bool? ?? true,
    );
  }
}

typedef RedKlineBatchFetcher =
    Future<Map<String, dynamic>> Function(String date, List<String> codes);

/// Reuses an entire date/code batch across normal radar-list rebuilds.
class RedKlineRepository {
  RedKlineRepository({
    Dio? dio,
    RedKlineBatchFetcher? fetchBatch,
    DateTime Function()? now,
  }) : _dio = dio,
       _fetchBatch = fetchBatch,
       _now = now ?? DateTime.now;

  final Dio? _dio;
  final RedKlineBatchFetcher? _fetchBatch;
  final DateTime Function() _now;
  final Map<String, Future<Map<String, RedKlineWindow>>> _pending = {};
  final Map<String, DateTime> _cachedAt = {};

  Future<Map<String, RedKlineWindow>> load(
    String date,
    List<String> codes, {
    String moduleCode = 'radar.red_strategy',
  }) {
    final unique = codes.where((code) => code.isNotEmpty).toSet().toList()
      ..sort();
    if (date.isEmpty || unique.isEmpty) return Future.value({});
    final key = '$moduleCode:$date:${unique.join(',')}';
    final now = _now();
    final shanghaiToday = now.toUtc().add(const Duration(hours: 8));
    final recentThreshold = DateTime.utc(
      shanghaiToday.year,
      shanghaiToday.month,
      shanghaiToday.day,
    ).subtract(const Duration(days: 14)).toIso8601String().substring(0, 10);
    final savedAt = _cachedAt[key];
    if (date.compareTo(recentThreshold) >= 0 &&
        savedAt != null &&
        now.difference(savedAt) >= const Duration(minutes: 5)) {
      _pending.remove(key);
      _cachedAt.remove(key);
    }
    if (_pending.length > 40) {
      _pending.clear();
      _cachedAt.clear();
    }
    _cachedAt.putIfAbsent(key, () => now);
    return _pending.putIfAbsent(key, () async {
      try {
        final items = <String, RedKlineWindow>{};
        for (var start = 0; start < unique.length; start += 25) {
          final end = (start + 25).clamp(0, unique.length);
          final batch = unique.sublist(start, end);
          final response = _fetchBatch != null
              ? await _fetchBatch(date, batch)
              : await _request(date, batch, moduleCode);
          final rawItems = response['items'];
          if (rawItems is! Map) continue;
          for (final entry in rawItems.entries) {
            if (entry.value is! Map) continue;
            items[entry.key.toString()] = RedKlineWindow.fromJson(
              Map<String, dynamic>.from(entry.value as Map),
            );
          }
        }
        return items;
      } catch (_) {
        _pending.remove(key);
        _cachedAt.remove(key);
        rethrow;
      }
    });
  }

  Future<Map<String, dynamic>> _request(
    String date,
    List<String> codes,
    String moduleCode,
  ) async {
    final dio = _dio;
    if (dio == null) throw StateError('K-line API client is missing');
    final response = await dio.get<dynamic>(
      '/api/t0-red/kline-window',
      queryParameters: {
        'date': date,
        'codes': codes.join(','),
        'module_code': moduleCode,
      },
    );
    if (response.data is! Map) {
      throw const FormatException('Invalid K-line response');
    }
    return Map<String, dynamic>.from(response.data as Map);
  }
}
