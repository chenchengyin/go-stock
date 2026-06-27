import 'package:flutter/foundation.dart';

import '../../../../shared/view_state.dart';
import '../../data/auth_repository.dart';
import '../../domain/auth_models.dart';

class AuthViewModel extends ChangeNotifier {
  AuthViewModel(this._repository);

  final AuthRepository _repository;

  ViewState state = const ViewState();
  AppUser? user;
  String? accessToken;

  bool get isLoggedIn => user != null && accessToken != null && accessToken!.isNotEmpty;

  Future<void> restore() async {
    state = const ViewState(status: ViewStatus.loading);
    notifyListeners();
    try {
      final session = await _repository.restoreSession();
      user = session?.user;
      accessToken = session?.accessToken;
      state = const ViewState(status: ViewStatus.ready);
    } catch (error) {
      state = ViewState(status: ViewStatus.error, message: error.toString());
    }
    notifyListeners();
  }

  Future<void> login({required String phone, required String password}) async {
    await _authenticate(() => _repository.login(phone: phone, password: password));
  }

  Future<void> register({required String phone, required String password, required String nickname}) async {
    await _authenticate(() => _repository.register(phone: phone, password: password, nickname: nickname));
  }

  Future<void> updateNickname(String nickname) async {
    state = const ViewState(status: ViewStatus.loading);
    notifyListeners();
    try {
      user = await _repository.updateNickname(nickname);
      state = const ViewState(status: ViewStatus.ready);
    } catch (error) {
      state = ViewState(status: ViewStatus.error, message: error.toString());
    }
    notifyListeners();
  }

  Future<void> logout() async {
    await _repository.logout();
    user = null;
    accessToken = null;
    state = const ViewState(status: ViewStatus.ready);
    notifyListeners();
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
      state = ViewState(status: ViewStatus.error, message: error.toString());
    }
    notifyListeners();
  }
}
