abstract class LocalCache {
  Future<void> setString(String key, String value);
  Future<String?> getString(String key);
}

class MemoryLocalCache implements LocalCache {
  final Map<String, String> _store = {};

  @override
  Future<String?> getString(String key) async => _store[key];

  @override
  Future<void> setString(String key, String value) async {
    _store[key] = value;
  }
}

