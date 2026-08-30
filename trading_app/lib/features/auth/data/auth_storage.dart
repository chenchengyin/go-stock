import 'dart:math';

import 'package:shared_preferences/shared_preferences.dart';

abstract class AuthStorage {
  Future<String?> read(String key);
  Future<void> write(String key, String value);
  Future<void> clearAuth();
  Future<String> getOrCreateDeviceId();
}

class SharedPreferencesAuthStorage implements AuthStorage {
  SharedPreferencesAuthStorage({Random? random})
    : _random = random ?? Random.secure();

  static const _authPrefix = 'auth:';
  static const _deviceIdKey = 'device:id';

  final Random _random;
  Future<String>? _deviceId;

  @override
  Future<String?> read(String key) async {
    final preferences = await SharedPreferences.getInstance();
    return preferences.getString(key);
  }

  @override
  Future<void> write(String key, String value) async {
    final preferences = await SharedPreferences.getInstance();
    await preferences.setString(key, value);
  }

  @override
  Future<void> clearAuth() async {
    final preferences = await SharedPreferences.getInstance();
    final authKeys = preferences.getKeys().where(
      (key) => key.startsWith(_authPrefix),
    );
    await Future.wait(authKeys.map(preferences.remove));
  }

  @override
  Future<String> getOrCreateDeviceId() {
    return _deviceId ??= _loadOrCreateDeviceId();
  }

  Future<String> _loadOrCreateDeviceId() async {
    final stored = await read(_deviceIdKey);
    if (stored != null && stored.isNotEmpty) {
      return stored;
    }

    final generated = List<String>.generate(
      16,
      (_) => _random.nextInt(256).toRadixString(16).padLeft(2, '0'),
    ).join();
    await write(_deviceIdKey, generated);
    return generated;
  }
}
