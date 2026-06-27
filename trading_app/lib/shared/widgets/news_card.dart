/// 新闻卡片 — iOS 原生资讯列表风格
library;

import 'package:flutter/cupertino.dart';

import '../../features/news/domain/news_models.dart';
import '../../features/news/presentation/news_detail/news_detail_page.dart';

class NewsCard extends StatelessWidget {
  const NewsCard({super.key, required this.item});

  final NewsItem item;

  Color get _sourceColor {
    final s = item.source;
    if (s.contains('新浪') || s.contains('财联社')) {
      return const Color(0xff1a73e8);
    }
    if (s.contains('外媒') || s.contains('国际')) {
      return const Color(0xffff9800);
    }
    return const Color(0xff4caf50);
  }

  @override
  Widget build(BuildContext context) {
    return GestureDetector(
      onTap: () => _navigateToDetail(context),
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
            // 左侧时间
            SizedBox(
              width: 56,
              child: Text(
                item.time,
                style: const TextStyle(
                  fontSize: 12,
                  color: CupertinoColors.systemGrey,
                  fontFeatures: [FontFeature.tabularFigures()],
                ),
              ),
            ),
            const SizedBox(width: 12),
            // 右侧标题 + 标签
            Expanded(
              child: Column(
                crossAxisAlignment: CrossAxisAlignment.start,
                children: [
                  if (item.title.isNotEmpty)
                    Text(
                      item.title,
                      style: const TextStyle(
                        fontSize: 15,
                        fontWeight: FontWeight.w500,
                        height: 1.35,
                        color: CupertinoColors.black,
                      ),
                      maxLines: 2,
                      overflow: TextOverflow.ellipsis,
                    )
                  else
                    Text(
                      item.content,
                      style: const TextStyle(
                        fontSize: 15,
                        fontWeight: FontWeight.w500,
                        height: 1.35,
                        color: CupertinoColors.black,
                      ),
                      maxLines: 2,
                      overflow: TextOverflow.ellipsis,
                    ),
                  if (item.subjects.isNotEmpty ||
                      item.sentimentResult.isNotEmpty) ...[
                    const SizedBox(height: 8),
                    Wrap(
                      spacing: 8,
                      runSpacing: 6,
                      children: [
                        ...item.subjects.take(3).map(
                              (s) => _Tag(
                                text: s,
                                textColor: _sourceColor,
                                bgColor: _sourceColor.withValues(alpha: 0.08),
                              ),
                            ),
                        if (item.sentimentResult.isNotEmpty)
                          _buildSentimentTag(item.sentimentResult),
                      ],
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

  Widget _buildSentimentTag(String sentiment) {
    final (Color textColor, Color bgColor) = switch (sentiment) {
      '看涨' => (
          const Color(0xffe53935),
          const Color(0xffe53935).withValues(alpha: 0.08)
        ),
      '看跌' => (
          const Color(0xff0d904f),
          const Color(0xff0d904f).withValues(alpha: 0.08)
        ),
      _ => (
          CupertinoColors.systemGrey,
          CupertinoColors.systemGrey6
        ),
    };
    return _Tag(text: sentiment, textColor: textColor, bgColor: bgColor);
  }

  void _navigateToDetail(BuildContext context) {
    Navigator.of(context).push(
      CupertinoPageRoute(
        builder: (context) => NewsDetailPage(item: item),
      ),
    );
  }
}

class _Tag extends StatelessWidget {
  const _Tag({
    required this.text,
    required this.textColor,
    required this.bgColor,
  });
  final String text;
  final Color textColor;
  final Color bgColor;

  @override
  Widget build(BuildContext context) {
    return Container(
      padding: const EdgeInsets.symmetric(horizontal: 6, vertical: 2),
      decoration: BoxDecoration(
        color: bgColor,
        borderRadius: BorderRadius.circular(4),
      ),
      child: Text(
        text,
        style: TextStyle(
          fontSize: 10,
          color: textColor,
          fontWeight: FontWeight.w500,
        ),
      ),
    );
  }
}
