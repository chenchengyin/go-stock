import '../../../core/storage/local_cache.dart';
import '../domain/radar_models.dart';

abstract class RadarRepository {
  Future<List<WatchStock>> loadWatchStocks();
}

class MockRadarRepository implements RadarRepository {
  MockRadarRepository(this._cache);

  final LocalCache _cache;

  @override
  Future<List<WatchStock>> loadWatchStocks() async {
    await _cache.setString('radar:last_load', DateTime.now().toIso8601String());
    return const [
      WatchStock(
        symbol: '600522.SH',
        name: '中天科技',
        price: 51.35,
        changePercent: 5.66,
        volumeRatio: 2.4,
        alerts: [
          AlertRule(type: AlertRuleType.priceChange, windowSeconds: 60, thresholdPercent: 1, enabled: true),
          AlertRule(type: AlertRuleType.waterfall, windowSeconds: 60, thresholdPercent: 1, enabled: true),
        ],
      ),
      WatchStock(
        symbol: '002378.SZ',
        name: '章源钨业',
        price: 37.88,
        changePercent: 1.28,
        volumeRatio: 1.9,
        alerts: [
          AlertRule(type: AlertRuleType.volumeSpike, windowSeconds: 60, thresholdPercent: 2, enabled: true),
        ],
      ),
    ];
  }
}

