/// 新闻卡片 — iOS 原生资讯列表风格
library;

import 'package:flutter/cupertino.dart';

import '../../core/theme/app_colors.dart';
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

  Color get _titleColor =>
      item.isRed ? const Color(0xffb71c1c) : AppColors.textPrimary;

  @override
  Widget build(BuildContext context) {
    AppColors.of(context);
    return GestureDetector(
      onTap: () => _navigateToDetail(context),
      child: Container(
        padding: const EdgeInsets.symmetric(horizontal: 16, vertical: 12),
        decoration: BoxDecoration(
          color: AppColors.cardBg,
          border: Border(
            bottom: BorderSide(color: AppColors.divider, width: 0.5),
          ),
        ),
        child: Column(
          crossAxisAlignment: CrossAxisAlignment.start,
          children: [
            // 标题 + 时间
            Row(
              crossAxisAlignment: CrossAxisAlignment.start,
              children: [
                Expanded(
                  child: item.title.isNotEmpty
                      ? Text(
                          item.title,
                          style: TextStyle(
                            fontSize: 15,
                            fontWeight: FontWeight.w500,
                            height: 1.35,
                            color: _titleColor,
                          ),
                          maxLines: 2,
                          overflow: TextOverflow.ellipsis,
                        )
                      : Text(
                          item.content,
                          style: TextStyle(
                            fontSize: 15,
                            fontWeight: FontWeight.w500,
                            height: 1.35,
                            color: _titleColor,
                          ),
                          maxLines: 2,
                          overflow: TextOverflow.ellipsis,
                        ),
                ),
                const SizedBox(width: 8),
                _TimeColumn(time: item.time, dataTime: item.dataTime),
              ],
            ),
            if (item.subjects.isNotEmpty ||
                item.sentimentResult.isNotEmpty) ...[
              const SizedBox(height: 8),
              Wrap(
                spacing: 8,
                runSpacing: 6,
                children: [
                  // 看涨/看跌标签排在第一位
                  if (item.sentimentResult.isNotEmpty)
                    _buildSentimentTag(item.sentimentResult),
                  ...item.subjects
                      .take(3)
                      .map(
                        (s) => _Tag(
                          text: s,
                          textColor: _sourceColor,
                          bgColor: _sourceColor.withValues(alpha: 0.08),
                        ),
                      ),
                  // 来源标签放在最后
                  _Tag(
                    text: item.source,
                    textColor: _sourceColor,
                    bgColor: _sourceColor.withValues(alpha: 0.08),
                  ),
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
      '看涨' => (
        const Color(0xffe53935),
        const Color(0xffe53935).withValues(alpha: 0.08),
      ),
      '看跌' => (
        const Color(0xff0d904f),
        const Color(0xff0d904f).withValues(alpha: 0.08),
      ),
      _ => (AppColors.textTertiary, AppColors.surfaceBg),
    };
    return _Tag(text: sentiment, textColor: textColor, bgColor: bgColor);
  }

  void _navigateToDetail(BuildContext context) {
    Navigator.of(context).push(
      CupertinoPageRoute(builder: (context) => NewsDetailPage(item: item)),
    );
  }
}

class _TimeColumn extends StatelessWidget {
  const _TimeColumn({required this.time, this.dataTime});

  final String time;
  final String? dataTime;

  @override
  Widget build(BuildContext context) {
    String date = '';
    String hms = time;
    // 从 dataTime 解析日期部分
    if (dataTime != null && dataTime!.contains('T')) {
      date = dataTime!.split('T')[0];
    }
    // 如果 time 是完整格式（含空格），切分
    if (time.contains(' ')) {
      final parts = time.split(' ');
      date = parts[0];
      hms = parts.length > 1 ? parts[1] : '';
    }
    // 转成"今天"/"昨天"
    String displayLabel = date;
    if (date.isNotEmpty) {
      final now = DateTime.now();
      final dateObj = DateTime.tryParse(date);
      if (dateObj != null) {
        final diff = now.difference(dateObj).inDays;
        if (diff == 0) {
          displayLabel = '今天';
        } else if (diff == 1) {
          displayLabel = '昨天';
        }
      }
    }
    return Column(
      crossAxisAlignment: CrossAxisAlignment.end,
      children: [
        if (displayLabel.isNotEmpty)
          Text(
            displayLabel,
            style: TextStyle(fontSize: 11, color: AppColors.textTertiary),
          ),
        if (hms.isNotEmpty)
          Text(
            hms,
            style: TextStyle(fontSize: 11, color: AppColors.textTertiary),
          ),
      ],
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
