import 'package:flutter/material.dart';
import 'package:provider/provider.dart';
import 'package:trading_app/core/theme/app_colors.dart';
import 'package:trading_app/core/utils/stock_launcher.dart';
import 'package:trading_app/features/radar/domain/radar_models.dart';
import 'package:trading_app/features/radar/domain/pankou_analyzer.dart';
import 'package:trading_app/features/radar/domain/voice_announcement_view_model.dart';
import 'package:trading_app/shared/widgets/stock_change_card.dart';
import 'monitor_settings_page.dart';
import 'radar_view_model.dart';
import 't0_strategy_view_model.dart';
import 'search_results_panel.dart';
import 'voice_manager_page.dart';
import '../stock_change_detail/stock_change_detail_page.dart';

class RadarPage extends StatefulWidget {
  const RadarPage({super.key});

  @override
  State<RadarPage> createState() => _RadarPageState();
}

class _RadarPageState extends State<RadarPage> with TickerProviderStateMixin {
  late final TabController _tabController;
  late final RadarViewModel _radarViewModel;

  // 各 tab 的排序状态
  bool _watchLoaded = false;
  bool _allLoaded = false;
  bool _strategyLoaded = false;
  bool _watchAscending = false;
  bool _allAscending = false;

  @override
  void initState() {
    super.initState();
    _tabController = TabController(length: 4, vsync: this);
    _tabController.addListener(_onTabChanged);
    _radarViewModel = context.read<RadarViewModel>();
    _bindVoiceAnnouncement();
    _checkVoicePermissionAfterBuild();
    // 测试语音播报时取消下面这行注释：启动到首页后自动播放模拟异动
    // _playTestVoiceAnnouncementOnce();
    // App 启动后触发 T0 主板策略预热（当天已预热则后端跳过）
    WidgetsBinding.instance.addPostFrameCallback((_) {
      if (!mounted) return;
      context.read<T0StrategyViewModel>().warmUpIfNeeded();
    });
  }

  void _onTabChanged() {
    if (!_tabController.indexIsChanging) return;
    _lazyLoadIfNeeded(_tabController.index);
  }

  void _lazyLoadIfNeeded(int index) {
    final vm = context.read<RadarViewModel>();
    if (index == 1 && !_strategyLoaded) {
      _strategyLoaded = true;
      final t0Vm = context.read<T0StrategyViewModel>();
      t0Vm.loadAvailableDates();
      t0Vm.loadResults();
    } else if (index == 2 && !_watchLoaded) {
      _watchLoaded = true;
      vm.loadWatchChanges();
    } else if (index == 3 && !_allLoaded) {
      _allLoaded = true;
      vm.loadAllChanges();
    }
  }

  @override
  void dispose() {
    _unbindVoiceAnnouncement();
    _tabController.removeListener(_onTabChanged);
    _tabController.dispose();
    super.dispose();
  }

  /// 绑定语音播报：把新异动抛给 VoiceAnnouncementViewModel
  void _bindVoiceAnnouncement() {
    final voiceVm = context.read<VoiceAnnouncementViewModel>();
    _radarViewModel.onNewVoiceChange = (change, {bool urgent = false}) {
      voiceVm.enqueueChange(change, urgent: urgent);
    };
  }

  /// 解绑语音播报回调，避免内存泄漏
  void _unbindVoiceAnnouncement() {
    _radarViewModel.onNewVoiceChange = null;
  }

  /// 构建完成后检查是否需要弹出语音授权提示
  void _checkVoicePermissionAfterBuild() {
    WidgetsBinding.instance.addPostFrameCallback((_) {
      if (!mounted) return;
      final voiceVm = context.read<VoiceAnnouncementViewModel>();
      if (!voiceVm.askedBefore) {
        _showVoicePermissionDialog(context);
      }
    });
  }

  /// 测试：程序启动到首页后，延迟播放一次当前位置的异动
  /// TODO: 测试完成后请删除本方法及 initState 中的调用
  // ignore: unused_element
  void _playTestVoiceAnnouncementOnce() {
    WidgetsBinding.instance.addPostFrameCallback((_) async {
      // 等待授权弹窗处理 + TTS 初始化
      await Future.delayed(const Duration(seconds: 4));
      if (!mounted) return;
      final voiceVm = context.read<VoiceAnnouncementViewModel>();
      voiceVm.playTestChanges();
    });
  }

  /// 顶部语音播报喇叭按钮
  Widget _buildVoiceButton() {
    return Selector<VoiceAnnouncementViewModel, ({bool enabled, bool speaking})>(
      selector: (_, vm) => (enabled: vm.enabled, speaking: vm.isSpeaking),
      builder: (_, state, __) {
        final icon = !state.enabled
            ? Icons.volume_off_outlined
            : state.speaking
                ? Icons.volume_up
                : Icons.volume_up_outlined;
        return IconButton(
          icon: Icon(icon, color: state.enabled ? AppColors.buttonPrimary : Colors.grey),
          tooltip: '语音播报管理',
          onPressed: () {
            Navigator.of(context).push(
              MaterialPageRoute(builder: (_) => const VoiceManagerPage()),
            );
          },
        );
      },
    );
  }

  /// 语音授权申请弹窗
  void _showVoicePermissionDialog(BuildContext context) {
    showDialog<void>(
      context: context,
      barrierDismissible: false,
      builder: (dialogContext) {
        final voiceVm = dialogContext.read<VoiceAnnouncementViewModel>();
        return AlertDialog(
          title: const Text('开启语音播报？'),
          content: const Text(
            '开启后，当自选股出现新的异动信息时，系统会自动进行语音播报，帮助您及时把握盘中变化。',
          ),
          actions: [
            TextButton(
              onPressed: () async {
                await voiceVm.denyPermission();
                if (dialogContext.mounted) {
                  Navigator.of(dialogContext).pop();
                }
              },
              child: const Text('暂不开启'),
            ),
            ElevatedButton(
              onPressed: () async {
                await voiceVm.grantPermission();
                if (dialogContext.mounted) {
                  Navigator.of(dialogContext).pop();
                }
              },
              style: ElevatedButton.styleFrom(
                backgroundColor: AppColors.buttonPrimary,
                foregroundColor: Colors.white,
              ),
              child: const Text('开启'),
            ),
          ],
        );
      },
    );
  }

  @override
  Widget build(BuildContext context) {
    return DefaultTabController(
      length: 4,
      child: Scaffold(
        appBar: AppBar(
          centerTitle: true,
          title: const Text('盘达', style: TextStyle(fontSize: 15, fontWeight: FontWeight.w600),),
          backgroundColor: Colors.white,
          foregroundColor: Colors.black,
          elevation: 0.5,
          actions: [
            _buildVoiceButton(),
          ],
          bottom: PreferredSize(
            preferredSize: const Size.fromHeight(44),
            child: Container(
              color: Colors.white,
              child: Selector<T0StrategyViewModel, int>(
                selector: (_, vm) => vm.results.length,
                builder: (_, count, __) {
                  final strategyLabel = count > 0 ? '主板策略($count)' : '主板策略';
                  return TabBar(
                    controller: _tabController,
                    labelColor: const Color(0xff2364aa),
                    unselectedLabelColor: Colors.grey,
                    indicatorColor: const Color(0xff2364aa),
                    labelStyle: const TextStyle(
                      fontWeight: FontWeight.w600,
                      fontSize: 15,
                    ),
                    onTap: _lazyLoadIfNeeded,
                    tabs: [
                      const Tab(text: '监控股票(自选)'),
                      Tab(text: strategyLabel),
                      const Tab(text: '自选异动'),
                      const Tab(text: '全市场'),
                    ],
                  );
                },
              ),
            ),
          ),
        ),
        backgroundColor: AppColors.backgroundColor,
        body: TabBarView(
          controller: _tabController,
          children: [
            // ─── Tab ① 监控股票 ──────────────────────────────
            Consumer<RadarViewModel>(
              builder: (_, vm, __) => _buildStockTab(vm),
            ),
            // ─── Tab ② 主板策略 ──────────────────────────────
            Consumer<T0StrategyViewModel>(
              builder: (_, vm, __) => SelectionArea(
                child: _buildStrategyTab(vm),
              ),
            ),
            // ─── Tab ③ 自选异动 ──────────────────────────────
            Selector<RadarViewModel, ({List<StockChange> changes, bool loading})>(
              selector: (_, vm) => (changes: vm.watchChanges, loading: vm.watchLoading),
              builder: (_, data, __) => _buildChangeTab(
                changes: data.changes,
                loading: data.loading,
                ascending: _watchAscending,
                onToggleSort: () =>
                    setState(() => _watchAscending = !_watchAscending),
                emptyText: '暂无持仓异动',
              ),
            ),
            // ─── Tab ④ 全市场 ────────────────────────────────
            Selector<RadarViewModel, ({List<StockChange> changes, bool loading})>(
              selector: (_, vm) => (changes: vm.allChanges, loading: vm.allLoading),
              builder: (_, data, __) => _buildChangeTab(
                changes: data.changes,
                loading: data.loading,
                ascending: _allAscending,
                onToggleSort: () =>
                    setState(() => _allAscending = !_allAscending),
                emptyText: '暂无全市场异动',
              ),
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
                  SliverToBoxAdapter(child: SearchResultsPanel()),

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
          hintStyle: TextStyle(
            fontSize: 14,

          ),
          prefixIcon: const Icon(Icons.search, size: 20),
          suffixIcon: Row(
            mainAxisSize: MainAxisSize.min,
            children: [
              if (vm.isSearching)
                const Padding(
                  padding: EdgeInsets.all(12),
                  child: SizedBox(
                    width: 18,
                    height: 18,
                    child: CircularProgressIndicator(strokeWidth: 2),
                  ),
                )
              else if (vm.searchKeyword.isNotEmpty)
                IconButton(
                  icon: const Icon(Icons.clear, size: 18),
                  onPressed: () => vm.searchKeyword = '',
                ),
              IconButton(
                icon: Icon(Icons.settings, size: 20, color: AppColors.buttonPrimary),
                onPressed: () async {
                  final changed = await Navigator.of(context).push<bool>(
                    MaterialPageRoute(
                      builder: (_) => const MonitorSettingsPage(),
                    ),
                  );
                  if (changed == true && context.mounted) {
                    // 设置已变更，刷新所有数据
                    final vm = context.read<RadarViewModel>();
                    _watchLoaded = false;
                    _allLoaded = false;
                    vm.loadMonitoredStocks();
                    _lazyLoadIfNeeded(_tabController.index);
                  }
                },
                tooltip: '异动类型设置',
              ),
            ],
          ),
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
                  // 第一行：名称 + 代码 + 异动 + 价格 + 涨幅
                  Row(
                    crossAxisAlignment: CrossAxisAlignment.center,
                    // mainAxisAlignment: MainAxisAlignment.center,
                    children: [
                      Text(
                        stock.name,
                        style: const TextStyle(
                          fontWeight: FontWeight.w600,
                          fontSize: 14,
                        ),
                      ),
                      const SizedBox(width: 8),
                      Text(
                        stock.code,
                        style: TextStyle(fontSize: 12, color: Colors.grey[600]),
                      ),
                      const SizedBox(width: 8),
                      Text(
                        '¥${stock.price.toStringAsFixed(2)}',
                        style: const TextStyle(
                          fontSize: 16,
                          fontWeight: FontWeight.bold,
                        ),
                      ),
                      const SizedBox(width: 10),
                      Text(
                        '${isUp ? "+" : ""}${stock.changePercent.toStringAsFixed(2)}%',
                        style: TextStyle(
                          fontSize: 14,
                          fontWeight: FontWeight.w800,
                          color: changeColor,
                        ),
                      ),
                      const SizedBox(width: 8),
                      // 盘口语言标签
                      _buildPanKouTags(stock),
                      
                    ],
                  ),
      
                  const SizedBox(height: 4),
                  // 成交额
                  if (stock.amount > 0)
                    Text(
                      '成交额 ${formatAmount(stock.amount)}',
                      style: TextStyle(fontSize: 12, color: AppColors.textSecondary),
                    ),
                  // 最新异动描述
                  Builder(builder: (_) {
                    final desc = vm.getLatestAlertDescription(stock.code);
                    if (desc == null) return const SizedBox.shrink();
                    return Padding(
                      padding: const EdgeInsets.only(top: 4),
                      child: Row(
                        children: [
                          Container(
                            padding: const EdgeInsets.symmetric(horizontal: 5, vertical: 2),
                            decoration: BoxDecoration(
                              color: AppColors.tagRed,
                              borderRadius: BorderRadius.circular(3),
                            ),
                            child: const Text(
                              '异动',
                              style: TextStyle(color: Colors.white, fontSize: 10),
                            ),
                          ),
                          const SizedBox(width: 6),
                          Icon(Icons.warning_amber_rounded,
                              size: 12, color: AppColors.warning),
                          const SizedBox(width: 4),
                          Expanded(
                            child: Text(
                              desc,
                              maxLines: 1,
                              overflow: TextOverflow.ellipsis,
                              style: TextStyle(
                                fontSize: 11,
                                color: AppColors.warning,
                              ),
                            ),
                          ),
                        ],
                      ),
                    );
                  }),
                ],
              ),
            ),
            // X（移除监控）
            IconButton(
              icon: const Icon(Icons.close, size: 18, color: Colors.grey),
              onPressed: () => vm.removeMonitoredStock(stock.code),
              tooltip: '移除监控',
            ),
            // 同花顺打开
            Material(
              color: Colors.transparent,
              child: InkWell(
                onTap: () => _openInTongHuaShun(context, stock.code),
                borderRadius: BorderRadius.circular(6),
                child: Image.asset(
                  'assets/images/kline_button.png',
                  width: 24,
                  height: 24,
                  fit: BoxFit.contain,
                ),
              ),
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

  // ─── Tab ④ 主板策略 ─────────────────────────────────────

  String _backfillTitle(T0WarmProgress wp) {
    if (wp.backfillDate != null && wp.backfillDate!.isNotEmpty) {
      return '正在补全 ${wp.backfillDate} 数据...';
    }
    return wp.isWarming ? '服务端正在预热数据...' : '数据预热完成，等待选股...';
  }

  String? _backfillSubtitle(T0WarmProgress wp) {
    if (wp.backfillPhase == 'selection' && wp.backfillDate != null) {
      return '生成 ${wp.backfillDate} 选股结果...';
    }
    if (wp.backfillPhase == 'close_refresh' && wp.backfillDate != null) {
      return '刷新 ${wp.backfillDate} 收盘涨幅...';
    }
    return null;
  }

  Widget _buildStrategyDateBar(T0StrategyViewModel vm) {
    if (!vm.showDateSelector) return const SizedBox.shrink();
    return Container(
      width: double.infinity,
      padding: const EdgeInsets.symmetric(horizontal: 16, vertical: 4),
      color: AppColors.cardBg,
      child: Row(
        children: [
          Text(
            '当前显示',
            style: TextStyle(fontSize: 12, color: AppColors.textSecondary),
          ),
          const SizedBox(width: 8),
          TextButton(
            onPressed: vm.canGoPreviousArchive ? vm.selectPreviousArchive : null,
            style: TextButton.styleFrom(
              padding: const EdgeInsets.symmetric(horizontal: 6),
              minimumSize: Size.zero,
              tapTargetSize: MaterialTapTargetSize.shrinkWrap,
            ),
            child: Text(
              '前一天',
              style: TextStyle(
                fontSize: 12,
                color: vm.canGoPreviousArchive
                    ? AppColors.textSecondary
                    : AppColors.textSecondary.withValues(alpha: 0.4),
              ),
            ),
          ),
          const SizedBox(width: 4),
          DropdownButton<String>(
            value: vm.selectedDate,
            underline: const SizedBox.shrink(),
            style: TextStyle(
              fontSize: 13,
              fontWeight: FontWeight.w600,
              color: AppColors.textPrimary,
            ),
            items: vm.dropdownDates
                .map(
                  (d) => DropdownMenuItem(
                    value: d,
                    child: Text(d),
                  ),
                )
                .toList(),
            onChanged: (d) {
              if (d != null) vm.selectDate(d);
            },
          ),
          const SizedBox(width: 4),
          TextButton(
            onPressed: vm.canGoNextArchive ? vm.selectNextArchive : null,
            style: TextButton.styleFrom(
              padding: const EdgeInsets.symmetric(horizontal: 6),
              minimumSize: Size.zero,
              tapTargetSize: MaterialTapTargetSize.shrinkWrap,
            ),
            child: Text(
              '后一天',
              style: TextStyle(
                fontSize: 12,
                color: vm.canGoNextArchive
                    ? AppColors.textSecondary
                    : AppColors.textSecondary.withValues(alpha: 0.4),
              ),
            ),
          ),
          const SizedBox(width: 8),
          Text(
            '选股结果',
            style: TextStyle(fontSize: 12, color: AppColors.textSecondary),
          ),
        ],
      ),
    );
  }

  Widget _buildStrategyTab(T0StrategyViewModel vm) {
    final wp = vm.warmProgress;

    // 预热进度 / 等待中
    if (wp != null) {
      return SafeArea(
        top: false,
        child: Column(
          crossAxisAlignment: CrossAxisAlignment.stretch,
          children: [
            _buildStrategyDateBar(vm),
            Expanded(
              child: Center(
                child: Padding(
                  padding: const EdgeInsets.all(32),
                  child: Column(
                    mainAxisSize: MainAxisSize.min,
                    children: [
                      const CircularProgressIndicator(),
                      const SizedBox(height: 20),
                      Text(
                        _backfillTitle(wp),
                        style: const TextStyle(
                            fontSize: 15, fontWeight: FontWeight.w500),
                      ),
                      if (_backfillSubtitle(wp) != null)
                        Padding(
                          padding: const EdgeInsets.only(top: 8),
                          child: Text(
                            _backfillSubtitle(wp)!,
                            style: TextStyle(
                                fontSize: 13, color: AppColors.textSecondary),
                          ),
                        ),
                      const SizedBox(height: 12),
                      if (wp.stockCount > 0)
                        Text(
                          '已拉取 ${wp.stockCount} 只主板股票',
                          style: TextStyle(
                              fontSize: 13, color: AppColors.textSecondary),
                        ),
                      if (wp.dailyFetched > 0)
                        Padding(
                          padding: const EdgeInsets.only(top: 4),
                          child: Text(
                            '日线进度: ${wp.dailyFetched}/${wp.dailyTotal}',
                            style: TextStyle(
                                fontSize: 13, color: AppColors.textSecondary),
                          ),
                        ),
                      if (wp.candidateCount > 0)
                        Padding(
                          padding: const EdgeInsets.only(top: 4),
                          child: Text(
                            '候选股票: ${wp.candidateCount} 只（涨停 + 成交额过滤后）',
                            style: TextStyle(
                                fontSize: 13, color: AppColors.textSecondary),
                          ),
                        ),
                      const SizedBox(height: 16),
                      Text(
                        '每10秒自动刷新',
                        style: TextStyle(fontSize: 12, color: Colors.grey[500]),
                      ),
                    ],
                  ),
                ),
              ),
            ),
          ],
        ),
      );
    }

    return SafeArea(
      top: false,
      child: vm.loading
          ? const Center(child: CircularProgressIndicator())
          : vm.error != null
              ? Center(
                  child: Padding(
                    padding: const EdgeInsets.all(24),
                    child: Text(
                      '加载失败: ${vm.error}',
                      textAlign: TextAlign.center,
                      style: TextStyle(fontSize: 14, color: AppColors.textSecondary),
                    ),
                  ),
                )
              : Column(
                  crossAxisAlignment: CrossAxisAlignment.stretch,
                  children: [
                    _buildStrategyDateBar(vm),
                    Expanded(
                      child: vm.results.isEmpty
                          ? Center(
                              child: Text(
                                '暂无符合条件的股票',
                                style: TextStyle(
                                    fontSize: 14, color: Colors.grey[500]),
                              ),
                            )
                          : ListView.builder(
                              padding: const EdgeInsets.symmetric(
                                  horizontal: 16, vertical: 8),
                              itemCount: vm.results.length,
                              itemBuilder: (_, i) => _buildStrategyCard(
                                    vm.results[i],
                                    preview: vm.showingCandidatePreview,
                                  ),
                            ),
                    ),
                  ],
                ),
    );
  }

  Widget _buildBuySignalChip(T0StrategyStock stock) {
    if (stock.buySignal.isEmpty && stock.pattern.isEmpty) {
      return const SizedBox.shrink();
    }

    Color dotColor;
    switch (stock.buySignal) {
      case 'blue':
        dotColor = AppColors.info;
        break;
      case 'orange':
        dotColor = AppColors.tagOrange;
        break;
      case 'green':
        dotColor = AppColors.success;
        break;
      case 'yellow':
        dotColor = AppColors.warning;
        break;
      case 'red':
        dotColor = AppColors.error;
        break;
      default:
        dotColor = AppColors.textTertiary;
    }

    // 灰灯仅表示样本不足/无库记录；只要有形态统计就显示整数率，不用 —/—
    final noStats = stock.pattern.isEmpty ||
        (stock.patternT0N == 0 &&
            stock.patternWinPct == 0 &&
            stock.patternFailPct == 0);
    final rateText = noStats
        ? '—/—'
        : '${stock.patternWinPct.round()}/${stock.patternEarnPct.round()}';

    return Tooltip(
      message: noStats
          ? '形态样本不足'
          : '达标 ${stock.patternWinPct.round()}%  ·  赚率 ${stock.patternEarnPct.round()}%（T0≥0，含小赚，不是达标率）',
      child: Row(
        mainAxisSize: MainAxisSize.min,
        children: [
          Container(
            width: 8,
            height: 8,
            decoration: BoxDecoration(color: dotColor, shape: BoxShape.circle),
          ),
          const SizedBox(width: 2),
          Text(
            rateText,
            style: TextStyle(
              fontSize: 11,
              fontWeight: FontWeight.w500,
              color: noStats ? AppColors.textTertiary : AppColors.textSecondary,
            ),
          ),
        ],
      ),
    );
  }

  Widget _buildStrategyCard(T0StrategyStock stock, {bool preview = false}) {
    final openUp = stock.openGap >= 0;
    final openColor = openUp ? AppColors.textPriceUp : AppColors.textPriceDown;
    final closeUp = stock.closeRet >= 0;
    final closeColor = closeUp ? AppColors.textPriceUp : AppColors.textPriceDown;
    final livePct = stock.liveChangePercent ?? 0.0;
    final liveUp = livePct >= 0;
    final liveColor = liveUp ? AppColors.textPriceUp : AppColors.textPriceDown;

    return GestureDetector(
      behavior: HitTestBehavior.opaque,
      onTap: () => _openInTongHuaShun(context, stock.rawCode),
      child: Container(
        margin: const EdgeInsets.only(bottom: 4),
        padding: const EdgeInsets.symmetric(horizontal: 10, vertical: 8),
        decoration: BoxDecoration(
          color: AppColors.cardBg,
          borderRadius: BorderRadius.circular(8),
          border: Border.all(color: AppColors.border),
        ),
        child: Row(
          children: [
          // 名称
          Text(
            stock.stockName,
            style: const TextStyle(fontWeight: FontWeight.w600, fontSize: 14),
          ),
          if (stock.tag.isNotEmpty) ...[
            const SizedBox(width: 4),
            Text(
              '[${stock.tag}]',
              style: TextStyle(
                fontSize: 11,
                fontWeight: FontWeight.w600,
                color: stock.tag == '涨停破板'
                    ? AppColors.tagRed
                    : AppColors.tagGreen,
              ),
            ),
          ],
          const SizedBox(width: 6),
          if (!preview)
            Text(
              '${closeUp ? "+" : ""}${stock.closeRet.toStringAsFixed(2)}%',
              style: TextStyle(fontSize: 13, fontWeight: FontWeight.w500, color: closeColor),
            ),
          if (!preview) const SizedBox(width: 6),
          Text(
            stock.rawCode,
            style: TextStyle(fontSize: 11, color: Colors.grey[600]),
          ),
          const SizedBox(width: 6),
          _buildBuySignalChip(stock),
          const Spacer(),
          if (preview) ...[
            Text(
              '竞价预览（未确认）',
              style: TextStyle(fontSize: 11, color: Colors.grey[600]),
            ),
            const SizedBox(width: 8),
            Text(
              '${liveUp ? "+" : ""}${livePct.toStringAsFixed(2)}%',
              style: TextStyle(
                fontSize: 14,
                fontWeight: FontWeight.bold,
                color: liveColor,
              ),
            ),
          ] else
            Text(
              '开盘${openUp ? "+" : ""}${stock.openGap.toStringAsFixed(2)}%',
              style: TextStyle(
                fontSize: 14,
                fontWeight: FontWeight.bold,
                color: openColor,
              ),
            ),
          const SizedBox(width: 8),
          // 同花顺按钮
          Material(
            color: Colors.transparent,
            child: InkWell(
              onTap: () => _openInTongHuaShun(context, stock.rawCode),
              borderRadius: BorderRadius.circular(6),
              child: Image.asset(
                'assets/images/kline_button.png',
                width: 22,
                height: 22,
                fit: BoxFit.contain,
              ),
            ),
          ),
        ],
        ),
      ),
    );
  }

  String _formatInflow(double amount) {
    // 后端返回的是千元，先转为元
    final yuan = amount * 10;
    final absAmount = yuan.abs();
    final sign = yuan >= 0 ? '+' : '-';
    if (absAmount >= 100000000) {
      return '$sign${(absAmount / 100000000).toStringAsFixed(2)}亿';
    } else if (absAmount >= 10000) {
      return '$sign${(absAmount / 10000).toStringAsFixed(2)}万';
    }
    return '$sign${absAmount.toStringAsFixed(0)}';
  }

  Widget _buildPanKouTags(MonitoredStock stock) {
    final tags = PanKouAnalyzer.analyzeTags(stock);
    if (tags.isEmpty) return const SizedBox.shrink();

    return Wrap(
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
          // final exposed = vm.isChangeExposed(change.id); // 暂时注释，曝光逻辑已禁用
          return Padding(
            padding: const EdgeInsets.only(bottom: 8),
            // 暂时注释曝光即已读逻辑，改为手动标记已读
            // child: ExposureTracker(
            //   itemId: change.id,
            //   alreadyExposed: exposed,
            //   onExposed: vm.markChangeExposed,
            //   child: StockChangeCard(
            //     key: ValueKey(change.id),
            //     change: change,
            //     onOpenTongHuaShun: () =>
            //         _openInTongHuaShun(context, change.stockCode),
            //   ),
            // ),
            child: StockChangeCard(
              key: ValueKey(change.id),
              change: change,
              onOpenTongHuaShun: () =>
                  _openInTongHuaShun(context, change.stockCode),
            ),
          );
        },
      ),
    );
  }
}
