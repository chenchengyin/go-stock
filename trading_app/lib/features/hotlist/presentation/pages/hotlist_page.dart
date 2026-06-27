import 'package:flutter/material.dart';
import 'package:provider/provider.dart';

import '../view_models/hotlist_view_model.dart';

class HotlistPage extends StatelessWidget {
  const HotlistPage({super.key});

  @override
  Widget build(BuildContext context) {
    final vm = context.watch<HotlistViewModel>();
    return ListView(
      padding: const EdgeInsets.fromLTRB(16, 8, 16, 16),
      children: [
        AppBar(
          title: const Text('全网热榜'),
          actions: [
            IconButton(onPressed: vm.load, icon: const Icon(Icons.refresh), tooltip: '刷新'),
          ],
        ),
        const SizedBox(height: 8),
        for (final stock in vm.stocks)
          Card(
            margin: const EdgeInsets.only(bottom: 10),
            child: ListTile(
              leading: CircleAvatar(child: Text('${stock.rank}')),
              title: Text('${stock.name} ${stock.symbol}'),
              subtitle: Text(stock.reason),
              trailing: Column(
                mainAxisAlignment: MainAxisAlignment.center,
                crossAxisAlignment: CrossAxisAlignment.end,
                children: [
                  Text(stock.heatScore.toStringAsFixed(1), style: const TextStyle(fontWeight: FontWeight.w700)),
                  Text('${stock.changePercent >= 0 ? '+' : ''}${stock.changePercent.toStringAsFixed(2)}%'),
                ],
              ),
            ),
          ),
      ],
    );
  }
}

