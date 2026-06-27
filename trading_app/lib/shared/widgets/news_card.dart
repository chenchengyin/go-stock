/// 新闻卡片 — 简洁卡片风格
library;

import 'package:flutter/cupertino.dart';

import '../../features/news/domain/news_models.dart';
import '../../features/news/presentation/news_detail/news_detail_page.dart';

class NewsCard extends StatelessWidget {
  const NewsCard({super.key, required this.item});

  final NewsItem item;

  Color get _accentColor =>
      item.isRed ? const Color(0xffe53935) : const Color(0xff1a73e8);

  @override
  Widget build(BuildContext context) {
    return GestureDetector(
      onTap: () => _navigateToDetail(context),
      child: Container(
        margin: const EdgeInsets.symmetric(horizontal: 12, vertical: 4),
        padding: const EdgeInsets.fromLTRB(12, 10, 12, 12),
        decoration: BoxDecoration(
          color: CupertinoColors.white,
          borderRadius: BorderRadius.circular(10),
          boxShadow: [
            BoxShadow(
              color: CupertinoColors.systemGrey4.withValues(alpha: 0.3),
              blurRadius: 6,
              offset: const Offset(0, 2),
            ),
          ],
        ),
        child: Column(
          crossAxisAlignment: CrossAxisAlignment.start,
          children: [
            // 第一行：时间 + 来源 + 重要标记
            Row(
              children: [
                _TimeBadge(time: item.time, color: _accentColor),
                if (item.isRed) ...[
                  const SizedBox(width: 6),
                  _Tag(
                    text: '重要',
                    textColor: const Color(0xffe53935),
                    bgColor: const Color(0xffe53935).withValues(alpha: 0.08),
                  ),
                ],
                const Spacer(),
                Text(
                  item.source,
                  style: const TextStyle(
                    fontSize: 11,
                    color: CupertinoColors.systemGrey,
                  ),
                ),
              ],
            ),
            const SizedBox(height: 8),
            // 正文（标题或内容）
            if (item.title.isNotEmpty)
              Text(
                item.title,
                style: const TextStyle(
                  fontSize: 14,
                  fontWeight: FontWeight.w600,
                  height: 1.4,
                  color: CupertinoColors.black,
                ),
                maxLines: 2,
                overflow: TextOverflow.ellipsis,
              )
            else
              Text(
                item.content,
                style: const TextStyle(
                  fontSize: 13,
                  height: 1.5,
                  color: CupertinoColors.black,
                ),
                maxLines: 3,
                overflow: TextOverflow.ellipsis,
              ),
            // 标签行
            if (item.subjects.isNotEmpty || item.sentimentResult.isNotEmpty) ...[
              const SizedBox(height: 8),
              Wrap(
                spacing: 6,
                runSpacing: 4,
                children: [
                  ...item.subjects.take(4).map(
                        (s) => _Tag(
                          text: s,
                          textColor: const Color(0xff0d904f),
                          bgColor: const Color(0xff0d904f).withValues(alpha: 0.08),
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
    );
  }

  Widget _buildSentimentTag(String sentiment) {
    final (Color textColor, Color bgColor) = switch (sentiment) {
      '看涨' => (const Color(0xffe53935), const Color(0xffe53935).withValues(alpha: 0.08)),
      '看跌' => (const Color(0xff0d904f), const Color(0xff0d904f).withValues(alpha: 0.08)),
      _ => (CupertinoColors.systemGrey, CupertinoColors.systemGrey5),
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

class _TimeBadge extends StatelessWidget {
  const _TimeBadge({required this.time, required this.color});
  final String time;
  final Color color;

  @override
  Widget build(BuildContext context) {
    return Container(
      padding: const EdgeInsets.symmetric(horizontal: 6, vertical: 2),
      decoration: BoxDecoration(
        color: color.withValues(alpha: 0.1),
        borderRadius: BorderRadius.circular(4),
      ),
      child: Text(
        time,
        style: TextStyle(
          color: color,
          fontSize: 11,
          fontWeight: FontWeight.w700,
          fontFeatures: const [FontFeature.tabularFigures()],
        ),
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
      padding: const EdgeInsets.symmetric(horizontal: 7, vertical: 2),
      decoration: BoxDecoration(
        color: bgColor,
        borderRadius: BorderRadius.circular(4),
      ),
      child: Text(
        text,
        style: TextStyle(
          fontSize: 10,
          color: textColor,
          fontWeight: FontWeight.w600,
        ),
      ),
    );
  }
}
