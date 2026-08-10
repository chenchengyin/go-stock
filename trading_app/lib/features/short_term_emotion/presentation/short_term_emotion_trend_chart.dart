import 'dart:math' as math;

import 'package:flutter/material.dart';

import '../../../core/theme/app_colors.dart';
import '../domain/short_term_emotion_models.dart';

class ShortTermEmotionTrendChart extends StatefulWidget {
  const ShortTermEmotionTrendChart({super.key, required this.items});

  final List<ShortTermEmotionTrendPoint> items;

  @override
  State<ShortTermEmotionTrendChart> createState() =>
      _ShortTermEmotionTrendChartState();
}

class _ShortTermEmotionTrendChartState
    extends State<ShortTermEmotionTrendChart> {
  int? _selectedIndex;

  void _updateSelectedPoint(
    Offset localPosition,
    Size size,
    List<ShortTermEmotionTrendPoint> displayItems,
  ) {
    if (displayItems.isEmpty) return;
    final plot = _chartPlotRect(size);
    final clampedX = localPosition.dx.clamp(plot.left, plot.right).toDouble();
    final index = const _TradingTimeAxis().nearestIndexForX(
      plot,
      displayItems,
      clampedX,
    );
    if (index < 0) return;
    setState(() {
      _selectedIndex = index;
    });
  }

  void _clearSelectedPoint() {
    if (_selectedIndex == null) return;
    setState(() {
      _selectedIndex = null;
    });
  }

  @override
  Widget build(BuildContext context) {
    final displayItems = _sampleTrendPointsForChart(widget.items);

    return Container(
      decoration: _cardDecoration(),
      padding: const EdgeInsets.fromLTRB(12, 12, 12, 10),
      child: Column(
        crossAxisAlignment: CrossAxisAlignment.start,
        children: [
          Row(
            children: [
              Text(
                '涨跌家数比',
                style: TextStyle(
                  fontSize: 15,
                  fontWeight: FontWeight.w700,
                  color: AppColors.textPrimary,
                ),
              ),
              const Spacer(),
              Text(
                '盘中趋势',
                style: TextStyle(fontSize: 11, color: AppColors.textTertiary),
              ),
            ],
          ),
          const SizedBox(height: 10),
          _Legend(),
          const SizedBox(height: 8),
          if (displayItems.length < 2)
            const _TrendEmpty()
          else
            SizedBox(
              height: 230,
              width: double.infinity,
              child: LayoutBuilder(
                builder: (context, constraints) {
                  final size = Size(constraints.maxWidth, 230);
                  return GestureDetector(
                    behavior: HitTestBehavior.opaque,
                    onLongPressStart: (details) => _updateSelectedPoint(
                      details.localPosition,
                      size,
                      displayItems,
                    ),
                    onLongPressMoveUpdate: (details) => _updateSelectedPoint(
                      details.localPosition,
                      size,
                      displayItems,
                    ),
                    onLongPressEnd: (_) => _clearSelectedPoint(),
                    onPanStart: (details) => _updateSelectedPoint(
                      details.localPosition,
                      size,
                      displayItems,
                    ),
                    onPanUpdate: (details) => _updateSelectedPoint(
                      details.localPosition,
                      size,
                      displayItems,
                    ),
                    onPanEnd: (_) => _clearSelectedPoint(),
                    onPanCancel: _clearSelectedPoint,
                    child: CustomPaint(
                      painter: _TrendChartPainter(
                        displayItems,
                        selectedIndex: _selectedIndex,
                      ),
                    ),
                  );
                },
              ),
            ),
        ],
      ),
    );
  }
}

class _Legend extends StatelessWidget {
  @override
  Widget build(BuildContext context) {
    return Wrap(
      spacing: 10,
      runSpacing: 6,
      children: const [
        _LegendItem(color: Color(0xFFFF4D4F), text: '上涨家数', isLine: false),
        _LegendItem(color: Color(0xFF23C466), text: '下跌家数', isLine: false),
        _LegendItem(color: Color(0xFFFF9800), text: '红盘率', isLine: true),
        _LegendItem(color: Color(0xFF8B5CF6), text: '情绪指标', isLine: true),
        _LegendItem(color: Color(0xFF2F80ED), text: '涨跌停比', isLine: true),
      ],
    );
  }
}

class _LegendItem extends StatelessWidget {
  const _LegendItem({
    required this.color,
    required this.text,
    required this.isLine,
  });

  final Color color;
  final String text;
  final bool isLine;

  @override
  Widget build(BuildContext context) {
    return Row(
      mainAxisSize: MainAxisSize.min,
      children: [
        Container(
          width: isLine ? 18 : 14,
          height: isLine ? 3 : 10,
          decoration: BoxDecoration(
            color: color,
            borderRadius: BorderRadius.circular(99),
          ),
        ),
        const SizedBox(width: 4),
        Text(
          text,
          style: TextStyle(fontSize: 11, color: AppColors.textSecondary),
        ),
      ],
    );
  }
}

class _TrendEmpty extends StatelessWidget {
  const _TrendEmpty();

  @override
  Widget build(BuildContext context) {
    return Container(
      height: 120,
      alignment: Alignment.center,
      decoration: BoxDecoration(
        color: AppColors.surfaceBg,
        borderRadius: BorderRadius.circular(8),
        border: Border.all(color: AppColors.border),
      ),
      child: Text(
        '暂无足够盘中趋势数据',
        style: TextStyle(fontSize: 12, color: AppColors.textTertiary),
      ),
    );
  }
}

class _TrendChartPainter extends CustomPainter {
  _TrendChartPainter(this.items, {this.selectedIndex});

  final List<ShortTermEmotionTrendPoint> items;
  final int? selectedIndex;
  final _axis = const _TradingTimeAxis();

  static const _upColor = Color(0xFFFF4D4F);
  static const _downColor = Color(0xFF23C466);
  static const _redRateColor = Color(0xFFFF9800);
  static const _emotionColor = Color(0xFF8B5CF6);
  static const _limitRatioColor = Color(0xFF2F80ED);

  @override
  void paint(Canvas canvas, Size size) {
    final plot = _chartPlotRect(size);
    if (plot.width <= 0 || plot.height <= 0) return;

    final maxCount = items.fold<int>(
      1,
      (maxValue, item) =>
          math.max(maxValue, math.max(item.upCount, item.downCount)),
    );
    final countMax = _niceMax(maxCount);

    _drawGrid(canvas, plot, countMax);
    _drawBars(canvas, plot, countMax);
    _drawLine(
      canvas,
      plot,
      _redRateColor,
      (item) => item.redRate.clamp(0, 100).toDouble(),
    );
    _drawLine(
      canvas,
      plot,
      _emotionColor,
      (item) => _scaleEmotionIndex(item.emotionIndex),
    );
    _drawLine(
      canvas,
      plot,
      _limitRatioColor,
      (item) => _scaleLimitRatio(item.limitRatio),
    );
    _drawXAxis(canvas, plot);
    _drawSelectedPoint(canvas, plot);
  }

  void _drawGrid(Canvas canvas, Rect plot, int countMax) {
    final gridPaint = Paint()
      ..color = AppColors.border
      ..strokeWidth = 1;
    final textStyle = TextStyle(fontSize: 10, color: AppColors.textTertiary);

    for (var i = 0; i <= 4; i++) {
      final y = plot.bottom - plot.height * i / 4;
      canvas.drawLine(Offset(plot.left, y), Offset(plot.right, y), gridPaint);
      _drawText(canvas, '${countMax * i ~/ 4}', Offset(0, y - 7), textStyle);
      _drawText(canvas, '${25 * i}%', Offset(plot.right + 8, y - 7), textStyle);
    }

    final midPaint = Paint()
      ..color = AppColors.textTertiary
      ..strokeWidth = 1;
    _drawDashedLine(
      canvas,
      Offset(plot.left, plot.bottom - plot.height * 0.5),
      Offset(plot.right, plot.bottom - plot.height * 0.5),
      midPaint,
    );
  }

  void _drawBars(Canvas canvas, Rect plot, int countMax) {
    final slotWidth =
        plot.width /
        (_TradingTimeAxis.endMinute - _TradingTimeAxis.startMinute);
    final barWidth = math.min(4.2, math.max(2.6, slotWidth * 1.8 + 1));
    final upPaint = Paint()..color = _upColor.withValues(alpha: 0.92);
    final downPaint = Paint()..color = _downColor.withValues(alpha: 0.92);

    for (final item in items) {
      final centerX = _axis.xForTime(plot, item.time);
      final upHeight = plot.height * item.upCount / countMax;
      final downHeight = plot.height * item.downCount / countMax;
      canvas.drawRRect(
        RRect.fromRectAndRadius(
          Rect.fromLTWH(
            centerX - barWidth - 1,
            plot.bottom - upHeight,
            barWidth,
            upHeight,
          ),
          const Radius.circular(2),
        ),
        upPaint,
      );
      canvas.drawRRect(
        RRect.fromRectAndRadius(
          Rect.fromLTWH(
            centerX + 1,
            plot.bottom - downHeight,
            barWidth,
            downHeight,
          ),
          const Radius.circular(2),
        ),
        downPaint,
      );
    }
  }

  void _drawLine(
    Canvas canvas,
    Rect plot,
    Color color,
    double Function(ShortTermEmotionTrendPoint item) valueOf,
  ) {
    final linePaint = Paint()
      ..color = color
      ..strokeWidth = 1.8
      ..style = PaintingStyle.stroke
      ..strokeCap = StrokeCap.round
      ..strokeJoin = StrokeJoin.round;
    final dotPaint = Paint()
      ..color = Colors.white
      ..style = PaintingStyle.fill;
    final dotStroke = Paint()
      ..color = color
      ..style = PaintingStyle.stroke
      ..strokeWidth = 2;

    final path = Path();
    for (var i = 0; i < items.length; i++) {
      final item = items[i];
      final point = _linePoint(plot, item, valueOf(item));
      if (i == 0) {
        path.moveTo(point.dx, point.dy);
      } else {
        path.lineTo(point.dx, point.dy);
      }
    }
    canvas.drawPath(path, linePaint);

    for (final item in items) {
      final point = _linePoint(plot, item, valueOf(item));
      canvas.drawCircle(point, 2.4, dotPaint);
      canvas.drawCircle(point, 2.4, dotStroke);
    }
  }

  Offset _linePoint(Rect plot, ShortTermEmotionTrendPoint item, double value) {
    final x = _axis.xForTime(plot, item.time);
    final y = plot.bottom - plot.height * value.clamp(0, 100) / 100;
    return Offset(x, y);
  }

  void _drawXAxis(Canvas canvas, Rect plot) {
    final textStyle = TextStyle(fontSize: 10, color: AppColors.textTertiary);
    for (final label in _TradingTimeAxis.labels) {
      final x = _axis.xForLabel(plot, label);
      _drawRotatedText(
        canvas,
        label,
        Offset(x - 12, plot.bottom + 12),
        textStyle,
      );
    }
  }

  void _drawSelectedPoint(Canvas canvas, Rect plot) {
    final index = selectedIndex;
    if (index == null || index < 0 || index >= items.length) return;

    final item = items[index];
    final x = _axis.xForTime(plot, item.time);
    final linePaint = Paint()
      ..color = AppColors.textPrimary.withValues(alpha: 0.55)
      ..strokeWidth = 1;
    _drawDashedLine(
      canvas,
      Offset(x, plot.top),
      Offset(x, plot.bottom),
      linePaint,
    );

    _drawSelectedDot(canvas, plot, item, item.redRate, _redRateColor);
    _drawSelectedDot(
      canvas,
      plot,
      item,
      _scaleEmotionIndex(item.emotionIndex),
      _emotionColor,
    );
    _drawSelectedDot(
      canvas,
      plot,
      item,
      _scaleLimitRatio(item.limitRatio),
      _limitRatioColor,
    );
    _drawTooltip(canvas, plot, item, x);
  }

  void _drawSelectedDot(
    Canvas canvas,
    Rect plot,
    ShortTermEmotionTrendPoint item,
    double value,
    Color color,
  ) {
    final point = _linePoint(plot, item, value);
    canvas.drawCircle(point, 4, Paint()..color = Colors.white);
    canvas.drawCircle(
      point,
      4,
      Paint()
        ..color = color
        ..style = PaintingStyle.stroke
        ..strokeWidth = 2.4,
    );
  }

  void _drawTooltip(
    Canvas canvas,
    Rect plot,
    ShortTermEmotionTrendPoint item,
    double anchorX,
  ) {
    const tooltipWidth = 154.0;
    const tooltipHeight = 140.0;
    final left = anchorX + tooltipWidth + 10 > plot.right
        ? anchorX - tooltipWidth - 10
        : anchorX + 10;
    final top = plot.top + 8;
    final rect = Rect.fromLTWH(
      left.clamp(plot.left, plot.right - tooltipWidth).toDouble(),
      top,
      tooltipWidth,
      tooltipHeight,
    );

    final shadowPaint = Paint()
      ..color = Colors.black.withValues(alpha: 0.12)
      ..maskFilter = const MaskFilter.blur(BlurStyle.normal, 8);
    canvas.drawRRect(
      RRect.fromRectAndRadius(
        rect.shift(const Offset(0, 2)),
        const Radius.circular(8),
      ),
      shadowPaint,
    );
    canvas.drawRRect(
      RRect.fromRectAndRadius(rect, const Radius.circular(8)),
      Paint()..color = Colors.white,
    );
    canvas.drawRRect(
      RRect.fromRectAndRadius(rect, const Radius.circular(8)),
      Paint()
        ..color = AppColors.border
        ..style = PaintingStyle.stroke,
    );

    final titleStyle = TextStyle(
      fontSize: 12,
      fontWeight: FontWeight.w700,
      color: AppColors.textPrimary,
    );
    final rowStyle = TextStyle(fontSize: 11, color: AppColors.textSecondary);
    var y = rect.top + 10;
    _drawText(canvas, item.time, Offset(rect.left + 10, y), titleStyle);
    y += 21;
    _drawTooltipRow(
      canvas,
      rect.left + 10,
      y,
      '上涨家数',
      '${item.upCount}',
      _upColor,
      rowStyle,
    );
    y += 18;
    _drawTooltipRow(
      canvas,
      rect.left + 10,
      y,
      '下跌家数',
      '${item.downCount}',
      _downColor,
      rowStyle,
    );
    y += 18;
    _drawTooltipRow(
      canvas,
      rect.left + 10,
      y,
      '红盘率',
      '${item.redRate.toStringAsFixed(1)}%',
      _redRateColor,
      rowStyle,
    );
    y += 18;
    _drawTooltipRow(
      canvas,
      rect.left + 10,
      y,
      '情绪指标',
      item.emotionIndex.toStringAsFixed(2),
      _emotionColor,
      rowStyle,
    );
    y += 18;
    _drawTooltipRow(
      canvas,
      rect.left + 10,
      y,
      '涨跌停比',
      item.limitRatio.toStringAsFixed(2),
      _limitRatioColor,
      rowStyle,
      subValue: '(${item.limitUp}:${item.limitDown})',
    );
  }

  void _drawTooltipRow(
    Canvas canvas,
    double x,
    double y,
    String label,
    String value,
    Color color,
    TextStyle style, {
    String? subValue,
  }) {
    canvas.drawCircle(Offset(x + 3, y + 7), 3, Paint()..color = color);
    _drawText(canvas, label, Offset(x + 12, y), style);
    final valuePainter = TextPainter(
      text: TextSpan(
        text: value,
        style: style.copyWith(
          fontWeight: FontWeight.w700,
          color: AppColors.textPrimary,
        ),
      ),
      textDirection: TextDirection.ltr,
    )..layout();
    valuePainter.paint(canvas, Offset(x + 122 - valuePainter.width, y));
    if (subValue != null) {
      final subPainter = TextPainter(
        text: TextSpan(
          text: subValue,
          style: style.copyWith(fontSize: 10, color: AppColors.textTertiary),
        ),
        textDirection: TextDirection.ltr,
      )..layout();
      subPainter.paint(canvas, Offset(x + 122 - subPainter.width, y + 13));
    }
  }

  int _niceMax(int rawMax) {
    if (rawMax <= 3000) return 3000;
    if (rawMax <= 6000) return 6000;
    if (rawMax <= 9000) return 9000;
    return ((rawMax / 3000).ceil()) * 3000;
  }

  double _scaleEmotionIndex(double value) {
    if (value <= 0) return 0;
    if (value >= 3) return 95;
    return (value / 3 * 95).clamp(0, 95);
  }

  double _scaleLimitRatio(double value) {
    if (value <= 0) return 0;
    if (value >= 10) return 95;
    return (value / 10 * 95).clamp(0, 95);
  }

  void _drawText(Canvas canvas, String text, Offset offset, TextStyle style) {
    final painter = TextPainter(
      text: TextSpan(text: text, style: style),
      textDirection: TextDirection.ltr,
    )..layout();
    painter.paint(canvas, offset);
  }

  void _drawRotatedText(
    Canvas canvas,
    String text,
    Offset offset,
    TextStyle style,
  ) {
    canvas.save();
    canvas.translate(offset.dx, offset.dy);
    canvas.rotate(-math.pi / 4);
    _drawText(canvas, text, Offset.zero, style);
    canvas.restore();
  }

  void _drawDashedLine(Canvas canvas, Offset start, Offset end, Paint paint) {
    const dashWidth = 5.0;
    const dashSpace = 4.0;
    var distance = 0.0;
    final total = (end - start).distance;
    final direction = (end - start) / total;
    while (distance < total) {
      final from = start + direction * distance;
      final to = start + direction * math.min(distance + dashWidth, total);
      canvas.drawLine(from, to, paint);
      distance += dashWidth + dashSpace;
    }
  }

  @override
  bool shouldRepaint(covariant _TrendChartPainter oldDelegate) {
    return oldDelegate.items != items ||
        oldDelegate.selectedIndex != selectedIndex;
  }
}

Rect _chartPlotRect(Size size) {
  return Rect.fromLTWH(34, 8, size.width - 78, size.height - 44);
}

@visibleForTesting
class TradingTimeAxisDebug extends _TradingTimeAxis {
  const TradingTimeAxisDebug();
}

@visibleForTesting
List<ShortTermEmotionTrendPoint> sampleTrendPointsForChartDebug(
  List<ShortTermEmotionTrendPoint> items,
) {
  return _sampleTrendPointsForChart(items);
}

List<ShortTermEmotionTrendPoint> _sampleTrendPointsForChart(
  List<ShortTermEmotionTrendPoint> items,
) {
  if (items.length <= 2) return items;

  final sampled = <ShortTermEmotionTrendPoint>[];
  var lastOffset = -1;

  for (final item in items) {
    final offset = _TradingTimeAxis.tradingMinuteOffsetForTime(item.time);
    if (sampled.isEmpty || offset - lastOffset >= 2) {
      sampled.add(item);
      lastOffset = offset;
    }
  }

  if (sampled.last.time != items.last.time) {
    final last = items.last;
    final lastItemOffset = _TradingTimeAxis.tradingMinuteOffsetForTime(
      last.time,
    );
    final previousOffset = sampled.length >= 2
        ? _TradingTimeAxis.tradingMinuteOffsetForTime(
            sampled[sampled.length - 2].time,
          )
        : -1;
    if (sampled.isNotEmpty && lastItemOffset - previousOffset >= 2) {
      sampled[sampled.length - 1] = last;
    } else {
      sampled.add(last);
    }
  }

  return sampled;
}

class _TradingTimeAxis {
  const _TradingTimeAxis();

  static const startMinute = 9 * 60 + 25;
  static const morningEndMinute = 11 * 60 + 30;
  static const afternoonStartMinute = 13 * 60;
  static const endMinute = 15 * 60;
  static const tradingMinuteCount =
      (morningEndMinute - startMinute) + (endMinute - afternoonStartMinute);
  static const labels = ['09:25', '10:30', '11:30/13:00', '14:00', '15:00'];

  double ratioForTime(String time) {
    return (tradingMinuteOffsetForTime(time) / tradingMinuteCount).clamp(0, 1);
  }

  double xForTime(Rect plot, String time) {
    return plot.left + plot.width * ratioForTime(time);
  }

  double xForLabel(Rect plot, String label) {
    final time = label.contains('/') ? label.split('/').first : label;
    return xForTime(plot, time);
  }

  int nearestIndexForX(
    Rect plot,
    List<ShortTermEmotionTrendPoint> items,
    double x,
  ) {
    if (items.isEmpty) return -1;
    var nearestIndex = 0;
    var nearestDistance = double.infinity;
    for (var i = 0; i < items.length; i++) {
      final pointX = xForTime(plot, items[i].time);
      final distance = (pointX - x).abs();
      if (distance < nearestDistance) {
        nearestDistance = distance;
        nearestIndex = i;
      }
    }
    return nearestIndex;
  }

  static int minuteOfDay(String time) {
    final parts = time.split(':');
    if (parts.length < 2) return startMinute;
    final hour = int.tryParse(parts[0]) ?? 9;
    final minute = int.tryParse(parts[1]) ?? 25;
    return hour * 60 + minute;
  }

  static int tradingMinuteOffsetForTime(String time) {
    return _tradingMinuteOffset(minuteOfDay(time));
  }

  static int _tradingMinuteOffset(int minute) {
    if (minute <= startMinute) return 0;
    if (minute <= morningEndMinute) return minute - startMinute;
    if (minute < afternoonStartMinute) return morningEndMinute - startMinute;
    if (minute <= endMinute) {
      return (morningEndMinute - startMinute) + (minute - afternoonStartMinute);
    }
    return tradingMinuteCount;
  }
}

BoxDecoration _cardDecoration() {
  return BoxDecoration(
    color: AppColors.cardBg,
    borderRadius: BorderRadius.circular(12),
    border: Border.all(color: AppColors.border),
    boxShadow: [
      BoxShadow(
        color: Colors.black.withValues(alpha: 0.03),
        blurRadius: 8,
        offset: const Offset(0, 2),
      ),
    ],
  );
}
