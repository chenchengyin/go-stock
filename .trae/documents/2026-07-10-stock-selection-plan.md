# 东财选股条件封装与测试计划

## Summary
将 `/api/stock-selection-test` 接口的默认选股条件改为用户指定策略：
- 竞价涨幅在 0.01% 到 3% 之间
- 近 7 日至少有一天涨幅大于 9.8%（表达为"近7日最大涨幅大于9.8%"，东财可识别）
- 前一天成交金额大于 5 亿
- 流通市值在 60 亿到 8000 亿之间
- 主板

并补充/更新 Go 单元测试，验证接口能正常返回符合条件的股票列表。

## Current State Analysis
- 东财选股底层调用已存在：[backend/data/search_stock_api.go](file:///Users/Zhuanz/aiproject/go-stock/backend/data/search_stock_api.go#L25-L86) 的 `SearchStockApi.SearchStock`
- HTTP 测试接口已存在：[backend/flutter_api/stock_selection_handler.go](file:///Users/Zhuanz/aiproject/go-stock/backend/flutter_api/stock_selection_handler.go#L16-L46) 的 `/api/stock-selection-test`
- 接口已支持通过 `query` 参数传入任意自然语言条件，默认 query 是旧条件
- 现有测试：[backend/data/search_stock_api_test.go](file:///Users/Zhuanz/aiproject/go-stock/backend/data/search_stock_api_test.go#L79-L118) 有 `TestSearchStock_CurrentChangeRange` 测试当前涨幅范围
- 用户后续会扩展为"一句话选股"通用接口，因此本次保持接口路径不变，只调整默认条件与测试

## Proposed Changes

### 1. 修改默认选股条件
**文件：** [backend/flutter_api/stock_selection_handler.go](file:///Users/Zhuanz/aiproject/go-stock/backend/flutter_api/stock_selection_handler.go)

- 将 `handleStockSelectionTest` 与 `TestHandleStockSelectionTest` 中的默认 query 改为：
  ```
  竞价涨幅在0.01%到3%之间;近7日最大涨幅大于9.8%;前一天成交金额大于5亿;流通市值在60亿到8000亿之间;主板
  ```
- 保留 `query` URL 参数透传能力，方便用户后期做"一句话选股"

### 2. 新增选股条件单元测试
**文件：** [backend/data/search_stock_api_test.go](file:///Users/Zhuanz/aiproject/go-stock/backend/data/search_stock_api_test.go)

新增 `TestSearchStock_UserStrategy`：
- 使用上述默认 query 调用 `NewSearchStockApi(query).SearchStock(50)`
- 验证接口返回 code == 0
- 验证 `data.result.dataList` 存在
- 打印命中股票数量与前 10 只股票的 code/name
- 不严格断言返回数量（依赖实时行情），但要求接口成功且有数据

### 3. （可选）验证接口端点
**文件：** [backend/flutter_api/server.go](file:///Users/Zhuanz/aiproject/go-stock/backend/flutter_api/server.go#L111)

- 确认 `/api/stock-selection-test` 路由已注册，无需新增路由

## Assumptions & Decisions
1. 使用东财自然语言接口一次性完成所有条件筛选，不做本地二次过滤
2. "7天内至少有一天涨幅大于9.8%"用"近7日最大涨幅大于9.8%"表达，东财接口可识别且等效于用户想要的"股性活跃、有资金合力"
3. 测试通过标准：接口返回成功（code=0）且 `dataList` 非空；不对具体股票做断言，避免行情变化导致测试不稳定
4. 保持现有接口路径 `/api/stock-selection-test` 不变，用户后期扩展为通用选股接口时再统一调整

## Verification Steps
1. `go test ./backend/data -run TestSearchStock_UserStrategy -v` 通过
2. 启动 HTTP 服务后，`curl "http://localhost:8080/api/stock-selection-test"` 返回成功且有股票列表
3. 可选：`curl "http://localhost:8080/api/stock-selection-test?query=..."` 验证自定义 query 仍生效
