import 'package:flutter/cupertino.dart';
import 'package:provider/provider.dart';

import '../../domain/radar_models.dart';
import '../view_models/radar_view_model.dart';

class RadarPage extends StatelessWidget {
  const RadarPage({super.key});

  @override
  Widget build(BuildContext context) {
    final vm = context.watch<RadarViewModel>();
    return CupertinoPageScaffold(
      navigationBar: CupertinoNavigationBar(
        middle: const Text('交易雷达'),
        trailing: CupertinoButton(
          padding: EdgeInsets.zero,
          onPressed: vm.load,
          child: const Icon(CupertinoIcons.refresh, size: 22),
        ),
      ),
      child: _buildBody(vm),
    );
  }

  Widget _buildBody(RadarViewModel vm) {
    return SafeArea(
      child: CustomScrollView(
        slivers: [
          // 监控摘要
          SliverToBoxAdapter(
            child: Padding(
              padding: const EdgeInsets.fromLTRB(16, 8, 16, 8),
              child: _AlertSummary(stocks: vm.stocks.length),
            ),
          ),
          // 标题
          const SliverToBoxAdapter(
            child: _SectionHeader(title: '持仓 / 特别关注'),
          ),
          // 股票列表
          SliverList.builder(
            itemCount: vm.stocks.length,
            itemBuilder: (context, index) {
              final stock = vm.stocks[index];
              final isUp = stock.changePercent >= 0;
              return Padding(
                padding: const EdgeInsets.symmetric(horizontal: 16, vertical: 6),
                child: _StockCard(stock: stock, isUp: isUp),
              );
            },
          ),
        ],
      ),
    );
  }
}

class _SectionHeader extends StatelessWidget {
  const _SectionHeader({required this.title});
  final String title;

  @override
  Widget build(BuildContext context) {
    return Padding(
      padding: const EdgeInsets.fromLTRB(16, 16, 16, 8),
      child: Text(
        title,
        style: const TextStyle(
          fontSize: 13,
          fontWeight: FontWeight.w600,
          color: CupertinoColors.systemGrey,
          letterSpacing: 0.3,
        ),
      ),
    );
  }
}

class _AlertSummary extends StatelessWidget {
  const _AlertSummary({required this.stocks});
  final int stocks;

  @override
  Widget build(BuildContext context) {
    return Container(
      padding: const EdgeInsets.all(16),
      decoration: BoxDecoration(
        color: CupertinoColors.systemGrey5,
        borderRadius: BorderRadius.circular(12),
      ),
      child: Row(
        children: [
          const Icon(CupertinoIcons.bell_solid, size: 28,
              color: CupertinoColors.systemRed),
          const SizedBox(width: 12),
          Expanded(
            child: Column(
              crossAxisAlignment: CrossAxisAlignment.start,
              children: [
                const Text('高频异动盯盘已开启',
                    style: TextStyle(
                        fontWeight: FontWeight.w700, fontSize: 15)),
                const SizedBox(height: 4),
                Text('正在监控 $stocks 只股票的价格波动、成交量异动、跳水与急拉。',
                    style: const TextStyle(
                        fontSize: 13,
                        color: CupertinoColors.systemGrey)),
              ],
            ),
          ),
        ],
      ),
    );
  }
}

class _StockCard extends StatelessWidget {
  const _StockCard({required this.stock, required this.isUp});
  final WatchStock stock;
  final bool isUp;

  @override
  Widget build(BuildContext context) {
    return Container(
      padding: const EdgeInsets.all(14),
      decoration: BoxDecoration(
        color: CupertinoColors.white,
        borderRadius: BorderRadius.circular(12),
        border: Border.all(color: CupertinoColors.systemGrey5),
      ),
      child: Column(
        crossAxisAlignment: CrossAxisAlignment.start,
        children: [
          Row(
            children: [
              Expanded(
                child: Text(
                  '${stock.name} ${stock.symbol}',
                  style: const TextStyle(
                      fontWeight: FontWeight.w700, fontSize: 15),
                ),
              ),
              Text(
                '${isUp ? '+' : ''}${stock.changePercent.toStringAsFixed(2)}%',
                style: TextStyle(
                  color: isUp
                      ? CupertinoColors.systemRed
                      : CupertinoColors.systemGreen,
                  fontWeight: FontWeight.w700,
                  fontSize: 16,
                ),
              ),
            ],
          ),
          const SizedBox(height: 8),
          Text(
              '现价 ${stock.price.toStringAsFixed(2)}  量比 ${stock.volumeRatio.toStringAsFixed(2)}'),
          if (stock.alerts.isNotEmpty) ...[
            const SizedBox(height: 10),
            Wrap(
              spacing: 8,
              runSpacing: 8,
              children: stock.alerts
                  .map(
                    (rule) => Container(
                      padding: const EdgeInsets.symmetric(
                          horizontal: 8, vertical: 4),
                      decoration: BoxDecoration(
                        color: CupertinoColors.systemGrey5,
                        borderRadius: BorderRadius.circular(6),
                      ),
                      child: Text(
                        '${rule.type.label} ${rule.windowSeconds}s / ${rule.thresholdPercent}%',
                        style: const TextStyle(fontSize: 12),
                      ),
                    ),
                  )
                  .toList(),
            ),
          ],
        ],
      ),
    );
  }
}
