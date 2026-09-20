import 'dart:convert';

import 'package:shared_preferences/shared_preferences.dart';

enum StockRating {
  unrated('未评级'),
  heavy('重仓'),
  light('轻仓'),
  avoid('不买');

  const StockRating(this.label);

  final String label;
}

class StockRatingStore {
  static const storageKey = 'radar:stock_ratings:v1';

  Future<Map<String, StockRating>> loadAll() async {
    final preferences = await SharedPreferences.getInstance();
    final raw = preferences.getString(storageKey);
    if (raw == null || raw.isEmpty) return <String, StockRating>{};

    try {
      final decoded = jsonDecode(raw);
      if (decoded is! Map<String, dynamic>) return <String, StockRating>{};

      final ratings = <String, StockRating>{};
      for (final entry in decoded.entries) {
        final rating = _fromStoredValue(entry.value);
        if (rating != null && rating != StockRating.unrated) {
          ratings[entry.key] = rating;
        }
      }
      return ratings;
    } on FormatException {
      return <String, StockRating>{};
    }
  }

  Future<void> save(String stockCode, StockRating rating) async {
    final ratings = await loadAll();
    final normalizedCode = normalizeStockCode(stockCode);
    if (normalizedCode.isEmpty) return;

    if (rating == StockRating.unrated) {
      ratings.remove(normalizedCode);
    } else {
      ratings[normalizedCode] = rating;
    }

    final preferences = await SharedPreferences.getInstance();
    await preferences.setString(
      storageKey,
      jsonEncode(ratings.map((code, value) => MapEntry(code, value.name))),
    );
  }

  static String normalizeStockCode(String stockCode) {
    final match = RegExp(r'\d{6}').firstMatch(stockCode);
    return match?.group(0) ?? '';
  }

  static StockRating? _fromStoredValue(Object? value) {
    if (value is! String) return null;
    for (final rating in StockRating.values) {
      if (rating.name == value) return rating;
    }
    return null;
  }
}
