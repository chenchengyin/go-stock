import 'package:dio/dio.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:trading_app/features/permissions/data/module_permission_repository.dart';
import 'package:trading_app/features/permissions/domain/module_definition.dart';

void main() {
  test('parses module metadata and nullable parent code', () {
    final snapshot = ModulePermissionSnapshot.fromJson({
      'version': 1,
      'modules': [
        {
          'code': 'radar.main_strategy',
          'name': '主板策略',
          'client': 'flutter_web',
          'placement': 'radar_tab',
          'parentCode': null,
          'sort': 30,
          'accessMode': 'user_allowlist',
        },
      ],
    });

    expect(snapshot.version, 1);
    expect(snapshot.modules.single.code, 'radar.main_strategy');
    expect(snapshot.modules.single.parentCode, isNull);
    expect(snapshot.modules.single.accessMode, ModuleAccessMode.userAllowlist);
  });

  test('repository requests the authenticated module endpoint', () async {
    final dio = Dio(BaseOptions(baseUrl: 'http://test'));
    String? requestedPath;
    dio.interceptors.add(
      InterceptorsWrapper(
        onRequest: (options, handler) {
          requestedPath = options.path;
          handler.resolve(
            Response(
              requestOptions: options,
              data: {'version': 1, 'modules': <dynamic>[]},
            ),
          );
        },
      ),
    );

    final snapshot = await ModulePermissionRepository(dio).fetchModules();

    expect(snapshot.version, 1);
    expect(requestedPath, '/api/auth/modules');
  });
}
