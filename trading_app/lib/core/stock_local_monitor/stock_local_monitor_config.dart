import 'stock_local_monitor_types.dart';

/// 本地监控配置，所有阈值在此集中管理
class StockLocalMonitorConfig {
  static const enabled = true;
  static const maxHistory = 10;
  static const refreshIntervalSec = 10;

  /// 规则列表（注释/取消注释即可开关）
  static List<WaveRuleConfig> get rules => [
        WaveRuleConfig(
          id: 'price_10s',
          name: '10秒急速波动',
          changeType: 9001,
          checkType: CheckType.price,
          windowSize: 1,
          cooldownSec: 30,
          thresholds: [
            Threshold('轻度', 0.3),
            Threshold('中度', 0.5),
            Threshold('重度', 0.8),
          ],
        ),
        WaveRuleConfig(
          id: 'price_30s',
          name: '30秒急速波动',
          changeType: 9002,
          checkType: CheckType.price,
          windowSize: 3,
          cooldownSec: 60,
          thresholds: [
            Threshold('轻度', 0.5),
            Threshold('中度', 1.0),
            Threshold('重度', 1.5),
          ],
        ),
        WaveRuleConfig(
          id: 'price_60s',
          name: '60秒急速波动',
          changeType: 9003,
          checkType: CheckType.price,
          windowSize: 6,
          cooldownSec: 120,
          thresholds: [
            Threshold('轻度', 0.8),
            Threshold('中度', 1.5),
            Threshold('重度', 2.5),
          ],
        ),
        WaveRuleConfig(
          id: 'vol_10s',
          name: '10秒量能异动',
          changeType: 9004,
          checkType: CheckType.volume,
          windowSize: 1,
          cooldownSec: 60,
          thresholds: [
            Threshold('爆量-轻度', 3),
            Threshold('爆量-中度', 5),
            Threshold('爆量-重度', 10),
            Threshold('缩量-轻度', 0.3),
            Threshold('缩量-中度', 0.1),
            Threshold('缩量-重度', 0.05),
          ],
        ),
        WaveRuleConfig(
          id: 'vol_30s',
          name: '30秒量能异动',
          changeType: 9005,
          checkType: CheckType.volume,
          windowSize: 3,
          cooldownSec: 120,
          thresholds: [
            Threshold('爆量-轻度', 2),
            Threshold('爆量-中度', 3),
            Threshold('爆量-重度', 5),
            Threshold('缩量-轻度', 0.4),
            Threshold('缩量-中度', 0.2),
            Threshold('缩量-重度', 0.1),
          ],
        ),
      ];
}
