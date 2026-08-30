import 'dart:async';
import 'dart:convert';
import 'dart:typed_data';

import 'package:dio/dio.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:shared_preferences/shared_preferences.dart';
import 'package:trading_app/features/auth/data/auth_repository.dart';
import 'package:trading_app/features/auth/data/auth_storage.dart';
import 'package:trading_app/features/auth/domain/auth_models.dart';

const _deviceId = '0123456789abcdef0123456789abcdef';
const _accessToken = 'test-access-token';
const _expiresAt = '2026-09-29T12:30:00Z';

const _userJson = <String, dynamic>{
  'id': 'user-1',
  'phone': '13800000000',
  'nickname': 'Alice',
  'role': 'user',
};

Map<String, dynamic> _sessionJson() => <String, dynamic>{
  'accessToken': _accessToken,
  'user': _userJson,
  'expiresAt': _expiresAt,
};

void main() {
  TestWidgetsFlutterBinding.ensureInitialized();

  setUp(() {
    SharedPreferences.setMockInitialValues(<String, Object>{});
  });

  group('auth models', () {
    test('parse and serialize user and session including expiresAt', () {
      final session = AuthSession.fromJson(_sessionJson());

      expect(session.accessToken, _accessToken);
      expect(session.user.id, 'user-1');
      expect(session.user.phone, '13800000000');
      expect(session.user.nickname, 'Alice');
      expect(session.user.role, 'user');
      expect(session.expiresAt, DateTime.utc(2026, 9, 29, 12, 30));
      expect(session.toJson(), <String, dynamic>{
        'accessToken': _accessToken,
        'user': _userJson,
        'expiresAt': '2026-09-29T12:30:00.000Z',
      });
    });
  });

  group('SharedPreferencesAuthStorage', () {
    test('device id is generated once and reused', () async {
      final storage = SharedPreferencesAuthStorage();

      final first = await storage.getOrCreateDeviceId();
      final second = await storage.getOrCreateDeviceId();

      expect(first, matches(RegExp(r'^[0-9a-f]{32}$')));
      expect(second, first);
      final prefs = await SharedPreferences.getInstance();
      expect(prefs.getString('device:id'), first);
    });

    test(
      'clearAuth removes only auth values and preserves device id',
      () async {
        SharedPreferences.setMockInitialValues(<String, Object>{
          'auth:token': _accessToken,
          'auth:user': jsonEncode(_userJson),
          'device:id': _deviceId,
          'cache:news': 'cached',
        });
        final storage = SharedPreferencesAuthStorage();

        await storage.clearAuth();

        expect(await storage.read('auth:token'), isNull);
        expect(await storage.read('auth:user'), isNull);
        expect(await storage.read('device:id'), _deviceId);
        expect(await storage.read('cache:news'), 'cached');
      },
    );
  });

  group('ApiAuthRepository', () {
    test(
      'login sends credentials and stable device id then persists session',
      () async {
        final storage = MemoryAuthStorage();
        final adapter = DioTestAdapter((request) {
          expect(request.method, 'POST');
          expect(request.uri.path, '/api/auth/login');
          expect(request.data, <String, dynamic>{
            'phone': '13800000000',
            'password': 'secret123',
            'deviceId': _deviceId,
          });
          return TestResponse.json(200, _sessionJson());
        });
        final repository = ApiAuthRepository(
          dio: _dioWith(adapter),
          storage: storage,
        );

        final session = await repository.login(
          phone: '13800000000',
          password: 'secret123',
        );

        expect(session.accessToken, _accessToken);
        expect(await storage.read('auth:token'), _accessToken);
        expect(jsonDecode((await storage.read('auth:user'))!), _userJson);
        expect(
          DateTime.parse((await storage.read('auth:expiresAt'))!),
          DateTime.utc(2026, 9, 29, 12, 30),
        );
        expect(await storage.getOrCreateDeviceId(), _deviceId);
        expect(adapter.requests, hasLength(1));
      },
    );

    test(
      'register sends nickname and reuses the persistent device id',
      () async {
        final storage = MemoryAuthStorage(<String, String>{
          'device:id': _deviceId,
        });
        final adapter = DioTestAdapter((request) {
          expect(request.method, 'POST');
          expect(request.uri.path, '/api/auth/register');
          expect(request.data, <String, dynamic>{
            'phone': '13800000000',
            'password': 'secret123',
            'nickname': 'Alice',
            'deviceId': _deviceId,
          });
          return TestResponse.json(201, _sessionJson());
        });
        final repository = ApiAuthRepository(
          dio: _dioWith(adapter),
          storage: storage,
        );

        final session = await repository.register(
          phone: '13800000000',
          password: 'secret123',
          nickname: 'Alice',
        );

        expect(session.user.nickname, 'Alice');
        expect(await storage.read('device:id'), _deviceId);
        expect(adapter.requests, hasLength(1));
      },
    );

    test(
      'restoreSession validates a stored token with me and refreshes user data',
      () async {
        final storage = MemoryAuthStorage(<String, String>{
          'auth:token': _accessToken,
          'auth:user': jsonEncode(<String, dynamic>{
            ..._userJson,
            'nickname': 'Old',
          }),
          'auth:expiresAt': '2026-09-01T00:00:00Z',
          'device:id': _deviceId,
        });
        final adapter = DioTestAdapter((request) {
          expect(request.method, 'GET');
          expect(request.uri.path, '/api/auth/me');
          return TestResponse.json(200, <String, dynamic>{
            'user': _userJson,
            'expiresAt': _expiresAt,
          });
        });
        final repository = ApiAuthRepository(
          dio: _dioWith(adapter),
          storage: storage,
        );

        final session = await repository.restoreSession();

        expect(session?.accessToken, _accessToken);
        expect(session?.user.nickname, 'Alice');
        expect(session?.expiresAt, DateTime.utc(2026, 9, 29, 12, 30));
        expect(jsonDecode((await storage.read('auth:user'))!), _userJson);
        expect(
          DateTime.parse((await storage.read('auth:expiresAt'))!),
          DateTime.utc(2026, 9, 29, 12, 30),
        );
        expect(adapter.requests, hasLength(1));
      },
    );

    test(
      'restoreSession returns null without a token and skips the network',
      () async {
        final storage = MemoryAuthStorage();
        final adapter = DioTestAdapter((request) {
          fail('unexpected request to ${request.uri}');
        });
        final repository = ApiAuthRepository(
          dio: _dioWith(adapter),
          storage: storage,
        );

        expect(await repository.restoreSession(), isNull);
        expect(adapter.requests, isEmpty);
      },
    );

    test(
      'restoreSession clears auth after API rejection and preserves device id',
      () async {
        final storage = MemoryAuthStorage(<String, String>{
          'auth:token': _accessToken,
          'auth:user': jsonEncode(_userJson),
          'auth:expiresAt': _expiresAt,
          'device:id': _deviceId,
        });
        final adapter = DioTestAdapter(
          (_) => TestResponse.json(401, <String, dynamic>{
            'code': 'SESSION_REPLACED',
            'message': '账号已在其他设备登录，请重新登录',
          }),
        );
        final repository = ApiAuthRepository(
          dio: _dioWith(adapter),
          storage: storage,
        );

        await expectLater(
          repository.restoreSession(),
          throwsA(
            isA<AuthApiException>()
                .having((error) => error.code, 'code', 'SESSION_REPLACED')
                .having((error) => error.message, 'message', '账号已在其他设备登录，请重新登录')
                .having((error) => error.statusCode, 'statusCode', 401),
          ),
        );
        expect(await storage.read('auth:token'), isNull);
        expect(await storage.read('auth:user'), isNull);
        expect(await storage.read('auth:expiresAt'), isNull);
        expect(await storage.read('device:id'), _deviceId);
      },
    );

    test(
      'updateNickname patches profile and persists the returned user',
      () async {
        final storage = MemoryAuthStorage(<String, String>{
          'auth:token': _accessToken,
          'auth:user': jsonEncode(_userJson),
          'auth:expiresAt': _expiresAt,
          'device:id': _deviceId,
        });
        final updatedUser = <String, dynamic>{..._userJson, 'nickname': 'Bob'};
        final adapter = DioTestAdapter((request) {
          expect(request.method, 'PATCH');
          expect(request.uri.path, '/api/auth/profile');
          expect(request.data, <String, dynamic>{'nickname': 'Bob'});
          return TestResponse.json(200, updatedUser);
        });
        final repository = ApiAuthRepository(
          dio: _dioWith(adapter),
          storage: storage,
        );

        final user = await repository.updateNickname('Bob');

        expect(user.nickname, 'Bob');
        expect(jsonDecode((await storage.read('auth:user'))!), updatedUser);
        expect(await storage.read('auth:token'), _accessToken);
        expect(await storage.read('auth:expiresAt'), _expiresAt);
      },
    );

    test('logout clears auth even when the API request fails', () async {
      final storage = MemoryAuthStorage(<String, String>{
        'auth:token': _accessToken,
        'auth:user': jsonEncode(_userJson),
        'auth:expiresAt': _expiresAt,
        'device:id': _deviceId,
      });
      final adapter = DioTestAdapter((request) {
        expect(request.method, 'POST');
        expect(request.uri.path, '/api/auth/logout');
        return TestResponse.json(503, <String, dynamic>{
          'code': 'INTERNAL',
          'message': '服务器开小差了',
        });
      });
      final repository = ApiAuthRepository(
        dio: _dioWith(adapter),
        storage: storage,
      );

      await repository.logout();

      expect(await storage.read('auth:token'), isNull);
      expect(await storage.read('auth:user'), isNull);
      expect(await storage.read('auth:expiresAt'), isNull);
      expect(await storage.read('device:id'), _deviceId);
      expect(adapter.requests, hasLength(1));
    });

    test('login converts server errors to AuthApiException', () async {
      final storage = MemoryAuthStorage();
      final adapter = DioTestAdapter(
        (_) => TestResponse.json(401, <String, dynamic>{
          'code': 'INVALID_CREDENTIALS',
          'message': '账号或密码错误',
        }),
      );
      final repository = ApiAuthRepository(
        dio: _dioWith(adapter),
        storage: storage,
      );

      await expectLater(
        repository.login(phone: '13800000000', password: 'wrong-password'),
        throwsA(
          isA<AuthApiException>()
              .having((error) => error.code, 'code', 'INVALID_CREDENTIALS')
              .having((error) => error.message, 'message', '账号或密码错误')
              .having((error) => error.statusCode, 'statusCode', 401)
              .having((error) => error.toString(), 'display text', '账号或密码错误'),
        ),
      );
    });
  });
}

class MemoryAuthStorage implements AuthStorage {
  MemoryAuthStorage([Map<String, String>? values])
    : _values = <String, String>{...?values};

  final Map<String, String> _values;

  @override
  Future<void> clearAuth() async {
    _values.removeWhere((key, _) => key.startsWith('auth:'));
  }

  @override
  Future<String> getOrCreateDeviceId() async {
    return _values.putIfAbsent('device:id', () => _deviceId);
  }

  @override
  Future<String?> read(String key) async => _values[key];

  @override
  Future<void> write(String key, String value) async {
    _values[key] = value;
  }
}

class TestResponse {
  const TestResponse.json(this.statusCode, this.data);

  final int statusCode;
  final Object? data;
}

class DioTestAdapter implements HttpClientAdapter {
  DioTestAdapter(this._handler);

  final FutureOr<TestResponse> Function(RequestOptions request) _handler;
  final List<RequestOptions> requests = <RequestOptions>[];

  @override
  Future<ResponseBody> fetch(
    RequestOptions options,
    Stream<Uint8List>? requestStream,
    Future<void>? cancelFuture,
  ) async {
    requests.add(options);
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

Dio _dioWith(HttpClientAdapter adapter) {
  final dio = Dio(BaseOptions(baseUrl: 'https://auth.test'));
  dio.httpClientAdapter = adapter;
  return dio;
}
