import 'package:flutter/cupertino.dart';

import '../features/hotlist/presentation/pages/hotlist_page.dart';
import '../features/news/presentation/pages/news_page.dart';
import '../features/profile/presentation/pages/profile_page.dart';
import '../features/radar/presentation/pages/radar_page.dart';

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

  @override
  Widget build(BuildContext context) {
    return CupertinoTabScaffold(
      tabBar: CupertinoTabBar(
        activeColor: CupertinoColors.systemRed,
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
    );
  }
}
