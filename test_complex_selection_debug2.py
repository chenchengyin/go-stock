#!/usr/bin/env python3
# -*- coding: utf-8 -*-
"""
测试'前20天至少有一天涨幅大于9.8%'的替代说法
"""

import json
import ssl
import sqlite3
import time
import urllib.request


def load_qgqp_b_id(db_path: str = "data/stock.db") -> str | None:
    try:
        conn = sqlite3.connect(db_path)
        cur = conn.cursor()
        cur.execute("SELECT qgqp_b_id FROM settings WHERE qgqp_b_id IS NOT NULL AND qgqp_b_id != '' LIMIT 1")
        row = cur.fetchone()
        conn.close()
        if row and row[0]:
            return row[0]
    except Exception as e:
        print(f"读取数据库失败: {e}")
    return None


def search_stock(query: str, qgqp_b_id: str, page_size: int = 50):
    url = "https://np-tjxg-g.eastmoney.com/api/smart-tag/stock/v3/pw/search-code"
    body = json.dumps({
        "keyWord": query,
        "pageSize": page_size,
        "pageNo": 1,
        "fingerprint": qgqp_b_id,
        "gids": [],
        "matchWord": "",
        "timestamp": str(int(time.time())),
        "shareToGuba": False,
        "requestId": "",
        "needCorrect": True,
        "removedConditionIdList": [],
        "xcId": "",
        "ownSelectAll": False,
        "dxInfo": [],
        "extraCondition": ""
    }, ensure_ascii=False).encode("utf-8")

    req = urllib.request.Request(
        url,
        data=body,
        headers={
            "Host": "np-tjxg-g.eastmoney.com",
            "Origin": "https://xuangu.eastmoney.com",
            "Referer": "https://xuangu.eastmoney.com/",
            "User-Agent": "Mozilla/5.0 (Windows NT 10.0; Win64; x64; rv:145.0) Gecko/20100101 Firefox/145.0",
            "Content-Type": "application/json",
            "Accept": "application/json, text/plain, */*",
        },
        method="POST",
    )

    ctx = ssl.create_default_context()
    ctx.check_hostname = False
    ctx.verify_mode = ssl.CERT_NONE

    with urllib.request.urlopen(req, timeout=30, context=ctx) as resp:
        return json.loads(resp.read().decode("utf-8"))


def test_query(name: str, query: str, qgqp_b_id: str):
    try:
        res = search_stock(query, qgqp_b_id, page_size=50)
    except Exception as e:
        print(f"[{name}] 请求失败: {e}")
        return

    code = res.get("code")
    msg = res.get("msg", "")
    data = res.get("data", {})
    result = data.get("result", {})
    data_list = result.get("dataList", [])
    print(f"[{name}] code={code} 命中={len(data_list):3d} msg={msg}")
    if code not in (0, "0", 100, "100"):
        print(f"  query: {query}")


def main():
    qgqp_b_id = load_qgqp_b_id()
    if not qgqp_b_id:
        print("缺少 qgqp_b_id")
        return

    cases = [
        ("近20日有涨停", "近20日有涨停;主板;非ST"),
        ("近20个交易日有涨停", "近20个交易日有涨停;主板;非ST"),
        ("20日内有涨停板", "20日内有涨停板;主板;非ST"),
        ("近20日涨幅大于9.8%", "近20日涨幅大于9.8%;主板;非ST"),
        ("近20日最大涨幅大于9.8%", "近20日最大涨幅大于9.8%;主板;非ST"),
        ("20日涨幅大于0", "20日涨幅大于0;主板;非ST"),
        ("近一个月有涨停", "近一个月有涨停;主板;非ST"),
        ("近20日曾涨停", "近20日曾涨停;主板;非ST"),
        ("近20日出现过涨停", "近20日出现过涨停;主板;非ST"),
        ("20日最大涨幅", "20日最大涨幅;主板;非ST"),
        ("近20日涨幅", "近20日涨幅;主板;非ST"),
    ]

    for name, query in cases:
        test_query(name, query, qgqp_b_id)


if __name__ == "__main__":
    main()
