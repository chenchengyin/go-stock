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
    _triggerSearch();
  }

  /// 搜索版本号，用于取消过期请求
  int _searchVersion = 0;

  /// 已读的异动 ID 集合（初始加载的也算已读）
  final Set<int> _knownChangeIds = {};

  /// 有未读异动的股票 code
  final Set<String> _codesWithNewChanges = {};

  /// 检查指定股票是否有未读异动
  bool hasNewChanges(String code) => _codesWithNewChanges.contains(code);

  /// 标记某只股票的异动为已读
  void markChangesSeen(String code) {
    _codesWithNewChanges.remove(code);
    notifyListeners();
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
        final newChanges = results[1] as List<StockChange>;

        // 检测新异动 -> 标记红点
        _detectNewChanges(newChanges);
        latestChanges = newChanges;

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
                open: (q['open'] as num?)?.toDouble() ?? s.open,
                preClose: (q['preClose'] as num?)?.toDouble() ?? s.preClose,
                high: (q['high'] as num?)?.toDouble() ?? s.high,
                low: (q['low'] as num?)?.toDouble() ?? s.low,
                changeTypes: s.changeTypes,
              );
            }
            return s;
          }).toList();
        }
      } else {
        final newChanges = await _repository.getLatestChanges([]);
        _detectNewChanges(newChanges);
        latestChanges = newChanges;
      }

      notifyListeners();
    } catch (_) {}
  }

  /// 检测并标记新异动（不在 knownChangeIds 中的视为新异动）
  void _detectNewChanges(List<StockChange> changes) {
    for (final change in changes) {
      if (!_knownChangeIds.contains(change.id)) {
        _knownChangeIds.add(change.id);
        _codesWithNewChanges.add(change.stockCode);
      }
    }
  }

  /// 每次输入时触发，启动 debounce，不立即 notifyListeners
  void _triggerSearch() {
    _debounce?.cancel();
    if (_searchKeyword.trim().isEmpty) {
      searchResults = [];
      isSearching = false;
      notifyListeners();
      return;
    }
    // 不立即设置 isSearching=true 和 notifyListeners
    // 等 debounce 结束后再触发搜索和 UI 更新
    _debounce = Timer(const Duration(milliseconds: 300), () {
      _doSearch(_searchKeyword.trim());
    });
  }

  Future<void> _doSearch(String keyword) async {
    final version = ++_searchVersion;
    isSearching = true;
    notifyListeners();
    try {
      final results = await _repository.searchStocks(keyword);
      // 只应用最新版本的搜索结果，过期请求丢弃
      if (version != _searchVersion) return;
      searchResults = results;
    } catch (e) {
      if (version != _searchVersion) return;
      if (kDebugMode) print('搜索股票失败: $e');
      searchResults = [];
    }
    if (version != _searchVersion) return;
    isSearching = false;
    notifyListeners();
  }

  /// 加载监控股票列表
  Future<void> loadMonitoredStocks() async {
    monitoredStocks = await _repository.getMonitoredStocks();
    if (monitoredStocks.isNotEmpty) {
      final codes = monitoredStocks.map((s) => s.code).toList();
      final results = await Future.wait([
        _repository.fetchRealtimeQuotes(codes),
        _repository.getLatestChanges(codes),
      ]);
      final quotes = results[0] as Map<String, Map<String, dynamic>>;
      final changes = results[1] as List<StockChange>;

      // 初始加载的异动全部标记为已读（不触发红点）
      latestChanges = changes;
      for (final c in changes) {
        _knownChangeIds.add(c.id);
      }

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
              open: (q['open'] as num?)?.toDouble() ?? s.open,
              preClose: (q['preClose'] as num?)?.toDouble() ?? s.preClose,
              high: (q['high'] as num?)?.toDouble() ?? s.high,
              low: (q['low'] as num?)?.toDouble() ?? s.low,
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
    _knownChangeIds.removeWhere((_) => false);
    _codesWithNewChanges.remove(code);
    await loadMonitoredStocks();
  }

  /// 加载最新异动数据
  Future<void> loadLatestChanges() async {
    final codes = monitoredStocks.map((s) => s.code).toList();
    final changes = await _repository.getLatestChanges(codes);
    _detectNewChanges(changes);
    latestChanges = changes;
    notifyListeners();
  }

  @override
  void dispose() {
    _debounce?.cancel();
    _refreshTimer?.cancel();
    super.dispose();
  }
}
