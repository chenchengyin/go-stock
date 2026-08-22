# T0 K 线形态：1 小时线 5 日回看扩展

## 背景

主 spec [`2026-08-22-t0-kline-pattern-design.md`](./2026-08-22-t0-kline-pattern-design.md) 中，Phase 1.5 的 1 小时线补充维度固定回看 **前 3 个交易日**（约 12 根 60 分钟 K 线）。

本 spec 描述在 3 日 hourly 验证跑通后，将回看窗口扩展到 **前 5 个交易日**（约 20 根）的独立迭代方案，用于对比更长盘中节奏是否提升 T0 形态信号的区分度。

## 前置依赖

- **必须先完成**主 spec Phase 1（日 K 3/5 根统计）及 Phase 1.5（hourly 3 日）。
- 复用 `backend/analysis/candlepattern/` 包内已有：
  - `classify_hour.go`（hourly 6 类分类）
  - hourly 缓存目录 `backend/data/cache/t0/pattern/hourly/`
  - CLI `cmd/t0-pattern-stats/`

## 目标

1. 支持 `--hourly-days 5`，回看观察日 T 之前 **5 个交易日** 的 60 分钟 K 线（约 20 根）。
2. 对 hourly 序列做与 3 日版相同的 6 类分类与交叉统计。
3. 输出 3 日 vs 5 日对比报表，便于判断延长窗口是否值得保留。
4. **仍不修改** T0 选股主链、API、Flutter UI。

## 非目标

- 不改动日 K 3/5 根统计逻辑。
- 不在本 spec 实现 UI 展示。
- 不注册新 HTTP API。
- 不回填 `t0_selection_*.json`。
- 不支持 5 日以外的其他窗口（如 7 日）；若需要，另开 spec。

## 决策摘要

| 项 | 选择 |
|----|------|
| 回看窗口 | 前 **5 个交易日**（不含观察日 T），约 20 根 hourly |
| 分类规则 | 与 3 日版相同（6 类：`H_ZT` / `H_DT` / `H_Y` / `H_YIN` / `H_DOJI` / `H_XX`） |
| 样本池 | A2 动量池（与主 spec 一致） |
| 成功指标 | 与主 spec 一致：下一日阳线率 + T0 子集浮盈 ≥ 2.5% |
| 缓存策略 | 增量复用 3 日缓存，仅补拉缺失的 2 个交易日 |
| 对比输出 | 同 pattern 在 3 日 / 5 日 hourly 下的指标并排 |

## 1. 与 3 日版的差异

| 维度 | 3 日版（Phase 1.5） | 5 日版（本 spec） |
|------|---------------------|-------------------|
| 回看交易日 | T-3 … T-1 | T-5 … T-1 |
| hourly 根数 | ~12 根 | ~20 根 |
| CLI 参数 | `--hourly-days 3`（默认） | `--hourly-days 5` |
| 缓存文件 | `hourly_3d_{code}_{date}.json` | `hourly_5d_{code}_{date}.json` |
| 序列窗口 | 末 2 / 3 根 hourly 交叉 | 末 2 / 3 / 5 根 hourly 交叉 |

说明：5 日版包含 3 日窗口的数据，统计时可做子集对比（例如末 3 根 `H_Y|H_Y|H_YIN` 在 3 日 vs 5 日样本下的胜率差异）。

## 2. 数据拉取与缓存

### 数据源

- 东财 60 分钟 K 线：`EastMoneyKLineApi.GetMinuteKLine(code, KLineType60Min, ...)`
- 与 3 日版相同接口，仅扩大日期范围。

### 缓存路径

```
backend/data/cache/t0/pattern/hourly/
  hourly_3d_{shortCode}_{tradeDate}.json   # Phase 1.5 已有
  hourly_5d_{shortCode}_{tradeDate}.json   # 本 spec 新增
```

### 增量策略

对 `--hourly-days 5` 请求：

1. 若 `hourly_5d_*` 缓存命中 → 直接读。
2. 若仅有 `hourly_3d_*` → 补拉 T-5、T-4 两个交易日 hourly，与 3 日数据合并写入 `hourly_5d_*`。
3. 若均无 → 全量拉 5 日并落盘。

### 并发与限流

- 最大并发：10（与 3 日版一致，可配置 `--hourly-concurrency`）。
- 单股失败：跳过该样本，不影响日 K / 3 日 hourly 统计；记入 `hourly_fetch_errors.log`。

## 3. hourly 序列与交叉统计

### 序列生成（5 日版额外支持）

在 5 日 hourly 窗口内，对每只股票观察日 T：

| 序列类型 | 含义 | 示例 |
|----------|------|------|
| 末 2 根 | T-1 最后 2 小时 | `H_Y\|H_Y` |
| 末 3 根 | T-1 最后 3 小时 | `H_YIN\|H_Y\|H_Y` |
| 末 5 根 | T-1 最后 5 小时 | `H_DOJI\|H_YIN\|H_Y\|H_Y\|H_Y` |
| 5 日全序列 | T-5…T-1 全部 ~20 根 | 仅用于聚合特征，不拼全串（组合爆炸） |

**与日 K 交叉**：日 K 3/5 根 pattern 命中后，附加 hourly 末 N 根序列做二维统计。

示例：`XY|MYIN|SY`（日 K 3 根）+ `H_Y|H_Y`（hourly 末 2 根）→ 统计下一日阳线率 / T0 浮盈率。

### 聚合指标（每个 cross-pattern）

| 字段 | 含义 |
|------|------|
| `daily_pattern` | 日 K 序列 |
| `hourly_pattern` | hourly 末 N 根序列 |
| `hourly_days` | 3 或 5 |
| `hourly_tail` | 2 / 3 / 5（末几根） |
| `sample_count` | 样本数 |
| `next_yang_rate` | 下一日阳线率 |
| `t0_subset_count` | T0 高开子集数 |
| `t0_win_rate_2p5` | 浮盈 ≥ 2.5% 比例 |
| `t0_avg_pnl` | 平均日内浮盈 |

## 4. 3 日 vs 5 日对比报表

### 输出文件

```
backend/data/cache/t0/pattern/
  hourly_cross_stats_3d_YYYY-MM-DD.json
  hourly_cross_stats_5d_YYYY-MM-DD.json
  hourly_3d_vs_5d_compare_YYYY-MM-DD.json   # 本 spec 核心产出
```

### 对比报表结构（`hourly_3d_vs_5d_compare`）

对每个 `(daily_pattern, hourly_tail_pattern)` 组合：

```json
{
  "daily_pattern": "XY|MYIN|SY",
  "hourly_tail": "H_Y|H_Y",
  "days_3": {
    "sample_count": 45,
    "t0_win_rate_2p5": 0.62,
    "t0_avg_pnl": 1.8
  },
  "days_5": {
    "sample_count": 52,
    "t0_win_rate_2p5": 0.58,
    "t0_avg_pnl": 1.5
  },
  "delta_win_rate": -0.04,
  "recommendation": "keep_3d"
}
```

### 推荐规则（自动标注）

| 条件 | `recommendation` |
|------|------------------|
| 5 日样本 ≥ 30 且 `t0_win_rate_2p5` 比 3 日高 ≥ 5pp | `prefer_5d` |
| 3 日胜率更高或差异 < 5pp | `keep_3d` |
| 5 日样本 < 30 | `insufficient_5d` |

## 5. CLI 扩展

在现有 `cmd/t0-pattern-stats/` 上扩展，**不新建命令**：

```bash
# 5 日 hourly 交叉统计
go run ./cmd/t0-pattern-stats/ \
  --date 2026-08-21 \
  --hourly \
  --hourly-days 5 \
  --hourly-tail 2,3,5

# 3 vs 5 对比报表
go run ./cmd/t0-pattern-stats/ \
  --date 2026-08-21 \
  --hourly-compare

# 批量跑历史（154 交易日）
go run ./cmd/t0-pattern-stats/ \
  --date-range 2026-01-05:2026-08-21 \
  --hourly \
  --hourly-days 5 \
  --hourly-compare
```

新增 flag：

| Flag | 默认 | 说明 |
|------|------|------|
| `--hourly-days` | 3 | 3 或 5 |
| `--hourly-tail` | 2,3 | 末几根 hourly 参与交叉；5 日版可加 5 |
| `--hourly-compare` | false | 生成 3d vs 5d 对比报表 |
| `--hourly-concurrency` | 10 | 拉取并发数 |

## 6. 模块改动范围

仅改动独立模块，**禁止**改动 `flutter_api/`、`trading_app/`：

```
backend/analysis/candlepattern/
  hourly_fetch.go      # 扩展：支持 days=5、增量补拉
  hourly_cache.go      # 新增：3d/5d 缓存读写与合并
  hourly_cross_stats.go # 新增：日 K × hourly 交叉统计
  hourly_compare.go    # 新增：3d vs 5d 对比报表

cmd/t0-pattern-stats/
  main.go              # 扩展 flag
```

## 7. 测试

| 类型 | 内容 |
|------|------|
| 单元测试 | 5 日窗口交易日历计算（跳过休市） |
| 单元测试 | 3d 缓存 + 2 日增量合并为 5d |
| 单元测试 | `recommendation` 规则边界（5pp 阈值、样本不足） |
| 集成测试 | 固定 3 只股票，mock hourly 数据，核对 cross-stats 样本数 |
| 对比测试 | 同日期跑 3d 与 5d，确认 3d 指标与 Phase 1.5 输出一致 |

## 8. 风险与缓解

| 风险 | 缓解 |
|------|------|
| 5 日拉取量 ≈ 3 日的 1.7 倍 | 增量缓存；默认不开启，需显式 `--hourly-days 5` |
| hourly 末 5 根组合稀疏 | 样本 < 30 标记不足；优先看末 2 / 3 根 |
| 3d vs 5d 结论不稳定 | 154 日批量跑 + 分月对比；不自动写入 T0 主链 |
| 与 Phase 1.5 行为不一致 | 对比测试确保 `--hourly-days 3` 输出不变 |

## 9. 完成标准

1. `--hourly-days 5` 可独立运行，输出 `hourly_cross_stats_5d_*.json`。
2. `--hourly-compare` 生成 3d vs 5d 对比报表，含 `recommendation` 字段。
3. 全部测试通过；无 `flutter_api/` / `trading_app/` 改动。
4. 对比报表中至少 10 个 `(daily_pattern, hourly_tail)` 组合样本 ≥ 30，可供人工 review。

## 10. 后续（不在本 spec）

- 若 5 日版显著优于 3 日版，可在 Phase 2 UI spec 中将默认 hourly 窗口定为 5 日。
- 若两者差异不大，生产默认保持 3 日以节省请求量。
