/// 热门话题详情页 — Cupertino 风格
library;

import 'package:flutter/cupertino.dart';
import 'package:url_launcher/url_launcher.dart';

import '../../domain/news_models.dart';

class HotTopicDetailPage extends StatelessWidget {
  const HotTopicDetailPage({super.key, required this.item});

  final HotTopicItem item;

  @override
  Widget build(BuildContext context) {
    final topicUrl =
        'https://gubatopic.eastmoney.com/topic_v3.html?htid=${item.htid}';

    return CupertinoPageScaffold(
      navigationBar: const CupertinoNavigationBar(
        middle: Text('热门话题'),
      ),
      child: SafeArea(
        child: Column(
          children: [
            Expanded(
              child: SingleChildScrollView(
                padding: const EdgeInsets.all(16),
                child: Column(
                  crossAxisAlignment: CrossAxisAlignment.start,
                  children: [
                    // 标题
                    Text(
                      item.nickname,
                      style: const TextStyle(
                        fontSize: 20,
                        fontWeight: FontWeight.bold,
                        height: 1.4,
                      ),
                    ),
                    const SizedBox(height: 8),
                    // 元信息：讨论数 + 浏览量
                    Row(
                      children: [
                        _buildMeta(CupertinoIcons.chat_bubble_2,
                            _formatNum(item.postNumber)),
                        const SizedBox(width: 16),
                        _buildMeta(CupertinoIcons.eye,
                            _formatNum(item.clickNumber)),
                      ],
                    ),
                    const SizedBox(height: 16),
                    // 关联股票标签
                    if (item.stockList.isNotEmpty) ...[
                      Wrap(
                        spacing: 6,
                        runSpacing: 4,
                        children: item.stockList.map((s) {
                          return Container(
                            padding: const EdgeInsets.symmetric(
                                horizontal: 8, vertical: 3),
                            decoration: BoxDecoration(
                              color: CupertinoColors.systemBlue
                                  .withValues(alpha: 0.08),
                              borderRadius: BorderRadius.circular(4),
                            ),
                            child: Text(
                              s,
                              style: const TextStyle(
                                fontSize: 12,
                                color: CupertinoColors.systemBlue,
                              ),
                            ),
                          );
                        }).toList(),
                      ),
                      const SizedBox(height: 16),
                    ],
                    // 分割线
                    Container(
                        height: 1,
                        color: CupertinoColors.systemGrey5),
                    const SizedBox(height: 12),
                    // 正文
                    Text(
                      item.desc,
                      style: const TextStyle(
                        fontSize: 15,
                        color: CupertinoColors.black,
                        height: 1.7,
                      ),
                    ),
                  ],
                ),
              ),
            ),
            // 底部按钮
            Container(
              padding: const EdgeInsets.fromLTRB(16, 8, 16, 24),
              decoration: BoxDecoration(
                color: CupertinoTheme.of(context).scaffoldBackgroundColor,
                border: Border(
                  top: BorderSide(
                      color: CupertinoColors.systemGrey5.withValues(alpha: 0.5)),
                ),
              ),
              child: SafeArea(
                top: false,
                child: SizedBox(
                  width: double.infinity,
                  height: 44,
                  child: CupertinoButton.filled(
                    child: const Text('查看原文'),
                    onPressed: () {
                      launchUrl(Uri.parse(topicUrl),
                          mode: LaunchMode.externalApplication);
                    },
                  ),
                ),
              ),
            ),
          ],
        ),
      ),
    );
  }

  Widget _buildMeta(IconData icon, String text) {
    return Row(
      mainAxisSize: MainAxisSize.min,
      children: [
        Icon(icon, size: 14, color: CupertinoColors.systemGrey),
        const SizedBox(width: 4),
        Text(
          text,
          style: const TextStyle(
              fontSize: 12, color: CupertinoColors.systemGrey),
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
