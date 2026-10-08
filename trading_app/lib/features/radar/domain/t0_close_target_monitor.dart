class T0CloseTargetMonitor {
  const T0CloseTargetMonitor({
    required this.tradeDate,
    required this.stockCode,
    required this.stockName,
    required this.entryClose,
    required this.targetPrice,
    this.triggered = false,
  });

  factory T0CloseTargetMonitor.fromJson(Map<String, dynamic> json) {
    return T0CloseTargetMonitor(
      tradeDate: json['trade_date'] as String? ?? '',
      stockCode: json['stock_code'] as String? ?? '',
      stockName: json['stock_name'] as String? ?? '',
      entryClose: (json['entry_close'] as num?)?.toDouble() ?? 0,
      targetPrice: (json['target_price'] as num?)?.toDouble() ?? 0,
      triggered: json['triggered'] as bool? ?? false,
    );
  }

  final String tradeDate;
  final String stockCode;
  final String stockName;
  final double entryClose;
  final double targetPrice;
  final bool triggered;

  bool isReachedBy(double price) =>
      price > 0 && targetPrice > 0 && price + 1e-9 >= targetPrice;

  T0CloseTargetMonitor copyWith({bool? triggered}) => T0CloseTargetMonitor(
    tradeDate: tradeDate,
    stockCode: stockCode,
    stockName: stockName,
    entryClose: entryClose,
    targetPrice: targetPrice,
    triggered: triggered ?? this.triggered,
  );

  Map<String, dynamic> toJson() => {
    'trade_date': tradeDate,
    'stock_code': stockCode,
    'stock_name': stockName,
    'entry_close': entryClose,
    'target_price': targetPrice,
    'triggered': triggered,
  };
}
