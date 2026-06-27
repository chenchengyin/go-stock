import 'package:flutter/cupertino.dart';
import 'package:flutter/material.dart';

import '../features/hotlist/presentation/hotlist_list/hotlist_page.dart';
import '../features/news/presentation/market_news/news_page.dart';
import '../features/profile/presentation/profile_page.dart';
import '../features/radar/presentation/radar_list/radar_page.dart';

class AppShell extends StatefulWidget {
  const AppShell({super.key});

  @override
  State<AppShell> createState() => _AppShellState();
}

class _AppShellState extends State<AppShell> {
  static const _pages = <Widget>[
    RadarPage(),
    NewsPage(),
    HotlistPage(),
    ProfilePage(),
  ];

  final _controller = CupertinoTabController(initialIndex: 0);

  @override
  void dispose() {
    _controller.dispose();
    super.dispose();
  }

  @override
  Widget build(BuildContext context) {
    return DefaultTextStyle(
      style: const TextStyle(
        fontFamily: 'PingFang SC',
        fontFamilyFallback: ['Hiragino Sans GB', 'Microsoft YaHei', 'sans-serif'],
        decoration: TextDecoration.none,
      ),
      child: CupertinoTabScaffold(
        controller: _controller,
      resizeToAvoidBottomInset: true,
      backgroundColor: Colors.white,
      tabBar: CupertinoTabBar(
        backgroundColor: CupertinoColors.white,
        activeColor: CupertinoColors.systemBlue,
        inactiveColor: CupertinoColors.systemGrey,
        items: const [
          BottomNavigationBarItem(
            icon: Icon(CupertinoIcons.antenna_radiowaves_left_right),
            label: '雷达',
          ),
          BottomNavigationBarItem(
            icon: Icon(CupertinoIcons.doc_text),
            label: '市场快讯',
          ),
          BottomNavigationBarItem(
            icon: Icon(CupertinoIcons.flame),
            label: '热榜',
          ),
          BottomNavigationBarItem(
            icon: Icon(CupertinoIcons.person),
            label: '我的',
          ),
        ],
      ),
      tabBuilder: (context, index) {
        return CupertinoTabView(
          builder: (_) => _pages[index],
        );
      },
      ),
    );
  }
}
