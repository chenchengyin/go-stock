/// 新闻资讯数据模型 — 匹配 Go 后端 Telegraph 结构
library;

class NewsItem {
  const NewsItem({
    required this.id,
    required this.title,
    required this.content,
    required this.source,
    required this.time,
    required this.isRed,
    required this.url,
    required this.sentimentResult,
    this.subjects = const [],
    this.dataTime,
  });

  final int id;
  final String title;
  final String content;
  final String source;
  final String time;
  final bool isRed;
  final String url;
  final String sentimentResult;
  final List<String> subjects;
  final String? dataTime;

  /// 从 Go 后端 /api/news 返回的 JSON 创建
  factory NewsItem.fromJson(Map<String, dynamic> json) {
    return NewsItem(
      id: json['ID'] as int? ?? 0,
      title: (json['title'] as String? ?? '').trim(),
      content: (json['content'] as String? ?? '').trim(),
      source: json['source'] as String? ?? '',
      time: json['time'] as String? ?? '',
      isRed: json['isRed'] as bool? ?? false,
      url: json['url'] as String? ?? '',
      sentimentResult: json['sentimentResult'] as String? ?? '',
      subjects: (json['subjects'] as List<dynamic>?)
              ?.map((e) => e.toString())
              .toList() ??
          [],
      dataTime: json['dataTime'] as String?,
    );
  }
}

/// 热门话题数据模型 — 匹配 Go 后端 HotTopic 结构
class HotTopicItem {
  const HotTopicItem({
    required this.htid,
    required this.nickname,
    required this.desc,
    this.squareImg,
    this.clickNumber = 0,
    this.postNumber = 0,
    this.stockList = const [],
  });

  final String htid;
  final String nickname;
  final String desc;
  final String? squareImg;
  final int clickNumber;
  final int postNumber;
  final List<String> stockList;

  factory HotTopicItem.fromJson(Map<String, dynamic> json) {
    final stocks = <String>[];
    if (json['stock_list'] is List) {
      for (final s in json['stock_list']) {
        if (s is Map<String, dynamic>) {
          final name = s['name']?.toString();
          if (name != null && name.isNotEmpty) stocks.add(name);
        }
      }
    }

    return HotTopicItem(
      htid: json['htid']?.toString() ?? '',
      nickname: json['nickname']?.toString() ?? '',
      desc: json['desc']?.toString() ?? '',
      squareImg: json['squareImg']?.toString(),
      clickNumber: (json['clickNumber'] as num?)?.toInt() ?? 0,
      postNumber: (json['postNumber'] as num?)?.toInt() ?? 0,
      stockList: stocks,
    );
  }
}

/// 新闻列表数据源
enum NewsSource {
  cailianpress('财联社电报'),
  sina('新浪财经'),
  foreign('外媒');

  final String label;
  const NewsSource(this.label);
}

class NewsListResult {
  const NewsListResult({
    required this.source,
    required this.items,
  });

  final String source;
  final List<NewsItem> items;

  factory NewsListResult.fromJson(String source, List<dynamic> jsonList) {
    final items = jsonList
        .whereType<Map<String, dynamic>>()
        .map((e) => NewsItem.fromJson(e))
        .toList();
    return NewsListResult(source: source, items: items);
  }
}
