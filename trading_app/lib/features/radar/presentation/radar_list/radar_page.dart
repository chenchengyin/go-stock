import 'package:flutter/cupertino.dart';
import 'package:provider/provider.dart';

import '../../domain/radar_models.dart';
import 'radar_view_model.dart';

class RadarPage extends StatelessWidget {
  const RadarPage({super.key});

  @override
  Widget build(BuildContext context) {
    final vm = context.watch<RadarViewModel>();
    return CupertinoPageScaffold(
      navigationBar: CupertinoNavigationBar(
        middle: const Text('交易雷达'),
        trailing: GestureDetector(
          onTap: vm.load,
          child: const Padding(
            padding: EdgeInsets.all(8),
            child: Icon(CupertinoIcons.refresh, size: 20),
          ),
        ),
      ),
      child: _buildBody(vm),
    );
  }

  Widget _buildBody(RadarViewModel vm) {
    return ListView(
      padding: const EdgeInsets.fromLTRB(16, 8, 16, 16),
      children: [
        _AlertSummary(stocks: vm.stocks.length),
        const _SectionHeader(title: '持仓 / 特别关注'),
        ...vm.stocks.map(
          (stock) => Padding(
            padding: const EdgeInsets.only(bottom: 8),
            child: _StockCard(stock: stock),
          ),
        ),
      ],
    );
  }
}

class _SectionHeader extends StatelessWidget {
  const _SectionHeader({required this.title});
  final String title;

  @override
  Widget build(BuildContext context) {
    return Padding(
      padding: const EdgeInsets.only(top: 16, bottom: 8),
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
        gradient: LinearGradient(
          colors: [
            const Color(0xffe53935).withValues(alpha: 0.08),
            CupertinoColors.white,
          ],
          begin: Alignment.topLeft,
          end: Alignment.bottomRight,
        ),
        borderRadius: BorderRadius.circular(12),
        boxShadow: [
          BoxShadow(
            color: CupertinoColors.systemGrey4.withValues(alpha: 0.25),
            blurRadius: 6,
            offset: const Offset(0, 2),
          ),
        ],
      ),
      child: Row(
        children: [
          Container(
            width: 40,
            height: 40,
            decoration: BoxDecoration(
              color: const Color(0xffe53935).withValues(alpha: 0.15),
              borderRadius: BorderRadius.circular(20),
            ),
            child: const Icon(CupertinoIcons.bell_solid,
                size: 20, color: Color(0xffe53935)),
          ),
          const SizedBox(width: 12),
          Expanded(
            child: Column(
              crossAxisAlignment: CrossAxisAlignment.start,
              children: [
                const Text('高频异动盯盘已开启',
                    style: TextStyle(
                        fontWeight: FontWeight.w700, fontSize: 15)),
                const SizedBox(height: 2),
                Text('正在监控 $stocks 只股票',
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
  const _StockCard({required this.stock});
  final WatchStock stock;

  @override
  Widget build(BuildContext context) {
    final isUp = stock.changePercent >= 0;
    final changeColor = isUp
        ? const Color(0xffe53935)
        : const Color(0xff0d904f);

    return Container(
      padding: const EdgeInsets.all(14),
      decoration: BoxDecoration(
        color: CupertinoColors.white,
        borderRadius: BorderRadius.circular(10),
        boxShadow: [
          BoxShadow(
            color: CupertinoColors.systemGrey4.withValues(alpha: 0.25),
            blurRadius: 6,
            offset: const Offset(0, 2),
          ),
        ],
      ),
      child: Row(
        crossAxisAlignment: CrossAxisAlignment.start,
        children: [
          Expanded(
            child: Column(
              crossAxisAlignment: CrossAxisAlignment.start,
              children: [
                Row(
                  children: [
                    Text(
                      stock.name,
                      style: const TextStyle(
                          fontWeight: FontWeight.w700, fontSize: 15),
                    ),
                    const SizedBox(width: 6),
                    Text(
                      stock.symbol,
                      style: const TextStyle(
                          fontSize: 12,
                          color: CupertinoColors.systemGrey),
                    ),
                  ],
                ),
                const SizedBox(height: 4),
                Text(
                  '现价 ${stock.price.toStringAsFixed(2)}',
                  style: const TextStyle(fontSize: 13),
                ),
              ],
            ),
          ),
          Column(
            crossAxisAlignment: CrossAxisAlignment.end,
            children: [
              Text(
                '${isUp ? '+' : ''}${stock.changePercent.toStringAsFixed(2)}%',
                style: TextStyle(
                  color: changeColor,
                  fontWeight: FontWeight.w700,
                  fontSize: 18,
                ),
              ),
              const SizedBox(height: 2),
              Text(
                '量比 ${stock.volumeRatio.toStringAsFixed(2)}',
                style: const TextStyle(
                  fontSize: 11,
                  color: CupertinoColors.systemGrey,
                ),
              ),
            ],
          ),
        ],
      ),
    );
  }
}
