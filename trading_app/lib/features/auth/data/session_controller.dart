import 'dart:convert';

import 'package:flutter/foundation.dart';

import '../domain/auth_models.dart';
import 'auth_storage.dart';

enum SessionInvalidationReason { replaced, expired, revoked }

abstract class SessionController extends ChangeNotifier {
  Future<String?> readToken();
  Future<void> save(AuthSession session);
  Future<void> clear({SessionInvalidationReason? reason});
  SessionInvalidationReason? get lastInvalidationReason;
  bool get isInvalidating;
}

class AuthSessionController extends SessionController {
  AuthSessionController(this._storage);

  static const _tokenKey = 'auth:token';
  static const _userKey = 'auth:user';
  static const _expiresAtKey = 'auth:expiresAt';

  final AuthStorage _storage;

  Future<void>? _clearOperation;
  SessionInvalidationReason? _lastInvalidationReason;
  bool _isInvalidating = false;
  bool _isInvalidated = false;

  @override
  SessionInvalidationReason? get lastInvalidationReason =>
      _lastInvalidationReason;

  @override
  bool get isInvalidating => _isInvalidating;

  @override
  Future<String?> readToken() => _storage.read(_tokenKey);

  @override
  Future<void> save(AuthSession session) async {
    await _storage.write(_tokenKey, session.accessToken);
    await _storage.write(_userKey, jsonEncode(session.user.toJson()));
    await _storage.write(
      _expiresAtKey,
      session.expiresAt.toUtc().toIso8601String(),
    );
    _isInvalidated = false;
    _lastInvalidationReason = null;
  }

  @override
  Future<void> clear({SessionInvalidationReason? reason}) {
    final activeOperation = _clearOperation;
    if (activeOperation != null) {
      return activeOperation;
    }
    if (_isInvalidated) {
      return Future<void>.value();
    }

    _lastInvalidationReason = reason;
    _isInvalidating = true;
    final operation = _clearAuth();
    _clearOperation = operation;
    return operation;
  }

  Future<void> _clearAuth() async {
    try {
      await _storage.clearAuth();
      _isInvalidated = true;
    } finally {
      _isInvalidating = false;
      _clearOperation = null;
      notifyListeners();
    }
  }
}
