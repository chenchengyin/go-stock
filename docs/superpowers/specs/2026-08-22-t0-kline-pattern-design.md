# T0 K 线形态研究：日 K 分类 + 3/5 根组合统计

## 背景

现有 T0 选股链（`t0_selection.go`）基于动量 + 竞价高开过滤，已有「涨停破板 / 大阴线 / 跌停」等**涨跌幅标记**，但没有 K 线几何形态分类，也没有多根 K 线组合统计。

本设计探索：在 A2 动量池内，对单根日 K 划分类型，拼接 3 根 / 5 根序列，统计组合之后下一日的阳线触发率及 T0 实战浮盈表现，为后续选股过滤提供数据依据。

## 目标

1. **单根日 K 分类**：将每根日 K 归入离散类型（涨停、跌停、十字、小阴小阳、中阴中阳、大阴大阳、破板等）。
2. **序列组合**：生成 3 根、5 根日 K 序列（如 `XY|MYIN|SY`）。
3. **统计指标**：每个组合输出样本数、下一日阳线率、T0 子集浮盈 ≥ 2.5% 率及均值/胜率。
4. **1 小时线补充（Phase 1.5）**：对前 **3 个交易日** 的 hourly K 做简化分类，作为独立补充维度验证。
5. **完全独立模块**：只读现有 gob 缓存，**不修改** T0 选股主链、API、Flutter UI。

## 非目标

- Phase 1 **不改** `t0_selection.go`、`server.go`、Flutter 任何文件。
- Phase 1 **不新增** HTTP API 路由（CLI 入口即可）。
- Phase 1 **不回填** 已有 `t0_selection_*.json` 归档。
- 不在 Phase 1 做 UI 展示；UI 落地留 Phase 2，验证后再做。
- 不改动现有 T0 过滤链顺序与阈值。
- 1 小时线 Phase 1 只看 **3 个交易日**（约 12 根）；后续如需扩展到 5 天，单独迭代，不影响日 K 主流程。

## 决策摘要

| 项 | 选择 |
|----|------|
| 实现路径 | 方案 C：Phase 1 离线统计 → Phase 2 验证后嵌入 T0 / UI |
| 模块位置 | `backend/analysis/candlepattern/`（独立 Go 包） |
| 入口 | Phase 1：`cmd/t0-pattern-stats/` CLI；不注册到 flutter_api |
| 数据来源（日 K） | 只读现有 `backend/data/cache/t0/daily/t0_daily_cache_*.gob` |
| 数据来源（hourly） | Phase 1.5 独立请求东财 60 分钟 K 线；与日 K 主流程解耦 |
| 样本池 | **A2 动量池**：过滤 1（主板 + 市值 50～9000 亿）+ 过滤 2（近 7 日涨停记忆）+ 过滤 3（昨日成交额 ≥ 5 亿） |
| 成功指标（T0 子集） | 日内浮盈 ≥ 2.5%（开盘买、收盘卖）；浮盈 = T0 收盘涨幅 − T0 开盘涨幅 |
| 成功指标（全样本） | 下一日阳线率（下一根日 K 收盘 > 开盘） |
| 1 小时回看 | 固定 **前 3 个交易日**（约 12 根 hourly） |
| 样本下限 | 组合样本 N < 30 标记「样本不足」，不进入 Top 榜 |

## 1. 单根日 K 分类（12 类）

按优先级匹配，一根 K 线只归一类。实体涨幅 = `(收盘 − 开盘) / 昨收 × 100`。

| 代号 | 名称 | 判定规则 |
|------|------|----------|
| `ZT` | 涨停 | 收盘涨幅 ≥ 9.89% |
| `DT` | 跌停 | 收盘涨幅 ≤ −9.9% |
| `PB` | 涨停破板 | 最高涨幅 ≥ 9.89%，收盘涨幅 < 9.85% |
| `YX` | 阳十字星 | \|收盘 − 开盘\|/ 昨收 < 0.5%，且收盘 ≥ 开盘 |
| `YXN` | 阴十字星 | \|收盘 − 开盘\|/ 昨收 < 0.5%，且收盘 < 开盘 |
| `SY` | 小阳 | 实体涨幅 0.5%～2.5% |
| `XY` | 小阴 | 实体跌幅 −0.5%～−2.5% |
| `MY` | 中阳 | 实体涨幅 2.5%～6% |
| `MYIN` | 中阴 | 实体跌幅 −2.5%～−6% |
| `DY` | 大阳 | 实体涨幅 > 6%（未涨停） |
| `DYIN` | 大阴 | 实体跌幅 < −6%（未跌停） |
| `XX` | 其他 | 不满足以上 |

说明：
- 涨停 / 跌停 / 破板优先于大阳 / 大阴，避免重复归类。
- 十字星阈值 0.5%、小阳小阴 2.5%、中阴中阳 6% 可在 Phase 1 报表中做敏感性分析，但不改 T0 主链。

## 2. 3 / 5 根序列与统计

### 序列生成

对每个观察日 T、每只股票（当日在 A2 池内）：

- **3 根序列**：`bar[T-3] | bar[T-2] | bar[T-1]` → 观察 `bar[T]`
- **5 根序列**：`bar[T-5] | ... | bar[T-1]` → 观察 `bar[T]`

序列字符串示例：`XY|MYIN|SY`、`PB|XY|MY|SY|ZT`

### A2 池判定（复用现有逻辑，只读）

对交易日 `tradeDate`，股票 `s` 在 A2 池当且仅当：

1. `s` 在过滤 1 池（主板 60/00 + 市值 50～9000 亿）—— 从 gob 缓存 `Stocks` 字段读取；
2. `histBarsBeforeTradeDate` 末尾 7 日存在涨停记忆（收盘 ≥ 9.89% 或破板）；
3. 前一交易日成交额 ≥ 5 亿（`Volume × Close / 1e8`）。

逻辑与 `t0_selection.go` 一致，但在独立包内**复制纯函数**或**抽取到共享包**；Phase 1 优先复制最小逻辑，避免改动 `t0_selection.go`。

### 聚合指标（每个 pattern）

| 字段 | 含义 |
|------|------|
| `pattern` | 序列字符串 |
| `window` | 3 或 5 |
| `sample_count` | 样本数 N |
| `next_yang_rate` | 下一日阳线率（收 > 开） |
| `t0_subset_count` | T 日高开 0.01%～3% 的子样本数 |
| `t0_win_rate_2p5` | 子集内浮盈 ≥ 2.5% 比例 |
| `t0_avg_pnl` | 子集平均日内浮盈 |
| `t0_median_pnl` | 子集中位数 |

### 输出

- `backend/data/cache/t0/pattern/pattern_stats_3bar_YYYY-MM-DD.json`
- `backend/data/cache/t0/pattern/pattern_stats_5bar_YYYY-MM-DD.json`
- 控制台 Top 20 摘要（按 `t0_win_rate_2p5` 降序，N ≥ 30）

## 3. 1 小时线补充（Phase 1.5）

### 范围

- 固定回看 **前 3 个交易日**（不含观察日 T），约 12 根 60 分钟 K 线。
- 后续如需 5 天，仅在本模块扩展参数，不改 T0 主链。

### 分类（简化 6 类）

| 代号 | 规则（同逻辑，阈值略宽） |
|------|--------------------------|
| `H_ZT` | 该小时收盘涨幅 ≥ 9.89% |
| `H_DT` | 该小时收盘涨幅 ≤ −9.9% |
| `H_Y` | 阳（收 > 开，实体 > 0.3%） |
| `H_YIN` | 阴（收 < 开，实体 > 0.3%） |
| `H_DOJI` | 十字（实体 ≤ 0.3%） |
| `H_XX` | 其他 |

### 用法

- 日 K 组合命中后，附加 hourly 末 N 根序列（如末 2 根 `H_Y|H_Y`）做交叉统计。
- hourly 数据**单独请求、单独缓存**（`backend/data/cache/t0/pattern/hourly/`），失败时不影响日 K 统计。

## 4. 模块架构

```
backend/analysis/candlepattern/
  types.go         # BarType 枚举、DailyBar 输入结构
  classify.go      # 单根日 K 12 类分类
  classify_hour.go # hourly 6 类分类（Phase 1.5）
  pool.go          # A2 动量池判定（只读逻辑，不依赖 t0_selection 包级变量）
  sequence.go      # 3/5 根序列生成
  stats.go         # 聚合统计
  report.go        # JSON / CSV 输出

cmd/t0-pattern-stats/
  main.go          # CLI：--date、--window 3|5|all、--hourly、--min-samples 30
```

### 与现有代码的边界

| 允许 | 禁止 |
|------|------|
| 只读 gob 缓存文件 | 修改 `t0_selection.go` 过滤链 |
| 只读 selection 归档（可选对照） | 注册新 API 到 `server.go` |
| 新建独立缓存目录 `pattern/` | 修改 Flutter / 前端 |
| 复制/抽取纯函数（若后续 PR 单独做） | Phase 1 改动任何现有测试行为 |

### CLI 示例

```bash
# 日 K 3/5 根统计（默认读最新 gob）
go run ./cmd/t0-pattern-stats/ --date 2026-08-21 --window all

# 含 hourly 补充（Phase 1.5）
go run ./cmd/t0-pattern-stats/ --date 2026-08-21 --hourly --hourly-days 3
```

## 5. Phase 2 预留（本 spec 不实现）

验证 Top 组合后：

- 在 `T0SelectionResult` 增加 `Pattern3`、`Pattern5` 可选字段；
- `server.go` 增加只读 API；
- Flutter 主板策略 Tab 展示形态标签与排序。

Phase 2 需单独 spec / plan，不在本模块 Phase 1 范围内。

## 6. 测试

| 类型 | 内容 |
|------|------|
| 单元测试 | 12 类边界：涨停 vs 大阳、十字 vs 小阳、破板 vs 涨停 |
| 单元测试 | 3/5 序列拼接、样本不足过滤 |
| 集成测试 | 读真实 gob fixture，对 5 只股票手工核对分类 |
| 回归 | 跑 2026-08-21，确认 A2 池数量与现有日志量级一致（~几百只级别） |

## 7. 风险与缓解

| 风险 | 缓解 |
|------|------|
| hourly 请求量大 | Phase 1 默认关闭 `--hourly`；开启时落盘缓存、限并发 |
| 组合爆炸（12^5） | 只统计 A2 池实际出现的序列；稀疏组合自然过滤 |
| 与 T0 逻辑漂移 | `pool.go` 注释标明对应 `t0_selection.go` 函数与行号；单测对齐 |
| 误改主链 | Code review 检查 Phase 1 PR 不包含 `flutter_api/`、`trading_app/` 改动 |
