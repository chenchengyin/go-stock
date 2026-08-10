class ShortTermEmotionExplainPageData {
  const ShortTermEmotionExplainPageData({
    required this.title,
    required this.subtitle,
    required this.summary,
    required this.sections,
  });

  final String title;
  final String subtitle;
  final String summary;
  final List<ShortTermEmotionExplainSection> sections;
}

class ShortTermEmotionExplainSection {
  const ShortTermEmotionExplainSection({
    required this.title,
    this.body,
    this.rows = const [],
  });

  final String title;
  final String? body;
  final List<ShortTermEmotionExplainRow> rows;
}

class ShortTermEmotionExplainRow {
  const ShortTermEmotionExplainRow({
    required this.label,
    required this.value,
    this.note,
  });

  final String label;
  final String value;
  final String? note;
}
