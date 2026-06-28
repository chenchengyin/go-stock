import 'package:flutter/material.dart';
import 'package:provider/provider.dart';
import 'package:trading_app/features/radar/domain/radar_models.dart';
import 'radar_view_model.dart';
import '../stock_change_detail/stock_change_detail_page.dart';
import '../../data/notification_util.dart';

class RadarPage extends StatelessWidget {
  const RadarPage({super.key});

  @override
  Widget build(BuildContext context) {
    final vm = context.watch<RadarViewModel>();
    return Scaffold(
      body: SafeArea(
        child: Column(
          children: [
            _buildSearchField(vm),
            Expanded(
              child: CustomScrollView(
                slivers: [
                  // 搜索结果
                  if (vm.searchKeyword.trim().isNotEmpty)
                    SliverToBoxAdapter(child: _SearchResultsPanel(vm: vm)),

                  // 统计卡片
                  if (vm.monitoredStocks.isNotEmpty &&
                      vm.searchKeyword.trim().isEmpty)
                    SliverToBoxAdapter(
                      child: Padding(
                        padding: const EdgeInsets.symmetric(horizontal: 16),
                        child: Container(
                          padding: const EdgeInsets.all(16),
                          decoration: BoxDecoration(
                            color: const Color(0xfff0f5ff),
                            borderRadius: BorderRadius.circular(12),
                            border: Border.all(
                              color: const Color(
                                0xff2364aa,
                              ).withValues(alpha: 0.15),
                            ),
                          ),
                          child: Row(
                            children: [
                              Container(
                                width: 40,
                                height: 40,
                                decoration: BoxDecoration(
                                  color: const Color(
                                    0xff2364aa,
                                  ).withValues(alpha: 0.15),
                                  borderRadius: BorderRadius.circular(20),
                                ),
                                child: const Icon(
                                  Icons.radar,
                                  size: 20,
                                  color: Color(0xff2364aa),
                                ),
                              ),
                              const SizedBox(width: 12),
                              Expanded(
                                child: Column(
                                  crossAxisAlignment: CrossAxisAlignment.start,
                                  children: [
                                    const Text(
                                      '异动监控已开启',
                                      style: TextStyle(
                                        fontWeight: FontWeight.w600,
                                        fontSize: 15,
                                      ),
                                    ),
                                    const SizedBox(height: 2),
                                    Text(
                                      '正在监控 ${vm.monitoredStocks.length} 只股票',
                                      style: const TextStyle(
                                        fontSize: 13,
                                        color: Colors.grey,
                                      ),
                                    ),
                                  ],
                                ),
                              ),
                            ],
                          ),
                        ),
                      ),
                    ),

                  // 通知测试按钮（临时）
                  // SliverToBoxAdapter(
                  //   child: Padding(
                  //     padding: const EdgeInsets.symmetric(
                  //       horizontal: 16,
                  //       vertical: 4,
                  //     ),
                  //     child: OutlinedButton.icon(
                  //       onPressed: () async {
                  //         await showStockChangeNotification(
                  //           '异动监控提醒',
                  //           '贵州茅台（sh600519）发生异动：火箭发射 🚀',
                  //         );
                  //       },
                  //       icon: const Icon(Icons.notifications_active, size: 16),
                  //       label: const Text('测试通知弹窗', style: TextStyle(fontSize: 13)),
                  //       style: OutlinedButton.styleFrom(
                  //         foregroundColor: Colors.grey[600],
                  //         side: BorderSide(color: Colors.grey[300]!),
                  //         padding: const EdgeInsets.symmetric(
                  //           horizontal: 16,
                  //           vertical: 8,
                  //         ),
                  //         minimumSize: const Size(0, 36),
                  //       ),
                  //     ),
                  //   ),
                  // ),

                  // 空状态
                  if (vm.monitoredStocks.isEmpty &&
                      vm.searchKeyword.trim().isEmpty)
                    const SliverToBoxAdapter(
                      child: Center(
                        child: Padding(
                          padding: EdgeInsets.all(60),
                          child: Column(
                            mainAxisAlignment: MainAxisAlignment.center,
                            children: [
                              Icon(Icons.radar, size: 64, color: Colors.grey),
                              SizedBox(height: 16),
                              Text(
                                '暂无监控股票',
                                style: TextStyle(
                                  fontSize: 16,
                                  color: Colors.grey,
                                  fontWeight: FontWeight.w500,
                                ),
                              ),
                              SizedBox(height: 8),
                              Text(
                                '在上方搜索框搜索股票并添加监控',
                                style: TextStyle(
                                  fontSize: 13,
                                  color: Colors.grey,
                                ),
                              ),
                            ],
                          ),
                        ),
                      ),
                    ),

                  // 监控股票列表 + 异动列表
                  if (vm.searchKeyword.trim().isEmpty)
                    SliverPadding(
                      padding: const EdgeInsets.symmetric(horizontal: 16,vertical: 8),
                      sliver: SliverList(
                        delegate: SliverChildBuilderDelegate(
                          (context, index) {
                            if (index < vm.monitoredStocks.length) {
                              return _buildStockCard(
                                context,
                                vm.monitoredStocks[index],
                              );
                            }
                            final changeIndex =
                                index - vm.monitoredStocks.length;
                            if (changeIndex < vm.latestChanges.length) {
                              return _buildChangeCard(
                                vm.latestChanges[changeIndex],
                              );
                            }
                            return null;
                          },
                          childCount:
                              vm.monitoredStocks.length +
                              vm.latestChanges.length,
                        ),
                      ),
                    ),
                ],
              ),
            ),
          ],
        ),
      ),
    );
  }

  Widget _buildSearchField(RadarViewModel vm) {
    return Padding(
      padding: const EdgeInsets.all(16),
      child: TextField(
        decoration: InputDecoration(
          hintText: '搜索股票代码/名称，点击 + 添加监控',
          prefixIcon: const Icon(Icons.search, size: 20),
          suffixIcon: vm.isSearching
              ? const Padding(
                  padding: EdgeInsets.all(12),
                  child: SizedBox(
                    width: 18,
                    height: 18,
                    child: CircularProgressIndicator(strokeWidth: 2),
                  ),
                )
              : (vm.searchKeyword.isNotEmpty
                    ? IconButton(
                        icon: const Icon(Icons.clear, size: 18),
                        onPressed: () {
                          vm.searchKeyword = '';
                        },
                      )
                    : null),
          border: OutlineInputBorder(borderRadius: BorderRadius.circular(12)),
          filled: true,
          fillColor: Colors.grey[100],
          contentPadding: const EdgeInsets.symmetric(
            horizontal: 16,
            vertical: 12,
          ),
        ),
        onChanged: (val) => vm.searchKeyword = val,
      ),
    );
  }

  Widget _buildStockCard(BuildContext context, MonitoredStock stock) {
    final vm = context.read<RadarViewModel>();
    final isUp = stock.changePercent >= 0;
    final changeColor = isUp
        ? const Color(0xffe53935)
        : const Color(0xff0d904f);
    final hasNew = vm.hasNewChanges(stock.code);

    return GestureDetector(
      onTap: () {
        Navigator.of(context).push(
          MaterialPageRoute(
            builder: (_) => StockChangeDetailPage(
              stockCode: stock.code,
              stockName: stock.name,
              onChangesSeen: () => vm.markChangesSeen(stock.code),
            ),
          ),
        );
      },
      child: Container(
        margin: const EdgeInsets.only(bottom: 8),
        padding: const EdgeInsets.all(12),
        decoration: BoxDecoration(
          color: Colors.white,
          borderRadius: BorderRadius.circular(12),
          border: Border.all(color: Colors.grey.shade200),
        ),
        child: Row(
          children: [
            const SizedBox(width: 4),
            Expanded(
              child: Column(
                crossAxisAlignment: CrossAxisAlignment.start,
                children: [
                  // 名称 + 代码
                  Row(
                    children: [
                      Text(
                        stock.name,
                        style: const TextStyle(
                          fontWeight: FontWeight.w600,
                          fontSize: 16,
                        ),
                      ),
                      const SizedBox(width: 8),
                      Text(
                        stock.code,
                        style: TextStyle(fontSize: 12, color: Colors.grey[600]),
                      ),
                    ],
                  ),
                  const SizedBox(height: 8),
                  // 价格 + 涨幅
                  Row(
                    children: [
                      Text(
                        '¥${stock.price.toStringAsFixed(2)}',
                        style: const TextStyle(
                          fontSize: 18,
                          fontWeight: FontWeight.bold,
                        ),
                      ),
                      const SizedBox(width: 10),
                      Text(
                        '${isUp ? "+" : ""}${stock.changePercent.toStringAsFixed(2)}%',
                        style: TextStyle(
                          fontSize: 15,
                          fontWeight: FontWeight.w600,
                          color: changeColor,
                        ),
                      ),
                    ],
                  ),
                  const SizedBox(height: 6),
                  // K线指标行：今开 昨收 最高 最低
                  // Row(
                  //   children: [
                  //     _buildKLineItem('今开', stock.open.toStringAsFixed(2)),
                  //     const SizedBox(width: 12),
                  //     _buildKLineItem('昨收', stock.preClose.toStringAsFixed(2)),
                  //     const SizedBox(width: 12),
                  //     _buildKLineItem('最高', stock.high.toStringAsFixed(2)),
                  //     const SizedBox(width: 12),
                  //     _buildKLineItem('最低', stock.low.toStringAsFixed(2)),
                  //   ],
                  // ),
                  const SizedBox(height: 4),
                  // 成交额
                  if (stock.amount > 0)
                    Text(
                      '成交额 ${_formatAmount(stock.amount)}',
                      style: TextStyle(fontSize: 13, color: Colors.grey[600]),
                    ),
                ],
              ),
            ),
            // 小红点
            if (hasNew)
              Container(
                // width: 8,
                // height: 8,
                margin: const EdgeInsets.only(right: 4),
                decoration: const BoxDecoration(
                  color: Colors.red,
                  shape: BoxShape.circle,
                ),
                padding: EdgeInsets.all(5),
                child: Text(
                  "异动",
                  style: TextStyle(color: Colors.white, fontSize: 6),
                ),
              ),
            IconButton(
              icon: const Icon(
                Icons.chevron_right,
                size: 20,
                color: Colors.grey,
              ),
              onPressed: () {
                Navigator.of(context).push(
                  MaterialPageRoute(
                    builder: (_) => StockChangeDetailPage(
                      stockCode: stock.code,
                      stockName: stock.name,
                      onChangesSeen: () => vm.markChangesSeen(stock.code),
                    ),
                  ),
                );
              },
              tooltip: '查看异动详情',
            ),
            IconButton(
              icon: const Icon(Icons.close, size: 18, color: Colors.grey),
              onPressed: () => vm.removeMonitoredStock(stock.code),
              tooltip: '移除监控',
            ),
          ],
        ),
      ),
    );
  }

  Widget _buildChangeCard(StockChange change) {
    final isUp = change.changeRate >= 0;
    final changeColor = isUp
        ? const Color(0xffe53935)
        : const Color(0xff0d904f);
    final typeColor = _getTypeColor(change.changeType);

    return GestureDetector(
      onTap: () => _showChangeNotification(change),
      child: Container(
        margin: const EdgeInsets.only(bottom: 8),
        padding: const EdgeInsets.all(16),
        decoration: BoxDecoration(
          color: Colors.white,
          borderRadius: BorderRadius.circular(12),
          border: Border.all(color: Colors.grey.shade200),
        ),
        child: Row(
          children: [
            Expanded(
              child: Column(
                crossAxisAlignment: CrossAxisAlignment.start,
                children: [
                  Row(
                    children: [
                      Text(
                        change.stockName,
                        style: const TextStyle(
                          fontWeight: FontWeight.w600,
                          fontSize: 15,
                        ),
                      ),
                      const SizedBox(width: 8),
                      Text(
                        change.stockCode,
                        style: TextStyle(fontSize: 12, color: Colors.grey[600]),
                      ),
                    ],
                  ),
                  const SizedBox(height: 6),
                  Row(
                    children: [
                      Text(
                        '¥${change.price.toStringAsFixed(2)}',
                        style: const TextStyle(fontSize: 14),
                      ),
                      const SizedBox(width: 8),
                      Text(
                        '${isUp ? "+" : ""}${change.changeRate.toStringAsFixed(2)}%',
                        style: TextStyle(
                          fontSize: 14,
                          fontWeight: FontWeight.w600,
                          color: changeColor,
                        ),
                      ),
                    ],
                  ),
                  const SizedBox(height: 4),
                  if (change.amount > 0)
                    Text(
                      '成交额 ${_formatAmount(change.amount)}',
                      style: TextStyle(fontSize: 12, color: Colors.grey[600]),
                    ),
                ],
              ),
            ),
            Column(
              crossAxisAlignment: CrossAxisAlignment.end,
              children: [
                Container(
                  padding: const EdgeInsets.symmetric(
                    horizontal: 8,
                    vertical: 4,
                  ),
                  decoration: BoxDecoration(
                    color: typeColor.withValues(alpha: 0.1),
                    borderRadius: BorderRadius.circular(6),
                    border: Border.all(color: typeColor.withValues(alpha: 0.3)),
                  ),
                  child: Text(
                    change.typeName,
                    style: TextStyle(
                      fontSize: 11,
                      fontWeight: FontWeight.w600,
                      color: typeColor,
                    ),
                  ),
                ),
                const SizedBox(height: 4),
                Text(
                  change.changeTime,
                  style: TextStyle(fontSize: 11, color: Colors.grey[600]),
                ),
                const SizedBox(height: 4),
                Icon(
                  Icons.notifications_active_outlined,
                  size: 16,
                  color: Colors.grey[500],
                ),
              ],
            ),
          ],
        ),
      ),
    );
  }

  Future<void> _showChangeNotification(StockChange change) async {
    final isUp = change.changeRate >= 0;
    await showStockChangeNotification(
      '异动监控提醒',
      '${change.stockName}（${change.stockCode}）${change.typeName}，'
          '现价 ${change.price.toStringAsFixed(2)}，'
          '${isUp ? "+" : ""}${change.changeRate.toStringAsFixed(2)}%',
    );
  }

  Color _getTypeColor(int changeType) {
    switch (changeType) {
      case 8201:
        return const Color(0xffff5722);
      case 8202:
        return const Color(0xff4caf50);
      case 8204:
        return const Color(0xff2196f3);
      case 8203:
        return const Color(0xff9c27b0);
      case 8193:
        return const Color(0xffe91e63);
      case 8194:
        return const Color(0xff795548);
      case 4:
        return const Color(0xffff5722);
      case 8:
        return const Color(0xff2196f3);
      case 64:
        return const Color(0xff4caf50);
      case 128:
        return const Color(0xff9c27b0);
      default:
        return Colors.orange;
    }
  }

  String _formatAmount(double amount) {
    if (amount >= 100000000) {
      return '${(amount / 100000000).toStringAsFixed(2)}亿';
    } else if (amount >= 10000) {
      return '${(amount / 10000).toStringAsFixed(2)}万';
    }
    return amount.toStringAsFixed(0);
  }
}

/// 搜索结果显示面板
class _SearchResultsPanel extends StatefulWidget {
  const _SearchResultsPanel({required this.vm});

  final RadarViewModel vm;

  @override
  State<_SearchResultsPanel> createState() => _SearchResultsPanelState();
}

class _SearchResultsPanelState extends State<_SearchResultsPanel> {
  /// 记录当前操作中的股票code，用于显示加载状态
  final Set<String> _addingCodes = {};

  @override
  Widget build(BuildContext context) {
    final results = widget.vm.searchResults;
    final vm = widget.vm;

    if (results.isEmpty && !vm.isSearching) {
      return const Padding(
        padding: EdgeInsets.symmetric(horizontal: 16),
        child: Card(
          child: Padding(
            padding: EdgeInsets.all(24),
            child: Center(
              child: Text('未找到匹配的股票', style: TextStyle(color: Colors.grey)),
            ),
          ),
        ),
      );
    }

    return Padding(
      padding: const EdgeInsets.symmetric(horizontal: 16),
      child: Card(
        margin: EdgeInsets.zero,
        elevation: 2,
        shape: RoundedRectangleBorder(borderRadius: BorderRadius.circular(12)),
        clipBehavior: Clip.antiAlias,
        child: Column(
          mainAxisSize: MainAxisSize.min,
          crossAxisAlignment: CrossAxisAlignment.start,
          children: [
            Padding(
              padding: const EdgeInsets.fromLTRB(16, 12, 16, 4),
              child: Text(
                '搜索结果（${results.length}）',
                style: const TextStyle(
                  fontSize: 13,
                  color: Colors.grey,
                  fontWeight: FontWeight.w500,
                ),
              ),
            ),
            ...results.map((item) => _buildResultItem(item)),
          ],
        ),
      ),
    );
  }

  Widget _buildResultItem(Map<String, String> item) {
    final code = item['code']!;
    final name = item['name']!;
    final isAlreadyAdded = widget.vm.monitoredStocks.any((s) => s.code == code);
    final isLoading = _addingCodes.contains(code);

    return InkWell(
      onTap: isAlreadyAdded || isLoading ? null : () => _addStock(code, name),
      child: Padding(
        padding: const EdgeInsets.symmetric(horizontal: 16, vertical: 10),
        child: Row(
          children: [
            Expanded(
              child: Column(
                crossAxisAlignment: CrossAxisAlignment.start,
                children: [
                  Text(
                    name,
                    style: const TextStyle(
                      fontWeight: FontWeight.w600,
                      fontSize: 15,
                    ),
                  ),
                  const SizedBox(height: 2),
                  Text(
                    code,
                    style: TextStyle(fontSize: 12, color: Colors.grey[600]),
                  ),
                ],
              ),
            ),
            if (isAlreadyAdded)
              Container(
                padding: const EdgeInsets.symmetric(
                  horizontal: 10,
                  vertical: 4,
                ),
                decoration: BoxDecoration(
                  color: Colors.green.withValues(alpha: 0.1),
                  borderRadius: BorderRadius.circular(6),
                ),
                child: const Text(
                  '已添加',
                  style: TextStyle(
                    fontSize: 12,
                    color: Colors.green,
                    fontWeight: FontWeight.w500,
                  ),
                ),
              )
            else
              SizedBox(
                height: 32,
                child: ElevatedButton.icon(
                  onPressed: isLoading ? null : () => _addStock(code, name),
                  icon: isLoading
                      ? const SizedBox(
                          width: 14,
                          height: 14,
                          child: CircularProgressIndicator(strokeWidth: 2),
                        )
                      : const Icon(Icons.add, size: 16),
                  label: Text(isLoading ? '添加中' : '添加'),
                  style: ElevatedButton.styleFrom(
                    padding: const EdgeInsets.symmetric(
                      horizontal: 12,
                      vertical: 0,
                    ),
                    textStyle: const TextStyle(fontSize: 13),
                    shape: RoundedRectangleBorder(
                      borderRadius: BorderRadius.circular(6),
                    ),
                  ),
                ),
              ),
          ],
        ),
      ),
    );
  }

  Future<void> _addStock(String code, String name) async {
    setState(() => _addingCodes.add(code));
    try {
      final success = await widget.vm.addMonitoredStock(
        MonitoredStock(code: code, name: name),
      );
      if (mounted) {
        if (success) {
          ScaffoldMessenger.of(context).showSnackBar(
            SnackBar(
              content: Text('已添加: $name'),
              duration: const Duration(seconds: 1),
            ),
          );
        } else {
          ScaffoldMessenger.of(context).showSnackBar(
            SnackBar(
              content: Text('添加失败，请稍后重试'),
              duration: const Duration(seconds: 2),
              backgroundColor: Colors.red,
            ),
          );
        }
      }
    } finally {
      if (mounted) {
        setState(() => _addingCodes.remove(code));
      }
    }
  }
}
