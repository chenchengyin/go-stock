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

  Widget _buildBody(HotlistViewModel vm) {
    return ListView.builder(
      padding: const EdgeInsets.fromLTRB(16, 8, 16, 16),
      itemCount: vm.stocks.length,
      itemBuilder: (context, index) {
        final stock = vm.stocks[index];
        final isUp = stock.changePercent >= 0;
        final rankColor = index < 3
            ? const Color(0xffe53935)
            : CupertinoColors.systemGrey;

        return Container(
          margin: const EdgeInsets.only(bottom: 8),
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
            children: [
              // 排名
              Container(
                width: 32,
                height: 32,
                decoration: BoxDecoration(
                  color: rankColor.withValues(alpha: 0.1),
                  borderRadius: BorderRadius.circular(16),
                ),
                child: Center(
                  child: Text(
                    '${stock.rank}',
                    style: TextStyle(
                      fontWeight: FontWeight.w700,
                      fontSize: 13,
                      color: rankColor,
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
                    const SizedBox(height: 2),
                    Text(stock.reason,
                        style: const TextStyle(
                            fontSize: 12,
                            color: CupertinoColors.systemGrey),
                        maxLines: 1,
                        overflow: TextOverflow.ellipsis),
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
                          ? const Color(0xffe53935)
                          : const Color(0xff0d904f),
                      fontSize: 12,
                      fontWeight: FontWeight.w600,
                    ),
                  ),
                ],
              ),
            ],
          ),
        );
      },
    );
  }
}
