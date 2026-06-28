import 'package:flutter/foundation.dart';

import '../../../../shared/view_state.dart';
import '../../data/news_repository.dart';
import '../../domain/news_models.dart';

class NewsViewModel extends ChangeNotifier {
  NewsViewModel(this._repository);

  final NewsRepository _repository;

  ViewState state = const ViewState();

  /// 财联社电报列表
  List<NewsItem> cailianpressNews = [];

  /// 新浪财经列表
  List<NewsItem> sinaNews = [];

  /// 外媒列表
  List<NewsItem> foreignNews = [];

  /// 是否有外媒数据（影响三栏/两栏布局）
  bool get hasForeignNews => foreignNews.isNotEmpty;

  /// 是否有热门话题
  bool get hasHotTopics => hotTopics.isNotEmpty;

  /// 所有新闻总数
  int get totalCount =>
      cailianpressNews.length + sinaNews.length + foreignNews.length;

  /// 国内新闻（财联社+新浪，按时间倒序）
  List<NewsItem> get domesticNews {
    final merged = [...cailianpressNews, ...sinaNews];
    merged.sort((a, b) {
      final aTime = a.dataTime ?? a.time;
      final bTime = b.dataTime ?? b.time;
      return bTime.compareTo(aTime);
    });
    return merged;
  }

  /// 重要新闻汇总（从所有来源中筛选 isRed=true）
  List<NewsItem> get importantNews =>
      [...cailianpressNews, ...sinaNews, ...foreignNews]
          .where((n) => n.isRed)
          .toList();

  /// 热门话题列表
  List<HotTopicItem> hotTopics = [];

  /// 加载所有新闻
  Future<void> load({bool forceRefresh = false}) async {
    if (!forceRefresh && state.isLoading) return;

    state = const ViewState(status: ViewStatus.loading);
    notifyListeners();

    try {
      final results = await Future.wait([
        _repository.fetchAllNews(),
        _repository.fetchHotTopics(),
      ]);
      final allNews = results[0] as Map<String, List<NewsItem>>;
      cailianpressNews = allNews['cailianpress'] ?? [];
      sinaNews = allNews['sina'] ?? [];
      foreignNews = allNews['foreign'] ?? [];
      hotTopics = results[1] as List<HotTopicItem>;
      state = const ViewState(status: ViewStatus.ready);
    } catch (error) {
      state = ViewState(status: ViewStatus.error, message: error.toString());
    }
    notifyListeners();
  }

  /// 刷新数据
  Future<void> refresh() async {
    await load(forceRefresh: true);
  }
}
