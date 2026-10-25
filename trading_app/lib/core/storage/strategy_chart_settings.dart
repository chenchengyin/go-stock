import 'package:flutter/foundation.dart';
import 'package:shared_preferences/shared_preferences.dart';

/// Independent persisted chart panels for each supported strategy.
class StrategyChartSettings extends ChangeNotifier {
  static const redModule = 'radar.red_strategy';
  static const mainModule = 'radar.main_strategy';
  static const preferenceKey = 'show_red_strategy_kline';
  static const t1IntradayPreferenceKey = 'show_red_strategy_t1_intraday';
  static const mainKlinePreferenceKey = 'show_main_strategy_kline';
  static const mainIntradayPreferenceKey = 'show_main_strategy_t1_intraday';
  static const _keys = {
    redModule: (preferenceKey, t1IntradayPreferenceKey),
    mainModule: (mainKlinePreferenceKey, mainIntradayPreferenceKey),
  };

  final Map<String, bool> _values = {};
  bool _ready = false;
  bool get ready => _ready;
  bool get enabled => enabledFor(redModule);
  bool get t1IntradayEnabled => intradayEnabledFor(redModule);

  String _module(String moduleCode) =>
      moduleCode == 'radar.red_shared_gob_strategy' ? redModule : moduleCode;

  bool enabledFor(String moduleCode) {
    final keys = _keys[_module(moduleCode)];
    return keys != null && (_values[keys.$1] ?? true);
  }

  bool intradayEnabledFor(String moduleCode) {
    final keys = _keys[_module(moduleCode)];
    return keys != null && (_values[keys.$2] ?? true);
  }

  Future<void> restore() async {
    try {
      final prefs = await SharedPreferences.getInstance();
      for (final keys in _keys.values) {
        for (final key in [keys.$1, keys.$2]) {
          _values[key] = prefs.getBool(key) ?? true;
        }
      }
    } catch (error) {
      debugPrint('Strategy chart settings restore failed: $error');
    } finally {
      _ready = true;
      notifyListeners();
    }
  }

  Future<bool> setEnabled(bool value) => setEnabledFor(redModule, value);
  Future<bool> setT1IntradayEnabled(bool value) =>
      setIntradayEnabledFor(redModule, value);

  Future<bool> setEnabledFor(String moduleCode, bool value) async {
    final keys = _keys[_module(moduleCode)];
    return keys != null && await _set(keys.$1, value);
  }

  Future<bool> setIntradayEnabledFor(String moduleCode, bool value) async {
    final keys = _keys[_module(moduleCode)];
    return keys != null && await _set(keys.$2, value);
  }

  Future<bool> _set(String key, bool value) async {
    final previous = _values[key] ?? true;
    if (previous == value) return true;
    _values[key] = value;
    notifyListeners();
    try {
      final prefs = await SharedPreferences.getInstance();
      if (await prefs.setBool(key, value)) return true;
    } catch (error) {
      debugPrint('Strategy chart setting save failed: $error');
    }
    _values[key] = previous;
    notifyListeners();
    return false;
  }
}
