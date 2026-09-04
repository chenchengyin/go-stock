import 'package:flutter/material.dart';
import 'package:provider/provider.dart';
import 'package:trading_app/core/theme/app_colors.dart';
import 'package:trading_app/features/permissions/domain/module_definition.dart';
import 'package:trading_app/features/permissions/presentation/module_permission_controller.dart';
import 'package:trading_app/features/radar/data/radar_repository.dart';
import 'package:trading_app/features/radar/domain/voice_announcement_view_model.dart';
import 'package:trading_app/features/radar/presentation/radar_list/radar_page.dart';
import 'package:trading_app/features/radar/presentation/radar_list/radar_view_model.dart';
import 'package:trading_app/features/radar/presentation/radar_list/t0_strategy_view_model.dart';

final publicRadarModules = List<ModuleDefinition>.unmodifiable(
  publicModuleDefinitions,
);

List<ModuleDefinition> radarModulesFor(Iterable<String> codes) {
  return codes.map(_moduleFor).toList();
}

ModuleDefinition _moduleFor(String code) {
  return switch (code) {
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
    _ => throw ArgumentError('Unknown radar test module: $code'),
  };
}

Widget radarTestApp({
  List<ModuleDefinition>? modules,
  ModulePermissionController? permissionController,
}) {
  final permissions =
      permissionController ??
      ModulePermissionController.forTesting(modules ?? publicRadarModules);
  return AppColorsWidget(
    colors: AppColors.instance,
    variant: AppThemeVariant.light,
    child: MultiProvider(
      providers: [
        ChangeNotifierProvider(
          create: (_) => RadarViewModel(RadarRepositoryImpl()),
        ),
        ChangeNotifierProvider<T0StrategyViewModel>(
          create: (_) => _NoNetworkRadarT0ViewModel(),
        ),
        ChangeNotifierProvider<VoiceAnnouncementViewModel>(
          create: (_) => _TestVoiceAnnouncementViewModel(),
        ),
        ChangeNotifierProvider<ModulePermissionController>.value(
          value: permissions,
        ),
      ],
      child: const MaterialApp(home: RadarPage()),
    ),
  );
}

class _NoNetworkRadarT0ViewModel extends T0StrategyViewModel {
  @override
  Future<void> warmUpIfNeeded({
    String moduleCode = t0MainStrategyModuleCode,
  }) async {}

  @override
  Future<void> loadAvailableDates({
    String moduleCode = t0MainStrategyModuleCode,
  }) async {}

  @override
  Future<void> loadResults({
    String moduleCode = t0MainStrategyModuleCode,
    String? date,
    bool archived = false,
  }) async {}
}

class _TestVoiceAnnouncementViewModel extends VoiceAnnouncementViewModel {
  @override
  bool get askedBefore => true;

  // The test does not install the flutter_tts platform channel.
  @override
  // ignore: must_call_super
  void dispose() {}
}
