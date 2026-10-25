import 'package:flutter/material.dart';
import 'package:trading_app/features/radar/data/red_kline_repository.dart';
import 'package:trading_app/features/radar/data/red_t1_intraday_repository.dart';
import 'strategy_chart_exposure.dart';
import 'strategy_charts.dart';

/// Composes independently loaded daily and intraday panels for one stock.
/// No requests are made until the section is exposed in the list viewport.
class StrategyStockCharts extends StatelessWidget {
  const StrategyStockCharts({
    super.key,
    required this.moduleCode,
    required this.date,
    required this.code,
    required this.showKline,
    required this.showIntraday,
    this.klineRepository,
    this.intradayRepository,
    this.revision = 0,
  });

  final String moduleCode;
  final String date;
  final String code;
  final bool showKline;
  final bool showIntraday;
  final RedKlineRepository? klineRepository;
  final RedT1IntradayRepository? intradayRepository;
  final int revision;

  String get _identity => '$moduleCode:$date:$code:$revision';

  @override
  Widget build(BuildContext context) {
    final intraday = intradayRepository;
    if (!showIntraday || intraday == null) {
      return _dailyPanel(null, false, null, false);
    }
    return StrategyChartExposure<RedT1IntradaySeries>(
      key: ValueKey('intraday:$_identity'),
      detectorId: 'intraday:$_identity',
      load: () async =>
          (await intraday.load(date, [code], moduleCode: moduleCode))[code] ??
          RedT1IntradaySeries(
            code: code,
            date: '',
            bars: const [],
            prevClose: 0,
            source: '',
            status: 'missing_series',
            message: '分时接口未返回该股票的数据条目',
          ),
      shouldRetry: (series) =>
          series.status != 'no_session' &&
          (series.bars.length < 2 || series.prevClose <= 0),
      errorMessage: redT1IntradayErrorMessage,
      builder: (_, series, loading, error, waiting) =>
          _dailyPanel(series, loading, error, waiting),
    );
  }

  Widget _dailyPanel(
    RedT1IntradaySeries? series,
    bool loading,
    String? error,
    bool waiting,
  ) {
    final kline = klineRepository;
    Widget charts(RedKlineWindow? window, bool klineLoading, bool klineError) =>
        StrategyCharts(
          key: ValueKey('charts:$_identity'),
          code: code,
          t0Date: date,
          showKline: showKline && kline != null,
          window: window,
          klineLoading: klineLoading,
          klineError: klineError,
          showIntraday: showIntraday && intradayRepository != null,
          series: series,
          intradayLoading: loading,
          intradayErrorMessage: error,
          intradayIdleMessage: waiting ? '滚动到卡片后自动加载分时' : null,
          loadSession: showIntraday && intradayRepository != null
              ? (sessionDate) => intradayRepository!.loadSession(
                  date,
                  code,
                  sessionDate,
                  moduleCode: moduleCode,
                )
              : null,
          loadKlineWindow: showKline && kline != null
              ? (sessionDate) async => (await kline.load(sessionDate, [
                  code,
                ], moduleCode: moduleCode))[code]
              : null,
        );
    if (!showKline || kline == null) return charts(null, false, false);
    return StrategyChartExposure<RedKlineWindow?>(
      key: ValueKey('kline:$_identity'),
      detectorId: 'kline:$_identity',
      load: () async =>
          (await kline.load(date, [code], moduleCode: moduleCode))[code],
      builder: (_, window, loading, error, waiting) =>
          charts(window, loading, error != null),
    );
  }
}
