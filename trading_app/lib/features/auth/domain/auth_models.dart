class AppUser {
  const AppUser({
    required this.id,
    required this.phone,
    required this.nickname,
    required this.role,
  });

  final String id;
  final String phone;
  final String nickname;
  final String role;

  AppUser copyWith({String? nickname}) {
    return AppUser(
      id: id,
      phone: phone,
      nickname: nickname ?? this.nickname,
      role: role,
    );
  }
}

class AuthSession {
  const AuthSession({required this.accessToken, required this.user});

  final String accessToken;
  final AppUser user;
}
