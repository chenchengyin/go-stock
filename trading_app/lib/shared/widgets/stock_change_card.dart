import 'package:flutter/material.dart';
import 'package:trading_app/core/theme/app_colors.dart';
import 'package:trading_app/features/radar/domain/radar_models.dart';

/// 曝光跟踪器 — 当子组件首次进入屏幕可视区域时回调 onExposed
class ExposureTracker extends StatefulWidget {
  const ExposureTracker({
    super.key,
    required this.itemId,
    required this.alreadyExposed,
    required this.onExposed,
    required this.child,
  });

  final int itemId;
  final bool alreadyExposed;
  final ValueChanged<int> onExposed;
  final Widget child;

  @override
  State<ExposureTracker> createState() => _ExposureTrackerState();
}

class _ExposureTrackerState extends State<ExposureTracker> {
  bool _exposed = false;

  @override
  void initState() {
    super.initState();
    _exposed = widget.alreadyExposed;
    if (!_exposed) _scheduleCheck();
  }

  @override
  void didUpdateWidget(ExposureTracker old) {
    super.didUpdateWidget(old);
    if (old.itemId != widget.itemId) {
      _exposed = widget.alreadyExposed;
      if (!_exposed) _scheduleCheck();
    }
  }

  void _scheduleCheck() {
    WidgetsBinding.instance.addPostFrameCallback((_) => _checkVisibility());
  }

  void _checkVisibility() {
    if (!mounted || _exposed) return;

    final renderBox = context.findRenderObject() as RenderBox?;
    if (renderBox == null || !renderBox.attached) return;

    final scrollable = Scrollable.maybeOf(context);
    if (scrollable == null) return;

    final viewport = scrollable.context.findRenderObject() as RenderBox?;
    if (viewport == null) return;

    // 计算此组件相对于 Scrollable viewport 的位置
    final itemRect = renderBox.localToGlobal(Offset.zero, ancestor: viewport);
    final viewportHeight = viewport.size.height;

    // 组件任何一部分在视口内即认为已曝光
    if (itemRect.dy < viewportHeight && itemRect.dy + renderBox.size.height > 0) {
      _exposed = true;
      widget.onExposed(widget.itemId);
    }
  }

  @override
  Widget build(BuildContext context) => widget.child;
}

/// 异动记录卡片
class StockChangeCard extends StatelessWidget {
  const StockChangeCard({
    super.key,
    required this.change,
    this.onOpenTongHuaShun,
    this.isRead = true,
  });

  final StockChange change;
  final VoidCallback? onOpenTongHuaShun;
  final bool isRead;

  @override
  Widget build(BuildContext context) {
    final isUp = change.changeRate >= 0;
    final changeColor = isUp ? AppColors.textPriceUp : AppColors.textPriceDown;
    final typeColor = getTypeColor(change.changeType);

    return Container(
      padding: const EdgeInsets.all(14),
      decoration: BoxDecoration(
        color: AppColors.cardBg,
        borderRadius: BorderRadius.circular(10),
        border: Border.all(color: Colors.grey.shade200),
      ),
      child: Column(
        crossAxisAlignment: CrossAxisAlignment.start,
        children: [
          // 第一行：股票名称 + 未读标记 + 时间
          Row(
            crossAxisAlignment: CrossAxisAlignment.start,
            children: [
              Text(
                change.stockName,
                style: TextStyle(
                  fontSize: 18,
                  fontWeight: FontWeight.w500,
                  color: AppColors.textPrimary,
                ),
              ),
              if (!isRead)
                Container(
                  margin: const EdgeInsets.only(left: 6),
                  padding: const EdgeInsets.symmetric(horizontal: 6, vertical: 3),
                  decoration: BoxDecoration(
                    color: Colors.red,
                    borderRadius: BorderRadius.circular(4),
                  ),
                  child: const Text(
                    '新异动',
                    style: TextStyle(color: Colors.white, fontSize: 10),
                  ),
                ),
              const Spacer(),
              Text(
                '${change.changeDate} ${change.changeTime}',
                style: TextStyle(fontSize: 14, fontWeight: FontWeight.w600, color: AppColors.textPrimary),
              ),
            ],
          ),

          const SizedBox(height: 4),

          // 第二行：价格 + 涨跌幅（独占一行），右侧可选同花顺跳转
          Row(
            children: [
              Text(
                '¥${change.price.toStringAsFixed(2)}',
                style: TextStyle(
                  fontSize: 20,
                  fontWeight: FontWeight.bold,
                  color: AppColors.textSecondary,
                ),
              ),
              const SizedBox(width: 10),
              Text(
                '${isUp ? "+" : ""}${change.changeRate.toStringAsFixed(2)}%',
                style: TextStyle(
                  fontSize: 16,
                  fontWeight: FontWeight.w600,
                  color: changeColor,
                ),
              ),
              if (onOpenTongHuaShun != null) ...[
                const Spacer(),
                Material(
                  color: Colors.transparent,
                  child: InkWell(
                    onTap: onOpenTongHuaShun,
                    borderRadius: BorderRadius.circular(6),
                    child: Image.asset(
                      'assets/images/kline_button.png',
                      width: 24,
                      height: 24,
                      fit: BoxFit.contain,
                    ),
                  ),
                ),
              ],
            ],
          ),

          // 第三行：异动类型标签（左）+ 成交额/成交量（右）
          const SizedBox(height: 6),
          Row(
            children: [
              Container(
                padding: const EdgeInsets.symmetric(horizontal: 8, vertical: 3),
                decoration: BoxDecoration(
                  color: typeColor.withValues(alpha: 0.1),
                  borderRadius: BorderRadius.circular(4),
                  border: Border.all(color: typeColor.withValues(alpha: 0.3)),
                ),
                child: Text(
                  change.typeName,
                  style: TextStyle(
                    fontSize: 12,
                    fontWeight: FontWeight.w600,
                    color: typeColor,
                  ),
                ),
              ),
              const SizedBox(width: 12),
              if (change.amount > 0)
                Text(
                  '成交额 ${formatAmount(change.amount)}',
                  style: TextStyle(fontSize: 12, color: Colors.red[600]),
                ),
              if (change.amount > 0 && change.volume > 0)
                const SizedBox(width: 16),
              if (change.volume > 0)
                Text(
                  '成交量 ${formatVolume(change.volume)}',
                  style: TextStyle(
                    fontSize: 12,
                    color: Colors.red[600],
                    fontWeight: FontWeight.w400,
                  ),
                ),
            ],
          ),
        ],
      ),
    );
  }
}

// ─── 工具方法 ─────────────────────────────────────────────────

Color getTypeColor(int changeType) {
  switch (changeType) {
    case 8201:
      return const Color(0xffe91e63);
    case 8202:
      return const Color(0xff4caf50);
    case 8204:
      return const Color(0xff2196f3);
    case 8203:
      return const Color(0xff9c27b0);
    case 8193:
      return const Color(0xffe91e63);
    case 8194:
      return const Color(0xff4caf50);
    case 4:
      return const Color(0xffff5722);
    case 8:
      return const Color(0xff2196f3);
    case 64:
      return const Color.fromARGB(255, 222, 64, 64);
    case 128:
      return const Color(0xff9c27b0);
    default:
      return Colors.orange;
  }
}

String formatAmount(double amount) {
  final absAmount = amount.abs();
  final sign = amount >= 0 ? '' : '-';
  if (absAmount >= 100000000) {
    return '${sign}${(absAmount / 100000000).toStringAsFixed(2)}亿';
  } else if (absAmount >= 10000) {
    return '${sign}${(absAmount / 10000).toStringAsFixed(2)}万';
  }
  return amount.toStringAsFixed(0);
}

String formatVolume(int volume) {
  if (volume >= 100000000) {
    return '${(volume / 100000000).toStringAsFixed(2)}亿股';
  } else if (volume >= 10000) {
    return '${(volume / 10000).toStringAsFixed(2)}万股';
  }
  return '$volume股';
}
