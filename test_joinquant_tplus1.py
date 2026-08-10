import importlib.util
import sys
import types
import unittest
from pathlib import Path


def load_strategy():
    jqdata = types.ModuleType('jqdata')
    sys.modules['jqdata'] = jqdata
    path = Path(__file__).with_name('joinquant_5min_surge_backtest.py')
    spec = importlib.util.spec_from_file_location('joinquant_5min_surge_backtest', path)
    module = importlib.util.module_from_spec(spec)
    spec.loader.exec_module(module)
    return module


class JoinQuantTPlusOneTest(unittest.TestCase):
    def test_initialize_schedules_0930_open_trade_only(self):
        strategy = load_strategy()
        calls = []

        strategy.g = types.SimpleNamespace()
        strategy.log = types.SimpleNamespace(info=lambda *args, **kwargs: None)
        strategy.set_benchmark = lambda *args, **kwargs: None
        strategy.set_option = lambda *args, **kwargs: None
        strategy.set_order_cost = lambda *args, **kwargs: None
        strategy.OrderCost = lambda **kwargs: kwargs
        strategy.run_daily = lambda fn, time, reference_security=None: calls.append((fn.__name__, time))

        strategy.initialize(types.SimpleNamespace())

        self.assertIn(('market_open_trade', '09:30'), calls)
        self.assertNotIn(('market_open_sell', '09:31'), calls)
        self.assertNotIn(('market_open_buy', '09:35'), calls)

    def test_get_sellable_amount_prefers_closeable_amount_for_tplus1(self):
        strategy = load_strategy()

        old_position = types.SimpleNamespace(total_amount=1000, closeable_amount=600)
        today_position = types.SimpleNamespace(total_amount=1000, closeable_amount=0)
        fallback_position = types.SimpleNamespace(total_amount=800)

        self.assertEqual(strategy.get_sellable_amount(old_position), 600)
        self.assertEqual(strategy.get_sellable_amount(today_position), 0)
        self.assertEqual(strategy.get_sellable_amount(fallback_position), 800)

    def test_buy_value_is_five_percent_of_total_portfolio_value(self):
        strategy = load_strategy()
        strategy.g = types.SimpleNamespace(single_stock_position_pct=0.05)

        enough_cash = types.SimpleNamespace(
            portfolio=types.SimpleNamespace(total_value=1_000_000, available_cash=200_000)
        )
        limited_cash = types.SimpleNamespace(
            portfolio=types.SimpleNamespace(total_value=1_000_000, available_cash=30_000)
        )

        self.assertEqual(strategy.get_single_stock_buy_value(enough_cash), 50_000)
        self.assertEqual(strategy.get_single_stock_buy_value(limited_cash), 30_000)

    def test_tradable_filter_indexes_current_data_without_membership_check(self):
        strategy = load_strategy()

        class CurrentData:
            def __contains__(self, code):
                return False

            def __getitem__(self, code):
                return types.SimpleNamespace(paused=False, is_st=False, name='正常股票')

        strategy.get_current_data = lambda: CurrentData()

        self.assertEqual(strategy.filter_tradable_and_not_st(['600001.XSHG']), ['600001.XSHG'])


if __name__ == '__main__':
    unittest.main()
