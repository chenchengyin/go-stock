import 'dart:async';
import 'dart:convert';
import 'package:flutter/foundation.dart';
import 'package:shared_preferences/shared_preferences.dart';
import 'package:trading_app/core/stock_local_monitor/stock_local_monitor.dart';
import 'package:trading_app/features/radar/domain/change_type_config.dart';
import 'package:trading_app/features/radar/domain/radar_models.dart';
import 'package:trading_app/features/radar/data/radar_repository.dart';
import 'package:trading_app/features/radar/data/notification_util.dart';

class RadarViewModel extends ChangeNotifier {
  RadarViewModel(this._repository) {
    _startPeriodicRefresh();
  }

  final RadarRepository _repository;
  final StockLocalMonitor _localMonitor = StockLocalMonitor();

  // ── 异动类型筛选 ─────────────────────────────────────
  Set<int> _selectedChangeTypes = ChangeTypeConfig.defaultMonitorIds;

  /// 当前选中的异动类型 ID 集合
  Set<int> get selectedChangeTypes => _selectedChangeTypes;

  /// 从 shared_preferences 加载已选异动类型
  Future<void> loadSelectedTypes() async {
    final prefs = await SharedPreferences.getInstance();
    final raw = prefs.getString(ChangeTypeConfig.storageKey);
    if (raw != null && raw.isNotEmpty) {
      var ids = raw.split(',').map(int.parse).toSet();
      // 一次性迁移：老用户补上本地监控类型（9001~9005）
      if (!ids.contains(9001)) {
        ids.addAll({9002, 9003, 9005, 9006});
        await prefs.setString(
          ChangeTypeConfig.storageKey,
          ids.join(','),
        );
      }
      _selectedChangeTypes = ids;
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

  /// 当日本地监控异动池（跨刷新保留，避免被服务端数据覆盖丢失）
  List<StockChange> _localAlertsToday = [];

  /// 过滤用户已禁用类型的本地异动
  List<StockChange> get _filteredLocalAlerts => filterChanges(_localAlertsToday);

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

  /// 获取指定股票最新的异动描述（用于监控列表展示）
  /// 只返回未读异动，全部已读后不显示
  String? getLatestAlertDescription(String code) {
    final alerts = watchChanges.where((c) => c.stockCode == code && !isChangeRead(c)).toList();
    if (alerts.isEmpty) return null;
    // 按时间倒序取最新一条
    alerts.sort((a, b) => '${b.changeDate}${b.changeTime}'.compareTo('${a.changeDate}${a.changeTime}'));
    final first = alerts.first;
    debugPrint('[getLatestAlertDescription] first.desc=${first.description} typeName=${first.typeName} changeRate=${first.changeRate}');
    // 优先用 description，服务端异动没有则用 typeName + changeRate 拼接
    if (first.description != null && first.description!.isNotEmpty) {
      return first.description;
    }
    final rate = first.changeRate;
    final rateStr = rate >= 0 ? '+${rate.toStringAsFixed(2)}%' : '${rate.toStringAsFixed(2)}%';
    return '${first.typeName} $rateStr';
  }

  /// 检查单个异动是否已读（公开方法）
  bool isChangeRead(StockChange change) {
    return _isChangeRead(change);
  }

  /// 检查单个异动是否已读（内部方法）
  bool _isChangeRead(StockChange change) {
    final readIds = _readChangeIdsByCode[change.stockCode];
    return readIds != null && readIds.contains(change.id);
  }

  /// 只保留最近一个交易日的异动（用行情日期对比）
  /// 如果 monitoredStocks 为空，用本地当前时间推断交易日
  List<StockChange> filterTodayChanges(List<StockChange> changes) {
    String tradingDate;
    if (monitoredStocks.isNotEmpty && monitoredStocks.first.date.isNotEmpty) {
      tradingDate = monitoredStocks.first.date;
    } else {
      // 兜底：用本地时间，周末退到周五
      final now = DateTime.now();
      var d = now;
      if (d.weekday == DateTime.saturday) d = d.subtract(const Duration(days: 1));
      if (d.weekday == DateTime.sunday) d = d.subtract(const Duration(days: 2));
      tradingDate = '${d.year}-${d.month.toString().padLeft(2, '0')}-${d.day.toString().padLeft(2, '0')}';
    }
    return changes.where((c) => c.changeDate == tradingDate).toList();
  }

  /// 合并本地监控异动到当日池（去重 + 日期过滤）
  void _mergeLocalAlerts(List<StockChange> alerts) {
    if (alerts.isEmpty) return;
    for (final alert in alerts) {
      final exists = _localAlertsToday.any((e) => e.id == alert.id);
      if (!exists) {
        _localAlertsToday.add(alert);
      }
    }
    // 只保留今天的
    _localAlertsToday = filterTodayChanges(_localAlertsToday);
    _saveLocalAlerts();
  }

  static const _localAlertsKey = 'local_alerts_today';
  static const _localAlertsVersionKey = 'local_alerts_cache_version';
  /// 缓存版本号：修改异动生成逻辑后递增，自动清除旧缓存
  static const int _localAlertsVersion = 0;

  /// 保存本地异动到 SharedPreferences
  Future<void> _saveLocalAlerts() async {
    final prefs = await SharedPreferences.getInstance();
    final jsonList = _localAlertsToday.map((c) => c.toJson()).toList();
    await prefs.setString(_localAlertsKey, jsonEncode(jsonList));
    await prefs.setInt(_localAlertsVersionKey, _localAlertsVersion);
  }

  /// 从 SharedPreferences 恢复本地异动
  Future<void> _loadLocalAlerts() async {
    final prefs = await SharedPreferences.getInstance();
    final cachedVersion = prefs.getInt(_localAlertsVersionKey) ?? 0;
    if (cachedVersion < _localAlertsVersion) {
      // 缓存版本不匹配，清除旧数据（如成交额计算逻辑变更）
      await prefs.remove(_localAlertsKey);
      _localAlertsToday = [];
      return;
    }
    final raw = prefs.getString(_localAlertsKey);
    if (raw == null || raw.isEmpty) return;
    try {
      final list = jsonDecode(raw) as List<dynamic>;
      _localAlertsToday = list
          .map((e) => StockChange.fromJson(Map<String, dynamic>.from(e)))
          .toList();
      // 跨天清理
      _localAlertsToday = filterTodayChanges(_localAlertsToday);
    } catch (_) {
      _localAlertsToday = [];
    }
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
    _recalcNewChanges();
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
      // 清理上一个交易日的本地异动
      _localAlertsToday = filterTodayChanges(_localAlertsToday);
      _saveLocalAlerts();
      if (monitoredStocks.isNotEmpty) {
        final codes = monitoredStocks.map((s) => s.code).toList();

        final results = await Future.wait([
          _repository.fetchRealtimeQuotes(codes),
          _repository.getLatestChanges(codes),
          _preloadReadStates(codes),
        ]);
        final quotes = results[0] as Map<String, Map<String, dynamic>>;
        final newChanges = results[1] as List<StockChange>;
        final todayOnly = filterTodayChanges(newChanges);
        final filtered = filterChanges(todayOnly);

        List<StockChange> newLocalAlerts = [];
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
                serverTime: (q['serverTime'] as num?)?.toInt() ?? s.serverTime,
                date: q['date'] as String? ?? s.date,
                mainForceNetInflow:
                    (q['mainForceNetInflow'] as num?)?.toDouble() ?? s.mainForceNetInflow,
                mainForceNetRatio:
                    (q['mainForceNetRatio'] as num?)?.toDouble() ?? s.mainForceNetRatio,
                dayNetInflow:
                    (q['dayNetInflow'] as num?)?.toDouble() ?? s.dayNetInflow,
                accumNetInflow:
                    (q['accumNetInflow'] as num?)?.toDouble() ?? s.accumNetInflow,
              );
            }
            return s;
          }).toList();

          // 本地监控检测，合并到当日池（跨刷新保留）
          newLocalAlerts = _localMonitor.pushSnapshots(monitoredStocks);
          if (newLocalAlerts.isNotEmpty) {
            _mergeLocalAlerts(newLocalAlerts);
          }
        }

        // 合并服务端异动 + 本地异动池（避免本地异动被覆盖丢失）
        watchChanges = [...filtered, ..._filteredLocalAlerts];
        _detectNewChanges(filtered);
        if (newLocalAlerts.isNotEmpty) {
          _detectNewChanges(newLocalAlerts);
        }
      } else {
        final newChanges = await _repository.getLatestChanges([]);
        final todayOnly = filterTodayChanges(newChanges);
        final filtered = filterChanges(todayOnly);
        watchChanges = filtered;
        _detectNewChanges(filtered);
      }

      notifyListeners();
    } catch (_) {}
  }

  /// 新异动产生时的语音播报回调（由外部注入）
  void Function(StockChange change, {bool urgent})? onNewVoiceChange;

  /// 判断异动是否属于紧急类型（价格急速波动优先插队）
  bool _isUrgentVoiceChange(StockChange change) {
    return change.changeType == 9001 || change.changeType == 9002;
  }

  /// 检测并标记新异动（根据已读状态判断）
  void _detectNewChanges(List<StockChange> changes) {
    final newOnes = <StockChange>[];
    for (final change in changes) {
      if (!_knownChangeIds.contains(change.id)) {
        _knownChangeIds.add(change.id);
        newOnes.add(change);
      }
    }
    // 根据已读状态重新计算哪些股票有未读异动
    _recalcNewChanges();
    // 只对真正的新异动触发通知（避免重复通知）
    _triggerNotifications(newOnes);
    // 触发语音播报
    _triggerVoiceAnnouncements(newOnes);
  }

  /// 触发语音播报：逐个入队，紧急类型插队到下一顺位
  void _triggerVoiceAnnouncements(List<StockChange> changes) {
    final callback = onNewVoiceChange;
    if (callback == null || changes.isEmpty) return;
    for (final change in changes) {
      callback(change, urgent: _isUrgentVoiceChange(change));
    }
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
    await _loadLocalAlerts();
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

      final todayOnly = filterTodayChanges(changes);
      final filtered = filterChanges(todayOnly);

      // 初始加载的异动标记为"已知"但不算"曝光"，同时检测新异动触发红点
      watchChanges = [...filtered, ..._filteredLocalAlerts];
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
              serverTime: (q['serverTime'] as num?)?.toInt() ?? s.serverTime,
              date: q['date'] as String? ?? s.date,
              mainForceNetInflow:
                  (q['mainForceNetInflow'] as num?)?.toDouble() ?? s.mainForceNetInflow,
              mainForceNetRatio:
                  (q['mainForceNetRatio'] as num?)?.toDouble() ?? s.mainForceNetRatio,
              dayNetInflow:
                  (q['dayNetInflow'] as num?)?.toDouble() ?? s.dayNetInflow,
              accumNetInflow:
                  (q['accumNetInflow'] as num?)?.toDouble() ?? s.accumNetInflow,
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
      final todayOnly = filterTodayChanges(changes);
      allChanges = filterChanges(todayOnly);
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
    // 清理该股票的本地异动
    _localAlertsToday = _localAlertsToday.where((c) => c.stockCode != code).toList();
    _saveLocalAlerts();
    await loadMonitoredStocks();
  }

  /// 加载最新的监控异动数据
  Future<void> loadWatchChanges() async {
    await loadSelectedTypes();
    await _loadLocalAlerts();
    watchLoading = true;
    notifyListeners();
    try {
      final codes = monitoredStocks.map((s) => s.code).toList();
      final results = await Future.wait([
        _repository.getLatestChanges(codes),
        _preloadReadStates(codes),
      ]);
      final changes = results[0] as List<StockChange>;
      final todayOnly = filterTodayChanges(changes);
      final filtered = filterChanges(todayOnly);
      watchChanges = [...filtered, ..._filteredLocalAlerts];
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
