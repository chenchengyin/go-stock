import 'package:flutter/material.dart';
import 'package:trading_app/core/theme/app_colors.dart';
import 'package:trading_app/features/radar/data/red_kline_repository.dart';
import 'strategy_card.dart';
import 'strategy_stock_chips.dart';
import 'stock_code_copy_button.dart';
import 't0_strategy_view_model.dart';

enum StrategyCardVariant { main, red, gold, purple, blue }

/// Stock-specific composition; charts and extra sections are supplied by callers.
class StrategyStockCard extends StatelessWidget {
  const StrategyStockCard({
    super.key,
    required this.stock,
    required this.kind,
    this.preview = false,
    this.tailPreview = false,
    this.selectedDate,
    this.selectedDateIsToday = false,
    this.redKlineWindow,
    this.historyDate,
    this.charts,
    this.sections = const [],
    this.onTap,
  });
  final T0StrategyStock stock;
  final StrategyCardVariant kind;
  final bool preview;
  final bool tailPreview;
  final String? selectedDate;
  final bool selectedDateIsToday;
  final RedKlineWindow? redKlineWindow;
  final String? historyDate;
  final Widget? charts;
  final List<Widget> sections;
  final VoidCallback? onTap;

  @override
  Widget build(BuildContext context) {
    final historyDate = this.historyDate;
    final openUp = stock.openGap >= 0;
    final openColor = openUp ? AppColors.textPriceUp : AppColors.textPriceDown;
    final historicalClosePercent = _historicalClosePercent(
      stock,
      redKlineWindow,
      selectedDate,
    );
    final displayPercent = kind == StrategyCardVariant.red
        ? selectedDateIsToday
              ? stock.liveChangePercent ?? stock.closeRet
              : historicalClosePercent
        : stock.closeRet;
    final displayPercentColor = displayPercent >= 0
        ? AppColors.textPriceUp
        : AppColors.textPriceDown;
    final tagLabel = stock.tag == '涨停破板'
        ? (kind == StrategyCardVariant.purple ? '皮' : 'P')
        : (stock.tag == '前一天跌停'
              ? '足'
              : (stock.tag == '前一天大阴线' ? '月' : stock.tag));
    final livePct = stock.liveChangePercent ?? 0.0;
    final liveUp = livePct >= 0;
    final liveColor = liveUp ? AppColors.textPriceUp : AppColors.textPriceDown;
    Widget buildStockName() => Text(
      stock.stockName,
      maxLines: 1,
      overflow: TextOverflow.ellipsis,
      style: TextStyle(
        fontWeight: FontWeight.w600,
        fontSize: 14,
        color: stock.hasTechBlueDisplayRuleHit
            ? AppColors.info
            : stock.hasStrongDisplayRuleHit
            ? AppColors.errorStrong
            : stock.hasDisplayRuleHit
            ? AppColors.error
            : AppColors.textPrimary,
      ),
    );

    List<Widget> buildLeadingItems() => <Widget>[
      if (historyDate != null && historyDate.isNotEmpty)
        Text(
          historyDate,
          style: TextStyle(fontSize: 11, color: AppColors.textTertiary),
        ),
      buildStockName(),
      if (historyDate != null && historyDate.isNotEmpty)
        Text(
          '${displayPercent >= 0 ? "+" : ""}${displayPercent.toStringAsFixed(2)}%',
          style: TextStyle(
            fontSize: 13,
            fontWeight: FontWeight.w500,
            color: displayPercentColor,
          ),
        ),
      if (stock.tag.isNotEmpty)
        Text(
          '[$tagLabel]',
          style: TextStyle(
            fontSize: 11,
            fontWeight: FontWeight.w600,
            color: stock.tag == '涨停破板' ? AppColors.tagRed : AppColors.tagGreen,
          ),
        ),
      if (!preview && stock.sellMonitor == null && historyDate == null)
        Text(
          '${displayPercent >= 0 ? "+" : ""}${displayPercent.toStringAsFixed(2)}%',
          style: TextStyle(
            fontSize: 13,
            fontWeight: FontWeight.w500,
            color: displayPercentColor,
          ),
        ),
      Text(
        stock.rawCode,
        style: TextStyle(fontSize: 11, color: AppColors.textSecondary),
      ),
      StrategyPatternChip(stock: stock),
      if (stock.sellMonitor != null)
        StrategySellTargetChip(monitor: stock.sellMonitor!),
      StrategyReferenceChips(stock: stock),
      if (kind == StrategyCardVariant.red)
        StrategyRedRuleChips(
          stock: stock,
          redKlineWindow: redKlineWindow,
          selectedDate: selectedDate,
        ),
      if (kind != StrategyCardVariant.red) StrategyRatingChip(stock: stock),
      if (kind == StrategyCardVariant.gold)
        Text(
          stock.strongGoldSignal
              ? '强金·${stock.patternScore.toStringAsFixed(1)}'
              : '评分${stock.patternScore.toStringAsFixed(1)}',
          style: TextStyle(
            fontSize: 11,
            fontWeight: FontWeight.w600,
            color: stock.strongGoldSignal
                ? AppColors.errorStrong
                : AppColors.tagOrange,
          ),
        ),
    ];

    List<Widget> buildTrailingItems() => <Widget>[
      if (preview) ...[
        Text(
          tailPreview ? '尾盘候选（待收盘确认）' : '竞价预览（未确认）',
          style: TextStyle(fontSize: 11, color: AppColors.textSecondary),
        ),
        Text(
          '${liveUp ? "+" : ""}${livePct.toStringAsFixed(2)}%',
          style: TextStyle(
            fontSize: 14,
            fontWeight: FontWeight.bold,
            color: liveColor,
          ),
        ),
      ] else if (stock.sellMonitor != null)
        Text(
          'T0收盘 ${stock.sellMonitor!.entryClose.toStringAsFixed(2)} · 目标 ${stock.sellMonitor!.targetPrice.toStringAsFixed(2)}',
          style: TextStyle(
            fontSize: 12,
            fontWeight: FontWeight.bold,
            color: stock.sellMonitor!.triggered
                ? AppColors.errorStrong
                : AppColors.textSecondary,
          ),
        )
      else
        Text(
          '开盘${openUp ? "+" : ""}${stock.openGap.toStringAsFixed(2)}%',
          style: TextStyle(
            fontSize: 14,
            fontWeight: FontWeight.bold,
            color: openColor,
          ),
        ),
      StockCodeCopyButton(code: stock.rawCode),
    ];

    return StrategyCard(
      onTap: onTap,
      header: StrategyCardHeader(
        information: buildLeadingItems(),
        actions: buildTrailingItems(),
        wrap: kind == StrategyCardVariant.red && stock.redRuleHits.isNotEmpty,
      ),
      sections: [if (charts != null) charts!, ...sections],
    );
  }
}

double _historicalClosePercent(
  T0StrategyStock stock,
  RedKlineWindow? window,
  String? selectedDate,
) {
  if (window != null &&
      (selectedDate == null || window.date == selectedDate) &&
      window.anchorIndex >= 0 &&
      window.anchorIndex < window.bars.length) {
    final anchor = window.bars[window.anchorIndex];
    if (anchor.date == window.date && anchor.close > 0) {
      final previousClose = window.anchorIndex > 0
          ? window.bars[window.anchorIndex - 1].close
          : stock.prevClose;
      if (previousClose > 0) {
        return (anchor.close / previousClose - 1) * 100;
      }
    }
  }
  return stock.closeRet;
}
