# T0 竞价前候选预览（09:15–09:25）设计

## 背景

当前主板策略在日线预热 ready 后、09:25 竞价确认前，服务端只返回 `candidate_count`，Flutter 停在「等待选股」页。集合竞价（约 09:15 起）已有可观察的实时涨幅，希望先展示预热候选全量名单，由客户端按实时涨幅排序查看；09:25 后再切换为正式选股结果（仅保留开盘涨幅 0.01%～3%）。

## 目标

1. 预热 ready 且**不在** 00:00–09:00 前日归档窗时，主接口响应附带当日 **`candidates`**（与 `candidate_count` 同源：近 7 日涨停 + 前日成交额 ≥5 亿）。
2. 客户端 **≥09:15 且 &lt;09:25** 进入候选预览：展示全量 `candidates`，通过现有 `/api/stock-realtime` 拉涨幅并本地排序。
3. 客户端 **≥09:25** 切换为正式 `/api/t0-selection` 的 `results`（服务端已 `filterOpenGap(0.01, 3.0)`），不再展示/请求候选模式。
4. **00:00–09:00** 行为不变：可附前日 `results`（`historical`），**不带**当日 `candidates`。

## 非目标

- 服务端不为候选预览附带竞价涨幅，也不在预热路径调用腾讯行情。
- 不新增独立 `/candidates` 路由（采用主接口 `candidates` 字段）。
- 不改正式选股过滤阈值；不强制重刷历史归档。
- 不在本需求中重做雷达异动 UI。

## 已锁定决策

| 项 | 选择 |
|----|------|
| 实时涨幅来源 | 客户端拉 `/api/stock-realtime`（主板策略新增能力） |
| 09:15 门槛 | **客户端**控制展示；服务端 ready 后即可带 `candidates`（09:00 起，因凌晨窗不带） |
| 凌晨窗 | 仅前日归档 `results`，不带 `candidates` |
| 响应形状 | 新字段 `candidates`，与 `results` 分离 |
| 一份名单 | 任意响应最多一种列表：历史 `results` / `candidates` / 正式 `results` |
| 实现风格 | 显式 phase 状态机（非最小堆分支） |

## 时间窗与状态

上海时区，且请求的 `date` 为「今天」时：

| 时段 | 服务端 | 客户端 UI 态 |
|------|--------|----------------|
| 00:00–09:00 | prewarm ready 可附前日 `results` + `historical`；**无** `candidates` | `historical` |
| 09:00–09:15 | ready 带 `candidates` + `candidate_count`；无正式 `results` | `waiting` |
| 09:15–09:25 | 同上 | `candidatePreview` |
| ≥09:25 | 正式选股：仅 `results`（0.01%～3%）；**无** `candidates` | `confirmed` |

非今日 `date`：不走竞价前候选预览；按现有 archived / 回测正式选股逻辑。

## 服务端设计

### 候选构建

抽出与预热进度一致的构建函数（伪名 `buildT0CandidateList(tradeDate)`）：

1. `loadT0DailyCache(tradeDate)`，未命中则不附 `candidates`（保持 warming/ready 仅进度字段）。
2. `histBarsBeforeTradeDate` 后：`filterLimitUpRecent(..., 7, 9.8)` → `filterTurnover(..., 5.0)`。
3. 组装候选条目（精简，兼容客户端已有字段名习惯）：

```json
{
  "股票代码": "600000.XSHG",
  "股票名称": "浦发银行",
  "成交额(亿)": 12.3,
  "前一交易日收盘": 10.5,
  "前一交易日收盘涨幅(%)": 2.1,
  "涨停日期": "2026-08-13",
  "MA20": 10.1,
  "标记": "涨停破板"
}
```

- **不包含**有意义的 `T0开盘涨幅(%)` / `T0收盘涨幅(%)`（可省略；若为兼容结构体序列化则固定为 0，客户端预览态忽略并用行情覆盖展示）。
- `candidate_count` 必须等于 `len(candidates)`（当附带列表时）。

### `buildPrewarmReadyResponse` 变更

- 若 `isPreopenPrevResultWindow(now, tradeDate)`：保持现状（可附历史 `results`），**不**附 `candidates`。
- 否则（典型为今日 ≥09:00 的 prewarm ready，或显式 `prewarm=1` 且非凌晨窗）：在缓存命中时附 `candidates`。
- 建议增加只读字段便于客户端分支（可选但推荐）：
  - `"phase": "prewarm_ready"`（有 candidates 时）或沿用「存在 `candidates` 键」作为探测条件；本设计采用 **存在非空/`candidates` 键** + 客户端本地时钟判定展示，服务端可同时返回 `"phase":"candidates"` 当列表已附上。

为降低歧义，约定：

- 附带候选时：`"phase": "candidates"`，且有 `candidates` 数组（可为空数组）。
- 凌晨历史时：可继续无 `phase`，或 `"phase":"historical"`（若已有 `historical:true` 则不必强制）。
- 正式选股响应：无 `prewarm`，无 `candidates`，有 `results`；可选 `"phase":"confirmed"`（非必须，客户端可用「无 prewarm + results」判断）。

### 正式选股路径（≥09:25）

`handleT0Selection` 在非 autoPrewarm 时仍 `RunT0Selection` + 归档；响应**不得**包含 `candidates`。

### 错误与降级

- 日线未 ready：保持 warming 进度，无 `candidates`。
- ready 但构建候选失败：仍返回 ready + `candidate_count`（若可算）或 0；可不带 `candidates` 键；客户端保持 waiting/预览空态并继续轮询。

## 客户端设计

### 状态机（`T0StrategyViewModel`）

四态：`historical` | `waiting` | `candidatePreview` | `confirmed`。

转换：

1. 主接口返回 `historical` + `results` → `historical`。
2. `prewarm` + warming / ready 且本地 &lt;09:15 → `waiting`（ready 时可缓存 `candidates` 但不展示列表，或仅显示候选数量）。
3. 本地 ≥09:15 且 &lt;09:25，且已有 `candidates`（本轮或缓存）→ `candidatePreview`：展示列表 + 启动行情轮询。
4. 本地 ≥09:25 或主接口返回非 prewarm 的正式 `results` → `confirmed`：停止候选行情轮询，展示正式结果（服务端排序：标记优先 + 开盘涨幅）。

### 行情与排序（仅 `candidatePreview`）

- 使用与雷达相同的 `fetchRealtimeQuotes(codes)`（`/api/stock-realtime`）。
- 展示用「实时涨幅」=`changePercent`（或接口等价字段）；按该值**降序**排序；行情缺失的条目沉底或保持原相对顺序。
- 轮询间隔：与现有雷达一致量级（约 10s）即可；名单在 09:15–09:25 内可随主接口轮询更新 `candidates`，再合并行情。

### UI

- `waiting`（ready）：文案可保留「数据预热完成，等待选股…」，并显示 `候选股票: N 只`。
- `candidatePreview`：复用策略卡片布局；主涨幅展示改为实时涨幅；可标注「竞价预览（未确认）」以免与正式结果混淆。
- `confirmed`：恢复现有正式卡片（开盘涨幅等）。

## 测试要点

**服务端**

- 非凌晨窗 + 有 daily 缓存的 ready 响应含 `candidates`，且 `len == candidate_count`。
- 凌晨窗 ready 响应有历史 `results` 时**无** `candidates`。
- 正式选股响应无 `candidates`。

**客户端**

- &lt;09:15 即使响应含 `candidates` 也不进入列表预览。
- ≥09:15 进入预览并按 mock 行情排序。
- ≥09:25 切到正式 `results` 并停止候选行情轮询。

## 风险

- 候选量可达数十～上百，`/api/stock-realtime` 需确认批量 codes 上限；必要时客户端分批请求。
- 设备时钟与上海时区偏差会导致 09:15/09:25 切换偏移；与现有「等待选股」依赖本地/服务端时间的方式一致，可后续再统一 NTP（非本需求）。
