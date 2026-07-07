/// 检测类型
enum CheckType { price, volume }

/// 阈值档位
class Threshold {
  final String label; // "轻度"/"中度"/"重度"
  final double value;

  const Threshold(this.label, this.value);
}

/// 单条规则配置
class WaveRuleConfig {
  final String id;
  final String name;
  final int changeType;
  final CheckType checkType;
  final int windowSize;
  final int cooldownSec;
  final int allowedDeviationMs;
  final List<Threshold> thresholds;

  const WaveRuleConfig({
    required this.id,
    required this.name,
    required this.changeType,
    required this.checkType,
    required this.windowSize,
    this.cooldownSec = 60,
    this.allowedDeviationMs = 3000,
    required this.thresholds,
  });
}

/// 单次快照
class StockSnapshot {
  final double price;
  final double amount;
  final int serverTime; // 服务端毫秒时间戳

  const StockSnapshot({
    required this.price,
    required this.amount,
    required this.serverTime,
  });
}
