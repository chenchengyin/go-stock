class ShortTermEmotion {
  const ShortTermEmotion({
    required this.score,
    required this.phase,
    required this.action,
    required this.riskLevel,
    required this.suggestedWeight,
    required this.mainTheme,
    required this.updateTime,
    required this.isTrading,
    required this.explanation,
    required this.dashboard,
    required this.components,
    required this.riskSignals,
    required this.intradayEvents,
    required this.intradayTrend,
  });

  factory ShortTermEmotion.fromJson(Map<String, dynamic> json) {
    return ShortTermEmotion(
      score: (json['score'] as num?)?.toInt() ?? 0,
      phase: json['phase'] as String? ?? '数据不足',
      action: json['action'] as String? ?? '谨慎观察',
      riskLevel: json['riskLevel'] as String? ?? '未知',
      suggestedWeight: json['suggestedWeight'] as String? ?? '0成',
      mainTheme: json['mainTheme'] as String? ?? '无明显主线',
      updateTime: json['updateTime'] as String? ?? '',
      isTrading: json['isTrading'] as bool? ?? false,
      explanation: json['explanation'] as String? ?? '',
      dashboard: _parseList(json['dashboard'], ShortTermEmotionMetric.fromJson),
      components: _parseList(
        json['components'],
        ShortTermEmotionComponent.fromJson,
      ),
      riskSignals: _parseList(
        json['riskSignals'],
        ShortTermEmotionSignal.fromJson,
      ),
      intradayEvents: _parseList(
        json['intradayEvents'],
        ShortTermEmotionEvent.fromJson,
      ),
      intradayTrend: _parseList(
        json['intradayTrend'],
        ShortTermEmotionTrendPoint.fromJson,
      ),
    );
  }

  final int score;
  final String phase;
  final String action;
  final String riskLevel;
  final String suggestedWeight;
  final String mainTheme;
  final String updateTime;
  final bool isTrading;
  final String explanation;
  final List<ShortTermEmotionMetric> dashboard;
  final List<ShortTermEmotionComponent> components;
  final List<ShortTermEmotionSignal> riskSignals;
  final List<ShortTermEmotionEvent> intradayEvents;
  final List<ShortTermEmotionTrendPoint> intradayTrend;
}

class ShortTermEmotionMetric {
  const ShortTermEmotionMetric({
    required this.name,
    required this.value,
    required this.note,
    required this.tone,
  });

  factory ShortTermEmotionMetric.fromJson(Map<String, dynamic> json) {
    return ShortTermEmotionMetric(
      name: json['name'] as String? ?? '',
      value: json['value'] as String? ?? '',
      note: json['note'] as String? ?? '',
      tone: json['tone'] as String? ?? 'default',
    );
  }

  final String name;
  final String value;
  final String note;
  final String tone;
}

class ShortTermEmotionComponent {
  const ShortTermEmotionComponent({
    required this.name,
    required this.score,
    required this.weight,
    required this.note,
  });

  factory ShortTermEmotionComponent.fromJson(Map<String, dynamic> json) {
    return ShortTermEmotionComponent(
      name: json['name'] as String? ?? '',
      score: (json['score'] as num?)?.toInt() ?? 0,
      weight: (json['weight'] as num?)?.toInt() ?? 0,
      note: json['note'] as String? ?? '',
    );
  }

  final String name;
  final int score;
  final int weight;
  final String note;
}

class ShortTermEmotionSignal {
  const ShortTermEmotionSignal({
    required this.name,
    required this.level,
    required this.note,
    required this.tone,
  });

  factory ShortTermEmotionSignal.fromJson(Map<String, dynamic> json) {
    return ShortTermEmotionSignal(
      name: json['name'] as String? ?? '',
      level: json['level'] as String? ?? '',
      note: json['note'] as String? ?? '',
      tone: json['tone'] as String? ?? 'default',
    );
  }

  final String name;
  final String level;
  final String note;
  final String tone;
}

class ShortTermEmotionEvent {
  const ShortTermEmotionEvent({
    required this.time,
    required this.title,
    required this.level,
  });

  factory ShortTermEmotionEvent.fromJson(Map<String, dynamic> json) {
    return ShortTermEmotionEvent(
      time: json['time'] as String? ?? '',
      title: json['title'] as String? ?? '',
      level: json['level'] as String? ?? 'info',
    );
  }

  final String time;
  final String title;
  final String level;
}

class ShortTermEmotionTrendPoint {
  const ShortTermEmotionTrendPoint({
    required this.time,
    required this.upCount,
    required this.downCount,
    required this.redRate,
    required this.emotionIndex,
    required this.limitRatio,
    required this.limitUp,
    required this.limitDown,
  });

  factory ShortTermEmotionTrendPoint.fromJson(Map<String, dynamic> json) {
    return ShortTermEmotionTrendPoint(
      time: json['time'] as String? ?? '',
      upCount: (json['upCount'] as num?)?.toInt() ?? 0,
      downCount: (json['downCount'] as num?)?.toInt() ?? 0,
      redRate: (json['redRate'] as num?)?.toDouble() ?? 0,
      emotionIndex: (json['emotionIndex'] as num?)?.toDouble() ?? 0,
      limitRatio: (json['limitRatio'] as num?)?.toDouble() ?? 0,
      limitUp: (json['limitUp'] as num?)?.toInt() ?? 0,
      limitDown: (json['limitDown'] as num?)?.toInt() ?? 0,
    );
  }

  final String time;
  final int upCount;
  final int downCount;
  final double redRate;
  final double emotionIndex;
  final double limitRatio;
  final int limitUp;
  final int limitDown;
}

List<T> _parseList<T>(Object? value, T Function(Map<String, dynamic>) mapper) {
  if (value is! List) return const [];
  return value
      .whereType<Map>()
      .map((item) => mapper(Map<String, dynamic>.from(item)))
      .toList();
}
