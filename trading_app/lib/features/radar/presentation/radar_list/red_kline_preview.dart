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
    required super.candlestickStyleProvider,
  });

  final List<RedKlineBar> bars;
  final Paint _maPaint = Paint()..style = PaintingStyle.stroke;

  @override
  void paint(
    Canvas canvas,
    ValueInCanvasProvider xInCanvasProvider,
    ValueInCanvasProvider yInCanvasProvider,
    CandlestickSpot spot,
    int spotIndex,
  ) {
    super.paint(canvas, xInCanvasProvider, yInCanvasProvider, spot, spotIndex);
    if (spotIndex <= 0 || spotIndex >= bars.length) return;
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

  @override
  FlCandlestickPainter lerp(
    FlCandlestickPainter a,
    FlCandlestickPainter b,
    double t,
  ) => b;

  @override
  List<Object?> get props => [...super.props, bars];
}

/// A compact, non-interactive 60 + anchor + up-to-5 daily candle overview.
class RedKlinePreview extends StatelessWidget {
  const RedKlinePreview({
    super.key,
    this.window,
    this.loading = false,
    this.error = false,
  });

  final RedKlineWindow? window;
  final bool loading;
  final bool error;

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
        Row(
          children: [
            Text(
              '日K · 前${data.beforeCount}根 / 后${data.afterCount}根',
              style: TextStyle(fontSize: 11, color: AppColors.textSecondary),
            ),
            const Spacer(),
            IconButton(
              tooltip: '放大K线',
              onPressed: () => _showExpanded(context, data),
              icon: const Icon(Icons.open_in_full, size: 17),
              constraints: const BoxConstraints(minWidth: 30, minHeight: 30),
              padding: EdgeInsets.zero,
            ),
          ],
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
          child: _RedCandleChart(window: data, expanded: false),
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

  void _showExpanded(BuildContext context, RedKlineWindow data) {
    showDialog<void>(
      context: context,
      builder: (dialogContext) => Dialog(
        child: ConstrainedBox(
          constraints: const BoxConstraints(maxWidth: 980, maxHeight: 490),
          child: Padding(
            padding: const EdgeInsets.all(16),
            child: Column(
              mainAxisSize: MainAxisSize.min,
              children: [
                Row(
                  children: [
                    Expanded(
                      child: Text(
                        '${data.code} · ${data.date} 前${data.beforeCount}根 / 后${data.afterCount}根',
                      ),
                    ),
                    IconButton(
                      tooltip: '关闭K线',
                      icon: const Icon(Icons.close),
                      onPressed: () => Navigator.pop(dialogContext),
                    ),
                  ],
                ),
                SizedBox(
                  height: 365,
                  child: _RedCandleChart(window: data, expanded: true),
                ),
                const Text(
                  '左右拖动、双指缩放；选中日右侧仅为事后走势',
                  style: TextStyle(fontSize: 11),
                ),
              ],
            ),
          ),
        ),
      ),
    );
  }
}

class _RedCandleChart extends StatelessWidget {
  const _RedCandleChart({required this.window, required this.expanded});
  final RedKlineWindow window;
  final bool expanded;

  @override
  Widget build(BuildContext context) {
    final bars = window.bars;
    if (bars.isEmpty) return const SizedBox.shrink();
    final minPrice = bars.map((b) => b.low).reduce(math.min);
    final maxPrice = bars.map((b) => b.high).reduce(math.max);
    final padding = math.max((maxPrice - minPrice) * 0.08, 0.01);
    final anchor = window.anchorIndex;
    final rise = AppColors.textPriceUp;
    final fall = AppColors.textPriceDown;

    return CandlestickChart(
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
        candlestickPainter: RedMovingAveragePainter(
          bars: bars,
          candlestickStyleProvider: (spot, index) {
            final color = spot.close >= spot.open ? rise : fall;
            return CandlestickStyle(
              lineColor: color,
              lineWidth: bars.length < 20 ? 1.5 : 1,
              bodyFillColor: color,
              bodyStrokeColor: color,
              bodyStrokeWidth: 0,
              bodyWidth: bars.length < 20
                  ? (expanded ? 11 : 9)
                  : (expanded ? 7 : 4.8),
              bodyRadius: 0,
            );
          },
        ),
        candlestickTouchData: CandlestickTouchData(
          enabled: expanded,
          touchTooltipData: CandlestickTouchTooltipData(
            fitInsideHorizontally: true,
            fitInsideVertically: true,
            maxContentWidth: 190,
            getTooltipItems: (_, spot, index) => CandlestickTooltipItem(
              '${bars[index].date}\n开 ${spot.open.toStringAsFixed(2)}  高 ${spot.high.toStringAsFixed(2)}\n低 ${spot.low.toStringAsFixed(2)}  收 ${spot.close.toStringAsFixed(2)}${bars[index].ma.isEmpty ? '' : '\n${[for (final period in _maColors.keys)
                      if (bars[index].ma[period] != null) 'MA$period ${bars[index].ma[period]!.toStringAsFixed(2)}'].join('  ')}'}',
              textStyle: const TextStyle(color: Colors.white, fontSize: 11),
            ),
          ),
        ),
      ),
      duration: Duration.zero,
      transformationConfig: expanded
          ? const FlTransformationConfig(
              scaleAxis: FlScaleAxis.horizontal,
              maxScale: 4,
            )
          : const FlTransformationConfig(),
    );
  }
}
