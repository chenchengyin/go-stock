/// 市场快讯页面 — Cupertino 风格 Tab 切换
library;

import 'package:flutter/cupertino.dart';
import 'package:provider/provider.dart';

import '../view_models/news_view_model.dart';
import '../../domain/news_models.dart';
import 'package:trading_app/shared/widgets/news_card.dart';
import '../widgets/hot_topic_card.dart';
import 'hot_topic_detail_page.dart';

class NewsPage extends StatefulWidget {
  const NewsPage({super.key});

  @override
  State<NewsPage> createState() => _NewsPageState();
}

class _NewsPageState extends State<NewsPage> {
  int _selectedTab = 0;

  @override
  void initState() {
    super.initState();
    WidgetsBinding.instance.addPostFrameCallback((_) {
      context.read<NewsViewModel>().load();
    });
  }

  Future<void> _onRefresh() async {
    final vm = context.read<NewsViewModel>();
    await vm.refresh();
  }

  void _openTopic(HotTopicItem item) {
    Navigator.of(context).push(
      CupertinoPageRoute(
        builder: (_) => HotTopicDetailPage(item: item),
      ),
    );
  }

  @override
  Widget build(BuildContext context) {
    final vm = context.watch<NewsViewModel>();
    final tabs = [
      '重要 (${vm.importantNews.length})',
      '热门话题 (${vm.hotTopics.length})',
      '财联社 (${vm.cailianpressNews.length})',
      '新浪 (${vm.sinaNews.length})',
      '外媒 (${vm.foreignNews.length})',
    ];

    return CupertinoPageScaffold(
      navigationBar: CupertinoNavigationBar(
        middle: const Text('市场快讯'),
        trailing: CupertinoButton(
          padding: EdgeInsets.zero,
          onPressed: _onRefresh,
          child: const Icon(CupertinoIcons.refresh, size: 22),
        ),
      ),
      child: _buildBody(vm, tabs),
    );
  }

  Widget _buildBody(NewsViewModel vm, List<String> tabs) {
    if (vm.state.isLoading && vm.totalCount == 0 && vm.hotTopics.isEmpty) {
      return const Center(child: CupertinoActivityIndicator());
    }

    if (vm.state.status.name == 'error' && vm.totalCount == 0 && vm.hotTopics.isEmpty) {
      return Center(
        child: Column(
          mainAxisAlignment: MainAxisAlignment.center,
          children: [
            const Icon(CupertinoIcons.exclamationmark_triangle, size: 48,
                color: CupertinoColors.systemGrey),
            const SizedBox(height: 16),
            Text(vm.state.message,
                style: const TextStyle(color: CupertinoColors.systemGrey)),
            const SizedBox(height: 16),
            CupertinoButton.filled(
              onPressed: _onRefresh,
              child: const Text('重试'),
            ),
          ],
        ),
      );
    }

    return SafeArea(
      child: Column(
        children: [
          // iOS 风格分段选择器
          _buildSegmentedControl(tabs),
          // 内容
          Expanded(
            child: _buildTabContent(vm, _selectedTab),
          ),
        ],
      ),
    );
  }

  Widget _buildSegmentedControl(List<String> tabs) {
    return SizedBox(
      width: double.infinity,
      child: CupertinoSegmentedControl<int>(
        groupValue: _selectedTab,
        selectedColor: CupertinoColors.systemRed,
        unselectedColor: CupertinoColors.systemGrey5,
        onValueChanged: (v) => setState(() => _selectedTab = v),
        children: {
          for (var i = 0; i < tabs.length; i++)
            i: Padding(
              padding: const EdgeInsets.symmetric(horizontal: 8, vertical: 6),
              child: Text(tabs[i], style: const TextStyle(fontSize: 12)),
            ),
        },
      ),
    );
  }

  Widget _buildTabContent(NewsViewModel vm, int index) {
    switch (index) {
      case 0:
        return _buildNewsList(vm.importantNews);
      case 1:
        return _buildHotTopicsList(vm.hotTopics);
      case 2:
        return _buildNewsList(vm.cailianpressNews);
      case 3:
        return _buildNewsList(vm.sinaNews);
      case 4:
        return _buildNewsList(vm.foreignNews);
      default:
        return _buildNewsList(vm.importantNews);
    }
  }

  Widget _buildNewsList(List<NewsItem> items) {
    if (items.isEmpty) {
      return const Center(
        child: Text('暂无数据',
            style: TextStyle(color: CupertinoColors.systemGrey)),
      );
    }
    return CupertinoScrollbar(
      child: ListView.builder(
        padding: const EdgeInsets.all(8),
        itemCount: items.length,
        itemBuilder: (_, i) => NewsCard(item: items[i]),
      ),
    );
  }

  Widget _buildHotTopicsList(List<HotTopicItem> items) {
    if (items.isEmpty) {
      return const Center(
        child: Text('暂无热门话题',
            style: TextStyle(color: CupertinoColors.systemGrey)),
      );
    }
    return CupertinoScrollbar(
      child: ListView.builder(
        padding: const EdgeInsets.all(8),
        itemCount: items.length,
        itemBuilder: (_, i) => HotTopicCard(
          item: items[i],
          onTap: () => _openTopic(items[i]),
        ),
      ),
    );
  }
}
