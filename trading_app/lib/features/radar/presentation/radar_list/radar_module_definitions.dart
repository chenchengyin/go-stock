import 'package:flutter/foundation.dart';

import '../../../permissions/domain/module_definition.dart';

enum RadarContentKind {
  monitored,
  purpleStrategy,
  mainStrategy,
  blueStrategy,
  watchChanges,
  allChanges,
}

@immutable
class RadarModuleDefinition {
  const RadarModuleDefinition({
    required this.code,
    required this.name,
    required this.sort,
    required this.accessMode,
    required this.contentKind,
  });

  final String code;
  final String name;
  final int sort;
  final ModuleAccessMode accessMode;
  final RadarContentKind contentKind;
}

const radarModuleDefinitions = <RadarModuleDefinition>[
  RadarModuleDefinition(
    code: 'radar.monitored',
    name: '监控股票(自选)',
    sort: 10,
    accessMode: ModuleAccessMode.public,
    contentKind: RadarContentKind.monitored,
  ),
  RadarModuleDefinition(
    code: 'radar.purple_strategy',
    name: '紫策',
    sort: 20,
    accessMode: ModuleAccessMode.userAllowlist,
    contentKind: RadarContentKind.purpleStrategy,
  ),
  RadarModuleDefinition(
    code: 'radar.main_strategy',
    name: '主板策略',
    sort: 30,
    accessMode: ModuleAccessMode.userAllowlist,
    contentKind: RadarContentKind.mainStrategy,
  ),
  RadarModuleDefinition(
    code: 'radar.blue_strategy',
    name: '蓝策',
    sort: 40,
    accessMode: ModuleAccessMode.userAllowlist,
    contentKind: RadarContentKind.blueStrategy,
  ),
  RadarModuleDefinition(
    code: 'radar.watch_changes',
    name: '自选异动',
    sort: 50,
    accessMode: ModuleAccessMode.public,
    contentKind: RadarContentKind.watchChanges,
  ),
  RadarModuleDefinition(
    code: 'radar.all_changes',
    name: '全市场',
    sort: 60,
    accessMode: ModuleAccessMode.public,
    contentKind: RadarContentKind.allChanges,
  ),
];
