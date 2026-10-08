import 'package:flutter/foundation.dart';
import 'package:shared_preferences/shared_preferences.dart';

/// Only controls the visibility and network work of the red-strategy chart.
class RedKlineSettings extends ChangeNotifier {
  static const preferenceKey = 'show_red_strategy_kline';
  bool _enabled = true;
  bool _ready = false;
  bool get enabled => _enabled;
  bool get ready => _ready;

  Future<void> restore() async {
    try {
      final prefs = await SharedPreferences.getInstance();
      _enabled = prefs.getBool(preferenceKey) ?? true;
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
}
