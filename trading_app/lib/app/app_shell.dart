import 'dart:async';

import 'package:flutter/material.dart';
import 'package:provider/provider.dart';

import '../core/theme/app_colors.dart';
import '../features/permissions/presentation/module_permission_controller.dart';
import '../features/news/presentation/market_news/news_page.dart';
import '../features/news/presentation/market_news/news_view_model.dart';
import '../features/profile/presentation/profile_page.dart';
import '../features/radar/presentation/radar_list/radar_page.dart';
import '../features/radar/presentation/stock_change_detail/stock_change_detail_page.dart';
import '../features/short_term_emotion/presentation/short_term_emotion_page.dart';

// 策略吧暂时隐藏入口，页面逻辑保留，后续需要时可恢复。
// import '../features/strategy/presentation/strategy_page.dart';

class AppShell extends StatefulWidget {
  const AppShell({super.key});

  static final GlobalKey<NavigatorState> navigatorKey =
      GlobalKey<NavigatorState>();

  static void navigateToStockDetail(String stockCode, String stockName) {
    navigatorKey.currentState?.push(
      MaterialPageRoute(
        builder: (context) =>
            StockChangeDetailPage(stockCode: stockCode, stockName: stockName),
      ),
    );
  }

  @override
  State<AppShell> createState() => _AppShellState();
}

class _AppShellState extends State<AppShell> with WidgetsBindingObserver {
  int _currentIndex = 0;
  final Set<int> _visitedIndexes = {0};

  static Widget _buildPage(int index) {
    return switch (index) {
      0 => const RadarPage(),
      1 => const ShortTermEmotionPage(),
      2 => const NewsPage(),
      3 => const ProfilePage(),
      // 策略吧暂时隐藏入口，原映射保留参考：
      // 3 => const StrategyPage(),
      _ => const SizedBox.shrink(),
    };
  }

  static const _items = [
    BottomNavigationBarItem(
      icon: Icon(Icons.radar_outlined),
      activeIcon: Icon(Icons.radar),
      label: '盘达',
    ),
    BottomNavigationBarItem(
      icon: Icon(Icons.speed_outlined),
      activeIcon: Icon(Icons.speed),
      label: '超短情绪',
    ),
    BottomNavigationBarItem(
      icon: Icon(Icons.article_outlined),
      activeIcon: Icon(Icons.article),
      label: '市场快讯',
    ),
    // 策略吧暂时隐藏入口，后续恢复时插回底部导航。
    // BottomNavigationBarItem(
    //   icon: Icon(Icons.local_fire_department_outlined),
    //   activeIcon: Icon(Icons.local_fire_department),
    //   label: '策略吧',
    // ),
    BottomNavigationBarItem(
      icon: Icon(Icons.person_outline),
      activeIcon: Icon(Icons.person),
      label: '我的',
    ),
  ];

  @override
  void initState() {
    super.initState();
    WidgetsBinding.instance.addObserver(this);
    _checkPendingDeepLink();
  }

  @override
  void dispose() {
    WidgetsBinding.instance.removeObserver(this);
    super.dispose();
  }

  @override
  void didChangeAppLifecycleState(AppLifecycleState state) {
    if (state != AppLifecycleState.resumed || !mounted) return;
    unawaited(context.read<ModulePermissionController>().refresh());
  }

  Future<void> _checkPendingDeepLink() async {
    await Future.delayed(const Duration(milliseconds: 500));
    final pendingCode = _pendingStockCode;
    final pendingName = _pendingStockName;
    if (pendingCode != null && pendingName != null) {
      AppShell.navigateToStockDetail(pendingCode, pendingName);
      _pendingStockCode = null;
      _pendingStockName = null;
    }
  }

  @override
  Widget build(BuildContext context) {
    AppColors.of(context);

    return WillPopScope(
      onWillPop: () async {
        final navigator = AppShell.navigatorKey.currentState;
        if (navigator != null && navigator.canPop()) {
          navigator.pop();
          return false;
        }
        return true;
      },
      child: Navigator(
        key: AppShell.navigatorKey,
        onGenerateRoute: (settings) {
          return MaterialPageRoute(
            builder: (context) => Scaffold(
              body: IndexedStack(
                index: _currentIndex,
                children: List.generate(_items.length, (index) {
                  if (!_visitedIndexes.contains(index)) {
                    return const SizedBox.shrink();
                  }
                  return _buildPage(index);
                }),
              ),
              bottomNavigationBar: BottomNavigationBar(
                currentIndex: _currentIndex,
                onTap: (index) {
                  final newsPageAlreadyVisited = _visitedIndexes.contains(2);
                  setState(() {
                    _currentIndex = index;
                    _visitedIndexes.add(index);
                  });
                  // 首次进入由 NewsPage 自己初始化；已经访问过后，每次
                  // 切回或重复点击都只重新读取服务端缓存，不触发外部抓取。
                  if (index == 2 && newsPageAlreadyVisited) {
                    unawaited(context.read<NewsViewModel>().refresh());
                  }
                },
                type: BottomNavigationBarType.fixed,
                backgroundColor: AppColors.cardBg,
                selectedItemColor: AppColors.brand,
                unselectedItemColor: AppColors.textTertiary,
                selectedFontSize: 11,
                unselectedFontSize: 11,
                elevation: 0,
                items: _items,
              ),
            ),
          );
        },
      ),
    );
  }
}

String? _pendingStockCode;
String? _pendingStockName;

void setPendingDeepLink(String code, String name) {
  _pendingStockCode = code;
  _pendingStockName = name;
}
