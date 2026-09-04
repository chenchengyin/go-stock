import 'dart:async';
import 'package:dio/dio.dart';
import 'package:flutter/foundation.dart';
import 'package:trading_app/core/network/api_client.dart';
import 'package:trading_app/features/radar/data/radar_repository.dart';

/// 主板策略 UI 态
enum T0StrategyPhase { historical, waiting, candidatePreview, confirmed }

typedef T0QuoteFetcher =
    Future<Map<String, Map<String, dynamic>>> Function(List<String> codes);

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
  final double patternWinPct; // 形态达标率(%) T0≥2.5%
  final double patternFailPct; // 形态真亏率(%) T0<0，库内口径；展示用赚率
  final String buySignal; // blue | orange | green | yellow | red | insufficient

  /// 赚率：T0≥0 的比例 = 100% − 真亏率（含未达标的小赚和打平，不等于达标率）。
  double get patternEarnPct => 100 - patternFailPct;

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
  final String? backfillDate;
  final String? backfillPhase;

  const T0WarmProgress({
    required this.status,
    this.stockCount = 0,
    this.dailyFetched = 0,
    this.dailyTotal = 0,
    this.candidateCount = 0,
    this.prewarm = false,
    this.backfillDate,
    this.backfillPhase,
  });

  bool get isWarming => status == 'warming';
  bool get isReady => status == 'ready';
  bool get isFailed => status == 'failed';
}

/// 主板策略（T0 开盘日线选股）ViewModel
const t0PurpleStrategyModuleCode = 'radar.purple_strategy';
const t0MainStrategyModuleCode = 'radar.main_strategy';
const t0BlueStrategyModuleCode = 'radar.blue_strategy';

typedef T0Request =
    Future<Map<String, dynamic>> Function(
      String moduleCode,
      Map<String, dynamic> query,
    );

@immutable
class T0ModuleState {
  const T0ModuleState({
    required this.results,
    required this.candidates,
    required this.loading,
    required this.error,
    required this.warmProgress,
    required this.displayDate,
    required this.showingHistorical,
    required this.availableDates,
    required this.selectedDate,
    required this.phase,
    required this.loaded,
  });

  final List<T0StrategyStock> results;
  final List<T0StrategyStock> candidates;
  final bool loading;
  final String? error;
  final T0WarmProgress? warmProgress;
  final String? displayDate;
  final bool showingHistorical;
  final List<String> availableDates;
  final String? selectedDate;
  final T0StrategyPhase phase;
  final bool loaded;

  bool get showingCandidatePreview => phase == T0StrategyPhase.candidatePreview;

  bool get needsPolling =>
      warmProgress != null &&
      (warmProgress!.isWarming || warmProgress!.prewarm);
}

class _MutableT0ModuleState {
  List<T0StrategyStock> results = [];
  List<T0StrategyStock> candidates = [];
  bool loading = false;
  String? error;
  T0WarmProgress? warmProgress;
  Timer? pollTimer;
  Timer? quoteTimer;
  bool warmUpTriggered = false;
  String? displayDate;
  bool showingHistorical = false;
  List<String> availableDates = [];
  String? selectedDate;
  T0StrategyPhase phase = T0StrategyPhase.waiting;
  bool loaded = false;
}

class T0StrategyViewModel extends ChangeNotifier {
  T0StrategyViewModel({
    DateTime Function()? now,
    T0QuoteFetcher? fetchRealtimeQuotes,
    T0Request? request,
    void Function(String moduleCode)? onModuleForbidden,
  }) : _now = now,
       _fetchRealtimeQuotes = fetchRealtimeQuotes,
       _request = request,
       _onModuleForbidden = onModuleForbidden;

  final DateTime Function()? _now;
  final T0QuoteFetcher? _fetchRealtimeQuotes;
  final T0Request? _request;
  final void Function(String moduleCode)? _onModuleForbidden;
  final Map<String, _MutableT0ModuleState> _states = {
    t0PurpleStrategyModuleCode: _MutableT0ModuleState(),
    t0MainStrategyModuleCode: _MutableT0ModuleState(),
    t0BlueStrategyModuleCode: _MutableT0ModuleState(),
  };

  _MutableT0ModuleState _mutableState(String moduleCode) {
    return _states.putIfAbsent(moduleCode, _MutableT0ModuleState.new);
  }

  T0ModuleState stateFor(String moduleCode) {
    final state = _mutableState(moduleCode);
    return T0ModuleState(
      results: List.unmodifiable(state.results),
      candidates: List.unmodifiable(state.candidates),
      loading: state.loading,
      error: state.error,
      warmProgress: state.warmProgress,
      displayDate: state.displayDate,
      showingHistorical: state.showingHistorical,
      availableDates: List.unmodifiable(state.availableDates),
      selectedDate: state.selectedDate,
      phase: state.phase,
      loaded: state.loaded,
    );
  }

  List<T0StrategyStock> resultsFor(String moduleCode) =>
      List.unmodifiable(_mutableState(moduleCode).results);

  List<T0StrategyStock> purpleResultsFor(String moduleCode) =>
      List.unmodifiable(_purpleFilter(resultsFor(moduleCode)));

  List<T0StrategyStock> blueResultsFor(String moduleCode) => List.unmodifiable(
    resultsFor(moduleCode).where((stock) => stock.buySignal == 'blue'),
  );

  // 兼容仍使用主板策略默认 getter 的旧页面和测试；新代码应按模块编码读取。
  List<T0StrategyStock> get results => resultsFor(t0MainStrategyModuleCode);

  List<T0StrategyStock> get purpleResults {
    final purple = _mutableState(t0PurpleStrategyModuleCode);
    final source = purple.loaded
        ? purple.results
        : _mutableState(t0MainStrategyModuleCode).results;
    return List.unmodifiable(_purpleFilter(source));
  }

  List<T0StrategyStock> get blueResults {
    final blue = _mutableState(t0BlueStrategyModuleCode);
    final source = blue.loaded
        ? blue.results
        : _mutableState(t0MainStrategyModuleCode).results;
    return List.unmodifiable(
      source.where((stock) => stock.buySignal == 'blue'),
    );
  }

  bool get loading => _mutableState(t0MainStrategyModuleCode).loading;
  String? get error => _mutableState(t0MainStrategyModuleCode).error;
  T0WarmProgress? get warmProgress =>
      _mutableState(t0MainStrategyModuleCode).warmProgress;
  T0StrategyPhase get phase => _mutableState(t0MainStrategyModuleCode).phase;
  bool get showingCandidatePreview =>
      stateFor(t0MainStrategyModuleCode).showingCandidatePreview;
  bool get isQuotePolling =>
      _mutableState(t0MainStrategyModuleCode).quoteTimer != null;

  /// 当前展示的归档日期（历史结果时为归档实际日期，否则为 null）
  String? get displayDate =>
      _mutableState(t0MainStrategyModuleCode).displayDate;

  /// 是否正在展示历史归档（凌晨窗口或手动切到归档日）
  bool get showingHistorical =>
      _mutableState(t0MainStrategyModuleCode).showingHistorical;

  /// 服务端已有选股归档日期（降序）
  List<String> get availableDates =>
      List.unmodifiable(_mutableState(t0MainStrategyModuleCode).availableDates);

  /// 当前选中的展示日期
  String? get selectedDate =>
      _mutableState(t0MainStrategyModuleCode).selectedDate;

  /// 下拉选项：归档日 +（今日正式结果且不在列表时）把今日放首位
  List<String> dropdownDatesFor(String moduleCode) {
    final state = _mutableState(moduleCode);
    final out = List<String>.from(state.availableDates);
    if (state.selectedDate != null && !out.contains(state.selectedDate)) {
      out.insert(0, state.selectedDate!);
    }
    return out;
  }

  List<String> get dropdownDates => dropdownDatesFor(t0MainStrategyModuleCode);

  /// 有可选归档日期时显示顶部日期条（候选预览不显示；预热/等待时也可切换历史日）
  bool showDateSelectorFor(String moduleCode) {
    final state = _mutableState(moduleCode);
    return state.phase != T0StrategyPhase.candidatePreview &&
        dropdownDatesFor(moduleCode).isNotEmpty &&
        state.selectedDate != null;
  }

  bool get showDateSelector => showDateSelectorFor(t0MainStrategyModuleCode);

  /// 测试用同步入口：直接套用一次响应体
  @visibleForTesting
  void applyResponseForTest(
    Map<String, dynamic> data, {
    String moduleCode = t0MainStrategyModuleCode,
  }) {
    _applyResponse(moduleCode, data, null);
    notifyListeners();
  }

  @visibleForTesting
  void applyAvailableDatesForTest(
    List<String> dates, {
    String moduleCode = t0MainStrategyModuleCode,
  }) {
    _mutableState(moduleCode).availableDates = List<String>.from(dates);
    notifyListeners();
  }

  @visibleForTesting
  void applyQuotesForTest(
    Map<String, Map<String, dynamic>> quotes, {
    String moduleCode = t0MainStrategyModuleCode,
  }) {
    _mergeQuotes(moduleCode, quotes);
    notifyListeners();
  }

  /// 是否正在预热或需要轮询
  bool get needsPolling => stateFor(t0MainStrategyModuleCode).needsPolling;

  /// App 启动时触发预热（当天已预热则后端跳过，静默调用不更新 UI）
  Future<void> warmUpIfNeeded({
    String moduleCode = t0MainStrategyModuleCode,
  }) async {
    final state = _mutableState(moduleCode);
    if (state.warmUpTriggered) return;
    state.warmUpTriggered = true;
    try {
      await _sendRequest(moduleCode, {'prewarm': '1'});
    } catch (error) {
      if (_isModuleForbidden(error)) _forbidModule(moduleCode);
    }
  }

  /// 拉取可用归档日期列表
  Future<void> loadAvailableDates({
    String moduleCode = t0MainStrategyModuleCode,
  }) async {
    try {
      final data = await _sendRequest(moduleCode, {'list_dates': '1'});
      final raw = data['dates'] as List<dynamic>? ?? [];
      final state = _mutableState(moduleCode);
      state.availableDates = raw.map((e) => e.toString()).toList();
      if (state.selectedDate == null && state.availableDates.isNotEmpty) {
        final today = _shanghaiToday();
        state.selectedDate = state.availableDates.contains(today)
            ? today
            : state.availableDates.first;
      }
      notifyListeners();
    } catch (error) {
      if (_isModuleForbidden(error)) _forbidModule(moduleCode);
      // 列表失败不影响主列表展示
    }
  }

  /// 归档列表中比当前选中更旧的一项；已在最早归档时为 null
  String? previousArchiveDateFor(String moduleCode) {
    final selected = _mutableState(moduleCode).selectedDate;
    if (selected == null) return null;
    final dates = dropdownDatesFor(moduleCode);
    final index = dates.indexOf(selected);
    if (index < 0 || index + 1 >= dates.length) return null;
    return dates[index + 1];
  }

  String? get previousArchiveDate =>
      previousArchiveDateFor(t0MainStrategyModuleCode);

  /// 归档列表中比当前选中更新的一项；已在最新项时为 null
  String? nextArchiveDateFor(String moduleCode) {
    final selected = _mutableState(moduleCode).selectedDate;
    if (selected == null) return null;
    final dates = dropdownDatesFor(moduleCode);
    final index = dates.indexOf(selected);
    if (index <= 0) return null;
    return dates[index - 1];
  }

  String? get nextArchiveDate => nextArchiveDateFor(t0MainStrategyModuleCode);

  bool canGoPreviousArchiveFor(String moduleCode) =>
      previousArchiveDateFor(moduleCode) != null &&
      !_mutableState(moduleCode).loading;

  bool get canGoPreviousArchive =>
      canGoPreviousArchiveFor(t0MainStrategyModuleCode);

  bool canGoNextArchiveFor(String moduleCode) =>
      nextArchiveDateFor(moduleCode) != null &&
      !_mutableState(moduleCode).loading;

  bool get canGoNextArchive => canGoNextArchiveFor(t0MainStrategyModuleCode);

  Future<void> selectPreviousArchive(String moduleCode) async {
    final date = previousArchiveDateFor(moduleCode);
    if (date == null) return;
    await selectDate(moduleCode, date);
  }

  Future<void> selectNextArchive(String moduleCode) async {
    final date = nextArchiveDateFor(moduleCode);
    if (date == null) return;
    await selectDate(moduleCode, date);
  }

  /// 切换展示日期：今日走正式选股；其他日走 archived
  Future<void> selectDate(String moduleCode, String date) async {
    final state = _mutableState(moduleCode);
    if (date.isEmpty || date == state.selectedDate) return;
    final today = _shanghaiToday();
    if (date == today) {
      await loadResults(moduleCode: moduleCode);
    } else {
      await loadResults(moduleCode: moduleCode, date: date, archived: true);
    }
  }

  /// 加载 T0 选股结果
  Future<void> loadResults({
    String moduleCode = t0MainStrategyModuleCode,
    String? date,
    bool archived = false,
  }) async {
    final state = _mutableState(moduleCode);
    if (state.loading) return;

    state.loading = true;
    state.error = null;
    notifyListeners();

    try {
      final queryParams = <String, dynamic>{'module_code': moduleCode};
      if (date != null && date.isNotEmpty) {
        queryParams['date'] = date;
      }
      if (archived) {
        queryParams['archived'] = '1';
      }

      final data = await _sendRequest(moduleCode, queryParams);
      _applyResponse(moduleCode, data, date);
      if (_mutableState(moduleCode).phase != T0StrategyPhase.candidatePreview) {
        await loadAvailableDates(moduleCode: moduleCode);
      }
    } catch (error) {
      if (_isModuleForbidden(error)) {
        _forbidModule(moduleCode);
      } else {
        state.error = error.toString();
        state.results = [];
        state.candidates = [];
        state.warmProgress = null;
      }
    } finally {
      state.loading = false;
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

  /// 解析一次 /api/t0-selection 响应，更新指定模块的结果与预热/历史状态。
  void _applyResponse(
    String moduleCode,
    Map<String, dynamic> data,
    String? date,
  ) {
    final state = _mutableState(moduleCode);
    state.loaded = true;
    final status = data['status'] as String?;
    final prewarm = data['prewarm'] as bool? ?? false;
    final archived = data['archived'] as bool? ?? false;
    final rawList = data['results'] as List<dynamic>?;
    final rawCands = data['candidates'] as List<dynamic>?;

    if (prewarm || (status != null && status != 'ready')) {
      state.warmProgress = T0WarmProgress(
        status: status ?? 'warming',
        stockCount: (data['stock_count'] as num?)?.toInt() ?? 0,
        dailyFetched: (data['daily_fetched'] as num?)?.toInt() ?? 0,
        dailyTotal: (data['daily_total'] as num?)?.toInt() ?? 0,
        candidateCount:
            (data['candidate_count'] as num?)?.toInt() ??
            (rawCands?.length ?? 0),
        prewarm: prewarm,
        backfillDate: data['backfill_date'] as String?,
        backfillPhase: data['backfill_phase'] as String?,
      );

      if (state.warmProgress!.isReady &&
          rawList != null &&
          (data['historical'] as bool? ?? false || rawList.isNotEmpty)) {
        // ready 且带 results：凌晨窗口的历史归档，直接展示
        state.results = rawList
            .map((e) => T0StrategyStock.fromJson(e as Map<String, dynamic>))
            .toList();
        state.candidates = [];
        state.displayDate = data['display_date'] as String?;
        state.showingHistorical = data['historical'] as bool? ?? false;
        state.selectedDate = state.displayDate ?? data['date'] as String?;
        state.phase = T0StrategyPhase.historical;
        state.warmProgress = null;
        _stopQuotePolling(moduleCode);
        _stopPolling(moduleCode);
        return;
      }

      if (state.warmProgress!.isReady && rawCands != null) {
        state.candidates = rawCands
            .map((e) => T0StrategyStock.fromJson(e as Map<String, dynamic>))
            .toList();
        final hm = _shanghaiMinutes();
        if (hm >= _previewStartHM && hm < _confirmedStartHM) {
          state.phase = T0StrategyPhase.candidatePreview;
          state.results = List<T0StrategyStock>.from(state.candidates);
          state.displayDate = null;
          state.showingHistorical = false;
          state.selectedDate = data['date'] as String? ?? date;
          state.warmProgress = null;
          _startPollingIfNeeded(moduleCode, date);
          _startQuotePolling(moduleCode);
          return;
        }
        // <09:15 或 ≥09:25：有名单但不展示预览
        state.results = [];
        state.phase = T0StrategyPhase.waiting;
        _stopQuotePolling(moduleCode);
        _startPollingIfNeeded(moduleCode, date);
        return;
      }

      if (state.warmProgress!.isReady && rawList == null) {
        state.phase = T0StrategyPhase.waiting;
        _startPollingIfNeeded(moduleCode, date);
      } else if (state.warmProgress!.isWarming) {
        state.phase = T0StrategyPhase.waiting;
        _startPollingIfNeeded(moduleCode, date);
      } else if (state.warmProgress!.isFailed) {
        state.error = data['error'] as String? ?? '预热失败';
        state.phase = T0StrategyPhase.waiting;
        _stopQuotePolling(moduleCode);
        _stopPolling(moduleCode);
      } else {
        _stopQuotePolling(moduleCode);
        _stopPolling(moduleCode);
      }
      return;
    }

    if (rawList != null) {
      state.results = rawList
          .map((e) => T0StrategyStock.fromJson(e as Map<String, dynamic>))
          .toList();
      state.candidates = [];
      if (archived || (data['historical'] as bool? ?? false)) {
        state.displayDate =
            (data['display_date'] as String?) ??
            (data['date'] as String?) ??
            date;
        state.showingHistorical = true;
        state.selectedDate = state.displayDate;
        state.phase = T0StrategyPhase.historical;
      } else {
        state.displayDate = null;
        state.showingHistorical = false;
        state.selectedDate = data['date'] as String? ?? date;
        state.phase = T0StrategyPhase.confirmed;
      }
      state.warmProgress = null;
      _stopQuotePolling(moduleCode);
      _stopPolling(moduleCode);
      return;
    }

    state.results = [];
    state.candidates = [];
    state.displayDate = null;
    state.showingHistorical = false;
    state.selectedDate = null;
    state.phase = T0StrategyPhase.waiting;
    state.warmProgress = null;
    _stopQuotePolling(moduleCode);
    _stopPolling(moduleCode);
  }

  void _mergeQuotes(
    String moduleCode,
    Map<String, Map<String, dynamic>> quotes,
  ) {
    final state = _mutableState(moduleCode);
    if (state.candidates.isEmpty) return;
    final merged = state.candidates.map((s) {
      final q = quotes[s.rawCode] ?? quotes[s.stockCode];
      final pct = (q?['changePercent'] as num?)?.toDouble();
      return s.copyWith(liveChangePercent: pct);
    }).toList();
    merged.sort((a, b) {
      final br = strategySortRank(a).compareTo(strategySortRank(b));
      if (br != 0) return br;
      final at = a.tag.isNotEmpty ? 0 : 1;
      final bt = b.tag.isNotEmpty ? 0 : 1;
      if (at != bt) return at - bt;
      final am = a.liveChangePercent != null;
      final bm = b.liveChangePercent != null;
      if (am != bm) return am ? -1 : 1;
      if (!am) return 0;
      return b.liveChangePercent!.compareTo(a.liveChangePercent!);
    });
    state.results = merged;
  }

  @visibleForTesting
  static int buySignalSortRank(String buySignal) => buySignal == 'blue'
      ? 0
      : (buySignal == 'orange' ? 1 : (buySignal == 'green' ? 2 : 3));

  @visibleForTesting
  static int strategySortRank(T0StrategyStock stock) {
    final signalRank = buySignalSortRank(stock.buySignal);
    if (signalRank < 3) return signalRank;

    switch (stock.tag) {
      case '涨停破板':
        return 3;
      case '前一天跌停':
        return 4;
      case '前一天大阴线':
        return 5;
      default:
        return 6;
    }
  }

  @visibleForTesting
  static List<T0StrategyStock> sortStrategyStocksForDisplay(
    List<T0StrategyStock> list, {
    required double? Function(T0StrategyStock s) liveChangePercent,
    bool preview = false,
  }) {
    final out = List<T0StrategyStock>.from(list);
    out.sort((a, b) {
      final br = strategySortRank(a).compareTo(strategySortRank(b));
      if (br != 0) return br;
      final at = a.tag.isNotEmpty ? 0 : 1;
      final bt = b.tag.isNotEmpty ? 0 : 1;
      if (at != bt) return at - bt;
      if (preview) {
        final am = liveChangePercent(a) != null;
        final bm = liveChangePercent(b) != null;
        if (am != bm) return am ? -1 : 1;
        if (am && bm) {
          return liveChangePercent(b)!.compareTo(liveChangePercent(a)!);
        }
      }
      return b.openGap.compareTo(a.openGap);
    });
    return out;
  }

  void _startPollingIfNeeded(String moduleCode, String? date) {
    final state = _mutableState(moduleCode);
    state.pollTimer?.cancel();
    state.pollTimer = Timer.periodic(const Duration(seconds: 10), (_) {
      loadResults(moduleCode: moduleCode, date: date);
    });
  }

  void _stopPolling(String moduleCode) {
    final state = _mutableState(moduleCode);
    state.pollTimer?.cancel();
    state.pollTimer = null;
  }

  void _startQuotePolling(String moduleCode) {
    final state = _mutableState(moduleCode);
    state.quoteTimer?.cancel();
    state.quoteTimer = Timer.periodic(const Duration(seconds: 10), (_) {
      _refreshQuotes(moduleCode);
    });
    if (_fetchRealtimeQuotes != null) {
      _refreshQuotes(moduleCode);
    }
  }

  void _stopQuotePolling(String moduleCode) {
    final state = _mutableState(moduleCode);
    state.quoteTimer?.cancel();
    state.quoteTimer = null;
  }

  Future<void> _refreshQuotes(String moduleCode) async {
    final state = _mutableState(moduleCode);
    if (state.phase != T0StrategyPhase.candidatePreview ||
        state.candidates.isEmpty) {
      return;
    }
    final codes = state.candidates
        .map((e) => e.rawCode)
        .where((c) => c.isNotEmpty)
        .toList();
    if (codes.isEmpty) return;
    try {
      final fetcher =
          _fetchRealtimeQuotes ??
          (cs) => RadarRepositoryImpl().fetchRealtimeQuotes(cs);
      final merged = <String, Map<String, dynamic>>{};
      for (var i = 0; i < codes.length; i += _quoteBatchSize) {
        final end = i + _quoteBatchSize > codes.length
            ? codes.length
            : i + _quoteBatchSize;
        final part = await fetcher(codes.sublist(i, end));
        merged.addAll(part);
      }
      _mergeQuotes(moduleCode, merged);
      notifyListeners();
    } catch (_) {
      // 行情失败时保留当前列表顺序
    }
  }

  Future<Map<String, dynamic>> _sendRequest(
    String moduleCode,
    Map<String, dynamic> query,
  ) async {
    final normalizedQuery = <String, dynamic>{
      ...query,
      'module_code': moduleCode,
    };
    if (_request != null) return _request(moduleCode, normalizedQuery);

    final response = await createApiClient().get<dynamic>(
      '/api/t0-selection',
      queryParameters: normalizedQuery,
    );
    final data = response.data;
    if (data is! Map) {
      throw const FormatException('T0 response must be an object');
    }
    return Map<String, dynamic>.from(data);
  }

  bool _isModuleForbidden(Object error) {
    if (error is! DioException || error.response?.statusCode != 403) {
      return false;
    }
    final data = error.response?.data;
    return data is Map && data['code'] == 'MODULE_FORBIDDEN';
  }

  void _forbidModule(String moduleCode) {
    final state = _mutableState(moduleCode);
    state.results = [];
    state.candidates = [];
    state.error = null;
    state.warmProgress = null;
    state.phase = T0StrategyPhase.waiting;
    _stopQuotePolling(moduleCode);
    _stopPolling(moduleCode);
    _onModuleForbidden?.call(moduleCode);
  }

  static List<T0StrategyStock> _purpleFilter(Iterable<T0StrategyStock> stocks) {
    return stocks
        .where(
          (stock) =>
              stock.patternT0N >= 2 &&
              stock.patternWinPct >= 30 &&
              stock.patternEarnPct > 60,
        )
        .toList();
  }

  @override
  void dispose() {
    for (final moduleCode in _states.keys) {
      _stopQuotePolling(moduleCode);
      _stopPolling(moduleCode);
    }
    super.dispose();
  }
}
