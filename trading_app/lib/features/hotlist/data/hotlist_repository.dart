import '../../../core/storage/local_cache.dart';
import '../domain/hotlist_models.dart';

abstract class HotlistRepository {
  Future<List<HotStock>> load();
}

class MockHotlistRepository implements HotlistRepository {
  MockHotlistRepository(this._cache);

  final LocalCache _cache;

  @override
  Future<List<HotStock>> load() async {
    await _cache.setString('hotlist:last_load', DateTime.now().toIso8601String());
    return const [
      HotStock(rank: 1, symbol: '600522.SH', name: '中天科技', heatScore: 98.5, changePercent: 5.66, reason: '全网资讯与盘中异动同步升温'),
      HotStock(rank: 2, symbol: '002378.SZ', name: '章源钨业', heatScore: 91.2, changePercent: 1.28, reason: '有色题材讨论度提升'),
      HotStock(rank: 3, symbol: '301377.SZ', name: '鼎泰高科', heatScore: 88.6, changePercent: 17.45, reason: '高位趋势票关注度快速上升'),
    ];
  }
}

