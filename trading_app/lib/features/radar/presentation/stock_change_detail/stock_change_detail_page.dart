import 'package:flutter/material.dart';
import 'package:trading_app/core/theme/app_colors.dart';
import 'package:trading_app/features/radar/domain/radar_models.dart';
import 'package:trading_app/features/radar/data/radar_repository.dart';
import 'package:trading_app/shared/widgets/stock_change_card.dart';
import 'package:provider/provider.dart';

/// 单只股票的异动详情页面
class StockChangeDetailPage extends StatefulWidget {
  const StockChangeDetailPage({
    super.key,
    this.stockCode,
    this.stockName,
    this.onChangesSeen,
  });

  final String? stockCode;
  final String? stockName;
  final VoidCallback? onChangesSeen;

  @override
  State<StockChangeDetailPage> createState() => _StockChangeDetailPageState();
}

class _StockChangeDetailPageState extends State<StockChangeDetailPage> {
  List<StockChange> _changes = [];
  bool _loading = true;
  String? _error;
  bool _ascending = false;

  @override
  void initState() {
    super.initState();
    _load();
  }

  Future<void> _load() async {
    setState(() => _loading = true);
    try {
      final repo = context.read<RadarRepository>();
      final changes = widget.stockCode != null
          ? await repo.getLatestChanges([widget.stockCode!])
          : await repo.getAllChanges();
      if (mounted) {
        setState(() {
          _changes = changes;
          _loading = false;
        });
        widget.onChangesSeen?.call();
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
    final title = widget.stockName != null
        ? '${widget.stockName} 异动详情'
        : '今日异动（${_changes.length}条）';
    return Scaffold(
      appBar: AppBar(
        title: Text(title),
        backgroundColor: Colors.white,
        foregroundColor: Colors.black,
        elevation: 0.5,
        actions: [
          if (widget.stockCode == null)
            IconButton(
              icon: Icon(
                _ascending ? Icons.arrow_upward : Icons.arrow_downward,
                size: 20,
              ),
              tooltip: _ascending ? '正序' : '倒序',
              onPressed: () {
                setState(() => _ascending = !_ascending);
              },
            ),
        ],
      ),
      backgroundColor: AppColors.backgroundColor,
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
    final sorted = List<StockChange>.of(_changes)
      ..sort((a, b) {
        final cmp = '${a.changeDate}${a.changeTime}'
            .compareTo('${b.changeDate}${b.changeTime}');
        return _ascending ? cmp : -cmp;
      });
    return RefreshIndicator(
      onRefresh: _load,
      child: ListView.separated(
        padding: const EdgeInsets.all(12),
        itemCount: sorted.length,
        separatorBuilder: (_, __) => const SizedBox(height: 8),
        itemBuilder: (context, index) => StockChangeCard(
          key: ValueKey(sorted[index].id),
          change: sorted[index],
          showStockName: widget.stockCode == null,
        ),
      ),
    );
  }
}
