import 'package:flutter/cupertino.dart';
import 'package:provider/provider.dart';

import 'hotlist_view_model.dart';

class HotlistPage extends StatelessWidget {
  const HotlistPage({super.key});

  @override
  Widget build(BuildContext context) {
    final vm = context.watch<HotlistViewModel>();
    return CupertinoPageScaffold(
      navigationBar: CupertinoNavigationBar(
        middle: const Text('全网热榜'),
        trailing: CupertinoButton(
          padding: EdgeInsets.zero,
          onPressed: vm.load,
          child: const Icon(CupertinoIcons.refresh, size: 22),
        ),
      ),
      child: _buildBody(vm),
    );
  }

  Widget _buildBody(HotlistViewModel vm) {
    return SafeArea(
      child: ListView.builder(
        padding: const EdgeInsets.fromLTRB(16, 8, 16, 16),
        itemCount: vm.stocks.length,
        itemBuilder: (context, index) {
          final stock = vm.stocks[index];
          final isUp = stock.changePercent >= 0;
          return Container(
            margin: const EdgeInsets.only(bottom: 10),
            padding: const EdgeInsets.all(14),
            decoration: BoxDecoration(
              color: CupertinoColors.white,
              borderRadius: BorderRadius.circular(12),
              border: Border.all(color: CupertinoColors.systemGrey5),
            ),
            child: Row(
              children: [
                // 排名
                Container(
                  width: 36,
                  height: 36,
                  decoration: BoxDecoration(
                    color: index < 3
                        ? CupertinoColors.systemRed.withValues(alpha: 0.12)
                        : CupertinoColors.systemGrey5,
                    borderRadius: BorderRadius.circular(18),
                  ),
                  child: Center(
                    child: Text(
                      '${stock.rank}',
                      style: TextStyle(
                        fontWeight: FontWeight.w700,
                        color: index < 3
                            ? CupertinoColors.systemRed
                            : CupertinoColors.systemGrey,
                      ),
                    ),
                  ),
                ),
                const SizedBox(width: 12),
                Expanded(
                  child: Column(
                    crossAxisAlignment: CrossAxisAlignment.start,
                    children: [
                      Text(
                        '${stock.name} ${stock.symbol}',
                        style: const TextStyle(
                            fontWeight: FontWeight.w700, fontSize: 14),
                      ),
                      const SizedBox(height: 4),
                      Text(stock.reason,
                          style: const TextStyle(
                              fontSize: 12,
                              color: CupertinoColors.systemGrey)),
                    ],
                  ),
                ),
                const SizedBox(width: 12),
                Column(
                  crossAxisAlignment: CrossAxisAlignment.end,
                  children: [
                    Text(
                      stock.heatScore.toStringAsFixed(1),
                      style: const TextStyle(
                          fontWeight: FontWeight.w700, fontSize: 14),
                    ),
                    Text(
                      '${isUp ? '+' : ''}${stock.changePercent.toStringAsFixed(2)}%',
                      style: TextStyle(
                        color: isUp
                            ? CupertinoColors.systemRed
                            : CupertinoColors.systemGreen,
                        fontSize: 12,
                      ),
                    ),
                  ],
                ),
              ],
            ),
          );
        },
      ),
    );
  }
}
