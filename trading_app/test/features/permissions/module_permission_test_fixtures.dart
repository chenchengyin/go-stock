import 'package:trading_app/features/permissions/data/module_permission_repository.dart';
import 'package:trading_app/features/permissions/domain/module_definition.dart';

ModuleDefinition moduleFor(String code) {
  return switch (code) {
    'radar.monitored' => publicModuleDefinitions[0],
    'radar.watch_changes' => publicModuleDefinitions[1],
    'radar.all_changes' => publicModuleDefinitions[2],
    'radar.purple_strategy' => const ModuleDefinition(
      code: 'radar.purple_strategy',
      name: '紫策',
      client: 'flutter_web',
      placement: 'radar_tab',
      parentCode: null,
      sort: 20,
      accessMode: ModuleAccessMode.userAllowlist,
    ),
    'radar.main_strategy' => const ModuleDefinition(
      code: 'radar.main_strategy',
      name: '主板策略',
      client: 'flutter_web',
      placement: 'radar_tab',
      parentCode: null,
      sort: 30,
      accessMode: ModuleAccessMode.userAllowlist,
    ),
    'radar.blue_strategy' => const ModuleDefinition(
      code: 'radar.blue_strategy',
      name: '蓝策',
      client: 'flutter_web',
      placement: 'radar_tab',
      parentCode: null,
      sort: 40,
      accessMode: ModuleAccessMode.userAllowlist,
    ),
    _ => throw ArgumentError('Unknown test module: $code'),
  };
}

List<ModuleDefinition> modulesFor(Iterable<String> codes) {
  return codes.map(moduleFor).toList();
}

class FakeModulePermissionSource implements ModulePermissionSource {
  FakeModulePermissionSource({
    this.modules = const [],
    this.error,
    this.beforeReturn,
  });

  final List<ModuleDefinition> modules;
  final Object? error;
  final Future<void> Function()? beforeReturn;
  int calls = 0;

  @override
  Future<ModulePermissionSnapshot> fetchModules() async {
    calls++;
    if (beforeReturn != null) await beforeReturn!();
    if (error != null) throw error!;
    return ModulePermissionSnapshot(version: 1, modules: modules);
  }
}
