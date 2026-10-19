import 'package:flutter/material.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:trading_app/features/radar/presentation/radar_list/t0_strategy_view_model.dart';
import 'package:trading_app/features/radar/presentation/radar_list/red_rule_widgets.dart';

void main() {
  testWidgets('窄屏名称省略仍完整显示真赚达标样本', (tester) async {
    final o = T0RedRuleHistoryOption.fromJson({
      'name': '板阴·长实上半',
      'rule_key': 'a',
      'rule_version': 1,
      'stats_status': 'verified',
      'stats': {
        'sample_count': 66,
        'profit_win_rate': 71.21,
        'target_rate': 57.58,
      },
    });
    await tester.pumpWidget(
      MaterialApp(
        home: Scaffold(
          body: SizedBox(width: 220, child: RedRuleOptionLabel(option: o)),
        ),
      ),
    );
    expect(find.text('(71/58/66)'), findsOneWidget);
    expect(tester.takeException(), isNull);
  });
  testWidgets('长按命中标签列出全部条件及各自统计', (tester) async {
    final h = [
      T0RedRuleHit.fromJson({
        'rule_key': 'a',
        'name': '板阴·长实上半',
        'description': '涨停实体至少8%',
        'stats_status': 'verified',
        'stats': {
          'sample_count': 66,
          'profit_win_rate': 71.21,
          'target_rate': 57.58,
        },
      }),
      T0RedRuleHit.fromJson({
        'rule_key': 'b',
        'name': '板阴·窄幅增强',
        'description': '前日振幅不超过8%',
        'stats_status': 'pending',
      }),
    ];
    await tester.pumpWidget(
      MaterialApp(
        home: Scaffold(body: RedRuleMatchBadge(hits: h)),
      ),
    );
    await tester.longPress(find.text('命中2组'));
    await tester.pumpAndSettle();
    expect(find.text('板阴·长实上半'), findsOneWidget);
    expect(find.text('板阴·窄幅增强'), findsOneWidget);
    expect(find.textContaining('涨停实体至少8%'), findsOneWidget);
    expect(find.textContaining('(71/58/66)'), findsOneWidget);
    expect(find.textContaining('统计待核验'), findsOneWidget);
  });
  testWidgets('未核验统计不用零胜率替代', (tester) async {
    final o = T0RedRuleHistoryOption.fromJson({
      'name': '板阴·小高开',
      'rule_key': 'x',
      'rule_version': 1,
      'match_count': 0,
    });
    await tester.pumpWidget(
      MaterialApp(
        home: Scaffold(
          body: SizedBox(width: 250, child: RedRuleOptionLabel(option: o)),
        ),
      ),
    );
    expect(find.text('(—/—/—)'), findsOneWidget);
    expect(find.text('(0/0/0)'), findsNothing);
  });
}
