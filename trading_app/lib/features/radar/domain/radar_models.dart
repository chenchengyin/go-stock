class WatchStock {
  const WatchStock({
    required this.symbol,
    required this.name,
    required this.price,
    required this.changePercent,
    required this.volumeRatio,
    required this.alerts,
  });

  final String symbol;
  final String name;
  final double price;
  final double changePercent;
  final double volumeRatio;
  final List<AlertRule> alerts;
}

class AlertRule {
  const AlertRule({
    required this.type,
    required this.windowSeconds,
    required this.thresholdPercent,
    required this.enabled,
  });

  final AlertRuleType type;
  final int windowSeconds;
  final double thresholdPercent;
  final bool enabled;
}

enum AlertRuleType { priceChange, volumeSpike, waterfall, surge }

extension AlertRuleTypeLabel on AlertRuleType {
  String get label {
    switch (this) {
      case AlertRuleType.priceChange:
        return '价格波动';
      case AlertRuleType.volumeSpike:
        return '成交量异动';
      case AlertRuleType.waterfall:
        return '跳水识别';
      case AlertRuleType.surge:
        return '急拉识别';
    }
  }
}

