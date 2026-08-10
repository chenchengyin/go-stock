import json
import sys
import tempfile
import unittest
from pathlib import Path

sys.path.insert(0, str(Path(__file__).resolve().parent))

import short_term_selection as selection


class ShortTermSelectionTest(unittest.TestCase):
    def test_build_selection_filters_scores_and_ranks_candidates(self):
        raw_stocks = [
            {
                "code": "600001",
                "name": "强势前排",
                "theme": "机器人",
                "theme_strength": 86,
                "auction_change_pct": 1.8,
                "had_limit_up_7d": True,
                "previous_amount": 980000000,
                "float_market_value": 18000000000,
                "is_main_board": True,
                "is_st": False,
                "relative_strength": 91,
                "intraday_support": 80,
                "limit_up_days": 2,
                "risk_flags": [],
            },
            {
                "code": "300001",
                "name": "非主板票",
                "theme": "机器人",
                "theme_strength": 88,
                "auction_change_pct": 1.2,
                "had_limit_up_7d": True,
                "previous_amount": 1200000000,
                "float_market_value": 20000000000,
                "is_main_board": False,
                "is_st": False,
                "relative_strength": 88,
                "intraday_support": 70,
                "limit_up_days": 1,
                "risk_flags": [],
            },
            {
                "code": "600002",
                "name": "竞价过高",
                "theme": "算力",
                "theme_strength": 70,
                "auction_change_pct": 6.2,
                "had_limit_up_7d": True,
                "previous_amount": 900000000,
                "float_market_value": 12000000000,
                "is_main_board": True,
                "is_st": False,
                "relative_strength": 75,
                "intraday_support": 65,
                "limit_up_days": 1,
                "risk_flags": [],
            },
            {
                "code": "600003",
                "name": "弱但合格",
                "theme": "通信",
                "theme_strength": 64,
                "auction_change_pct": 0.8,
                "had_limit_up_7d": True,
                "previous_amount": 650000000,
                "float_market_value": 10000000000,
                "is_main_board": True,
                "is_st": False,
                "relative_strength": 55,
                "intraday_support": 58,
                "limit_up_days": 0,
                "risk_flags": ["题材不是前排"],
            },
        ]

        result = selection.build_selection(raw_stocks, emotion_score=58)

        self.assertEqual(result["source"], "sample")
        self.assertEqual(result["strategy"], "超短避坑候选池")
        self.assertEqual([item["code"] for item in result["items"]], ["600001", "600003"])
        self.assertGreater(result["items"][0]["score"], result["items"][1]["score"])
        self.assertEqual(result["items"][0]["status"], "重点盯盘")
        self.assertTrue(
            any("市场情绪偏谨慎" in risk for risk in result["items"][0]["risks"])
        )
        self.assertIn("非主板", result["excluded"][0]["reasons"])
        self.assertIn("竞价涨幅超过3%", result["excluded"][1]["reasons"])

    def test_write_output_creates_json_file(self):
        result = selection.build_selection(selection.sample_stocks(), emotion_score=62)

        with tempfile.TemporaryDirectory() as tmpdir:
            output = Path(tmpdir) / "selected_stocks.json"
            selection.write_output(result, output)

            payload = json.loads(output.read_text(encoding="utf-8"))

        self.assertEqual(payload["strategy"], "超短避坑候选池")
        self.assertTrue(payload["items"])
        self.assertIn("updatedAt", payload)


if __name__ == "__main__":
    unittest.main()
