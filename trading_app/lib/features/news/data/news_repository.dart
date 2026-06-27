import '../../../core/storage/local_cache.dart';
import '../domain/news_models.dart';
import 'news_remote_datasource.dart';

abstract class NewsRepository {
  /// 获取指定来源的新闻列表
  Future<NewsListResult> fetchNews(String source, {int limit = 50});

  /// 获取所有来源的新闻（三栏展示）
  Future<Map<String, List<NewsItem>>> fetchAllNews();

  /// 获取热门话题
  Future<List<HotTopicItem>> fetchHotTopics({int limit = 10});
}

class NewsRepositoryImpl implements NewsRepository {
  NewsRepositoryImpl({
    required LocalCache cache,
    required INewsRemoteDataSource remoteDataSource,
  })  : _cache = cache,
        _remote = remoteDataSource;

  final LocalCache _cache;
  final INewsRemoteDataSource _remote;

  @override
  Future<NewsListResult> fetchNews(String source, {int limit = 50}) async {
    await _cache.setString('news:last_source', source);
    try {
      final result = await _remote.fetchNews(source, limit: limit);
      return result;
    } catch (e) {
      // 请求失败，返回空列表
      return NewsListResult(source: source, items: []);
    }
  }

  @override
  Future<Map<String, List<NewsItem>>> fetchAllNews() async {
    // 并行获取三个数据源
    final results = await Future.wait([
      fetchNews('财联社电报', limit: 100),
      fetchNews('新浪财经', limit: 100),
      fetchNews('外媒', limit: 100),
    ]);

    return {
      'cailianpress': results[0].items,
      'sina': results[1].items,
      'foreign': results[2].items,
    };
  }

  @override
  Future<List<HotTopicItem>> fetchHotTopics({int limit = 10}) async {
    try {
      final result = await _remote.fetchHotTopics(limit: limit);
      return result;
    } catch (e) {
      return [];
    }
  }
}
