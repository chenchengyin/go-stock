import 'dart:convert';

import 'package:dio/dio.dart';

import 'auth_storage.dart';
import 'session_controller.dart';
import '../domain/auth_models.dart';

abstract class AuthRepository {
  Future<AuthSession?> restoreSession();
  Future<AuthSession> login({required String phone, required String password});
  Future<RegistrationResult> register({
    required String phone,
    required String password,
    required String nickname,
  });
  Future<AppUser> updateNickname(String nickname);
  Future<void> logout();
}

class AuthApiException implements Exception {
  const AuthApiException({
    required this.code,
    required this.message,
    required this.statusCode,
  });

  final String code;
  final String message;
  final int? statusCode;

  factory AuthApiException.fromDio(DioException error) {
    final response = error.response;
    final data = response?.data;
    final json = data is Map<String, dynamic> ? data : null;
    final code = json?['code'];
    final message = json?['message'];

    return AuthApiException(
      code: code is String && code.isNotEmpty
          ? code
          : response == null
          ? 'NETWORK_ERROR'
          : 'REQUEST_FAILED',
      message: message is String && message.isNotEmpty
          ? message
          : '网络请求失败，请稍后重试',
      statusCode: response?.statusCode,
    );
  }

  @override
  String toString() => message;
}

class ApiAuthRepository implements AuthRepository {
  ApiAuthRepository({
    required Dio dio,
    required AuthStorage storage,
    SessionController? sessionController,
  }) : _dio = dio,
       _storage = storage,
       _sessionController = sessionController;

  static const _tokenKey = 'auth:token';
  static const _userKey = 'auth:user';
  static const _expiresAtKey = 'auth:expiresAt';

  final Dio _dio;
  final AuthStorage _storage;
  final SessionController? _sessionController;

  @override
  Future<AuthSession?> restoreSession() async {
    final token =
        await (_sessionController?.readToken() ?? _storage.read(_tokenKey));
    if (token == null || token.isEmpty) {
      return null;
    }

    try {
      final session = await _execute(() async {
        final response = await _dio.get<dynamic>('/api/auth/me');
        final json = _jsonObject(response.data);
        return AuthSession.fromJson(<String, dynamic>{
          ...json,
          'accessToken': token,
        });
      });
      await _persistSession(session);
      return session;
    } catch (_) {
      await _clearSession();
      rethrow;
    }
  }

  @override
  Future<AuthSession> login({
    required String phone,
    required String password,
  }) async {
    final deviceId = await _storage.getOrCreateDeviceId();
    final session = await _execute(() async {
      final response = await _dio.post<dynamic>(
        '/api/auth/login',
        data: <String, dynamic>{
          'phone': phone,
          'password': password,
          'deviceId': deviceId,
        },
        options: Options(extra: <String, dynamic>{'skipAuth': true}),
      );
      return AuthSession.fromJson(_jsonObject(response.data));
    });
    await _persistSession(session);
    return session;
  }

  @override
  Future<RegistrationResult> register({
    required String phone,
    required String password,
    required String nickname,
  }) async {
    return _execute(() async {
      final response = await _dio.post<dynamic>(
        '/api/auth/register',
        data: <String, dynamic>{
          'phone': phone,
          'password': password,
          'nickname': nickname,
        },
        options: Options(extra: <String, dynamic>{'skipAuth': true}),
      );
      return RegistrationResult.fromJson(_jsonObject(response.data));
    });
  }

  @override
  Future<AppUser> updateNickname(String nickname) async {
    final user = await _execute(() async {
      final response = await _dio.patch<dynamic>(
        '/api/auth/profile',
        data: <String, dynamic>{'nickname': nickname},
      );
      return AppUser.fromJson(_jsonObject(response.data));
    });
    await _storage.write(_userKey, jsonEncode(user.toJson()));
    return user;
  }

  @override
  Future<void> logout() async {
    try {
      await _dio.post<dynamic>('/api/auth/logout');
    } catch (_) {
      // Logout is best effort; local auth state must always be removed.
    } finally {
      await _clearSession();
    }
  }

  Future<void> _persistSession(AuthSession session) async {
    final sessionController = _sessionController;
    if (sessionController != null) {
      await sessionController.save(session);
      return;
    }
    await _storage.write(_tokenKey, session.accessToken);
    await _storage.write(_userKey, jsonEncode(session.user.toJson()));
    await _storage.write(
      _expiresAtKey,
      session.expiresAt.toUtc().toIso8601String(),
    );
  }

  Future<void> _clearSession() async {
    final sessionController = _sessionController;
    if (sessionController != null) {
      await sessionController.clear();
      return;
    }
    await _storage.clearAuth();
  }

  Future<T> _execute<T>(Future<T> Function() request) async {
    try {
      return await request();
    } on AuthApiException {
      rethrow;
    } on DioException catch (error) {
      throw AuthApiException.fromDio(error);
    } catch (_) {
      throw const AuthApiException(
        code: 'INVALID_RESPONSE',
        message: '服务器返回数据格式不正确',
        statusCode: null,
      );
    }
  }
}

Map<String, dynamic> _jsonObject(dynamic data) {
  if (data is Map<String, dynamic>) {
    return data;
  }
  throw const FormatException('expected a JSON object');
}
