import 'package:flutter/material.dart';
import 'package:provider/provider.dart';
import 'package:trading_app/core/theme/app_colors.dart';
import 'package:trading_app/core/utils/stock_launcher.dart';
import 'package:trading_app/features/radar/domain/radar_models.dart';
import 'package:trading_app/features/radar/domain/pankou_analyzer.dart';
import 'package:trading_app/shared/widgets/stock_change_card.dart';
import 'radar_view_model.dart';
import '../stock_change_detail/stock_change_detail_page.dart';

class RadarPage extends StatefulWidget {
  const RadarPage({super.key});

  @override
  State<RadarPage> createState() => _RadarPageState();
}

class _RadarPageState extends State<RadarPage> with TickerProviderStateMixin {
  late final TabController _tabController;

  // 各 tab 的排序状态
  bool _watchAscending = false;
  bool _allAscending = false;

  // 是否已触发延迟加载
  bool _watchLoaded = false;
  bool _allLoaded = false;

  @override
  void initState() {
    super.initState();
    _tabController = TabController(length: 3, vsync: this);
    _tabController.addListener(_onTabChanged);
  }

  void _onTabChanged() {
    if (!_tabController.indexIsChanging) return;
    _lazyLoadIfNeeded(_tabController.index);
  }

  void _lazyLoadIfNeeded(int index) {
    final vm = context.read<RadarViewModel>();
    if (index == 1 && !_watchLoaded) {
      _watchLoaded = true;
      vm.loadWatchChanges();
    } else if (index == 2 && !_allLoaded) {
      _allLoaded = true;
      vm.loadAllChanges();
    }
  }

  @override
  void dispose() {
    _tabController.removeListener(_onTabChanged);
    _tabController.dispose();
    super.dispose();
  }

  @override
  Widget build(BuildContext context) {
    final vm = context.watch<RadarViewModel>();
    return DefaultTabController(
      length: 3,
      child: Scaffold(
        appBar: AppBar(
          title: const Text('雷达'),
          backgroundColor: Colors.white,
          foregroundColor: Colors.black,
          elevation: 0.5,
          bottom: PreferredSize(
            preferredSize: const Size.fromHeight(44),
            child: Container(
              color: Colors.white,
              child: TabBar(
                controller: _tabController,
                labelColor: const Color(0xff2364aa),
                unselectedLabelColor: Colors.grey,
                indicatorColor: const Color(0xff2364aa),
                labelStyle: const TextStyle(
                  fontWeight: FontWeight.w600,
                  fontSize: 15,
                ),
                onTap: _lazyLoadIfNeeded,
                tabs: const [
                  Tab(text: '监控股票'),
                  Tab(text: '持仓异动'),
                  Tab(text: '全市场'),
                ],
              ),
            ),
          ),
        ),
        backgroundColor: AppColors.backgroundColor,
        body: TabBarView(
          controller: _tabController,
          children: [
            // ─── Tab ① 监控股票 ──────────────────────────────
            _buildStockTab(vm),
            // ─── Tab ② 持仓异动 ──────────────────────────────
            _buildChangeTab(
              changes: vm.watchChanges,
              loading: vm.watchLoading,
              ascending: _watchAscending,
              onToggleSort: () =>
                  setState(() => _watchAscending = !_watchAscending),
              emptyText: '暂无持仓异动',
            ),
            // ─── Tab ③ 全市场 ────────────────────────────────
            _buildChangeTab(
              changes: vm.allChanges,
              loading: vm.allLoading,
              ascending: _allAscending,
              onToggleSort: () =>
                  setState(() => _allAscending = !_allAscending),
              emptyText: '暂无全市场异动',
            ),
          ],
        ),
      ),
    );
  }

  // ─── Tab ① 监控股票 ─────────────────────────────────────

  Widget _buildStockTab(RadarViewModel vm) {
    return SafeArea(
      top: false,
      child: Column(
        children: [
          _buildSearchField(vm),
          Expanded(
            child: CustomScrollView(
              slivers: [
                if (vm.searchKeyword.trim().isNotEmpty)
                  SliverToBoxAdapter(child: _SearchResultsPanel(vm: vm)),

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
                              style: TextStyle(fontSize: 13, color: Colors.grey),
                            ),
                          ],
                        ),
                      ),
                    ),
                  ),

                // 监控股票列表
                if (vm.searchKeyword.trim().isEmpty)
                  SliverPadding(
                    padding: const EdgeInsets.symmetric(
                      horizontal: 16,
                      vertical: 8,
                    ),
                    sliver: SliverList(
                      delegate: SliverChildBuilderDelegate((context, index) {
                        if (index < vm.monitoredStocks.length) {
                          return _buildStockCard(
                            context,
                            vm.monitoredStocks[index],
                          );
                        }
                        return null;
                      }, childCount: vm.monitoredStocks.length),
                    ),
                  ),
              ],
            ),
          ),
        ],
      ),
    );
  }

  Widget _buildSearchField(RadarViewModel vm) {
    return Padding(
      padding: const EdgeInsets.all(16),
      child: TextField(
        decoration: InputDecoration(
          hintText: '搜索股票代码/名称后按确定，点击 + 添加监控',
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
    final changeColor =
        isUp ? AppColors.textPriceUp : AppColors.textPriceDown;
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
          color: AppColors.cardBg,
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
                  // 盘口语言标签
                  _buildPanKouTags(stock),
                  const SizedBox(height: 4),
                  // 成交额
                  if (stock.amount > 0)
                    Text(
                      '成交额 ${formatAmount(stock.amount)}',
                      style: TextStyle(fontSize: 13, color: Colors.grey[600]),
                    ),
                ],
              ),
            ),
            // 小红点
            if (hasNew)
              Container(
                margin: const EdgeInsets.only(right: 4),
                decoration: const BoxDecoration(
                  color: Colors.red,
                  shape: BoxShape.circle,
                ),
                padding: const EdgeInsets.all(5),
                child: const Text(
                  "异动",
                  style: TextStyle(color: Colors.white, fontSize: 6),
                ),
              ),
            IconButton(
              icon: const Icon(Icons.open_in_new, size: 18, color: Colors.grey),
              onPressed: () => _openInTongHuaShun(context, stock.code),
              tooltip: '同花顺打开',
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

  Future<void> _openInTongHuaShun(BuildContext context, String code) async {
    final opened = await StockLauncher.openTongHuaShun(code: code);
    if (!opened && context.mounted) {
      ScaffoldMessenger.of(context).showSnackBar(
        const SnackBar(content: Text('未能打开同花顺或浏览器')),
      );
    }
  }

  Widget _buildPanKouTags(MonitoredStock stock) {
    final tags = PanKouAnalyzer.analyzeTags(stock);
    if (tags.isEmpty) return const SizedBox.shrink();

    return Padding(
      padding: const EdgeInsets.only(top: 6),
      child: Wrap(
        spacing: 6,
        runSpacing: 4,
        children: tags.map((tag) {
          final color = PanKouAnalyzer.getTagColor(tag);
          final name = PanKouAnalyzer.getTagName(tag);
          return Container(
            padding: const EdgeInsets.symmetric(horizontal: 6, vertical: 2),
            decoration: BoxDecoration(
              color: color.withValues(alpha: 0.1),
              borderRadius: BorderRadius.circular(4),
              border: Border.all(color: color.withValues(alpha: 0.3)),
            ),
            child: Text(
              name,
              style: TextStyle(
                fontSize: 11,
                fontWeight: FontWeight.w500,
                color: color,
              ),
            ),
          );
        }).toList(),
      ),
    );
  }

  // ─── Tab ② / ③ 异动列表 ────────────────────────────────

  Widget _buildChangeTab({
    required List<StockChange> changes,
    required bool loading,
    required bool ascending,
    required VoidCallback onToggleSort,
    required String emptyText,
  }) {
    return SafeArea(
      top: false,
      child: Column(
        children: [
          // 排序栏
          if (changes.isNotEmpty)
            Container(
              padding: const EdgeInsets.symmetric(horizontal: 16, vertical: 8),
              color: AppColors.backgroundColor,
              child: Row(
                children: [
                  Text(
                    '共 ${changes.length} 条',
                    style: TextStyle(fontSize: 13, color: Colors.grey[600]),
                  ),
                  const Spacer(),
                  GestureDetector(
                    onTap: onToggleSort,
                    child: Row(
                      children: [
                        Icon(
                          ascending ? Icons.arrow_upward : Icons.arrow_downward,
                          size: 16,
                          color: const Color(0xff2364aa),
                        ),
                        const SizedBox(width: 4),
                        Text(
                          ascending ? '正序' : '倒序',
                          style: const TextStyle(
                            fontSize: 13,
                            color: Color(0xff2364aa),
                          ),
                        ),
                      ],
                    ),
                  ),
                ],
              ),
            ),
          // 列表
          Expanded(
            child: loading
                ? const Center(child: CircularProgressIndicator())
                : changes.isEmpty
                    ? Center(
                        child: Text(
                          emptyText,
                          style: TextStyle(fontSize: 14, color: Colors.grey[500]),
                        ),
                      )
                    : _buildChangeListView(changes, ascending),
          ),
        ],
      ),
    );
  }

  Widget _buildChangeListView(List<StockChange> changes, bool ascending) {
    final sorted = List<StockChange>.of(changes)
      ..sort((a, b) {
        final cmp = '${a.changeDate}${a.changeTime}'
            .compareTo('${b.changeDate}${b.changeTime}');
        return ascending ? cmp : -cmp;
      });

    final vm = context.read<RadarViewModel>();

    return RefreshIndicator(
      onRefresh: () async {
        if (_tabController.index == 1) {
          await vm.loadWatchChanges();
        } else {
          await vm.loadAllChanges();
        }
      },
      child: ListView.builder(
        padding: const EdgeInsets.all(12),
        itemCount: sorted.length,
        itemBuilder: (context, index) {
          final change = sorted[index];
          final exposed = vm.isChangeExposed(change.id);
          return Padding(
            padding: const EdgeInsets.only(bottom: 8),
            child: ExposureTracker(
              itemId: change.id,
              alreadyExposed: exposed,
              onExposed: vm.markChangeExposed,
              child: StockChangeCard(
                key: ValueKey(change.id),
                change: change,
                showStockName: true,
              ),
            ),
          );
        },
      ),
    );
  }
}

// ─── 搜索结果显示面板 ─────────────────────────────────────

class _SearchResultsPanel extends StatefulWidget {
  const _SearchResultsPanel({required this.vm});
  final RadarViewModel vm;

  @override
  State<_SearchResultsPanel> createState() => _SearchResultsPanelState();
}

class _SearchResultsPanelState extends State<_SearchResultsPanel> {
  final Set<String> _adding = {};

  bool _isMonitored(RadarViewModel vm, String code) =>
      vm.monitoredStocks.any((s) => s.code == code);

  Future<void> _addStock(String code, String name) async {
    setState(() => _adding.add(code));
    final vm = widget.vm;
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
    final vm = widget.vm;
    return Padding(
      padding: const EdgeInsets.symmetric(horizontal: 16),
      child: Column(
        crossAxisAlignment: CrossAxisAlignment.start,
        children: [
          Padding(
            padding: const EdgeInsets.only(top: 8, bottom: 8, left: 4),
            child: Text(
              '搜索结果 (${vm.searchResults.length})',
              style: TextStyle(
                fontSize: 13,
                fontWeight: FontWeight.w500,
                color: Colors.grey[600],
              ),
            ),
          ),
          if (vm.searchResults.isEmpty)
            Container(
              padding: const EdgeInsets.all(24),
              alignment: Alignment.center,
              child: Text(
                '未找到匹配的股票',
                style: TextStyle(fontSize: 14, color: Colors.grey[500]),
              ),
            )
          else
            ...vm.searchResults.map(
              (result) => Container(
                margin: const EdgeInsets.only(bottom: 6),
                padding: const EdgeInsets.all(12),
                decoration: BoxDecoration(
                  color: AppColors.cardBg,
                  borderRadius: BorderRadius.circular(10),
                  border: Border.all(color: Colors.grey.shade200),
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
                              color: Colors.grey[600],
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
                              child: CircularProgressIndicator(strokeWidth: 2),
                            )
                          : _isMonitored(vm, result['code'] ?? '')
                              ? const Icon(Icons.check_circle,
                                  color: Colors.green, size: 22)
                              : const Icon(Icons.add_circle_outline,
                                  color: Color(0xff2364aa)),
                      onPressed: _adding.contains(result['code']) ||
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
  }
}
