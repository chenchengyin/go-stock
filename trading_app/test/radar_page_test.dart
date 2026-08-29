import 'package:flutter/material.dart';
import 'package:flutter/services.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:provider/provider.dart';
import 'package:shared_preferences/shared_preferences.dart';
import 'package:trading_app/features/radar/data/radar_repository.dart';
import 'package:trading_app/features/radar/domain/voice_announcement_view_model.dart';
import 'package:trading_app/features/radar/presentation/radar_list/radar_page.dart';
import 'package:trading_app/features/radar/presentation/radar_list/radar_view_model.dart';
import 'package:trading_app/features/radar/presentation/radar_list/t0_strategy_view_model.dart';

void main() {
  testWidgets('主板策略页内容支持文本选择', (tester) async {
    SharedPreferences.setMockInitialValues({'voice_announcement_asked': true});
    final radarVm = RadarViewModel(RadarRepositoryImpl());
    final strategyVm = T0StrategyViewModel();
    final voiceVm = VoiceAnnouncementViewModel();

    await tester.pumpWidget(
      MultiProvider(
        providers: [
          ChangeNotifierProvider.value(value: radarVm),
          ChangeNotifierProvider.value(value: strategyVm),
          ChangeNotifierProvider.value(value: voiceVm),
        ],
        child: MaterialApp(home: const RadarPage()),
      ),
    );
    await tester.tap(find.byType(Tab).at(1));
    await tester.pumpAndSettle();

    expect(find.byType(SelectionArea), findsOneWidget);

    // RadarViewModel 启动了周期刷新；测试结束前取消定时器，避免测试框架报未处理定时器。
    radarVm.dispose();
    strategyVm.dispose();
    voiceVm.dispose();
  });

  testWidgets('主板策略条目复制按钮复制纯数字股票代码', (tester) async {
    String? copiedText;
    final messenger =
        TestDefaultBinaryMessengerBinding.instance.defaultBinaryMessenger;
    messenger.setMockMethodCallHandler(SystemChannels.platform, (call) async {
      if (call.method == 'Clipboard.setData') {
        copiedText =
            (call.arguments as Map<dynamic, dynamic>)['text'] as String?;
      }
      if (call.method == 'Clipboard.getData') {
        return {'text': copiedText};
      }
      return null;
    });
    addTearDown(() {
      messenger.setMockMethodCallHandler(SystemChannels.platform, null);
    });

    await tester.pumpWidget(
      MaterialApp(
        home: const Scaffold(body: StockCodeCopyButton(code: '600519.XSHG')),
      ),
    );

    final copyButton = find.byTooltip('复制股票代码', skipOffstage: false);
    expect(copyButton, findsOneWidget);
    await tester.tap(copyButton);
    await tester.pump();

    final clipboard = await Clipboard.getData('text/plain');
    expect(clipboard?.text, '600519');
    expect(find.text('股票代码已复制'), findsOneWidget);
  });
}
