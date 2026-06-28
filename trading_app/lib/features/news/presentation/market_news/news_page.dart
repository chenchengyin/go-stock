/// 市场快讯页面 — iOS 原生资讯风格
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

class _NewsPageState extends State<NewsPage>
    with SingleTickerProviderStateMixin {
  int _selectedTab = 0;
  late AnimationController _refreshController;

  @override
  void initState() {
    super.initState();
    _refreshController = AnimationController(
      vsync: this,
      duration: const Duration(milliseconds: 600),
    );
    WidgetsBinding.instance.addPostFrameCallback((_) {
      context.read<NewsViewModel>().load();
    });
  }

  @override
  void dispose() {
    _refreshController.dispose();
    super.dispose();
  }

  Future<void> _onRefresh() async {
    _refreshController.repeat();
    final vm = context.read<NewsViewModel>();
    await vm.refresh();
    _refreshController.stop();
    _refreshController.reset();
  }

  void _openTopic(HotTopicItem item) {
    Navigator.of(
      context,
    ).push(CupertinoPageRoute(builder: (_) => HotTopicDetailPage(item: item)));
  }

  @override
  Widget build(BuildContext context) {
    final vm = context.watch<NewsViewModel>();
    final tabs = ['重要', '国内资讯', '热门话题', '外媒'];

    return Container(
      color: CupertinoColors.white,
      child: SafeArea(
        bottom: false,
        child: Column(
          children: [
            _Header(
              onRefresh: _onRefresh,
              refreshController: _refreshController,
            ),
          _TabBar(
            tabs: tabs,
            selectedIndex: _selectedTab,
            onSelect: (i) => setState(() {
              _selectedTab = i;
              // 切到"国内资讯"tab 时异步加载
              if (i == 1 && vm.domesticNews.isEmpty) {
                context.read<NewsViewModel>().loadDomesticNews();
              }
            }),
          ),
          Expanded(child: _buildBody(vm)),
          ],
        ),
      ),
    );
  }

  Widget _buildBody(NewsViewModel vm) {
    if (vm.state.isLoading && vm.totalCount == 0 && vm.hotTopics.isEmpty) {
      return const Center(child: CupertinoActivityIndicator());
    }

    if (vm.state.status.name == 'error' &&
        vm.totalCount == 0 &&
        vm.hotTopics.isEmpty) {
      return Center(
        child: Column(
          mainAxisAlignment: MainAxisAlignment.center,
          children: [
            const Icon(
              CupertinoIcons.exclamationmark_triangle,
              size: 48,
              color: CupertinoColors.systemGrey,
            ),
            const SizedBox(height: 16),
            Text(
              vm.state.message,
              style: const TextStyle(color: CupertinoColors.systemGrey),
            ),
            const SizedBox(height: 16),
            CupertinoButton.filled(
              onPressed: _onRefresh,
              child: const Text('重试'),
            ),
          ],
        ),
      );
    }

    return _buildTabContent(vm, _selectedTab);
  }

  Widget _buildTabContent(NewsViewModel vm, int index) {
    switch (index) {
      case 0:
        return _buildNewsList(vm.importantNews);
      case 1:
        return _buildNewsList(vm.domesticNews);
      case 2:
        return _buildHotTopicsList(vm.hotTopics);
      case 3:
        return _buildNewsList(vm.foreignNews);
      default:
        return _buildNewsList(vm.importantNews);
    }
  }

  Widget _buildNewsList(List<NewsItem> items) {
    if (items.isEmpty) {
      return const Center(
        child: Text(
          '暂无数据',
          style: TextStyle(color: CupertinoColors.systemGrey),
        ),
      );
    }
    return ListView.builder(
      padding: EdgeInsets.zero,
      itemCount: items.length,
      itemBuilder: (_, i) => NewsCard(item: items[i]),
    );
  }

  Widget _buildHotTopicsList(List<HotTopicItem> items) {
    if (items.isEmpty) {
      return const Center(
        child: Text(
          '暂无热门话题',
          style: TextStyle(color: CupertinoColors.systemGrey),
        ),
      );
    }
    return ListView.builder(
      padding: EdgeInsets.zero,
      itemCount: items.length,
      itemBuilder: (_, i) =>
          HotTopicCard(item: items[i], onTap: () => _openTopic(items[i])),
    );
  }
}

class _Header extends StatelessWidget {
  const _Header({
    required this.onRefresh,
    required this.refreshController,
  });

  final VoidCallback onRefresh;
  final AnimationController refreshController;

  @override
  Widget build(BuildContext context) {
    return Container(
      height: 44,
      color: CupertinoColors.white,
      padding: const EdgeInsets.symmetric(horizontal: 8),
      child: Stack(
        alignment: Alignment.center,
        children: [
          const Text(
            '市场快讯',
            style: TextStyle(fontSize: 17, fontWeight: FontWeight.w600),
          ),
          Align(
            alignment: Alignment.centerRight,
            child: GestureDetector(
              onTap: onRefresh,
              child: Padding(
                padding: const EdgeInsets.all(8),
                child: RotationTransition(
                  turns: refreshController,
                  child: const Icon(
                    CupertinoIcons.refresh,
                    size: 22,
                    color: CupertinoColors.systemBlue,
                  ),
                ),
              ),
            ),
          ),
        ],
      ),
    );
  }
}

class _TabBar extends StatelessWidget implements PreferredSizeWidget {
  const _TabBar({
    required this.tabs,
    required this.selectedIndex,
    required this.onSelect,
  });

  final List<String> tabs;
  final int selectedIndex;
  final ValueChanged<int> onSelect;

  @override
  Size get preferredSize => const Size.fromHeight(42);

  @override
  Widget build(BuildContext context) {
    return Container(
      color: CupertinoColors.white,
      child: Column(
        children: [
          SingleChildScrollView(
            scrollDirection: Axis.horizontal,
            padding: const EdgeInsets.symmetric(horizontal: 12),
            child: Row(
              children: List.generate(tabs.length, (i) {
                final isSelected = i == selectedIndex;
                return GestureDetector(
                  onTap: () => onSelect(i),
                  child: Container(
                    padding: const EdgeInsets.symmetric(horizontal: 12),
                    child: Column(
                      mainAxisSize: MainAxisSize.min,
                      children: [
                        const SizedBox(height: 8),
                        Text(
                          tabs[i],
                          style: TextStyle(
                            fontSize: 16,
                            fontWeight: isSelected
                                ? FontWeight.w600
                                : FontWeight.w400,
                            // color: i == 0 && isSelected
                            //     ? CupertinoColors.systemRed: isSelected ?
                            //     CupertinoColors.activeBlue
                            //     : CupertinoColors.darkBackgroundGray,
                            color: i == 0
                                ? CupertinoColors.systemRed: isSelected ?
                            CupertinoColors.activeBlue
                                : CupertinoColors.darkBackgroundGray,
                          ),
                        ),
                        const SizedBox(height: 4),
                        Container(
                          height: 2,
                          width: 24,
                          decoration: BoxDecoration(
                            color: isSelected
                                ? CupertinoColors.systemBlue
                                : CupertinoColors.transparent,
                            borderRadius: BorderRadius.circular(1),
                          ),
                        ),
                        const SizedBox(height: 6),
                      ],
                    ),
                  ),
                );
              }),
            ),
          ),
          Container(height: 0.5, color: CupertinoColors.systemGrey5),
        ],
      ),
    );
  }
}
