import 'radar_models.dart';

enum SellWarningType { lowOpen, highOpen }

class SellWarning {
  const SellWarning({required this.type, required this.tradeDate});

  final SellWarningType type;
  final String tradeDate;

  String get label => '卖出';
}

SellWarning? evaluateSellWarning(MonitoredStock stock) {
  if (!_isPositiveFinite(stock.price) ||
      !_isPositiveFinite(stock.open) ||
      !_isPositiveFinite(stock.preClose) ||
      !stock.changePercent.isFinite) {
    return null;
  }

  if (stock.open < stock.preClose && stock.changePercent <= -1.0) {
    return SellWarning(type: SellWarningType.lowOpen, tradeDate: stock.date);
  }

  if (stock.open > stock.preClose &&
      _isPositiveFinite(stock.high) &&
      _isPositiveFinite(stock.previousHigh) &&
      stock.high >= stock.previousHigh) {
    return SellWarning(type: SellWarningType.highOpen, tradeDate: stock.date);
  }

  return null;
}

class SellWarningTracker {
  final Map<String, SellWarning> _warnings = <String, SellWarning>{};
  final Map<String, String> _dismissedTradeDates = <String, String>{};

  Map<String, String> get dismissedTradeDates =>
      Map<String, String>.unmodifiable(_dismissedTradeDates);

  void restoreDismissedTradeDates(Map<String, String> tradeDates) {
    _dismissedTradeDates
      ..clear()
      ..addAll(
        Map<String, String>.fromEntries(
          tradeDates.entries.where(
            (entry) => entry.key.isNotEmpty && entry.value.isNotEmpty,
          ),
        ),
      );
  }

  bool observe(Iterable<MonitoredStock> stocks) {
    var dismissalsChanged = false;
    final activeCodes = <String>{};
    for (final stock in stocks) {
      activeCodes.add(stock.code);
      final dismissedTradeDate = _dismissedTradeDates[stock.code];
      if (dismissedTradeDate != null) {
        if (dismissedTradeDate == stock.date) {
          _warnings.remove(stock.code);
          continue;
        }
        _dismissedTradeDates.remove(stock.code);
        dismissalsChanged = true;
      }

      final previous = _warnings[stock.code];
      if (previous != null && previous.tradeDate == stock.date) {
        continue;
      }
      _warnings.remove(stock.code);
      final warning = evaluateSellWarning(stock);
      if (warning != null) {
        _warnings[stock.code] = warning;
      }
    }
    _warnings.removeWhere((code, _) => !activeCodes.contains(code));
    final dismissalCount = _dismissedTradeDates.length;
    _dismissedTradeDates.removeWhere((code, _) => !activeCodes.contains(code));
    return dismissalsChanged || dismissalCount != _dismissedTradeDates.length;
  }

  bool contains(String code) => _warnings.containsKey(code);

  bool dismiss(String code) {
    final warning = _warnings.remove(code);
    if (warning == null) return false;
    _dismissedTradeDates[code] = warning.tradeDate;
    return true;
  }

  void remove(String code) {
    _warnings.remove(code);
    _dismissedTradeDates.remove(code);
  }
}

bool _isPositiveFinite(double value) => value.isFinite && value > 0;
