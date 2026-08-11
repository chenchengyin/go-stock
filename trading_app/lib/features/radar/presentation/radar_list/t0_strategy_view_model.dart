import 'dart:async';
import 'package:flutter/foundation.dart';
import 'package:trading_app/core/network/api_client.dart';

/// T0 选股结果单条数据
class T0StrategyStock {
  final String stockCode;
  final String stockName;
  final double openGap; // T0开盘涨幅(%)
  final double closeRet; // T0收盘涨幅(%)
  final String limitUpDates; // 涨停日期
  final double ma20;
  final double amountYi; // 成交额(亿)
  final double prevClose; // 前一交易日收盘
  final double prevCloseRet; // 前一交易日收盘涨幅(%)
  final String tag; // 标记：涨停破板 / 前一天跌停 / 前一天大阴线，无标记为空串

  const T0StrategyStock({
    required this.stockCode,
    required this.stockName,
    required this.openGap,
    required this.closeRet,
    required this.limitUpDates,
    required this.ma20,
    required this.amountYi,
    required this.prevClose,
    required this.prevCloseRet,
    this.tag = '',
  });

  factory T0StrategyStock.fromJson(Map<String, dynamic> json) {
    return T0StrategyStock(
      stockCode: json['股票代码'] as String? ?? '',
      stockName: json['股票名称'] as String? ?? '',
      openGap: (json['T0开盘涨幅(%)'] as num?)?.toDouble() ?? 0.0,
      closeRet: (json['T0收盘涨幅(%)'] as num?)?.toDouble() ?? 0.0,
      limitUpDates: json['涨停日期'] as String? ?? '-',
      ma20: (json['MA20'] as num?)?.toDouble() ?? 0.0,
      amountYi: (json['成交额(亿)'] as num?)?.toDouble() ?? 0.0,
      prevClose: (json['前一交易日收盘'] as num?)?.toDouble() ?? 0.0,
      prevCloseRet: (json['前一交易日收盘涨幅(%)'] as num?)?.toDouble() ?? 0.0,
      tag: json['标记'] as String? ?? '',
    );
  }

  /// 去掉 .XSHE/.XSHG 后缀的纯数字代码
  String get rawCode => stockCode.replaceAll(RegExp(r'\..*$'), '');
}

/// 预热进度信息
class T0WarmProgress {
  final String status; // idle / warming / ready / failed
  final int stockCount;
  final int dailyFetched;
  final int dailyTotal;
  final int candidateCount;
  final bool prewarm;

  const T0WarmProgress({
    required this.status,
    this.stockCount = 0,
    this.dailyFetched = 0,
    this.dailyTotal = 0,
    this.candidateCount = 0,
    this.prewarm = false,
  });

  bool get isWarming => status == 'warming';
  bool get isReady => status == 'ready';
  bool get isFailed => status == 'failed';
}

/// 主板策略（T0 开盘日线选股）ViewModel
class T0StrategyViewModel extends ChangeNotifier {
  List<T0StrategyStock> _results = [];
  bool _loading = false;
  String? _error;
  T0WarmProgress? _warmProgress;
  Timer? _pollTimer;
  bool _warmUpTriggered = false;
  String? _displayDate;
  bool _showingHistorical = false;
  List<String> _availableDates = [];
  String? _selectedDate;

  List<T0StrategyStock> get results => _results;
  bool get loading => _loading;
  String? get error => _error;
  T0WarmProgress? get warmProgress => _warmProgress;

  /// 当前展示的归档日期（历史结果时为归档实际日期，否则为 null）
  String? get displayDate => _displayDate;

  /// 是否正在展示历史归档（凌晨窗口或手动切到归档日）
  bool get showingHistorical => _showingHistorical;

  /// 服务端已有选股归档日期（降序）
  List<String> get availableDates => List.unmodifiable(_availableDates);

  /// 当前选中的展示日期
  String? get selectedDate => _selectedDate;

  /// 下拉选项：归档日 +（今日正式结果且不在列表时）把今日放首位
  List<String> get dropdownDates {
    final out = List<String>.from(_availableDates);
    if (_selectedDate != null && !out.contains(_selectedDate)) {
      out.insert(0, _selectedDate!);
    }
    return out;
  }

  /// 有结果且有可选日期时显示顶部日期条
  bool get showDateSelector =>
      _results.isNotEmpty && dropdownDates.isNotEmpty && _selectedDate != null;

  /// 测试用同步入口：直接套用一次响应体
  @visibleForTesting
  void applyResponseForTest(Map<String, dynamic> data) {
    _applyResponse(data, null);
    notifyListeners();
  }

  @visibleForTesting
  void applyAvailableDatesForTest(List<String> dates) {
    _availableDates = List<String>.from(dates);
    notifyListeners();
  }

  /// 是否正在预热或需要轮询
  bool get needsPolling =>
      _warmProgress != null && (_warmProgress!.isWarming || _warmProgress!.prewarm);

  /// App 启动时触发预热（当天已预热则后端跳过，静默调用不更新 UI）
  Future<void> warmUpIfNeeded() async {
    if (_warmUpTriggered) return;
    _warmUpTriggered = true;
    try {
      final dio = createApiClient();
      await dio.get('/api/t0-selection', queryParameters: {'prewarm': '1'});
    } catch (_) {
      // 静默失败，用户切换到主板策略 Tab 时会触发 loadResults
    }
  }

  /// 拉取可用归档日期列表
  Future<void> loadAvailableDates() async {
    try {
      final dio = createApiClient();
      final resp = await dio.get(
        '/api/t0-selection',
        queryParameters: {'list_dates': '1'},
      );
      final data = resp.data as Map<String, dynamic>;
      final raw = data['dates'] as List<dynamic>? ?? [];
      _availableDates = raw.map((e) => e.toString()).toList();
      notifyListeners();
    } catch (_) {
      // 列表失败不影响主列表展示
    }
  }

  /// 切换展示日期：今日走正式选股；其他日走 archived
  Future<void> selectDate(String date) async {
    if (date.isEmpty || date == _selectedDate) return;
    final today = _shanghaiToday();
    if (date == today) {
      await loadResults();
    } else {
      await loadResults(date: date, archived: true);
    }
  }

  /// 加载 T0 选股结果
  Future<void> loadResults({String? date, bool archived = false}) async {
    if (_loading) return;

    _loading = true;
    _error = null;
    notifyListeners();

    try {
      final dio = createApiClient();
      final queryParams = <String, dynamic>{};
      if (date != null && date.isNotEmpty) {
        queryParams['date'] = date;
      }
      if (archived) {
        queryParams['archived'] = '1';
      }

      final resp =
          await dio.get('/api/t0-selection', queryParameters: queryParams);
      final data = resp.data as Map<String, dynamic>;
      _applyResponse(data, date);
      if (_results.isNotEmpty) {
        await loadAvailableDates();
      }
    } catch (e) {
      _error = e.toString();
      _results = [];
      _warmProgress = null;
    } finally {
      _loading = false;
      notifyListeners();
    }
  }

  String _shanghaiToday() {
    final now = DateTime.now().toUtc().add(const Duration(hours: 8));
    final y = now.year.toString().padLeft(4, '0');
    final m = now.month.toString().padLeft(2, '0');
    final d = now.day.toString().padLeft(2, '0');
    return '$y-$m-$d';
  }

  /// 解析一次 /api/t0-selection 响应，更新结果与预热/历史状态
  void _applyResponse(Map<String, dynamic> data, String? date) {
    final status = data['status'] as String?;
    final prewarm = data['prewarm'] as bool? ?? false;
    final archived = data['archived'] as bool? ?? false;
    final rawList = data['results'] as List<dynamic>?;

    if (prewarm || (status != null && status != 'ready')) {
      _warmProgress = T0WarmProgress(
        status: status ?? 'warming',
        stockCount: (data['stock_count'] as num?)?.toInt() ?? 0,
        dailyFetched: (data['daily_fetched'] as num?)?.toInt() ?? 0,
        dailyTotal: (data['daily_total'] as num?)?.toInt() ?? 0,
        candidateCount: (data['candidate_count'] as num?)?.toInt() ?? 0,
        prewarm: prewarm,
      );

      if (_warmProgress!.isReady && rawList != null && rawList.isNotEmpty) {
        // ready 且带 results：凌晨窗口的历史归档，直接展示
        _results = rawList
            .map((e) => T0StrategyStock.fromJson(e as Map<String, dynamic>))
            .toList();
        _displayDate = data['display_date'] as String?;
        _showingHistorical = data['historical'] as bool? ?? false;
        _selectedDate = _displayDate ?? data['date'] as String?;
        _warmProgress = null;
        _stopPolling();
      } else if (_warmProgress!.isReady && rawList == null) {
        // ready 但无 results：预热完成等待正式选股，继续轮询
        _startPollingIfNeeded(date);
      } else if (_warmProgress!.isWarming) {
        _startPollingIfNeeded(date);
      } else if (_warmProgress!.isFailed) {
        _error = data['error'] as String? ?? '预热失败';
        _stopPolling();
      } else {
        _stopPolling();
      }
    } else if (rawList != null) {
      _results = rawList
          .map((e) => T0StrategyStock.fromJson(e as Map<String, dynamic>))
          .toList();
      if (archived || (data['historical'] as bool? ?? false)) {
        _displayDate = (data['display_date'] as String?) ??
            (data['date'] as String?) ??
            date;
        _showingHistorical = true;
        _selectedDate = _displayDate;
      } else {
        _displayDate = null;
        _showingHistorical = false;
        _selectedDate = data['date'] as String? ?? date;
      }
      _warmProgress = null;
      _stopPolling();
    } else {
      // 空结果（非预热）
      _results = [];
      _displayDate = null;
      _showingHistorical = false;
      _selectedDate = null;
      _warmProgress = null;
      _stopPolling();
    }
  }

  void _startPollingIfNeeded(String? date) {
    _pollTimer?.cancel();
    _pollTimer = Timer.periodic(const Duration(seconds: 10), (_) {
      loadResults(date: date);
    });
  }

  void _stopPolling() {
    _pollTimer?.cancel();
    _pollTimer = null;
  }

  @override
  void dispose() {
    _stopPolling();
    super.dispose();
  }
}
