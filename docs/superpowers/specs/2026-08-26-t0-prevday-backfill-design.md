# T0 前一交易日自动补全设计

## 背景与目标

现有主板策略在 **00:00～09:00** 凌晨窗口内，当天日线预热完成后会展示**最近一份早于当天的选股归档**（见 `2026-08-12-t0-preopen-previous-results-design.md`）。该逻辑**只读已有 JSON**，不会补跑缺失数据。

若前一交易日因服务未启动、无人访问等原因未完成预热或选股（例如 8/25 缺失，8/26 早盘前打开 App），用户只能看到更早的归档或停留在「等待选股」页。

**本设计目标**：在凌晨窗口内，若**紧邻的前一交易日**选股归档缺失，自动补全「日线预热 → 选股归档 → 收盘涨幅刷新」，使用户在当天开盘前能看到昨日正式选股结果。

## 已确认的产品决策

| 项 | 决策 |
| --- | --- |
| 补全内容 | 日线预热（gob）+ 正式选股归档（json），两者串行 |
| 触发方式 | 服务端 00:00～09:00 主动预热 ticker + 客户端 `prewarm=1` 请求，共用同一入口 |
| 补全范围 | 仅**紧邻的前一交易日**（周一补上周五） |
| 前一交易日判定 | 工作日回退 + 抽样 K 线校验（最多回退 5 个自然日） |
| 进行中 UI | 复用现有预热进度 UI，增加 `backfill_date` / `backfill_phase` |
| 实现方案 | 嵌入现有预热链路（方案 A），在 `tryStartT0Prewarm(today)` 之前执行 |

## 行为规则

1. 仅在上海时区 **00:00～09:00**（`isPreopenPrevResultWindow`）且请求日期为**当天**时，才可能启动前一交易日补全。
2. **触发条件**：`resolvePrevTradingDay(today)` 得到 `prevDate`，且 `t0_selection_{prevDate}.json` **不存在**（或损坏/不可读，视为不存在）。
3. 若 `prevDate` 归档**已存在**，即使日线 gob 缺失也**不补全**（展示只依赖归档，与历史只读原则一致）。
4. 补全顺序（串行）：
   - 若 `t0_daily_cache_{prevDate}.gob` 缺失 → `tryStartT0Prewarm(prevDate)` 直至 ready 或 failed；
   - `RunT0Selection(prevDate)` → `saveT0SelectionArchive(prevDate, force=false)`；
   - `refreshSelectionCloseRet(prevDate, force=false)`（昨日已收盘，写入真实 T0 收盘涨幅）；
   - 完成后继续 `tryStartT0Prewarm(today)`。
5. 补全完成后，现有 `buildPrewarmReadyResponseAt` 在凌晨窗口内通过 `findLatestSelectionArchiveBefore(today)` 注入 `historical`、`display_date`、`results`，前端直接展示昨日结果。
6. **09:00（含）后**不再启动新的补全任务；进行中的 job 可跑完。
7. 补全失败不阻塞当天预热：日志记录后仍尝试启动 today 预热。

## 架构与触发流程

### 统一入口

新增 `ensurePrevTradingDayReady(tradeDate string, now time.Time)`，由以下路径调用：

| 入口 | 文件 |
| --- | --- |
| `runT0AutoPrewarmTick` | `backend/flutter_api/server.go` |
| `writeT0PrewarmHTTP` | `backend/flutter_api/t0_selection.go` |

调用顺序：

```text
ensurePrevTradingDayReady(today, now)
  → tryStartT0Prewarm(today)
```

### 幂等与并发

- 进程内 mutex + 按 `today` 键维护补全状态，避免重复启动。
- 补全进行中再次请求 `/api/t0-selection?prewarm=1` 立刻返回当前 `warming` 进度（不阻塞）。
- `tryStartT0Prewarm(prevDate)` 复用现有日线预热 job 与 gob 写入逻辑。

## 前一交易日判定

纯函数 `resolvePrevTradingDay(tradeDate string, now time.Time) (string, bool)`：

1. 从 `tradeDate` 的前一个自然日开始，向前遍历，跳过周六、周日。
2. 最多回退 **5 个自然日**；超出则返回 `( "", false )`。
3. 对每个候选日 `d`，调用 `isValidTradingDay(d)`：
   - 若 `t0_daily_cache_{d}.gob` 已存在 → 有效；
   - 否则对基准股（如 `sh600000`）拉取日线，检查是否存在 `date == d` 的 K 线；
   - 有 K 线 → 有效；无 → 继续往前。
4. 返回第一个有效交易日。

纯函数 `needsPrevDayBackfill(tradeDate string, now time.Time) (prevDate string, need bool)`：在凌晨窗口内且 `prevDate` 归档缺失时返回 `need=true`。

## API 响应与状态机

### 进程内状态扩展

在现有 `t0WarmProgress`（键为 **today**）上增加：

| 字段 | 类型 | 含义 |
| --- | --- | --- |
| `backfill_date` | string | 正在补全的交易日，如 `"2026-08-25"` |
| `backfill_phase` | string | `"daily"` \| `"selection"` \| `"close_refresh"` |

客户端始终轮询 **today** 的预热响应，无需改 `date` 参数。

### 补全进行中响应示例

```json
{
  "date": "2026-08-26",
  "prewarm": true,
  "status": "warming",
  "backfill_date": "2026-08-25",
  "backfill_phase": "daily",
  "daily_fetched": 1200,
  "daily_total": 3500,
  "stock_count": 3500
}
```

`buildPrewarmProgressResponse` 与 `buildPrewarmReadyResponse` 在补全阶段透传上述字段。

### 补全完成 + 凌晨展示

today 预热 `ready` 且在 00:00～09:00 时，响应与现有历史注入一致：

```json
{
  "date": "2026-08-26",
  "prewarm": true,
  "status": "ready",
  "historical": true,
  "display_date": "2026-08-25",
  "results": []
}
```

## Flutter 设计

修改 `T0StrategyViewModel` / `T0WarmProgress`：

- 解析 `backfill_date`、`backfill_phase`。
- `radar_page.dart` 预热等待页：有 `backfill_date` 时主文案为「正在补全 YYYY-MM-DD 数据…」；`backfill_phase == "selection"` 时副文案为「生成 YYYY-MM-DD 选股结果…」。
- 进度条仍用 `daily_fetched / daily_total`（`daily` 阶段）；`selection` / `close_refresh` 阶段可显示 indeterminate 或保留满进度。
- 收到 `historical + results` 后走现有历史展示逻辑，顶部提示「当前显示 YYYY-MM-DD 选股结果」。

**不改动**：股票卡片、日期下拉导航、09:15 候选预览、09:25 正式选股流程。

## 错误处理与边界情况

| 场景 | 处理 |
| --- | --- |
| 5 天内无法解析有效交易日 | 不启动补全；today 预热照常 |
| 补全日线失败 | `status: failed`，`error` 含上下文；仍尝试 today 预热 |
| 选股过滤后无股票 | 写入 `count: 0` 的空归档，避免反复拉取；凌晨窗口展示空列表 |
| 09:00 后新请求 | 不启动补全 |
| 并发重复请求 | 返回当前 warming 进度，不重复 job |
| `refresh_close` 失败 | 归档保留；日志 Warn；不置整体 failed |
| 归档已存在 | 跳过补全（即使 gob 缺失） |

补全原则：**尽力而为**；失败不拖垮当天预热；已有归档不重跑选股。

## 与既有 spec 的关系

- 扩展 `2026-08-12-t0-preopen-previous-results-design.md`：历史展示仍通过 `findLatestSelectionArchiveBefore` 读取归档；本设计仅在**归档缺失**时写入新归档，不改变 09:00 后行为。
- 复用 `2026-08-10-mainboard-prewarm-and-morning-keepalive-design.md` 的主动预热 ticker 与 `2026-08-05-t0-cache-status-and-selection-archive-design.md` 的状态机。

## 测试

### 后端 `t0_prevday_backfill_test.go`

- `resolvePrevTradingDay`：周二→周一；周一→上周五；无 K 线候选继续回退。
- `needsPrevDayBackfill`：归档缺失触发；归档存在不触发；09:00 后不触发。
- `ensurePrevTradingDayReady`：temp dir 下验证 daily → selection → archive 顺序。
- warming 响应含 `backfill_date`、`backfill_phase`。
- 补全完成后 08:00 窗口注入 `historical`。
- 幂等：连续调用不重复写归档。

### Flutter

- 解析 `backfill_date` 的 warming 响应。
- 补全完成后 `historical` 列表与 `display_date` 正确。

### 手工验证

1. 删除 `t0_selection_2026-08-25.json`（及对应 gob）。
2. 08:00 前启动服务并打开 App。
3. 应看到补全进度 → 完成后展示 8/25 选股结果。
4. `list_dates` 含 `2026-08-25`。

## 主要改动文件

| 文件 | 改动 |
| --- | --- |
| `backend/flutter_api/t0_selection.go` | `resolvePrevTradingDay`、`ensurePrevTradingDayReady`、进度响应字段 |
| `backend/flutter_api/server.go` | `runT0AutoPrewarmTick` 调用补全 |
| `backend/flutter_api/t0_prevday_backfill_test.go` | 新增测试 |
| `trading_app/.../t0_strategy_view_model.dart` | 解析 backfill 字段 |
| `trading_app/.../radar_page.dart` | 补全文案 |
