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

  void observe(Iterable<MonitoredStock> stocks) {
    final activeCodes = <String>{};
    for (final stock in stocks) {
      activeCodes.add(stock.code);
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
  }

  bool contains(String code) => _warnings.containsKey(code);

  void remove(String code) => _warnings.remove(code);
}

bool _isPositiveFinite(double value) => value.isFinite && value > 0;
