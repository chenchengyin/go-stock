import 'package:flutter/cupertino.dart';
import 'package:provider/provider.dart';

import 'hotlist_view_model.dart';

class HotlistPage extends StatelessWidget {
  const HotlistPage({super.key});

  @override
  Widget build(BuildContext context) {
    final vm = context.watch<HotlistViewModel>();
    return CupertinoPageScaffold(
      backgroundColor: CupertinoColors.white,
      // navigationBar: CupertinoNavigationBar(
      //   middle: const Text('全网热榜'),
      //   trailing: GestureDetector(
      //     onTap: vm.load,
      //     child: const Padding(
      //       padding: EdgeInsets.all(8),
      //       child: Icon(CupertinoIcons.refresh, size: 22, color: CupertinoColors.systemBlue),
      //     ),
      //   ),
      // ),
      child: _buildBody(vm),
    );
  }

  Widget _buildBody(HotlistViewModel vm) {
    return ListView.builder(
      padding: EdgeInsets.zero,
      itemCount: vm.stocks.length,
      itemBuilder: (context, index) {
        final stock = vm.stocks[index];
        final isUp = stock.changePercent >= 0;
        final rankColor = index < 3
            ? const Color(0xffe53935)
            : CupertinoColors.systemGrey;

        return Container(
          padding: const EdgeInsets.symmetric(horizontal: 16, vertical: 12),
          decoration: const BoxDecoration(
            color: CupertinoColors.white,
            border: Border(
              bottom: BorderSide(color: CupertinoColors.systemGrey5, width: 0.5),
            ),
          ),
          child: Row(
            children: [
              // 排名
              SizedBox(
                width: 32,
                child: Text(
                  '${stock.rank}',
                  style: TextStyle(
                    fontWeight: FontWeight.w600,
                    fontSize: 15,
                    color: rankColor,
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
                          fontWeight: FontWeight.w600, fontSize: 15),
                    ),
                    const SizedBox(height: 3),
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
                        fontWeight: FontWeight.w600, fontSize: 15),
                  ),
                  Text(
                    '${isUp ? '+' : ''}${stock.changePercent.toStringAsFixed(2)}%',
                    style: TextStyle(
                      color: isUp
                          ? const Color(0xffe53935)
                          : const Color(0xff0d904f),
                      fontSize: 12,
                      fontWeight: FontWeight.w500,
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
