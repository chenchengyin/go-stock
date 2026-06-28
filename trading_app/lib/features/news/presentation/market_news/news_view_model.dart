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

  /// 国内新闻列表（财联社+新浪，按时间倒序）
  List<NewsItem> domesticNews = [];

  /// 是否有外媒数据（影响三栏/两栏布局）
  bool get hasForeignNews => foreignNews.isNotEmpty;

  /// 是否有热门话题
  bool get hasHotTopics => hotTopics.isNotEmpty;

  /// 所有新闻总数
  int get totalCount =>
      cailianpressNews.length + sinaNews.length + foreignNews.length;

  /// 重要新闻汇总（从所有来源中筛选 isRed=true）
  List<NewsItem> get importantNews => _dedupeNews(
    [...cailianpressNews, ...sinaNews, ...foreignNews].where((n) => n.isRed),
  );

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
      cailianpressNews = _dedupeNews(allNews['cailianpress'] ?? []);
      sinaNews = _dedupeNews(allNews['sina'] ?? []);
      foreignNews = _dedupeNews(allNews['foreign'] ?? []);
      if (domesticNews.isNotEmpty || forceRefresh) {
        domesticNews = _dedupeNews(
          await _repository.fetchDomesticNews(limit: 100),
        );
      }
      hotTopics = results[1] as List<HotTopicItem>;
      state = const ViewState(status: ViewStatus.ready);
    } catch (error) {
      state = ViewState(status: ViewStatus.error, message: error.toString());
    }
    notifyListeners();
  }

  /// 加载国内新闻
  Future<void> loadDomesticNews() async {
    domesticNews = _dedupeNews(await _repository.fetchDomesticNews(limit: 100));
    notifyListeners();
  }

  /// 刷新数据
  Future<void> refresh() async {
    await load(forceRefresh: true);
  }

  static List<NewsItem> _dedupeNews(Iterable<NewsItem> items) {
    final seen = <String>{};
    final result = <NewsItem>[];

    for (final item in items) {
      final key = _dedupeKey(item);
      if (key.isEmpty || seen.add(key)) {
        result.add(item);
      }
    }

    return result;
  }

  static String _dedupeKey(NewsItem item) {
    final text = item.title.trim().isNotEmpty ? item.title : item.content;
    return text
        .trim()
        .replaceAll(RegExp(r'\s+'), '')
        .replaceAll(RegExp(r'[，。！？、,.!?:：；;“”"‘’（）()\[\]【】]'), '');
  }
}
