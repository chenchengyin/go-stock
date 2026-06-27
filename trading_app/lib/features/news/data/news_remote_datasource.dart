/// 新闻远程数据源 — 调用 Go 后端的 REST API
library;

import 'package:dio/dio.dart';

import '../../../core/network/api_client.dart';
import '../domain/news_models.dart';

abstract class INewsRemoteDataSource {
  /// 获取指定来源的新闻列表
  Future<NewsListResult> fetchNews(String source, {int limit = 50});

  /// 获取热门话题
  Future<List<HotTopicItem>> fetchHotTopics({int limit = 10});
}

class NewsRemoteDataSource implements INewsRemoteDataSource {
  NewsRemoteDataSource({Dio? dio})
      : _dio = dio ?? createApiClient();

  final Dio _dio;

  @override
  Future<NewsListResult> fetchNews(String source, {int limit = 50}) async {
    final response = await _dio.get(
      '/api/news',
      queryParameters: {
        'source': source,
        'limit': limit.toString(),
      },
    );

    if (response.statusCode == 200) {
      final data = response.data;
      if (data is List) {
        return NewsListResult.fromJson(source, data);
      }
      return NewsListResult(source: source, items: []);
    }

    throw Exception('获取新闻失败: ${response.statusCode}');
  }

  @override
  Future<List<HotTopicItem>> fetchHotTopics({int limit = 10}) async {
    final response = await _dio.get(
      '/api/hot-topics',
      queryParameters: {'limit': limit.toString()},
    );

    if (response.statusCode == 200 && response.data is List) {
      return (response.data as List)
          .whereType<Map<String, dynamic>>()
          .map((e) => HotTopicItem.fromJson(e))
          .toList();
    }

    throw Exception('获取热门话题失败: ${response.statusCode}');
  }
}
