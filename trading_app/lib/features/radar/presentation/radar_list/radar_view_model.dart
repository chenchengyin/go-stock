import 'dart:async';
import 'package:flutter/foundation.dart';
import 'package:trading_app/features/radar/domain/radar_models.dart';
import 'package:trading_app/features/radar/data/radar_repository.dart';

class RadarViewModel extends ChangeNotifier {
  RadarViewModel(this._repository);

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
    super.dispose();
  }
}
