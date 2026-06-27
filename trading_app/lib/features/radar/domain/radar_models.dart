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

/// 监控股票配置
class MonitoredStock {
  const MonitoredStock({
    required this.code,
    required this.name,
    this.changeTypes = '',
  });

  factory MonitoredStock.fromJson(Map<String, dynamic> json) {
    return MonitoredStock(
      code: json['code'] as String? ?? '',
      name: json['name'] as String? ?? '',
      changeTypes: json['changeTypes'] as String? ?? '',
    );
  }

  Map<String, dynamic> toJson() => {
        'code': code,
        'name': name,
        'changeTypes': changeTypes,
      };

  final String code;
  final String name;
  final String changeTypes;

  List<int> get changeTypeCodes =>
      changeTypes.split(',').where((e) => e.isNotEmpty).map(int.parse).toList();
}
