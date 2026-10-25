import 'package:flutter/material.dart';
import 'package:trading_app/core/theme/app_colors.dart';
import 'package:trading_app/features/radar/data/red_kline_repository.dart';
import 'package:trading_app/features/radar/data/red_t1_intraday_repository.dart';

import 'red_kline_preview.dart';
import 'red_t1_intraday_preview.dart';

/// Keeps one stock's intraday session and daily-candle selection in sync.
class StrategyCharts extends StatefulWidget {
  const StrategyCharts({
    super.key,
    required this.code,
    required this.t0Date,
    required this.showKline,
    required this.showIntraday,
    this.window,
    this.klineLoading = false,
    this.klineError = false,
    this.series,
    this.intradayLoading = false,
    this.intradayErrorMessage,
    this.intradayIdleMessage,
    this.loadSession,
    this.loadKlineWindow,
  });

  final String code;
  final String t0Date;
  final bool showKline;
  final bool showIntraday;
  final RedKlineWindow? window;
  final bool klineLoading;
  final bool klineError;
  final RedT1IntradaySeries? series;
  final bool intradayLoading;
  final String? intradayErrorMessage;
  final String? intradayIdleMessage;
  final Future<RedT1IntradaySeries> Function(String date)? loadSession;
  final Future<RedKlineWindow?> Function(String date)? loadKlineWindow;

  @override
  State<StrategyCharts> createState() => _StrategyChartsState();
}

class _StrategyChartsState extends State<StrategyCharts> {
  String? _selectedDate;
  RedKlineWindow? _loadedWindow;
  bool _loadingWindow = false;
  bool _windowError = false;
  int _requestVersion = 0;

  @override
  void didUpdateWidget(covariant StrategyCharts oldWidget) {
    super.didUpdateWidget(oldWidget);
    if (oldWidget.code != widget.code ||
        oldWidget.t0Date != widget.t0Date ||
        oldWidget.series != widget.series) {
      _requestVersion++;
      _selectedDate = null;
      _loadedWindow = null;
      _loadingWindow = false;
      _windowError = false;
    }
  }

  void _selectSession(String date) {
    final version = ++_requestVersion;
    setState(() {
      _selectedDate = date;
      _loadedWindow = null;
      _loadingWindow = false;
      _windowError = false;
    });
    if (!widget.showKline ||
        widget.window?.bars.any((bar) => bar.date == date) == true) {
      return;
    }
    final load = widget.loadKlineWindow;
    if (load == null || date.isEmpty) return;
    setState(() => _loadingWindow = true);
    _loadWindow(date, version, load);
  }

  Future<void> _loadWindow(
    String date,
    int version,
    Future<RedKlineWindow?> Function(String date) load,
  ) async {
    try {
      final window = await load(date);
      if (!mounted || version != _requestVersion) return;
      setState(() {
        _loadedWindow =
            window?.code == widget.code &&
                window?.bars.any((bar) => bar.date == date) == true
            ? window
            : null;
        _loadingWindow = false;
      });
    } catch (_) {
      if (!mounted || version != _requestVersion) return;
      setState(() {
        _loadingWindow = false;
        _windowError = true;
      });
    }
  }

  @override
  Widget build(BuildContext context) => Column(
    crossAxisAlignment: CrossAxisAlignment.stretch,
    children: [
      if (widget.showKline)
        RedKlinePreview(
          window: _loadedWindow ?? widget.window,
          loading:
              _loadingWindow || (widget.klineLoading && _loadedWindow == null),
          error: widget.klineError || _windowError,
          selectedDate:
              _selectedDate ??
              (widget.showIntraday ? widget.series?.date : null),
        ),
      if (_windowError && widget.showKline)
        Text(
          '$_selectedDate 日K线加载失败',
          style: TextStyle(fontSize: 11, color: AppColors.warning),
        ),
      if (widget.showIntraday)
        RedT1IntradayPreview(
          series: widget.series,
          loading: widget.intradayLoading,
          errorMessage: widget.intradayErrorMessage,
          idleMessage: widget.intradayIdleMessage,
          loadSession: widget.loadSession,
          onSessionChanged: _selectSession,
        ),
    ],
  );
}
