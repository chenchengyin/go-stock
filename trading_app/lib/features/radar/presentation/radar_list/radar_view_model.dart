import 'dart:async';
import 'package:flutter/foundation.dart';
import 'package:trading_app/features/radar/domain/radar_models.dart';
import 'package:trading_app/features/radar/data/radar_repository.dart';

class RadarViewModel extends ChangeNotifier {
  RadarViewModel(this._repository) {
    _startPeriodicRefresh();
  }

  final RadarRepository _repository;

  List<MonitoredStock> monitoredStocks = [];
  List<StockChange> latestChanges = [];
  List<Map<String, String>> searchResults = [];
  bool isSearching = false;
  String _searchKeyword = '';

  String get searchKeyword => _searchKeyword;
  set searchKeyword(String val) {
    _searchKeyword = val;
    _onSearchChanged(val);
  }

  Timer? _debounce;
  Timer? _refreshTimer;
  static const _refreshInterval = Duration(seconds: 10);

  void _startPeriodicRefresh() {
    _refreshTimer?.cancel();
    _refreshTimer = Timer.periodic(_refreshInterval, (_) async {
      await _refreshData();
    });
  }

  Future<void> _refreshData() async {
    try {
      // 只拉实时行情合并到本地监控列表 + 拉异动
      // 不重新拉 follow-list，避免多余请求
      if (monitoredStocks.isNotEmpty) {
        final results = await Future.wait([
          _repository.fetchRealtimeQuotes(
              monitoredStocks.map((s) => s.code).toList()),
          _repository.getLatestChanges(
              monitoredStocks.map((s) => s.code).toList()),
        ]);
        final quotes = results[0] as Map<String, Map<String, dynamic>>;
        latestChanges = results[1] as List<StockChange>;

        if (quotes.isNotEmpty) {
          monitoredStocks = monitoredStocks.map((s) {
            final q = quotes[s.code];
            if (q != null) {
              return MonitoredStock(
                code: s.code,
                name: q['name'] as String? ?? s.name,
                price: (q['price'] as num?)?.toDouble() ?? s.price,
                changePercent:
                    (q['changePercent'] as num?)?.toDouble() ?? s.changePercent,
                volume: (q['volume'] as num?)?.toInt() ?? s.volume,
                amount: (q['amount'] as num?)?.toDouble() ?? s.amount,
                changeTypes: s.changeTypes,
              );
            }
            return s;
          }).toList();
        }
      } else {
        latestChanges = await _repository.getLatestChanges([]);
      }

      notifyListeners();
    } catch (_) {}
  }

  void _onSearchChanged(String keyword) {
    _debounce?.cancel();
    if (keyword.trim().isEmpty) {
      searchResults = [];
      isSearching = false;
      notifyListeners();
      return;
    }
    isSearching = true;
    notifyListeners();
    _debounce = Timer(const Duration(milliseconds: 300), () {
      _doSearch(keyword.trim());
    });
  }

  Future<void> _doSearch(String keyword) async {
    try {
      searchResults = await _repository.searchStocks(keyword);
    } catch (e) {
      if (kDebugMode) print('搜索股票失败: $e');
      searchResults = [];
    }
    isSearching = false;
    notifyListeners();
  }

  /// 加载监控股票列表
  Future<void> loadMonitoredStocks() async {
    monitoredStocks = await _repository.getMonitoredStocks();
    if (monitoredStocks.isNotEmpty) {
      final codes = monitoredStocks.map((s) => s.code).toList();
      final quotes = await _repository.fetchRealtimeQuotes(codes);
      if (quotes.isNotEmpty) {
        monitoredStocks = monitoredStocks.map((s) {
          final q = quotes[s.code];
          if (q != null) {
            return MonitoredStock(
              code: s.code,
              name: q['name'] as String? ?? s.name,
              price: (q['price'] as num?)?.toDouble() ?? s.price,
              changePercent: (q['changePercent'] as num?)?.toDouble() ?? s.changePercent,
              volume: (q['volume'] as num?)?.toInt() ?? s.volume,
              amount: (q['amount'] as num?)?.toDouble() ?? s.amount,
              changeTypes: s.changeTypes,
            );
          }
          return s;
        }).toList();
      }
    }
    notifyListeners();
  }

  /// 添加监控股票
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

  @override
  void dispose() {
    _debounce?.cancel();
    _refreshTimer?.cancel();
    super.dispose();
  }
}
