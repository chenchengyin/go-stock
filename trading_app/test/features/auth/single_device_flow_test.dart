import 'dart:async';
import 'dart:convert';
import 'dart:typed_data';

import 'package:dio/dio.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:trading_app/core/network/api_client.dart';
import 'package:trading_app/features/auth/data/auth_repository.dart';
import 'package:trading_app/features/auth/data/auth_storage.dart';
import 'package:trading_app/features/auth/data/session_controller.dart';
import 'package:trading_app/features/auth/presentation/auth_view_model.dart';

void main() {
  TestWidgetsFlutterBinding.ensureInitialized();

  tearDown(resetApiClientForTesting);

  test(
    'device A clears a replaced session while device B remains logged in',
    () async {
      final server = MemorySingleDeviceServer();
      final storageA = MemoryAuthStorage(<String, String>{
        'device:id': 'device-a',
      });
      final controllerA = AuthSessionController(storageA);
      final dioA = createIndependentDeviceClient(
        baseUrl: 'https://device-a.test',
        sessionController: controllerA,
        adapter: MemoryServerAdapter(server.handle),
      );
      addTearDown(() => dioA.close(force: true));
      final authA = AuthViewModel(
        ApiAuthRepository(
          dio: dioA,
          storage: storageA,
          sessionController: controllerA,
        ),
        sessionController: controllerA,
      );
      addTearDown(authA.dispose);

      await authA.register(
        phone: '13800000000',
        password: 'secret123',
        nickname: 'Alice',
      );
      expect(authA.isLoggedIn, isTrue);
      expect(await storageA.read('auth:token'), 'token-device-a');
      expect(await dioA.get<dynamic>('/api/news'), isA<Response<dynamic>>());

      final storageB = MemoryAuthStorage(<String, String>{
        'device:id': 'device-b',
      });
      final controllerB = AuthSessionController(storageB);
      final dioB = createIndependentDeviceClient(
        baseUrl: 'https://device-b.test',
        sessionController: controllerB,
        adapter: MemoryServerAdapter(server.handle),
      );
      addTearDown(() => dioB.close(force: true));
      final authB = AuthViewModel(
        ApiAuthRepository(
          dio: dioB,
          storage: storageB,
          sessionController: controllerB,
        ),
        sessionController: controllerB,
      );
      addTearDown(authB.dispose);

      await authB.login(phone: '13800000000', password: 'secret123');
      expect(authB.isLoggedIn, isTrue);
      expect(await storageB.read('auth:token'), 'token-device-b');

      await expectLater(
        dioA.get<dynamic>('/api/news'),
        throwsA(
          isA<DioException>()
              .having((error) => error.response?.statusCode, 'status', 401)
              .having(
                (error) =>
                    (error.response?.data as Map<String, dynamic>)['code'],
                'code',
                'SESSION_REPLACED',
              ),
        ),
      );

      expect(storageA.clearAuthCalls, 1);
      expect(await storageA.read('auth:token'), isNull);
      expect(await storageA.read('device:id'), 'device-a');
      expect(
        controllerA.lastInvalidationReason,
        SessionInvalidationReason.replaced,
      );
      expect(authA.isLoggedIn, isFalse);

      final responseB = await dioB.get<dynamic>('/api/news');
      expect(responseB.statusCode, 200);
      expect(responseB.data, <String, dynamic>{'ok': true});
      expect(storageB.clearAuthCalls, 0);
      expect(await storageB.read('auth:token'), 'token-device-b');
      expect(await storageB.read('device:id'), 'device-b');
      expect(controllerB.lastInvalidationReason, isNull);
      expect(authB.isLoggedIn, isTrue);
    },
  );
}

Dio createIndependentDeviceClient({
  required String baseUrl,
  required SessionController sessionController,
  required HttpClientAdapter adapter,
}) {
  final configured = createApiClient(
    baseUrl: baseUrl,
    sessionController: sessionController,
  );
  final client = Dio(BaseOptions(baseUrl: baseUrl));
  client.interceptors.addAll(configured.interceptors);
  client.httpClientAdapter = adapter;
  return client;
}

class MemoryAuthStorage implements AuthStorage {
  MemoryAuthStorage([Map<String, String>? values])
    : _values = <String, String>{...?values};

  final Map<String, String> _values;
  int clearAuthCalls = 0;

  @override
  Future<void> clearAuth() async {
    clearAuthCalls++;
    _values.removeWhere((key, _) => key.startsWith('auth:'));
  }

  @override
  Future<String> getOrCreateDeviceId() async {
    final deviceId = _values['device:id'];
    if (deviceId == null || deviceId.isEmpty) {
      throw StateError('the test must provide a persistent device id');
    }
    return deviceId;
  }

  @override
  Future<String?> read(String key) async => _values[key];

  @override
  Future<void> write(String key, String value) async {
    _values[key] = value;
  }
}

class MemorySingleDeviceServer {
  static const _expiresAt = '2026-09-29T09:00:00Z';
  static const _user = <String, dynamic>{
    'id': 'user-1',
    'phone': '13800000000',
    'nickname': 'Alice',
    'role': 'user',
  };

  String? _registeredPhone;
  String? _activeToken;

  TestResponse handle(RequestOptions request) {
    switch ((request.method, request.uri.path)) {
      case ('POST', '/api/auth/register'):
        final body = request.data as Map<String, dynamic>;
        _registeredPhone = body['phone'] as String;
        return _createSession(body['deviceId'] as String, 201);
      case ('POST', '/api/auth/login'):
        final body = request.data as Map<String, dynamic>;
        if (body['phone'] != _registeredPhone ||
            body['password'] != 'secret123') {
          return const TestResponse.json(401, <String, dynamic>{
            'code': 'INVALID_CREDENTIALS',
            'message': '账号或密码错误',
          });
        }
        return _createSession(body['deviceId'] as String, 200);
      case ('GET', '/api/news'):
        final authorization = request.headers['Authorization'];
        if (authorization != 'Bearer $_activeToken') {
          return const TestResponse.json(401, <String, dynamic>{
            'code': 'SESSION_REPLACED',
            'message': '账号已在其他设备登录，请重新登录',
          });
        }
        return const TestResponse.json(200, <String, dynamic>{'ok': true});
      default:
        return const TestResponse.json(404, <String, dynamic>{
          'code': 'NOT_FOUND',
        });
    }
  }

  TestResponse _createSession(String deviceId, int statusCode) {
    _activeToken = 'token-$deviceId';
    return TestResponse.json(statusCode, <String, dynamic>{
      'accessToken': _activeToken,
      'user': _user,
      'expiresAt': _expiresAt,
    });
  }
}

class TestResponse {
  const TestResponse.json(this.statusCode, this.data);

  final int statusCode;
  final Object? data;
}

class MemoryServerAdapter implements HttpClientAdapter {
  MemoryServerAdapter(this._handler);

  final FutureOr<TestResponse> Function(RequestOptions request) _handler;

  @override
  Future<ResponseBody> fetch(
    RequestOptions options,
    Stream<Uint8List>? requestStream,
    Future<void>? cancelFuture,
  ) async {
    final response = await _handler(options);
    return ResponseBody.fromString(
      jsonEncode(response.data),
      response.statusCode,
      headers: <String, List<String>>{
        Headers.contentTypeHeader: <String>[Headers.jsonContentType],
      },
    );
  }

  @override
  void close({bool force = false}) {}
}
