import 'package:flutter/material.dart';
import 'package:trading_app/core/theme/app_colors.dart';
import 'package:trading_app/features/radar/data/red_kline_repository.dart';
import 'package:trading_app/features/radar/domain/t0_close_target_monitor.dart';
import 't0_strategy_view_model.dart';
import 'stock_rating_store.dart';

const _patternDescriptions = <String, String>{
  'ZT': '涨停',
  'DT': '跌停',
  'PB': '涨停破板',
  'YX': '阳十字星',
  'YXN': '阴十字星',
  'SY': '小阳',
  'XY': '小阴',
  'MY': '中阳',
  'MYIN': '中阴',
  'DY': '大阳',
  'DYIN': '大阴',
  'XX': '其他',
};

String _patternDescription(String pattern) {
  return pattern
      .split('|')
      .map((code) => _patternDescriptions[code] ?? code)
      .join('＋');
}

class StrategyPatternChip extends StatelessWidget {
  const StrategyPatternChip({super.key, required this.stock});
  final T0StrategyStock stock;

  @override
  Widget build(BuildContext context) {
    if (stock.buySignal.isEmpty && stock.pattern.isEmpty) {
      return const SizedBox.shrink();
    }

    Color dotColor;
    switch (stock.buySignal) {
      case 'blue':
        dotColor = AppColors.info;
        break;
      case 'orange':
        dotColor = AppColors.tagOrange;
        break;
      case 'green':
        dotColor = AppColors.success;
        break;
      case 'yellow':
        dotColor = AppColors.warning;
        break;
      case 'red':
        dotColor = AppColors.error;
        break;
      default:
        dotColor = AppColors.textTertiary;
    }

    // 灰灯仅表示样本不足/无库记录；只要有形态统计就显示整数率，不用 —/—
    final noStats =
        stock.pattern.isEmpty ||
        (stock.patternT0N == 0 &&
            stock.patternWinPct == 0 &&
            stock.patternFailPct == 0);
    final rateText = noStats
        ? '—/—/${stock.patternT0N}'
        : '${stock.patternWinPct.round()}/${stock.patternEarnPct.round()}/${stock.patternT0N}';

    final tooltipMessage = noStats
        ? '形态样本不足'
        : '形态：${stock.pattern}（${_patternDescription(stock.pattern)}）\n'
              '达标率：${stock.patternWinPct.round()}%\n'
              '赚率：${stock.patternEarnPct.round()}%\n'
              '样本数：${stock.patternT0N}';

    return Tooltip(
      message: tooltipMessage,
      child: Row(
        mainAxisSize: MainAxisSize.min,
        children: [
          Container(
            width: 8,
            height: 8,
            decoration: BoxDecoration(color: dotColor, shape: BoxShape.circle),
          ),
          const SizedBox(width: 2),
          Text(
            rateText,
            style: TextStyle(
              fontSize: 11,
              fontWeight: FontWeight.w500,
              color: noStats ? AppColors.textTertiary : AppColors.textSecondary,
            ),
          ),
        ],
      ),
    );
  }
}

Color _stockRatingColor(StockRating rating) {
  return switch (rating) {
    StockRating.heavy => AppColors.error,
    StockRating.light => AppColors.tagOrange,
    StockRating.avoid => AppColors.textSecondary,
    StockRating.unrated => AppColors.textTertiary,
  };
}

class StrategyRatingChip extends StatelessWidget {
  const StrategyRatingChip({super.key, required this.stock});
  final T0StrategyStock stock;

  @override
  Widget build(BuildContext context) {
    final hasStats = stock.pattern.isNotEmpty && stock.patternT0N > 0;
    final rating = calculateStockRating(
      hasStats: hasStats,
      earnPct: stock.patternEarnPct,
    );
    final color = _stockRatingColor(rating);
    final message = rating == StockRating.unrated
        ? '评级：未评级\n原因：暂无有效形态统计'
        : '评级：${rating.label}\n真赚率：${stock.patternEarnPct.round()}%\n'
              '规则：重仓≥75%，轻仓≥55%，不买<55%';

    return Tooltip(
      key: ValueKey('stock-rating-${stock.rawCode}'),
      message: message,
      child: Container(
        padding: const EdgeInsets.symmetric(horizontal: 5, vertical: 2),
        decoration: BoxDecoration(
          color: color.withValues(alpha: 0.1),
          borderRadius: BorderRadius.circular(4),
          border: Border.all(color: color.withValues(alpha: 0.35)),
        ),
        child: Text(
          rating.label,
          style: TextStyle(
            fontSize: 11,
            fontWeight: FontWeight.w600,
            color: color,
          ),
        ),
      ),
    );
  }
}

class StrategyReferenceChips extends StatelessWidget {
  const StrategyReferenceChips({super.key, required this.stock});
  final T0StrategyStock stock;

  @override
  Widget build(BuildContext context) {
    if (stock.t0ReferenceHits.isEmpty) {
      return const SizedBox.shrink();
    }
    return Wrap(
      spacing: 4,
      runSpacing: 2,
      children: stock.t0ReferenceHits.map((hit) {
        final hasStats = hit.sampleCount > 0;
        final rateText = hasStats
            ? '${hit.targetRate.round()}/${hit.earnRate.round()}/${hit.sampleCount}'
            : '—/—/${hit.sampleCount}';
        final explanation = hit.description.isEmpty
            ? ''
            : '条件说明：${hit.description}\n';
        final message =
            'T0参考形态：${hit.name}\n'
            '$explanation'
            '评判：T0开盘→T0收盘（当日价格表现）\n'
            '达标率：${hasStats ? '${hit.targetRate.round()}%' : '—'}\n'
            '不亏率（含持平）：${hasStats ? '${hit.earnRate.round()}%' : '—'}\n'
            '正收益率：${hasStats ? '${hit.strictWinRate.round()}%' : '—'}\n'
            '达标为收益≥2.5%，正收益为收益>0%\n'
            '样本数：${hit.sampleCount}';
        return Tooltip(
          message: message,
          child: Container(
            padding: const EdgeInsets.symmetric(horizontal: 5, vertical: 2),
            decoration: BoxDecoration(
              color: AppColors.brand.withValues(alpha: 0.08),
              borderRadius: BorderRadius.circular(4),
              border: Border.all(
                color: AppColors.brand.withValues(alpha: 0.28),
              ),
            ),
            child: Text(
              '${hit.name} $rateText',
              style: TextStyle(
                fontSize: 11,
                fontWeight: FontWeight.w500,
                color: hasStats ? AppColors.brand : AppColors.textTertiary,
              ),
            ),
          ),
        );
      }).toList(),
    );
  }
}

String? _redRuleHitConditionSummary(
  T0StrategyStock stock,
  T0RedRuleHit hit, {
  RedKlineWindow? window,
  String? selectedDate,
}) {
  if (hit.ruleKey != 'gob_zt_t1_bear_open_close_body_4combos_v1') {
    return null;
  }
  if (window == null ||
      window.date != stock.tradeDate ||
      (selectedDate != null && selectedDate != stock.tradeDate) ||
      window.anchorIndex < 2 ||
      window.anchorIndex >= window.bars.length ||
      window.bars[window.anchorIndex].date != stock.tradeDate) {
    return '命中分组：待日线数据确认';
  }

  final t2 = window.bars[window.anchorIndex - 2];
  final t1 = window.bars[window.anchorIndex - 1];
  if (t2.close <= 0 || t1.open <= 0 || t1.close <= 0) {
    return '命中分组：待日线数据确认';
  }

  final openRet = (t1.open / t2.close - 1) * 100;
  final closeRet = (t1.close / t2.close - 1) * 100;
  final body = (t1.open - t1.close) / t2.close * 100;
  final bodyRet = body.abs();
  final group =
      openRet >= 2 &&
          openRet < 4 &&
          closeRet >= -5 &&
          closeRet < -2 &&
          bodyRet >= 5 &&
          bodyRet < 8
      ? 1
      : openRet >= 4 &&
            openRet < 6 &&
            closeRet >= -2 &&
            closeRet < 0 &&
            bodyRet >= 5 &&
            bodyRet < 8
      ? 2
      : openRet >= 2 &&
            openRet < 4 &&
            closeRet >= -2 &&
            closeRet < 0 &&
            bodyRet > 0 &&
            bodyRet < 3
      ? 3
      : openRet >= 6 &&
            openRet < 10 &&
            closeRet >= 0 &&
            closeRet < 2 &&
            bodyRet >= 5 &&
            bodyRet < 8
      ? 4
      : null;
  if (group == null) return '命中分组：当前日线未能匹配';

  final groupRule = switch (group) {
    1 => '开盘≥2%且<4%、收盘≥-5%且<-2%、实体≥5%且<8%',
    2 => '开盘≥4%且<6%、收盘≥-2%且<0%、实体≥5%且<8%',
    3 => '开盘≥2%且<4%、收盘≥-2%且<0%、实体>0%且<3%',
    _ => '开盘≥6%且<10%、收盘≥0%且<2%、实体≥5%且<8%',
  };
  String formatPct(double value) =>
      '${value >= 0 ? "+" : ""}${value.toStringAsFixed(2)}%';
  return '第$group组（$groupRule）；本次T-1：开盘${formatPct(openRet)}、'
      '收盘${formatPct(closeRet)}、实体${bodyRet.toStringAsFixed(2)}%';
}

class StrategyRedRuleChips extends StatelessWidget {
  const StrategyRedRuleChips({
    super.key,
    required this.stock,
    this.redKlineWindow,
    this.selectedDate,
  });
  final T0StrategyStock stock;
  final RedKlineWindow? redKlineWindow;
  final String? selectedDate;

  @override
  Widget build(BuildContext context) {
    if (stock.redRuleHits.isEmpty) return const SizedBox.shrink();
    return Wrap(
      spacing: 4,
      runSpacing: 2,
      children: stock.redRuleHits.map((hit) {
        final stats = hit.stats;
        final bodyStats = hit.exitHorizon == 'T0_T1_body';
        final verified =
            hit.statsStatus == 'verified' &&
            stats != null &&
            stats.sampleCount > 0 &&
            (stats.entrySource == hit.entrySource ||
                (hit.entrySource == 'realtime_open' &&
                    stats.entrySource == 'daily_open') ||
                (hit.entrySource == 'realtime_close' &&
                    stats.entrySource == 'daily_close')) &&
            stats.exitHorizon == hit.exitHorizon &&
            (!bodyStats ||
                (stats.t0BodyTargetRate != null &&
                    stats.t1BodyTargetRate != null));
        final horizon = hit.exitHorizon == 'T0_high'
            ? 'T0盘中最高价'
            : hit.exitHorizon == 'T0_close'
            ? 'T0收盘'
            : hit.exitHorizon == 'T+1_close'
            ? 'T+1收盘'
            : hit.exitHorizon == 'T+1_target_2_5'
            ? 'T+1目标+2.5%触及'
            : bodyStats
            ? 'T0/T+1阳线实体'
            : hit.exitHorizon;
        final measurementNote = hit.exitHorizon == 'T0_high'
            ? '\n口径：按T0日线最高价计算盘中浮盈触及，不代表实际成交或落袋收益'
            : hit.exitHorizon == 'T0_close'
            ? '\n口径：按T0开盘到收盘计算；T0买入的A股不能当日卖出'
            : hit.exitHorizon == 'T+1_target_2_5'
            ? '\n口径：目标触及率按T+1日线最高价计算；真赚率按T+1收盘价计算，不代表实际成交'
            : bodyStats
            ? '\n口径：实体=(当日收盘/开盘−1)×100%，严格>2.5%；两天任一天达标按股票日去重，不代表交易盈利；缺少任一日开盘或收盘不计入共同分母'
            : '';
        final profitLabel = hit.exitHorizon == 'T0_high'
            ? '浮盈触及'
            : hit.exitHorizon == 'T0_close'
            ? '收盘高于开盘'
            : '真赚';
        final targetLabel = hit.targetReturnPct > 0
            ? '达标(≥${hit.targetReturnPct.toStringAsFixed(0)}%)'
            : hit.exitHorizon == 'T0_high'
            ? '目标触及(>2.5%)'
            : hit.exitHorizon == 'T+1_target_2_5'
            ? '目标触及'
            : '达标';
        final source = hit.entrySource == 'daily_open'
            ? '日线开盘代理'
            : hit.entrySource == 'daily_close'
            ? '日线收盘代理'
            : hit.entrySource == 'realtime_open'
            ? '实时确认开盘'
            : hit.entrySource == 'realtime_close'
            ? '收盘价'
            : hit.entrySource == 'pending_close'
            ? '等待尾盘确认'
            : '待竞价';
        final statisticsLabel = verified && bodyStats
            ? 'T0达标${stats.t0BodyTargetRate!.toStringAsFixed(1)}% · T1达标${stats.t1BodyTargetRate!.toStringAsFixed(1)}% · 任一天${stats.targetRate.toStringAsFixed(1)}%'
            : verified
            ? '$profitLabel${stats.profitWinRate.toStringAsFixed(1)}% · $targetLabel${stats.targetRate.toStringAsFixed(1)}%'
            : '';
        final label = hit.statsStatus == 'preclose'
            ? '${hit.name} · 待尾盘确认'
            : hit.statsStatus == 'preopen'
            ? '${hit.name} · 待竞价'
            : verified
            ? '${hit.name} · $horizon $statisticsLabel · 样本${stats.sampleCount}'
            : '${hit.name} · $horizon 统计待核验';
        final hitSummary = _redRuleHitConditionSummary(
          stock,
          hit,
          window: redKlineWindow,
          selectedDate: selectedDate,
        );
        final explanation =
            '${hit.description.isEmpty ? '' : '条件说明：${hit.description}\n'}'
            '${hitSummary == null ? '' : '命中情况：$hitSummary\n'}';
        final statisticsMessage = verified && bodyStats
            ? 'T0实体达标率：${stats.t0BodyTargetRate!.toStringAsFixed(1)}%\n'
                  'T1实体达标率：${stats.t1BodyTargetRate!.toStringAsFixed(1)}%\n'
                  '任一天实体达标率：${stats.targetRate.toStringAsFixed(1)}%\n'
            : verified
            ? '$profitLabel率：${stats.profitWinRate.toStringAsFixed(1)}%\n'
                  '$targetLabel率：${stats.targetRate.toStringAsFixed(1)}%\n'
            : '';
        final message = verified
            ? '形态：${hit.name}\n$explanation评判：$horizon$measurementNote\n入场：$source\n'
                  '$statisticsMessage'
                  '样本数：${stats.sampleCount}\n覆盖：${stats.dateStart}～${stats.dateEnd}'
            : '形态：${hit.name}\n$explanation评判：$horizon$measurementNote\n入场：$source\n'
                  '${hit.statsStatus == 'preclose'
                      ? '待T0收阴收盘确认'
                      : hit.statsStatus == 'preopen'
                      ? '待竞价确认'
                      : '统计待核验'}';
        return Tooltip(
          message: message,
          child: Container(
            padding: const EdgeInsets.symmetric(horizontal: 5, vertical: 2),
            decoration: BoxDecoration(
              color: AppColors.tagRed.withValues(alpha: 0.08),
              borderRadius: BorderRadius.circular(4),
              border: Border.all(
                color: AppColors.tagRed.withValues(alpha: 0.28),
              ),
            ),
            child: Text(
              label,
              style: TextStyle(
                fontSize: 11,
                fontWeight: FontWeight.w500,
                color: verified ? AppColors.tagRed : AppColors.textTertiary,
              ),
            ),
          ),
        );
      }).toList(),
    );
  }
}

class StrategySellTargetChip extends StatelessWidget {
  const StrategySellTargetChip({super.key, required this.monitor});
  final T0CloseTargetMonitor monitor;
  @override
  Widget build(BuildContext context) {
    final reached = monitor.triggered;
    final color = reached ? AppColors.errorStrong : AppColors.tagRed;
    return Container(
      padding: const EdgeInsets.symmetric(horizontal: 5, vertical: 2),
      decoration: BoxDecoration(
        color: color.withValues(alpha: 0.08),
        borderRadius: BorderRadius.circular(4),
        border: Border.all(color: color.withValues(alpha: 0.3)),
      ),
      child: Text(
        reached ? '卖出' : '目标 ${monitor.targetPrice.toStringAsFixed(2)}',
        style: TextStyle(
          fontSize: 11,
          fontWeight: FontWeight.w700,
          color: color,
        ),
      ),
    );
  }
}
