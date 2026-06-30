/// 异动记录模型
class StockChange {
  const StockChange({
    required this.id,
    required this.changeTime,
    required this.changeDate,
    required this.stockCode,
    required this.stockName,
    required this.changeType,
    required this.typeName,
    required this.price,
    required this.changeRate,
    required this.volume,
    required this.amount,
  });

  factory StockChange.fromJson(Map<String, dynamic> json) {
    return StockChange(
      id: json['id'] as int? ?? 0,
      changeTime: json['changeTime'] as String? ?? '',
      changeDate: json['changeDate'] as String? ?? '',
      stockCode: json['stockCode'] as String? ?? '',
      stockName: json['stockName'] as String? ?? '',
      changeType: json['changeType'] as int? ?? 0,
      typeName: json['typeName'] as String? ?? '',
      price: (json['price'] as num?)?.toDouble() ?? 0.0,
      changeRate: (json['changeRate'] as num?)?.toDouble() ?? 0.0,
      volume: (json['volume'] as num?)?.toInt() ?? 0,
      amount: (json['amount'] as num?)?.toDouble() ?? 0.0,
    );
  }

  final int id;
  final String changeTime;
  final String changeDate;
  final String stockCode;
  final String stockName;
  final int changeType;
  final String typeName;
  final double price;
  final double changeRate;
  final int volume;
  final double amount;
}

/// 监控股票配置（含实时行情）
class MonitoredStock {
  const MonitoredStock({
    required this.code,
    required this.name,
    this.price = 0,
    this.changePercent = 0,
    this.volume = 0,
    this.amount = 0,
    this.open = 0,
    this.preClose = 0,
    this.high = 0,
    this.low = 0,
    this.changeTypes = '',
    this.createdAt,
  });

  factory MonitoredStock.fromJson(Map<String, dynamic> json) {
    return MonitoredStock(
      code: json['code'] as String? ?? '',
      name: json['name'] as String? ?? '',
      changeTypes: json['changeTypes'] as String? ?? '',
      price: (json['price'] as num?)?.toDouble() ?? 0.0,
      changePercent: (json['changePercent'] as num?)?.toDouble() ?? 0.0,
      volume: (json['volume'] as num?)?.toInt() ?? 0,
      amount: (json['amount'] as num?)?.toDouble() ?? 0.0,
      open: (json['open'] as num?)?.toDouble() ?? 0.0,
      preClose: (json['preClose'] as num?)?.toDouble() ?? 0.0,
      high: (json['high'] as num?)?.toDouble() ?? 0.0,
      low: (json['low'] as num?)?.toDouble() ?? 0.0,
      createdAt: json['createdAt'] as String?,
    );
  }

  Map<String, dynamic> toJson() => {
        'code': code,
        'name': name,
        'changeTypes': changeTypes,
        'price': price,
        'changePercent': changePercent,
        'volume': volume,
        'amount': amount,
        'open': open,
        'preClose': preClose,
        'high': high,
        'low': low,
        'createdAt': createdAt,
      };

  final String code;
  final String name;
  final double price;
  final double changePercent;
  final int volume;
  final double amount;
  final double open;
  final double preClose;
  final double high;
  final double low;
  final String changeTypes;
  final String? createdAt;

  List<int> get changeTypeCodes =>
      changeTypes.split(',').where((e) => e.isNotEmpty).map(int.parse).toList();
}
