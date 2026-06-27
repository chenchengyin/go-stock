/// 新闻详情页 — Cupertino 风格
library;

import 'package:flutter/cupertino.dart';
import 'package:url_launcher/url_launcher.dart';

import '../../domain/news_models.dart';

class NewsDetailPage extends StatelessWidget {
  const NewsDetailPage({super.key, required this.item});

  final NewsItem item;

  @override
  Widget build(BuildContext context) {
    return CupertinoPageScaffold(
      backgroundColor: CupertinoColors.white,
      navigationBar: const CupertinoNavigationBar(
        middle: Text('详情'),
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
                    // 头部
                    Row(
                      children: [
                        if (item.isRed)
                          Container(
                            padding: const EdgeInsets.symmetric(
                                horizontal: 8, vertical: 4),
                            decoration: BoxDecoration(
                              color: CupertinoColors.systemRed.withValues(alpha: 0.12),
                              borderRadius: BorderRadius.circular(6),
                            ),
                            child: const Text(
                              '重要',
                              style: TextStyle(
                                color: CupertinoColors.systemRed,
                                fontSize: 11,
                                fontWeight: FontWeight.w700,
                              ),
                            ),
                          ),
                        if (item.isRed) const SizedBox(width: 8),
                        Text(
                          item.source,
                          style: const TextStyle(
                              fontSize: 13,
                              color: CupertinoColors.systemGrey),
                        ),
                        const Spacer(),
                        Text(
                          item.time,
                          style: const TextStyle(
                              fontSize: 12,
                              color: CupertinoColors.systemGrey),
                        ),
                      ],
                    ),
                    const SizedBox(height: 12),
                    // 标题
                    if (item.title.isNotEmpty) ...[
                      Text(
                        item.title,
                        style: const TextStyle(
                          fontSize: 18,
                          fontWeight: FontWeight.bold,
                          height: 1.3,
                        ),
                      ),
                      const SizedBox(height: 12),
                    ],
                    // 分隔线
                    Container(height: 1, color: CupertinoColors.systemGrey5),
                    const SizedBox(height: 12),
                    // 正文
                    Text(
                      item.content,
                      style: const TextStyle(fontSize: 15, height: 1.6),
                    ),
                    // 题材标签
                    if (item.subjects.isNotEmpty) ...[
                      const SizedBox(height: 16),
                      Wrap(
                        spacing: 6,
                        runSpacing: 6,
                        children: item.subjects.map((subject) {
                          return Container(
                            padding: const EdgeInsets.symmetric(
                                horizontal: 10, vertical: 4),
                            decoration: BoxDecoration(
                              color: CupertinoColors.systemGrey5,
                              borderRadius: BorderRadius.circular(6),
                            ),
                            child: Text(subject,
                                style: const TextStyle(fontSize: 12)),
                          );
                        }).toList(),
                      ),
                    ],
                  ],
                ),
              ),
            ),
            if (item.url.isNotEmpty)
              Padding(
                padding: const EdgeInsets.fromLTRB(16, 8, 16, 24),
                child: SafeArea(
                  top: false,
                  child: SizedBox(
                    width: double.infinity,
                    height: 44,
                    child: CupertinoButton.filled(
                      child: const Text('查看原文'),
                      onPressed: () => _openInBrowser(context),
                    ),
                  ),
                ),
              ),
          ],
        ),
      ),
    );
  }

  Future<void> _openInBrowser(BuildContext context) async {
    final uri = Uri.parse(item.url);
    try {
      if (await canLaunchUrl(uri)) {
        await launchUrl(uri, mode: LaunchMode.externalApplication);
      }
    } catch (_) {}
  }
}
