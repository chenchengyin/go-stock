#!/usr/bin/env python3
# -*- coding: utf-8 -*-
"""
研究同花顺问财 Cookie 获取可行性
测试：不登录情况下，访问 iwencai.com 能拿到哪些 Cookie，能否调用选股接口
"""

import json
import ssl
import urllib.parse
import urllib.request


def make_ssl_ctx():
    ctx = ssl.create_default_context()
    ctx.check_hostname = False
    ctx.verify_mode = ssl.CERT_NONE
    return ctx


def fetch_homepage():
    """访问问财首页，获取服务端返回的 Cookie"""
    url = "https://www.iwencai.com/"
    req = urllib.request.Request(
        url,
        headers={
            "User-Agent": "Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/120.0.0.0 Safari/537.36",
            "Accept": "text/html,application/xhtml+xml,application/xml;q=0.9,image/webp,*/*;q=0.8",
            "Accept-Language": "zh-CN,zh;q=0.9",
        },
    )

    cookies = {}
    with urllib.request.urlopen(req, timeout=30, context=make_ssl_ctx()) as resp:
        headers = resp.headers
        # 解析 Set-Cookie
        for raw in headers.get_all("Set-Cookie") or []:
            key = raw.split(";")[0].split("=")[0].strip()
            val = raw.split(";")[0].split("=", 1)[1].strip() if "=" in raw else ""
            cookies[key] = val
        html = resp.read().decode("utf-8", errors="ignore")

    return cookies, html


def try_search(cookie_str: str, method: str = "POST"):
    """尝试用给定 Cookie 调用问财搜索接口"""
    if method == "GET":
        params = urllib.parse.urlencode({
            "query": "竞价涨幅在0.01%到3%之间",
            "typed": 1,
            "header": "港股",
            "catalogId": "",
            "searchType": "stock",
            "extflag": "",
            "isRead": 1,
        })
        url = f"https://www.iwencai.com/stockpick/search?{params}"
        req = urllib.request.Request(
            url,
            headers={
                "User-Agent": "Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/120.0.0.0 Safari/537.36",
                "Referer": "https://www.iwencai.com/",
                "Accept": "application/json, text/javascript, */*; q=0.01",
                "Accept-Language": "zh-CN,zh;q=0.9,en;q=0.8",
                "Cookie": cookie_str,
            },
            method="GET",
        )
    else:
        url = "https://www.iwencai.com/stockpick/search"
        body = json.dumps({
            "query": "竞价涨幅在0.01%到3%之间",
            "typed": 1,
            "header": "港股",
            "catalogId": "",
            "searchType": "stock",
            "extflag": "",
            "isRead": 1,
        }, ensure_ascii=False).encode("utf-8")
        req = urllib.request.Request(
            url,
            data=body,
            headers={
                "User-Agent": "Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/120.0.0.0 Safari/537.36",
                "Content-Type": "application/json",
                "Referer": "https://www.iwencai.com/",
                "Accept": "application/json, text/javascript, */*; q=0.01",
                "Accept-Language": "zh-CN,zh;q=0.9,en;q=0.8",
                "Cookie": cookie_str,
            },
            method="POST",
        )

    try:
        with urllib.request.urlopen(req, timeout=30, context=make_ssl_ctx()) as resp:
            return resp.status, resp.read().decode("utf-8", errors="ignore")
    except urllib.error.HTTPError as e:
        return e.code, e.read().decode("utf-8", errors="ignore")
    except Exception as e:
        return -1, str(e)


def main():
    print("=" * 60)
    print("步骤1: 访问问财首页获取初始 Cookie")
    print("=" * 60)
    cookies, html = fetch_homepage()
    print(f"首页返回 Cookie 字段数: {len(cookies)}")
    for k, v in cookies.items():
        print(f"  {k}={v[:60]}{'...' if len(v) > 60 else ''}")

    print("\n" + "=" * 60)
    print("步骤2: 不带 Cookie 调用搜索接口")
    print("=" * 60)
    status, text = try_search("")
    print(f"HTTP 状态: {status}")
    print(f"响应前 500 字: {text[:500]}")

    print("\n" + "=" * 60)
    print("步骤3: POST 方式调用搜索接口（带首页 Cookie）")
    print("=" * 60)
    cookie_str = "; ".join(f"{k}={v}" for k, v in cookies.items())
    status, text = try_search(cookie_str, method="POST")
    print(f"HTTP 状态: {status}")
    print(f"响应前 1000 字: {text[:1000]}")

    print("\n" + "=" * 60)
    print("步骤4: GET 方式调用搜索接口（带首页 Cookie）")
    print("=" * 60)
    status, text = try_search(cookie_str, method="GET")
    print(f"HTTP 状态: {status}")
    print(f"响应前 1000 字: {text[:1000]}")

    print("\n" + "=" * 60)
    print("可行性结论")
    print("=" * 60)
    has_data = status == 200 and len(text) > 200 and '"status_code":0' in text
    if has_data:
        print("结论: 通过访问首页获取 Cookie 即可调用问财搜索接口，WebView 方案可行。")
    else:
        print("结论: 仅通过访问首页获取的 Cookie 不足以调用问财搜索接口。")
        print("      通常需要用户登录同花顺账号后，WebView 才能拿到有效的认证 Cookie。")
        print("      关键字段可能包括 'v' (hexin-v)，该字段常在登录后或 JS 执行后生成。")
        print("      另外问财搜索接口可能是 GET 而非 POST，后端代码也需要调整。")


if __name__ == "__main__":
    main()
