import 'package:dio/dio.dart';

import '../domain/module_definition.dart';

abstract interface class ModulePermissionSource {
  Future<ModulePermissionSnapshot> fetchModules();
}

class ModulePermissionRepository implements ModulePermissionSource {
  const ModulePermissionRepository(this._dio);

  final Dio _dio;

  @override
  Future<ModulePermissionSnapshot> fetchModules() async {
    final response = await _dio.get<dynamic>('/api/auth/modules');
    final data = response.data;
    if (data is! Map) {
      throw const FormatException(
        'Module permission response must be an object',
      );
    }
    return ModulePermissionSnapshot.fromJson(Map<String, dynamic>.from(data));
  }
}
