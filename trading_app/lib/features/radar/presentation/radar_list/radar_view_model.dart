import 'dart:async';
import 'package:flutter/foundation.dart';
import 'package:shared_preferences/shared_preferences.dart';
import 'package:trading_app/features/radar/domain/change_type_config.dart';
import 'package:trading_app/features/radar/domain/radar_models.dart';
import 'package:trading_app/features/radar/data/radar_repository.dart';
import 'package:trading_app/features/radar/data/notification_util.dart';

class RadarViewModel extends ChangeNotifier {
  RadarViewModel(this._repository) {
    _startPeriodicRefresh();
  }

  final RadarRepository _repository;

  // ── 异动类型筛选 ─────────────────────────────────────
  Set<int> _selectedChangeTypes = ChangeTypeConfig.defaultMonitorIds;

  /// 当前选中的异动类型 ID 集合
  Set<int> get selectedChangeTypes => _selectedChangeTypes;

  /// 从 shared_preferences 加载已选异动类型
  Future<void> loadSelectedTypes() async {
    final prefs = await SharedPreferences.getInstance();
    final raw = prefs.getString(ChangeTypeConfig.storageKey);
    if (raw != null && raw.isNotEmpty) {
      _selectedChangeTypes = raw.split(',').map(int.parse).toSet();
    } else {
      _selectedChangeTypes = ChangeTypeConfig.defaultMonitorIds;
    }
  }

  /// 根据已选类型过滤异动列表
  List<StockChange> filterChanges(List<StockChange> changes) {
    if (_selectedChangeTypes.length == ChangeTypeConfig.allTypes.length) {
      return changes;
    }
    return changes.where((c) => _selectedChangeTypes.contains(c.changeType)).toList();
  }

  // ── Tab ① 监控股票 ─────────────────────────────────────
  List<MonitoredStock> monitoredStocks = [];
  List<Map<String, String>> searchResults = [];
  bool isSearching = false;
  String _searchKeyword = '';
  String get searchKeyword => _searchKeyword;
  set searchKeyword(String val) {
    _searchKeyword = val;
    _triggerSearch();
  }

  // ── Tab ② 持仓异动（监控股票的异动列表） ──────────────
  List<StockChange> watchChanges = [];
  bool watchLoading = false;

  // ── Tab ③ 全市场异动 ───────────────────────────────────
  List<StockChange> allChanges = [];
  bool allLoading = false;

  /// 搜索版本号，用于取消过期请求
  int _searchVersion = 0;

  /// 所有已从服务端获取到的异动 ID
  final Set<int> _knownChangeIds = {};

  /// 已读的异动 ID（按股票分组，内存缓存）
  /// key: 股票代码, value: 该股票已读的异动ID集合
  final Map<String, Set<int>> _readChangeIdsByCode = {};

  /// 有未读异动的股票 code（缓存，避免重复计算）
  final Set<String> _codesWithNewChanges = {};

  // ── 已读逻辑（手动标记，按股票分组持久化）───────────────

  /// 获取指定股票的已读异动ID（按需加载）
  Future<Set<int>> _getReadChangesForCode(String code) async {
    if (_readChangeIdsByCode.containsKey(code)) {
      return _readChangeIdsByCode[code]!;
    }
    // 从本地加载
    final prefs = await SharedPreferences.getInstance();
    final raw = prefs.getStringList('read_change_ids_$code');
    final readIds = raw != null ? raw.map(int.parse).toSet() : <int>{};
    _readChangeIdsByCode[code] = readIds;
    return readIds;
  }

  /// 保存指定股票的已读状态到本地
  Future<void> _saveReadStateForCode(String code, Set<int> readIds) async {
    final prefs = await SharedPreferences.getInstance();
    if (readIds.isEmpty) {
      await prefs.remove('read_change_ids_$code');
    } else {
      await prefs.setStringList(
        'read_change_ids_$code',
        readIds.map((id) => id.toString()).toList(),
      );
    }
  }

  /// 检查指定股票是否有未读异动
  bool hasNewChanges(String code) => _codesWithNewChanges.contains(code);

  /// 检查单个异动是否已读（公开方法）
  bool isChangeRead(StockChange change) {
    return _isChangeRead(change);
  }

  /// 检查单个异动是否已读（内部方法）
  bool _isChangeRead(StockChange change) {
    final readIds = _readChangeIdsByCode[change.stockCode];
    return readIds != null && readIds.contains(change.id);
  }

  /// 批量预加载监控股票的已读状态
  Future<void> _preloadReadStates(List<String> codes) async {
    await Future.wait(
      codes.map((code) => _getReadChangesForCode(code)),
    );
  }

  /// 重新计算哪些股票有未读异动
  void _recalcNewChanges() {
    _codesWithNewChanges.clear();
    for (final c in watchChanges) {
      if (!_isChangeRead(c)) {
        _codesWithNewChanges.add(c.stockCode);
      }
    }
  }

  /// 标记某只股票的异动为已读（保留兼容）
  void markChangesSeen(String code) {
    _codesWithNewChanges.remove(code);
    notifyListeners();
  }

  /// 标记某只股票的所有异动为已读（手动点击才触发）
  Future<void> markAllChangesExposedForCode(String code) async {
    // 获取该股票当前的已读列表
    final readIds = await _getReadChangesForCode(code);
    // 将该股票的所有异动ID加入已读列表
    for (final c in watchChanges) {
      if (c.stockCode == code) {
        readIds.add(c.id);
      }
    }
    for (final c in allChanges) {
      if (c.stockCode == code) {
        readIds.add(c.id);
      }
    }
    // 更新内存缓存
    _readChangeIdsByCode[code] = readIds;
    // 只保存该股票的已读状态，不影响其他股票
    await _saveReadStateForCode(code, readIds);
    // 重新计算未读状态
    _recalcNewChanges();
    notifyListeners();
    // 清除该股票的所有通知
    await cancelStockNotifications(code);
  }

  // ── 定时刷新 ──────────────────────────────────────────

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
      await loadSelectedTypes();
      if (monitoredStocks.isNotEmpty) {
        final codes = monitoredStocks.map((s) => s.code).toList();

        final results = await Future.wait([
          _repository.fetchRealtimeQuotes(codes),
          _repository.getLatestChanges(codes),
          _preloadReadStates(codes),
        ]);
        final quotes = results[0] as Map<String, Map<String, dynamic>>;
        final newChanges = results[1] as List<StockChange>;
        final filtered = filterChanges(newChanges);
        watchChanges = filtered;
        _detectNewChanges(filtered);

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
                createdAt: s.createdAt,
                mainForceNetInflow:
                    (q['mainForceNetInflow'] as num?)?.toDouble() ?? s.mainForceNetInflow,
                mainForceNetRatio:
                    (q['mainForceNetRatio'] as num?)?.toDouble() ?? s.mainForceNetRatio,
              );
            }
            return s;
          }).toList();
        }
      } else {
        final newChanges = await _repository.getLatestChanges([]);
        final filtered = filterChanges(newChanges);
        watchChanges = filtered;
        _detectNewChanges(filtered);
      }

      notifyListeners();
    } catch (_) {}
  }

  /// 检测并标记新异动（根据已读状态判断）
  void _detectNewChanges(List<StockChange> changes) {
    for (final change in changes) {
      if (!_knownChangeIds.contains(change.id)) {
        _knownChangeIds.add(change.id);
      }
    }
    // 根据已读状态重新计算哪些股票有未读异动
    _recalcNewChanges();
    // 对未读异动触发通知
    _triggerNotifications(changes);
  }

  /// 触发未读异动的通知
  void _triggerNotifications(List<StockChange> changes) {
    for (final change in changes) {
      if (!_isChangeRead(change)) {
        showStockChangeNotificationWithGroup(change: change);
      }
    }
  }

  // ── 搜索 ──────────────────────────────────────────────

  void _triggerSearch() {
    _debounce?.cancel();
    if (_searchKeyword.trim().isEmpty) {
      searchResults = [];
      isSearching = false;
      notifyListeners();
      return;
    }
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

  // ── 数据加载 ──────────────────────────────────────────

  /// 加载监控股票列表 + Tab② 初始异动
  Future<void> loadMonitoredStocks() async {
    await loadSelectedTypes();
    monitoredStocks = await _repository.getMonitoredStocks();
    if (monitoredStocks.isNotEmpty) {
      debugPrint('[Radar] loadMonitoredStocks: ${monitoredStocks.length} items');
      for (final s in monitoredStocks) {
        debugPrint('  ${s.code} ${s.name} createdAt=${s.createdAt}');
      }

      final codes = monitoredStocks.map((s) => s.code).toList();
      final results = await Future.wait([
        _repository.fetchRealtimeQuotes(codes),
        _repository.getLatestChanges(codes),
        _preloadReadStates(codes),
      ]);
      final quotes = results[0] as Map<String, Map<String, dynamic>>;
      final changes = results[1] as List<StockChange>;

      final filtered = filterChanges(changes);

      // 初始加载的异动标记为"已知"但不算"曝光"，同时检测新异动触发红点
      watchChanges = filtered;
      _detectNewChanges(filtered);

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
              createdAt: s.createdAt,
              mainForceNetInflow:
                  (q['mainForceNetInflow'] as num?)?.toDouble() ?? s.mainForceNetInflow,
              mainForceNetRatio:
                  (q['mainForceNetRatio'] as num?)?.toDouble() ?? s.mainForceNetRatio,
            );
          }
          return s;
        }).toList();
      }

      // 按添加时间倒序排列（新添加在最前），无时间戳的排末尾
      monitoredStocks.sort((a, b) {
        final ta = a.createdAt ?? '';
        final tb = b.createdAt ?? '';
        debugPrint('[Radar] sort: ${a.code}($ta) vs ${b.code}($tb) => ${tb.compareTo(ta)}');
        return tb.compareTo(ta);
      });
      // 排序后验证
      debugPrint('[Radar] after sort top=${monitoredStocks.first.code}(${monitoredStocks.first.createdAt}) bottom=${monitoredStocks.last.code}(${monitoredStocks.last.createdAt})');
    }
    notifyListeners();
  }

  /// 加载 Tab③ 全市场异动
  Future<void> loadAllChanges() async {
    await loadSelectedTypes();
    allLoading = true;
    notifyListeners();
    try {
      final changes = await _repository.getAllChanges();
      allChanges = filterChanges(changes);
      // 全市场异动不自动加入 _knownChangeIds（不触发红点）
    } catch (e) {
      debugPrint('加载全市场异动失败: $e');
    }
    allLoading = false;
    notifyListeners();
  }

  static const int maxMonitoredCount = 20;

  /// 添加监控股票
  Future<bool> addMonitoredStock(MonitoredStock stock) async {
    if (monitoredStocks.any((s) => s.code == stock.code)) {
      return false;
    }
    if (monitoredStocks.length >= maxMonitoredCount) {
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
    _codesWithNewChanges.remove(code);
    await loadMonitoredStocks();
  }

  /// 加载最新的监控异动数据
  Future<void> loadWatchChanges() async {
    await loadSelectedTypes();
    watchLoading = true;
    notifyListeners();
    try {
      final codes = monitoredStocks.map((s) => s.code).toList();
      final results = await Future.wait([
        _repository.getLatestChanges(codes),
        _preloadReadStates(codes),
      ]);
      final changes = results[0] as List<StockChange>;
      final filtered = filterChanges(changes);
      watchChanges = filtered;
      _detectNewChanges(filtered);
    } catch (e) {
      debugPrint('加载持仓异动失败: $e');
    }
    watchLoading = false;
    notifyListeners();
  }

  @override
  void dispose() {
    _debounce?.cancel();
    _refreshTimer?.cancel();
    super.dispose();
  }
}
