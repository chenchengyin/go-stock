/// 策略吧数据模型
class StrategyPost {
  const StrategyPost({
    required this.id,
    required this.userId,
    required this.nickname,
    this.title,
    this.content,
    this.images = const [],
    this.likeCount = 0,
    this.viewCount = 0,
    this.commentCnt = 0,
    required this.createdAt,
  });

  final int id;
  final String userId;
  final String nickname;
  final String? title;
  final String? content;
  final List<String> images;
  final int likeCount;
  final int viewCount;
  final int commentCnt;
  final String createdAt;

  factory StrategyPost.fromJson(Map<String, dynamic> json) {
    return StrategyPost(
      id: json['id'] ?? 0,
      userId: json['userId'] ?? '',
      nickname: json['nickname'] ?? '',
      title: json['title'],
      content: json['content'],
      images: (json['images'] as List?)?.map((e) => e.toString()).toList() ?? [],
      likeCount: (json['likeCount'] ?? 0).toInt(),
      viewCount: (json['viewCount'] ?? 0).toInt(),
      commentCnt: (json['commentCnt'] ?? 0).toInt(),
      createdAt: json['createdAt'] ?? '',
    );
  }
}

class StrategyComment {
  const StrategyComment({
    required this.id,
    required this.postId,
    this.parentId,
    this.replyToUid,
    this.replyToName,
    required this.userId,
    required this.nickname,
    this.content,
    this.images = const [],
    required this.createdAt,
  });

  final int id;
  final int postId;
  final int? parentId;
  final String? replyToUid;
  final String? replyToName;
  final String userId;
  final String nickname;
  final String? content;
  final List<String> images;
  final String createdAt;

  factory StrategyComment.fromJson(Map<String, dynamic> json) {
    return StrategyComment(
      id: json['id'] ?? 0,
      postId: json['postId'] ?? 0,
      parentId: json['parentId']?.toInt(),
      replyToUid: json['replyToUid'],
      replyToName: json['replyToName'],
      userId: json['userId'] ?? '',
      nickname: json['nickname'] ?? '',
      content: json['content'],
      images: (json['images'] as List?)?.map((e) => e.toString()).toList() ?? [],
      createdAt: json['createdAt'] ?? '',
    );
  }
}

class StrategyUser {
  const StrategyUser({
    this.id = 0,
    this.userId = '',
    this.nickname = '',
    this.points = 0,
    this.totalIn = 0,
    this.totalOut = 0,
  });

  final int id;
  final String userId;
  final String nickname;
  final int points;
  final int totalIn;
  final int totalOut;

  factory StrategyUser.fromJson(Map<String, dynamic> json) {
    return StrategyUser(
      id: json['id'] ?? 0,
      userId: json['userId'] ?? '',
      nickname: json['nickname'] ?? '',
      points: (json['points'] ?? 0).toInt(),
      totalIn: (json['totalIn'] ?? 0).toInt(),
      totalOut: (json['totalOut'] ?? 0).toInt(),
    );
  }
}
