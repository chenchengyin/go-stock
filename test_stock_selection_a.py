#!/usr/bin/env python3
# -*- coding: utf-8 -*-
"""
方案A测试：东方财富智能选股接口
查询条件：当前涨幅在0.01%到3%之间
需要先在项目设置里配置好东财 qgqp_b_id
"""

import json
import ssl
import time
import urllib.request
import urllib.parse


def load_qgqp_b_id(db_path: str = "data/stock.db") -> str | None:
    """从项目数据库读取 qgqp_b_id"""
    try:
        import sqlite3
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

    # macOS 测试环境可能出现 SSL 证书验证失败，测试脚本里临时跳过
    ctx = ssl.create_default_context()
    ctx.check_hostname = False
    ctx.verify_mode = ssl.CERT_NONE

    with urllib.request.urlopen(req, timeout=30, context=ctx) as resp:
        return json.loads(resp.read().decode("utf-8"))


def main():
    qgqp_b_id = load_qgqp_b_id()
    if not qgqp_b_id:
        qgqp_b_id = input("请输入东财 qgqp_b_id（从浏览器 Cookie 中复制）: ").strip()
    if not qgqp_b_id:
        print("缺少 qgqp_b_id，无法继续")
        return

    query = "当前涨幅在0.01%到3%之间"
    print(f"\n查询条件: {query}")
    print(f"qgqp_b_id: {qgqp_b_id[:8]}...")

    try:
        res = search_stock(query, qgqp_b_id, page_size=50)
    except Exception as e:
        print(f"请求失败: {e}")
        return

    print(f"\n原始响应摘要:")
    print(json.dumps({k: v for k, v in res.items() if k != "data"}, ensure_ascii=False, indent=2))

    code = res.get("code")
    # 东财接口 code=100 表示解析成功并查询到数据
    if code is not None and code not in (0, "0", 100, "100"):
        print(f"\n接口返回错误: code={code}, message={res.get('message')}")
        return

    data = res.get("data", {})
    result = data.get("result", {})
    data_list = result.get("dataList", [])
    print(f"\n命中股票数量: {len(data_list)}")

    # 打印第一条数据的所有字段，方便确认涨幅字段名
    if data_list:
        print(f"\n第一条数据字段:\n{json.dumps(data_list[0], ensure_ascii=False, indent=2)}")

    for i, item in enumerate(data_list[:10], 1):
        code = item.get("SECURITY_CODE", "-")
        name = item.get("SECURITY_SHORT_NAME") or item.get("SECURITY_NAME_ABBR", "-")
        # CHG 看起来是涨跌幅(%)，PCHG 是另一个口径的涨跌幅
        chg = item.get("CHG", "-")
        pchg = item.get("PCHG", "-")
        price = item.get("NEWEST_PRICE", "-")
        print(f"第{i:02d}只: {code} {name:8s} 最新价:{price:>8s} CHG:{chg:>6s}% PCHG:{pchg:>6s}%")


if __name__ == "__main__":
    main()
