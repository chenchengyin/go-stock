import 'dart:convert';

import 'package:flutter/foundation.dart';

import '../domain/auth_models.dart';
import 'auth_storage.dart';

enum SessionInvalidationReason { replaced, expired, revoked }

class SessionTokenSnapshot {
  const SessionTokenSnapshot({required this.token, required this.generation});

  final String? token;
  final int generation;
}

abstract class SessionController extends ChangeNotifier {
  Future<String?> readToken();
  Future<SessionTokenSnapshot> captureToken();
  Future<void> save(AuthSession session);
  Future<void> clear({SessionInvalidationReason? reason});
  Future<void> clearIfCurrent(
    SessionTokenSnapshot snapshot, {
    SessionInvalidationReason? reason,
  });
  SessionInvalidationReason? get lastInvalidationReason;
  bool get isInvalidating;
}

class AuthSessionController extends SessionController {
  AuthSessionController(this._storage);

  static const _tokenKey = 'auth:token';
  static const _userKey = 'auth:user';
  static const _expiresAtKey = 'auth:expiresAt';

  final AuthStorage _storage;

  Future<void> _transitions = Future<void>.value();
  int _generation = 0;
  SessionInvalidationReason? _lastInvalidationReason;
  bool _isInvalidating = false;
  bool _isInvalidated = false;

  @override
  SessionInvalidationReason? get lastInvalidationReason =>
      _lastInvalidationReason;

  @override
  bool get isInvalidating => _isInvalidating;

  @override
  Future<String?> readToken() => _serialize(() => _storage.read(_tokenKey));

  @override
  Future<SessionTokenSnapshot> captureToken() => _serialize(() async {
    return SessionTokenSnapshot(
      token: await _storage.read(_tokenKey),
      generation: _generation,
    );
  });

  @override
  Future<void> save(AuthSession session) => _serialize(() async {
    await _storage.write(_tokenKey, session.accessToken);
    await _storage.write(_userKey, jsonEncode(session.user.toJson()));
    await _storage.write(
      _expiresAtKey,
      session.expiresAt.toUtc().toIso8601String(),
    );
    _generation++;
    _isInvalidated = false;
    _lastInvalidationReason = null;
  });

  @override
  Future<void> clear({SessionInvalidationReason? reason}) =>
      _serialize(() => _clearAuth(reason));

  @override
  Future<void> clearIfCurrent(
    SessionTokenSnapshot snapshot, {
    SessionInvalidationReason? reason,
  }) => _serialize(() async {
    if (snapshot.generation != _generation ||
        snapshot.token != await _storage.read(_tokenKey)) {
      return;
    }
    await _clearAuth(reason);
  });

  Future<void> _clearAuth(SessionInvalidationReason? reason) async {
    if (_isInvalidated) {
      return;
    }

    _lastInvalidationReason = reason;
    _isInvalidating = true;
    try {
      await _storage.clearAuth();
      _generation++;
      _isInvalidated = true;
    } finally {
      _isInvalidating = false;
      notifyListeners();
    }
  }

  Future<T> _serialize<T>(Future<T> Function() operation) {
    final result = _transitions.then((_) => operation());
    _transitions = result.then<void>((_) {}, onError: (_, _) {});
    return result;
  }
}
