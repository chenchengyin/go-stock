/// 热门话题卡片 — iOS 原生资讯列表风格
library;

import 'package:flutter/cupertino.dart';

import '../../domain/news_models.dart';

class HotTopicCard extends StatelessWidget {
  const HotTopicCard({super.key, required this.item, this.onTap});

  final HotTopicItem item;
  final VoidCallback? onTap;

  @override
  Widget build(BuildContext context) {
    return GestureDetector(
      onTap: onTap,
      child: Container(
        padding: const EdgeInsets.symmetric(horizontal: 16, vertical: 12),
        decoration: const BoxDecoration(
          color: CupertinoColors.white,
          border: Border(
            bottom: BorderSide(color: CupertinoColors.systemGrey5, width: 0.5),
          ),
        ),
        child: Row(
          crossAxisAlignment: CrossAxisAlignment.start,
          children: [
            // 左侧图片
            ClipRRect(
              borderRadius: BorderRadius.circular(6),
              child: _buildImage(),
            ),
            const SizedBox(width: 12),
            // 右侧内容
            Expanded(
              child: Column(
                crossAxisAlignment: CrossAxisAlignment.start,
                children: [
                  Text(
                    item.nickname,
                    style: const TextStyle(
                      fontSize: 15,
                      fontWeight: FontWeight.w500,
                      height: 1.35,
                      color: CupertinoColors.black,
                    ),
                    maxLines: 2,
                    overflow: TextOverflow.ellipsis,
                  ),
                  const SizedBox(height: 4),
                  Text(
                    item.desc,
                    style: const TextStyle(
                      fontSize: 13,
                      color: CupertinoColors.systemGrey,
                      height: 1.4,
                    ),
                    maxLines: 2,
                    overflow: TextOverflow.ellipsis,
                  ),
                  const SizedBox(height: 8),
                  Row(
                    children: [
                      _Badge(
                        icon: CupertinoIcons.chat_bubble_2,
                        text: _formatNum(item.postNumber),
                      ),
                      const SizedBox(width: 12),
                      _Badge(
                        icon: CupertinoIcons.eye,
                        text: _formatNum(item.clickNumber),
                      ),
                      const Spacer(),
                      if (item.stockList.isNotEmpty)
                        Flexible(
                          child: Text(
                            item.stockList.take(3).join(' '),
                            style: const TextStyle(
                              fontSize: 11,
                              color: CupertinoColors.systemGrey,
                            ),
                            maxLines: 1,
                            overflow: TextOverflow.ellipsis,
                          ),
                        ),
                    ],
                  ),
                ],
              ),
            ),
          ],
        ),
      ),
    );
  }

  Widget _buildImage() {
    if (item.squareImg != null && item.squareImg!.isNotEmpty) {
      return Image.network(
        item.squareImg!,
        width: 68,
        height: 68,
        fit: BoxFit.cover,
        loadingBuilder: (_, child, progress) =>
            progress == null ? child : _placeholder(),
        errorBuilder: (_, __, ___) => _placeholder(),
      );
    }
    return _placeholder();
  }

  Widget _placeholder() {
    return Container(
      width: 68,
      height: 68,
      decoration: BoxDecoration(
        color: CupertinoColors.systemGrey6,
        borderRadius: BorderRadius.circular(6),
      ),
      child: const Icon(
        CupertinoIcons.doc_text,
        size: 28,
        color: CupertinoColors.systemGrey4,
      ),
    );
  }

  String _formatNum(int num) {
    if (num >= 10000) {
      return '${(num / 10000).toStringAsFixed(num >= 100000 ? 0 : 1)}万';
    }
    return num.toString();
  }
}

class _Badge extends StatelessWidget {
  const _Badge({required this.icon, required this.text});
  final IconData icon;
  final String text;

  @override
  Widget build(BuildContext context) {
    return Row(
      mainAxisSize: MainAxisSize.min,
      children: [
        Icon(icon, size: 13, color: CupertinoColors.systemGrey),
        const SizedBox(width: 3),
        Text(
          text,
          style: const TextStyle(
            fontSize: 11,
            color: CupertinoColors.systemGrey,
            fontWeight: FontWeight.w500,
          ),
        ),
      ],
    );
  }
}
