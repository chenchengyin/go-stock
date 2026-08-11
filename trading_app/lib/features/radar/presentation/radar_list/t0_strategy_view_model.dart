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

  List<T0StrategyStock> get results => _results;
  bool get loading => _loading;
  String? get error => _error;
  T0WarmProgress? get warmProgress => _warmProgress;

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

  /// 加载 T0 选股结果
  Future<void> loadResults({String? date}) async {
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

      final resp = await dio.get('/api/t0-selection', queryParameters: queryParams);
      final data = resp.data as Map<String, dynamic>;

      // 检查预热/轮询状态
      final status = data['status'] as String?;
      final prewarm = data['prewarm'] as bool? ?? false;
      final rawList = data['results'] as List<dynamic>?;

      if (prewarm || (status != null && status != 'ready')) {
        // 预热中 / 预热进度
        _warmProgress = T0WarmProgress(
          status: status ?? 'warming',
          stockCount: (data['stock_count'] as num?)?.toInt() ?? 0,
          dailyFetched: (data['daily_fetched'] as num?)?.toInt() ?? 0,
          dailyTotal: (data['daily_total'] as num?)?.toInt() ?? 0,
          candidateCount: (data['candidate_count'] as num?)?.toInt() ?? 0,
          prewarm: prewarm,
        );

        // 预热完成但没有 results：可能有数据但还没跑选股
        if (_warmProgress!.isReady && rawList != null && rawList.isNotEmpty) {
          _results = rawList
              .map((e) => T0StrategyStock.fromJson(e as Map<String, dynamic>))
              .toList();
          _warmProgress = null;
          _stopPolling();
        } else if (_warmProgress!.isReady && rawList == null) {
          // ready 但无 results：prewarm 阶段，等待正式选股
          // 这类响应只有预热摘要无 results，继续轮询
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
        // 正常数据：停止轮询
        _results = rawList
            .map((e) => T0StrategyStock.fromJson(e as Map<String, dynamic>))
            .toList();
        _warmProgress = null;
        _stopPolling();
      } else {
        // 空结果（不是预热）
        _results = [];
        _warmProgress = null;
        _stopPolling();
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
