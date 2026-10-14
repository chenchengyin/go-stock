import 'dart:math' as math;

import 'package:fl_chart/fl_chart.dart';
import 'package:flutter/material.dart';
import 'package:trading_app/core/theme/app_colors.dart';
import 'package:trading_app/features/radar/data/red_t1_intraday_repository.dart';

class RedT1IntradayPreview extends StatefulWidget {
  const RedT1IntradayPreview({
    super.key,
    required this.series,
    this.loading = false,
    this.errorMessage,
    this.idleMessage,
    this.loadSession,
    this.onSessionChanged,
  });

  final RedT1IntradaySeries? series;
  final bool loading;
  final String? errorMessage;
  final String? idleMessage;
  final Future<RedT1IntradaySeries> Function(String date)? loadSession;
  final ValueChanged<String>? onSessionChanged;

  @override
  State<RedT1IntradayPreview> createState() => _RedT1IntradayPreviewState();
}

class _RedT1IntradayPreviewState extends State<RedT1IntradayPreview> {
  static const double _minX = -0.5;
  static const double _maxX = 240.5;

  int? _selectedPoint;
  RedT1IntradaySeries? _navigatedSeries;
  bool _loading = false;
  bool _loadFailed = false;
  String? _loadErrorMessage;
  int _requestVersion = 0;

  Future<void> _navigate(String date) async {
    final load = widget.loadSession;
    if (load == null || _loading) return;
    final version = ++_requestVersion;
    setState(() {
      _loading = true;
      _loadFailed = false;
      _loadErrorMessage = null;
    });
    try {
      final result = await load(date);
      if (!mounted || version != _requestVersion) return;
      setState(() {
        _navigatedSeries = result;
        _selectedPoint = null;
        _loading = false;
      });
      widget.onSessionChanged?.call(result.date);
    } catch (error) {
      if (!mounted || version != _requestVersion) return;
      setState(() {
        _loadFailed = true;
        _loadErrorMessage = redT1IntradayErrorMessage(error);
        _loading = false;
      });
    }
  }

  @override
  void didUpdateWidget(covariant RedT1IntradayPreview oldWidget) {
    super.didUpdateWidget(oldWidget);
    if (oldWidget.series?.code != widget.series?.code ||
        oldWidget.series?.date != widget.series?.date ||
        oldWidget.series != widget.series) {
      _selectedPoint = null;
      _navigatedSeries = null;
      _loadFailed = false;
      _loadErrorMessage = null;
      _loading = false;
      _requestVersion++;
    }
  }

  @override
  Widget build(BuildContext context) {
    final data = _navigatedSeries ?? widget.series;
    if (data == null) {
      return Column(
        crossAxisAlignment: CrossAxisAlignment.stretch,
        children: [
          if (widget.loading)
            const Column(
              children: [
                LinearProgressIndicator(minHeight: 2),
                Padding(
                  padding: EdgeInsets.symmetric(vertical: 12),
                  child: Text('正在加载分时数据…'),
                ),
              ],
            )
          else
            Padding(
              padding: const EdgeInsets.symmetric(vertical: 12),
              child: Text(
                widget.errorMessage != null
                    ? '分时加载失败：${widget.errorMessage}'
                    : widget.idleMessage ?? '分时数据暂不可用',
                style: TextStyle(
                  fontSize: 11,
                  color: widget.errorMessage != null
                      ? AppColors.warning
                      : AppColors.textSecondary,
                ),
              ),
            ),
        ],
      );
    }

    final navigationEnabled = widget.loadSession != null;
    final previous = data.previousDate;
    final next = data.nextDate;
    Widget navigator(
      String? date,
      IconData icon,
      String tooltip,
      double width,
    ) => SizedBox.square(
      dimension: width,
      child: IconButton(
        tooltip: tooltip,
        onPressed:
            !_loading && date != null && date.isNotEmpty && navigationEnabled
            ? () => _navigate(date)
            : null,
        icon: Icon(icon, size: 18),
        padding: EdgeInsets.zero,
        constraints: BoxConstraints.tightFor(width: width, height: width),
        style: IconButton.styleFrom(
          minimumSize: Size.zero,
          tapTargetSize: MaterialTapTargetSize.shrinkWrap,
          padding: EdgeInsets.zero,
        ),
      ),
    );
    Widget chartWithNavigation(Widget chart) => LayoutBuilder(
      builder: (context, constraints) => Row(
        children: [
          SizedBox(
            width: constraints.maxWidth / 16,
            child: navigator(
              previous,
              Icons.chevron_left,
              '前一交易日分时',
              constraints.maxWidth / 16,
            ),
          ),
          SizedBox(width: constraints.maxWidth * 7 / 8, child: chart),
          SizedBox(
            width: constraints.maxWidth / 16,
            child: navigator(
              next,
              Icons.chevron_right,
              '后一交易日分时',
              constraints.maxWidth / 16,
            ),
          ),
        ],
      ),
    );
    if (data.prevClose <= 0 || data.bars.length < 2) {
      final reason = data.message.isNotEmpty
          ? data.message
          : data.bars.isNotEmpty && data.prevClose <= 0
          ? '缺少上一交易日收盘价，无法计算涨跌幅'
          : data.bars.isNotEmpty
          ? '有效分钟线不足 2 个点，无法绘图'
          : '未取得该交易日的分钟线';
      return Column(
        children: [
          chartWithNavigation(
            SizedBox(
              height: 100,
              child: Center(
                child: Text(
                  '分时 · ${data.date.isEmpty ? 'T+1' : data.date}：$reason',
                  maxLines: 3,
                  overflow: TextOverflow.ellipsis,
                  textAlign: TextAlign.center,
                  style: TextStyle(
                    fontSize: 10,
                    color: AppColors.textSecondary,
                  ),
                ),
              ),
            ),
          ),
          if (_loading || widget.loading)
            const LinearProgressIndicator(minHeight: 2),
          if (widget.errorMessage != null)
            Text(
              '分时加载失败：${widget.errorMessage}',
              style: TextStyle(fontSize: 10, color: AppColors.warning),
            ),
          if (_loadFailed)
            Text(
              '分时切换失败：${_loadErrorMessage ?? '请重试'}',
              style: TextStyle(fontSize: 10, color: AppColors.warning),
            ),
        ],
      );
    }

    final points = <_IntradayPoint>[];
    for (var i = 0; i < data.bars.length; i++) {
      final x = _sessionX(data.bars[i].time);
      if (x != null) {
        points.add(_IntradayPoint(index: i, x: x, bar: data.bars[i]));
      }
    }
    if (points.length < 2) {
      return chartWithNavigation(
        SizedBox(
          height: 100,
          child: Center(
            child: Text(
              data.message.isNotEmpty
                  ? data.message
                  : '分时 · ${data.date} 有效分钟线不足 2 个点',
              textAlign: TextAlign.center,
              style: TextStyle(fontSize: 10, color: AppColors.textSecondary),
            ),
          ),
        ),
      );
    }

    _IntradayPoint? selectedPoint;
    if (_selectedPoint != null) {
      for (final point in points) {
        if (point.index == _selectedPoint) {
          selectedPoint = point;
          break;
        }
      }
    }
    final current = selectedPoint ?? points.last;
    final currentChange = (current.bar.price / data.prevClose - 1) * 100;
    final changeColor = currentChange >= 0
        ? AppColors.textPriceUp
        : AppColors.textPriceDown;
    final hasAverage = points.any((point) => point.bar.averagePrice > 0);
    final maxAbsChange = points
        .map((point) => ((point.bar.price / data.prevClose - 1) * 100).abs())
        .reduce(math.max);
    // Keep the percentage axis and grid on the same symmetric 2% steps.
    final yBound = math.max(2.0, (maxAbsChange * 1.1 / 2).ceil() * 2.0);

    return Column(
      crossAxisAlignment: CrossAxisAlignment.stretch,
      children: [
        const SizedBox(height: 4),
        Text(
          '${widget.series?.date == data.date ? 'T+1 ' : ''}分时 · ${data.date}',
          overflow: TextOverflow.ellipsis,
          style: TextStyle(fontSize: 11, color: AppColors.textSecondary),
        ),
        const SizedBox(height: 4),
        Row(
          children: [
            Text(
              _formatPercent(currentChange),
              style: TextStyle(fontSize: 10, color: changeColor),
            ),
            const SizedBox(width: 6),
            Text(
              current.bar.time,
              style: TextStyle(fontSize: 10, color: AppColors.textSecondary),
            ),
            const SizedBox(width: 8),
            _priceLegend(),
            if (hasAverage) ...[
              const SizedBox(width: 8),
              _legend(AppColors.tagOrange, '均价'),
            ],
          ],
        ),
        const SizedBox(height: 2),
        Padding(
          padding: const EdgeInsets.only(left: 22),
          child: Text(
            '涨跌幅',
            style: TextStyle(fontSize: 9, color: AppColors.textTertiary),
          ),
        ),
        chartWithNavigation(
          SizedBox(
            // Leave enough room for every 2% label even on a wide-limit stock.
            height: math.max(100, (yBound + 1) * 14),
            child: LineChart(
              LineChartData(
                minX: _minX,
                maxX: _maxX,
                minY: -yBound,
                maxY: yBound,
                gridData: FlGridData(
                  show: true,
                  horizontalInterval: 2,
                  verticalInterval: 30,
                  getDrawingHorizontalLine: (_) => FlLine(
                    color: AppColors.divider.withValues(alpha: 0.75),
                    strokeWidth: 0.55,
                    dashArray: const [3, 3],
                  ),
                  getDrawingVerticalLine: (_) => FlLine(
                    color: AppColors.divider.withValues(alpha: 0.5),
                    strokeWidth: 0.45,
                    dashArray: const [2, 4],
                  ),
                ),
                borderData: FlBorderData(show: false),
                titlesData: FlTitlesData(
                  topTitles: const AxisTitles(
                    sideTitles: SideTitles(showTitles: false),
                  ),
                  bottomTitles: AxisTitles(
                    sideTitles: SideTitles(
                      showTitles: true,
                      reservedSize: 40,
                      interval: 30,
                      getTitlesWidget: _timeTitle,
                    ),
                  ),
                  leftTitles: AxisTitles(
                    sideTitles: SideTitles(
                      showTitles: true,
                      reservedSize: 48,
                      interval: 2,
                      getTitlesWidget: (value, meta) => SideTitleWidget(
                        meta: meta,
                        space: 2,
                        child: Text(
                          _formatPercent(value),
                          style: TextStyle(
                            fontSize: 9,
                            color: value == 0
                                ? AppColors.textPrimary
                                : AppColors.textSecondary,
                            fontWeight: value == 0
                                ? FontWeight.w600
                                : FontWeight.normal,
                          ),
                        ),
                      ),
                    ),
                  ),
                  rightTitles: const AxisTitles(
                    sideTitles: SideTitles(showTitles: false),
                  ),
                ),
                extraLinesData: ExtraLinesData(
                  horizontalLines: [
                    HorizontalLine(
                      y: 0,
                      color: AppColors.textSecondary.withValues(alpha: 0.8),
                      strokeWidth: 1.2,
                    ),
                  ],
                  verticalLines: [
                    if (selectedPoint != null)
                      VerticalLine(
                        x: selectedPoint.x,
                        color: AppColors.textSecondary.withValues(alpha: 0.7),
                        strokeWidth: 0.8,
                        dashArray: const [3, 3],
                      ),
                  ],
                ),
                lineTouchData: LineTouchData(
                  enabled: true,
                  handleBuiltInTouches: false,
                  touchSpotThreshold: 18,
                  touchCallback: _handleChartTouch,
                ),
                lineBarsData: [
                  ..._priceLineBars(points, data.prevClose),
                  if (hasAverage)
                    LineChartBarData(
                      spots: [
                        for (final point in points)
                          if (point.bar.averagePrice > 0)
                            FlSpot(
                              point.x,
                              (point.bar.averagePrice / data.prevClose - 1) *
                                  100,
                            ),
                      ],
                      isCurved: false,
                      color: AppColors.tagOrange,
                      barWidth: 1.2,
                      dotData: const FlDotData(show: false),
                      belowBarData: BarAreaData(show: false),
                    ),
                ],
              ),
              duration: Duration.zero,
            ),
          ),
        ),
        if (_loading || widget.loading)
          const LinearProgressIndicator(minHeight: 2),
        if (widget.errorMessage != null)
          Text(
            '分时加载失败：${widget.errorMessage}',
            style: TextStyle(fontSize: 10, color: AppColors.warning),
          ),
        if (_loadFailed)
          Text(
            '分时切换失败：${_loadErrorMessage ?? '请重试'}',
            style: TextStyle(fontSize: 10, color: AppColors.warning),
          ),
      ],
    );
  }

  void _handleChartTouch(FlTouchEvent event, LineTouchResponse? response) {
    if (event is FlLongPressEnd || event is FlPanCancelEvent) {
      if (_selectedPoint != null) setState(() => _selectedPoint = null);
      return;
    }
    if (event is! FlLongPressStart && event is! FlLongPressMoveUpdate) return;
    final touched = response?.lineBarSpots;
    if (touched == null || touched.isEmpty) return;
    final touchedX = touched.first.x;
    final bars =
        (_navigatedSeries ?? widget.series)?.bars ?? const <RedT1IntradayBar>[];
    var bestIndex = -1;
    var bestDistance = double.infinity;
    for (var i = 0; i < bars.length; i++) {
      final x = _sessionX(bars[i].time);
      if (x == null) continue;
      final distance = (x - touchedX).abs();
      if (distance < bestDistance) {
        bestDistance = distance;
        bestIndex = i;
      }
    }
    if (bestIndex >= 0 && bestIndex != _selectedPoint) {
      setState(() => _selectedPoint = bestIndex);
    }
  }

  List<LineChartBarData> _priceLineBars(
    List<_IntradayPoint> points,
    double prevClose,
  ) {
    final upSpots = <FlSpot>[];
    final downSpots = <FlSpot>[];
    Color? activeColor;

    void addSegment(Color color, FlSpot start, FlSpot end) {
      final target = color == AppColors.textPriceUp ? upSpots : downSpots;
      if (activeColor != color) {
        if (target.isNotEmpty && !target.last.isNull()) {
          target.add(FlSpot.nullSpot);
        }
        target
          ..add(start)
          ..add(end);
        activeColor = color;
      } else {
        if (target.isEmpty || target.last != start) target.add(start);
        target.add(end);
      }
    }

    FlSpot priceSpot(_IntradayPoint point) =>
        FlSpot(point.x, (point.bar.price / prevClose - 1) * 100);

    for (var i = 1; i < points.length; i++) {
      final start = priceSpot(points[i - 1]);
      final end = priceSpot(points[i]);
      if (start.y >= 0 && end.y >= 0) {
        addSegment(AppColors.textPriceUp, start, end);
      } else if (start.y <= 0 && end.y <= 0) {
        addSegment(AppColors.textPriceDown, start, end);
      } else {
        final crossingX =
            start.x + (-start.y) * (end.x - start.x) / (end.y - start.y);
        final crossing = FlSpot(crossingX, 0);
        if (start.y > 0) {
          addSegment(AppColors.textPriceUp, start, crossing);
          addSegment(AppColors.textPriceDown, crossing, end);
        } else {
          addSegment(AppColors.textPriceDown, start, crossing);
          addSegment(AppColors.textPriceUp, crossing, end);
        }
      }
    }

    return [
      if (upSpots.length > 1)
        LineChartBarData(
          spots: upSpots,
          isCurved: false,
          color: AppColors.textPriceUp,
          barWidth: 1.5,
          dotData: const FlDotData(show: false),
          belowBarData: BarAreaData(show: false),
        ),
      if (downSpots.length > 1)
        LineChartBarData(
          spots: downSpots,
          isCurved: false,
          color: AppColors.textPriceDown,
          barWidth: 1.5,
          dotData: const FlDotData(show: false),
          belowBarData: BarAreaData(show: false),
        ),
    ];
  }

  Widget _priceLegend() => Row(
    mainAxisSize: MainAxisSize.min,
    children: [
      Container(width: 5, height: 2, color: AppColors.textPriceUp),
      Container(width: 5, height: 2, color: AppColors.textPriceDown),
      const SizedBox(width: 3),
      Text('分时', style: TextStyle(fontSize: 9, color: AppColors.textSecondary)),
    ],
  );

  Widget _legend(Color color, String label) => Row(
    mainAxisSize: MainAxisSize.min,
    children: [
      Container(width: 9, height: 2, color: color),
      const SizedBox(width: 3),
      Text(
        label,
        style: TextStyle(fontSize: 9, color: AppColors.textSecondary),
      ),
    ],
  );

  String _formatPercent(double value) {
    final sign = value > 0 ? '+' : '';
    return '$sign${value.toStringAsFixed(1)}%';
  }

  Widget _timeTitle(double value, TitleMeta meta) {
    // Stagger neighbouring half-hour labels on narrow cards. The two clocks
    // sharing the lunch boundary occupy the first and third rows.
    final index = value.round() ~/ 30;
    final label = switch (value.round()) {
      0 => '09:30',
      30 => '10:00',
      60 => '10:30',
      90 => '11:00',
      120 => null,
      150 => '13:30',
      180 => '14:00',
      210 => '14:30',
      240 => '15:00',
      _ => '',
    };
    if (label == '') return const SizedBox.shrink();
    final style = TextStyle(
      fontSize: 8,
      height: 1,
      color: AppColors.textSecondary,
    );
    return SideTitleWidget(
      meta: meta,
      space: 1,
      fitInside: SideTitleFitInsideData.fromTitleMeta(
        meta,
        distanceFromEdge: 0,
      ),
      child: SizedBox(
        width: 42,
        height: 34,
        child: label == null
            ? Column(
                mainAxisAlignment: MainAxisAlignment.spaceBetween,
                children: [
                  Text('11:30', style: style),
                  Text('13:00', style: style),
                ],
              )
            : Align(
                alignment: index.isEven
                    ? Alignment.topCenter
                    : Alignment.center,
                child: Text(label, style: style),
              ),
      ),
    );
  }

  double? _sessionX(String time) {
    final parts = time.split(':');
    if (parts.length != 2) return null;
    final hour = int.tryParse(parts[0]);
    final minute = int.tryParse(parts[1]);
    if (hour == null || minute == null || hour > 23 || minute > 59) return null;
    final minuteOfDay = hour * 60 + minute;
    final opening = 9 * 60 + 30;
    final morningClose = 11 * 60 + 30;
    final afternoonOpen = 13 * 60;
    final closing = 15 * 60;
    int sessionMinute;
    if (minuteOfDay >= opening && minuteOfDay <= morningClose) {
      sessionMinute = minuteOfDay - opening;
    } else if (minuteOfDay >= afternoonOpen && minuteOfDay <= closing) {
      sessionMinute = 121 + minuteOfDay - afternoonOpen;
    } else {
      return null;
    }
    return sessionMinute * _maxX / 241;
  }
}

class _IntradayPoint {
  const _IntradayPoint({
    required this.index,
    required this.x,
    required this.bar,
  });

  final int index;
  final double x;
  final RedT1IntradayBar bar;
}
