#!/usr/bin/env python3
# -*- coding: utf-8 -*-
"""
涨速榜测试 demo
从东方财富获取沪深A股按涨速排序的股票列表
"""

import requests


def get_speed_rank(top_n: int = 50) -> list:
    """获取沪深A股涨速榜"""
    url = "https://push2.eastmoney.com/api/qt/clist/get"
    params = {
        "pn": 1,          # 页码
        "pz": top_n,      # 每页条数
        "po": 1,          # 1=降序
        "np": 1,
        "fltt": 2,
        "invt": 2,
        "fid": "f22",     # 按涨速排序
        "fs": "m:0+t:6,m:0+t:13,m:1+t:2,m:1+t:23",  # 沪深A股
        "fields": "f12,f13,f14,f2,f3,f22,f5,f6,f7,f8,f10,f17,f18,f20,f21"
    }
    headers = {
        "User-Agent": "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36",
        "Referer": "https://quote.eastmoney.com/"
    }

    resp = requests.get(
        url, params=params, headers=headers, timeout=15,
        proxies={"http": None, "https": None}  # 避免系统代理干扰
    )
    resp.raise_for_status()
    data = resp.json()

    stocks = data.get("data", {}).get("diff", [])
    result = []
    for item in stocks:
        result.append({
            "代码": item.get("f12"),
            "市场": "SH" if item.get("f13") == 1 else "SZ",
            "名称": item.get("f14"),
            "最新价": item.get("f2"),
            "涨跌幅": item.get("f3"),
            "涨速": item.get("f22"),
            "今开": item.get("f17"),
            "昨收": item.get("f18"),
            "换手率": item.get("f8"),
            "成交额": item.get("f6"),
            "流通市值": item.get("f21"),
        })
    return result


def format_amount(value):
    """格式化成交额/市值（接口返回单位为元）"""
    if value is None:
        return "-"
    if value >= 1e8:
        return f"{value / 1e8:.2f}亿"
    return f"{value / 1e4:.0f}万"


if __name__ == "__main__":
    stocks = get_speed_rank(50)
    print(f"沪深A股涨速榜 TOP {len(stocks)}")
    print("-" * 100)
    print(f"{'排名':<4}{'代码':<8}{'名称':<10}{'最新价':<10}{'涨跌幅':<10}{'涨速':<10}{'换手':<10}{'成交额':<12}{'流通市值':<12}")
    print("-" * 100)

    for i, s in enumerate(stocks, 1):
        print(
            f"{i:<4}"
            f"{s['代码']:<8}"
            f"{s['名称']:<10}"
            f"{s['最新价']:<10.2f}"
            f"{s['涨跌幅']:<10.2f}"
            f"{s['涨速']:<10.2f}"
            f"{s['换手率']:<10.2f}"
            f"{format_amount(s['成交额']):<12}"
            f"{format_amount(s['流通市值']):<12}"
        )
