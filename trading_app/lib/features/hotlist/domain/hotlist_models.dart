class HotStock {
  const HotStock({
    required this.rank,
    required this.symbol,
    required this.name,
    required this.heatScore,
    required this.changePercent,
    required this.reason,
  });

  final int rank;
  final String symbol;
  final String name;
  final double heatScore;
  final double changePercent;
  final String reason;
}

