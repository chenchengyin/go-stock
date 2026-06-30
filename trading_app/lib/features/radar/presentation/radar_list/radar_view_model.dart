import 'dart:async';
import 'package:flutter/foundation.dart';
import 'package:trading_app/features/radar/domain/radar_models.dart';
import 'package:trading_app/features/radar/data/radar_repository.dart';

class RadarViewModel extends ChangeNotifier {
  RadarViewModel(this._repository) {
    _startPeriodicRefresh();
  }

  final RadarRepository _repository;

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

  /// 所有已从服务端获取到的异动 ID（用于判断是否是"新"异动）
  final Set<int> _knownChangeIds = {};

  /// 已在屏幕上曝光过的异动 ID（曝光才算已读）
  final Set<int> _exposedChangeIds = {};

  /// 有未曝光异动的股票 code
  final Set<String> _codesWithNewChanges = {};

  // ── 曝光已读逻辑 ─────────────────────────────────────

  /// 检查某个异动是否已曝光
  bool isChangeExposed(int changeId) => _exposedChangeIds.contains(changeId);

  /// 标记异动为已曝光，更新股票红点状态
  void markChangeExposed(int changeId) {
    if (_exposedChangeIds.contains(changeId)) return;
    _exposedChangeIds.add(changeId);

    // 找到此异动对应的股票 code
    // 遍历所有异动列表查找
    String? code;
    for (final c in watchChanges) {
      if (c.id == changeId) {
        code = c.stockCode;
        break;
      }
    }
    if (code == null) {
      for (final c in allChanges) {
        if (c.id == changeId) {
          code = c.stockCode;
          break;
        }
      }
    }

    // 如果该股票的所有异动都已曝光，移除红点
    if (code != null) {
      _recalcNewChangesForCode(code);
    }

    notifyListeners();
  }

  /// 重新计算某个股票是否还有未曝光的异动
  void _recalcNewChangesForCode(String code) {
    final hasUnseen = watchChanges.any(
      (c) => c.stockCode == code && !_exposedChangeIds.contains(c.id),
    );
    if (!hasUnseen) {
      _codesWithNewChanges.remove(code);
    } else {
      _codesWithNewChanges.add(code);
    }
  }

  /// 检查指定股票是否有未曝光异动
  bool hasNewChanges(String code) => _codesWithNewChanges.contains(code);

  /// 标记某只股票的异动为已读（保留兼容，实际对外关闭红点）
  void markChangesSeen(String code) {
    _codesWithNewChanges.remove(code);
    notifyListeners();
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
      if (monitoredStocks.isNotEmpty) {
        final codes = monitoredStocks.map((s) => s.code).toList();

        final results = await Future.wait([
          _repository.fetchRealtimeQuotes(codes),
          _repository.getLatestChanges(codes),
        ]);
        final quotes = results[0] as Map<String, Map<String, dynamic>>;
        final newChanges = results[1] as List<StockChange>;

        _detectNewChanges(newChanges);
        watchChanges = newChanges;

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
              );
            }
            return s;
          }).toList();
        }
      } else {
        final newChanges = await _repository.getLatestChanges([]);
        _detectNewChanges(newChanges);
        watchChanges = newChanges;
      }

      notifyListeners();
    } catch (_) {}
  }

  /// 检测并标记新异动（不在 _exposedChangeIds 中的视为新异动）
  void _detectNewChanges(List<StockChange> changes) {
    for (final change in changes) {
      if (!_knownChangeIds.contains(change.id)) {
        _knownChangeIds.add(change.id);
        if (!_exposedChangeIds.contains(change.id)) {
          _codesWithNewChanges.add(change.stockCode);
        }
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
      ]);
      final quotes = results[0] as Map<String, Map<String, dynamic>>;
      final changes = results[1] as List<StockChange>;

      // 初始加载的异动标记为"已知"但不算"曝光"
      watchChanges = changes;
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
    allLoading = true;
    notifyListeners();
    try {
      final changes = await _repository.getAllChanges();
      allChanges = changes;
      // 全市场异动不自动加入 _knownChangeIds（不触发红点）
    } catch (e) {
      debugPrint('加载全市场异动失败: $e');
    }
    allLoading = false;
    notifyListeners();
  }

  /// 添加监控股票
  Future<bool> addMonitoredStock(MonitoredStock stock) async {
    if (monitoredStocks.any((s) => s.code == stock.code)) {
      return false;
    }
    final result = await _repository.addMonitoredStock(stock);
    if (result == '关注成功') {
      // 全量重载后按后端 createdAt 倒序排序
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
    watchLoading = true;
    notifyListeners();
    try {
      final codes = monitoredStocks.map((s) => s.code).toList();
      final changes = await _repository.getLatestChanges(codes);
      _detectNewChanges(changes);
      watchChanges = changes;
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
