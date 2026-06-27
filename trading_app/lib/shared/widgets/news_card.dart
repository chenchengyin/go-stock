/// 新闻卡片 — 匹配 go-stock 桌面端样式
library;

import 'package:flutter/material.dart';

import '../../features/news/domain/news_models.dart';
import '../../features/news/presentation/pages/news_detail_page.dart';

class NewsCard extends StatelessWidget {
  const NewsCard({
    super.key,
    required this.item,
  });

  final NewsItem item;

  Color get _textColor => item.isRed ? Colors.red.shade700 : Colors.black87;
  Color get _timeColor => item.isRed ? Colors.red : Colors.orange;

  @override
  Widget build(BuildContext context) {
    return Card(
      margin: const EdgeInsets.only(bottom: 4),
      elevation: 0.5,
      shape: RoundedRectangleBorder(
        borderRadius: BorderRadius.circular(6),
        side: BorderSide(
          color: item.isRed ? Colors.red.withValues(alpha: 0.15) : Colors.grey.withValues(alpha: 0.15),
        ),
      ),
      child: InkWell(
        borderRadius: BorderRadius.circular(6),
        onTap: () => _navigateToDetail(context),
        child: Padding(
          padding: const EdgeInsets.fromLTRB(12, 8, 12, 10),
          child: Column(
            crossAxisAlignment: CrossAxisAlignment.start,
            children: [
              // 第一行：时间标签 + 内容
              _buildContentLine(context),
              // 底部标签行
              _buildTagsRow(context),
            ],
          ),
        ),
      ),
    );
  }

  /// 时间标签 + 标题/内容
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
      // 有标题 → 时间标签 + 标题
      return Wrap(
        crossAxisAlignment: WrapCrossAlignment.center,
        children: [
          timeBadge,
          Flexible(
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

    // 无标题 → 时间标签 + 内容摘要
    return Column(
      crossAxisAlignment: CrossAxisAlignment.start,
      children: [
        Row(
          children: [
            timeBadge,
            if (item.isRed)
              Container(
                margin: const EdgeInsets.only(left: 2),
                padding: const EdgeInsets.symmetric(horizontal: 5, vertical: 2),
                decoration: BoxDecoration(
                  color: Colors.red.withValues(alpha: 0.12),
                  borderRadius: BorderRadius.circular(4),
                ),
                child: const Text(
                  '重要',
                  style: TextStyle(
                    color: Colors.red,
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

  /// 底部标签行：题材标签 + 情感 + 股票
  Widget _buildTagsRow(BuildContext context) {
    final List<Widget> chips = [];

    // 题材标签（绿色）
    for (final sub in item.subjects.take(5)) {
      chips.add(_buildTag(sub, Colors.green.shade700, Colors.green.shade50));
    }

    // 情感标签
    if (item.sentimentResult.isNotEmpty) {
      final (Color textColor, Color bgColor) = switch (item.sentimentResult) {
        '看涨' => (Colors.red.shade700, Colors.red.shade50),
        '看跌' => (Colors.green.shade700, Colors.green.shade50),
        _ => (Colors.grey.shade700, Colors.grey.shade100),
      };
      chips.add(_buildTag(item.sentimentResult, textColor, bgColor));
    }

    if (chips.isEmpty) return const SizedBox.shrink();

    return Padding(
      padding: const EdgeInsets.only(top: 6),
      child: Wrap(
        spacing: 4,
        runSpacing: 4,
        children: chips,
      ),
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
      MaterialPageRoute(
        builder: (context) => NewsDetailPage(item: item),
      ),
    );
  }
}
