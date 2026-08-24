---
name: trade_analysis
description: >-
  T0 与 A2 动量池交易分析：日 K 12 类分类、3/5 根组合形态统计、成功/失败/未达标定义、
  CLI 离线统计、Canvas K 线画法、中文结论与个股案例。在用户说 trade_analysis、交易分析、
  形态复盘、校准统计、或积累新数据后再分析时使用。
---

# 交易分析（trade_analysis）

## 何时读取本 skill

- 用户说 **trade_analysis** 或要求交易/形态分析
- 要用**新一批缓存数据**重新跑统计或矫正结论
- 用户要求解释形态含义、失败原因、或带**个股案例**的中文结论
- 用户要画/校准 K 线示意图（Canvas 或文字描述）
- 用户提到后续要细化涨停/跌停子类型（见「待扩展」）

## 代码与数据路径

| 项 | 路径 |
|----|------|
| 分类与统计模块 | `backend/analysis/candlepattern/` |
| CLI | `cmd/t0-pattern-stats/` |
| 日 K 缓存 | `backend/data/cache/t0/daily/t0_daily_cache_YYYY-MM-DD.gob` |
| 形态统计输出 | `backend/data/cache/t0/pattern/pattern_stats_{3,5}bar_YYYY-MM-DD.json` |
| 跨日聚合 JSON | `backend/data/cache/t0/pattern/pattern_aggregate_*.json` |
| 形态结论 SQLite | `backend/data/stock.db`（表 `t0_pattern_stats`、`t0_pattern_config`） |
| sync-db CLI | `cmd/t0-pattern-sync-db/` |
| 买入信号 spec | `docs/superpowers/specs/2026-08-24-t0-pattern-buy-signal-design.md` |
| 形态图鉴 Canvas | `canvases/t0-kline-pattern-guide.canvas.tsx`（Cursor 托管目录） |
| 12 类 K 线对照 Canvas | `canvases/t0-bar-types-reference.canvas.tsx` |

Go 环境：项目内 `.tools/go/bin/go`；网络受限时 `GOPROXY=https://goproxy.cn,direct`。

## 样本池 A2（与 t0_selection 对齐）

- 主板
- 近 **7 个交易日**有涨停记忆（封板或末日本日破板）
- **昨日**成交额 ≥ **5 亿**

模块独立，不改 `t0_selection.go` / Flutter。

## 单根日 K：12 类分类（`classify.go`）

**判定顺序**：涨停 → 跌停 → 破板 → 按实体分档。

| 代码 | 名称 | 规则 |
|------|------|------|
| ZT | 涨停 | **收盘涨幅** ≥ +9.89%（相对昨收） |
| DT | 跌停 | **收盘涨幅** ≤ −9.9% |
| PB | 破板 | 最高触板且收盘未封（高 ≥ +9.89%，收 < +9.85%） |
| YX / YXN | 十字阳/阴 | \|实体\| < 0.5%，收 ≥ 开 / 收 < 开 |
| SY / XY | 小阳/小阴 | 实体 ±0.5% ~ ±2.5% |
| MY / MYIN | 中阳/中阴 | 实体 ±2.5% ~ ±6% |
| DY / DYIN | 大阳/大阴 | 实体 > +6% / < −6%，且未涨跌停 |

**实体涨幅**（十字/小/中/大的唯一依据）：

```text
body = (收 − 开) / 昨收 × 100
```

**收盘涨幅**（涨跌停的唯一依据）：

```text
closeRet = (收 − 昨收) / 昨收 × 100
```

⚠️ 常见错误：用 closeRet 画实体档，或假定涨停必须「开盘 +8%」。**涨跌停只看收盘是否封板**；开盘任意，只要收盘达标即为 ZT/DT。

## Canvas K 线（组合形态）

- A 股：**阳线红、阴线绿**
- 单根对照图：开/收/高/低相对**该 K 线自己的昨收**（见 `t0-bar-types-reference.canvas.tsx`）
- **组合形态图（3 根 + 观察日）必须用累积坐标**：第 1 根相对起点 0%，第 2 根相对第 1 根收盘链式换算，第 3 根同理；观察日相对第 3 根收盘。否则十字/小 K 会错误落在底部基准线上
- 换算：`cum = (1 + prevCum/100) × (1 + rel/100) − 1`，再 ×100

## 涨跌停示意（当前 12 类不分子类型）

当前 **12 类不分子类型**；画图时每种涨跌停至少应理解三种典型开盘（仅供示意，非代码分类）：

**涨停 ZT（收盘 +10% 附近）**

| 俗称 | 开盘相对昨收 | 特征 |
|------|-------------|------|
| 一字板 | ≈ +10% | 开≈收，几乎无上影 |
| 小阳涨停 | ≥ +5% | 开盘已高，实体较小 |
| 大阳涨停 | < +5% | 开盘近昨收，中长阳拉板 |

**跌停 DT（收盘 −10% 附近）** — 对称理解：

| 俗称 | 开盘相对昨收 | 特征 |
|------|-------------|------|
| 一字跌停 | ≈ −10% | 开≈收 |
| 小阴跌停 | 已低开较多（如 ≤ −5%） | 实体较小 |
| 大阴跌停 | 高开或平开再砸 | 长阴封跌停 |

> **待扩展**（用户明确要求后再做）：在 ZT/DT 标签下增加子类型字段；当前 `BarType` 与 JSON 统计**不区分**上述子类。

## 组合形态与 T0 指标

- **窗口**：3 根 / 5 根日 K（观察日前连续 hist）
- **hist 长度**：`BuildPatternLabels` 需 `len(hist) >= window+1`（首根分类要前一日收盘价）
- **T0 子集**：当日高开 **1% ~ 3%**（相对昨收）
- **T0 浮盈**：`(closeRet − gap)`，即开盘买、收盘卖相对开盘价的盈亏

### 三类结果（用户口径，写结论时必须遵守）

| 类别 | 条件 |
|------|------|
| **成功** | T0 浮盈 ≥ **+2.5%** |
| **失败** | T0 浮盈 **< 0%**（真亏） |
| **未达标但盈利** | **0% ≤ T0 < 2.5%**（不算失败） |

不要把「赚了但不到 2.5%」算作失败。

## 运行统计

```bash
export PATH="$PWD/.tools/go/bin:$PATH"
export GOPROXY=https://goproxy.cn,direct

# 单日
go run ./cmd/t0-pattern-stats/ --date 2026-08-21 --window all

# 批量（跳过无 gob 的日期）
go run ./cmd/t0-pattern-stats/ --date-range 2026-01-05:2026-08-21 --window all --min-samples 30 --top 20
```

- 批量按**日**写 JSON；单日 Top 因样本分散常为空
- 跨日结论需**合并**多日 JSON 或重跑聚合（CLI 暂无 `--aggregate`）
- `--hourly` 为 Phase 1.5；5 日 hourly 见独立 spec

## 输出结论规范

1. **语言**：中文，避免堆代码
2. **证据**：每个重要结论配 **3~5 只个股**（日期、名称、代码、形态、高开、T0 盈亏）
3. **区分三类结果**，失败只指亏损
4. **Canvas K 线**：A 股习惯 **阳线红、阴线绿**；组合图用累积坐标
5. 形态示意图用 **实体 = c − o**（开收均相对昨收 %）校验，见 `t0-bar-types-reference.canvas.tsx`

## 分析工作流（数据更新后）

1. 确认 `backend/data/cache/t0/daily/` 日期范围
2. `go test ./backend/analysis/candlepattern/ -run TestLoadDailyCache`
3. 跑 `--date-range` 批量生成 pattern JSON
4. 跨日聚合：按 pattern 合并 T0 子集，算均盈、达标率、亏损率
5. 失败归因：收阴（收<昨收） vs 冲高回落（收≥昨收但收<开）
6. 用 Canvas 或文字交付；需要时更新 `canvases/*.canvas.tsx` 中的 `BAR` 预设

### 形态结论入库（主板策略买入信号）

聚合 JSON 生成后，写入 SQLite 供选股 API 查库：

```bash
# 1. 跨日聚合（398 日示例）
go run ./cmd/t0-pattern-aggregate/ \
  --date-range 2025-01-02:2026-08-22 \
  --out backend/data/cache/t0/pattern/pattern_aggregate_2025-2026.json

# 2. JSON → SQLite（全量替换 stats + 更新 config.batch_id）
# 默认写入 data/stock.db（与 cmd/server 一致）
go run ./cmd/t0-pattern-sync-db/ \
  --from backend/data/cache/t0/pattern/pattern_aggregate_2025-2026.json
```

- 每次扩展 `--date-range` 重跑 aggregate 后，**必须**再跑 `sync-db`
- Flutter 主板策略行：股票代码后展示交通灯 + `达标%/真亏%`（如 `41/45`）；规则见买入信号 spec §3
- 旧归档 JSON 无形态字段时，后端读档会用 gob 补算

## 已知局限

- 日 K 形态是**滞后**指标；T0 失败主因是高开低走，不单靠加 K 线标签解决
- 5 根组合样本稀疏，需更长区间或更大池子
- 涨跌停子类型、hourly 交叉均为后续 phase；**3K 形态买入信号已接入** T0 选股 API 与 Flutter 主板策略 Tab

## 参考 Baseline（2025-01-02 ~ 2026-08-22，398 日）

- T0 样本约 **14926**；成功 **26.4%**；真亏 **51.9%**；未达标但盈利 **21.6%**
- 亏损中约 **80%** 为收阴，**20%** 为冲高回落
- 末根涨停 T0 决策成功率约 **29%**（N=3120）；震荡形态（如小阳小阴小阳）达标率约 **19%**（N=42）
- 跨日聚合 CLI：`go run ./cmd/t0-pattern-aggregate/ --date-range START:END --out path.json`

新数据跑完后用上述 workflow 重算，**勿硬套旧数字**。
