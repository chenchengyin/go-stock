/// 市场快讯页面 — 顶部 Tab 切换 重要/热门话题/财联社/新浪/外媒
library;

import 'package:flutter/material.dart';
import 'package:provider/provider.dart';
import 'package:url_launcher/url_launcher.dart';

import '../view_models/news_view_model.dart';
import '../../domain/news_models.dart';
import 'package:trading_app/shared/widgets/news_card.dart';
import '../widgets/hot_topic_card.dart';

class NewsPage extends StatefulWidget {
  const NewsPage({super.key});

  @override
  State<NewsPage> createState() => _NewsPageState();
}

class _NewsPageState extends State<NewsPage>
    with SingleTickerProviderStateMixin, AutomaticKeepAliveClientMixin {
  late TabController _tabController;

  @override
  bool get wantKeepAlive => true;

  @override
  void initState() {
    super.initState();
    _tabController = TabController(length: 5, vsync: this);
    WidgetsBinding.instance.addPostFrameCallback((_) {
      context.read<NewsViewModel>().load();
    });
  }

  @override
  void dispose() {
    _tabController.dispose();
    super.dispose();
  }

  Future<void> _onRefresh() async {
    final vm = context.read<NewsViewModel>();
    await vm.refresh();
  }

  void _openTopic(HotTopicItem item) {
    final url = 'https://gubatopic.eastmoney.com/topic_v3.html?htid=${item.htid}';
    launchUrl(Uri.parse(url), mode: LaunchMode.externalApplication);
  }

  @override
  Widget build(BuildContext context) {
    super.build(context);
    final vm = context.watch<NewsViewModel>();

    return Scaffold(
      appBar: AppBar(
        title: const Text('市场快讯'),
        actions: [
          IconButton(
            icon: const Icon(Icons.refresh),
            tooltip: '刷新',
            onPressed: _onRefresh,
          ),
        ],
        bottom: TabBar(
          controller: _tabController,
          indicatorColor: Colors.red,
          labelColor: Colors.red,
          unselectedLabelColor: Colors.grey,
          isScrollable: true,
          tabs: [
            Tab(text: '重要 (${vm.importantNews.length})'),
            Tab(text: '热门话题 (${vm.hotTopics.length})'),
            Tab(text: '财联社 (${vm.cailianpressNews.length})'),
            Tab(text: '新浪 (${vm.sinaNews.length})'),
            Tab(text: '外媒 (${vm.foreignNews.length})'),
          ],
        ),
      ),
      body: vm.state.isLoading && vm.totalCount == 0 && vm.hotTopics.isEmpty
          ? const Center(child: CircularProgressIndicator())
          : vm.state.status.name == 'error' && vm.totalCount == 0 && vm.hotTopics.isEmpty
              ? Center(
                  child: Column(
                    mainAxisAlignment: MainAxisAlignment.center,
                    children: [
                      const Icon(Icons.error_outline, size: 48,
                          color: Colors.grey),
                      const SizedBox(height: 16),
                      Text(vm.state.message,
                          style: const TextStyle(color: Colors.grey)),
                      const SizedBox(height: 16),
                      ElevatedButton(
                          onPressed: _onRefresh,
                          child: const Text('重试')),
                    ],
                  ),
                )
              : TabBarView(
                  controller: _tabController,
                  children: [
                    _buildTabList(vm.importantNews),
                    _buildHotTopicsTab(vm.hotTopics),
                    _buildTabList(vm.cailianpressNews),
                    _buildTabList(vm.sinaNews),
                    _buildTabList(vm.foreignNews),
                  ],
                ),
    );
  }

  Widget _buildTabList(List<NewsItem> items) {
    if (items.isEmpty) {
      return const Center(child: Text('暂无数据',
          style: TextStyle(color: Colors.grey)));
    }
    return RefreshIndicator(
      onRefresh: _onRefresh,
      child: ListView.builder(
        padding: const EdgeInsets.all(8),
        itemCount: items.length,
        itemBuilder: (_, i) => NewsCard(item: items[i]),
      ),
    );
  }

  Widget _buildHotTopicsTab(List<HotTopicItem> items) {
    if (items.isEmpty) {
      return const Center(child: Text('暂无热门话题',
          style: TextStyle(color: Colors.grey)));
    }
    return RefreshIndicator(
      onRefresh: _onRefresh,
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
