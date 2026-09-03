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

      final registration = await authA.register(
        phone: '13800000000',
        password: 'secret123',
        nickname: 'Alice',
      );
      expect(registration, isNotNull);
      expect(registration!.status, 'disabled');
      expect(authA.isLoggedIn, isFalse);
      expect(await storageA.read('auth:token'), isNull);
      server.enableUser();
      await authA.login(phone: '13800000000', password: 'secret123');
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
                (error) => error.requestOptions.headers['Authorization'],
                'bearer token',
                'Bearer token-device-a',
              )
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

      final probe = Dio(BaseOptions(baseUrl: 'https://probe.test'))
        ..httpClientAdapter = MemoryServerAdapter(server.handle);
      addTearDown(() => probe.close(force: true));
      await expectAuthErrorCode(
        probe.get<dynamic>('/api/news'),
        'UNAUTHENTICATED',
      );
      await expectAuthErrorCode(
        probe.get<dynamic>(
          '/api/news',
          options: Options(
            headers: <String, dynamic>{'Authorization': 'Bearer token-unknown'},
          ),
        ),
        'UNAUTHENTICATED',
      );

      await authB.logout();
      expect(authB.isLoggedIn, isFalse);
      expect(await storageB.read('auth:token'), isNull);
      await expectAuthErrorCode(
        probe.get<dynamic>(
          '/api/news',
          options: Options(
            headers: <String, dynamic>{
              'Authorization': 'Bearer token-device-b',
            },
          ),
        ),
        'UNAUTHENTICATED',
      );
    },
  );
}

Future<void> expectAuthErrorCode(
  Future<Response<dynamic>> request,
  String code,
) async {
  await expectLater(
    request,
    throwsA(
      isA<DioException>()
          .having((error) => error.response?.statusCode, 'status', 401)
          .having(
            (error) => (error.response?.data as Map<String, dynamic>)['code'],
            'code',
            code,
          ),
    ),
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
  bool _enabled = false;
  String? _activeToken;
  final Map<String, MemoryTokenState> _tokenStates =
      <String, MemoryTokenState>{};

  TestResponse handle(RequestOptions request) {
    switch ((request.method, request.uri.path)) {
      case ('POST', '/api/auth/register'):
        final body = request.data as Map<String, dynamic>;
        _registeredPhone = body['phone'] as String;
        _enabled = false;
        _activeToken = null;
        _tokenStates.clear();
        return const TestResponse.json(201, <String, dynamic>{
          'user': _user,
          'status': 'disabled',
          'message': '注册成功，请等待管理员启用后再登录',
        });
      case ('POST', '/api/auth/login'):
        final body = request.data as Map<String, dynamic>;
        if (body['phone'] != _registeredPhone ||
            body['password'] != 'secret123') {
          return const TestResponse.json(401, <String, dynamic>{
            'code': 'INVALID_CREDENTIALS',
            'message': '账号或密码错误',
          });
        }
        if (!_enabled) {
          return const TestResponse.json(403, <String, dynamic>{
            'code': 'ACCOUNT_DISABLED',
            'message': '账号已禁用',
          });
        }
        return _createSession(body['deviceId'] as String, 200);
      case ('GET', '/api/news'):
        return _protectedResponse(request);
      case ('POST', '/api/auth/logout'):
        return _logout(request);
      default:
        return const TestResponse.json(404, <String, dynamic>{
          'code': 'NOT_FOUND',
        });
    }
  }

  void enableUser() {
    _enabled = true;
  }

  TestResponse _createSession(String deviceId, int statusCode) {
    final previousToken = _activeToken;
    if (previousToken != null) {
      _tokenStates[previousToken] = MemoryTokenState.replaced;
    }
    _activeToken = 'token-$deviceId';
    _tokenStates[_activeToken!] = MemoryTokenState.active;
    return TestResponse.json(statusCode, <String, dynamic>{
      'accessToken': _activeToken,
      'user': _user,
      'expiresAt': _expiresAt,
    });
  }

  TestResponse _protectedResponse(RequestOptions request) {
    final token = _bearerToken(request);
    final state = token == null ? null : _tokenStates[token];
    if (state == MemoryTokenState.active && token == _activeToken) {
      return const TestResponse.json(200, <String, dynamic>{'ok': true});
    }
    if (state == MemoryTokenState.replaced) {
      return const TestResponse.json(401, <String, dynamic>{
        'code': 'SESSION_REPLACED',
        'message': '账号已在其他设备登录，请重新登录',
      });
    }
    return const TestResponse.json(401, <String, dynamic>{
      'code': 'UNAUTHENTICATED',
      'message': '未认证',
    });
  }

  TestResponse _logout(RequestOptions request) {
    final token = _bearerToken(request);
    if (token == null ||
        token != _activeToken ||
        _tokenStates[token] != MemoryTokenState.active) {
      return _protectedResponse(request);
    }
    _tokenStates[token] = MemoryTokenState.loggedOut;
    _activeToken = null;
    return const TestResponse.json(200, <String, dynamic>{'status': 'ok'});
  }

  String? _bearerToken(RequestOptions request) {
    final authorization = request.headers['Authorization'];
    if (authorization is! String || !authorization.startsWith('Bearer ')) {
      return null;
    }
    final token = authorization.substring('Bearer '.length).trim();
    return token.isEmpty ? null : token;
  }
}

enum MemoryTokenState { active, replaced, loggedOut }

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
