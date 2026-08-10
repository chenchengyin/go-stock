#!/usr/bin/env python3
"""超短避坑候选池生成脚本。

第一版只负责把外部/样例股票数据加工成稳定 JSON，方便后续 Go 接口和
Flutter 页面接入。它不直接给买卖建议，只输出候选、风险和淘汰原因。
"""

from __future__ import annotations

import argparse
import json
from datetime import datetime
from pathlib import Path
from typing import Any


DEFAULT_OUTPUT = Path("data/selected_stocks.json")


def sample_stocks() -> list[dict[str, Any]]:
    """离线样例，用来先验证选股流程和 App 接入结构。"""
    return [
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
            "code": "600171",
            "name": "上海贝岭",
            "theme": "半导体",
            "theme_strength": 78,
            "auction_change_pct": 2.4,
            "had_limit_up_7d": True,
            "previous_amount": 1320000000,
            "float_market_value": 33000000000,
            "is_main_board": True,
            "is_st": False,
            "relative_strength": 82,
            "intraday_support": 72,
            "limit_up_days": 1,
            "risk_flags": ["高位波动偏大"],
        },
        {
            "code": "600050",
            "name": "中国联通",
            "theme": "通信",
            "theme_strength": 66,
            "auction_change_pct": 0.6,
            "had_limit_up_7d": False,
            "previous_amount": 860000000,
            "float_market_value": 154000000000,
            "is_main_board": True,
            "is_st": False,
            "relative_strength": 60,
            "intraday_support": 62,
            "limit_up_days": 0,
            "risk_flags": ["弹性一般"],
        },
        {
            "code": "300001",
            "name": "非主板样例",
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
    ]


def build_selection(
    raw_stocks: list[dict[str, Any]],
    emotion_score: float,
    source: str = "sample",
    max_items: int = 20,
) -> dict[str, Any]:
    items: list[dict[str, Any]] = []
    excluded: list[dict[str, Any]] = []

    for stock in raw_stocks:
        reasons = exclusion_reasons(stock)
        if reasons:
            excluded.append(
                {
                    "code": str(stock.get("code", "")),
                    "name": str(stock.get("name", "")),
                    "reasons": reasons,
                }
            )
            continue

        score = candidate_score(stock, emotion_score)
        items.append(build_item(stock, score, emotion_score))

    items.sort(key=lambda item: item["score"], reverse=True)
    return {
        "strategy": "超短避坑候选池",
        "source": source,
        "updatedAt": datetime.now().astimezone().isoformat(timespec="seconds"),
        "emotionScore": round(float(emotion_score), 1),
        "summary": build_summary(items, excluded, emotion_score),
        "items": items[:max_items],
        "excluded": excluded,
    }


def exclusion_reasons(stock: dict[str, Any]) -> list[str]:
    reasons: list[str] = []
    if bool(stock.get("is_st")):
        reasons.append("ST或退市风险")
    if not bool(stock.get("is_main_board", True)):
        reasons.append("非主板")
    if float(stock.get("auction_change_pct", 0)) < 0.01:
        reasons.append("竞价涨幅低于0.01%")
    if float(stock.get("auction_change_pct", 0)) > 3:
        reasons.append("竞价涨幅超过3%")
    if not bool(stock.get("had_limit_up_7d")):
        reasons.append("近7日没有涨停记忆")
    if float(stock.get("previous_amount", 0)) < 500_000_000:
        reasons.append("昨日成交额低于5亿")

    float_mv = float(stock.get("float_market_value", 0))
    if float_mv < 6_000_000_000:
        reasons.append("流通市值低于60亿")
    if float_mv > 800_000_000_000:
        reasons.append("流通市值高于8000亿")
    return reasons


def candidate_score(stock: dict[str, Any], emotion_score: float) -> int:
    theme_strength = clamp(float(stock.get("theme_strength", 50)), 0, 100)
    relative_strength = clamp(float(stock.get("relative_strength", 50)), 0, 100)
    intraday_support = clamp(float(stock.get("intraday_support", 50)), 0, 100)
    liquidity_score = liquidity_to_score(float(stock.get("previous_amount", 0)))
    board_bonus = min(int(stock.get("limit_up_days", 0)) * 4, 10)
    risk_penalty = len(stock.get("risk_flags") or []) * 5

    score = (
        clamp(float(emotion_score), 0, 100) * 0.25
        + theme_strength * 0.25
        + relative_strength * 0.25
        + intraday_support * 0.15
        + liquidity_score * 0.10
        + board_bonus
        - risk_penalty
    )
    return int(round(clamp(score, 0, 100)))


def build_item(stock: dict[str, Any], score: int, emotion_score: float) -> dict[str, Any]:
    risks = list(stock.get("risk_flags") or [])
    if emotion_score < 60:
        risks.append("市场情绪偏谨慎，禁止追高")
    if float(stock.get("auction_change_pct", 0)) > 2.5:
        risks.append("竞价接近上限，只适合回踩确认")

    reasons = [
        f"题材强度 {int(stock.get('theme_strength', 0))}",
        f"个股主动性 {int(stock.get('relative_strength', 0))}",
        f"分时承接 {int(stock.get('intraday_support', 0))}",
        f"昨日成交额 {format_yuan(float(stock.get('previous_amount', 0)))}",
    ]
    if int(stock.get("limit_up_days", 0)) > 0:
        reasons.append(f"连板/涨停记忆 {int(stock.get('limit_up_days', 0))}")

    return {
        "code": str(stock.get("code", "")),
        "name": str(stock.get("name", "")),
        "theme": str(stock.get("theme", "")),
        "score": score,
        "status": score_to_status(score, emotion_score),
        "auctionChangePct": round(float(stock.get("auction_change_pct", 0)), 2),
        "previousAmount": float(stock.get("previous_amount", 0)),
        "floatMarketValue": float(stock.get("float_market_value", 0)),
        "reasons": reasons,
        "risks": risks,
    }


def build_summary(items: list[dict[str, Any]], excluded: list[dict[str, Any]], emotion_score: float) -> str:
    if emotion_score < 40:
        prefix = "情绪低位，候选仅作观察"
    elif emotion_score < 60:
        prefix = "情绪偏谨慎，只看前排确认"
    else:
        prefix = "情绪允许试错，仍需等待盘中触发"
    return f"{prefix}；入选 {len(items)} 只，剔除 {len(excluded)} 只。"


def score_to_status(score: int, emotion_score: float) -> str:
    if emotion_score < 45 and score < 85:
        return "只观察"
    if score >= 75:
        return "重点盯盘"
    if score >= 65:
        return "可观察"
    if score >= 50:
        return "弱关注"
    return "剔除"


def liquidity_to_score(amount: float) -> float:
    if amount >= 2_000_000_000:
        return 92
    if amount >= 1_000_000_000:
        return 84
    if amount >= 500_000_000:
        return 72
    if amount >= 200_000_000:
        return 55
    return 35


def load_input(path: Path) -> list[dict[str, Any]]:
    payload = json.loads(path.read_text(encoding="utf-8"))
    if isinstance(payload, list):
        return payload
    if isinstance(payload, dict) and isinstance(payload.get("items"), list):
        return payload["items"]
    raise ValueError("输入 JSON 必须是数组，或包含 items 数组的对象")


def write_output(result: dict[str, Any], output: Path) -> None:
    output.parent.mkdir(parents=True, exist_ok=True)
    output.write_text(json.dumps(result, ensure_ascii=False, indent=2) + "\n", encoding="utf-8")


def clamp(value: float, min_value: float, max_value: float) -> float:
    return max(min_value, min(max_value, value))


def format_yuan(value: float) -> str:
    if value >= 100_000_000:
        return f"{value / 100_000_000:.1f}亿"
    if value >= 10_000:
        return f"{value / 10_000:.0f}万"
    return f"{value:.0f}"


def parse_args() -> argparse.Namespace:
    parser = argparse.ArgumentParser(description="生成超短避坑候选股 JSON")
    parser.add_argument("--input", type=Path, help="外部股票数据 JSON，不传则使用样例数据")
    parser.add_argument("--output", type=Path, default=DEFAULT_OUTPUT, help="输出文件路径")
    parser.add_argument("--emotion-score", type=float, default=55, help="市场情绪分，默认 55")
    parser.add_argument("--max-items", type=int, default=20, help="最多输出候选数量")
    return parser.parse_args()


def main() -> None:
    args = parse_args()
    stocks = load_input(args.input) if args.input else sample_stocks()
    source = str(args.input) if args.input else "sample"
    result = build_selection(stocks, args.emotion_score, source=source, max_items=args.max_items)
    write_output(result, args.output)
    print(f"已生成 {args.output}，候选 {len(result['items'])} 只，剔除 {len(result['excluded'])} 只")


if __name__ == "__main__":
    main()
