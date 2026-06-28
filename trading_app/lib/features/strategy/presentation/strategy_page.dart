import 'package:flutter/cupertino.dart';
import 'package:flutter/material.dart';
import 'package:provider/provider.dart';

import '../../auth/presentation/auth_view_model.dart';
import '../../auth/presentation/login_page.dart';
import '../domain/strategy_models.dart';
import 'strategy_view_model.dart';
import 'create_post_page.dart';
import 'post_detail_page.dart';

class StrategyPage extends StatefulWidget {
  const StrategyPage({super.key});

  @override
  State<StrategyPage> createState() => _StrategyPageState();
}

class _StrategyPageState extends State<StrategyPage> {
  @override
  void initState() {
    super.initState();
    WidgetsBinding.instance.addPostFrameCallback((_) {
      final vm = context.read<StrategyViewModel>();
      final auth = context.read<AuthViewModel>();
      if (auth.user != null) {
        vm.setCurrentUser(auth.user!.id, auth.user!.nickname);
      }
      if (vm.posts.isEmpty) {
        vm.load();
      }
    });
  }

  @override
  Widget build(BuildContext context) {
    final vm = context.watch<StrategyViewModel>();
    final auth = context.watch<AuthViewModel>();
    final isLoggedIn = auth.user != null;
    _syncCurrentUser(auth, vm);

    return CupertinoPageScaffold(
      backgroundColor: CupertinoColors.white,
      navigationBar: CupertinoNavigationBar(
        backgroundColor: CupertinoColors.white,
        middle: const Text(
          '策略吧',
          style: TextStyle(fontWeight: FontWeight.w600),
        ),
        trailing: Row(
          mainAxisSize: MainAxisSize.min,
          children: [
            // 签到按钮
            if (isLoggedIn) ...[
              GestureDetector(
                onTap:
                    vm.currentUserId != null &&
                        vm.currentUserId!.isNotEmpty &&
                        !vm.checkedInToday
                    ? () => vm.checkIn()
                    : null,
                child: Container(
                  padding: const EdgeInsets.symmetric(
                    horizontal: 8,
                    vertical: 4,
                  ),
                  decoration: BoxDecoration(
                    color: vm.checkedInToday
                        ? CupertinoColors.systemGrey5
                        : const Color(0xffe53935),
                    borderRadius: BorderRadius.circular(12),
                  ),
                  child: Text(
                    vm.checkedInToday ? '已签到' : '签到+3',
                    style: TextStyle(
                      fontSize: 11,
                      color: vm.checkedInToday
                          ? CupertinoColors.systemGrey
                          : CupertinoColors.white,
                      fontWeight: FontWeight.w600,
                    ),
                  ),
                ),
              ),
              const SizedBox(width: 8),
              // 积分显示
              Flexible(
                child: Container(
                  padding: const EdgeInsets.symmetric(
                    horizontal: 8,
                    vertical: 4,
                  ),
                  decoration: BoxDecoration(
                    color: CupertinoColors.systemGrey6,
                    borderRadius: BorderRadius.circular(12),
                  ),
                  child: Text(
                    '💎 ${vm.pointsInfo.points}',
                    style: const TextStyle(
                      fontSize: 11,
                      fontWeight: FontWeight.w600,
                    ),
                  ),
                ),
              ),
            ],
          ],
        ),
      ),
      child: Column(
        children: [
          // 积分提示条
          if (isLoggedIn && vm.checkedInToday && vm.todayReplyPoints > 0)
            Container(
              width: double.infinity,
              padding: const EdgeInsets.symmetric(horizontal: 16, vertical: 6),
              color: CupertinoColors.systemGreen.withValues(alpha: 0.08),
              child: Text(
                '今日回复获得 ${vm.todayReplyPoints}/10 积分',
                style: const TextStyle(
                  fontSize: 11,
                  color: CupertinoColors.systemGreen,
                ),
              ),
            ),
          Expanded(child: _buildList(vm)),
        ],
      ),
    );
  }

  Widget _buildList(StrategyViewModel vm) {
    if (vm.state.isLoading && vm.posts.isEmpty) {
      return const Center(child: CupertinoActivityIndicator());
    }

    return Stack(
      children: [
        RefreshIndicator(
          onRefresh: () async => vm.load(refresh: true),
          child: ListView.builder(
            padding: const EdgeInsets.only(top: 4, bottom: 80),
            itemCount: vm.posts.length + 1,
            itemBuilder: (context, index) {
              if (index == vm.posts.length) {
                if (vm.hasMore) {
                  if (!vm.isLoading) {
                    WidgetsBinding.instance.addPostFrameCallback((_) {
                      if (mounted) vm.load();
                    });
                  }
                  return const Padding(
                    padding: EdgeInsets.all(16),
                    child: Center(child: CupertinoActivityIndicator()),
                  );
                }
                return const SizedBox.shrink();
              }

              final post = vm.posts[index];
              return _PostCard(
                post: post,
                isOwner: post.userId == vm.currentUserId,
                onTap: () => _openDetail(context, post.id),
                onDelete: () => _deletePost(context, vm, post.id),
              );
            },
          ),
        ),
        // 发布按钮
        Positioned(
          right: 16,
          bottom: 16,
          child: GestureDetector(
            onTap: () async {
              final auth = context.read<AuthViewModel>();
              if (auth.user == null) {
                // 未登录，跳转登录页
                await Navigator.of(context).push(
                  CupertinoPageRoute(builder: (_) => const LoginPage()),
                );
                // 登录成功后返回，如果已登录再跳转发帖页
                if (context.mounted) {
                  final latestAuth = context.read<AuthViewModel>();
                  if (latestAuth.user == null) return;

                  vm.setCurrentUser(
                    latestAuth.user!.id,
                    latestAuth.user!.nickname,
                  );
                  final result = await Navigator.of(context).push<bool>(
                    CupertinoPageRoute(builder: (_) => const CreatePostPage()),
                  );
                  if (result == true && mounted) {
                    vm.load(refresh: true);
                  }
                }
                return;
              }
              // 已登录，直接跳转发帖页
              final result = await Navigator.of(context).push<bool>(
                CupertinoPageRoute(builder: (_) => const CreatePostPage()),
              );
              if (result == true && mounted) {
                vm.load(refresh: true);
              }
            },
            child: Container(
              width: 52,
              height: 52,
              decoration: const BoxDecoration(
                color: Color(0xff2364aa),
                shape: BoxShape.circle,
                boxShadow: [
                  BoxShadow(
                    color: Color(0x29000000),
                    blurRadius: 8,
                    offset: Offset(0, 2),
                  ),
                ],
              ),
              child: const Icon(
                CupertinoIcons.pencil,
                color: CupertinoColors.white,
                size: 22,
              ),
            ),
          ),
        ),
      ],
    );
  }

  void _syncCurrentUser(AuthViewModel auth, StrategyViewModel vm) {
    final user = auth.user;
    if (user != null && vm.currentUserId != user.id) {
      WidgetsBinding.instance.addPostFrameCallback((_) {
        if (mounted) {
          vm.setCurrentUser(user.id, user.nickname);
        }
      });
    } else if (user == null && vm.currentUserId != null) {
      WidgetsBinding.instance.addPostFrameCallback((_) {
        if (mounted) {
          vm.clearCurrentUser();
        }
      });
    }
  }

  void _openDetail(BuildContext context, int postId) {
    Navigator.of(
      context,
    ).push(CupertinoPageRoute(builder: (_) => PostDetailPage(postId: postId)));
  }

  void _deletePost(BuildContext context, StrategyViewModel vm, int postId) {
    showCupertinoDialog(
      context: context,
      builder: (ctx) => CupertinoAlertDialog(
        title: const Text('确认删除'),
        content: const Text('删除后无法恢复'),
        actions: [
          CupertinoDialogAction(
            child: const Text('取消'),
            onPressed: () => Navigator.of(ctx).pop(),
          ),
          CupertinoDialogAction(
            isDestructiveAction: true,
            child: const Text('删除'),
            onPressed: () {
              Navigator.of(ctx).pop();
              vm.deletePost(postId);
            },
          ),
        ],
      ),
    );
  }
}

class _PostCard extends StatelessWidget {
  const _PostCard({
    required this.post,
    required this.isOwner,
    required this.onTap,
    required this.onDelete,
  });

  final StrategyPost post;
  final bool isOwner;
  final VoidCallback onTap;
  final VoidCallback onDelete;

  @override
  Widget build(BuildContext context) {
    return GestureDetector(
      onTap: onTap,
      child: Container(
        padding: const EdgeInsets.all(16),
        decoration: const BoxDecoration(
          color: CupertinoColors.white,
          border: Border(
            bottom: BorderSide(color: CupertinoColors.systemGrey5, width: 0.5),
          ),
        ),
        child: Column(
          crossAxisAlignment: CrossAxisAlignment.start,
          children: [
            // 用户信息行
            Row(
              children: [
                Container(
                  width: 28,
                  height: 28,
                  decoration: const BoxDecoration(
                    color: CupertinoColors.systemGrey5,
                    shape: BoxShape.circle,
                  ),
                  alignment: Alignment.center,
                  child: Text(
                    post.nickname.isNotEmpty ? post.nickname[0] : '?',
                    style: const TextStyle(
                      fontSize: 12,
                      fontWeight: FontWeight.w600,
                    ),
                  ),
                ),
                const SizedBox(width: 8),
                Text(
                  post.nickname,
                  style: const TextStyle(
                    fontSize: 14,
                    fontWeight: FontWeight.w500,
                  ),
                ),
                const Spacer(),
                Text(
                  _formatTime(post.createdAt),
                  style: const TextStyle(
                    fontSize: 11,
                    color: CupertinoColors.systemGrey,
                  ),
                ),
                if (isOwner) ...[
                  const SizedBox(width: 8),
                  GestureDetector(
                    onTap: onDelete,
                    child: const Icon(
                      CupertinoIcons.trash,
                      size: 14,
                      color: CupertinoColors.systemGrey,
                    ),
                  ),
                ],
              ],
            ),
            // 内容
            if (post.content != null && post.content!.isNotEmpty) ...[
              const SizedBox(height: 8),
              Text(
                post.content!,
                style: const TextStyle(fontSize: 15, height: 1.4),
                maxLines: 4,
                overflow: TextOverflow.ellipsis,
              ),
            ],
            // 标题（如果有）
            if (post.title != null && post.title!.isNotEmpty) ...[
              const SizedBox(height: 4),
              Text(
                post.title!,
                style: const TextStyle(
                  fontSize: 13,
                  color: CupertinoColors.systemGrey,
                ),
              ),
            ],
            // 图片（仅展示第一张）
            if (post.images.isNotEmpty) ...[
              const SizedBox(height: 8),
              ClipRRect(
                borderRadius: BorderRadius.circular(8),
                child: Image.network(
                  post.images.first,
                  height: 160,
                  width: double.infinity,
                  fit: BoxFit.cover,
                  errorBuilder: (_, __, ___) => const SizedBox.shrink(),
                ),
              ),
            ],
            // 底部操作栏
            const SizedBox(height: 12),
            Row(
              children: [
                _ActionIcon(icon: CupertinoIcons.heart, count: post.likeCount),
                const SizedBox(width: 24),
                _ActionIcon(
                  icon: CupertinoIcons.bubble_left_bubble_right,
                  count: post.commentCnt,
                ),
                const SizedBox(width: 24),
                _ActionIcon(icon: CupertinoIcons.eye, count: post.viewCount),
              ],
            ),
          ],
        ),
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

class _ActionIcon extends StatelessWidget {
  const _ActionIcon({required this.icon, required this.count});
  final IconData icon;
  final int count;

  @override
  Widget build(BuildContext context) {
    return Row(
      mainAxisSize: MainAxisSize.min,
      children: [
        Icon(icon, size: 16, color: CupertinoColors.systemGrey),
        const SizedBox(width: 4),
        Text(
          count > 999 ? '999+' : count.toString(),
          style: const TextStyle(
            fontSize: 12,
            color: CupertinoColors.systemGrey,
          ),
        ),
      ],
    );
  }
}
