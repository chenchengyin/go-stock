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
  int _currentIndex = 0;

  static const _pages = <Widget>[
    RadarPage(),
    NewsPage(),
    HotlistPage(),
    ProfilePage(),
  ];

  static const _items = [
    BottomNavigationBarItem(
      icon: Icon(Icons.radar_outlined),
      activeIcon: Icon(Icons.radar),
      label: '雷达',
    ),
    BottomNavigationBarItem(
      icon: Icon(Icons.article_outlined),
      activeIcon: Icon(Icons.article),
      label: '市场快讯',
    ),
    BottomNavigationBarItem(
      icon: Icon(Icons.local_fire_department_outlined),
      activeIcon: Icon(Icons.local_fire_department),
      label: '热榜',
    ),
    BottomNavigationBarItem(
      icon: Icon(Icons.person_outline),
      activeIcon: Icon(Icons.person),
      label: '我的',
    ),
  ];

  @override
  Widget build(BuildContext context) {
    return Scaffold(
      body: IndexedStack(
        index: _currentIndex,
        children: _pages,
      ),
      bottomNavigationBar: BottomNavigationBar(
        currentIndex: _currentIndex,
        onTap: (index) => setState(() => _currentIndex = index),
        type: BottomNavigationBarType.fixed,
        backgroundColor: Colors.white,
        selectedItemColor: Colors.blue,
        unselectedItemColor: Colors.grey,
        selectedFontSize: 11,
        unselectedFontSize: 11,
        elevation: 0,
        items: _items,
      ),
    );
  }
}
