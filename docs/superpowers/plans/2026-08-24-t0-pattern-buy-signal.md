# 主板策略形态买入信号 Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** 将 3K 形态统计结论写入 SQLite，选股时为每只票附加买入信号（交通灯 + 达标/真亏率），Flutter 主板策略行在**股票代码后**展示 `🟢41/45`。

**Architecture:** 离线 `t0-pattern-aggregate` → JSON；新增 `t0-pattern-sync-db` 全量写入 `t0_pattern_stats`。`flutter_api` 在组装 `T0SelectionResult` 时用 `candlepattern.BuildPatternLabels` 算形态键、查库、`signalFromRates` 算灯色；结果写入 selection JSON 与 API。Flutter 只解析新 JSON 字段，在 `_buildStrategyCard` 代码后追加 chip。

**Tech Stack:** Go 1.x、GORM + SQLite（`data/stock.db`）、`backend/analysis/candlepattern`、Flutter/Dart（`trading_app`）

**Spec:** `docs/superpowers/specs/2026-08-24-t0-pattern-buy-signal-design.md`

## Global Constraints

- 形态窗口固定 **3** 根日 K；统计口径与 `candlepattern` 离线分析一致
- 买入信号值仅：`green` | `yellow` | `red` | `insufficient`
- 方案 A 阈值默认值：`green_max_fail=45`、`green_min_win=30`、`red_min_fail=52`、`red_max_win=22`、`min_samples=10`
- JSON 字段名：`形态`、`形态样本数`、`形态达标率(%)`、`形态真亏率(%)`、`买入信号`
- Flutter **不得**改动现有 Row 元素顺序；新 UI **仅**在 `stock.rawCode` 之后、`Spacer` 之前
- insufficient 时：灰灯；若库中有记录且 N&lt;10，仍显示整数 `%` 如 `19/62`；无记录显示 `—/—`
- 不改 T0 选股过滤链；`candlepattern` 包不依赖 GORM

## File Structure

| 文件 | 职责 |
|------|------|
| `backend/models/t0_pattern.go` | GORM：`T0PatternStat`、`T0PatternConfig` |
| `backend/flutter_api/t0_pattern.go` | `signalFromRates`、DB lookup、hist 转换、`enrichSelectionWithPattern` |
| `backend/flutter_api/t0_pattern_test.go` | signal + enrich 单测 |
| `cmd/t0-pattern-sync-db/main.go` | JSON → SQLite CLI |
| `cmd/t0-pattern-sync-db/main_test.go` | sync 往返测试 |
| `backend/flutter_api/t0_selection.go` | 新字段、组装/预览/归档 enrich 接入 |
| `backend/flutter_api/server.go` | `AutoMigrate` 注册新 model |
| `trading_app/.../t0_strategy_view_model.dart` | 模型 + fromJson |
| `trading_app/.../radar_page.dart` | `_buildBuySignalChip` |
| `trading_app/test/t0_strategy_view_model_test.dart` | JSON 解析测试 |

---

### Task 1: GORM 模型 + `signalFromRates` 纯函数

**Files:**
- Create: `backend/models/t0_pattern.go`
- Create: `backend/flutter_api/t0_pattern.go`（仅 `signalFromRates` + 常量）
- Create: `backend/flutter_api/t0_pattern_test.go`
- Modify: `backend/flutter_api/server.go` — `AutoMigrate` 增加 `&models.T0PatternStat{}`, `&models.T0PatternConfig{}`

**Interfaces:**
- Produces: `func signalFromRates(winPct, failPct float64, t0N int, cfg models.T0PatternConfig) string`
- Produces: `const BuySignalGreen = "green"` 等四个常量

- [ ] **Step 1: Write failing tests**

```go
// backend/flutter_api/t0_pattern_test.go
func TestSignalFromRates(t *testing.T) {
	cfg := models.T0PatternConfig{
		GreenMaxFail: 45, GreenMinWin: 30,
		RedMinFail: 52, RedMaxWin: 22, MinSamples: 10,
	}
	cases := []struct {
		win, fail float64
		n         int
		want      string
	}{
		{41, 44, 56, BuySignalGreen},
		{19, 62, 42, BuySignalRed},
		{28, 48, 30, BuySignalYellow},
		{50, 40, 5, BuySignalInsufficient},
		{41, 44, 0, BuySignalInsufficient},
	}
	for _, c := range cases {
		got := signalFromRates(c.win, c.fail, c.n, cfg)
		if got != c.want {
			t.Fatalf("win=%v fail=%v n=%v => %q want %q", c.win, c.fail, c.n, got, c.want)
		}
	}
}
```

- [ ] **Step 2: Run test — expect FAIL**

Run: `export PATH="$PWD/.tools/go/bin:$PATH" && go test ./backend/flutter_api/ -run TestSignalFromRates -count=1`

- [ ] **Step 3: Implement models + signalFromRates**

`backend/models/t0_pattern.go`:

```go
type T0PatternStat struct {
	ID        uint      `gorm:"primaryKey"`
	Pattern   string    `gorm:"size:64;uniqueIndex:idx_pattern_window"`
	Window    int       `gorm:"uniqueIndex:idx_pattern_window"`
	T0N       int
	WinRate   float64
	FailRate  float64
	AvgPnL    float64
	MedPnL    float64
	BatchID   string    `gorm:"size:32"`
	UpdatedAt time.Time
}

type T0PatternConfig struct {
	ID           uint `gorm:"primaryKey"`
	GreenMaxFail float64
	GreenMinWin  float64
	RedMinFail   float64
	RedMaxWin    float64
	MinSamples   int
	BatchID      string
	UpdatedAt    time.Time
}
```

`signalFromRates` 按 spec §3 顺序：insufficient → green → red → yellow。

- [ ] **Step 4: Register AutoMigrate in `server.go`**

- [ ] **Step 5: Run test — expect PASS**

Run: `go test ./backend/flutter_api/ -run TestSignalFromRates -count=1`

- [ ] **Step 6: Commit**

```bash
git add backend/models/t0_pattern.go backend/flutter_api/t0_pattern.go backend/flutter_api/t0_pattern_test.go backend/flutter_api/server.go
git commit -m "feat(t0): add pattern stat models and buy signal rules"
```

---

### Task 2: `t0-pattern-sync-db` CLI

**Files:**
- Create: `cmd/t0-pattern-sync-db/main.go`
- Create: `cmd/t0-pattern-sync-db/main_test.go`

**Interfaces:**
- Consumes: aggregate JSON shape from `t0-pattern-aggregate` (`patterns[].pattern`, `t0_n`, `win_rate`, `fail_rate`, `avg_t0`, `med_t0`; top-level `range` as batch_id)
- Produces: `func syncPatternStats(db *gorm.DB, path string) (batchID string, count int, err error)`

- [ ] **Step 1: Write failing test**

使用 `:memory:` SQLite，`syncPatternStats` 写入 2 条 pattern，断言行数与字段。

- [ ] **Step 2: Run test — expect FAIL**

Run: `go test ./cmd/t0-pattern-sync-db/ -count=1`

- [ ] **Step 3: Implement CLI**

```bash
go run ./cmd/t0-pattern-sync-db/ \
  --from backend/data/cache/t0/pattern/pattern_aggregate_2025-2026.json
```

- 事务内 `DELETE FROM t0_pattern_stats` 再批量 INSERT
- `t0_pattern_config` id=1 upsert，写入默认阈值 + batch_id
- 启动时 `db.Init` + `flutter_api.AutoMigrate()` 或仅 migrate pattern models

- [ ] **Step 4: Run test — expect PASS**

- [ ] **Step 5: Run against real aggregate JSON**

Run: `go run ./cmd/t0-pattern-sync-db/ --from backend/data/cache/t0/pattern/pattern_aggregate_2025-2026.json`

Expected: ~656 rows inserted

- [ ] **Step 6: Commit**

```bash
git add cmd/t0-pattern-sync-db/
git commit -m "feat(t0): add pattern stats sync-db CLI"
```

---

### Task 3: DB lookup + `enrichSelectionWithPattern`

**Files:**
- Modify: `backend/flutter_api/t0_pattern.go`
- Modify: `backend/flutter_api/t0_pattern_test.go`

**Interfaces:**
- Produces: `func lookupPatternStat(pattern string, window int) (models.T0PatternStat, bool)`
- Produces: `func loadPatternConfig() models.T0PatternConfig`
- Produces: `func patternFromHist(hist []dailyBar) string` — 转 `[]candlepattern.DailyBar` 后 `BuildPatternLabels(hist, 3)`
- Produces: `func enrichResultWithPattern(r *T0SelectionResult, hist []dailyBar)`

- [ ] **Step 1: Write failing test for `patternFromHist`**

用固定 OHLC hist（3+1 根）断言 pattern 字符串如 `XY|ZT|ZT`。

- [ ] **Step 2: Write failing test for `enrichResultWithPattern`**

内存 DB 插入 stat 行，enrich 后断言 JSON 字段。

- [ ] **Step 3: Implement lookup + enrich**

```go
func enrichResultWithPattern(r *T0SelectionResult, hist []dailyBar) {
	pat := patternFromHist(hist)
	r.Pattern = pat
	if pat == "" {
		r.BuySignal = BuySignalInsufficient
		return
	}
	st, ok := lookupPatternStat(pat, 3)
	cfg := loadPatternConfig()
	if !ok {
		r.BuySignal = BuySignalInsufficient
		return
	}
	r.PatternT0N = st.T0N
	r.PatternWinPct = st.WinRate
	r.PatternFailPct = st.FailRate
	r.BuySignal = signalFromRates(st.WinRate, st.FailRate, st.T0N, cfg)
}
```

- [ ] **Step 4: Run tests — expect PASS**

Run: `go test ./backend/flutter_api/ -run 'TestPatternFromHist|TestEnrichResult' -count=1`

- [ ] **Step 5: Commit**

```bash
git add backend/flutter_api/t0_pattern.go backend/flutter_api/t0_pattern_test.go
git commit -m "feat(t0): pattern lookup and selection enrich"
```

---

### Task 4: 接入选股链路与归档补全

**Files:**
- Modify: `backend/flutter_api/t0_selection.go`

**Interfaces:**
- Consumes: `enrichResultWithPattern(*T0SelectionResult, []dailyBar)`
- Produces: `T0SelectionResult` 五字段在正式选股、预览、读档路径均可用

- [ ] **Step 1: Extend `T0SelectionResult` struct**

添加 §2.2 五个字段。

- [ ] **Step 2: `RunT0Selection` 组装循环**

在 `results = append(...)` 之前：

```go
res := T0SelectionResult{ ... }
enrichResultWithPattern(&res, hist)
results = append(results, res)
```

- [ ] **Step 3: `assembleT0CandidateResults`**

同样在 append 前调用 `enrichResultWithPattern(&item, hist)`。

- [ ] **Step 4: 归档读档 enrich**

新增 `func enrichArchivedResults(tradeDate string, results []T0SelectionResult) []T0SelectionResult`：

- 若 `买入信号` 已有值 → 跳过
- 否则从 gob 加载 hist（`loadOrFetchT0Daily` 或只读 gob）→ enrich

在 `handleT0Selection` 返回 archived 结果前调用。

- [ ] **Step 5: Integration test**

扩展或新增 `t0_selection_*_test.go`：mock DB stat + 小 hist → `RunT0Selection` 或单测 enrich 路径。

Run: `go test ./backend/flutter_api/ -run TestT0SelectionPatternEnrich -count=1`

- [ ] **Step 6: Commit**

```bash
git add backend/flutter_api/t0_selection.go backend/flutter_api/t0_*_test.go
git commit -m "feat(t0): attach pattern buy signal to selection results"
```

---

### Task 5: Flutter 模型 + 行内 chip

**Files:**
- Modify: `trading_app/lib/features/radar/presentation/radar_list/t0_strategy_view_model.dart`
- Modify: `trading_app/lib/features/radar/presentation/radar_list/radar_page.dart`
- Modify: `trading_app/test/t0_strategy_view_model_test.dart`

**Interfaces:**
- Consumes: API JSON keys `形态`、`形态样本数`、`形态达标率(%)`、`形态真亏率(%)`、`买入信号`

- [ ] **Step 1: Write failing fromJson test**

```dart
test('parses pattern buy signal fields', () {
  final s = T0StrategyStock.fromJson({
    '股票代码': '001203.XSHE',
    '股票名称': '大中矿业',
    'T0开盘涨幅(%)': 1.27,
    'T0收盘涨幅(%)': -1.73,
    '标记': '涨停破板',
    '形态': 'XY|ZT|ZT',
    '形态样本数': 56,
    '形态达标率(%)': 41.1,
    '形态真亏率(%)': 44.6,
    '买入信号': 'green',
  });
  expect(s.buySignal, 'green');
  expect(s.patternWinPct, 41.1);
});
```

- [ ] **Step 2: Run test — expect FAIL**

Run: `cd trading_app && flutter test test/t0_strategy_view_model_test.dart`

- [ ] **Step 3: Extend `T0StrategyStock` + fromJson/copyWith**

- [ ] **Step 4: Add `_buildBuySignalChip(T0StrategyStock stock)` in `radar_page.dart`**

插入位置（`radar_page.dart` `_buildStrategyCard`）：

```dart
Text(stock.rawCode, ...),
const SizedBox(width: 6),
_buildBuySignalChip(stock),
const Spacer(),
```

Chip 实现：
- 8px 圆点颜色由 `buySignal` 映射
- `Text('${win.round()}/${fail.round()}', fontSize: 11)` 
- insufficient + 无数据 → `—/—`

- [ ] **Step 5: Run Flutter tests — expect PASS**

- [ ] **Step 6: Commit**

```bash
git add trading_app/lib/features/radar/presentation/radar_list/ trading_app/test/
git commit -m "feat(flutter): show pattern buy signal chip on strategy row"
```

---

### Task 6: 文档与首次灌库

**Files:**
- Modify: `.cursor/skills/trade_analysis/SKILL.md` — 增加 sync-db 工作流与 UI 说明

- [ ] **Step 1: Update skill 工作流**

追加：

```bash
go run ./cmd/t0-pattern-aggregate/ --date-range ... --out ...
go run ./cmd/t0-pattern-sync-db/ --from ...
```

- [ ] **Step 2: Run full pipeline on existing data**

Run aggregate + sync-db（见 Task 2 Step 5）

- [ ] **Step 3: Manual smoke**

1. 启动 backend + Flutter Web
2. 打开盘达 → 主板策略 → 选有归档的日期
3. 确认代码后出现 `🟢41/45` 或灰灯 `—/—`

- [ ] **Step 4: Commit**

```bash
git add .cursor/skills/trade_analysis/SKILL.md
git commit -m "docs(trade_analysis): document pattern sync-db workflow"
```

---

## Spec Coverage Checklist

| Spec § | Task |
|--------|------|
| §1 DB | Task 1, 2 |
| §2 选股 enrich | Task 3, 4 |
| §3 信号规则 | Task 1 |
| §4 Flutter UI 代码后 | Task 5 |
| §6 错误处理 | Task 3, 4 |
| §7 测试 | 各 Task |
| §9 验收 | Task 6 smoke |

## Execution Handoff

Plan 保存于 `docs/superpowers/plans/2026-08-24-t0-pattern-buy-signal.md`。

**两种执行方式：**

1. **Subagent-Driven（推荐）** — 每 Task 派生子 agent，Task 间人工/自动 review  
2. **Inline Execution** — 本会话按 Task 顺序直接实现，每 Task 结束 checkpoint

你选哪种？确认后我开始写代码（从 Task 1 起）。
