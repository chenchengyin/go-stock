/// 新闻卡片 — Cupertino 风格
library;

import 'package:flutter/cupertino.dart';

import '../../features/news/domain/news_models.dart';
import '../../features/news/presentation/pages/news_detail_page.dart';

class NewsCard extends StatelessWidget {
  const NewsCard({super.key, required this.item});

  final NewsItem item;

  Color get _textColor =>
      item.isRed ? CupertinoColors.systemRed : CupertinoColors.black;
  Color get _timeColor =>
      item.isRed ? CupertinoColors.systemRed : CupertinoColors.systemOrange;

  @override
  Widget build(BuildContext context) {
    return Container(
      margin: const EdgeInsets.only(bottom: 4),
      decoration: BoxDecoration(
        color: CupertinoColors.white,
        borderRadius: BorderRadius.circular(8),
        border: Border.all(
          color: item.isRed
              ? CupertinoColors.systemRed.withValues(alpha: 0.2)
              : CupertinoColors.systemGrey5,
        ),
      ),
      child: CupertinoButton(
        padding: const EdgeInsets.fromLTRB(12, 8, 12, 10),
        pressedOpacity: 0.7,
        onPressed: () => _navigateToDetail(context),
        child: Column(
          crossAxisAlignment: CrossAxisAlignment.start,
          children: [
            _buildContentLine(context),
            if (_hasTags) _buildTagsRow(context),
          ],
        ),
      ),
    );
  }

  bool get _hasTags =>
      item.subjects.isNotEmpty || item.sentimentResult.isNotEmpty;

  Widget _buildContentLine(BuildContext context) {
    final Widget timeBadge = Container(
      margin: const EdgeInsets.only(right: 6, bottom: 2),
      padding: const EdgeInsets.symmetric(horizontal: 6, vertical: 2),
      decoration: BoxDecoration(
        color: _timeColor.withValues(alpha: 0.12),
        borderRadius: BorderRadius.circular(4),
      ),
      child: Text(
        item.time,
        style: TextStyle(
          color: _timeColor,
          fontSize: 11,
          fontWeight: FontWeight.w700,
          fontFeatures: const [FontFeature.tabularFigures()],
        ),
      ),
    );

    if (item.title.isNotEmpty) {
      return Row(
        crossAxisAlignment: CrossAxisAlignment.start,
        children: [
          timeBadge,
          const SizedBox(width: 4),
          Expanded(
            child: Text(
              item.title,
              style: TextStyle(
                fontSize: 13,
                fontWeight: FontWeight.w600,
                color: _textColor,
                height: 1.3,
              ),
              maxLines: 2,
              overflow: TextOverflow.ellipsis,
            ),
          ),
        ],
      );
    }

    return Column(
      crossAxisAlignment: CrossAxisAlignment.start,
      children: [
        Row(
          children: [
            timeBadge,
            if (item.isRed)
              Container(
                margin: const EdgeInsets.only(left: 2),
                padding:
                    const EdgeInsets.symmetric(horizontal: 5, vertical: 2),
                decoration: BoxDecoration(
                  color: CupertinoColors.systemRed.withValues(alpha: 0.12),
                  borderRadius: BorderRadius.circular(4),
                ),
                child: const Text(
                  '重要',
                  style: TextStyle(
                    color: CupertinoColors.systemRed,
                    fontSize: 10,
                    fontWeight: FontWeight.w700,
                  ),
                ),
              ),
          ],
        ),
        const SizedBox(height: 4),
        Text(
          item.content,
          style: TextStyle(
            fontSize: 12,
            color: _textColor.withValues(alpha: 0.85),
            height: 1.4,
          ),
          maxLines: 3,
          overflow: TextOverflow.ellipsis,
        ),
      ],
    );
  }

  Widget _buildTagsRow(BuildContext context) {
    final List<Widget> chips = [];

    for (final sub in item.subjects.take(5)) {
      chips.add(_buildTag(sub, CupertinoColors.systemGreen.darkColor,
          CupertinoColors.systemGreen.withValues(alpha: 0.1)));
    }

    if (item.sentimentResult.isNotEmpty) {
      final (Color, Color) colors = switch (item.sentimentResult) {
        '看涨' => (CupertinoColors.systemRed, CupertinoColors.systemRed.withValues(alpha: 0.1)),
        '看跌' => (CupertinoColors.systemGreen, CupertinoColors.systemGreen.withValues(alpha: 0.1)),
        _ => (CupertinoColors.systemGrey, CupertinoColors.systemGrey5),
      };
      chips.add(_buildTag(item.sentimentResult, colors.$1, colors.$2));
    }

    if (chips.isEmpty) return const SizedBox.shrink();

    return Padding(
      padding: const EdgeInsets.only(top: 6),
      child: Wrap(spacing: 4, runSpacing: 4, children: chips),
    );
  }

  Widget _buildTag(String text, Color textColor, Color bgColor) {
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
          fontWeight: FontWeight.w600,
        ),
      ),
    );
  }

  void _navigateToDetail(BuildContext context) {
    Navigator.of(context).push(
      CupertinoPageRoute(
        builder: (context) => NewsDetailPage(item: item),
      ),
    );
  }
}
