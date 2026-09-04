import 'dart:async';
import 'dart:convert';
import 'dart:typed_data';

import 'package:dio/dio.dart';
import 'package:flutter/cupertino.dart';
import 'package:flutter/material.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:provider/provider.dart';
import 'package:shared_preferences/shared_preferences.dart';
import 'package:trading_app/app/app_shell.dart';
import 'package:trading_app/core/network/api_client.dart';
import 'package:trading_app/core/theme/app_colors.dart';
import 'package:trading_app/core/theme/theme_manager.dart';
import 'package:trading_app/features/auth/data/auth_repository.dart';
import 'package:trading_app/features/auth/data/auth_storage.dart';
import 'package:trading_app/features/auth/data/session_controller.dart';
import 'package:trading_app/features/auth/domain/auth_models.dart';
import 'package:trading_app/features/auth/presentation/auth_gate.dart';
import 'package:trading_app/features/auth/presentation/auth_view_model.dart';
import 'package:trading_app/features/auth/presentation/login_page.dart';
import 'package:trading_app/features/permissions/domain/module_definition.dart';
import 'package:trading_app/features/permissions/presentation/module_permission_controller.dart';
import 'package:trading_app/features/radar/data/radar_repository.dart';
import 'package:trading_app/features/radar/domain/voice_announcement_view_model.dart';
import 'package:trading_app/features/radar/presentation/radar_list/radar_view_model.dart';
import 'package:trading_app/features/radar/presentation/radar_list/t0_strategy_view_model.dart';

const _token = 'test-access-token';
const _deviceId = '0123456789abcdef0123456789abcdef';

const _user = AppUser(
  id: 'user-1',
  phone: '13800000000',
  nickname: 'Alice',
  role: 'user',
);

final _session = AuthSession(
  accessToken: _token,
  user: _user,
  expiresAt: DateTime.utc(2026, 9, 29, 12, 30),
);

void main() {
  TestWidgetsFlutterBinding.ensureInitialized();

  tearDown(resetApiClientForTesting);

  group('session Dio interceptor', () {
    test('adds the persisted bearer token to protected requests', () async {
      final storage = MemoryAuthStorage(<String, String>{'auth:token': _token});
      final controller = AuthSessionController(storage);
      final adapter = DioTestAdapter((request) {
        expect(request.headers['Authorization'], 'Bearer $_token');
        return const TestResponse.json(200, <String, dynamic>{'ok': true});
      });
      final dio = createApiClient(
        baseUrl: 'https://auth.test',
        sessionController: controller,
      )..httpClientAdapter = adapter;

      await dio.get<dynamic>('/api/follow-list');

      expect(adapter.requests, hasLength(1));
    });

    test('skipAuth requests do not carry a persisted token', () async {
      final storage = MemoryAuthStorage(<String, String>{'auth:token': _token});
      final controller = AuthSessionController(storage);
      final adapter = DioTestAdapter((request) {
        expect(request.headers, isNot(contains('Authorization')));
        return const TestResponse.json(200, <String, dynamic>{'ok': true});
      });
      final dio = createApiClient(
        baseUrl: 'https://auth.test',
        sessionController: controller,
      )..httpClientAdapter = adapter;

      await dio.post<dynamic>(
        '/api/auth/login',
        options: Options(extra: <String, dynamic>{'skipAuth': true}),
      );

      expect(adapter.requests, hasLength(1));
    });

    test(
      'two concurrent replaced responses clear and notify only once without retry',
      () async {
        final storage = MemoryAuthStorage(<String, String>{
          'auth:token': _token,
          'device:id': _deviceId,
        });
        final controller = AuthSessionController(storage);
        var invalidationNotifications = 0;
        controller.addListener(() {
          if (controller.lastInvalidationReason != null) {
            invalidationNotifications++;
          }
        });

        final bothRequestsStarted = Completer<void>();
        late DioTestAdapter adapter;
        adapter = DioTestAdapter((_) async {
          if (adapter.requests.length == 2) {
            bothRequestsStarted.complete();
          }
          await bothRequestsStarted.future;
          return const TestResponse.json(401, <String, dynamic>{
            'code': 'SESSION_REPLACED',
            'message': '账号已在其他设备登录，请重新登录',
          });
        });
        final dio = createApiClient(
          baseUrl: 'https://auth.test',
          sessionController: controller,
        )..httpClientAdapter = adapter;

        await Future.wait<void>(<Future<void>>[
          _expectDioFailure(dio.get<dynamic>('/api/follow-list')),
          _expectDioFailure(dio.get<dynamic>('/api/news')),
        ]);

        expect(adapter.requests, hasLength(2));
        expect(storage.clearAuthCalls, 1);
        expect(invalidationNotifications, 1);
        expect(
          controller.lastInvalidationReason,
          SessionInvalidationReason.replaced,
        );
        expect(await storage.read('auth:token'), isNull);
        expect(await storage.read('device:id'), _deviceId);
      },
    );

    test(
      'delayed 401 only invalidates the token used by its request',
      () async {
        for (final requestToken in <String?>[_token, null]) {
          resetApiClientForTesting();
          final storage = MemoryAuthStorage(<String, String>{
            if (requestToken != null) 'auth:token': requestToken,
            'device:id': _deviceId,
          });
          final controller = AuthSessionController(storage);
          var invalidationNotifications = 0;
          controller.addListener(() => invalidationNotifications++);
          final requestStarted = Completer<void>();
          final releaseResponse = Completer<void>();
          final adapter = DioTestAdapter((request) async {
            expect(
              request.headers['Authorization'],
              requestToken == null ? isNull : 'Bearer $requestToken',
            );
            requestStarted.complete();
            await releaseResponse.future;
            return const TestResponse.json(401, <String, dynamic>{
              'code': 'SESSION_REPLACED',
              'message': '账号已在其他设备登录，请重新登录',
            });
          });
          final dio = createApiClient(
            baseUrl: 'https://auth.test',
            sessionController: controller,
          )..httpClientAdapter = adapter;

          final staleRequest = _expectDioFailure(dio.get<dynamic>('/api/news'));
          await requestStarted.future;
          await controller.save(
            AuthSession(
              accessToken: 'token-b',
              user: _user,
              expiresAt: DateTime.utc(2026, 10, 30),
            ),
          );
          releaseResponse.complete();
          await staleRequest;

          expect(await storage.read('auth:token'), 'token-b');
          expect(storage.clearAuthCalls, 0);
          expect(controller.lastInvalidationReason, isNull);
          expect(invalidationNotifications, 0);
        }
      },
    );

    test(
      'maps expired and generic 401 codes to invalidation reasons',
      () async {
        for (final testCase in <(String, SessionInvalidationReason)>[
          ('SESSION_EXPIRED', SessionInvalidationReason.expired),
          ('UNAUTHENTICATED', SessionInvalidationReason.revoked),
        ]) {
          resetApiClientForTesting();
          final storage = MemoryAuthStorage(<String, String>{
            'auth:token': _token,
          });
          final controller = AuthSessionController(storage);
          final adapter = DioTestAdapter(
            (_) =>
                TestResponse.json(401, <String, dynamic>{'code': testCase.$1}),
          );
          final dio = createApiClient(
            baseUrl: 'https://auth.test',
            sessionController: controller,
          )..httpClientAdapter = adapter;

          await _expectDioFailure(dio.get<dynamic>('/api/follow-list'));

          expect(controller.lastInvalidationReason, testCase.$2);
        }
      },
    );

    test('403 ACCOUNT_DISABLED clears the current session', () async {
      final storage = MemoryAuthStorage(<String, String>{
        'auth:token': _token,
        'device:id': _deviceId,
      });
      final controller = AuthSessionController(storage);
      final adapter = DioTestAdapter(
        (_) => const TestResponse.json(403, <String, dynamic>{
          'code': 'ACCOUNT_DISABLED',
          'message': '账号已禁用',
        }),
      );
      final dio = createApiClient(
        baseUrl: 'https://auth.test',
        sessionController: controller,
      )..httpClientAdapter = adapter;

      await _expectDioFailure(dio.get<dynamic>('/api/news'));

      expect(storage.clearAuthCalls, 1);
      expect(await storage.read('auth:token'), isNull);
      expect(
        controller.lastInvalidationReason,
        SessionInvalidationReason.revoked,
      );
    });

    test(
      'restore 401 does not clear again after interceptor invalidation',
      () async {
        final storage = MemoryAuthStorage(<String, String>{
          'auth:token': _token,
          'device:id': _deviceId,
        });
        final controller = AuthSessionController(storage);
        final adapter = DioTestAdapter(
          (_) => const TestResponse.json(401, <String, dynamic>{
            'code': 'SESSION_REPLACED',
            'message': '账号已在其他设备登录，请重新登录',
          }),
        );
        final dio = createApiClient(
          baseUrl: 'https://auth.test',
          sessionController: controller,
        )..httpClientAdapter = adapter;
        final repository = ApiAuthRepository(
          dio: dio,
          storage: storage,
          sessionController: controller,
        );

        await expectLater(
          repository.restoreSession(),
          throwsA(isA<AuthApiException>()),
        );

        expect(storage.clearAuthCalls, 1);
        expect(await storage.read('device:id'), _deviceId);
      },
    );
  });

  group('AuthViewModel session invalidation', () {
    test('shows replaced and expired session messages', () async {
      for (final testCase in <(SessionInvalidationReason, String)>[
        (SessionInvalidationReason.replaced, '账号已在其他设备登录，请重新登录'),
        (SessionInvalidationReason.expired, '登录状态已失效，请重新登录'),
      ]) {
        final controller = AuthSessionController(MemoryAuthStorage());
        final auth = AuthViewModel(
          FakeAuthRepository(loginResult: _session),
          sessionController: controller,
        );

        await auth.login(phone: '13800000000', password: 'secret123');
        await controller.clear(reason: testCase.$1);

        expect(auth.isLoggedIn, isFalse);
        expect(auth.state.message, testCase.$2);
        auth.dispose();
      }
    });

    test('profile 401 keeps the session invalidation message', () async {
      final controller = AuthSessionController(MemoryAuthStorage());
      final repository = FakeAuthRepository(
        loginResult: _session,
        updateNicknameAction: (_) async {
          await controller.clear(reason: SessionInvalidationReason.replaced);
          throw const AuthApiException(
            code: 'SESSION_REPLACED',
            message: 'raw server message',
            statusCode: 401,
          );
        },
      );
      final auth = AuthViewModel(repository, sessionController: controller);

      await auth.login(phone: '13800000000', password: 'secret123');
      await auth.updateNickname('Bob');

      expect(auth.isLoggedIn, isFalse);
      expect(auth.state.message, '账号已在其他设备登录，请重新登录');
      auth.dispose();
    });
  });

  group('AuthGate', () {
    testWidgets(
      'root login keeps the home route and lets AuthGate show AppShell',
      (tester) async {
        SharedPreferences.setMockInitialValues(<String, Object>{
          'voice_announcement_asked': true,
        });
        final adapter = DioTestAdapter(
          (_) => const TestResponse.json(200, <String, dynamic>{}),
        );
        final dio = createApiClient()..httpClientAdapter = adapter;
        final controller = AuthSessionController(MemoryAuthStorage());
        final auth = AuthViewModel(
          FakeAuthRepository(loginResult: _session),
          sessionController: controller,
        );
        await auth.restore();
        final radar = RadarViewModel(RadarRepositoryImpl(dio: dio));
        final t0Strategy = T0StrategyViewModel();
        final voice = _TestVoiceAnnouncementViewModel();
        final navigatorObserver = RecordingNavigatorObserver();
        final theme = ThemeManager();

        try {
          await tester.pumpWidget(
            AppColorsWidget(
              colors: theme.colors,
              variant: theme.variant,
              child: MultiProvider(
                providers: [
                  ChangeNotifierProvider<AuthViewModel>.value(value: auth),
                  ChangeNotifierProvider<RadarViewModel>.value(value: radar),
                  ChangeNotifierProvider<T0StrategyViewModel>.value(
                    value: t0Strategy,
                  ),
                  ChangeNotifierProvider<VoiceAnnouncementViewModel>.value(
                    value: voice,
                  ),
                  ChangeNotifierProvider(
                    create: (_) => ModulePermissionController.forTesting(
                      publicModuleDefinitions,
                    ),
                  ),
                ],
                child: MaterialApp(
                  navigatorObservers: <NavigatorObserver>[navigatorObserver],
                  home: const AuthGate(
                    authenticatedBuilder: _buildTestAppShell,
                  ),
                ),
              ),
            ),
          );
          await tester.pump();
          await tester.tap(find.widgetWithText(CupertinoDialogAction, '确定'));
          await tester.pumpAndSettle();
          final popsBeforeLogin = navigatorObserver.popCount;

          await tester.enterText(
            find.byType(CupertinoTextField).first,
            _user.phone,
          );
          await tester.enterText(
            find.byType(CupertinoTextField).last,
            'secret123',
          );
          await tester.pump();
          await tester.tap(find.widgetWithText(CupertinoButton, '登录'));
          await tester.pumpAndSettle();

          expect(find.byType(AppShell), findsOneWidget);
          expect(navigatorObserver.popCount, popsBeforeLogin);
          expect(AppShell.navigatorKey.currentState, isNotNull);
          await tester.pump(const Duration(milliseconds: 500));
        } finally {
          await tester.pumpWidget(const SizedBox.shrink());
          radar.dispose();
          t0Strategy.dispose();
          voice.dispose();
          auth.dispose();
          AppColors.applyLight();
        }
      },
    );

    testWidgets(
      'keeps authenticated content unbuilt while restoring then shows it',
      (tester) async {
        final restore = Completer<AuthSession?>();
        final controller = AuthSessionController(MemoryAuthStorage());
        final auth = AuthViewModel(
          FakeAuthRepository(restoreResult: restore.future),
          sessionController: controller,
        );
        var authenticatedBuilds = 0;

        unawaited(auth.restore());
        await tester.pumpWidget(
          ChangeNotifierProvider<AuthViewModel>.value(
            value: auth,
            child: MaterialApp(
              home: AuthGate(
                authenticatedBuilder: (_) {
                  authenticatedBuilds++;
                  return const Text('authenticated-app');
                },
              ),
            ),
          ),
        );

        expect(find.byType(CircularProgressIndicator), findsOneWidget);
        expect(find.text('authenticated-app'), findsNothing);
        expect(authenticatedBuilds, 0);

        restore.complete(_session);
        await tester.pump();

        expect(find.byType(CircularProgressIndicator), findsNothing);
        expect(find.text('authenticated-app'), findsOneWidget);
        expect(authenticatedBuilds, 1);
      },
    );

    testWidgets('shows LoginPage after restore fails', (tester) async {
      final restore = Completer<AuthSession?>();
      final controller = AuthSessionController(MemoryAuthStorage());
      final auth = AuthViewModel(
        FakeAuthRepository(restoreResult: restore.future),
        sessionController: controller,
      );

      unawaited(auth.restore());
      await tester.pumpWidget(
        ChangeNotifierProvider<AuthViewModel>.value(
          value: auth,
          child: const MaterialApp(home: AuthGate()),
        ),
      );
      restore.completeError(
        const AuthApiException(
          code: 'UNAUTHENTICATED',
          message: '登录状态已失效，请重新登录',
          statusCode: 401,
        ),
      );
      await tester.pump();

      expect(find.byType(LoginPage), findsOneWidget);
      expect(auth.isLoggedIn, isFalse);
    });
  });
}

Widget _buildTestAppShell(BuildContext context) => const AppShell();

class _TestVoiceAnnouncementViewModel extends VoiceAnnouncementViewModel {
  @override
  bool get askedBefore => true;

  // The test does not install the flutter_tts platform channel.
  @override
  // ignore: must_call_super
  void dispose() {}
}

Future<void> _expectDioFailure(Future<dynamic> request) async {
  try {
    await request;
    fail('request unexpectedly succeeded');
  } on DioException {
    // Expected: invalid requests are forwarded without retry.
  }
}

class FakeAuthRepository implements AuthRepository {
  FakeAuthRepository({
    this.restoreResult,
    this.loginResult,
    this.updateNicknameAction,
  });

  final Future<AuthSession?>? restoreResult;
  final AuthSession? loginResult;
  final Future<AppUser> Function(String nickname)? updateNicknameAction;

  @override
  Future<AuthSession?> restoreSession() async =>
      restoreResult == null ? null : await restoreResult;

  @override
  Future<AuthSession> login({
    required String phone,
    required String password,
  }) async {
    return loginResult!;
  }

  @override
  Future<void> logout() async {}

  @override
  Future<RegistrationResult> register({
    required String phone,
    required String password,
    required String nickname,
  }) {
    throw UnimplementedError();
  }

  @override
  Future<AppUser> updateNickname(String nickname) {
    return updateNicknameAction?.call(nickname) ??
        Future<AppUser>.error(UnimplementedError());
  }
}

class MemoryAuthStorage implements AuthStorage {
  MemoryAuthStorage([Map<String, String>? values])
    : _values = <String, String>{...?values};

  final Map<String, String> _values;
  int clearAuthCalls = 0;

  @override
  Future<void> clearAuth() async {
    clearAuthCalls++;
    await Future<void>.delayed(Duration.zero);
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

class RecordingNavigatorObserver extends NavigatorObserver {
  int popCount = 0;

  @override
  void didPop(Route<dynamic> route, Route<dynamic>? previousRoute) {
    popCount++;
    super.didPop(route, previousRoute);
  }
}
