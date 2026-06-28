import 'package:flutter/material.dart';
import 'package:trading_app/features/radar/domain/radar_models.dart';
import 'package:trading_app/features/radar/data/radar_repository.dart';
import 'package:provider/provider.dart';

/// 单只股票的异动详情页面
class StockChangeDetailPage extends StatefulWidget {
  const StockChangeDetailPage({
    super.key,
    required this.stockCode,
    required this.stockName,
    required this.onChangesSeen,
  });

  final String stockCode;
  final String stockName;
  final VoidCallback onChangesSeen;

  @override
  State<StockChangeDetailPage> createState() => _StockChangeDetailPageState();
}

class _StockChangeDetailPageState extends State<StockChangeDetailPage> {
  List<StockChange> _changes = [];
  bool _loading = true;
  String? _error;

  @override
  void initState() {
    super.initState();
    _load();
  }

  Future<void> _load() async {
    setState(() => _loading = true);
    try {
      final repo = context.read<RadarRepository>();
      final changes = await repo.getLatestChanges([widget.stockCode]);
      if (mounted) {
        setState(() {
          _changes = changes;
          _loading = false;
        });
        // 标记已读
        widget.onChangesSeen();
      }
    } catch (e) {
      if (mounted) {
        setState(() {
          _error = e.toString();
          _loading = false;
        });
      }
    }
  }

  @override
  Widget build(BuildContext context) {
    return Scaffold(
      appBar: AppBar(
        title: Text('${widget.stockName} 异动详情'),
        backgroundColor: Colors.white,
        foregroundColor: Colors.black,
        elevation: 0.5,
      ),
      backgroundColor: const Color(0xfff5f5f5),
      body: _buildBody(),
    );
  }

  Widget _buildBody() {
    if (_loading) {
      return const Center(child: CircularProgressIndicator());
    }
    if (_error != null) {
      return Center(
        child: Column(
          mainAxisSize: MainAxisSize.min,
          children: [
            const Icon(Icons.error_outline, size: 48, color: Colors.grey),
            const SizedBox(height: 12),
            Text('加载失败', style: TextStyle(color: Colors.grey[600])),
            const SizedBox(height: 8),
            ElevatedButton(onPressed: _load, child: const Text('重试')),
          ],
        ),
      );
    }
    if (_changes.isEmpty) {
      return const Center(
        child: Column(
          mainAxisSize: MainAxisSize.min,
          children: [
            Icon(Icons.check_circle_outline, size: 48, color: Colors.green),
            SizedBox(height: 12),
            Text('暂无异动记录', style: TextStyle(color: Colors.grey, fontSize: 15)),
          ],
        ),
      );
    }
    return RefreshIndicator(
      onRefresh: _load,
      child: ListView.separated(
        padding: const EdgeInsets.all(12),
        itemCount: _changes.length,
        separatorBuilder: (_, __) => const SizedBox(height: 8),
        itemBuilder: (context, index) => _buildChangeItem(_changes[index]),
      ),
    );
  }

  Widget _buildChangeItem(StockChange change) {
    final isUp = change.changeRate >= 0;
    final changeColor = isUp ? const Color(0xffe53935) : const Color(0xff0d904f);
    final typeColor = _getTypeColor(change.changeType);

    return Container(
      padding: const EdgeInsets.all(14),
      decoration: BoxDecoration(
        color: Colors.white,
        borderRadius: BorderRadius.circular(10),
        border: Border.all(color: Colors.grey.shade200),
      ),
      child: Column(
        crossAxisAlignment: CrossAxisAlignment.start,
        children: [
          // 异动类型标签 + 时间
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
              const Spacer(),
              Text(
                '${change.changeDate} ${change.changeTime}',
                style: TextStyle(fontSize: 11, color: Colors.grey[500]),
              ),
            ],
          ),
          const SizedBox(height: 10),
          // 价格 + 涨跌幅
          Row(
            children: [
              Text(
                '¥${change.price.toStringAsFixed(2)}',
                style: const TextStyle(
                  fontSize: 20,
                  fontWeight: FontWeight.bold,
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
            ],
          ),
          const SizedBox(height: 6),
          // 成交额 + 成交量
          Row(
            children: [
              if (change.amount > 0)
                Text(
                  '成交额 ${_formatAmount(change.amount)}',
                  style: TextStyle(fontSize: 12, color: Colors.grey[600]),
                ),
              if (change.amount > 0 && change.volume > 0)
                const SizedBox(width: 16),
              if (change.volume > 0)
                Text(
                  '成交量 ${_formatVolume(change.volume)}',
                  style: TextStyle(fontSize: 12, color: Colors.grey[600]),
                ),
            ],
          ),
        ],
      ),
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

  String _formatVolume(int volume) {
    if (volume >= 100000000) {
      return '${(volume / 100000000).toStringAsFixed(2)}亿股';
    } else if (volume >= 10000) {
      return '${(volume / 10000).toStringAsFixed(2)}万股';
    }
    return '$volume股';
  }
}
