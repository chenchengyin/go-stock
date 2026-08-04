# T0 预热状态返回 + 选股结果归档

## 背景

[`backend/flutter_api/t0_selection.go`](../../../backend/flutter_api/t0_selection.go) 已支持日线 gob 预缓存（`?prewarm=1`）与正式选股优先读缓存。当前同日用互斥锁阻塞后续请求，预热期间调用方无法立刻得知进度。同时需要按日归档选股结果，便于事后回看。

## 目标

1. 预热进行中时，接口立刻返回当前状态，不阻塞等待。
2. 正式选股：日线缓存文件已存在则继续；不存在且正在预热则返回预热中状态。
3. 按日归档选股结果；默认每天只写一次，`save=1` 可强制覆盖。
4. 缓存文件统一落在 `/tmp/go-stock-cache/` 子目录下，便于后续扩展其它模块缓存。

## 非目标

- 不新增独立路由（仍用 `/api/t0-selection` + 查询参数）。
- 不改 Flutter 前端（本轮仅后端）。
- 不引入 Redis/DB；单进程内存状态 + 本地文件即可。
- 不改变选股过滤阈值（市值 / 涨停 / 成交额 / 开盘涨幅；MA20 门闸仍暂缓）。

## 目录布局

根目录：`/tmp/go-stock-cache/`（不存在则 `MkdirAll`）

```
/tmp/go-stock-cache/
  t0/
    daily/
      t0_daily_cache_<YYYY-MM-DD>.gob
    selection/
      t0_selection_<YYYY-MM-DD>.json
```

原先若存在 `/tmp/t0_daily_cache_*.gob`，新逻辑只读写新路径，不强制迁移旧文件。

## 体积估算

| 内容 | 约占用 |
|------|--------|
| 日线 gob（主板+市值后约 1000～2000 只 × ≤30 根） | **5～15 MB/日** |
| 选股结果 JSON（通常几十只） | **10～100 KB/日** |
| 合计 | **约 5～15 MB/日**（30 个交易日约 0.15～0.5 GB） |

## API 语义（同一接口）

`GET /api/t0-selection`

| 参数 | 行为 |
|------|------|
| `prewarm=1` | 见下方「预热状态机」 |
| （正式选股，无 prewarm） | 见下方「正式选股与预热交互」 |
| `archived=1&date=` | 只读该日选股结果归档；不拉行情、不重算 |
| `save=1` | 本次正式选股成功后**强制覆盖**结果归档 |
| `date=` | 交易日，默认今天 |

### 预热中状态响应示例

```json
{
  "date": "2026-08-05",
  "prewarm": true,
  "status": "warming",
  "stock_count": 1200,
  "daily_fetched": 400,
  "daily_total": 1200,
  "elapsed_sec": 12.5
}
```

### 预热完成响应（保持现有字段，可带 status）

```json
{
  "date": "2026-08-05",
  "prewarm": true,
  "status": "ready",
  "stock_count": 1200,
  "daily_count": 1180,
  "candidate_count": 80,
  "cache_hit": true,
  "elapsed_sec": 0.05
}
```

### 正式选股遇到预热中（无日线文件）

```json
{
  "date": "2026-08-05",
  "status": "warming",
  "count": 0,
  "results": []
}
```

### 归档读取

```json
{
  "date": "2026-08-05",
  "archived": true,
  "saved_at": "2026-08-05T09:26:01+08:00",
  "count": 12,
  "results": [ ... ]
}
```

无归档文件时：`error` + `archived: true` + `count: 0`。

## 预热状态机

进程内按 `tradeDate` 维护状态：`idle` / `warming` / `ready` / `failed`。

```mermaid
stateDiagram_v2
  [*] --> Idle
  Idle --> Warming: prewarm且无日线文件
  Warming --> Ready: gob写完
  Warming --> Failed: 拉取失败
  Ready --> Ready: 再次prewarm返回完成统计
  Failed --> Warming: 再次prewarm重试
```

行为细则：

1. `prewarm=1` 且当日日线文件**已存在** → 返回完成统计（`status: ready`，`cache_hit: true`），不重拉。
2. `prewarm=1` 且状态为 `warming` → **立刻**返回进度（`daily_fetched` / `daily_total`），不二次启动。
3. `prewarm=1` 且无文件、非 warming → 启动**后台** goroutine 拉股票池+日线并写 gob；**本次请求立刻**返回 `status: warming`。
4. 进度：在并发拉日线过程中原子更新 `daily_fetched`；`daily_total` 在股票池确定后写入。
5. 失败：状态 `failed`，错误信息可放在响应 `error` 字段；再次 `prewarm=1` 可重试。

替换当前「整段 `Mutex.Lock` 阻塞到结束」的行为；日线写入仍用原子写（`.tmp` + rename）。

## 正式选股与预热交互

1. 日线缓存**文件已存在** → 读缓存并走完整选股（涨停/成交额 → 腾讯竞价 → 开盘过滤），即使内存状态仍为 warming（理论上写完文件后应变 ready）。
2. 文件**不存在**且状态 `warming` → 立刻返回 `status: warming`，不阻塞。
3. 文件**不存在**且非 warming → **现场拉取并写入**日线（兜底），再继续选股。

默认正式选股每次仍重算腾讯竞价结果（不自动用归档替代实时结果）。

## 选股结果归档

- 路径：`/tmp/go-stock-cache/t0/selection/t0_selection_<date>.json`
- 内容：`{ "date", "saved_at", "count", "results" }`，`results` 与现有 `T0SelectionResult` 一致。
- **默认写入**：正式选股成功后，若文件**不存在**则写入；已存在则跳过。
- **`save=1`**：本次成功后强制覆盖写入。
- **`archived=1`**：只读归档；与实时选股、预热互不干扰。参数优先级：`archived=1` 优先于 `prewarm=1`（同请求若同时带，只读归档）。

## 错误处理

- 日期格式非法：与现有一致，返回错误信息。
- 预热失败：状态 `failed`，`prewarm=1` 返回 `error` + `status: failed`。
- 归档不存在：`archived=1` 返回明确 error，HTTP 仍可用 200 + JSON error（与现有选股错误风格一致）。
- 磁盘写入失败：打日志；选股结果仍返回给调用方，不因归档失败而整单失败。

## 测试要点

- 预热启动后立即再调 `prewarm=1`，应立刻得到 `warming` 且 `daily_fetched` 递增（或至少不阻塞超时）。
- 预热中、无日线文件时正式选股返回 `warming`。
- 预热完成后正式选股命中日线缓存。
- 首次选股写入结果归档；第二次无 `save=1` 不覆盖；带 `save=1` 覆盖。
- `archived=1` 读回与写入内容一致。

## 实现落点

主要改动文件：[`backend/flutter_api/t0_selection.go`](../../../backend/flutter_api/t0_selection.go)

- 缓存根路径常量改为 `/tmp/go-stock-cache/t0/...`
- 预热改为后台任务 + 内存进度；去掉阻塞式按日长锁（或缩小为仅保护状态转换）
- 结果归档 load/save + handler 参数分流（`archived` / `save` / `prewarm`）
