import 'package:flutter/material.dart';

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
  int _index = 0;

  static const _pages = [
    RadarPage(),
    NewsPage(),
    HotlistPage(),
    ProfilePage(),
  ];

  @override
  Widget build(BuildContext context) {
    return Scaffold(
      body: SafeArea(child: _pages[_index]),
      bottomNavigationBar: NavigationBar(
        selectedIndex: _index,
        onDestinationSelected: (value) => setState(() => _index = value),
        destinations: const [
          NavigationDestination(icon: Icon(Icons.radar), label: '雷达'),
          NavigationDestination(icon: Icon(Icons.article_outlined), label: '市场快讯'),
          NavigationDestination(icon: Icon(Icons.local_fire_department_outlined), label: '热榜'),
          NavigationDestination(icon: Icon(Icons.person_outline), label: '我的'),
        ],
      ),
    );
  }
}

