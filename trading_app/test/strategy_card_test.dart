import 'package:flutter/material.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:trading_app/features/radar/presentation/radar_list/strategy_card.dart';

void main() {
  for (final width in [320.0, 1000.0]) {
    testWidgets(
      'card slots wrap at $width and actions do not trigger card tap',
      (tester) async {
        await tester.binding.setSurfaceSize(Size(width, 600));
        addTearDown(() => tester.binding.setSurfaceSize(null));
        var opened = 0;
        var copied = 0;
        await tester.pumpWidget(
          MaterialApp(
            home: Scaffold(
              body: Align(
                alignment: Alignment.topLeft,
                child: SizedBox(
                  width: width,
                  child: StrategyCard(
                    onTap: () => opened++,
                    header: StrategyCardHeader(
                      information: const [
                        Text('股票名称'),
                        Text('70/80/123'),
                        Text('重仓'),
                      ],
                      actions: [
                        TextButton(
                          onPressed: () => copied++,
                          child: const Text('复制'),
                        ),
                      ],
                    ),
                    sections: const [Text('日K区域'), Text('分时区域')],
                  ),
                ),
              ),
            ),
          ),
        );
        expect(find.text('日K区域'), findsOneWidget);
        expect(find.text('分时区域'), findsOneWidget);
        await tester.tap(find.text('复制'));
        expect(copied, 1);
        expect(opened, 0);
        await tester.tap(find.text('股票名称'));
        expect(opened, 1);
        expect(tester.takeException(), isNull);
      },
    );
  }

  testWidgets('long headers and labels wrap on narrow screens', (tester) async {
    await tester.pumpWidget(
      const MaterialApp(
        home: Scaffold(
          body: SizedBox(
            width: 320,
            child: StrategyCard(
              header: StrategyCardHeader(
                information: [
                  Text('非常长的股票名称与额外描述'),
                  Text('条件组合及统计信息 70/80/123456'),
                ],
                actions: [Text('开盘+1.23%')],
              ),
            ),
          ),
        ),
      ),
    );
    expect(find.text('开盘+1.23%'), findsOneWidget);
    expect(tester.takeException(), isNull);
  });
}
