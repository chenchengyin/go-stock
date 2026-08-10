import 'package:flutter/material.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:trading_app/features/short_term_emotion/domain/short_term_emotion_models.dart';
import 'package:trading_app/features/short_term_emotion/presentation/short_term_emotion_trend_chart.dart';

void main() {
  group('Trading time axis', () {
    test('maps market times to compressed A-share trading-session ratio', () {
      const axis = TradingTimeAxisDebug();

      expect(axis.ratioForTime('09:25'), 0);
      expect(axis.ratioForTime('15:00'), 1);
      expect(axis.ratioForTime('09:15'), 0);
      expect(axis.ratioForTime('15:10'), 1);
      expect(axis.ratioForTime('10:10'), closeTo(45 / 245, 0.001));
      expect(axis.ratioForTime('11:30'), closeTo(125 / 245, 0.001));
      expect(axis.ratioForTime('12:00'), closeTo(125 / 245, 0.001));
      expect(axis.ratioForTime('13:00'), closeTo(125 / 245, 0.001));
      expect(axis.ratioForTime('14:00'), closeTo(185 / 245, 0.001));
    });

    test('maps x coordinate by fixed market time instead of item index', () {
      const axis = TradingTimeAxisDebug();
      const plot = Rect.fromLTWH(10, 0, 335, 100);

      expect(axis.xForTime(plot, '09:25'), 10);
      expect(axis.xForTime(plot, '15:00'), 345);
      expect(axis.xForTime(plot, '10:10'), closeTo(71.53, 0.01));
      expect(axis.xForTime(plot, '11:30'), axis.xForTime(plot, '13:00'));
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

    test('samples dense trend points by two-minute interval', () {
      final items = [
        _point('09:25'),
        _point('09:26'),
        _point('09:27'),
        _point('09:28'),
        _point('09:29'),
        _point('11:30'),
        _point('13:00'),
        _point('13:01'),
        _point('13:02'),
      ];

      final sampled = sampleTrendPointsForChartDebug(items);

      expect(sampled.map((item) => item.time), [
        '09:25',
        '09:27',
        '09:29',
        '11:30',
        '13:02',
      ]);
    });
  });
}

ShortTermEmotionTrendPoint _point(String time) {
  return ShortTermEmotionTrendPoint(
    time: time,
    upCount: 1000,
    downCount: 3000,
    redRate: 25,
    emotionIndex: 0.5,
    limitRatio: 1,
    limitUp: 10,
    limitDown: 10,
  );
}
