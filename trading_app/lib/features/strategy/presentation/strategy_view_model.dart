import 'package:flutter/foundation.dart';
import 'package:dio/dio.dart';

import '../../../shared/view_state.dart';
import '../domain/strategy_models.dart';
import '../data/strategy_repository.dart';

class StrategyViewModel extends ChangeNotifier {
  StrategyViewModel({
    Dio? dio,
    this.currentUserId,
    this.currentNickname,
  }) : _repo = StrategyRepository(dio: dio);

  final StrategyRepository _repo;

  /// 公开访问仓库（用于详情页等需要直接调用 repo 的场景）
  StrategyRepository get repository => _repo;

  ViewState state = const ViewState();
  List<StrategyPost> posts = [];
  int total = 0;
  int page = 1;
  bool hasMore = true;

  /// 当前用户
  String? currentUserId;
  String? currentNickname;

  /// 积分信息
  StrategyUser pointsInfo = const StrategyUser();
  bool checkedInToday = false;
  int todayReplyPoints = 0;

  void setCurrentUser(String userId, String nickname) {
    currentUserId = userId;
    currentNickname = nickname;
    if (userId.isNotEmpty) {
      loadPoints();
    }
  }

  Future<void> load({bool refresh = false}) async {
    if (refresh) {
      page = 1;
      hasMore = true;
    }
    if (!hasMore && !refresh) return;

    state = const ViewState(status: ViewStatus.loading);
    notifyListeners();
    try {
      final result = await _repo.getPosts(page: page);
      final list = (result['posts'] as List?)?.map((e) => StrategyPost.fromJson(e)).toList() ?? <StrategyPost>[];
      total = (result['total'] ?? 0).toInt();

      if (refresh) {
        posts = list;
      } else {
        posts.addAll(list);
      }
      hasMore = posts.length < total;
      page++;
      state = const ViewState(status: ViewStatus.ready);
    } catch (error) {
      state = ViewState(status: ViewStatus.error, message: error.toString());
    }
    notifyListeners();
  }

  Future<void> loadPoints() async {
    if (currentUserId == null || currentUserId!.isEmpty) return;
    try {
      pointsInfo = await _repo.getUserPoints(currentUserId!);
      checkedInToday = await _repo.hasCheckedIn(currentUserId!);
      todayReplyPoints = await _repo.getTodayReplyPoints(currentUserId!);
      notifyListeners();
    } catch (_) {}
  }

  Future<bool> checkIn() async {
    if (currentUserId == null || currentUserId!.isEmpty) return false;
    try {
      final result = await _repo.checkIn(currentUserId!, currentNickname ?? '');
      checkedInToday = result['checkedIn'] == true;
      pointsInfo = StrategyUser(
        userId: currentUserId ?? '',
        points: (result['points'] ?? pointsInfo.points).toInt(),
      );
      notifyListeners();
      return checkedInToday;
    } catch (_) {
      return false;
    }
  }

  Future<StrategyPost?> createPost({
    String? title,
    String? content,
    List<String> images = const [],
  }) async {
    if (currentUserId == null || currentUserId!.isEmpty) return null;
    try {
      final post = await _repo.createPost(
        userId: currentUserId!,
        nickname: currentNickname ?? '用户',
        title: title,
        content: content,
        images: images,
      );
      posts.insert(0, post);
      notifyListeners();
      return post;
    } catch (_) {
      return null;
    }
  }

  /// 查看帖子（含扣分逻辑）
  Future<Map<String, dynamic>> viewPost(int postId) async {
    if (currentUserId == null || currentUserId!.isEmpty) {
      return {'post': null, 'deducted': false, 'remain': 0};
    }
    return await _repo.viewPost(postId, currentUserId!, currentNickname ?? '用户');
  }

  Future<bool> toggleLike(int postId) async {
    if (currentUserId == null || currentUserId!.isEmpty) return false;
    try {
      final result = await _repo.toggleLike(postId, currentUserId!);
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
    } catch (_) {
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
    if (currentUserId == null || currentUserId!.isEmpty) {
      return {'comment': null, 'addedPoints': false, 'remain': 0};
    }
    try {
      final result = await _repo.addComment(
        postId: postId,
        parentId: parentId,
        userId: currentUserId!,
        nickname: currentNickname ?? '用户',
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
    } catch (_) {
      return {'comment': null, 'addedPoints': false, 'remain': 0};
    }
  }

  Future<void> deletePost(int postId) async {
    if (currentUserId == null || currentUserId!.isEmpty) return;
    try {
      await _repo.deletePost(postId, currentUserId!);
      posts.removeWhere((p) => p.id == postId);
      notifyListeners();
    } catch (_) {}
  }
}
