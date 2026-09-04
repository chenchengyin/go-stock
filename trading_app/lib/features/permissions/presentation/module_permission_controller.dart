import 'package:flutter/foundation.dart';

import '../data/module_permission_repository.dart';
import '../domain/module_definition.dart';

enum ModulePermissionState { initial, loading, ready, failure }

class ModulePermissionController extends ChangeNotifier {
  ModulePermissionController(this._source);

  ModulePermissionController.forTesting(Iterable<ModuleDefinition> modules)
    : _source = null,
      _visibleModules = _sortedModules(modules),
      _state = ModulePermissionState.ready;

  final ModulePermissionSource? _source;
  List<ModuleDefinition> _visibleModules = const [];
  ModulePermissionState _state = ModulePermissionState.initial;
  String? _error;
  Future<void>? _pendingLoad;

  ModulePermissionState get state => _state;
  String? get error => _error;
  List<ModuleDefinition> get visibleModules =>
      List.unmodifiable(_visibleModules);

  bool canView(String moduleCode) {
    return _visibleModules.any((module) => module.code == moduleCode);
  }

  Future<void> load({bool force = false}) {
    if (_source == null) return Future<void>.value();
    if (!force && _state == ModulePermissionState.ready) {
      return Future<void>.value();
    }
    final pending = _pendingLoad;
    if (pending != null) return pending;

    final future = _loadFromServer();
    _pendingLoad = future;
    future.whenComplete(() {
      if (identical(_pendingLoad, future)) _pendingLoad = null;
    });
    return future;
  }

  Future<void> refresh() => load(force: true);

  void revoke(String moduleCode) {
    final index = _visibleModules.indexWhere(
      (module) =>
          module.code == moduleCode &&
          module.accessMode == ModuleAccessMode.userAllowlist,
    );
    if (index < 0) return;
    final next = List<ModuleDefinition>.from(_visibleModules)..removeAt(index);
    _visibleModules = List.unmodifiable(next);
    notifyListeners();
  }

  Future<void> _loadFromServer() async {
    _state = ModulePermissionState.loading;
    _error = null;
    notifyListeners();

    try {
      final snapshot = await _source!.fetchModules();
      final modulesByCode = <String, ModuleDefinition>{
        for (final module in publicModuleDefinitions) module.code: module,
      };
      for (final module in snapshot.modules) {
        if (module.client != 'flutter_web' || module.placement != 'radar_tab') {
          continue;
        }
        modulesByCode[module.code] = module;
      }
      _visibleModules = _sortedModules(modulesByCode.values);
      _state = ModulePermissionState.ready;
    } catch (error) {
      _visibleModules = List.unmodifiable(publicModuleDefinitions);
      _state = ModulePermissionState.failure;
      _error = error.toString();
    }
    notifyListeners();
  }

  static List<ModuleDefinition> _sortedModules(
    Iterable<ModuleDefinition> modules,
  ) {
    final out = List<ModuleDefinition>.from(modules);
    out.sort((left, right) {
      final sortOrder = left.sort.compareTo(right.sort);
      return sortOrder == 0 ? left.code.compareTo(right.code) : sortOrder;
    });
    return List.unmodifiable(out);
  }
}
