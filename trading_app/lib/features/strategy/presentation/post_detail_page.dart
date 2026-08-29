import 'package:flutter/cupertino.dart';
import 'package:provider/provider.dart';

import '../../../core/theme/app_colors.dart';
import '../domain/strategy_models.dart';
import 'strategy_view_model.dart';

class PostDetailPage extends StatefulWidget {
  const PostDetailPage({super.key, required this.postId});

  final int postId;

  @override
  State<PostDetailPage> createState() => _PostDetailPageState();
}

class _PostDetailPageState extends State<PostDetailPage> {
  StrategyPost? _post;
  List<StrategyComment> _comments = [];
  bool _loading = true;
  bool _liked = false;
  bool _deducted = false;
  bool _showDeductedTip = false;

  final _replyController = TextEditingController();
  int? _replyToCommentId; // 当前正在回复的评论id
  String? _replyToUid;
  String? _replyToName;

  @override
  void initState() {
    super.initState();
    _loadData();
  }

  @override
  void dispose() {
    _replyController.dispose();
    super.dispose();
  }

  Future<void> _loadData() async {
    final vm = context.read<StrategyViewModel>();
    try {
      // 查看帖子（触发扣分）
      final viewResult = await vm.viewPost(widget.postId);
      _post = viewResult['post'] != null
          ? StrategyPost.fromJson(Map<String, dynamic>.from(viewResult['post']))
          : null;

      if (_post != null) {
        _deducted = viewResult['deducted'] == true;
        if (_deducted) {
          _showDeductedTip = true;
          Future.delayed(const Duration(seconds: 3), () {
            if (mounted) setState(() => _showDeductedTip = false);
          });
        }
      }

      // 加载评论
      _comments = await vm.repository.getComments(widget.postId);

      // 加载点赞状态
      if (vm.currentUserId != null && vm.currentUserId!.isNotEmpty) {
        final status = await vm.repository.getLikeStatus(
          widget.postId,
          vm.currentUserId!,
        );
        _liked = status['liked'] ?? false;
      }

      if (_post != null) {
        vm.updatePostViewCount(widget.postId, _post!.viewCount);
      }

      setState(() => _loading = false);
    } catch (e) {
      setState(() => _loading = false);
    }
  }

  Future<void> _toggleLike() async {
    final vm = context.read<StrategyViewModel>();
    final liked = await vm.toggleLike(widget.postId);
    setState(() => _liked = liked);
    // 更新本地帖子数据
    if (_post != null) {
      final delta = liked ? 1 : -1;
      _post = StrategyPost(
        id: _post!.id,
        userId: _post!.userId,
        nickname: _post!.nickname,
        title: _post!.title,
        content: _post!.content,
        images: _post!.images,
        likeCount: _post!.likeCount + delta,
        viewCount: _post!.viewCount,
        commentCnt: _post!.commentCnt,
        createdAt: _post!.createdAt,
      );
    }
  }

  void _startReply(int? commentId, String? replyUid, String? replyName) {
    setState(() {
      _replyToCommentId = commentId;
      _replyToUid = replyUid;
      _replyToName = replyName;
    });
  }

  void _cancelReply() {
    setState(() {
      _replyToCommentId = null;
      _replyToUid = null;
      _replyToName = null;
    });
  }

  Future<void> _submitComment() async {
    final text = _replyController.text.trim();
    if (text.isEmpty) return;

    final vm = context.read<StrategyViewModel>();
    final result = await vm.addComment(
      postId: widget.postId,
      parentId: _replyToCommentId,
      content: text,
      replyToUid: _replyToUid,
      replyToName: _replyToName,
    );

    _replyController.clear();
    _cancelReply();

    if (result['comment'] != null) {
      final comment = StrategyComment.fromJson(
        Map<String, dynamic>.from(result['comment']),
      );
      setState(() => _comments.add(comment));
      if (_post != null) {
        _post = StrategyPost(
          id: _post!.id,
          userId: _post!.userId,
          nickname: _post!.nickname,
          title: _post!.title,
          content: _post!.content,
          images: _post!.images,
          likeCount: _post!.likeCount,
          viewCount: _post!.viewCount,
          commentCnt: _post!.commentCnt + 1,
          createdAt: _post!.createdAt,
        );
      }
      // 显示积分提示
      if (result['addedPoints'] == true) {
        _showToast('回复获得 +1 积分');
      }
    }
  }

  void _showToast(String msg) {
    if (!mounted) return;
    showCupertinoDialog(
      context: context,
      builder: (ctx) => CupertinoAlertDialog(
        title: Text(msg),
        actions: [
          CupertinoDialogAction(
            child: const Text('确定'),
            onPressed: () => Navigator.of(ctx).pop(),
          ),
        ],
      ),
    );
  }

  @override
  Widget build(BuildContext context) {
    AppColors.of(context);
    return CupertinoPageScaffold(
      backgroundColor: AppColors.scaffoldBg,
      navigationBar: CupertinoNavigationBar(
        backgroundColor: AppColors.appBarBg,
        middle: const Text(
          '帖子详情',
          style: TextStyle(fontWeight: FontWeight.w600),
        ),
      ),
      child: _loading
          ? const Center(child: CupertinoActivityIndicator())
          : _post == null
          ? const Center(child: Text('帖子不存在'))
          : Column(
              children: [
                // 扣分提示
                if (_showDeductedTip)
                  Container(
                    width: double.infinity,
                    padding: const EdgeInsets.symmetric(
                      horizontal: 16,
                      vertical: 8,
                    ),
                    color: CupertinoColors.systemYellow.withValues(alpha: 0.15),
                    child: Consumer<StrategyViewModel>(
                      builder: (_, vm, __) => Text(
                        '首次阅读 -1 积分，剩余 ${vm.pointsInfo.points} 积分',
                        style: const TextStyle(
                          fontSize: 12,
                          color: Color(0xffb71c1c),
                        ),
                      ),
                    ),
                  ),
                // 帖子内容
                Expanded(
                  child: ListView(
                    padding: EdgeInsets.zero,
                    children: [
                      _buildPostContent(),
                      if (_comments.isNotEmpty) ...[
                        Padding(
                          padding: EdgeInsets.fromLTRB(16, 16, 16, 8),
                          child: Text(
                            '评论',
                            style: TextStyle(
                              fontSize: 13,
                              color: AppColors.textTertiary,
                              fontWeight: FontWeight.w600,
                            ),
                          ),
                        ),
                        ..._comments.map((c) => _buildComment(c)),
                      ],
                    ],
                  ),
                ),
                // 底部输入栏
                _buildBottomBar(),
              ],
            ),
    );
  }

  Widget _buildPostContent() {
    return Container(
      padding: const EdgeInsets.all(16),
      child: Column(
        crossAxisAlignment: CrossAxisAlignment.start,
        children: [
          // 用户信息
          Row(
            children: [
              Container(
                width: 32,
                height: 32,
                decoration: BoxDecoration(
                  color: AppColors.surfaceBg,
                  shape: BoxShape.circle,
                ),
                alignment: Alignment.center,
                child: Text(
                  _post!.nickname.isNotEmpty ? _post!.nickname[0] : '?',
                  style: const TextStyle(
                    fontSize: 14,
                    fontWeight: FontWeight.w600,
                  ),
                ),
              ),
              const SizedBox(width: 8),
              Text(
                _post!.nickname,
                style: TextStyle(fontSize: 15, fontWeight: FontWeight.w600),
              ),
              const Spacer(),
              Text(
                _formatTime(_post!.createdAt),
                style: TextStyle(fontSize: 11, color: AppColors.textSecondary),
              ),
            ],
          ),
          // 内容
          if (_post!.content != null && _post!.content!.isNotEmpty) ...[
            const SizedBox(height: 12),
            Text(
              _post!.content!,
              style: const TextStyle(fontSize: 16, height: 1.5),
            ),
          ],
          // 图片
          if (_post!.images.isNotEmpty) ...[
            const SizedBox(height: 12),
            ..._post!.images.map(
              (url) => Padding(
                padding: const EdgeInsets.only(bottom: 8),
                child: ClipRRect(
                  borderRadius: BorderRadius.circular(8),
                  child: Image.network(
                    url,
                    width: double.infinity,
                    fit: BoxFit.contain,
                    errorBuilder: (_, __, ___) => const SizedBox.shrink(),
                  ),
                ),
              ),
            ),
          ],
          // 操作
          const SizedBox(height: 16),
          Row(
            children: [
              GestureDetector(
                onTap: _toggleLike,
                child: Row(
                  children: [
                    Icon(
                      _liked ? CupertinoIcons.heart_fill : CupertinoIcons.heart,
                      size: 20,
                      color: _liked
                          ? const Color(0xffe53935)
                          : AppColors.textTertiary,
                    ),
                    const SizedBox(width: 4),
                    Text(
                      _post!.likeCount.toString(),
                      style: TextStyle(
                        fontSize: 13,
                        color: _liked
                            ? const Color(0xffe53935)
                            : AppColors.textTertiary,
                      ),
                    ),
                  ],
                ),
              ),
              const SizedBox(width: 24),
              Row(
                children: [
                  Icon(
                    CupertinoIcons.bubble_left_bubble_right,
                    size: 20,
                    color: AppColors.textTertiary,
                  ),
                  const SizedBox(width: 4),
                  Text(
                    _post!.commentCnt.toString(),
                    style: TextStyle(
                      fontSize: 13,
                      color: AppColors.textTertiary,
                    ),
                  ),
                ],
              ),
            ],
          ),
        ],
      ),
    );
  }

  Widget _buildComment(StrategyComment comment) {
    return Container(
      padding: const EdgeInsets.symmetric(horizontal: 16, vertical: 10),
      decoration: BoxDecoration(
        border: Border(
          bottom: BorderSide(color: AppColors.divider, width: 0.5),
        ),
      ),
      child: Column(
        crossAxisAlignment: CrossAxisAlignment.start,
        children: [
          Row(
            children: [
              Text(
                comment.nickname,
                style: const TextStyle(
                  fontSize: 13,
                  fontWeight: FontWeight.w600,
                ),
              ),
              if (comment.replyToName != null) ...[
                Text(
                  ' 回复 ',
                  style: TextStyle(fontSize: 12, color: AppColors.textTertiary),
                ),
                Text(
                  comment.replyToName!,
                  style: const TextStyle(
                    fontSize: 13,
                    fontWeight: FontWeight.w600,
                  ),
                ),
              ],
              const SizedBox(width: 8),
              Text(
                _formatTime(comment.createdAt),
                style: TextStyle(fontSize: 10, color: AppColors.textTertiary),
              ),
              const Spacer(),
              GestureDetector(
                onTap: () =>
                    _startReply(comment.id, comment.userId, comment.nickname),
                child: Icon(
                  CupertinoIcons.reply,
                  size: 14,
                  color: AppColors.textTertiary,
                ),
              ),
            ],
          ),
          if (comment.content != null && comment.content!.isNotEmpty) ...[
            const SizedBox(height: 4),
            Text(
              comment.content!,
              style: const TextStyle(fontSize: 14, height: 1.3),
            ),
          ],
          if (comment.images.isNotEmpty) ...[
            const SizedBox(height: 4),
            Wrap(
              spacing: 4,
              children: comment.images
                  .map(
                    (url) => ClipRRect(
                      borderRadius: BorderRadius.circular(4),
                      child: Image.network(
                        url,
                        width: 60,
                        height: 60,
                        fit: BoxFit.cover,
                        errorBuilder: (_, __, ___) => const SizedBox.shrink(),
                      ),
                    ),
                  )
                  .toList(),
            ),
          ],
        ],
      ),
    );
  }

  Widget _buildBottomBar() {
    return Container(
      padding: EdgeInsets.only(
        left: 12,
        right: 12,
        top: 8,
        bottom: MediaQuery.of(context).padding.bottom + 8,
      ),
      decoration: BoxDecoration(
        color: AppColors.cardBg,
        border: Border(top: BorderSide(color: AppColors.divider, width: 0.5)),
      ),
      child: Row(
        children: [
          if (_replyToCommentId != null)
            GestureDetector(
              onTap: _cancelReply,
              child: Padding(
                padding: EdgeInsets.only(right: 8),
                child: Icon(
                  CupertinoIcons.xmark_circle,
                  size: 18,
                  color: AppColors.textTertiary,
                ),
              ),
            ),
          Expanded(
            child: CupertinoTextField(
              controller: _replyController,
              placeholder: _replyToName != null
                  ? '回复 $_replyToName...'
                  : '写评论...',
              padding: const EdgeInsets.symmetric(horizontal: 12, vertical: 8),
              decoration: BoxDecoration(
                color: AppColors.inputFill,
                borderRadius: BorderRadius.circular(20),
              ),
              onSubmitted: (_) => _submitComment(),
            ),
          ),
          const SizedBox(width: 8),
          GestureDetector(
            onTap: _submitComment,
            child: Container(
              padding: const EdgeInsets.all(8),
              decoration: BoxDecoration(
                color: AppColors.brand,
                shape: BoxShape.circle,
              ),
              child: Icon(
                CupertinoIcons.arrow_up,
                size: 16,
                color: CupertinoColors.white,
              ),
            ),
          ),
        ],
      ),
    );
  }

  String _formatTime(String iso) {
    try {
      final dt = DateTime.parse(iso);
      final now = DateTime.now();
      final diff = now.difference(dt);
      if (diff.inMinutes < 60) return '${diff.inMinutes}分钟前';
      if (diff.inHours < 24) return '${diff.inHours}小时前';
      return '${dt.month}/${dt.day}';
    } catch (_) {
      return iso;
    }
  }
}
