import 'package:trading_app/core/stock_local_monitor/stock_local_monitor_config.dart';
import 'package:trading_app/core/stock_local_monitor/stock_local_monitor_types.dart';
import 'package:trading_app/features/radar/domain/radar_models.dart';

/// 本地监控引擎
class StockLocalMonitor {
  StockLocalMonitor() : _enabled = StockLocalMonitorConfig.enabled;

  final bool _enabled;

  /// 每只股票的历史快照队列
  final Map<String, List<StockSnapshot>> _history = {};

  /// 冷却记录: key = "ruleId:stockCode", value = 下次允许触发的时间(秒)
  final Map<String, int> _cooldowns = {};

  /// 推送最新快照，返回检测到的异动
  List<StockChange> pushSnapshots(List<MonitoredStock> stocks) {
    if (!_enabled) return [];

    final alerts = <StockChange>[];
    final activeCodes = <String>{};

    for (final stock in stocks) {
      activeCodes.add(stock.code);
      if (stock.serverTime <= 0) continue;

      // 1. 保存快照
      _history.putIfAbsent(stock.code, () => []);
      final hist = _history[stock.code]!;
      hist.add(StockSnapshot(
        price: stock.price,
        amount: stock.amount,
        serverTime: stock.serverTime,
      ));
      // 超出上限时移除最老数据
      while (hist.length > StockLocalMonitorConfig.maxHistory) {
        hist.removeAt(0);
      }

      // 2. 遍历规则
      for (final rule in StockLocalMonitorConfig.rules) {
        final alert = _check(rule, stock, hist);
        if (alert != null) alerts.add(alert);
      }
    }

    // 3. 清理已删除股票的历史
    _cleanup(activeCodes);

    return alerts;
  }

  /// 清理不再监控的股票历史快照
  void _cleanup(Set<String> activeCodes) {
    _history.removeWhere((k, v) => !activeCodes.contains(k));
  }

  /// 对单条规则、单只股票执行检测
  StockChange? _check(
      WaveRuleConfig rule, MonitoredStock stock, List<StockSnapshot> hist) {
    // ── 数据充足性校验 ──
    if (hist.length <= rule.windowSize) return null;

    final now = hist.last;

    // ── 按服务端时间查找目标快照 ──
    final targetTime = now.serverTime - rule.windowSize * StockLocalMonitorConfig.refreshIntervalSec * 1000;
    StockSnapshot? then;
    for (final snap in hist.reversed) {
      final diff = (snap.serverTime - targetTime).abs();
      if (diff <= rule.allowedDeviationMs) {
        then = snap;
        break;
      }
    }
    if (then == null) return null;

    // ── 时间差校验：确保 then 和 now 的时间差在窗口预期范围内 ──
    final expectedWindowMs = rule.windowSize * StockLocalMonitorConfig.refreshIntervalSec * 1000;
    final actualDiffMs = (now.serverTime - then.serverTime).abs();
    if (actualDiffMs > expectedWindowMs + rule.allowedDeviationMs) {
      return null; // then 太远了，数据可能不可靠
    }

    // ── 冷却校验 ──
    final coolKey = '${rule.id}:${stock.code}';
    final nowSec = now.serverTime ~/ 1000;
    final lastTrigger = _cooldowns[coolKey] ?? 0;
    if (nowSec < lastTrigger) return null;

    // ── 计算波动值 ──
    double rawValue; // 用于阈值匹配
    String desc;
    // 价格涨幅（统一用于 changeRate 字段，所有规则都显示价格变化）
    final priceChangeRate = then.price != 0
        ? (now.price - then.price) / then.price * 100
        : 0.0;
    // 异动对应的成交额（统一用窗口内成交额，而非当日累计）
    double displayAmount = stock.amount;

    if (rule.checkType == CheckType.price) {
      rawValue = priceChangeRate;
      final dir = rawValue >= 0 ? '急涨' : '急跌';
      desc = '${rule.name} $dir ${rawValue.abs().toStringAsFixed(2)}%';
      // 价格异动也显示窗口内成交额，而非当日累计
      final curSum = now.amount - then.amount;
      // 窗口内成交额：now.amount 减 then.amount 的差值
      // 如果差值 <= 0（数据异常/倒退），显示 0 而非当日累计，避免误导
      displayAmount = curSum > 0 ? curSum : 0;
    } else {
      // 成交额：amount 是当日累计值，用差值计算窗口内增量
      // 当前窗口成交额 = now.amount - then.amount
      // 前一个窗口成交额 = then.amount - beforeThen.amount
      final beforeTargetTime = then.serverTime - rule.windowSize * StockLocalMonitorConfig.refreshIntervalSec * 1000;
      StockSnapshot? beforeThen;
      for (final snap in hist.reversed) {
        if (snap.serverTime >= then.serverTime) continue;
        final diff = (snap.serverTime - beforeTargetTime).abs();
        if (diff <= rule.allowedDeviationMs) {
          beforeThen = snap;
          break;
        }
      }
      if (beforeThen == null) return null;

      // 校验 beforeThen 和 then 的时间差
      final beforeDiffMs = (then.serverTime - beforeThen.serverTime).abs();
      if (beforeDiffMs > expectedWindowMs + rule.allowedDeviationMs) {
        return null;
      }

      final curSum = now.amount - then.amount;
      final prevSum = then.amount - beforeThen.amount;
      if (curSum <= 0 || prevSum <= 0) return null;
      rawValue = curSum / prevSum;
      displayAmount = curSum; // 窗口内成交额，才是异动对应的真实成交额
      desc = rawValue >= 1
          ? '${rule.name} 爆量 ${rawValue.toStringAsFixed(1)}倍'
          : '${rule.name} 缩量 ${(rawValue * 100).toStringAsFixed(0)}%';
    }

    // ── 匹配阈值（从重到轻） ──
    final absValue = rawValue.abs();
    Threshold? hit;

    if (rule.checkType == CheckType.volume) {
      // 成交额规则：>1 匹配爆量阈值，<1 匹配缩量阈值
      if (rawValue >= 1) {
        for (final t in rule.thresholds.reversed) {
          if (t.value >= 1 && absValue >= t.value) {
            hit = t;
            break;
          }
        }
      } else {
        for (final t in rule.thresholds.reversed) {
          if (t.value < 1 && absValue <= t.value) {
            hit = t;
            break;
          }
        }
      }
    } else {
      // 价格规则：走绝对值，所有阈值通用
      for (final t in rule.thresholds.reversed) {
        if (absValue >= t.value) {
          hit = t;
          break;
        }
      }
    }
    if (hit == null) return null;

    // ── 更新冷却 ──
    _cooldowns[coolKey] = nowSec + rule.cooldownSec;

    // ── 解析时间 ──
    final dt =
        DateTime.fromMillisecondsSinceEpoch(now.serverTime, isUtc: true).toLocal();
    final dateStr =
        '${dt.year}-${_pad(dt.month)}-${_pad(dt.day)}';
    final timeStr =
        '${_pad(dt.hour)}:${_pad(dt.minute)}:${_pad(dt.second)}';

    return StockChange(
      id: -(now.serverTime * 10000 + rule.changeType), // 负值唯一ID，避免与服务端重复
      stockCode: stock.code,
      stockName: stock.name,
      changeType: rule.changeType,
      typeName: rule.name,
      price: stock.price,
      changeRate: priceChangeRate,
      currentChangeRate: stock.changePercent,
      volume: stock.volume,
      amount: displayAmount,
      changeTime: timeStr,
      changeDate: dateStr,
      description: '$desc [${hit.label}]',
    );
  }

  String _pad(int n) => n.toString().padLeft(2, '0');
}
