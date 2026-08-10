# 测试复杂选股条件 + 研究问财 Cookie 获取方案

## Summary

用户有两个诉求：
1. 用一组复杂条件筛选股票，先通过脚本验证东财接口是否支持这些条件、能命中多少只。
2. 研究是否可以通过 WebView 自动获取同花顺问财（iwencai.com）的 Cookie，让 `/api/ths-selection-test` 从不可用变成可用。

本计划先做脚本验证，再研究问财 Cookie 获取可行性。

## Current State Analysis

### 已有选股能力

1. **东方财富选股接口**（可用）
   - 后端：[backend/flutter_api/stock_selection_handler.go](file:///Users/Zhuanz/aiproject/go-stock/backend/flutter_api/stock_selection_handler.go)
   - 数据源：[backend/data/search_stock_api.go](file:///Users/Zhuanz/aiproject/go-stock/backend/data/search_stock_api.go)
   - 接口：`GET /api/stock-selection-test?query=...`
   - 依赖：数据库 `settings.qgqp_b_id`（项目里已有）
   - 支持自然语言组合条件，用 `;` 分隔

2. **同花顺问财接口**（当前不可用）
   - 后端：[backend/flutter_api/ths_selection_handler.go](file:///Users/Zhuanz/aiproject/go-stock/backend/flutter_api/ths_selection_handler.go)
   - 接口：`GET /api/ths-selection-test?query=...&cookie=...`
   - 当前 `getTHSCookie()` 返回空字符串，导致请求不带 Cookie
   - 问财接口需要有效的 Cookie 才能返回股票数据

### 用户选股条件

```
竞价涨幅在0.01%到3%之间
前一天曾涨停或者跌停
前20天至少有一天涨幅大于9.8%
前一天成交金额大于5亿
收盘价在20日线上方
流通市值在30亿到8000亿之间
主板
非ST
```

## Proposed Changes

### 任务 1：编写复杂条件测试脚本

新增脚本：`test_complex_selection.py`

位置：`/Users/Zhuanz/aiproject/go-stock/test_complex_selection.py`

内容：
- 自动从 `data/stock.db` 读取 `qgqp_b_id`
- 把用户条件组合成东财自然语言 query：
  ```
  竞价涨幅在0.01%到3%之间;前一天曾涨停或者跌停;前20天至少有一天涨幅大于9.8%;前一天成交金额大于5亿;收盘价在20日线上方;流通市值在30亿到8000亿之间;主板;非ST
  ```
- 调用东财接口 `https://np-tjxg-g.eastmoney.com/api/smart-tag/stock/v3/pw/search-code`
- 解析响应：
  - 如果返回 `code: 100`，打印命中数量、前 N 只股票代码/名称/关键字段
  - 如果返回错误，打印错误码和 msg
- 打印第一条完整字段，确认东财返回了哪些可用字段

### 任务 2：研究问财 Cookie 获取可行性

新增临时研究脚本：`research_iwencai_cookie.py`

位置：`/Users/Zhuanz/aiproject/go-stock/research_iwencai_cookie.py`

研究步骤：
1. 用 Python `requests` 或 `urllib` 访问 `https://www.iwencai.com/stockpick/search`
2. 不登录情况下检查服务端返回的 Set-Cookie，记录关键字段名（如 `v`、`cid`、`ComputerId`、`WafStatus` 等）
3. 尝试不带 Cookie / 只带部分 Cookie 访问选股接口，观察返回结果
4. 如果无登录 Cookie 无法返回数据，尝试：
   - 访问问财首页获取初始 Cookie
   - 模拟一次简单搜索，看是否会设置额外 Cookie
5. 输出结论：
   - 需要哪些 Cookie
   - 是否必须登录
   - WebView 是否能自动获取到足够 Cookie
   - 与同花顺账号登录的关系

### 任务 3：根据研究结果更新结论

- 如果问财 Cookie 可以在不登录情况下通过 WebView 自动获取，则制定后续集成方案。
- 如果需要登录，评估实现成本和稳定性。
- 如果东财接口已经能满足复杂条件，优先使用东财方案。

## Assumptions & Decisions

- 先用东财接口测试复杂条件，因为东财接口已经可用且不需要登录 Cookie。
- 同花顺问财 Cookie 研究只做可行性分析，不直接修改生产代码。
- 测试脚本与项目代码解耦，使用 Python 直接调用接口，避免 Go 项目编译问题（macOS 上 `go-toast/toast` 导致 `go test` 失败）。

## Verification Steps

1. 运行 `python3 test_complex_selection.py`，确认接口返回成功并打印命中股票。
2. 检查返回股票数量是否在合理范围内（几十到几百只）。
3. 检查关键字段（SECURITY_CODE、SECURITY_SHORT_NAME、CHG、NEWEST_PRICE 等）是否正常。
4. 运行 `python3 research_iwencai_cookie.py`，记录问财 Cookie 获取结论。
5. 汇总两个结果，向用户报告：
   - 复杂条件在东财能选出多少只
   - 问财 Cookie 是否能通过 WebView 获取
