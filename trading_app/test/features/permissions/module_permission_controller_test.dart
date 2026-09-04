import 'dart:async';

import 'package:flutter_test/flutter_test.dart';
import 'package:trading_app/features/permissions/presentation/module_permission_controller.dart';

import 'module_permission_test_fixtures.dart';

void main() {
  test('failure falls back to the three public modules', () async {
    final controller = ModulePermissionController(
      FakeModulePermissionSource(error: StateError('offline')),
    );
    addTearDown(controller.dispose);

    await controller.load();

    expect(controller.state, ModulePermissionState.failure);
    expect(controller.visibleModules.map((module) => module.code), [
      'radar.monitored',
      'radar.watch_changes',
      'radar.all_changes',
    ]);
    expect(controller.canView('radar.main_strategy'), isFalse);
  });

  test('revoke removes only the selected controlled module', () async {
    final controller = ModulePermissionController(
      FakeModulePermissionSource(
        modules: modulesFor(['radar.main_strategy', 'radar.blue_strategy']),
      ),
    );
    addTearDown(controller.dispose);

    await controller.load();
    controller.revoke('radar.main_strategy');

    expect(controller.canView('radar.main_strategy'), isFalse);
    expect(controller.canView('radar.blue_strategy'), isTrue);
  });

  test('concurrent loads share one permission request', () async {
    final gate = Completer<void>();
    final source = FakeModulePermissionSource(
      modules: modulesFor(['radar.main_strategy']),
      beforeReturn: () => gate.future,
    );
    final controller = ModulePermissionController(source);
    addTearDown(controller.dispose);

    final first = controller.load();
    final second = controller.load();
    expect(source.calls, 1);

    gate.complete();
    await Future.wait<void>([first, second]);

    expect(controller.state, ModulePermissionState.ready);
    expect(source.calls, 1);
  });
}
