import 'package:flutter/material.dart';
import 'package:trading_app/core/theme/app_colors.dart';
import 'package:trading_app/core/utils/stock_launcher.dart';
import 'package:trading_app/features/radar/domain/radar_models.dart';
import 'package:trading_app/features/radar/data/radar_repository.dart';
import 'package:trading_app/features/radar/presentation/radar_list/radar_view_model.dart';
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
      final vm = context.read<RadarViewModel>();
      final changes = widget.stockCode != null
          ? await repo.getLatestChanges([widget.stockCode!])
          : await repo.getAllChanges();
      // 合并本地监控异动到详情
      final merged = <StockChange>[...changes];
      for (final c in vm.watchChanges) {
        if (widget.stockCode == null || c.stockCode == widget.stockCode) {
          if (!merged.any((e) => e.id == c.id && e.changeType == c.changeType && e.changeTime == c.changeTime)) {
            merged.add(c);
          }
        }
      }
      final todayOnly = vm.filterTodayChanges(merged);
      final filtered = vm.filterChanges(todayOnly);
      if (mounted) {
        setState(() {
          _changes = filtered;
          _loading = false;
        });
        // widget.onChangesSeen?.call(); // 已禁用，改为手动点击标记已读按钮
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

  Future<void> _openInTongHuaShun(String code) async {
    final opened = await StockLauncher.openTongHuaShun(code: code);
    if (!opened && mounted) {
      ScaffoldMessenger.of(context).showSnackBar(
        const SnackBar(content: Text('未安装同花顺或跳转失败')),
      );
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
      
      bottomNavigationBar: _buildBottomBar(),
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
    final vm = context.read<RadarViewModel>();
    return RefreshIndicator(
      onRefresh: _load,
      child: ListView.separated(
        padding: const EdgeInsets.all(12),
        itemCount: sorted.length,
        separatorBuilder: (_, __) => const SizedBox(height: 8),
        itemBuilder: (context, index) {
          final change = sorted[index];
          return StockChangeCard(
            key: ValueKey(change.id),
            change: change,
            isRead: vm.isChangeRead(change),
            onOpenTongHuaShun: widget.stockCode != null
                 ? () => _openInTongHuaShun(change.stockCode)
                 : null,
          );
        },
      ),
    );
  }

  Widget _buildBottomBar() {
    return SafeArea(
      child: Container(
        padding: const EdgeInsets.fromLTRB(12, 8, 12, 12),
        color: Colors.white,
        child: SizedBox(
          width: double.infinity,
          height: 48,
          child: ElevatedButton(
            onPressed: _markAllRead,
            style: ElevatedButton.styleFrom(
              backgroundColor: AppColors.buttonPrimary,
              shape: RoundedRectangleBorder(
                borderRadius: BorderRadius.circular(10),
              ),
            ),
            child: const Text(
              '全部标记为已读',
              style: TextStyle(fontSize: 16, color: Colors.white),
            ),
          ),
        ),
      ),
    );
  }

  Future<void> _markAllRead() async {
    if (widget.stockCode != null) {
      await context.read<RadarViewModel>().markAllChangesExposedForCode(widget.stockCode!);
    }
    if (mounted) {
      Navigator.of(context).pop();
    }
  }
}
