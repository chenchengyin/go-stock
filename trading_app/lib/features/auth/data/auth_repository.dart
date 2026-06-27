import '../../../core/storage/local_cache.dart';
import '../domain/auth_models.dart';

abstract class AuthRepository {
  Future<AuthSession?> restoreSession();
  Future<AuthSession> login({required String phone, required String password});
  Future<AuthSession> register({required String phone, required String password, required String nickname});
  Future<AppUser> updateNickname(String nickname);
  Future<void> logout();
}

class MockAuthRepository implements AuthRepository {
  MockAuthRepository(this._cache);

  final LocalCache _cache;
  AuthSession? _session;

  @override
  Future<AuthSession?> restoreSession() async {
    final token = await _cache.getString('auth:token');
    final phone = await _cache.getString('auth:phone');
    final nickname = await _cache.getString('auth:nickname');
    if (token == null || phone == null || nickname == null) return null;
    _session = AuthSession(
      accessToken: token,
      user: AppUser(id: 'local-user', phone: phone, nickname: nickname, role: 'user'),
    );
    return _session;
  }

  @override
  Future<AuthSession> login({required String phone, required String password}) async {
    _validate(phone, password);
    return _saveSession(phone: phone, nickname: phone);
  }

  @override
  Future<AuthSession> register({required String phone, required String password, required String nickname}) async {
    _validate(phone, password);
    return _saveSession(phone: phone, nickname: nickname.trim().isEmpty ? phone : nickname.trim());
  }

  @override
  Future<AppUser> updateNickname(String nickname) async {
    final current = _session;
    if (current == null) throw StateError('请先登录');
    final nextUser = current.user.copyWith(nickname: nickname.trim());
    _session = AuthSession(accessToken: current.accessToken, user: nextUser);
    await _cache.setString('auth:nickname', nextUser.nickname);
    return nextUser;
  }

  @override
  Future<void> logout() async {
    _session = null;
    await _cache.setString('auth:token', '');
    await _cache.setString('auth:phone', '');
    await _cache.setString('auth:nickname', '');
  }

  void _validate(String phone, String password) {
    if (phone.trim().length < 5) throw ArgumentError('请输入正确的手机号/账号');
    if (password.length < 6) throw ArgumentError('密码至少 6 位');
  }

  Future<AuthSession> _saveSession({required String phone, required String nickname}) async {
    final session = AuthSession(
      accessToken: 'mock-token-${DateTime.now().millisecondsSinceEpoch}',
      user: AppUser(id: 'local-user', phone: phone.trim(), nickname: nickname, role: 'user'),
    );
    _session = session;
    await _cache.setString('auth:token', session.accessToken);
    await _cache.setString('auth:phone', session.user.phone);
    await _cache.setString('auth:nickname', session.user.nickname);
    return session;
  }
}
