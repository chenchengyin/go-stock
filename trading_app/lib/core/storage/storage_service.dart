import 'package:shared_preferences/shared_preferences.dart';

/// 本地配置存储服务
class LocalStorageService {
  static const String _qgqpBIdKey = 'qgqp_b_id';

  SharedPreferences? _prefs;

  Future<void> init() async {
    _prefs = await SharedPreferences.getInstance();
  }

  /// 保存 qgqp_b_id
  Future<void> saveQgqpBId(String bid) async {
    await _prefs?.setString(_qgqpBIdKey, bid);
  }

  /// 获取 qgqp_b_id
  String? getQgqpBId() {
    return _prefs?.getString(_qgqpBIdKey);
  }

  /// 删除 qgqp_b_id
  Future<void> removeQgqpBId() async {
    await _prefs?.remove(_qgqpBIdKey);
  }
}
