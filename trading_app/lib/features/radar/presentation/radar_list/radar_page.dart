import 'dart:async';

import 'package:flutter/foundation.dart';
import 'package:flutter/material.dart';
import 'package:provider/provider.dart';
import 'package:visibility_detector/visibility_detector.dart';
import 'package:trading_app/core/theme/app_colors.dart';
import 'package:trading_app/core/storage/red_kline_settings.dart';
import 'package:trading_app/features/radar/data/red_kline_repository.dart';
import 'package:trading_app/features/radar/data/red_t1_intraday_repository.dart';
import 'package:trading_app/features/radar/data/red_auction_notification_bridge.dart'
    as auction_notifications;
import 'package:trading_app/features/radar/domain/red_auction_reminder.dart';
import 'package:trading_app/features/auth/presentation/auth_view_model.dart';
import 'package:trading_app/core/utils/stock_launcher.dart';
import 'package:trading_app/features/radar/domain/radar_models.dart';
import 'package:trading_app/features/radar/domain/pankou_analyzer.dart';
import 'package:trading_app/features/radar/domain/voice_announcement_view_model.dart';
import 'package:trading_app/features/permissions/domain/module_definition.dart';
import 'package:trading_app/shared/widgets/stock_change_card.dart';
import 'package:trading_app/features/permissions/presentation/module_permission_controller.dart';
import 'monitor_settings_page.dart';
import 'radar_view_model.dart';
import 't0_strategy_view_model.dart';
import 'radar_module_definitions.dart';
import 'search_results_panel.dart';
import 'strategy_charts.dart';
import 'strategy_stock_card.dart';
import 'strategy_stock_charts.dart';
import 'red_empty_reminders.dart';

import 'red_t1_intraday_exposure.dart';
import 'voice_manager_page.dart';
import '../stock_change_detail/stock_change_detail_page.dart';

export 'stock_code_copy_button.dart' show StockCodeCopyButton;

class RadarPage extends StatefulWidget {
  const RadarPage({super.key, this.onShowRadar});

  final VoidCallback? onShowRadar;

  @override
  State<RadarPage> createState() => _RadarPageState();
}

class _RadarPageState extends State<RadarPage>
    with TickerProviderStateMixin, WidgetsBindingObserver {
  late TabController _tabController;
  late final RadarViewModel _radarViewModel;
  late final ModulePermissionController _permissionController;
  late final T0StrategyViewModel _t0StrategyViewModel;
  RedAuctionReminder? _redAuctionReminder;
  String? _redAuctionUserId;
  bool _redDeepLinkOpened = false;
  bool _pairingLocalReminder = false;
  List<RadarModuleDefinition> _visibleModules = [];
  final Set<String> _loadedModuleCodes = {};
  final Map<String, bool> _ascendingByModule = {};
  final Set<String> _shownT0WarningKeys = {};
  int _chartRefreshRevision = 0;
  final Map<String, Future<Map<String, RedKlineWindow>>>
  _redRegressionKlineFutures = {};

  bool get _permissionLoadFailed =>
      _permissionController.state == ModulePermissionState.failure;

  @override
  void initState() {
    super.initState();
    VisibilityDetectorController.instance.updateInterval = const Duration(
      milliseconds: 100,
    );
    _permissionController = context.read<ModulePermissionController>();
    _t0StrategyViewModel = context.read<T0StrategyViewModel>();
    _visibleModules = _resolveVisibleModules();
    _tabController = TabController(length: _visibleModules.length, vsync: this);
    _tabController.addListener(_onTabChanged);
    _permissionController.addListener(_onPermissionsChanged);
    _radarViewModel = context.read<RadarViewModel>();
    _t0StrategyViewModel.addListener(_onT0DataWarning);
    if (kIsWeb) {
      _redAuctionUserId = context.read<AuthViewModel?>()?.user?.id;
      _redAuctionReminder = RedAuctionReminder(
        request: _t0StrategyViewModel.fetchRedAuctionConfirmation,
        onConfirmed: _onRedAuctionConfirmed,
        onTimeout: _onRedAuctionTimeout,
      );
      WidgetsBinding.instance.addObserver(this);
      _syncRedAuctionReminder();
    }
    _bindVoiceAnnouncement();
    _consumeRedDeepLink();
    _checkVoicePermissionAfterBuild();
    // 测试语音播报时取消下面这行注释：启动到首页后自动播放模拟异动
    // _playTestVoiceAnnouncementOnce();
    // App 启动后触发一个已授权策略模块的 T0 预热（当天已预热则后端跳过）。
    WidgetsBinding.instance.addPostFrameCallback((_) {
      if (!mounted) return;
      final strategy = _visibleModules
          .cast<RadarModuleDefinition?>()
          .firstWhere(
            (module) =>
                module != null &&
                module.accessMode == ModuleAccessMode.userAllowlist &&
                module.contentKind != RadarContentKind.redStrategyRegression,
            orElse: () => null,
          );
      if (strategy != null) {
        context.read<T0StrategyViewModel>().warmUpIfNeeded(
          moduleCode: strategy.code,
        );
      }
    });
  }

  List<RadarModuleDefinition> _resolveVisibleModules() {
    final allowedCodes = _permissionController.visibleModules
        .map((module) => module.code)
        .toSet();
    if (allowedCodes.isEmpty) {
      allowedCodes.addAll(publicModuleDefinitions.map((module) => module.code));
    }
    return radarModuleDefinitions.where((module) {
      if (module.contentKind == RadarContentKind.redStrategyRegression) {
        return allowedCodes.contains(t0SharedGobRedStrategyModuleCode);
      }
      return allowedCodes.contains(module.code);
    }).toList();
  }

  void _onPermissionsChanged() {
    if (!mounted) return;
    _syncRedAuctionReminder();
    _consumeRedDeepLink();
    final nextModules = _resolveVisibleModules();
    final oldCodes = _visibleModules.map((module) => module.code).toList();
    final nextCodes = nextModules.map((module) => module.code).toList();
    if (listEquals(oldCodes, nextCodes)) {
      setState(() {});
      return;
    }

    final currentCode = _tabController.index < _visibleModules.length
        ? _visibleModules[_tabController.index].code
        : null;
    final nextIndex = currentCode == null
        ? 0
        : nextCodes.indexOf(currentCode).clamp(0, nextModules.length - 1);
    final oldController = _tabController;
    oldController.removeListener(_onTabChanged);
    _visibleModules = nextModules;
    _loadedModuleCodes.removeWhere((code) => !nextCodes.contains(code));
    _tabController = TabController(
      length: _visibleModules.length,
      initialIndex: nextIndex,
      vsync: this,
    )..addListener(_onTabChanged);
    oldController.dispose();
    setState(() {});
    _lazyLoadIfNeeded(_tabController.index);
  }

  void _syncRedAuctionReminder() {
    _redAuctionReminder?.setEnabled(
      _redAuctionUserId != null &&
          !auction_notifications.localReminderEnabled() &&
          _permissionController.canView(t0SharedGobRedStrategyModuleCode),
    );
  }

  @override
  void didChangeAppLifecycleState(AppLifecycleState state) {
    if (state != AppLifecycleState.resumed || !mounted) return;
    _redAuctionReminder?.resume();
    if (kIsWeb) setState(() {});
  }

  void _onRedAuctionConfirmed(Map<String, dynamic> data) {
    if (!mounted ||
        !_permissionController.canView(t0SharedGobRedStrategyModuleCode)) {
      return;
    }
    _t0StrategyViewModel.acceptRedAuctionConfirmation(data);
    final rows = data['results'] as List;
    final names = rows
        .whereType<Map>()
        .take(5)
        .map((row) => row['股票名称']?.toString() ?? '')
        .where((name) => name.isNotEmpty)
        .join('、');
    final body = rows.isEmpty
        ? '今日暂无符合条件的股票，点击查看红策。'
        : '${names.isEmpty ? '今日名单已就绪' : names}${rows.length > 5 ? '等' : ''}，点击查看红策。';
    auction_notifications.show(
      date: data['date'] as String,
      userId: _redAuctionUserId!,
      count: rows.length,
      body: body,
      onTap: () => _openRedAuctionResults(data),
    );
    if (auction_notifications.permission() != 'granted') {
      ScaffoldMessenger.of(context).showSnackBar(
        SnackBar(
          content: Text('今日红策已更新，共 ${rows.length} 只'),
          action: SnackBarAction(
            label: '查看',
            onPressed: () => _openRedAuctionResults(data),
          ),
        ),
      );
    }
    if (_t0StrategyViewModel
            .stateFor(t0SharedGobRedStrategyModuleCode)
            .selectedDate ==
        data['date']) {
      unawaited(
        _t0StrategyViewModel.loadAvailableDates(
          moduleCode: t0SharedGobRedStrategyModuleCode,
        ),
      );
    }
  }

  void _onRedAuctionTimeout() {
    if (!mounted) return;
    ScaffoldMessenger.of(context).showSnackBar(
      SnackBar(
        content: const Text('红策数据暂未就绪，可手动刷新'),
        action: SnackBarAction(
          label: '查看',
          onPressed: () => _openRedAuctionResults(),
        ),
      ),
    );
  }

  void _consumeRedDeepLink() {
    if (!kIsWeb ||
        _redDeepLinkOpened ||
        !_permissionController.canView(t0SharedGobRedStrategyModuleCode) ||
        Uri.base.queryParameters['radar_module'] !=
            t0SharedGobRedStrategyModuleCode) {
      return;
    }
    final date = Uri.base.queryParameters['radar_date'];
    if (date != null &&
        (!RegExp(r'^\d{4}-\d{2}-\d{2}$').hasMatch(date) ||
            DateTime.tryParse(date)?.toIso8601String().substring(0, 10) !=
                date)) {
      return;
    }
    _redDeepLinkOpened = true;
    WidgetsBinding.instance.addPostFrameCallback((_) {
      if (mounted) _openRedAuctionResults(null, date);
    });
  }

  Future<void> _pairLocalReminder() async {
    final token = context.read<AuthViewModel>().accessToken;
    if (token == null || token.isEmpty || _pairingLocalReminder) return;
    setState(() => _pairingLocalReminder = true);
    final result = await auction_notifications.pairLocalReminder(token);
    if (!mounted) return;
    _syncRedAuctionReminder();
    setState(() => _pairingLocalReminder = false);
    ScaffoldMessenger.of(context).showSnackBar(
      SnackBar(
        content: Text(
          result == 'connected'
              ? '本机提醒已连接，可在竞价名单就绪后提醒'
              : result.replaceFirst('error:', ''),
        ),
      ),
    );
  }

  void _openRedAuctionResults([Map<String, dynamic>? data, String? date]) {
    if (!mounted ||
        !_permissionController.canView(t0SharedGobRedStrategyModuleCode)) {
      return;
    }
    final index = _visibleModules.indexWhere(
      (module) => module.code == t0SharedGobRedStrategyModuleCode,
    );
    if (index < 0) return;
    widget.onShowRadar?.call();
    _loadedModuleCodes.add(t0SharedGobRedStrategyModuleCode);
    _tabController.animateTo(index);
    if (data != null) {
      _t0StrategyViewModel.acceptRedAuctionConfirmation(data, show: true);
    } else {
      unawaited(
        _t0StrategyViewModel.loadResults(
          moduleCode: t0SharedGobRedStrategyModuleCode,
          date: date,
        ),
      );
    }
    unawaited(
      _t0StrategyViewModel.loadAvailableDates(
        moduleCode: t0SharedGobRedStrategyModuleCode,
      ),
    );
  }

  Future<void> _enableRedAuctionNotifications() async {
    if (auction_notifications.localReminderEnabled()) {
      ScaffoldMessenger.of(
        context,
      ).showSnackBar(const SnackBar(content: Text('已使用本机提醒，无需保持网页打开')));
      return;
    }
    final permission = await auction_notifications.requestPermission();
    if (!mounted) return;
    setState(() {});
    final message = switch (permission) {
      'granted' => '已开启红策竞价通知，09:25:10 自动获取名单',
      'denied' => '请在浏览器的网站设置中允许通知',
      'insecure' => '浏览器通知需要通过 HTTPS 或 localhost 打开网页',
      'unsupported' => '当前浏览器不支持桌面通知',
      _ => '允许通知后，竞价名单就绪时会自动提醒',
    };
    ScaffoldMessenger.of(
      context,
    ).showSnackBar(SnackBar(content: Text(message)));
  }

  void _onTabChanged() {
    if (_tabController.indexIsChanging || _tabController.offset != 0) return;
    _lazyLoadIfNeeded(_tabController.index);
  }

  void _lazyLoadIfNeeded(int index) {
    if (index < 0 || index >= _visibleModules.length) return;
    final module = _visibleModules[index];
    final vm = context.read<RadarViewModel>();
    if (module.contentKind == RadarContentKind.redStrategyRegression &&
        !_loadedModuleCodes.contains(module.code)) {
      _loadedModuleCodes.add(module.code);
      context.read<T0StrategyViewModel>().loadRedRuleHistoryOptions();
    } else if (module.accessMode == ModuleAccessMode.userAllowlist &&
        !_loadedModuleCodes.contains(module.code)) {
      _loadedModuleCodes.add(module.code);
      final t0Vm = context.read<T0StrategyViewModel>();
      t0Vm.loadAvailableDates(moduleCode: module.code);
      t0Vm.loadResults(moduleCode: module.code);
    } else if (module.contentKind == RadarContentKind.watchChanges &&
        !_loadedModuleCodes.contains(module.code)) {
      _loadedModuleCodes.add(module.code);
      vm.loadWatchChanges();
    } else if (module.contentKind == RadarContentKind.allChanges &&
        !_loadedModuleCodes.contains(module.code)) {
      _loadedModuleCodes.add(module.code);
      vm.loadAllChanges();
    }
  }

  void _onT0DataWarning() {
    if (!mounted) return;
    for (final module in _visibleModules) {
      if (module.accessMode != ModuleAccessMode.userAllowlist ||
          module.contentKind == RadarContentKind.redStrategyRegression) {
        continue;
      }
      final state = _t0StrategyViewModel.stateFor(module.code);
      final message = state.dataWarningMessage;
      final key = state.dataWarningKey;
      if (message == null || message.isEmpty || key == null) continue;
      final uniqueKey = '${module.code}:$key';
      if (!_shownT0WarningKeys.add(uniqueKey)) continue;
      WidgetsBinding.instance.addPostFrameCallback((_) {
        if (!mounted) return;
        ScaffoldMessenger.of(context)
          ..hideCurrentSnackBar()
          ..showSnackBar(SnackBar(content: Text('T0数据提醒：$message')));
      });
    }
  }

  @override
  void dispose() {
    WidgetsBinding.instance.removeObserver(this);
    _redAuctionReminder?.dispose();
    _unbindVoiceAnnouncement();
    _t0StrategyViewModel.removeListener(_onT0DataWarning);
    _permissionController.removeListener(_onPermissionsChanged);
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
    return Selector<
      VoiceAnnouncementViewModel,
      ({bool enabled, bool speaking})
    >(
      selector: (_, vm) => (enabled: vm.enabled, speaking: vm.isSpeaking),
      builder: (_, state, __) {
        final icon = !state.enabled
            ? Icons.volume_off_outlined
            : state.speaking
            ? Icons.volume_up
            : Icons.volume_up_outlined;
        return IconButton(
          icon: Icon(
            icon,
            color: state.enabled
                ? AppColors.buttonPrimary
                : AppColors.textTertiary,
          ),
          tooltip: '语音播报管理',
          onPressed: () {
            Navigator.of(
              context,
            ).push(MaterialPageRoute(builder: (_) => const VoiceManagerPage()));
          },
        );
      },
    );
  }

  /// 语音授权申请弹窗
  void _showVoicePermissionDialog(BuildContext context) {
    showDialog<void>(
      context: context,
      useRootNavigator: false,
      barrierDismissible: false,
      builder: (dialogContext) {
        final voiceVm = dialogContext.read<VoiceAnnouncementViewModel>();
        return AlertDialog(
          title: const Text('开启语音播报？'),
          content: const Text('开启后，当自选股出现新的异动信息时，系统会自动进行语音播报，帮助您及时把握盘中变化。'),
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
    AppColors.of(context);
    return Scaffold(
      appBar: AppBar(
        centerTitle: true,
        title: const Text(
          '盘达',
          style: TextStyle(fontSize: 15, fontWeight: FontWeight.w600),
        ),
        backgroundColor: AppColors.appBarBg,
        foregroundColor: AppColors.appBarFg,
        elevation: 0.5,
        actions: [
          if (kIsWeb &&
              auction_notifications.pairingAvailable() &&
              _permissionController.canView(t0SharedGobRedStrategyModuleCode))
            TextButton.icon(
              icon: const Icon(Icons.link, size: 18),
              label: Text(_pairingLocalReminder ? '正在连接' : '连接本机提醒'),
              style: TextButton.styleFrom(foregroundColor: AppColors.appBarFg),
              onPressed: _pairingLocalReminder ? null : _pairLocalReminder,
            ),
          if (kIsWeb &&
              _permissionController.canView(t0SharedGobRedStrategyModuleCode))
            IconButton(
              icon: Icon(
                auction_notifications.localReminderEnabled() ||
                        auction_notifications.permission() == 'granted'
                    ? Icons.notifications_active_outlined
                    : Icons.notifications_none_outlined,
              ),
              tooltip: auction_notifications.localReminderEnabled()
                  ? '已使用本机提醒'
                  : auction_notifications.permission() == 'granted'
                  ? '红策竞价通知已开启'
                  : '开启红策竞价通知',
              onPressed: _enableRedAuctionNotifications,
            ),
          IconButton(
            icon: const Icon(Icons.refresh),
            tooltip: '刷新模块权限',
            onPressed: _permissionController.refresh,
          ),
          _buildVoiceButton(),
        ],
        bottom: PreferredSize(
          preferredSize: Size.fromHeight(_permissionLoadFailed ? 76 : 44),
          child: Container(
            color: AppColors.appBarBg,
            child: Column(
              children: [
                Consumer<T0StrategyViewModel>(
                  builder: (_, vm, __) => TabBar(
                    controller: _tabController,
                    isScrollable: true,
                    labelColor: AppColors.brand,
                    unselectedLabelColor: AppColors.textTertiary,
                    indicatorColor: AppColors.brand,
                    labelStyle: const TextStyle(
                      fontWeight: FontWeight.w600,
                      fontSize: 15,
                    ),
                    onTap: _lazyLoadIfNeeded,
                    tabs: _visibleModules
                        .map((module) => Tab(text: _tabLabel(module, vm)))
                        .toList(),
                  ),
                ),
                if (_permissionLoadFailed)
                  SizedBox(
                    height: 32,
                    child: TextButton.icon(
                      onPressed: _permissionController.refresh,
                      icon: const Icon(Icons.sync, size: 14),
                      label: const Text('模块权限加载失败，点击重试'),
                      style: TextButton.styleFrom(
                        foregroundColor: AppColors.warning,
                        padding: EdgeInsets.zero,
                        textStyle: const TextStyle(fontSize: 12),
                      ),
                    ),
                  ),
              ],
            ),
          ),
        ),
      ),
      backgroundColor: AppColors.scaffoldBg,
      body: TabBarView(
        controller: _tabController,
        children: _visibleModules.map(_buildModuleTab).toList(),
      ),
    );
  }

  String _tabLabel(RadarModuleDefinition module, T0StrategyViewModel t0Vm) {
    final count = switch (module.contentKind) {
      RadarContentKind.redStrategy => t0Vm.resultsFor(module.code).length,
      RadarContentKind.redStrategyRegression => 0,
      RadarContentKind.goldStrategy => t0Vm.resultsFor(module.code).length,
      RadarContentKind.purpleStrategy =>
        t0Vm.purpleResultsFor(module.code).length,
      RadarContentKind.mainStrategy => t0Vm.resultsFor(module.code).length,
      RadarContentKind.blueStrategy => t0Vm.blueResultsFor(module.code).length,
      _ => 0,
    };
    return count > 0 ? '${module.name}($count)' : module.name;
  }

  Widget _buildModuleTab(RadarModuleDefinition module) {
    switch (module.contentKind) {
      case RadarContentKind.monitored:
        return Consumer<RadarViewModel>(
          builder: (_, vm, __) => _buildStockTab(vm),
        );
      case RadarContentKind.redStrategy:
      case RadarContentKind.mainStrategy:
        return Consumer<T0StrategyViewModel>(
          builder: (ctx, vm, __) => _buildStrategyTab(
            vm,
            moduleCode: module.code,
            kind: module.contentKind == RadarContentKind.mainStrategy
                ? StrategyCardVariant.main
                : StrategyCardVariant.red,
            showKline:
                (ctx.watch<RedKlineSettings?>()?.ready ?? false) &&
                (ctx.read<RedKlineSettings?>()?.enabledFor(module.code) ??
                    false),
            klineRepository: ctx.read<RedKlineRepository?>(),
            showIntraday:
                (ctx.watch<RedKlineSettings?>()?.ready ?? false) &&
                (ctx.read<RedKlineSettings?>()?.intradayEnabledFor(
                      module.code,
                    ) ??
                    false),
            intradayRepository: ctx.read<RedT1IntradayRepository?>(),
          ),
        );
      case RadarContentKind.redStrategyRegression:
        return Consumer<T0StrategyViewModel>(
          builder: (ctx, vm, __) => _buildRedStrategyRegressionTab(
            vm,
            showKline:
                (ctx.watch<RedKlineSettings?>()?.ready ?? false) &&
                (ctx.read<RedKlineSettings?>()?.enabled ?? false),
            klineRepository: ctx.read<RedKlineRepository?>(),
            showIntraday:
                (ctx.watch<RedKlineSettings?>()?.ready ?? false) &&
                (ctx.read<RedKlineSettings?>()?.t1IntradayEnabled ?? false),
            intradayRepository: ctx.read<RedT1IntradayRepository?>(),
          ),
        );
      case RadarContentKind.goldStrategy:
        return Consumer<T0StrategyViewModel>(
          builder: (_, vm, __) => _buildStrategyTab(
            vm,
            moduleCode: module.code,
            kind: StrategyCardVariant.gold,
          ),
        );
      case RadarContentKind.purpleStrategy:
        return Consumer<T0StrategyViewModel>(
          builder: (_, vm, __) => _buildStrategyTab(
            vm,
            moduleCode: module.code,
            kind: StrategyCardVariant.purple,
          ),
        );
      case RadarContentKind.blueStrategy:
        return Consumer<T0StrategyViewModel>(
          builder: (_, vm, __) => _buildStrategyTab(
            vm,
            moduleCode: module.code,
            kind: StrategyCardVariant.blue,
          ),
        );
      case RadarContentKind.watchChanges:
        return Selector<
          RadarViewModel,
          ({List<StockChange> changes, bool loading})
        >(
          selector: (_, vm) =>
              (changes: vm.watchChanges, loading: vm.watchLoading),
          builder: (_, data, __) => _buildChangeTab(
            changes: data.changes,
            loading: data.loading,
            ascending: _ascendingByModule[module.code] ?? false,
            onToggleSort: () => setState(() {
              _ascendingByModule[module.code] =
                  !(_ascendingByModule[module.code] ?? false);
            }),
            emptyText: '暂无持仓异动',
          ),
        );
      case RadarContentKind.allChanges:
        return Selector<
          RadarViewModel,
          ({List<StockChange> changes, bool loading})
        >(
          selector: (_, vm) => (changes: vm.allChanges, loading: vm.allLoading),
          builder: (_, data, __) => _buildChangeTab(
            changes: data.changes,
            loading: data.loading,
            ascending: _ascendingByModule[module.code] ?? false,
            onToggleSort: () => setState(() {
              _ascendingByModule[module.code] =
                  !(_ascendingByModule[module.code] ?? false);
            }),
            emptyText: '暂无全市场异动',
          ),
        );
    }
  }

  Widget _buildRedStrategyRegressionTab(
    T0StrategyViewModel vm, {
    required bool showKline,
    required RedKlineRepository? klineRepository,
    required bool showIntraday,
    required RedT1IntradayRepository? intradayRepository,
  }) {
    final options = vm.redRuleHistoryOptions;
    final selectedId = vm.selectedRedRuleHistoryId;
    T0RedRuleHistoryOption? selectedOption;
    for (final option in options) {
      if (option.id == selectedId) {
        selectedOption = option;
        break;
      }
    }
    final stocks = vm.redRuleHistoryStocks;

    return SafeArea(
      top: false,
      child: Column(
        children: [
          Padding(
            padding: const EdgeInsets.fromLTRB(16, 12, 16, 4),
            child: Row(
              children: [
                Expanded(
                  child: DropdownButtonFormField<String>(
                    initialValue: selectedId,
                    isExpanded: true,
                    decoration: const InputDecoration(
                      labelText: '红策条件组合',
                      border: OutlineInputBorder(),
                      contentPadding: EdgeInsets.symmetric(
                        horizontal: 12,
                        vertical: 10,
                      ),
                    ),
                    hint: const Text('请选择条件组合'),
                    items: options
                        .map(
                          (option) => DropdownMenuItem<String>(
                            value: option.id,
                            child: Text(
                              option.matchCountKnown
                                  ? '${option.name} · ${option.matchCount}次'
                                  : '${option.name} · 待加载',
                              maxLines: 1,
                              overflow: TextOverflow.ellipsis,
                            ),
                          ),
                        )
                        .toList(),
                    onChanged: vm.redRuleHistoryOptionsLoading
                        ? null
                        : (id) {
                            for (final option in options) {
                              if (option.id == id) {
                                vm.loadRedRuleHistoryStocks(option);
                                break;
                              }
                            }
                          },
                  ),
                ),
                const SizedBox(width: 8),
                IconButton(
                  tooltip: switch (vm.redRuleHistorySort) {
                    RedRuleHistorySort.original => 'T0日内浮盈：默认排序',
                    RedRuleHistorySort.descending => 'T0日内浮盈：高到低',
                    RedRuleHistorySort.ascending => 'T0日内浮盈：低到高',
                  },
                  onPressed: vm.toggleRedRuleHistorySort,
                  color: vm.redRuleHistorySort == RedRuleHistorySort.original
                      ? AppColors.textSecondary
                      : Theme.of(context).colorScheme.primary,
                  icon: Icon(switch (vm.redRuleHistorySort) {
                    RedRuleHistorySort.original => Icons.sort,
                    RedRuleHistorySort.descending => Icons.arrow_downward,
                    RedRuleHistorySort.ascending => Icons.arrow_upward,
                  }),
                ),
              ],
            ),
          ),
          Expanded(
            child: vm.redRuleHistoryOptionsLoading && options.isEmpty
                ? Center(
                    child: Column(
                      mainAxisSize: MainAxisSize.min,
                      children: [
                        const CircularProgressIndicator(),
                        const SizedBox(height: 16),
                        Text(
                          '正在读取条件列表，历史命中按所选条件加载',
                          style: TextStyle(
                            fontSize: 12,
                            color: AppColors.textTertiary,
                          ),
                        ),
                      ],
                    ),
                  )
                : vm.redRuleHistoryError != null && options.isEmpty
                ? Center(
                    child: Column(
                      mainAxisSize: MainAxisSize.min,
                      children: [
                        Padding(
                          padding: const EdgeInsets.all(24),
                          child: Text(
                            '加载红策条件失败：${vm.redRuleHistoryError}',
                            textAlign: TextAlign.center,
                          ),
                        ),
                        TextButton(
                          onPressed: () =>
                              vm.loadRedRuleHistoryOptions(force: true),
                          child: const Text('重试'),
                        ),
                      ],
                    ),
                  )
                : selectedOption == null
                ? Center(
                    child: Text(
                      options.isEmpty ? '暂无红策条件组合' : '请选择红策条件组合',
                      style: TextStyle(
                        fontSize: 14,
                        color: AppColors.textTertiary,
                      ),
                    ),
                  )
                : vm.redRuleHistoryResultsLoading
                ? const Center(child: CircularProgressIndicator())
                : vm.redRuleHistoryError != null
                ? Center(
                    child: Column(
                      mainAxisSize: MainAxisSize.min,
                      children: [
                        Padding(
                          padding: const EdgeInsets.all(24),
                          child: Text(
                            '加载历史命中失败：${vm.redRuleHistoryError}',
                            textAlign: TextAlign.center,
                          ),
                        ),
                        TextButton(
                          onPressed: () =>
                              vm.loadRedRuleHistoryStocks(selectedOption!),
                          child: const Text('重试'),
                        ),
                      ],
                    ),
                  )
                : stocks.isEmpty
                ? Center(
                    child: Text(
                      vm.redRuleHistoryIncomplete
                          ? '历史数据尚不完整，当前可用数据中暂未查到命中'
                          : '暂无符合该条件的历史股票',
                      style: TextStyle(
                        fontSize: 14,
                        color: AppColors.textTertiary,
                      ),
                    ),
                  )
                : ListView.builder(
                    padding: const EdgeInsets.symmetric(
                      horizontal: 16,
                      vertical: 8,
                    ),
                    itemCount: stocks.length,
                    itemBuilder: (_, index) => _buildRedRegressionCard(
                      stocks[index],
                      showKline: showKline,
                      klineRepository: klineRepository,
                      showIntraday: showIntraday,
                      intradayRepository: intradayRepository,
                    ),
                  ),
          ),
        ],
      ),
    );
  }

  Widget _buildRedRegressionCard(
    T0StrategyStock stock, {
    required bool showKline,
    required RedKlineRepository? klineRepository,
    required bool showIntraday,
    required RedT1IntradayRepository? intradayRepository,
  }) {
    final date = stock.tradeDate;
    const moduleCode = t0SharedGobRedStrategyModuleCode;
    final klineFuture = showKline && klineRepository != null
        ? _redRegressionKlineFor(klineRepository, date, stock.stockCode)
        : null;
    final loadIntraday = showIntraday && intradayRepository != null;

    Widget buildCard(
      RedT1IntradaySeries? series,
      bool intradayLoading,
      String? intradayError,
      bool waitingForExposure,
    ) => FutureBuilder<Map<String, RedKlineWindow>>(
      future: klineFuture,
      builder: (_, klineSnapshot) => _buildStrategyCard(
        stock,
        kind: StrategyCardVariant.red,
        selectedDate: date,
        historyDate: date,
        redKlineEnabled: klineFuture != null,
        redKlineLoading:
            klineFuture != null &&
            klineSnapshot.connectionState != ConnectionState.done,
        redKlineError: klineSnapshot.hasError,
        redKlineWindow: klineSnapshot.data?[stock.stockCode],
        redT1IntradayEnabled: loadIntraday,
        redT1IntradayLoading: intradayLoading,
        redT1IntradayError: intradayError,
        redT1IntradayIdleMessage: waitingForExposure ? '滚动到卡片后自动加载分时' : null,
        redT1IntradaySeries: series,
        loadRedT1Session: loadIntraday && intradayRepository != null
            ? (sessionDate) => intradayRepository.loadSession(
                date,
                stock.stockCode,
                sessionDate,
                moduleCode: moduleCode,
              )
            : null,
        loadRedKlineDate: klineFuture != null && klineRepository != null
            ? (klineDate) async => (await klineRepository.load(klineDate, [
                stock.stockCode,
              ], moduleCode: moduleCode))[stock.stockCode]
            : null,
      ),
    );

    if (!loadIntraday) return buildCard(null, false, null, false);
    final detectorId =
        'regression:$moduleCode:$date:${stock.stockCode}:$_chartRefreshRevision';
    return RedT1IntradayExposure(
      key: ValueKey(detectorId),
      detectorId: detectorId,
      load: () async {
        final items = await intradayRepository!.load(date, [
          stock.stockCode,
        ], moduleCode: moduleCode);
        return items[stock.stockCode] ??
            RedT1IntradaySeries(
              code: stock.stockCode,
              date: date,
              bars: const [],
              prevClose: 0,
              source: '',
              status: 'missing_series',
              message: '分时接口未返回该股票的数据条目',
            );
      },
      builder: (_, series, loading, error, waiting) =>
          buildCard(series, loading, error, waiting),
    );
  }

  Future<Map<String, RedKlineWindow>> _redRegressionKlineFor(
    RedKlineRepository repository,
    String date,
    String code,
  ) {
    const moduleCode = t0SharedGobRedStrategyModuleCode;
    final key = '$moduleCode:$date:$code';
    if (!_redRegressionKlineFutures.containsKey(key)) {
      _trimRegressionChartFutures(_redRegressionKlineFutures);
    }
    return _redRegressionKlineFutures.putIfAbsent(
      key,
      () => _loadRegressionKline(repository, date, code, key),
    );
  }

  Future<Map<String, RedKlineWindow>> _loadRegressionKline(
    RedKlineRepository repository,
    String date,
    String code,
    String key,
  ) async {
    try {
      return await repository.load(date, [
        code,
      ], moduleCode: t0SharedGobRedStrategyModuleCode);
    } catch (_) {
      _redRegressionKlineFutures.remove(key);
      rethrow;
    }
  }

  void _trimRegressionChartFutures<T>(Map<String, Future<T>> futures) {
    while (futures.length >= 100) {
      futures.remove(futures.keys.first);
    }
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
                  SliverToBoxAdapter(
                    child: Center(
                      child: Padding(
                        padding: EdgeInsets.all(60),
                        child: Column(
                          mainAxisAlignment: MainAxisAlignment.center,
                          children: [
                            Icon(
                              Icons.radar,
                              size: 64,
                              color: AppColors.textTertiary,
                            ),
                            SizedBox(height: 16),
                            Text(
                              '暂无监控股票',
                              style: TextStyle(
                                fontSize: 16,
                                color: AppColors.textSecondary,
                                fontWeight: FontWeight.w500,
                              ),
                            ),
                            SizedBox(height: 8),
                            Text(
                              '在上方搜索框搜索股票并添加监控',
                              style: TextStyle(
                                fontSize: 13,
                                color: AppColors.textTertiary,
                              ),
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
          hintStyle: TextStyle(fontSize: 14),
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
                icon: Icon(
                  Icons.settings,
                  size: 20,
                  color: AppColors.buttonPrimary,
                ),
                onPressed: () async {
                  final changed = await Navigator.of(context).push<bool>(
                    MaterialPageRoute(
                      builder: (_) => const MonitorSettingsPage(),
                    ),
                  );
                  if (changed == true && mounted) {
                    // 设置已变更，刷新所有数据
                    final vm = _radarViewModel;
                    _loadedModuleCodes.remove('radar.watch_changes');
                    _loadedModuleCodes.remove('radar.all_changes');
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
          fillColor: AppColors.inputFill,
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
    final changeColor = isUp ? AppColors.textPriceUp : AppColors.textPriceDown;
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
          border: Border.all(color: AppColors.border),
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
                        style: TextStyle(
                          fontSize: 12,
                          color: AppColors.textSecondary,
                        ),
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
                      style: TextStyle(
                        fontSize: 12,
                        color: AppColors.textSecondary,
                      ),
                    ),
                  // 最新异动描述
                  Builder(
                    builder: (_) {
                      final desc = vm.getLatestAlertDescription(stock.code);
                      if (desc == null) return const SizedBox.shrink();
                      return Padding(
                        padding: const EdgeInsets.only(top: 4),
                        child: Row(
                          children: [
                            Container(
                              padding: const EdgeInsets.symmetric(
                                horizontal: 5,
                                vertical: 2,
                              ),
                              decoration: BoxDecoration(
                                color: AppColors.tagRed,
                                borderRadius: BorderRadius.circular(3),
                              ),
                              child: const Text(
                                '异动',
                                style: TextStyle(
                                  color: Colors.white,
                                  fontSize: 10,
                                ),
                              ),
                            ),
                            const SizedBox(width: 6),
                            Icon(
                              Icons.warning_amber_rounded,
                              size: 12,
                              color: AppColors.warning,
                            ),
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
                    },
                  ),
                ],
              ),
            ),
            if (vm.hasSellWarning(stock.code))
              _buildSellWarningBadge(vm, stock.code),
            // X（移除监控）
            IconButton(
              icon: Icon(Icons.close, size: 18, color: AppColors.textTertiary),
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

  Widget _buildSellWarningBadge(RadarViewModel vm, String code) {
    return Tooltip(
      message: '点击后今日不再显示',
      child: Material(
        color: Colors.transparent,
        child: InkWell(
          onTap: () => vm.dismissSellWarning(code),
          borderRadius: BorderRadius.circular(999),
          child: Container(
            margin: const EdgeInsets.only(right: 4),
            padding: const EdgeInsets.symmetric(horizontal: 10, vertical: 5),
            decoration: BoxDecoration(
              color: AppColors.tagRed,
              borderRadius: BorderRadius.circular(999),
            ),
            child: const Text(
              '卖出',
              style: TextStyle(
                color: Colors.white,
                fontSize: 12,
                fontWeight: FontWeight.w700,
              ),
            ),
          ),
        ),
      ),
    );
  }

  Future<void> _openInTongHuaShun(BuildContext context, String code) async {
    final opened = await StockLauncher.openTongHuaShun(code: code);
    if (!opened && context.mounted) {
      ScaffoldMessenger.of(
        context,
      ).showSnackBar(const SnackBar(content: Text('未能打开同花顺或浏览器')));
    }
  }

  // ─── 策略 Tab 共用内容 ──────────────────────────────────

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

  List<String> _strategyDateOptions(
    T0StrategyViewModel vm,
    String moduleCode,
    StrategyCardVariant kind,
  ) {
    return vm.dropdownDatesFor(moduleCode);
  }

  Widget _buildStrategyDateBar(
    T0StrategyViewModel vm, {
    required String moduleCode,
    StrategyCardVariant kind = StrategyCardVariant.main,
  }) {
    const dateBarFontSize = 16.0;
    const dateBarDateFontSize = 17.0;
    final state = vm.stateFor(moduleCode);
    if (!vm.showDateSelectorFor(moduleCode)) return const SizedBox.shrink();
    final dropdownDates = _strategyDateOptions(vm, moduleCode, kind);
    final selectedDate = dropdownDates.contains(state.selectedDate)
        ? state.selectedDate
        : null;
    final selectedDateIndex = selectedDate == null
        ? -1
        : dropdownDates.indexOf(selectedDate);
    final canGoPrevious = kind == StrategyCardVariant.purple
        ? selectedDateIndex >= 0 &&
              selectedDateIndex + 1 < dropdownDates.length &&
              !state.loading
        : vm.canGoPreviousArchiveFor(moduleCode);
    final canGoNext = kind == StrategyCardVariant.purple
        ? selectedDateIndex > 0 && !state.loading
        : vm.canGoNextArchiveFor(moduleCode);
    return Container(
      width: double.infinity,
      padding: const EdgeInsets.symmetric(horizontal: 16, vertical: 4),
      color: AppColors.cardBg,
      child: Wrap(
        crossAxisAlignment: WrapCrossAlignment.center,
        runSpacing: 4,
        children: [
          Text(
            '当前显示',
            style: TextStyle(
              fontSize: dateBarFontSize,
              color: AppColors.textSecondary,
            ),
          ),
          const SizedBox(width: 8),
          TextButton(
            onPressed: canGoPrevious
                ? kind == StrategyCardVariant.purple
                      ? () => vm.selectDate(
                          moduleCode,
                          dropdownDates[selectedDateIndex + 1],
                        )
                      : () => vm.selectPreviousArchive(moduleCode)
                : null,
            style: TextButton.styleFrom(
              padding: const EdgeInsets.symmetric(horizontal: 10),
              minimumSize: Size.zero,
              tapTargetSize: MaterialTapTargetSize.shrinkWrap,
            ),
            child: Text(
              '前一天',
              style: TextStyle(
                fontSize: dateBarFontSize,
                color: canGoPrevious
                    ? AppColors.textSecondary
                    : AppColors.textSecondary.withValues(alpha: 0.4),
              ),
            ),
          ),
          const SizedBox(width: 4),
          DropdownButton<String>(
            value: selectedDate,
            hint: kind == StrategyCardVariant.purple && selectedDate == null
                ? const Text('选择日期', style: TextStyle(fontSize: 15))
                : null,
            underline: const SizedBox.shrink(),
            style: TextStyle(
              fontSize: dateBarDateFontSize,
              fontWeight: FontWeight.w600,
              color: AppColors.textPrimary,
            ),
            items: dropdownDates
                .map((d) => DropdownMenuItem(value: d, child: Text(d)))
                .toList(),
            onChanged: (d) {
              if (d != null) vm.selectDate(moduleCode, d);
            },
          ),
          const SizedBox(width: 4),
          TextButton(
            onPressed: canGoNext
                ? kind == StrategyCardVariant.purple
                      ? () => vm.selectDate(
                          moduleCode,
                          dropdownDates[selectedDateIndex - 1],
                        )
                      : () => vm.selectNextArchive(moduleCode)
                : null,
            style: TextButton.styleFrom(
              padding: const EdgeInsets.symmetric(horizontal: 10),
              minimumSize: Size.zero,
              tapTargetSize: MaterialTapTargetSize.shrinkWrap,
            ),
            child: Text(
              '后一天',
              style: TextStyle(
                fontSize: dateBarFontSize,
                color: canGoNext
                    ? AppColors.textSecondary
                    : AppColors.textSecondary.withValues(alpha: 0.4),
              ),
            ),
          ),
          const SizedBox(width: 8),
          Text(
            '选股结果',
            style: TextStyle(
              fontSize: dateBarFontSize,
              color: AppColors.textSecondary,
            ),
          ),
        ],
      ),
    );
  }

  Widget _buildStrategyTab(
    T0StrategyViewModel vm, {
    required String moduleCode,
    StrategyCardVariant kind = StrategyCardVariant.main,
    bool showKline = false,
    RedKlineRepository? klineRepository,
    bool showIntraday = false,
    RedT1IntradayRepository? intradayRepository,
  }) {
    final state = vm.stateFor(moduleCode);
    final stocks = switch (kind) {
      StrategyCardVariant.main => vm.resultsFor(moduleCode),
      StrategyCardVariant.red => vm.resultsFor(moduleCode),
      StrategyCardVariant.gold => vm.resultsFor(moduleCode),
      StrategyCardVariant.purple => vm.purpleResultsFor(moduleCode),
      StrategyCardVariant.blue => vm.blueResultsFor(moduleCode),
    };
    final emptyText = state.noData
        ? '无数据'
        : switch (kind) {
            StrategyCardVariant.main => '暂无符合条件的股票',
            StrategyCardVariant.red => '暂无符合红策条件的股票',
            StrategyCardVariant.gold => '暂无符合金策条件的股票',
            StrategyCardVariant.purple => '暂无符合紫策条件的股票',
            StrategyCardVariant.blue => '暂无蓝色灯股票',
          };
    final wp = state.warmProgress;
    final chartDate = state.displayDate ?? state.selectedDate;
    final chartFuture =
        !state.loading &&
            wp == null &&
            state.error == null &&
            showKline &&
            kind != StrategyCardVariant.main &&
            klineRepository != null &&
            chartDate != null &&
            stocks.isNotEmpty
        ? klineRepository.load(
            chartDate,
            stocks.map((stock) => stock.stockCode).toList(),
            moduleCode: moduleCode,
          )
        : null;
    final canLoadIntraday =
        !state.loading &&
        wp == null &&
        state.error == null &&
        kind == StrategyCardVariant.red &&
        showIntraday &&
        intradayRepository != null &&
        chartDate != null &&
        stocks.isNotEmpty;

    final mainChartsEnabled =
        kind == StrategyCardVariant.main &&
        chartDate != null &&
        ((showKline && klineRepository != null) ||
            (showIntraday && intradayRepository != null));
    final canRefreshCharts = canLoadIntraday || mainChartsEnabled;

    // 预热进度 / 等待中
    if (wp != null) {
      return SafeArea(
        top: false,
        child: Column(
          crossAxisAlignment: CrossAxisAlignment.stretch,
          children: [
            _buildStrategyDateBar(vm, moduleCode: moduleCode, kind: kind),
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
                          fontSize: 15,
                          fontWeight: FontWeight.w500,
                        ),
                      ),
                      if (_backfillSubtitle(wp) != null)
                        Padding(
                          padding: const EdgeInsets.only(top: 8),
                          child: Text(
                            _backfillSubtitle(wp)!,
                            style: TextStyle(
                              fontSize: 13,
                              color: AppColors.textSecondary,
                            ),
                          ),
                        ),
                      const SizedBox(height: 12),
                      if (wp.stockCount > 0)
                        Text(
                          '已拉取 ${wp.stockCount} 只主板股票',
                          style: TextStyle(
                            fontSize: 13,
                            color: AppColors.textSecondary,
                          ),
                        ),
                      if (wp.dailyFetched > 0)
                        Padding(
                          padding: const EdgeInsets.only(top: 4),
                          child: Text(
                            '日线进度: ${wp.dailyFetched}/${wp.dailyTotal}',
                            style: TextStyle(
                              fontSize: 13,
                              color: AppColors.textSecondary,
                            ),
                          ),
                        ),
                      if (wp.candidateCount > 0)
                        Padding(
                          padding: const EdgeInsets.only(top: 4),
                          child: Text(
                            '候选股票: ${wp.candidateCount} 只（涨停 + 成交额过滤后）',
                            style: TextStyle(
                              fontSize: 13,
                              color: AppColors.textSecondary,
                            ),
                          ),
                        ),
                      const SizedBox(height: 16),
                      Text(
                        '每10秒自动刷新',
                        style: TextStyle(
                          fontSize: 12,
                          color: AppColors.textTertiary,
                        ),
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
      child: state.loading
          ? const Center(child: CircularProgressIndicator())
          : state.error != null
          ? Column(
              crossAxisAlignment: CrossAxisAlignment.stretch,
              children: [
                _buildStrategyDateBar(vm, moduleCode: moduleCode, kind: kind),
                Expanded(
                  child: Center(
                    child: SingleChildScrollView(
                      padding: const EdgeInsets.all(24),
                      child: Column(
                        mainAxisSize: MainAxisSize.min,
                        children: [
                          Text(
                            '加载失败: ${state.error}',
                            textAlign: TextAlign.center,
                            style: TextStyle(
                              fontSize: 14,
                              color: AppColors.textSecondary,
                            ),
                          ),
                          const SizedBox(height: 12),
                          TextButton.icon(
                            icon: const Icon(Icons.refresh),
                            label: const Text('重试'),
                            onPressed: () => vm.loadResults(
                              moduleCode: moduleCode,
                              date: state.selectedDate,
                              archived:
                                  state.selectedDate != null &&
                                  !vm.isTodayDate(state.selectedDate!),
                            ),
                          ),
                        ],
                      ),
                    ),
                  ),
                ),
              ],
            )
          : Column(
              crossAxisAlignment: CrossAxisAlignment.stretch,
              children: [
                _buildStrategyDateBar(vm, moduleCode: moduleCode, kind: kind),
                if (kind == StrategyCardVariant.red && state.dataIncomplete)
                  Padding(
                    padding: const EdgeInsets.symmetric(
                      horizontal: 16,
                      vertical: 6,
                    ),
                    child: Text(
                      state.dataWarningMessage ??
                          '红策数据不完整，仅显示已确认命中；列表数量不是完整统计',
                      style: TextStyle(fontSize: 12, color: AppColors.warning),
                    ),
                  ),
                Expanded(
                  child: stocks.isEmpty
                      ? Center(
                          child: Padding(
                            padding: EdgeInsets.symmetric(
                              horizontal: kind == StrategyCardVariant.red
                                  ? 24
                                  : 0,
                            ),
                            child: kind == StrategyCardVariant.red
                                ? RedEmptyReminder(
                                    revision: vm.resultsRevisionFor(moduleCode),
                                  )
                                : Text(
                                    emptyText,
                                    textAlign: TextAlign.center,
                                    style: TextStyle(
                                      fontSize: 14,
                                      color: AppColors.textTertiary,
                                    ),
                                  ),
                          ),
                        )
                      : FutureBuilder<Map<String, RedKlineWindow>>(
                          future: chartFuture,
                          builder: (_, snapshot) {
                            final listView = ListView.builder(
                              physics: canRefreshCharts
                                  ? const AlwaysScrollableScrollPhysics()
                                  : null,
                              padding: const EdgeInsets.symmetric(
                                horizontal: 16,
                                vertical: 8,
                              ),
                              itemCount: stocks.length,
                              itemBuilder: (_, i) {
                                final stock = stocks[i];
                                if (mainChartsEnabled) {
                                  return _buildStrategyCard(
                                    stock,
                                    kind: kind,
                                    preview:
                                        state.showingCandidatePreview ||
                                        state.showingTailCandidatePreview,
                                    tailPreview:
                                        state.showingTailCandidatePreview,
                                    selectedDate: chartDate,
                                    selectedDateIsToday: vm.isTodayDate(
                                      chartDate,
                                    ),
                                    charts: StrategyStockCharts(
                                      key: ValueKey(
                                        'main-charts:$moduleCode:$chartDate:${stock.stockCode}:$_chartRefreshRevision',
                                      ),
                                      moduleCode: moduleCode,
                                      date: chartDate,
                                      code: stock.stockCode,
                                      showKline: showKline,
                                      showIntraday: showIntraday,
                                      klineRepository: klineRepository,
                                      intradayRepository: intradayRepository,
                                      revision: _chartRefreshRevision,
                                    ),
                                  );
                                }

                                Widget buildCard(
                                  RedT1IntradaySeries? series,
                                  bool intradayLoading,
                                  String? intradayError,
                                  bool waitingForExposure,
                                ) => _buildStrategyCard(
                                  stock,
                                  preview:
                                      state.showingCandidatePreview ||
                                      state.showingTailCandidatePreview,
                                  tailPreview:
                                      state.showingTailCandidatePreview,
                                  kind: kind,
                                  selectedDate: chartDate,
                                  selectedDateIsToday: vm.isTodayDate(
                                    chartDate,
                                  ),
                                  redKlineEnabled: chartFuture != null,
                                  redKlineLoading:
                                      chartFuture != null &&
                                      snapshot.connectionState !=
                                          ConnectionState.done,
                                  redKlineWindow:
                                      snapshot.data?[stock.stockCode],
                                  redKlineError: snapshot.hasError,
                                  redT1IntradayEnabled: canLoadIntraday,
                                  redT1IntradayLoading: intradayLoading,
                                  redT1IntradayError: intradayError,
                                  redT1IntradayIdleMessage: waitingForExposure
                                      ? '滚动到卡片后自动加载分时'
                                      : null,
                                  redT1IntradaySeries: series,
                                  loadRedT1Session:
                                      canLoadIntraday &&
                                          intradayRepository != null &&
                                          chartDate != null
                                      ? (sessionDate) =>
                                            intradayRepository.loadSession(
                                              chartDate,
                                              stock.stockCode,
                                              sessionDate,
                                              moduleCode: moduleCode,
                                            )
                                      : null,
                                  loadRedKlineDate:
                                      chartFuture != null &&
                                          klineRepository != null
                                      ? (date) async =>
                                            (await klineRepository.load(
                                              date,
                                              [stock.stockCode],
                                              moduleCode: moduleCode,
                                            ))[stock.stockCode]
                                      : null,
                                );

                                if (!canLoadIntraday) {
                                  return buildCard(null, false, null, false);
                                }
                                final detectorId =
                                    'list:$moduleCode:$chartDate:${stock.stockCode}:$_chartRefreshRevision';
                                return RedT1IntradayExposure(
                                  key: ValueKey(detectorId),
                                  detectorId: detectorId,
                                  load: () async {
                                    final items = await intradayRepository!
                                        .load(chartDate!, [
                                          stock.stockCode,
                                        ], moduleCode: moduleCode);
                                    return items[stock.stockCode] ??
                                        RedT1IntradaySeries(
                                          code: stock.stockCode,
                                          date: chartDate!,
                                          bars: const [],
                                          prevClose: 0,
                                          source: '',
                                          status: 'missing_series',
                                          message: '分时接口未返回该股票的数据条目',
                                        );
                                  },
                                  builder:
                                      (_, series, loading, error, waiting) =>
                                          buildCard(
                                            series,
                                            loading,
                                            error,
                                            waiting,
                                          ),
                                );
                              },
                            );
                            if (canRefreshCharts) {
                              return RefreshIndicator(
                                onRefresh: () async {
                                  intradayRepository?.clearCache(
                                    moduleCode: moduleCode,
                                  );
                                  if (mainChartsEnabled) {
                                    klineRepository?.clearCache(
                                      moduleCode: moduleCode,
                                    );
                                  }
                                  setState(() {
                                    _chartRefreshRevision++;
                                  });
                                  await Future<void>.delayed(
                                    const Duration(milliseconds: 400),
                                  );
                                },
                                child: listView,
                              );
                            }
                            return listView;
                          },
                        ),
                ),
              ],
            ),
    );
  }

  Widget _buildStrategyCard(
    T0StrategyStock stock, {
    bool preview = false,
    bool tailPreview = false,
    required StrategyCardVariant kind,
    String? selectedDate,
    bool selectedDateIsToday = false,
    bool redKlineEnabled = false,
    bool redKlineLoading = false,
    bool redKlineError = false,
    RedKlineWindow? redKlineWindow,
    bool redT1IntradayEnabled = false,
    bool redT1IntradayLoading = false,
    String? redT1IntradayError,
    String? redT1IntradayIdleMessage,
    RedT1IntradaySeries? redT1IntradaySeries,
    Future<RedT1IntradaySeries> Function(String date)? loadRedT1Session,
    Future<RedKlineWindow?> Function(String date)? loadRedKlineDate,
    String? historyDate,
    Widget? charts,
  }) {
    return StrategyStockCard(
      stock: stock,
      kind: kind,
      preview: preview,
      tailPreview: tailPreview,
      selectedDate: selectedDate,
      selectedDateIsToday: selectedDateIsToday,
      redKlineWindow: redKlineWindow,
      historyDate: historyDate,
      onTap: () => _openInTongHuaShun(context, stock.rawCode),
      charts:
          charts ??
          (kind == StrategyCardVariant.red &&
                  (redKlineEnabled || redT1IntradayEnabled)
              ? StrategyCharts(
                  key: ValueKey('red-charts:${stock.stockCode}:$selectedDate'),
                  code: stock.stockCode,
                  t0Date: selectedDate ?? '',
                  showKline: redKlineEnabled,
                  window: redKlineWindow,
                  klineLoading: redKlineLoading,
                  klineError: redKlineError,
                  showIntraday: redT1IntradayEnabled,
                  series: redT1IntradaySeries,
                  intradayLoading: redT1IntradayLoading,
                  intradayErrorMessage: redT1IntradayError,
                  intradayIdleMessage: redT1IntradayIdleMessage,
                  loadSession: loadRedT1Session,
                  loadKlineWindow: loadRedKlineDate,
                )
              : null),
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
              color: AppColors.scaffoldBg,
              child: Row(
                children: [
                  Text(
                    '共 ${changes.length} 条',
                    style: TextStyle(
                      fontSize: 13,
                      color: AppColors.textSecondary,
                    ),
                  ),
                  const Spacer(),
                  GestureDetector(
                    onTap: onToggleSort,
                    child: Row(
                      children: [
                        Icon(
                          ascending ? Icons.arrow_upward : Icons.arrow_downward,
                          size: 16,
                          color: AppColors.brand,
                        ),
                        const SizedBox(width: 4),
                        Text(
                          ascending ? '正序' : '倒序',
                          style: TextStyle(
                            fontSize: 13,
                            color: AppColors.brand,
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
                      style: TextStyle(
                        fontSize: 14,
                        color: AppColors.textTertiary,
                      ),
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
        final cmp = '${a.changeDate}${a.changeTime}'.compareTo(
          '${b.changeDate}${b.changeTime}',
        );
        return ascending ? cmp : -cmp;
      });

    final vm = context.read<RadarViewModel>();

    return RefreshIndicator(
      onRefresh: () async {
        if (_tabController.index == 4) {
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
