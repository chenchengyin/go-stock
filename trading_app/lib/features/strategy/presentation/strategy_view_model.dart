import 'package:flutter/foundation.dart';
import 'package:dio/dio.dart';

import '../../../shared/view_state.dart';
import '../domain/strategy_models.dart';
import '../data/strategy_repository.dart';

class StrategyViewModel extends ChangeNotifier {
  StrategyViewModel({Dio? dio}) : _repo = StrategyRepository(dio: dio);

  final StrategyRepository _repo;

  /// 公开访问仓库（用于详情页等需要直接调用 repo 的场景）
  StrategyRepository get repository => _repo;

  ViewState state = const ViewState();
  List<StrategyPost> posts = [];
  int total = 0;
  int page = 1;
  bool hasMore = true;
  bool _isLoading = false;

  bool get isLoading => _isLoading;

  /// 积分信息
  StrategyUser pointsInfo = const StrategyUser();
  bool checkedInToday = false;
  int todayReplyPoints = 0;

  Future<void> load({bool refresh = false}) async {
    if (_isLoading) return;
    if (refresh) {
      page = 1;
      hasMore = true;
    }
    if (!hasMore && !refresh) return;

    _isLoading = true;
    state = const ViewState(status: ViewStatus.loading);
    notifyListeners();
    try {
      final result = await _repo.getPosts(page: page);
      final list =
          (result['posts'] as List?)
              ?.map((e) => StrategyPost.fromJson(e))
              .toList() ??
          <StrategyPost>[];
      total = (result['total'] ?? 0).toInt();

      if (refresh) {
        posts = list;
      } else {
        posts.addAll(list);
      }
      hasMore = posts.length < total;
      page++;
      state = const ViewState(status: ViewStatus.ready);
    } catch (error, stackTrace) {
      state = ViewState(status: ViewStatus.error, message: error.toString());
      _isLoading = false;
      notifyListeners();
      if (isSessionReplacedError(error)) {
        Error.throwWithStackTrace(error, stackTrace);
      }
      return;
    }
    _isLoading = false;
    notifyListeners();
  }

  void updatePostViewCount(int postId, int viewCount) {
    final idx = posts.indexWhere((p) => p.id == postId);
    if (idx < 0) return;

    final old = posts[idx];
    posts[idx] = StrategyPost(
      id: old.id,
      userId: old.userId,
      nickname: old.nickname,
      title: old.title,
      content: old.content,
      images: old.images,
      likeCount: old.likeCount,
      viewCount: viewCount,
      commentCnt: old.commentCnt,
      createdAt: old.createdAt,
    );
    notifyListeners();
  }

  Future<void> loadPoints() async {
    try {
      pointsInfo = await _repo.getUserPoints();
      checkedInToday = await _repo.hasCheckedIn();
      todayReplyPoints = await _repo.getTodayReplyPoints();
      notifyListeners();
    } catch (error, stackTrace) {
      if (isSessionReplacedError(error)) {
        Error.throwWithStackTrace(error, stackTrace);
      }
    }
  }

  Future<bool> checkIn() async {
    try {
      final result = await _repo.checkIn();
      checkedInToday = result['checkedIn'] == true;
      pointsInfo = StrategyUser(
        points: (result['points'] ?? pointsInfo.points).toInt(),
      );
      notifyListeners();
      return checkedInToday;
    } catch (error, stackTrace) {
      if (isSessionReplacedError(error)) {
        Error.throwWithStackTrace(error, stackTrace);
      }
      return false;
    }
  }

  Future<StrategyPost?> createPost({
    String? title,
    String? content,
    List<String> images = const [],
  }) async {
    try {
      final post = await _repo.createPost(
        title: title,
        content: content,
        images: images,
      );
      posts.insert(0, post);
      notifyListeners();
      return post;
    } catch (error, stackTrace) {
      if (isSessionReplacedError(error)) {
        Error.throwWithStackTrace(error, stackTrace);
      }
      debugPrint('createPost error: $error\n$stackTrace');
      return null;
    }
  }

  /// 查看帖子（含扣分逻辑）
  Future<Map<String, dynamic>> viewPost(int postId) async {
    return _repo.viewPost(postId);
  }

  Future<bool> toggleLike(int postId) async {
    try {
      final result = await _repo.toggleLike(postId);
      // 更新本地帖子数据
      final idx = posts.indexWhere((p) => p.id == postId);
      if (idx >= 0) {
        final old = posts[idx];
        posts[idx] = StrategyPost(
          id: old.id,
          userId: old.userId,
          nickname: old.nickname,
          title: old.title,
          content: old.content,
          images: old.images,
          likeCount: (result['likeCount'] ?? old.likeCount).toInt(),
          viewCount: old.viewCount,
          commentCnt: old.commentCnt,
          createdAt: old.createdAt,
        );
        notifyListeners();
      }
      return result['liked'] == true;
    } catch (error, stackTrace) {
      if (isSessionReplacedError(error)) {
        Error.throwWithStackTrace(error, stackTrace);
      }
      return false;
    }
  }

  Future<Map<String, dynamic>> addComment({
    required int postId,
    int? parentId,
    required String content,
    List<String> images = const [],
    String? replyToUid,
    String? replyToName,
  }) async {
    try {
      final result = await _repo.addComment(
        postId: postId,
        parentId: parentId,
        content: content,
        images: images,
        replyToUid: replyToUid,
        replyToName: replyToName,
      );
      // 更新评论数
      final idx = posts.indexWhere((p) => p.id == postId);
      if (idx >= 0) {
        final old = posts[idx];
        posts[idx] = StrategyPost(
          id: old.id,
          userId: old.userId,
          nickname: old.nickname,
          title: old.title,
          content: old.content,
          images: old.images,
          likeCount: old.likeCount,
          viewCount: old.viewCount,
          commentCnt: old.commentCnt + 1,
          createdAt: old.createdAt,
        );
        notifyListeners();
      }
      if (result['addedPoints'] == true) {
        loadPoints();
      }
      return result;
    } catch (error, stackTrace) {
      if (isSessionReplacedError(error)) {
        Error.throwWithStackTrace(error, stackTrace);
      }
      return {'comment': null, 'addedPoints': false, 'remain': 0};
    }
  }

  Future<void> deletePost(int postId) async {
    try {
      await _repo.deletePost(postId);
      posts.removeWhere((p) => p.id == postId);
      notifyListeners();
    } catch (error, stackTrace) {
      if (isSessionReplacedError(error)) {
        Error.throwWithStackTrace(error, stackTrace);
      }
    }
  }
}

bool isSessionReplacedError(Object error) {
  if (error is! DioException) {
    return false;
  }
  final data = error.response?.data;
  return data is Map && data['code'] == 'SESSION_REPLACED';
}
