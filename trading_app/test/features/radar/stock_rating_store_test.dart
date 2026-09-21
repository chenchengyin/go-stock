import 'package:flutter_test/flutter_test.dart';
import 'package:shared_preferences/shared_preferences.dart';
import 'package:trading_app/features/radar/presentation/radar_list/stock_rating_store.dart';

void main() {
  setUp(() {
    SharedPreferences.setMockInitialValues(<String, Object>{});
  });

  test('股票评级按纯数字代码持久化并可清除', () async {
    final store = StockRatingStore();

    await store.save('600519.XSHG', StockRating.heavy);
    expect(await store.loadAll(), {'600519': StockRating.heavy});

    await store.save('600519.XSHG', StockRating.unrated);
    expect(await store.loadAll(), isEmpty);
  });

  test('读取时忽略无效评级值', () async {
    SharedPreferences.setMockInitialValues({
      StockRatingStore.storageKey: '{"600519":"light","000001":"unknown"}',
    });

    expect(await StockRatingStore().loadAll(), {'600519': StockRating.light});
  });

  group('自动评级', () {
    test('按真赚率边界计算重仓轻仓和不买', () {
      expect(
        calculateStockRating(hasStats: true, earnPct: 75),
        StockRating.heavy,
      );
      expect(
        calculateStockRating(hasStats: true, earnPct: 74.99),
        StockRating.light,
      );
      expect(
        calculateStockRating(hasStats: true, earnPct: 55),
        StockRating.light,
      );
      expect(
        calculateStockRating(hasStats: true, earnPct: 54.99),
        StockRating.avoid,
      );
    });

    test('没有有效形态统计时保持未评级', () {
      expect(
        calculateStockRating(hasStats: false, earnPct: 100),
        StockRating.unrated,
      );
    });
  });
}
