import 'package:flutter/foundation.dart';

import '../../../shared/view_state.dart';
import '../data/auth_repository.dart';
import '../data/session_controller.dart';
import '../domain/auth_models.dart';

class AuthViewModel extends ChangeNotifier {
  AuthViewModel(this._repository, {SessionController? sessionController})
    : _sessionController = sessionController {
    _sessionController?.addListener(_handleSessionInvalidation);
  }

  final AuthRepository _repository;
  final SessionController? _sessionController;

  ViewState state = const ViewState();
  AppUser? user;
  String? accessToken;

  bool get isLoggedIn =>
      user != null && accessToken != null && accessToken!.isNotEmpty;

  Future<void> restore() async {
    state = const ViewState(status: ViewStatus.loading);
    notifyListeners();
    try {
      final session = await _repository.restoreSession();
      user = session?.user;
      accessToken = session?.accessToken;
      state = const ViewState(status: ViewStatus.ready);
    } catch (error) {
      user = null;
      accessToken = null;
      final reason = _sessionController?.lastInvalidationReason;
      state = ViewState(
        status: reason == null ? ViewStatus.error : ViewStatus.ready,
        message: reason == null ? _messageForError(error) : _messageFor(reason),
      );
    }
    notifyListeners();
  }

  Future<void> login({required String phone, required String password}) async {
    await _authenticate(
      () => _repository.login(phone: phone, password: password),
    );
  }

  Future<RegistrationResult?> register({
    required String phone,
    required String password,
    required String nickname,
  }) async {
    state = const ViewState(status: ViewStatus.loading);
    notifyListeners();
    try {
      final result = await _repository.register(
        phone: phone,
        password: password,
        nickname: nickname,
      );
      state = const ViewState(status: ViewStatus.ready);
      notifyListeners();
      return result;
    } catch (error) {
      state = ViewState(
        status: ViewStatus.error,
        message: _messageForError(error),
      );
      notifyListeners();
      return null;
    }
  }

  Future<bool> updateNickname(String nickname) async {
    state = const ViewState(status: ViewStatus.loading);
    notifyListeners();
    try {
      user = await _repository.updateNickname(nickname);
      state = const ViewState(status: ViewStatus.ready);
      notifyListeners();
      return true;
    } catch (error) {
      final reason = _sessionController?.lastInvalidationReason;
      state = reason != null && !isLoggedIn
          ? ViewState(status: ViewStatus.ready, message: _messageFor(reason))
          : ViewState(
              status: ViewStatus.error,
              message: _messageForError(error),
            );
      notifyListeners();
      return false;
    }
  }

  Future<void> logout() async {
    state = const ViewState(status: ViewStatus.loading);
    notifyListeners();
    try {
      await _repository.logout();
    } catch (_) {
      // Remote logout is best effort. Local auth must always be removed.
    } finally {
      user = null;
      accessToken = null;
      state = const ViewState(status: ViewStatus.ready);
      notifyListeners();
    }
  }

  Future<void> _authenticate(Future<AuthSession> Function() action) async {
    state = const ViewState(status: ViewStatus.loading);
    notifyListeners();
    try {
      final session = await action();
      user = session.user;
      accessToken = session.accessToken;
      state = const ViewState(status: ViewStatus.ready);
    } catch (error) {
      state = ViewState(
        status: ViewStatus.error,
        message: _messageForError(error),
      );
    }
    notifyListeners();
  }

  void _handleSessionInvalidation() {
    final reason = _sessionController?.lastInvalidationReason;
    if (reason == null) {
      return;
    }
    user = null;
    accessToken = null;
    state = ViewState(status: ViewStatus.ready, message: _messageFor(reason));
    notifyListeners();
  }

  String _messageFor(SessionInvalidationReason reason) {
    return switch (reason) {
      SessionInvalidationReason.replaced => '账号已在其他设备登录，请重新登录',
      SessionInvalidationReason.expired ||
      SessionInvalidationReason.revoked => '登录状态已失效，请重新登录',
    };
  }

  String _messageForError(Object error) {
    return error is AuthApiException ? error.message : error.toString();
  }

  @override
  void dispose() {
    _sessionController?.removeListener(_handleSessionInvalidation);
    super.dispose();
  }
}
