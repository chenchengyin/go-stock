import 'dart:async';

import 'package:flutter/cupertino.dart';
import 'package:flutter/material.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:provider/provider.dart';
import 'package:trading_app/core/theme/app_colors.dart';
import 'package:trading_app/core/theme/theme_manager.dart';
import 'package:trading_app/features/auth/data/auth_repository.dart';
import 'package:trading_app/features/auth/data/session_controller.dart';
import 'package:trading_app/features/auth/domain/auth_models.dart';
import 'package:trading_app/features/auth/presentation/auth_view_model.dart';
import 'package:trading_app/features/auth/presentation/login_page.dart';
import 'package:trading_app/features/auth/presentation/register_page.dart';
import 'package:trading_app/features/profile/presentation/edit_profile_page.dart';
import 'package:trading_app/shared/view_state.dart';

const _user = AppUser(
  id: 'user-1',
  phone: '13800000000',
  nickname: 'Alice',
  role: 'user',
);

final _session = AuthSession(
  accessToken: 'access-token',
  user: _user,
  expiresAt: DateTime.utc(2026, 9, 29),
);

void main() {
  group('AuthViewModel', () {
    test('login stores the authenticated user and access token', () async {
      final auth = AuthViewModel(
        _FakeAuthRepository(
          loginAction: ({required phone, required password}) async => _session,
        ),
      );

      await auth.login(phone: _user.phone, password: 'secret123');

      expect(auth.user, _user);
      expect(auth.accessToken, 'access-token');
      expect(auth.isLoggedIn, isTrue);
      expect(auth.state.status, ViewStatus.ready);
    });

    test('registration keeps the returned session logged in', () async {
      final auth = AuthViewModel(
        _FakeAuthRepository(
          registerAction:
              ({required phone, required password, required nickname}) async =>
                  _session,
        ),
      );

      await auth.register(
        phone: _user.phone,
        password: 'secret123',
        nickname: _user.nickname,
      );

      expect(auth.user, _user);
      expect(auth.accessToken, 'access-token');
      expect(auth.isLoggedIn, isTrue);
    });

    test('registration displays the backend ACCOUNT_EXISTS message', () async {
      final auth = AuthViewModel(
        _FakeAuthRepository(
          registerAction:
              ({required phone, required password, required nickname}) async =>
                  throw const AuthApiException(
                    code: 'ACCOUNT_EXISTS',
                    message: '该手机号已注册',
                    statusCode: 409,
                  ),
        ),
      );

      await auth.register(
        phone: _user.phone,
        password: 'secret123',
        nickname: _user.nickname,
      );

      expect(auth.isLoggedIn, isFalse);
      expect(auth.state.status, ViewStatus.error);
      expect(auth.state.message, '该手机号已注册');
    });

    test(
      'SESSION_REPLACED clears auth and records a readable reason',
      () async {
        final controller = _FakeSessionController();
        final auth = AuthViewModel(
          _FakeAuthRepository(
            loginAction: ({required phone, required password}) async =>
                _session,
          ),
          sessionController: controller,
        );

        await auth.login(phone: _user.phone, password: 'secret123');
        await controller.clear(reason: SessionInvalidationReason.replaced);

        expect(auth.user, isNull);
        expect(auth.accessToken, isNull);
        expect(auth.isLoggedIn, isFalse);
        expect(auth.state.message, '账号已在其他设备登录，请重新登录');
        auth.dispose();
      },
    );

    test(
      'logout clears local auth even when the remote request fails',
      () async {
        final auth = AuthViewModel(
          _FakeAuthRepository(
            loginAction: ({required phone, required password}) async =>
                _session,
            logoutAction: () async => throw const AuthApiException(
              code: 'NETWORK_ERROR',
              message: '网络请求失败，请稍后重试',
              statusCode: null,
            ),
          ),
        );
        await auth.login(phone: _user.phone, password: 'secret123');

        await auth.logout();

        expect(auth.user, isNull);
        expect(auth.accessToken, isNull);
        expect(auth.isLoggedIn, isFalse);
        expect(auth.state.status, ViewStatus.ready);
      },
    );

    test('profile success updates the local user', () async {
      const updated = AppUser(
        id: 'user-1',
        phone: '13800000000',
        nickname: 'Bob',
        role: 'user',
      );
      final auth = AuthViewModel(
        _FakeAuthRepository(
          loginAction: ({required phone, required password}) async => _session,
          updateNicknameAction: (_) async => updated,
        ),
      );
      await auth.login(phone: _user.phone, password: 'secret123');

      final updatedSuccessfully = await auth.updateNickname('Bob');

      expect(updatedSuccessfully, isTrue);
      expect(auth.user, updated);
      expect(auth.state.status, ViewStatus.ready);
      expect(auth.state.message, isEmpty);
    });

    test(
      'profile failure keeps the current user and backend message',
      () async {
        final auth = AuthViewModel(
          _FakeAuthRepository(
            loginAction: ({required phone, required password}) async =>
                _session,
            updateNicknameAction: (_) async => throw const AuthApiException(
              code: 'INVALID_NICKNAME',
              message: '昵称长度不符合要求',
              statusCode: 400,
            ),
          ),
        );
        await auth.login(phone: _user.phone, password: 'secret123');

        final updatedSuccessfully = await auth.updateNickname('x');

        expect(updatedSuccessfully, isFalse);
        expect(auth.user, _user);
        expect(auth.isLoggedIn, isTrue);
        expect(auth.state.status, ViewStatus.error);
        expect(auth.state.message, '昵称长度不符合要求');
      },
    );
  });

  testWidgets(
    'login fields start empty and submit enables only at account and password limits',
    (tester) async {
      final auth = AuthViewModel(_FakeAuthRepository());
      final theme = ThemeManager();

      await tester.pumpWidget(
        AppColorsWidget(
          colors: theme.colors,
          variant: theme.variant,
          child: ChangeNotifierProvider<AuthViewModel>.value(
            value: auth,
            child: const MaterialApp(home: LoginPage()),
          ),
        ),
      );

      final fields = tester.widgetList<CupertinoTextField>(
        find.byType(CupertinoTextField),
      );
      expect(fields.map((field) => field.controller!.text), <String>['', '']);
      expect(
        tester
            .widget<CupertinoButton>(find.widgetWithText(CupertinoButton, '登录'))
            .onPressed,
        isNull,
      );

      await tester.pump();
      await tester.tap(find.widgetWithText(CupertinoDialogAction, '确定'));
      await tester.pumpAndSettle();

      await tester.enterText(find.byType(CupertinoTextField).first, '12345');
      await tester.enterText(find.byType(CupertinoTextField).last, '123456');
      await tester.pump();

      final updatedFields = tester.widgetList<CupertinoTextField>(
        find.byType(CupertinoTextField),
      );
      expect(updatedFields.map((field) => field.controller!.text), <String>[
        '12345',
        '123456',
      ]);

      expect(
        tester
            .widget<CupertinoButton>(find.widgetWithText(CupertinoButton, '登录'))
            .onPressed,
        isNotNull,
      );

      await tester.pumpWidget(const SizedBox.shrink());
      auth.dispose();
      AppColors.applyLight();
    },
  );

  testWidgets(
    'registration validates limits and displays the backend message',
    (tester) async {
      final auth = AuthViewModel(
        _FakeAuthRepository(
          registerAction:
              ({required phone, required password, required nickname}) async =>
                  throw const AuthApiException(
                    code: 'ACCOUNT_EXISTS',
                    message: '该手机号已注册',
                    statusCode: 409,
                  ),
        ),
      );
      final theme = ThemeManager();

      await tester.pumpWidget(
        AppColorsWidget(
          colors: theme.colors,
          variant: theme.variant,
          child: ChangeNotifierProvider<AuthViewModel>.value(
            value: auth,
            child: const MaterialApp(home: RegisterPage()),
          ),
        ),
      );

      expect(
        tester
            .widget<CupertinoButton>(find.widgetWithText(CupertinoButton, '注册'))
            .onPressed,
        isNull,
      );

      final fields = find.byType(CupertinoTextField);
      await tester.enterText(fields.at(0), '12345');
      await tester.enterText(fields.at(1), '123456');
      await tester.enterText(fields.at(2), 'Alice');
      await tester.pump();

      expect(
        tester
            .widget<CupertinoButton>(find.widgetWithText(CupertinoButton, '注册'))
            .onPressed,
        isNotNull,
      );

      await tester.tap(find.widgetWithText(CupertinoButton, '注册'));
      await tester.pump();

      expect(find.text('该手机号已注册'), findsOneWidget);
      expect(find.byType(RegisterPage), findsOneWidget);

      await tester.pumpWidget(const SizedBox.shrink());
      auth.dispose();
      AppColors.applyLight();
    },
  );

  testWidgets('profile update failure keeps the edit page open', (
    tester,
  ) async {
    final auth = AuthViewModel(
      _FakeAuthRepository(
        loginAction: ({required phone, required password}) async => _session,
        updateNicknameAction: (_) async => throw const AuthApiException(
          code: 'INVALID_NICKNAME',
          message: '昵称长度不符合要求',
          statusCode: 400,
        ),
      ),
    );
    await auth.login(phone: _user.phone, password: 'secret123');
    final theme = ThemeManager();
    final navigatorKey = GlobalKey<NavigatorState>();

    await tester.pumpWidget(
      AppColorsWidget(
        colors: theme.colors,
        variant: theme.variant,
        child: ChangeNotifierProvider<AuthViewModel>.value(
          value: auth,
          child: MaterialApp(
            navigatorKey: navigatorKey,
            home: const SizedBox.shrink(),
          ),
        ),
      ),
    );
    unawaited(
      navigatorKey.currentState!.push(
        CupertinoPageRoute<void>(builder: (_) => const EditProfilePage()),
      ),
    );
    await tester.pumpAndSettle();

    await tester.enterText(find.byType(CupertinoTextField), 'x');
    await tester.tap(find.widgetWithText(CupertinoButton, '保存资料'));
    await tester.pumpAndSettle();

    expect(find.byType(EditProfilePage), findsOneWidget);
    expect(find.text('昵称长度不符合要求'), findsOneWidget);

    await tester.pumpWidget(const SizedBox.shrink());
    auth.dispose();
    AppColors.applyLight();
  });
}

typedef _LoginAction =
    Future<AuthSession> Function({
      required String phone,
      required String password,
    });
typedef _RegisterAction =
    Future<AuthSession> Function({
      required String phone,
      required String password,
      required String nickname,
    });

class _FakeAuthRepository implements AuthRepository {
  _FakeAuthRepository({
    this.loginAction,
    this.registerAction,
    this.updateNicknameAction,
    this.logoutAction,
  });

  final _LoginAction? loginAction;
  final _RegisterAction? registerAction;
  final Future<AppUser> Function(String nickname)? updateNicknameAction;
  final Future<void> Function()? logoutAction;

  @override
  Future<AuthSession?> restoreSession() async => null;

  @override
  Future<AuthSession> login({required String phone, required String password}) {
    return loginAction?.call(phone: phone, password: password) ??
        Future<AuthSession>.error(UnimplementedError());
  }

  @override
  Future<AuthSession> register({
    required String phone,
    required String password,
    required String nickname,
  }) {
    return registerAction?.call(
          phone: phone,
          password: password,
          nickname: nickname,
        ) ??
        Future<AuthSession>.error(UnimplementedError());
  }

  @override
  Future<AppUser> updateNickname(String nickname) {
    return updateNicknameAction?.call(nickname) ??
        Future<AppUser>.error(UnimplementedError());
  }

  @override
  Future<void> logout() => logoutAction?.call() ?? Future<void>.value();
}

class _FakeSessionController extends SessionController {
  SessionInvalidationReason? _reason;

  @override
  bool get isInvalidating => false;

  @override
  SessionInvalidationReason? get lastInvalidationReason => _reason;

  @override
  Future<void> clear({SessionInvalidationReason? reason}) async {
    _reason = reason;
    notifyListeners();
  }

  @override
  Future<String?> readToken() async => null;

  @override
  Future<void> save(AuthSession session) async {
    _reason = null;
  }
}
