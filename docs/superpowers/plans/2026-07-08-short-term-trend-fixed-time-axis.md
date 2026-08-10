# Short-Term Trend Fixed Time Axis Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Make the Flutter App "涨跌家数比" chart use a fixed trading-day x-axis from `09:25` to `15:00`, so current intraday data only fills the proportional left portion of the chart and gradually extends rightward through the day.

**Architecture:** Extract trading-time x-axis math into a small pure helper next to the chart, then update the chart painter and touch selection to use timestamps instead of item indexes. The chart will keep all existing visual series and tooltip behavior, but x positions, x labels, and nearest-point selection will be based on fixed market time.

**Tech Stack:** Flutter/Dart, `CustomPainter`, `flutter_test`, existing `ShortTermEmotionTrendPoint` model.

## Global Constraints

- Do not change backend data shape for this task.
- Do not add charting dependencies; keep the existing `CustomPainter`.
- Fixed x-axis visible range is `09:25` to `15:00`.
- A data point before `09:25` clamps to the left edge; a data point after `15:00` clamps to the right edge.
- Keep existing long-press/pan tooltip interaction.
- Keep current App visual style and avoid unrelated UI changes.
- Use TDD: failing helper tests first, then implementation, then chart integration.

---

## File Structure

- Modify: `trading_app/lib/features/short_term_emotion/presentation/short_term_emotion_trend_chart.dart`
  - Add a private `_TradingTimeAxis` helper near the painter.
  - Replace index-based x calculations with time-based x calculations.
  - Replace index-ratio gesture selection with nearest x-position selection.
  - Replace dynamic x-axis labels with fixed labels.
- Create: `trading_app/test/features/short_term_emotion/short_term_emotion_trend_chart_test.dart`
  - Tests the time-axis mapping and nearest-point selection through a debug-visible helper API.

## Design Details

The chart must no longer use:

```dart
final x = plot.left + plot.width * index / (items.length - 1);
```

Instead, every point uses:

```dart
final x = axis.xForTime(plot, item.time);
```

The first implementation should treat the full wall-clock span `09:25 -> 15:00` as continuous. That means lunch break time still occupies x-axis space. This matches the user's request literally: "x轴应该是9点25分到15:00". If later the user wants a compressed A-share trading axis excluding lunch, that should be a separate change.

For the current screenshot example, `10:10` should be drawn around:

```text
(10:10 - 09:25) / (15:00 - 09:25)
= 45 / 335
= 13.4%
```

So the right side of the chart remains mostly empty.

---

### Task 1: Add Fixed Trading-Time Axis Helper

**Files:**
- Modify: `trading_app/lib/features/short_term_emotion/presentation/short_term_emotion_trend_chart.dart`
- Create: `trading_app/test/features/short_term_emotion/short_term_emotion_trend_chart_test.dart`

**Interfaces:**
- Produces: `_TradingTimeAxis`
  - `static const startMinute = 9 * 60 + 25`
  - `static const endMinute = 15 * 60`
  - `double ratioForTime(String time)`
  - `double xForTime(Rect plot, String time)`
  - `int nearestIndexForX(Rect plot, List<ShortTermEmotionTrendPoint> items, double x)`
  - `List<String> labels`
- Consumes: existing `ShortTermEmotionTrendPoint`

- [ ] **Step 1: Write the failing test**

Create `trading_app/test/features/short_term_emotion/short_term_emotion_trend_chart_test.dart`:

```dart
import 'package:flutter/material.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:go_stock_app/features/short_term_emotion/domain/short_term_emotion_models.dart';
import 'package:go_stock_app/features/short_term_emotion/presentation/short_term_emotion_trend_chart.dart';

void main() {
  group('Trading time axis', () {
    test('maps market times to fixed 09:25-15:00 ratio', () {
      const axis = TradingTimeAxisDebug();

      expect(axis.ratioForTime('09:25'), 0);
      expect(axis.ratioForTime('15:00'), 1);
      expect(axis.ratioForTime('09:15'), 0);
      expect(axis.ratioForTime('15:10'), 1);
      expect(axis.ratioForTime('10:10'), closeTo(45 / 335, 0.001));
    });

    test('maps x coordinate by fixed market time instead of item index', () {
      const axis = TradingTimeAxisDebug();
      const plot = Rect.fromLTWH(10, 0, 335, 100);

      expect(axis.xForTime(plot, '09:25'), 10);
      expect(axis.xForTime(plot, '15:00'), 345);
      expect(axis.xForTime(plot, '10:10'), closeTo(55, 0.01));
    });

    test('selects nearest item by rendered time position', () {
      const axis = TradingTimeAxisDebug();
      const plot = Rect.fromLTWH(10, 0, 335, 100);
      const items = [
        ShortTermEmotionTrendPoint(
          time: '09:25',
          upCount: 1000,
          downCount: 3000,
          redRate: 25,
          emotionIndex: 0.5,
          limitRatio: 1,
          limitUp: 10,
          limitDown: 10,
        ),
        ShortTermEmotionTrendPoint(
          time: '10:10',
          upCount: 2000,
          downCount: 2500,
          redRate: 44,
          emotionIndex: 1.2,
          limitRatio: 2,
          limitUp: 20,
          limitDown: 10,
        ),
        ShortTermEmotionTrendPoint(
          time: '15:00',
          upCount: 3000,
          downCount: 1800,
          redRate: 62,
          emotionIndex: 2.1,
          limitRatio: 4,
          limitUp: 40,
          limitDown: 10,
        ),
      ];

      expect(axis.nearestIndexForX(plot, items, 54), 1);
      expect(axis.nearestIndexForX(plot, items, 330), 2);
    });
  });
}
```

- [ ] **Step 2: Run test to verify it fails**

Run:

```bash
cd trading_app
flutter test test/features/short_term_emotion/short_term_emotion_trend_chart_test.dart
```

Expected:

```text
Error: Method not found: 'TradingTimeAxisDebug'
```

- [ ] **Step 3: Add minimal helper implementation**

Modify `trading_app/lib/features/short_term_emotion/presentation/short_term_emotion_trend_chart.dart` near the bottom before `_chartPlotRect`:

```dart
@visibleForTesting
class TradingTimeAxisDebug extends _TradingTimeAxis {
  const TradingTimeAxisDebug();
}

class _TradingTimeAxis {
  const _TradingTimeAxis();

  static const startMinute = 9 * 60 + 25;
  static const endMinute = 15 * 60;
  static const labels = ['09:25', '10:30', '11:30', '13:00', '14:00', '15:00'];

  double ratioForTime(String time) {
    final minute = _parseMinuteOfDay(time);
    final clamped = minute.clamp(startMinute, endMinute).toDouble();
    return ((clamped - startMinute) / (endMinute - startMinute)).clamp(0, 1);
  }

  double xForTime(Rect plot, String time) {
    return plot.left + plot.width * ratioForTime(time);
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

  int _parseMinuteOfDay(String time) {
    final parts = time.split(':');
    if (parts.length < 2) return startMinute;
    final hour = int.tryParse(parts[0]) ?? 9;
    final minute = int.tryParse(parts[1]) ?? 25;
    return hour * 60 + minute;
  }
}
```

- [ ] **Step 4: Run test to verify it passes**

Run:

```bash
cd trading_app
flutter test test/features/short_term_emotion/short_term_emotion_trend_chart_test.dart
```

Expected:

```text
All tests passed!
```

- [ ] **Step 5: Commit**

Do not commit yet in this repo unless the user explicitly asks. Record the changed files for review:

```bash
git status --short trading_app/lib/features/short_term_emotion/presentation/short_term_emotion_trend_chart.dart trading_app/test/features/short_term_emotion/short_term_emotion_trend_chart_test.dart
```

---

### Task 2: Use Fixed Time Axis in Chart Drawing and Touch Selection

**Files:**
- Modify: `trading_app/lib/features/short_term_emotion/presentation/short_term_emotion_trend_chart.dart`
- Test: `trading_app/test/features/short_term_emotion/short_term_emotion_trend_chart_test.dart`

**Interfaces:**
- Consumes: `_TradingTimeAxis.xForTime(...)`
- Consumes: `_TradingTimeAxis.nearestIndexForX(...)`
- Produces: chart behavior where bars, lines, selected vertical line, selected dots, and x-axis labels all use the fixed time range.

- [ ] **Step 1: Write the failing behavior test**

Append this test to `trading_app/test/features/short_term_emotion/short_term_emotion_trend_chart_test.dart`:

```dart
testWidgets('early morning data does not visually fill the whole chart', (
  tester,
) async {
  const items = [
    ShortTermEmotionTrendPoint(
      time: '09:25',
      upCount: 1000,
      downCount: 3000,
      redRate: 25,
      emotionIndex: 0.5,
      limitRatio: 1,
      limitUp: 10,
      limitDown: 10,
    ),
    ShortTermEmotionTrendPoint(
      time: '10:10',
      upCount: 2000,
      downCount: 2500,
      redRate: 44,
      emotionIndex: 1.2,
      limitRatio: 2,
      limitUp: 20,
      limitDown: 10,
    ),
  ];

  await tester.pumpWidget(
    const MaterialApp(
      home: Scaffold(
        body: SizedBox(
          width: 430,
          child: ShortTermEmotionTrendChart(items: items),
        ),
      ),
    ),
  );

  expect(find.text('涨跌家数比'), findsOneWidget);
  expect(find.text('盘中趋势'), findsOneWidget);
  expect(find.text('09:25'), findsOneWidget);
  expect(find.text('15:00'), findsOneWidget);
});
```

- [ ] **Step 2: Run test to verify it fails**

Run:

```bash
cd trading_app
flutter test test/features/short_term_emotion/short_term_emotion_trend_chart_test.dart
```

Expected failure:

```text
Expected: exactly one matching candidate
Actual: _TextWidgetFinder:<Found 0 widgets with text "09:25">
```

The current x-axis is painted inside `CustomPainter`, so the text finder may not see it. If this exact widget test is not suitable, keep the helper tests as the primary safety net and verify the visual behavior with a manual App screenshot after implementation.

- [ ] **Step 3: Update selection logic**

Modify `_ShortTermEmotionTrendChartState._updateSelectedPoint`:

```dart
void _updateSelectedPoint(Offset localPosition, Size size) {
  if (widget.items.isEmpty) return;
  final plot = _chartPlotRect(size);
  final clampedX = localPosition.dx.clamp(plot.left, plot.right).toDouble();
  final index = const _TradingTimeAxis().nearestIndexForX(
    plot,
    widget.items,
    clampedX,
  );
  if (index < 0) return;
  setState(() {
    _selectedIndex = index;
  });
}
```

- [ ] **Step 4: Add axis field to painter**

Modify `_TrendChartPainter`:

```dart
class _TrendChartPainter extends CustomPainter {
  _TrendChartPainter(this.items, {this.selectedIndex});

  final List<ShortTermEmotionTrendPoint> items;
  final int? selectedIndex;
  final _axis = const _TradingTimeAxis();
```

- [ ] **Step 5: Update bars to use time x positions**

Replace `_drawBars` with:

```dart
void _drawBars(Canvas canvas, Rect plot, int countMax) {
  final slotWidth = plot.width / (_TradingTimeAxis.endMinute - _TradingTimeAxis.startMinute);
  final barWidth = math.min(4.0, math.max(1.6, slotWidth * 1.8));
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
```

- [ ] **Step 6: Update line points to use time x positions**

Replace `_linePoint` with:

```dart
Offset _linePoint(Rect plot, ShortTermEmotionTrendPoint item, double value) {
  final x = _axis.xForTime(plot, item.time);
  final y = plot.bottom - plot.height * value.clamp(0, 100) / 100;
  return Offset(x, y);
}
```

Then update `_drawLine` loops:

```dart
for (var i = 0; i < items.length; i++) {
  final item = items[i];
  final point = _linePoint(plot, item, valueOf(item));
  if (i == 0) {
    path.moveTo(point.dx, point.dy);
  } else {
    path.lineTo(point.dx, point.dy);
  }
}

for (final item in items) {
  final point = _linePoint(plot, item, valueOf(item));
  canvas.drawCircle(point, 3.4, dotPaint);
  canvas.drawCircle(point, 3.4, dotStroke);
}
```

- [ ] **Step 7: Update fixed x-axis labels**

Replace `_drawXAxis` with:

```dart
void _drawXAxis(Canvas canvas, Rect plot) {
  final textStyle = TextStyle(fontSize: 10, color: AppColors.textTertiary);
  for (final label in _TradingTimeAxis.labels) {
    final x = _axis.xForTime(plot, label);
    _drawRotatedText(
      canvas,
      label,
      Offset(x - 12, plot.bottom + 12),
      textStyle,
    );
  }
}
```

- [ ] **Step 8: Update selected line and selected dots**

In `_drawSelectedPoint`, replace:

```dart
final x = plot.left + plot.width * index / (items.length - 1);
```

with:

```dart
final x = _axis.xForTime(plot, item.time);
```

Replace `_drawSelectedDot` with:

```dart
void _drawSelectedDot(
  Canvas canvas,
  Rect plot,
  ShortTermEmotionTrendPoint item,
  double value,
  Color color,
) {
  final point = _linePoint(plot, item, value);
  canvas.drawCircle(point, 5, Paint()..color = Colors.white);
  canvas.drawCircle(
    point,
    5,
    Paint()
      ..color = color
      ..style = PaintingStyle.stroke
      ..strokeWidth = 2.4,
  );
}
```

Update its three call sites:

```dart
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
```

- [ ] **Step 9: Run focused tests**

Run:

```bash
cd trading_app
flutter test test/features/short_term_emotion/short_term_emotion_trend_chart_test.dart
```

Expected:

```text
All tests passed!
```

- [ ] **Step 10: Run analyzer for the feature**

Run:

```bash
cd trading_app
dart analyze lib/features/short_term_emotion test/features/short_term_emotion
```

Expected:

```text
No issues found!
```

- [ ] **Step 11: Manual App verification**

Run the Flutter App and open the `超短情绪` Tab. With current morning data, verify:

```text
1. x-axis shows fixed labels from 09:25 to 15:00.
2. 10:10 data appears near the left side, not at the far right.
3. The right side of the chart remains empty before later trading times arrive.
4. Long-press or mouse drag near a plotted point still shows the tooltip.
5. Tooltip vertical line aligns with the actual plotted point time.
```

- [ ] **Step 12: Commit**

Do not commit unless requested. Record changed files:

```bash
git status --short trading_app/lib/features/short_term_emotion/presentation/short_term_emotion_trend_chart.dart trading_app/test/features/short_term_emotion/short_term_emotion_trend_chart_test.dart
```

---

## Self-Review

**Spec coverage:** The plan covers fixed `09:25-15:00` x-axis range, gradual left-to-right drawing, chart rendering, axis labels, and tooltip/gesture selection.

**Placeholder scan:** No TBD/TODO placeholders remain. Code snippets include exact class/function names and concrete commands.

**Type consistency:** `TradingTimeAxisDebug` extends `_TradingTimeAxis`; helper methods use `ShortTermEmotionTrendPoint` and `Rect`, matching current chart code.

**Known limitation:** The plan uses continuous wall-clock time including lunch break. This is intentional for this change because the user specifically asked for `09:25` to `15:00` fixed x-axis. A compressed trading-session axis can be planned separately if needed.
