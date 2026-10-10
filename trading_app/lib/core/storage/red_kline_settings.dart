import 'package:flutter/foundation.dart';
import 'package:shared_preferences/shared_preferences.dart';

/// Controls visibility and network work for red-strategy charts.
class RedKlineSettings extends ChangeNotifier {
  static const preferenceKey = 'show_red_strategy_kline';
  static const t1IntradayPreferenceKey = 'show_red_strategy_t1_intraday';
  bool _enabled = true;
  bool _t1IntradayEnabled = true;
  bool _ready = false;
  bool get enabled => _enabled;
  bool get t1IntradayEnabled => _t1IntradayEnabled;
  bool get ready => _ready;

  Future<void> restore() async {
    try {
      final prefs = await SharedPreferences.getInstance();
      _enabled = prefs.getBool(preferenceKey) ?? true;
      _t1IntradayEnabled = prefs.getBool(t1IntradayPreferenceKey) ?? true;
    } catch (error) {
      debugPrint('Red K-line setting restore failed: $error');
    } finally {
      _ready = true;
      notifyListeners();
    }
  }

  Future<bool> setEnabled(bool value) async {
    if (_enabled == value) return true;
    final previous = _enabled;
    _enabled = value;
    notifyListeners();
    try {
      final prefs = await SharedPreferences.getInstance();
      if (await prefs.setBool(preferenceKey, value)) return true;
    } catch (error) {
      debugPrint('Red K-line setting save failed: $error');
    }
    _enabled = previous;
    notifyListeners();
    return false;
  }

  Future<bool> setT1IntradayEnabled(bool value) async {
    if (_t1IntradayEnabled == value) return true;
    final previous = _t1IntradayEnabled;
    _t1IntradayEnabled = value;
    notifyListeners();
    try {
      final prefs = await SharedPreferences.getInstance();
      if (await prefs.setBool(t1IntradayPreferenceKey, value)) return true;
    } catch (error) {
      debugPrint('Red T+1 intraday setting save failed: $error');
    }
    _t1IntradayEnabled = previous;
    notifyListeners();
    return false;
  }
}
