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

  factory AppUser.fromJson(Map<String, dynamic> json) {
    return AppUser(
      id: json['id'] as String,
      phone: json['phone'] as String,
      nickname: json['nickname'] as String,
      role: json['role'] as String,
    );
  }

  Map<String, dynamic> toJson() {
    return <String, dynamic>{
      'id': id,
      'phone': phone,
      'nickname': nickname,
      'role': role,
    };
  }

  AppUser copyWith({String? nickname}) {
    return AppUser(
      id: id,
      phone: phone,
      nickname: nickname ?? this.nickname,
      role: role,
    );
  }
}

class RegistrationResult {
  const RegistrationResult({
    required this.user,
    required this.status,
    required this.message,
  });

  final AppUser user;
  final String status;
  final String message;

  factory RegistrationResult.fromJson(Map<String, dynamic> json) {
    return RegistrationResult(
      user: AppUser.fromJson(json['user'] as Map<String, dynamic>),
      status: json['status'] as String,
      message: json['message'] as String,
    );
  }
}

class AuthSession {
  const AuthSession({
    required this.accessToken,
    required this.user,
    required this.expiresAt,
  });

  final String accessToken;
  final AppUser user;
  final DateTime expiresAt;

  factory AuthSession.fromJson(Map<String, dynamic> json) {
    return AuthSession(
      accessToken: json['accessToken'] as String,
      user: AppUser.fromJson(json['user'] as Map<String, dynamic>),
      expiresAt: DateTime.parse(json['expiresAt'] as String),
    );
  }

  Map<String, dynamic> toJson() {
    return <String, dynamic>{
      'accessToken': accessToken,
      'user': user.toJson(),
      'expiresAt': expiresAt.toUtc().toIso8601String(),
    };
  }
}
