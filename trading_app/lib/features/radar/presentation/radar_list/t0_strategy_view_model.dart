import 'dart:async';
import 'package:flutter/foundation.dart';
import 'package:trading_app/core/network/api_client.dart';
import 'package:trading_app/features/radar/data/radar_repository.dart';

/// 主板策略 UI 态
enum T0StrategyPhase { historical, waiting, candidatePreview, confirmed }

typedef T0QuoteFetcher = Future<Map<String, Map<String, dynamic>>> Function(
  List<String> codes,
);

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
  final double? liveChangePercent; // 候选预览用实时涨幅；null 表示尚无行情
  final String pattern; // 3K 形态键
  final int patternT0N; // 形态样本数
  final double patternWinPct; // 形态达标率(%)
  final double patternFailPct; // 形态真亏率(%)
  final String buySignal; // blue | green | yellow | red | insufficient

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
    this.liveChangePercent,
    this.pattern = '',
    this.patternT0N = 0,
    this.patternWinPct = 0,
    this.patternFailPct = 0,
    this.buySignal = '',
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
      pattern: json['形态'] as String? ?? '',
      patternT0N: (json['形态样本数'] as num?)?.toInt() ?? 0,
      patternWinPct: (json['形态达标率(%)'] as num?)?.toDouble() ?? 0.0,
      patternFailPct: (json['形态真亏率(%)'] as num?)?.toDouble() ?? 0.0,
      buySignal: json['买入信号'] as String? ?? '',
    );
  }

  T0StrategyStock copyWith({double? liveChangePercent}) {
    return T0StrategyStock(
      stockCode: stockCode,
      stockName: stockName,
      openGap: openGap,
      closeRet: closeRet,
      limitUpDates: limitUpDates,
      ma20: ma20,
      amountYi: amountYi,
      prevClose: prevClose,
      prevCloseRet: prevCloseRet,
      tag: tag,
      liveChangePercent: liveChangePercent ?? this.liveChangePercent,
      pattern: pattern,
      patternT0N: patternT0N,
      patternWinPct: patternWinPct,
      patternFailPct: patternFailPct,
      buySignal: buySignal,
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
  T0StrategyViewModel({
    DateTime Function()? now,
    T0QuoteFetcher? fetchRealtimeQuotes,
  })  : _now = now,
        _fetchRealtimeQuotes = fetchRealtimeQuotes;

  final DateTime Function()? _now;
  final T0QuoteFetcher? _fetchRealtimeQuotes;

  List<T0StrategyStock> _results = [];
  List<T0StrategyStock> _candidates = [];
  bool _loading = false;
  String? _error;
  T0WarmProgress? _warmProgress;
  Timer? _pollTimer;
  Timer? _quoteTimer;
  bool _warmUpTriggered = false;
  String? _displayDate;
  bool _showingHistorical = false;
  List<String> _availableDates = [];
  String? _selectedDate;
  T0StrategyPhase _phase = T0StrategyPhase.waiting;

  List<T0StrategyStock> get results => _results;
  bool get loading => _loading;
  String? get error => _error;
  T0WarmProgress? get warmProgress => _warmProgress;
  T0StrategyPhase get phase => _phase;
  bool get showingCandidatePreview =>
      _phase == T0StrategyPhase.candidatePreview;
  bool get isQuotePolling => _quoteTimer != null;

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

  /// 有结果且有可选日期时显示顶部日期条（候选预览不显示）
  bool get showDateSelector =>
      !showingCandidatePreview &&
      _results.isNotEmpty &&
      dropdownDates.isNotEmpty &&
      _selectedDate != null;

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

  @visibleForTesting
  void applyQuotesForTest(Map<String, Map<String, dynamic>> quotes) {
    _mergeQuotes(quotes);
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

  /// 归档列表中比当前选中更旧的一项；已在最早归档时为 null
  String? get previousArchiveDate {
    final selected = _selectedDate;
    if (selected == null) return null;
    final dates = dropdownDates;
    final index = dates.indexOf(selected);
    if (index < 0 || index + 1 >= dates.length) return null;
    return dates[index + 1];
  }

  /// 归档列表中比当前选中更新的一项；已在最新项时为 null
  String? get nextArchiveDate {
    final selected = _selectedDate;
    if (selected == null) return null;
    final dates = dropdownDates;
    final index = dates.indexOf(selected);
    if (index <= 0) return null;
    return dates[index - 1];
  }

  bool get canGoPreviousArchive =>
      previousArchiveDate != null && !_loading;

  bool get canGoNextArchive => nextArchiveDate != null && !_loading;

  Future<void> selectPreviousArchive() async {
    final date = previousArchiveDate;
    if (date == null) return;
    await selectDate(date);
  }

  Future<void> selectNextArchive() async {
    final date = nextArchiveDate;
    if (date == null) return;
    await selectDate(date);
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
      if (_results.isNotEmpty && !showingCandidatePreview) {
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

  DateTime _shanghaiNow() {
    return (_now ?? DateTime.now)().toUtc().add(const Duration(hours: 8));
  }

  String _shanghaiToday() {
    final now = _shanghaiNow();
    final y = now.year.toString().padLeft(4, '0');
    final m = now.month.toString().padLeft(2, '0');
    final d = now.day.toString().padLeft(2, '0');
    return '$y-$m-$d';
  }

  int _shanghaiMinutes() {
    final n = _shanghaiNow();
    return n.hour * 60 + n.minute;
  }

  static const _previewStartHM = 9 * 60 + 15;
  static const _confirmedStartHM = 9 * 60 + 25;
  static const _quoteBatchSize = 80;

  /// 解析一次 /api/t0-selection 响应，更新结果与预热/历史状态
  void _applyResponse(Map<String, dynamic> data, String? date) {
    final status = data['status'] as String?;
    final prewarm = data['prewarm'] as bool? ?? false;
    final archived = data['archived'] as bool? ?? false;
    final rawList = data['results'] as List<dynamic>?;
    final rawCands = data['candidates'] as List<dynamic>?;

    if (prewarm || (status != null && status != 'ready')) {
      _warmProgress = T0WarmProgress(
        status: status ?? 'warming',
        stockCount: (data['stock_count'] as num?)?.toInt() ?? 0,
        dailyFetched: (data['daily_fetched'] as num?)?.toInt() ?? 0,
        dailyTotal: (data['daily_total'] as num?)?.toInt() ?? 0,
        candidateCount: (data['candidate_count'] as num?)?.toInt() ??
            (rawCands?.length ?? 0),
        prewarm: prewarm,
      );

      if (_warmProgress!.isReady && rawList != null && rawList.isNotEmpty) {
        // ready 且带 results：凌晨窗口的历史归档，直接展示
        _results = rawList
            .map((e) => T0StrategyStock.fromJson(e as Map<String, dynamic>))
            .toList();
        _candidates = [];
        _displayDate = data['display_date'] as String?;
        _showingHistorical = data['historical'] as bool? ?? false;
        _selectedDate = _displayDate ?? data['date'] as String?;
        _phase = T0StrategyPhase.historical;
        _warmProgress = null;
        _stopQuotePolling();
        _stopPolling();
        return;
      }

      if (_warmProgress!.isReady && rawCands != null) {
        _candidates = rawCands
            .map((e) => T0StrategyStock.fromJson(e as Map<String, dynamic>))
            .toList();
        final hm = _shanghaiMinutes();
        if (hm >= _previewStartHM && hm < _confirmedStartHM) {
          _phase = T0StrategyPhase.candidatePreview;
          _results = List<T0StrategyStock>.from(_candidates);
          _displayDate = null;
          _showingHistorical = false;
          _selectedDate = data['date'] as String? ?? date;
          _warmProgress = null;
          _startPollingIfNeeded(date);
          _startQuotePolling();
          return;
        }
        // <09:15 或 ≥09:25：有名单但不展示预览
        _results = [];
        _phase = T0StrategyPhase.waiting;
        _stopQuotePolling();
        _startPollingIfNeeded(date);
        return;
      }

      if (_warmProgress!.isReady && rawList == null) {
        _phase = T0StrategyPhase.waiting;
        _startPollingIfNeeded(date);
      } else if (_warmProgress!.isWarming) {
        _phase = T0StrategyPhase.waiting;
        _startPollingIfNeeded(date);
      } else if (_warmProgress!.isFailed) {
        _error = data['error'] as String? ?? '预热失败';
        _phase = T0StrategyPhase.waiting;
        _stopQuotePolling();
        _stopPolling();
      } else {
        _stopQuotePolling();
        _stopPolling();
      }
      return;
    }

    if (rawList != null) {
      _results = rawList
          .map((e) => T0StrategyStock.fromJson(e as Map<String, dynamic>))
          .toList();
      _candidates = [];
      if (archived || (data['historical'] as bool? ?? false)) {
        _displayDate = (data['display_date'] as String?) ??
            (data['date'] as String?) ??
            date;
        _showingHistorical = true;
        _selectedDate = _displayDate;
        _phase = T0StrategyPhase.historical;
      } else {
        _displayDate = null;
        _showingHistorical = false;
        _selectedDate = data['date'] as String? ?? date;
        _phase = T0StrategyPhase.confirmed;
      }
      _warmProgress = null;
      _stopQuotePolling();
      _stopPolling();
      return;
    }

    _results = [];
    _candidates = [];
    _displayDate = null;
    _showingHistorical = false;
    _selectedDate = null;
    _phase = T0StrategyPhase.waiting;
    _warmProgress = null;
    _stopQuotePolling();
    _stopPolling();
  }

  void _mergeQuotes(Map<String, Map<String, dynamic>> quotes) {
    if (_candidates.isEmpty) return;
    final merged = _candidates.map((s) {
      final q = quotes[s.rawCode] ?? quotes[s.stockCode];
      final pct = (q?['changePercent'] as num?)?.toDouble();
      return s.copyWith(liveChangePercent: pct);
    }).toList();
    merged.sort((a, b) {
      final br = buySignalSortRank(a.buySignal).compareTo(buySignalSortRank(b.buySignal));
      if (br != 0) return br;
      final am = a.liveChangePercent != null;
      final bm = b.liveChangePercent != null;
      if (am != bm) return am ? -1 : 1;
      if (!am) return 0;
      return b.liveChangePercent!.compareTo(a.liveChangePercent!);
    });
    _results = merged;
  }

  @visibleForTesting
  static int buySignalSortRank(String buySignal) =>
      buySignal == 'blue' ? 0 : 1;

  @visibleForTesting
  static List<T0StrategyStock> sortStrategyStocksForDisplay(
    List<T0StrategyStock> list, {
    required double? Function(T0StrategyStock s) liveChangePercent,
    bool preview = false,
  }) {
    final out = List<T0StrategyStock>.from(list);
    out.sort((a, b) {
      final br = buySignalSortRank(a.buySignal).compareTo(buySignalSortRank(b.buySignal));
      if (br != 0) return br;
      if (preview) {
        final am = liveChangePercent(a) != null;
        final bm = liveChangePercent(b) != null;
        if (am != bm) return am ? -1 : 1;
        if (am && bm) {
          return liveChangePercent(b)!.compareTo(liveChangePercent(a)!);
        }
      }
      return 0;
    });
    return out;
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

  void _startQuotePolling() {
    _quoteTimer?.cancel();
    _quoteTimer = Timer.periodic(const Duration(seconds: 10), (_) {
      _refreshQuotes();
    });
    if (_fetchRealtimeQuotes != null) {
      _refreshQuotes();
    }
  }

  void _stopQuotePolling() {
    _quoteTimer?.cancel();
    _quoteTimer = null;
  }

  Future<void> _refreshQuotes() async {
    if (!showingCandidatePreview || _candidates.isEmpty) return;
    final codes = _candidates.map((e) => e.rawCode).where((c) => c.isNotEmpty).toList();
    if (codes.isEmpty) return;
    try {
      final fetcher = _fetchRealtimeQuotes ??
          (cs) => RadarRepositoryImpl().fetchRealtimeQuotes(cs);
      final merged = <String, Map<String, dynamic>>{};
      for (var i = 0; i < codes.length; i += _quoteBatchSize) {
        final end = i + _quoteBatchSize > codes.length
            ? codes.length
            : i + _quoteBatchSize;
        final part = await fetcher(codes.sublist(i, end));
        merged.addAll(part);
      }
      _mergeQuotes(merged);
      notifyListeners();
    } catch (_) {
      // 行情失败时保留当前列表顺序
    }
  }

  @override
  void dispose() {
    _stopQuotePolling();
    _stopPolling();
    super.dispose();
  }
}
