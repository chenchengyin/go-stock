import 'package:dio/dio.dart';

import '../../../core/network/api_client.dart';
import '../domain/strategy_models.dart';

class StrategyRepository {
  StrategyRepository({Dio? dio}) : _dio = dio ?? createApiClient();

  final Dio _dio;

  /// 获取帖子列表
  Future<Map<String, dynamic>> getPosts({
    int page = 1,
    int pageSize = 20,
  }) async {
    final resp = await _dio.get(
      '/api/strategy',
      queryParameters: {
        'action': 'list',
        'page': page.toString(),
        'pageSize': pageSize.toString(),
      },
    );
    return resp.data;
  }

  /// 获取帖子详情
  Future<StrategyPost> getPostDetail(int postId) async {
    final resp = await _dio.get(
      '/api/strategy',
      queryParameters: {'action': 'detail', 'postId': postId.toString()},
    );
    return StrategyPost.fromJson(resp.data);
  }

  /// 查看帖子（扣分逻辑）
  Future<Map<String, dynamic>> viewPost(int postId) async {
    final resp = await _dio.post(
      '/api/strategy',
      data: {'action': 'view_post', 'postId': postId},
    );
    return resp.data;
  }

  /// 获取用户积分
  Future<StrategyUser> getUserPoints() async {
    final resp = await _dio.get(
      '/api/strategy',
      queryParameters: {'action': 'points'},
    );
    return StrategyUser.fromJson(resp.data);
  }

  /// 签到
  Future<Map<String, dynamic>> checkIn() async {
    final resp = await _dio.post('/api/strategy', data: {'action': 'checkin'});
    return resp.data;
  }

  /// 签到状态
  Future<bool> hasCheckedIn() async {
    final resp = await _dio.get(
      '/api/strategy',
      queryParameters: {'action': 'checkin_status'},
    );
    return resp.data['checkedIn'] == true;
  }

  /// 创建帖子
  Future<StrategyPost> createPost({
    String? title,
    String? content,
    List<String> images = const [],
  }) async {
    final resp = await _dio.post(
      '/api/strategy',
      data: {
        'action': 'create_post',
        'title': title ?? '',
        'content': content ?? '',
        'images': images,
      },
    );
    return StrategyPost.fromJson(resp.data);
  }

  /// 获取评论
  Future<List<StrategyComment>> getComments(int postId) async {
    final resp = await _dio.get(
      '/api/strategy',
      queryParameters: {'action': 'comments', 'postId': postId.toString()},
    );
    return (resp.data as List).map((e) => StrategyComment.fromJson(e)).toList();
  }

  /// 添加评论
  Future<Map<String, dynamic>> addComment({
    required int postId,
    int? parentId,
    required String content,
    List<String> images = const [],
    String? replyToUid,
    String? replyToName,
  }) async {
    final resp = await _dio.post(
      '/api/strategy',
      data: {
        'action': 'add_comment',
        'postId': postId,
        if (parentId != null) 'parentId': parentId,
        'content': content,
        'images': images,
        if (replyToUid != null) 'replyToUid': replyToUid,
        if (replyToName != null) 'replyToName': replyToName,
      },
    );
    return resp.data;
  }

  /// 点赞/取消点赞
  Future<Map<String, dynamic>> toggleLike(int postId) async {
    final resp = await _dio.post(
      '/api/strategy',
      data: {'action': 'toggle_like', 'postId': postId},
    );
    return resp.data;
  }

  /// 点赞状态
  Future<Map<String, bool>> getLikeStatus(int postId) async {
    final resp = await _dio.get(
      '/api/strategy',
      queryParameters: {'action': 'liked', 'postId': postId.toString()},
    );
    return {
      'liked': resp.data['liked'] == true,
      'viewed': resp.data['viewed'] == true,
    };
  }

  /// 获取今日回复已获得积分
  Future<int> getTodayReplyPoints() async {
    final resp = await _dio.get(
      '/api/strategy',
      queryParameters: {'action': 'today_reply_points'},
    );
    return (resp.data['todayReplyPoints'] ?? 0).toInt();
  }

  /// 删除评论
  Future<void> deleteComment(int commentId) async {
    await _dio.post(
      '/api/strategy',
      data: {'action': 'delete_comment', 'commentId': commentId},
    );
  }

  /// 删除帖子
  Future<void> deletePost(int postId) async {
    await _dio.post(
      '/api/strategy',
      data: {'action': 'delete_post', 'postId': postId},
    );
  }
}
