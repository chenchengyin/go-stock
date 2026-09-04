import 'package:flutter_test/flutter_test.dart';
import 'package:shared_preferences/shared_preferences.dart';
import 'package:trading_app/features/permissions/presentation/module_permission_controller.dart';

import 'radar_page_test_fixtures.dart';

void main() {
  TestWidgetsFlutterBinding.ensureInitialized();

  setUp(() {
    SharedPreferences.setMockInitialValues({'voice_announcement_asked': true});
  });

  testWidgets('only public modules render three tabs', (tester) async {
    await tester.pumpWidget(radarTestApp(modules: publicRadarModules));
    await tester.pump();

    expect(find.text('监控股票(自选)'), findsOneWidget);
    expect(find.text('自选异动'), findsOneWidget);
    expect(find.text('全市场'), findsOneWidget);
    expect(find.text('紫策'), findsNothing);
    expect(find.text('主板策略'), findsNothing);
    expect(find.text('蓝策'), findsNothing);
  });

  testWidgets('hiding middle strategy keeps remaining tab identities', (
    tester,
  ) async {
    await tester.pumpWidget(
      radarTestApp(
        modules: [
          ...publicRadarModules,
          ...radarModulesFor(['radar.purple_strategy', 'radar.blue_strategy']),
        ],
      ),
    );
    await tester.pump();

    expect(find.text('紫策'), findsOneWidget);
    expect(find.text('主板策略'), findsNothing);
    expect(find.text('蓝策'), findsOneWidget);
  });

  testWidgets('revoking current module returns to first visible tab', (
    tester,
  ) async {
    final permissions = ModulePermissionController.forTesting([
      ...publicRadarModules,
      ...radarModulesFor(['radar.main_strategy']),
    ]);
    addTearDown(permissions.dispose);

    await tester.pumpWidget(radarTestApp(permissionController: permissions));
    await tester.pump();
    await tester.tap(find.text('主板策略'));
    await tester.pump();

    permissions.revoke('radar.main_strategy');
    await tester.pumpAndSettle();

    expect(find.text('主板策略'), findsNothing);
    expect(find.text('监控股票(自选)'), findsOneWidget);
  });
}
