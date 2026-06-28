import 'package:flutter/material.dart';
import 'package:provider/provider.dart';
import 'package:trading_app/features/radar/domain/radar_models.dart';
import 'radar_view_model.dart';

class RadarPage extends StatelessWidget {
  const RadarPage({super.key});

  @override
  Widget build(BuildContext context) {
    final vm = context.watch<RadarViewModel>();
    return Scaffold(
      body: SafeArea(
        child: CustomScrollView(
          slivers: [
            // 顶部搜索栏
            SliverToBoxAdapter(
              child: Padding(
                padding: const EdgeInsets.all(16),
                child: TextField(
                  decoration: InputDecoration(
                    hintText: '搜索股票代码/名称',
                    prefixIcon: const Icon(Icons.search),
                    suffixIcon: vm.monitoredStocks.isNotEmpty
                        ? IconButton(
                            icon: const Icon(Icons.add_circle),
                            onPressed: () => _showAddStockDialog(context),
                            tooltip: '添加监控股票',
                          )
                        : null,
                    border: OutlineInputBorder(
                      borderRadius: BorderRadius.circular(12),
                    ),
                    filled: true,
                    fillColor: Colors.grey[100],
                  ),
                  onChanged: (val) => vm.searchKeyword = val,
                ),
              ),
            ),

            // 统计卡片
            if (vm.monitoredStocks.isNotEmpty)
              SliverToBoxAdapter(
                child: Padding(
                  padding: const EdgeInsets.symmetric(horizontal: 16),
                  child: Container(
                    padding: const EdgeInsets.all(16),
                    decoration: BoxDecoration(
                      color: const Color(0xfff0f5ff),
                      borderRadius: BorderRadius.circular(12),
                      border: Border.all(
                        color: const Color(0xff2364aa).withValues(alpha: 0.15),
                      ),
                    ),
                    child: Row(
                      children: [
                        Container(
                          width: 40,
                          height: 40,
                          decoration: BoxDecoration(
                            color: const Color(0xff2364aa).withValues(alpha: 0.15),
                            borderRadius: BorderRadius.circular(20),
                          ),
                          child: const Icon(Icons.radar, size: 20, color: Color(0xff2364aa)),
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

            // 异动列表
            if (vm.latestChanges.isEmpty && vm.monitoredStocks.isNotEmpty)
              const SliverToBoxAdapter(
                child: Center(
                  child: Padding(
                    padding: EdgeInsets.all(40),
                    child: Text(
                      '暂无异动',
                      style: TextStyle(color: Colors.grey),
                    ),
                  ),
                ),
              ),

            SliverPadding(
              padding: const EdgeInsets.symmetric(horizontal: 16),
              sliver: SliverList(
                delegate: SliverChildBuilderDelegate(
                  (context, index) {
                    if (index < vm.monitoredStocks.length) {
                      // 监控股票列表
                      final stock = vm.monitoredStocks[index];
                      return _buildStockCard(context, stock);
                    }
                    final changeIndex = index - vm.monitoredStocks.length;
                    if (changeIndex < vm.latestChanges.length) {
                      // 异动记录列表
                      return _buildChangeCard(vm.latestChanges[changeIndex]);
                    }
                    return null;
                  },
                  childCount: vm.monitoredStocks.length + vm.latestChanges.length,
                ),
              ),
            ),

            // 空状态
            if (vm.monitoredStocks.isEmpty)
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
                          '点击右上角 + 添加股票',
                          style: TextStyle(fontSize: 13, color: Colors.grey),
                        ),
                      ],
                    ),
                  ),
                ),
              ),
          ],
        ),
      ),
    );
  }

  Widget _buildStockCard(BuildContext context, MonitoredStock stock) {
    return Container(
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
                      stock.name,
                      style: const TextStyle(
                        fontWeight: FontWeight.w600,
                        fontSize: 16,
                      ),
                    ),
                    const SizedBox(width: 8),
                    Text(
                      stock.code,
                      style: TextStyle(
                        fontSize: 12,
                        color: Colors.grey[600],
                      ),
                    ),
                  ],
                ),
              ],
            ),
          ),
          IconButton(
            icon: const Icon(Icons.close, size: 20, color: Colors.grey),
            onPressed: () => context.read<RadarViewModel>().removeMonitoredStock(stock.code),
            tooltip: '移除监控',
          ),
        ],
      ),
    );
  }

  Widget _buildChangeCard(StockChange change) {
    final isUp = change.changeRate >= 0;
    final changeColor = isUp ? const Color(0xffe53935) : const Color(0xff0d904f);
    final typeColor = _getTypeColor(change.changeType);

    return Container(
      margin: const EdgeInsets.only(bottom: 8),
      padding: const EdgeInsets.all(16),
      decoration: BoxDecoration(
        color: Colors.white,
        borderRadius: BorderRadius.circular(12),
        border: Border.all(color: Colors.grey.shade200),
      ),
      child: Row(
        children: [
          // 左侧信息
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
                      style: TextStyle(
                        fontSize: 12,
                        color: Colors.grey[600],
                      ),
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
                    style: TextStyle(
                      fontSize: 12,
                      color: Colors.grey[600],
                    ),
                  ),
              ],
            ),
          ),
          // 右侧标签
          Column(
            crossAxisAlignment: CrossAxisAlignment.end,
            children: [
              Container(
                padding: const EdgeInsets.symmetric(horizontal: 8, vertical: 4),
                decoration: BoxDecoration(
                  color: typeColor.withValues(alpha: 0.1),
                  borderRadius: BorderRadius.circular(6),
                  border: Border.all(
                    color: typeColor.withValues(alpha: 0.3),
                  ),
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
                style: TextStyle(
                  fontSize: 11,
                  color: Colors.grey[600],
                ),
              ),
            ],
          ),
        ],
      ),
    );
  }

  Color _getTypeColor(int changeType) {
    switch (changeType) {
      case 8201: // 火箭发射
        return const Color(0xffff5722);
      case 8202: // 快速反弹
        return const Color(0xff4caf50);
      case 8204: // 加速下跌
        return const Color(0xff2196f3);
      case 8203: // 高台跳水
        return const Color(0xff9c27b0);
      case 8193: // 大笔买入
        return const Color(0xffe91e63);
      case 8194: // 大笔卖出
        return const Color(0xff795548);
      case 4: // 封涨停板
        return const Color(0xffff5722);
      case 8: // 封跌停板
        return const Color(0xff2196f3);
      case 64: // 有大买盘
        return const Color(0xff4caf50);
      case 128: // 有大卖盘
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

  void _showAddStockDialog(BuildContext context) {
    final vm = context.read<RadarViewModel>();
    showDialog(
      context: context,
      builder: (ctx) => _AddStockDialog(vm: vm),
    );
  }
}

// 添加股票的弹窗
class _AddStockDialog extends StatefulWidget {
  const _AddStockDialog({required this.vm});
  final RadarViewModel vm;

  @override
  State<_AddStockDialog> createState() => _AddStockDialogState();
}

class _AddStockDialogState extends State<_AddStockDialog> {
  final TextEditingController _controller = TextEditingController();
  List<Map<String, String>> searchResults = [];
  bool _isLoading = false;

  @override
  void dispose() {
    _controller.dispose();
    super.dispose();
  }

  Future<void> _search() async {
    final keyword = _controller.text.trim();
    if (keyword.isEmpty) {
      setState(() => searchResults = []);
      return;
    }
    setState(() => _isLoading = true);
    final results = await widget.vm.searchStocks(keyword);
    setState(() {
      searchResults = results;
      _isLoading = false;
    });
  }

  @override
  Widget build(BuildContext context) {
    return AlertDialog(
      title: const Text('添加监控股票'),
      content: SizedBox(
        width: 300,
        child: Column(
          mainAxisSize: MainAxisSize.min,
          children: [
            TextField(
              controller: _controller,
              decoration: InputDecoration(
                hintText: '输入股票代码或名称',
                suffixIcon: _isLoading
                    ? const CircularProgressIndicator(strokeWidth: 2)
                    : IconButton(
                        icon: const Icon(Icons.search),
                        onPressed: _search,
                      ),
                border: const OutlineInputBorder(),
              ),
              onChanged: (val) => _search(),
              onSubmitted: (_) => _search(),
            ),
            const SizedBox(height: 12),
            if (searchResults.isNotEmpty)
              SizedBox(
                height: 200,
                child: ListView.builder(
                  itemCount: searchResults.length,
                  itemBuilder: (context, index) {
                    final item = searchResults[index];
                    return ListTile(
                      dense: true,
                      title: Text(item['name'] ?? ''),
                      subtitle: Text(item['code'] ?? ''),
                      trailing: ElevatedButton(
                        onPressed: () async {
                          final code = item['code']!;
                          final name = item['name']!;
                          // 检查是否已关注
                          if (widget.vm.monitoredStocks.any((s) => s.code == code)) {
                            if (mounted) {
                              ScaffoldMessenger.of(context).showSnackBar(
                                SnackBar(content: Text('已关注: $name')),
                              );
                            }
                            return;
                          }
                          final success = await widget.vm.addMonitoredStock(
                            MonitoredStock(code: code, name: name),
                          );
                          if (mounted) {
                            Navigator.of(context).pop();
                            if (!success) {
                              ScaffoldMessenger.of(context).showSnackBar(
                                const SnackBar(content: Text('添加失败，请稍后重试')),
                              );
                            }
                          }
                        },
                        style: ElevatedButton.styleFrom(
                          padding: const EdgeInsets.symmetric(
                              horizontal: 12, vertical: 4),
                          minimumSize: const Size(0, 0),
                          tapTargetSize: MaterialTapTargetSize.shrinkWrap,
                        ),
                        child: const Text('添加'),
                      ),
                    );
                  },
                ),
              ),
          ],
        ),
      ),
      actions: [
        TextButton(
          onPressed: () => Navigator.of(context).pop(),
          child: const Text('取消'),
        ),
      ],
    );
  }
}
