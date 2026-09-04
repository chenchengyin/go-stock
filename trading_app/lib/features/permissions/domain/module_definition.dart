import 'package:flutter/foundation.dart';

enum ModuleAccessMode { public, userAllowlist }

ModuleAccessMode _parseAccessMode(Object? value) {
  return switch (value) {
    'public' => ModuleAccessMode.public,
    'user_allowlist' => ModuleAccessMode.userAllowlist,
    _ => throw FormatException('Unsupported module access mode: $value'),
  };
}

@immutable
class ModuleDefinition {
  const ModuleDefinition({
    required this.code,
    required this.name,
    required this.client,
    required this.placement,
    required this.parentCode,
    required this.sort,
    required this.accessMode,
  });

  factory ModuleDefinition.fromJson(Map<String, dynamic> json) {
    final code = (json['code'] as String?)?.trim() ?? '';
    if (code.isEmpty) {
      throw const FormatException('Module code must not be empty');
    }

    final parentCode = (json['parentCode'] as String?)?.trim();
    return ModuleDefinition(
      code: code,
      name: (json['name'] as String?)?.trim() ?? '',
      client: (json['client'] as String?)?.trim() ?? '',
      placement: (json['placement'] as String?)?.trim() ?? '',
      parentCode: parentCode == null || parentCode.isEmpty ? null : parentCode,
      sort: (json['sort'] as num?)?.toInt() ?? 0,
      accessMode: _parseAccessMode(json['accessMode']),
    );
  }

  final String code;
  final String name;
  final String client;
  final String placement;
  final String? parentCode;
  final int sort;
  final ModuleAccessMode accessMode;
}

@immutable
class ModulePermissionSnapshot {
  const ModulePermissionSnapshot({
    required this.version,
    required this.modules,
  });

  factory ModulePermissionSnapshot.fromJson(Map<String, dynamic> json) {
    final rawVersion = (json['version'] as num?)?.toInt();
    if (rawVersion != 1) {
      throw FormatException(
        'Unsupported module permission version: $rawVersion',
      );
    }

    final rawModules = json['modules'];
    if (rawModules is! List) {
      throw const FormatException('Module permission modules must be a list');
    }

    final modules = rawModules
        .map((entry) {
          if (entry is! Map) {
            throw const FormatException('Module definition must be an object');
          }
          return ModuleDefinition.fromJson(Map<String, dynamic>.from(entry));
        })
        .toList(growable: true);
    modules.sort((left, right) {
      final sortOrder = left.sort.compareTo(right.sort);
      return sortOrder == 0 ? left.code.compareTo(right.code) : sortOrder;
    });

    return ModulePermissionSnapshot(
      version: rawVersion!,
      modules: List.unmodifiable(modules),
    );
  }

  final int version;
  final List<ModuleDefinition> modules;
}

const publicModuleDefinitions = <ModuleDefinition>[
  ModuleDefinition(
    code: 'radar.monitored',
    name: '监控股票（自选）',
    client: 'flutter_web',
    placement: 'radar_tab',
    parentCode: null,
    sort: 10,
    accessMode: ModuleAccessMode.public,
  ),
  ModuleDefinition(
    code: 'radar.watch_changes',
    name: '自选异动',
    client: 'flutter_web',
    placement: 'radar_tab',
    parentCode: null,
    sort: 50,
    accessMode: ModuleAccessMode.public,
  ),
  ModuleDefinition(
    code: 'radar.all_changes',
    name: '全市场',
    client: 'flutter_web',
    placement: 'radar_tab',
    parentCode: null,
    sort: 60,
    accessMode: ModuleAccessMode.public,
  ),
];
