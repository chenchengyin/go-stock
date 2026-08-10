import 'stock_local_monitor_types.dart';

/// 本地监控配置，所有阈值在此集中管理
class StockLocalMonitorConfig {
  static const enabled = true;
  static const maxHistory = 10;
  static const refreshIntervalSec = 10;

  /// 规则列表（注释/取消注释即可开关）
  static List<WaveRuleConfig> get rules => [
        // WaveRuleConfig(
        //   id: 'price_10s',
        //   name: '10秒急速波动',
        //   changeType: 9001,
        //   checkType: CheckType.price,
        //   windowSize: 1,
        //   cooldownSec: 30,
        //   thresholds: [
        //     Threshold('轻度', 0.3),
        //     Threshold('中度', 0.5),
        //     Threshold('重度', 0.8),
        //   ],
        // ),
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
        // WaveRuleConfig(
        //   id: 'vol_10s',
        //   name: '10秒量能异动',
        //   changeType: 9004,
        //   checkType: CheckType.volume,
        //   windowSize: 1,
        //   cooldownSec: 60,
        //   thresholds: [
        //     Threshold('爆量3倍', 3),
        //     Threshold('爆量5倍', 5),
        //     Threshold('爆量10倍', 10),
        //     Threshold('缩量0.3倍', 0.3),
        //     Threshold('缩量0.1倍', 0.1),
        //     Threshold('缩量0.05倍', 0.05),
        //   ],
        // ),
        WaveRuleConfig(
          id: 'vol_30s',
          name: '30秒量能异动',
          changeType: 9005,
          checkType: CheckType.volume,
          windowSize: 3,
          cooldownSec: 120,
          thresholds: [
            Threshold('爆量4倍', 4),
          ],
        ),
        WaveRuleConfig(
          id: 'vol_60s',
          name: '60秒量能异动',
          changeType: 9006,
          checkType: CheckType.volume,
          windowSize: 6,
          cooldownSec: 180,
          thresholds: [
            Threshold('爆量4倍', 4),
          ],
        ),
      ];
}
