import 'dart:math' as math;

import 'package:fl_chart/fl_chart.dart';
import 'package:flutter/material.dart';
import 'package:trading_app/core/theme/app_colors.dart';
import 'package:trading_app/features/radar/data/red_kline_repository.dart';

const _maColors = <int, Color>{
  5: Color(0xFFD79B19),
  10: Color(0xFF219E96),
  20: Color(0xFF8469C4),
  60: Color(0xFF397AC4),
};

/// Draw each moving-average segment in the candle chart's own coordinates.
/// Using fl_chart's coordinate providers keeps the lines aligned when the
/// expanded chart is panned or scaled.
class RedMovingAveragePainter extends DefaultCandlestickPainter {
  RedMovingAveragePainter({
    required this.bars,
    required this.bodyWidth,
    required this.riseColor,
    required this.fallColor,
    required super.candlestickStyleProvider,
  });

  final List<RedKlineBar> bars;
  final double bodyWidth;
  final Color riseColor;
  final Color fallColor;
  final Paint _maPaint = Paint()..style = PaintingStyle.stroke;
  final Paint _dojiBodyPaint = Paint()
    ..style = PaintingStyle.stroke
    ..strokeCap = StrokeCap.butt;

  @override
  void paint(
    Canvas canvas,
    ValueInCanvasProvider xInCanvasProvider,
    ValueInCanvasProvider yInCanvasProvider,
    CandlestickSpot spot,
    int spotIndex,
  ) {
    super.paint(canvas, xInCanvasProvider, yInCanvasProvider, spot, spotIndex);
    if (spotIndex >= bars.length) return;
    if (spotIndex > 0) {
      for (final entry in _maColors.entries) {
        final previous = bars[spotIndex - 1].ma[entry.key];
        final current = bars[spotIndex].ma[entry.key];
        if (previous == null || current == null) continue;
        canvas.drawLine(
          Offset(
            xInCanvasProvider((spotIndex - 1).toDouble()),
            yInCanvasProvider(previous),
          ),
          Offset(xInCanvasProvider(spot.x), yInCanvasProvider(current)),
          _maPaint
            ..color = entry.value
            ..strokeWidth = entry.key == 60 ? 1.5 : 1.2,
        );
      }
    }

    final bar = bars[spotIndex];
    if (bar.open == bar.close) {
      final color = spotIndex == 0 || bar.close >= bars[spotIndex - 1].close
          ? riseColor
          : fallColor;
      final x = xInCanvasProvider(spot.x);
      final y = yInCanvasProvider(bar.close);
      canvas.drawLine(
        Offset(x - bodyWidth / 2, y),
        Offset(x + bodyWidth / 2, y),
        _dojiBodyPaint
          ..color = color
          ..strokeWidth = 1.5,
      );
    }
  }

  @override
  FlCandlestickPainter lerp(
    FlCandlestickPainter a,
    FlCandlestickPainter b,
    double t,
  ) => b;

  @override
  List<Object?> get props => [
    ...super.props,
    bars,
    bodyWidth,
    riseColor,
    fallColor,
  ];
}

/// A compact 60 + anchor + up-to-5 daily candle overview.
class RedKlinePreview extends StatelessWidget {
  const RedKlinePreview({
    super.key,
    this.window,
    this.loading = false,
    this.error = false,
    this.selectedDate,
  });

  final RedKlineWindow? window;
  final bool loading;
  final bool error;

  /// Date selected by the neighbouring intraday chart; long press is temporary.
  final String? selectedDate;

  @override
  Widget build(BuildContext context) {
    AppColors.of(context);
    final data = window;
    if (loading) {
      return const SizedBox(
        height: 136,
        child: Center(
          child: SizedBox(
            width: 18,
            height: 18,
            child: CircularProgressIndicator(strokeWidth: 2),
          ),
        ),
      );
    }
    if (data == null || data.bars.isEmpty) {
      return SizedBox(
        height: 48,
        child: Center(
          child: Text(
            error ? '日K加载失败' : '暂无可用日K线',
            style: TextStyle(fontSize: 12, color: AppColors.textTertiary),
          ),
        ),
      );
    }

    return Column(
      crossAxisAlignment: CrossAxisAlignment.stretch,
      children: [
        const SizedBox(height: 8),
        Text(
          '日K · 前${data.beforeCount}根 / 后${data.afterCount}根',
          style: TextStyle(fontSize: 11, color: AppColors.textSecondary),
        ),
        if (data.bars.any((bar) => bar.ma.isNotEmpty))
          Wrap(
            spacing: 10,
            children: [
              for (final entry in _maColors.entries)
                if (data.bars.any((bar) => bar.ma.containsKey(entry.key)))
                  Text(
                    'MA${entry.key}',
                    style: TextStyle(fontSize: 10, color: entry.value),
                  ),
            ],
          ),
        SizedBox(
          height: 112,
          child: _RedCandleChart(window: data, selectedDate: selectedDate),
        ),
        if (selectedDate != null &&
            !data.bars.any((bar) => bar.date == selectedDate))
          Text(
            '$selectedDate 日K线暂无可用数据',
            style: TextStyle(fontSize: 11, color: AppColors.warning),
          ),
        if (data.anchorIndex < 0)
          Text(
            '所选日K线尚未形成或不可用',
            style: TextStyle(fontSize: 11, color: AppColors.warning),
          )
        else
          Text(
            '${data.date} 为选中日 · 右侧为事后走势',
            style: TextStyle(fontSize: 11, color: AppColors.textTertiary),
          ),
      ],
    );
  }
}

class _RedCandleChart extends StatefulWidget {
  const _RedCandleChart({required this.window, this.selectedDate});
  final RedKlineWindow window;
  final String? selectedDate;

  @override
  State<_RedCandleChart> createState() => _RedCandleChartState();
}

class _RedCandleChartState extends State<_RedCandleChart> {
  int? _selectedIndex;

  @override
  void didUpdateWidget(covariant _RedCandleChart oldWidget) {
    super.didUpdateWidget(oldWidget);
    if (oldWidget.window != widget.window) _selectedIndex = null;
  }

  void _handleTouch(FlTouchEvent event, CandlestickTouchResponse? response) {
    if (event is FlLongPressEnd || event is FlPanCancelEvent) {
      if (_selectedIndex != null) setState(() => _selectedIndex = null);
      return;
    }
    if (event is! FlLongPressStart && event is! FlLongPressMoveUpdate) return;
    final bars = widget.window.bars;
    final coordinate = response?.touchChartCoordinate.dx;
    final index =
        response?.touchedSpot?.spotIndex ??
        (coordinate != null && coordinate.isFinite
            ? coordinate.round().clamp(0, bars.length - 1)
            : null);
    if (index != null && index != _selectedIndex) {
      setState(() => _selectedIndex = index);
    }
  }

  String _change(double value, double? previousClose) {
    if (previousClose == null || previousClose <= 0) return '--%';
    final change = (value / previousClose - 1) * 100;
    return '${change > 0 ? '+' : ''}${change.toStringAsFixed(2)}%';
  }

  Widget _selectionDetails(List<RedKlineBar> bars, int index) {
    final bar = bars[index];
    final previousClose = index > 0 ? bars[index - 1].close : null;
    final style = TextStyle(
      fontSize: 10,
      color: AppColors.textPrimary,
      height: 1.2,
    );
    return Padding(
      padding: const EdgeInsets.symmetric(horizontal: 5, vertical: 3),
      child: Column(
        crossAxisAlignment: CrossAxisAlignment.start,
        mainAxisSize: MainAxisSize.min,
        children: [
          Text(
            '${bar.date}  开 ${bar.open.toStringAsFixed(2)} (${_change(bar.open, previousClose)})',
            style: style,
          ),
          Text(
            '收 ${bar.close.toStringAsFixed(2)} (${_change(bar.close, previousClose)})  高 ${bar.high.toStringAsFixed(2)} (${_change(bar.high, previousClose)})',
            style: style,
          ),
          Text(
            '低 ${bar.low.toStringAsFixed(2)} (${_change(bar.low, previousClose)})',
            style: style,
          ),
        ],
      ),
    );
  }

  @override
  Widget build(BuildContext context) {
    final bars = widget.window.bars;
    if (bars.isEmpty) return const SizedBox.shrink();
    final minPrice = bars.map((b) => b.low).reduce(math.min);
    final maxPrice = bars.map((b) => b.high).reduce(math.max);
    final padding = math.max((maxPrice - minPrice) * 0.08, 0.01);
    final anchor = widget.window.anchorIndex;
    final rise = AppColors.textPriceUp;
    final fall = AppColors.textPriceDown;
    final bodyWidth = bars.length < 20 ? 9.0 : 4.8;

    // Do not pick a nearby candle when a trading day is absent in this window.
    final externalIndex = widget.selectedDate == null
        ? -1
        : bars.indexWhere((bar) => bar.date == widget.selectedDate);
    final selected = _selectedIndex != null && _selectedIndex! < bars.length
        ? _selectedIndex
        : externalIndex >= 0
        ? externalIndex
        : null;
    return Stack(
      children: [
        Positioned.fill(
          child: CandlestickChart(
            CandlestickChartData(
              candlestickSpots: [
                for (var i = 0; i < bars.length; i++)
                  CandlestickSpot(
                    x: i.toDouble(),
                    open: bars[i].open,
                    high: bars[i].high,
                    low: bars[i].low,
                    close: bars[i].close,
                  ),
              ],
              minX: -0.5,
              maxX: bars.length - 0.5,
              minY: minPrice - padding,
              maxY: maxPrice + padding,
              gridData: const FlGridData(show: false),
              borderData: FlBorderData(show: false),
              titlesData: const FlTitlesData(show: false),
              rangeAnnotations: RangeAnnotations(
                verticalRangeAnnotations: anchor < 0
                    ? []
                    : [
                        VerticalRangeAnnotation(
                          x1: anchor - 0.48,
                          x2: anchor + 0.48,
                          color: AppColors.tagOrange.withValues(alpha: 0.24),
                        ),
                      ],
              ),
              touchedPointIndicator: selected == null
                  ? null
                  : AxisSpotIndicator(
                      x: selected.toDouble(),
                      y: bars[selected].close,
                      painter: AxisLinesIndicatorPainter(
                        verticalLineProvider: (_) => VerticalLine(
                          x: selected.toDouble(),
                          color: AppColors.textSecondary,
                          strokeWidth: 0.8,
                          dashArray: const [3, 3],
                        ),
                        horizontalLineProvider: (_) => HorizontalLine(
                          y: bars[selected].close,
                          color: AppColors.textSecondary,
                          strokeWidth: 0.8,
                          dashArray: const [3, 3],
                        ),
                      ),
                    ),
              candlestickPainter: RedMovingAveragePainter(
                bars: bars,
                bodyWidth: bodyWidth,
                riseColor: rise,
                fallColor: fall,
                candlestickStyleProvider: (spot, index) {
                  final color = spot.close >= spot.open ? rise : fall;
                  return CandlestickStyle(
                    lineColor: color,
                    lineWidth: bars.length < 20 ? 1.5 : 1,
                    bodyFillColor: color,
                    bodyStrokeColor: color,
                    bodyStrokeWidth: 0,
                    bodyWidth: bodyWidth,
                    bodyRadius: 0,
                  );
                },
              ),
              candlestickTouchData: CandlestickTouchData(
                enabled: true,
                handleBuiltInTouches: false,
                touchSpotThreshold: 24,
                touchCallback: _handleTouch,
              ),
            ),
            duration: Duration.zero,
            transformationConfig: const FlTransformationConfig(),
          ),
        ),
        if (selected != null)
          Positioned(
            top: 2,
            left: 2,
            right: 2,
            child: IgnorePointer(
              child: Align(
                alignment: Alignment.topLeft,
                child: _selectionDetails(bars, selected),
              ),
            ),
          ),
      ],
    );
  }
}
