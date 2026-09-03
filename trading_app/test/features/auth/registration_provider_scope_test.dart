import 'dart:convert';
import 'dart:typed_data';

import 'package:dio/dio.dart';
import 'package:flutter/cupertino.dart';
import 'package:flutter/material.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:provider/provider.dart';
import 'package:shared_preferences/shared_preferences.dart';
import 'package:trading_app/core/network/api_client.dart';
import 'package:trading_app/features/auth/data/auth_repository.dart';
import 'package:trading_app/features/auth/domain/auth_models.dart';
import 'package:trading_app/features/auth/presentation/auth_gate.dart';
import 'package:trading_app/features/auth/presentation/auth_view_model.dart';
import 'package:trading_app/features/auth/presentation/login_page.dart';
import 'package:trading_app/features/auth/presentation/register_page.dart';

void main() {
  TestWidgetsFlutterBinding.ensureInitialized();

  tearDown(resetApiClientForTesting);

  testWidgets(
    'successful registration stays unauthenticated until an admin enables it',
    (tester) async {
      SharedPreferences.setMockInitialValues(<String, Object>{});
      final auth = AuthViewModel(_RegisteringAuthRepository());
      resetApiClientForTesting();
      final dio = createApiClient()
        ..httpClientAdapter = _EmptyResponseAdapter();

      await auth.restore();
      await tester.pumpWidget(
        MultiProvider(
          providers: [
            ChangeNotifierProvider<AuthViewModel>.value(value: auth),
            Provider<Dio>.value(value: dio),
          ],
          child: const MaterialApp(home: AuthGate()),
        ),
      );
      await tester.pump();

      await tester.tap(find.widgetWithText(CupertinoDialogAction, '确定'));
      await tester.pumpAndSettle();
      await tester.tap(find.widgetWithText(CupertinoButton, '还没有账号？去注册'));
      await tester.pumpAndSettle();

      expect(find.byType(RegisterPage), findsOneWidget);
      final fields = find.byType(CupertinoTextField);
      await tester.enterText(fields.at(0), '13900000000');
      await tester.enterText(fields.at(1), 'secret123');
      await tester.enterText(fields.at(2), 'Smoke User');
      await tester.pump();

      await tester.tap(find.widgetWithText(CupertinoButton, '注册'));
      await tester.pumpAndSettle();

      expect(tester.takeException(), isNull);
      expect(find.byType(RegisterPage), findsOneWidget);
      expect(find.text('注册成功，请等待管理员启用后再登录'), findsOneWidget);
      expect(find.byType(LoginPage), findsNothing);
      expect(auth.isLoggedIn, isFalse);
      await tester.tap(find.widgetWithText(CupertinoDialogAction, '确定'));
      await tester.pumpAndSettle();
      expect(find.byType(RegisterPage), findsNothing);
      expect(find.byType(LoginPage), findsOneWidget);
      expect(find.byType(Scaffold), findsOneWidget);

      await tester.pumpWidget(const SizedBox.shrink());
      auth.dispose();
    },
  );
}

class _RegisteringAuthRepository implements AuthRepository {
  static final _registration = RegistrationResult(
    user: AppUser(
      id: 'registration-user',
      phone: '13900000000',
      nickname: 'Smoke User',
      role: 'user',
    ),
    status: 'disabled',
    message: '注册成功，请等待管理员启用后再登录',
  );
  static final _session = AuthSession(
    accessToken: 'registration-token',
    user: _registration.user,
    expiresAt: DateTime.utc(2026, 9, 30),
  );

  @override
  Future<AuthSession?> restoreSession() async => null;

  @override
  Future<AuthSession> login({
    required String phone,
    required String password,
  }) async => _session;

  @override
  Future<RegistrationResult> register({
    required String phone,
    required String password,
    required String nickname,
  }) async => _registration;

  @override
  Future<AppUser> updateNickname(String nickname) async => _session.user;

  @override
  Future<void> logout() async {}
}

class _EmptyResponseAdapter implements HttpClientAdapter {
  @override
  Future<ResponseBody> fetch(
    RequestOptions options,
    Stream<Uint8List>? requestStream,
    Future<void>? cancelFuture,
  ) async {
    return ResponseBody.fromString(
      jsonEncode(<dynamic>[]),
      200,
      headers: <String, List<String>>{
        Headers.contentTypeHeader: <String>[Headers.jsonContentType],
      },
    );
  }

  @override
  void close({bool force = false}) {}
}
