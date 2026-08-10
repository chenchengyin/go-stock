#!/usr/bin/env python3
# -*- coding: utf-8 -*-
"""
复杂选股条件拆分测试，定位哪个子条件导致东财返回 201
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
        ("基础", "主板;非ST"),
        ("竞价涨幅", "竞价涨幅在0.01%到3%之间;主板;非ST"),
        ("前1天涨跌停", "前一天曾涨停或者跌停;主板;非ST"),
        ("前1天涨停", "前一天涨停;主板;非ST"),
        ("前1天跌停", "前一天跌停;主板;非ST"),
        ("前20天大涨", "前20天至少有一天涨幅大于9.8%;主板;非ST"),
        ("前1天成交", "前一天成交金额大于5亿;主板;非ST"),
        ("20日线上方", "收盘价在20日线上方;主板;非ST"),
        ("流通市值", "流通市值在30亿到8000亿之间;主板;非ST"),
        ("竞价+前1天涨停", "竞价涨幅在0.01%到3%之间;前一天涨停;主板;非ST"),
        ("竞价+前1天涨跌停", "竞价涨幅在0.01%到3%之间;前一天曾涨停或者跌停;主板;非ST"),
        ("竞价+前20天", "竞价涨幅在0.01%到3%之间;前20天至少有一天涨幅大于9.8%;主板;非ST"),
        ("竞价+成交", "竞价涨幅在0.01%到3%之间;前一天成交金额大于5亿;主板;非ST"),
        ("竞价+20日线", "竞价涨幅在0.01%到3%之间;收盘价在20日线上方;主板;非ST"),
        ("竞价+市值", "竞价涨幅在0.01%到3%之间;流通市值在30亿到8000亿之间;主板;非ST"),
        ("完整OR简化", "竞价涨幅在0.01%到3%之间;前一天涨停;前20天至少有一天涨幅大于9.8%;前一天成交金额大于5亿;收盘价在20日线上方;流通市值在30亿到8000亿之间;主板;非ST"),
        ("完整OR保留", "竞价涨幅在0.01%到3%之间;前一天曾涨停或者跌停;前20天至少有一天涨幅大于9.8%;前一天成交金额大于5亿;收盘价在20日线上方;流通市值在30亿到8000亿之间;主板;非ST"),
    ]

    for name, query in cases:
        test_query(name, query, qgqp_b_id)


if __name__ == "__main__":
    main()
