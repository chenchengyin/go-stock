/// 市场快讯页面 — 可滚动 Tab 栏 + 卡片列表
library;

import 'package:flutter/cupertino.dart';
import 'package:provider/provider.dart';

import 'news_view_model.dart';
import '../../domain/news_models.dart';
import 'package:trading_app/shared/widgets/news_card.dart';
import '../hot_topic/hot_topic_card.dart';
import '../hot_topic/hot_topic_detail_page.dart';

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
    final tabs = ['重要', '热门话题', '财联社', '新浪', '外媒'];

    return CupertinoPageScaffold(
      navigationBar: CupertinoNavigationBar(
        middle: const Text('市场快讯'),
        trailing: GestureDetector(
          onTap: _onRefresh,
          child: const Padding(
            padding: EdgeInsets.all(8),
            child: Icon(CupertinoIcons.refresh, size: 20),
          ),
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

    return Column(
      children: [
        _buildTabBar(tabs),
        Expanded(child: _buildTabContent(vm, _selectedTab)),
      ],
    );
  }

  Widget _buildTabBar(List<String> tabs) {
    return Container(
      padding: const EdgeInsets.symmetric(vertical: 8),
      decoration: BoxDecoration(
        color: CupertinoColors.white,
        border: Border(
          bottom: BorderSide(color: CupertinoColors.systemGrey5.withValues(alpha: 0.5)),
        ),
      ),
      child: SingleChildScrollView(
        scrollDirection: Axis.horizontal,
        padding: const EdgeInsets.symmetric(horizontal: 12),
        child: Row(
          children: List.generate(tabs.length, (i) {
            final isSelected = i == _selectedTab;
            return GestureDetector(
              onTap: () => setState(() => _selectedTab = i),
              child: Container(
                padding: const EdgeInsets.symmetric(horizontal: 16, vertical: 6),
                margin: const EdgeInsets.only(right: 8),
                decoration: BoxDecoration(
                  color: isSelected
                      ? const Color(0xffe53935)
                      : CupertinoColors.systemGrey6,
                  borderRadius: BorderRadius.circular(16),
                ),
                child: Text(
                  tabs[i],
                  style: TextStyle(
                    fontSize: 13,
                    fontWeight: isSelected ? FontWeight.w600 : FontWeight.w400,
                    color: isSelected
                        ? CupertinoColors.white
                        : CupertinoColors.black,
                  ),
                ),
              ),
            );
          }),
        ),
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
        padding: const EdgeInsets.only(top: 8, bottom: 16),
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
        padding: const EdgeInsets.only(top: 8, bottom: 16),
        itemCount: items.length,
        itemBuilder: (_, i) => HotTopicCard(
          item: items[i],
          onTap: () => _openTopic(items[i]),
        ),
      ),
    );
  }
}
