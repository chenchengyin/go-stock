# 主板策略：形态买入信号（数据库 + Flutter 行内展示）

## 背景

T0 形态统计已在离线链路跑通（398 日、14,926 T0 样本），结论暂存 JSON（`pattern_aggregate_*.json`），未进入 SQLite，也未在盘达「主板策略」Tab 展示。

用户开盘买入时需要快速参考：**该股的 3 根日 K 形态在历史上达标率/真亏率如何**。展示形式为：

- **交通灯**（绿 / 黄 / 红 / 灰）— 方案 A，以真亏率为主
- **两个整数百分比** — 达标率 / 真亏率，如 `41/45`

## 目标

1. 将形态聚合结论写入 `data/stock.db`，分析更新时同步刷新库。
2. 正式选股（及归档 JSON）为每只入选股附加：3K 形态键、样本数、达标率、真亏率、买入信号。
3. Flutter 主板策略行 **保持现有布局不变**，仅在 **股票代码后面** 追加买入信号 UI。
4. 重跑离线分析 + sync-db 后，次日选股自动使用新结论，无需改 Flutter。

## 非目标

- 不改 T0 选股过滤链（市值、涨停记忆、开盘 0.01%～3% 等）。
- 第一版不做 hourly 交叉、5 根形态、涨跌停子类型。
- 不做按买入信号排序列表（Phase 2）。
- 不改动行内已有元素顺序：名称、标记、收盘涨幅、代码、Spacer、开盘涨幅/竞价预览、K 线按钮。

## 决策摘要

| 项 | 选择 |
|----|------|
| 结论存储 | SQLite 表 `t0_pattern_stats` + 配置表 `t0_pattern_config` |
| 更新方式 | CLI `t0-pattern-sync-db` 读 aggregate JSON 写入 DB |
| 形态窗口 | 3 根日 K（与现有分析一致） |
| 形态计算 | 复用 `backend/analysis/candlepattern`（与离线统计同口径） |
| 查库时机 | `RunT0Selection` 组装 `T0SelectionResult` 时 |
| 信号规则 | 方案 A（真亏率为主，见 §3） |
| 样本不足 | N < 10 → `insufficient`，灰灯，仍显示实际率（若有） |
| 降级链 Phase 1 | 仅精确 3K；查无 / N<10 走 insufficient |
| Flutter 位置 | **股票代码 `Text` 之后**，见 §4 |

---

## 1. 数据库

### 1.1 表 `t0_pattern_stats`

| 列 | 类型 | 说明 |
|----|------|------|
| `id` | INTEGER PK | 自增 |
| `pattern` | TEXT | 如 `XY\|ZT\|ZT`，UNIQUE(pattern, window) |
| `window` | INT | 固定 3 |
| `t0_n` | INT | T0 子集样本数 |
| `win_rate` | REAL | 达标率 ≥2.5%（%） |
| `fail_rate` | REAL | 真亏率 <0%（%） |
| `avg_pnl` | REAL | T0 均盈（%），可选展示 |
| `med_pnl` | REAL | T0 中位（%） |
| `batch_id` | TEXT | 如 `2025-01-02:2026-08-22` |
| `updated_at` | DATETIME | 写入时间 |

索引：`(pattern, window)` UNIQUE。

### 1.2 表 `t0_pattern_config`

单行活跃配置（`id=1`），便于改阈值不重跑形态：

| 列 | 默认值 | 含义 |
|----|--------|------|
| `green_max_fail` | 45 | 🟢 要求真亏率 ≤ 此值 |
| `green_min_win` | 30 | 🟢 要求达标率 ≥ 此值 |
| `red_min_fail` | 52 | 🔴 真亏率 > 此值 → 红 |
| `red_max_win` | 22 | 🔴 达标率 < 此值 → 红 |
| `min_samples` | 10 | 低于此值为 insufficient |
| `batch_id` | | 与 stats 同步 |

### 1.3 迁移

- 在 `backend/flutter_api/server.go` 的 `AutoMigrate()` 注册新 model（或 `backend/models/t0_pattern.go`）。
- 与现有 `data/stock.db`、WAL 模式兼容。

### 1.4 更新 CLI

```bash
# 1. 生成/更新聚合 JSON（已有）
go run ./cmd/t0-pattern-aggregate/ \
  --date-range 2025-01-02:2026-08-22 \
  --out backend/data/cache/t0/pattern/pattern_aggregate_2025-2026.json

# 2. 写入 SQLite（新增）
go run ./cmd/t0-pattern-sync-db/ \
  --from backend/data/cache/t0/pattern/pattern_aggregate_2025-2026.json
```

行为：事务内 DELETE+INSERT 或 UPSERT 全量 `t0_pattern_stats`；更新 `t0_pattern_config.batch_id` 与 `updated_at`。

**工作流约定**：每次扩展历史数据并 `--date-range` 重跑 aggregate 后，执行一次 `sync-db`，即完成「更新结论到数据库」。

---

## 2. 选股链路：算形态 + 查库

### 2.1 形态计算

对 `RunT0Selection` 结果组装循环中每只**已入选**股票：

1. 从当日日线 gob 取 `hist`（选股日前，`HistBeforeTradeDate` 同口径）。
2. `BuildPatternLabels(hist, 3)` → `pattern`；失败则 `pattern=""`，信号 `insufficient`。
3. `SELECT * FROM t0_pattern_stats WHERE pattern=? AND window=3`。
4. 用 `t0_pattern_config` 计算 `signal`（§3）。
5. 写入 `T0SelectionResult` 新字段（§2.2），并随 `saveT0SelectionArchive` 落 JSON。

与离线 `CollectObservations` **同一 package**，避免口径漂移。

### 2.2 API / JSON 新字段

```go
type T0SelectionResult struct {
    // ... 现有字段 ...
    Pattern       string  `json:"形态"`
    PatternT0N    int     `json:"形态样本数"`
    PatternWinPct float64 `json:"形态达标率(%)"`
    PatternFailPct float64 `json:"形态真亏率(%)"`
    BuySignal     string  `json:"买入信号"` // green | yellow | red | insufficient
}
```

- 归档 JSON 含上述字段 → 历史日只读 API 也能展示，无需重算（除非 gob 仍在且用户强制 refresh）。
- 若读归档时缺字段（旧文件）→ 后端尝试用 gob 补算；仍无则 `insufficient`。

### 2.3 竞价预览阶段（09:15–09:25）

- 3K 形态仅依赖 **选股日前 hist**，与当日竞价涨幅无关 → **预览名单可提前算形态信号**。
- 预览 API 返回候选列表时同样附带买入信号字段（预热候选走同一 enrich 函数）。

---

## 3. 买入信号规则（方案 A）

输入：库中 `win_rate`、`fail_rate`、`t0_n` 及 config。

| `买入信号` | 条件 |
|------------|------|
| `green` | `t0_n ≥ min_samples` 且 `fail_rate ≤ green_max_fail` 且 `win_rate ≥ green_min_win` |
| `red` | `t0_n ≥ min_samples` 且 (`fail_rate > red_min_fail` 或 `win_rate < red_max_win`) |
| `yellow` | `t0_n ≥ min_samples` 且非 green 非 red |
| `insufficient` | `t0_n < min_samples` 或 pattern 为空 或 库中无记录 |

**数字来源**：始终为库中（或 insufficient 时为空/0）的统计值，**不是**灯色反推的假数。

默认阈值依据 398 日全池：达标 26.4%、真亏 51.9%，及深入分析中 2 板、2.0–2.5% 高开等分层结论。

---

## 4. Flutter UI

### 4.1 原则

- **不改动**现有 Row 子元素顺序与样式。
- **仅追加**一段 widget，紧接在 **股票代码** `Text(stock.rawCode)` 之后、`Spacer()` 之前。

### 4.2 现有行结构（保持不变）

```
[名称] [标记] [收盘涨幅%] [代码]  ← 此处追加  [Spacer] [开盘涨幅%|竞价预览] [K线]
```

参考截图（大中矿业）：

```
大中矿业 [涨停破板] -1.73% 001203  🟢41/45  ·····  开盘+1.27%  [📈]
```

### 4.3 追加 widget 规范

| 元素 | 规范 |
|------|------|
| 间距 | 代码后 `SizedBox(width: 6)` |
| 交通灯 | 圆点直径 ~8px；颜色见下表 |
| 数字 | `"{win}/{fail}"` 整数%，fontSize 11，字重 w500 |
| 间距 | 灯与数字 `SizedBox(width: 2)` |

| `buySignal` | 灯色 | 数字色 |
|-------------|------|--------|
| `green` | 绿（success） | 默认 secondary |
| `yellow` | 黄/橙（warning） | 默认 secondary |
| `red` | 红（danger） | 默认 secondary |
| `insufficient` | 灰 | tertiary；可显示 `—/—` 或仅 `N=5` |

- **preview / confirmed / historical** 三种 phase 均展示（有字段则显示）。
- 长按或 Tooltip（可选 Phase 1.5）：`形态: 小阴→涨停→涨停 · N=56 · batch 2025-01~2026-08`。

### 4.4 模型

`T0StrategyStock` 增加：

```dart
final String pattern;
final int patternT0N;
final double patternWinPct;
final double patternFailPct;
final String buySignal; // green | yellow | red | insufficient
```

`fromJson` 映射 `形态`、`形态样本数`、`形态达标率(%)`、`形态真亏率(%)`、`买入信号`。

### 4.5 修改文件

| 文件 | 改动 |
|------|------|
| `t0_strategy_view_model.dart` | 模型 + fromJson |
| `radar_page.dart` `_buildStrategyCard` | 代码 `Text` 后插入 `_buildBuySignalChip(stock)` |
| `t0_strategy_view_model_test.dart` | 解析新字段 |

---

## 5. 模块边界

```
backend/analysis/candlepattern/     # 形态计算（已有）
cmd/t0-pattern-aggregate/           # 离线聚合 → JSON（已有）
cmd/t0-pattern-sync-db/             # JSON → SQLite（新增）
backend/models/t0_pattern.go        # GORM models（新增）
backend/flutter_api/t0_pattern.go   # 查库 + signal 计算（新增）
backend/flutter_api/t0_selection.go # enrich 结果（修改）
trading_app/.../t0_strategy_*       # 展示（修改）
```

`candlepattern` 包仍不依赖 GORM；DB 访问仅在 `flutter_api` / `cmd`。

---

## 6. 错误处理

| 场景 | 行为 |
|------|------|
| DB 无 stats 行 | `insufficient`，日志 Warn |
| gob 缺失无法算 pattern | `insufficient`，不阻塞选股 |
| sync-db 未跑过 | 全 insufficient；文档说明需先 sync |
| 旧归档无新 JSON 字段 | 读档时尝试 enrich；失败则 insufficient |

---

## 7. 测试

| 层 | 内容 |
|----|------|
| Go | `signalFromRates()` 单元测试：边界 45/52/30/22、N<10 |
| Go | sync-db 往返：JSON → DB → lookup |
| Go | selection enrich：mock gob + mock stats → 结果字段 |
| Flutter | `fromJson` 含买入信号；widget 测试 signal chip 颜色 |

---

## 8. Phase 2（后续）

- 形态降级：精确 3K → `ZT计数+末根` 聚合行
- 按 `buySignal` 排序（绿优先，其次达标率）
- Canvas / 设置页展示 config 阈值与 batch 日期
- Hourly 交叉维度独立表

---

## 9. 验收标准

1. 执行 aggregate + sync-db 后，`t0_pattern_stats` 行数 ≈ 656（T0 N≥5 形态数）。
2. 当日选股 JSON 与 API 每只票含 `形态`、`买入信号`、`形态达标率(%)`、`形态真亏率(%)`。
3. Flutter 主板策略行：**名称/标记/收盘/代码/开盘/K 线** 与现网一致；代码后可见 `🟢41/45` 形态。
4. 重跑 sync-db 更新 batch 后，新选股结果反映新比率，Flutter 无发版改动。
