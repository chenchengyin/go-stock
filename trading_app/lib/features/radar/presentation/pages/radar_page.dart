import 'package:flutter/material.dart';
import 'package:provider/provider.dart';

import '../../../../shared/widgets/section_header.dart';
import '../../domain/radar_models.dart';
import '../view_models/radar_view_model.dart';

class RadarPage extends StatelessWidget {
  const RadarPage({super.key});

  @override
  Widget build(BuildContext context) {
    final vm = context.watch<RadarViewModel>();
    return CustomScrollView(
      slivers: [
        SliverAppBar(
          title: const Text('交易雷达'),
          floating: true,
          actions: [
            IconButton(
              tooltip: '刷新',
              onPressed: vm.load,
              icon: const Icon(Icons.refresh),
            ),
          ],
        ),
        SliverToBoxAdapter(
          child: Padding(
            padding: const EdgeInsets.fromLTRB(16, 8, 16, 8),
            child: _AlertSummary(stocks: vm.stocks.length),
          ),
        ),
        const SliverToBoxAdapter(child: SectionHeader(title: '持仓 / 特别关注')),
        SliverList.builder(
          itemCount: vm.stocks.length,
          itemBuilder: (context, index) {
            final stock = vm.stocks[index];
            final isUp = stock.changePercent >= 0;
            return Padding(
              padding: const EdgeInsets.symmetric(horizontal: 16, vertical: 6),
              child: Card(
                child: Padding(
                  padding: const EdgeInsets.all(14),
                  child: Column(
                    crossAxisAlignment: CrossAxisAlignment.start,
                    children: [
                      Row(
                        children: [
                          Expanded(
                            child: Text(
                              '${stock.name} ${stock.symbol}',
                              style: Theme.of(context).textTheme.titleMedium?.copyWith(fontWeight: FontWeight.w700),
                            ),
                          ),
                          Text(
                            '${isUp ? '+' : ''}${stock.changePercent.toStringAsFixed(2)}%',
                            style: TextStyle(
                              color: isUp ? const Color(0xffd93025) : const Color(0xff188038),
                              fontWeight: FontWeight.w700,
                            ),
                          ),
                        ],
                      ),
                      const SizedBox(height: 8),
                      Text('现价 ${stock.price.toStringAsFixed(2)}  量比 ${stock.volumeRatio.toStringAsFixed(2)}'),
                      const SizedBox(height: 10),
                      Wrap(
                        spacing: 8,
                        runSpacing: 8,
                        children: stock.alerts
                            .map(
                              (rule) => Chip(
                                label: Text('${rule.type.label} ${rule.windowSeconds}s / ${rule.thresholdPercent}%'),
                                visualDensity: VisualDensity.compact,
                              ),
                            )
                            .toList(),
                      ),
                    ],
                  ),
                ),
              ),
            );
          },
        ),
      ],
    );
  }
}

class _AlertSummary extends StatelessWidget {
  const _AlertSummary({required this.stocks});

  final int stocks;

  @override
  Widget build(BuildContext context) {
    return Card(
      child: Padding(
        padding: const EdgeInsets.all(16),
        child: Row(
          children: [
            const Icon(Icons.notifications_active_outlined, size: 32),
            const SizedBox(width: 12),
            Expanded(
              child: Column(
                crossAxisAlignment: CrossAxisAlignment.start,
                children: [
                  Text('高频异动盯盘已开启', style: Theme.of(context).textTheme.titleMedium?.copyWith(fontWeight: FontWeight.w700)),
                  const SizedBox(height: 4),
                  Text('正在监控 $stocks 只股票的价格波动、成交量异动、跳水与急拉。'),
                ],
              ),
            ),
          ],
        ),
      ),
    );
  }
}
