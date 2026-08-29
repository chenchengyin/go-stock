import 'package:flutter/material.dart';
import 'package:provider/provider.dart';
import 'package:trading_app/core/theme/app_colors.dart';
import 'package:trading_app/features/radar/domain/radar_models.dart';
import 'radar_view_model.dart';

class SearchResultsPanel extends StatefulWidget {
  const SearchResultsPanel({super.key});

  @override
  State<SearchResultsPanel> createState() => _SearchResultsPanelState();
}

class _SearchResultsPanelState extends State<SearchResultsPanel> {
  final Set<String> _adding = {};

  bool _isMonitored(RadarViewModel vm, String code) =>
      vm.monitoredStocks.any((s) => s.code == code);

  Future<void> _addStock(String code, String name) async {
    setState(() => _adding.add(code));
    final vm = context.read<RadarViewModel>();
    if (vm.monitoredStocks.length >= RadarViewModel.maxMonitoredCount) {
      setState(() => _adding.remove(code));
      if (mounted) {
        ScaffoldMessenger.of(context).showSnackBar(
          const SnackBar(
            content: Text('最多监控20支,大哥不要开超市呀~'),
            duration: Duration(seconds: 2),
            behavior: SnackBarBehavior.floating,
            margin: EdgeInsets.only(top: 60, left: 16, right: 16),
          ),
        );
      }
      return;
    }
    final stock = MonitoredStock(code: code, name: name);
    final ok = await vm.addMonitoredStock(stock);
    if (mounted) {
      setState(() => _adding.remove(code));
      if (ok) {
        vm.searchKeyword = '';
        if (context.mounted) {
          ScaffoldMessenger.of(context).showSnackBar(
            const SnackBar(
              content: Text('已添加监控'),
              duration: Duration(seconds: 1),
            ),
          );
        }
      } else if (mounted) {
        ScaffoldMessenger.of(context).showSnackBar(
          const SnackBar(
            content: Text('该股票已在监控列表中'),
            duration: Duration(seconds: 1),
          ),
        );
      }
    }
  }

  @override
  Widget build(BuildContext context) {
    AppColors.of(context);
    return Selector<
      RadarViewModel,
      ({List<Map<String, String>> results, bool searching})
    >(
      selector: (_, vm) =>
          (results: vm.searchResults, searching: vm.isSearching),
      builder: (_, data, __) {
        final vm = context.read<RadarViewModel>();
        return Padding(
          padding: const EdgeInsets.symmetric(horizontal: 16),
          child: Column(
            crossAxisAlignment: CrossAxisAlignment.start,
            children: [
              Padding(
                padding: const EdgeInsets.only(top: 8, bottom: 8, left: 4),
                child: Text(
                  '搜索结果 (${data.results.length})',
                  style: TextStyle(
                    fontSize: 13,
                    fontWeight: FontWeight.w500,
                    color: AppColors.textSecondary,
                  ),
                ),
              ),
              if (data.results.isEmpty)
                Container(
                  padding: const EdgeInsets.all(24),
                  alignment: Alignment.center,
                  child: Text(
                    '未找到匹配的股票',
                    style: TextStyle(
                      fontSize: 14,
                      color: AppColors.textTertiary,
                    ),
                  ),
                )
              else
                ...data.results.map(
                  (result) => Container(
                    margin: const EdgeInsets.only(bottom: 6),
                    padding: const EdgeInsets.all(12),
                    decoration: BoxDecoration(
                      color: AppColors.cardBg,
                      borderRadius: BorderRadius.circular(10),
                      border: Border.all(color: AppColors.border),
                    ),
                    child: Row(
                      children: [
                        Expanded(
                          child: Column(
                            crossAxisAlignment: CrossAxisAlignment.start,
                            children: [
                              Text(
                                result['name'] ?? '',
                                style: const TextStyle(
                                  fontWeight: FontWeight.w600,
                                  fontSize: 15,
                                ),
                              ),
                              const SizedBox(height: 2),
                              Text(
                                result['code'] ?? '',
                                style: TextStyle(
                                  fontSize: 12,
                                  color: AppColors.textSecondary,
                                ),
                              ),
                            ],
                          ),
                        ),
                        IconButton(
                          icon: _adding.contains(result['code'])
                              ? const SizedBox(
                                  width: 18,
                                  height: 18,
                                  child: CircularProgressIndicator(
                                    strokeWidth: 2,
                                  ),
                                )
                              : _isMonitored(vm, result['code'] ?? '')
                              ? Icon(
                                  Icons.check_circle,
                                  color: AppColors.success,
                                  size: 22,
                                )
                              : Icon(
                                  Icons.add_circle_outline,
                                  color: AppColors.brand,
                                ),
                          onPressed:
                              _adding.contains(result['code']) ||
                                  _isMonitored(vm, result['code'] ?? '')
                              ? null
                              : () => _addStock(
                                  result['code'] ?? '',
                                  result['name'] ?? '',
                                ),
                        ),
                      ],
                    ),
                  ),
                ),
            ],
          ),
        );
      },
    );
  }
}
