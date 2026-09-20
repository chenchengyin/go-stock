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
}
