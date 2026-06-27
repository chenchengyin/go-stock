/// 新闻详情页
library;

import 'package:flutter/material.dart';
import 'package:url_launcher/url_launcher.dart';

import '../../domain/news_models.dart';

class NewsDetailPage extends StatelessWidget {
  const NewsDetailPage({super.key, required this.item});

  final NewsItem item;

  @override
  Widget build(BuildContext context) {
    return Scaffold(
      backgroundColor: const Color(0xfff6f8fb),
      appBar: AppBar(
        title: const Text('详情'),
        actions: [
          if (item.url.isNotEmpty)
            IconButton(
              icon: const Icon(Icons.open_in_new),
              tooltip: '在浏览器中打开',
              onPressed: () => _openInBrowser(context),
            ),
        ],
      ),
      body: SingleChildScrollView(
        padding: const EdgeInsets.all(16),
        child: Column(
          crossAxisAlignment: CrossAxisAlignment.start,
          children: [
            // 头部
            Row(
              children: [
                if (item.isRed)
                  Container(
                    padding: const EdgeInsets.symmetric(horizontal: 8, vertical: 4),
                    decoration: BoxDecoration(
                      color: Colors.red.withValues(alpha: 0.12),
                      borderRadius: BorderRadius.circular(6),
                    ),
                    child: const Text(
                      '重要',
                      style: TextStyle(
                        color: Colors.red,
                        fontSize: 11,
                        fontWeight: FontWeight.w700,
                      ),
                    ),
                  ),
                if (item.isRed) const SizedBox(width: 8),
                Text(
                  item.source,
                  style: const TextStyle(fontSize: 13, color: Color(0xff5f6368)),
                ),
                const Spacer(),
                Text(
                  item.time,
                  style: const TextStyle(fontSize: 12, color: Colors.grey),
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
            const Divider(height: 1),

            // 正文
            const SizedBox(height: 12),
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
                  return Chip(
                    label: Text(subject, style: const TextStyle(fontSize: 12)),
                    visualDensity: VisualDensity.compact,
                    padding: const EdgeInsets.symmetric(horizontal: 6, vertical: 2),
                  );
                }).toList(),
              ),
            ],

            if (item.url.isNotEmpty) ...[
              const SizedBox(height: 24),
              SizedBox(
                width: double.infinity,
                child: OutlinedButton.icon(
                  onPressed: () => _openInBrowser(context),
                  icon: const Icon(Icons.open_in_browser),
                  label: const Text('查看原文'),
                ),
              ),
            ],
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
      } else if (context.mounted) {
        ScaffoldMessenger.of(context).showSnackBar(
          const SnackBar(content: Text('无法打开链接')),
        );
      }
    } catch (e) {
      if (context.mounted) {
        ScaffoldMessenger.of(context).showSnackBar(
          SnackBar(content: Text('打开失败: $e')),
        );
      }
    }
  }
}
