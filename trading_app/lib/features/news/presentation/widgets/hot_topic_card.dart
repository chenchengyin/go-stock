/// 热门话题卡片 — 匹配前端 HotTopics.vue 风格
library;

import 'package:flutter/material.dart';
import 'package:cached_network_image/cached_network_image.dart';

import '../../domain/news_models.dart';

class HotTopicCard extends StatelessWidget {
  const HotTopicCard({
    super.key,
    required this.item,
    this.onTap,
  });

  final HotTopicItem item;
  final VoidCallback? onTap;

  @override
  Widget build(BuildContext context) {
    return Card(
      margin: const EdgeInsets.only(bottom: 6),
      elevation: 0.5,
      shape: RoundedRectangleBorder(
        borderRadius: BorderRadius.circular(8),
        side: BorderSide(color: Colors.grey.withValues(alpha: 0.12)),
      ),
      child: InkWell(
        borderRadius: BorderRadius.circular(8),
        onTap: onTap,
        child: Padding(
          padding: const EdgeInsets.fromLTRB(12, 10, 12, 10),
          child: Row(
            crossAxisAlignment: CrossAxisAlignment.start,
            children: [
              // 左侧头像/图片（使用 CachedNetworkImage）
              if (item.squareImg != null && item.squareImg!.isNotEmpty)
                ClipRRect(
                  borderRadius: BorderRadius.circular(6),
                  child: CachedNetworkImage(
                    imageUrl: item.squareImg!,
                    width: 60,
                    height: 60,
                    fit: BoxFit.cover,
                    placeholder: (_, __) => _buildPlaceholder(),
                    errorWidget: (_, __, ___) => _buildPlaceholder(),
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
                      ),
                      maxLines: 2,
                      overflow: TextOverflow.ellipsis,
                    ),
                    const SizedBox(height: 4),
                    // 描述
                    Text(
                      item.desc,
                      style: TextStyle(
                        fontSize: 12,
                        color: Colors.grey.shade600,
                        height: 1.4,
                      ),
                      maxLines: 2,
                      overflow: TextOverflow.ellipsis,
                    ),
                    const SizedBox(height: 6),
                    // 底部：讨论数 + 浏览量 + 股票标签
                    Row(
                      children: [
                        _buildBadge(
                          Icons.chat_bubble_outline,
                          _formatNum(item.postNumber),
                          Colors.orange,
                        ),
                        const SizedBox(width: 8),
                        _buildBadge(
                          Icons.visibility_outlined,
                          _formatNum(item.clickNumber),
                          Colors.orange,
                        ),
                        const Spacer(),
                        if (item.stockList.isNotEmpty)
                          Flexible(
                            child: Text(
                              item.stockList.take(3).join(' '),
                              style: TextStyle(
                                fontSize: 10,
                                color: Colors.blue.shade600,
                              ),
                              maxLines: 1,
                              overflow: TextOverflow.ellipsis,
                            ),
                          ),
                      ],
                    ),
                    // 股票标签
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
                              color: Colors.blue.withValues(alpha: 0.08),
                              borderRadius: BorderRadius.circular(4),
                            ),
                            child: Text(
                              s,
                              style: TextStyle(
                                fontSize: 10,
                                color: Colors.blue.shade600,
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
      ),
    );
  }

  Widget _buildPlaceholder() {
    return Container(
      width: 60,
      height: 60,
      decoration: BoxDecoration(
        color: Colors.grey.shade100,
        borderRadius: BorderRadius.circular(6),
      ),
      child: Icon(Icons.forum_outlined, size: 28, color: Colors.grey.shade300),
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
          style: TextStyle(fontSize: 11, color: color, fontWeight: FontWeight.w500),
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
