/// 热门话题卡片 — Cupertino 风格
library;

import 'package:flutter/cupertino.dart';

import '../../domain/news_models.dart';

class HotTopicCard extends StatelessWidget {
  const HotTopicCard({super.key, required this.item, this.onTap});

  final HotTopicItem item;
  final VoidCallback? onTap;

  @override
  Widget build(BuildContext context) {
    return Container(
      margin: const EdgeInsets.only(bottom: 6),
      decoration: BoxDecoration(
        color: CupertinoColors.white,
        borderRadius: BorderRadius.circular(10),
        border: Border.all(color: CupertinoColors.systemGrey5),
      ),
      child: CupertinoButton(
        padding: const EdgeInsets.fromLTRB(12, 10, 12, 10),
        pressedOpacity: 0.7,
        onPressed: onTap,
        child: Row(
          crossAxisAlignment: CrossAxisAlignment.start,
          children: [
            // 左侧头像/图片
            if (item.squareImg != null && item.squareImg!.isNotEmpty)
              ClipRRect(
                borderRadius: BorderRadius.circular(6),
                child: Image.network(
                  item.squareImg!,
                  width: 60,
                  height: 60,
                  fit: BoxFit.cover,
                  loadingBuilder: (_, child, progress) =>
                      progress == null ? child : _buildPlaceholder(),
                  errorBuilder: (_, __, ___) => _buildPlaceholder(),
                ),
              )
            else
              _buildPlaceholder(),
            const SizedBox(width: 10),
            // 右侧内容
            Expanded(
              child: Column(
                crossAxisAlignment: CrossAxisAlignment.start,
                children: [
                  // 标题
                  Text(
                    item.nickname,
                    style: const TextStyle(
                      fontSize: 14,
                      fontWeight: FontWeight.w600,
                      height: 1.3,
                      color: CupertinoColors.black,
                    ),
                    maxLines: 2,
                    overflow: TextOverflow.ellipsis,
                  ),
                  const SizedBox(height: 4),
                  // 描述
                  Text(
                    item.desc,
                    style: const TextStyle(
                      fontSize: 12,
                      color: CupertinoColors.systemGrey,
                      height: 1.4,
                    ),
                    maxLines: 2,
                    overflow: TextOverflow.ellipsis,
                  ),
                  const SizedBox(height: 6),
                  // 底部：讨论数 + 浏览量 + 股票标签
                  Row(
                    children: [
                      _buildBadge(CupertinoIcons.chat_bubble_2,
                          _formatNum(item.postNumber),
                          CupertinoColors.systemOrange),
                      const SizedBox(width: 8),
                      _buildBadge(CupertinoIcons.eye,
                          _formatNum(item.clickNumber),
                          CupertinoColors.systemOrange),
                      const Spacer(),
                      if (item.stockList.isNotEmpty)
                        Flexible(
                          child: Text(
                            item.stockList.take(3).join(' '),
                            style: TextStyle(
                              fontSize: 10,
                              color: CupertinoColors.systemBlue,
                            ),
                            maxLines: 1,
                            overflow: TextOverflow.ellipsis,
                          ),
                        ),
                    ],
                  ),
                  if (item.stockList.length > 3) ...[
                    const SizedBox(height: 4),
                    Wrap(
                      spacing: 4,
                      runSpacing: 2,
                      children: item.stockList.take(6).map((s) {
                        return Container(
                          padding: const EdgeInsets.symmetric(
                              horizontal: 6, vertical: 2),
                          decoration: BoxDecoration(
                            color: CupertinoColors.systemBlue
                                .withValues(alpha: 0.08),
                            borderRadius: BorderRadius.circular(4),
                          ),
                          child: Text(
                            s,
                            style: const TextStyle(
                              fontSize: 10,
                              color: CupertinoColors.systemBlue,
                            ),
                          ),
                        );
                      }).toList(),
                    ),
                  ],
                ],
              ),
            ),
          ],
        ),
      ),
    );
  }

  Widget _buildPlaceholder() {
    return Container(
      width: 60,
      height: 60,
      decoration: BoxDecoration(
        color: CupertinoColors.systemGrey5,
        borderRadius: BorderRadius.circular(6),
      ),
      child: Icon(CupertinoIcons.doc_text,
          size: 28, color: CupertinoColors.systemGrey4),
    );
  }

  Widget _buildBadge(IconData icon, String text, Color color) {
    return Row(
      mainAxisSize: MainAxisSize.min,
      children: [
        Icon(icon, size: 12, color: color),
        const SizedBox(width: 2),
        Text(
          text,
          style: TextStyle(
              fontSize: 11,
              color: color,
              fontWeight: FontWeight.w500),
        ),
      ],
    );
  }

  String _formatNum(int num) {
    if (num >= 10000) {
      return '${(num / 10000).toStringAsFixed(num >= 100000 ? 0 : 1)}万';
    }
    return num.toString();
  }
}
