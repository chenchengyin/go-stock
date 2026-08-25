# 主板策略蓝色优先排序 Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** 主板策略所有展示列表中，`买入信号=blue` 的股票稳定排在最前，其余排序规则与现有一致（标记优先 → 开盘涨幅降序；竞价预览再按实时涨幅）。

**Architecture:** 在后端 `sortT0ResultsForClient` 增加第一排序键「是否 blue」，所有 `results` API 出口继续走该函数；竞价预览 `candidates` 也经同一函数排序。Flutter 在 `_mergeQuotes` 中把 blue 置于实时涨幅排序之前，防止 09:15–09:25 轮询覆盖后端顺序。不改磁盘归档 JSON 顺序。

**Tech Stack:** Go 1.x（`backend/flutter_api`）、Flutter/Dart（`trading_app`）

**Related specs:** `docs/superpowers/specs/2026-08-12-t0-tag-first-sort-design.md`、`docs/superpowers/specs/2026-08-24-t0-pattern-buy-signal-design.md`

## Global Constraints

- 仅 **`买入信号 == "blue"`** 提升到全局最前；green/yellow/red/insufficient 相对顺序不变
- blue 组内、非 blue 组内：沿用 **标记非空优先**，再 **T0开盘涨幅(%) 降序**，同键 **稳定排序**（`sort.SliceStable`）
- **不得**修改 `t0_selection_*.json` 磁盘写入顺序
- **不得**改动 `_buildStrategyCard` 行内 UI 布局
- `buySignal` 字符串与后端常量一致：`blue` | `green` | `yellow` | `red` | `insufficient`
- 竞价预览（09:15–09:25）：blue 仍置顶；有实时行情的排在无行情前；组内按 `liveChangePercent` 降序
- Go 测试：`export PATH="$PWD/.tools/go/bin:$PATH"`；Flutter 测试：在 `trading_app/` 下 `flutter test`

## File Structure

| 文件 | 职责 |
|------|------|
| `backend/flutter_api/t0_selection.go` | 扩展 `sortT0ResultsForClient`；`candidates` 出口调用排序 |
| `backend/flutter_api/t0_sort_test.go` | blue 优先 + 与标记/涨幅组合用例 |
| `trading_app/lib/features/radar/presentation/radar_list/t0_strategy_view_model.dart` | 预览阶段 blue 优先排序 |
| `trading_app/test/t0_strategy_view_model_test.dart` | 客户端排序单测 |
| `docs/superpowers/specs/2026-08-25-t0-blue-first-sort-design.md` | 本需求设计摘要（Task 6） |

---

### Task 1: 后端排序键 `blue` 置顶

**Files:**
- Modify: `backend/flutter_api/t0_selection.go:327-341`
- Test: `backend/flutter_api/t0_sort_test.go`

**Interfaces:**
- Consumes: `BuySignalBlue`（`backend/flutter_api/t0_pattern.go`）、`T0SelectionResult.BuySignal`
- Produces: `func sortT0ResultsForClient(results []T0SelectionResult) []T0SelectionResult` — 第一键 blue 优先，其余行为不变

- [ ] **Step 1: Write the failing test**

在 `backend/flutter_api/t0_sort_test.go` 追加：

```go
func TestSortT0ResultsForClientBlueFirst(t *testing.T) {
	in := []T0SelectionResult{
		{StockCode: "A", OpenGap: 3.0, Tag: "涨停破板", BuySignal: BuySignalGreen},
		{StockCode: "B", OpenGap: 1.0, Tag: "", BuySignal: BuySignalBlue},
		{StockCode: "C", OpenGap: 2.5, Tag: "", BuySignal: BuySignalRed},
		{StockCode: "D", OpenGap: 0.5, Tag: "前一天跌停", BuySignal: BuySignalBlue},
	}
	got := sortT0ResultsForClient(in)
	want := []string{"B", "D", "A", "C"}
	for i, code := range want {
		if got[i].StockCode != code {
			t.Fatalf("pos %d = %s want %s (full %v)", i, got[i].StockCode, code, codes(got))
		}
	}
}

func TestSortT0ResultsForClientBlueThenOpenGap(t *testing.T) {
	in := []T0SelectionResult{
		{StockCode: "L", OpenGap: 1.0, BuySignal: BuySignalBlue},
		{StockCode: "H", OpenGap: 2.8, BuySignal: BuySignalBlue},
	}
	got := sortT0ResultsForClient(in)
	want := []string{"H", "L"}
	for i, code := range want {
		if got[i].StockCode != code {
			t.Fatalf("pos %d = %s want %s", i, got[i].StockCode, code)
		}
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `export PATH="$PWD/.tools/go/bin:$PATH" && go test ./backend/flutter_api/ -run 'TestSortT0ResultsForClientBlue' -count=1 -v`

Expected: FAIL — `B` 不在首位（当前仅按标记+涨幅排）

- [ ] **Step 3: Implement sort key**

替换 `backend/flutter_api/t0_selection.go` 中 `sortT0ResultsForClient`：

```go
// sortT0ResultsForClient 返回用于客户端展示的排序副本：
// 1) 买入信号 blue 最前；2) 有标记优先；3) T0开盘涨幅降序；稳定排序。不修改入参切片与磁盘归档。
func sortT0ResultsForClient(results []T0SelectionResult) []T0SelectionResult {
	sorted := make([]T0SelectionResult, len(results))
	copy(sorted, results)
	sort.SliceStable(sorted, func(i, j int) bool {
		bi := sorted[i].BuySignal == BuySignalBlue
		bj := sorted[j].BuySignal == BuySignalBlue
		if bi != bj {
			return bi
		}
		ti := sorted[i].Tag != ""
		tj := sorted[j].Tag != ""
		if ti != tj {
			return ti
		}
		return sorted[i].OpenGap > sorted[j].OpenGap
	})
	return sorted
}
```

- [ ] **Step 4: Run tests to verify they pass**

Run: `export PATH="$PWD/.tools/go/bin:$PATH" && go test ./backend/flutter_api/ -run TestSortT0ResultsForClient -count=1 -v`

Expected: PASS（含原有 `TestSortT0ResultsForClient`、`TestSortT0ResultsForClientStableSameKey`）

- [ ] **Step 5: Commit**

```bash
git add backend/flutter_api/t0_selection.go backend/flutter_api/t0_sort_test.go
git commit -m "feat(t0): sort blue buy signal first in strategy results"
```

---

### Task 2: 竞价预览 `candidates` 同样排序

**Files:**
- Modify: `backend/flutter_api/t0_selection.go:588-592`
- Test: `backend/flutter_api/t0_candidates_test.go`

**Interfaces:**
- Consumes: `sortT0ResultsForClient`
- Produces: prewarm ready 响应中 `candidates` 已 blue 置顶

- [ ] **Step 1: Write the failing test**

在 `backend/flutter_api/t0_candidates_test.go` 的 `TestBuildPrewarmReadyResponseAttachesCandidatesAfter0900` 末尾追加断言（或新建测试），构造带 `BuySignal` 的 mock 数据经 `assembleT0CandidateResults` + `sortT0ResultsForClient` 验证。更简单：在 `t0_sort_test.go` 增加：

```go
func TestSortT0ResultsForClientPreservesInput(t *testing.T) {
	in := []T0SelectionResult{{StockCode: "A", BuySignal: BuySignalBlue}}
	got := sortT0ResultsForClient(in)
	if got[0].StockCode != "A" {
		t.Fatal("single element changed")
	}
	if &in[0] == &got[0] {
		t.Fatal("should return copy not same backing")
	}
}
```

并在 `buildPrewarmReadyResponseAt` 修改后，扩展 `t0_candidates_test.go`：

```go
// 在 TestBuildPrewarmReadyResponseAttachesCandidatesAfter0900 内，mock DB + hist 使两只候选一只 blue 一只 green，
// 断言 resp["candidates"].([]T0SelectionResult)[0].BuySignal == BuySignalBlue
```

若 mock 过重，可仅改 integration 断言：手动构造 `cands` 切片调用 `sortT0ResultsForClient` 在 handler 路径的单元测试替代（见 Step 3）。

- [ ] **Step 2: Run test — expect FAIL**（若测 handler 且尚未改 handler）

- [ ] **Step 3: Apply sort to candidates**

`backend/flutter_api/t0_selection.go` `buildPrewarmReadyResponseAt`：

```go
	if ok {
		cands := assembleT0CandidateResults(tradeDate, step2, hist)
		cands = sortT0ResultsForClient(cands)
		resp["candidates"] = cands
		resp["phase"] = "candidates"
		resp["candidate_count"] = len(cands)
	}
```

- [ ] **Step 4: Run tests**

Run: `export PATH="$PWD/.tools/go/bin:$PATH" && go test ./backend/flutter_api/ -run 'TestSortT0ResultsForClient|TestBuildPrewarmReadyResponseAttachesCandidatesAfter0900' -count=1 -v`

Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add backend/flutter_api/t0_selection.go backend/flutter_api/t0_candidates_test.go
git commit -m "feat(t0): apply blue-first sort to prewarm candidates"
```

---

### Task 3: Flutter 预览轮询保持 blue 置顶

**Files:**
- Modify: `trading_app/lib/features/radar/presentation/radar_list/t0_strategy_view_model.dart:449-464`
- Test: `trading_app/test/t0_strategy_view_model_test.dart`

**Interfaces:**
- Consumes: `T0StrategyStock.buySignal`
- Produces: `void _mergeQuotes(...)` — 排序键：blue → 有行情 → 涨幅降序

- [ ] **Step 1: Write the failing test**

```dart
test('mergeQuotes：blue 信号排在实时涨幅更高的非 blue 之前', () {
  final vm = T0StrategyViewModel();
  vm.applyResponseForTest(_candidateReady(
    date: '2026-08-25',
    candidates: [
      {
        '股票代码': '600000.XSHG',
        '股票名称': '浦发银行',
        '买入信号': 'green',
      },
      {
        '股票代码': '600519.XSHG',
        '股票名称': '贵州茅台',
        '买入信号': 'blue',
      },
    ],
  ));
  // 模拟 09:20 预览阶段
  vm.applyResponseForTest({
    'prewarm': true,
    'status': 'ready',
    'phase': 'candidates',
    'date': '2026-08-25',
    'candidates': vm.results.map((s) => {
      '股票代码': s.stockCode,
      '股票名称': s.stockName,
      '买入信号': s.buySignal,
    }).toList(),
  });
  // 直接测排序 helper（Step 3 提取后）
  final sorted = T0StrategyViewModel.sortStrategyStocksForDisplay(
    vm.results,
    liveChangePercent: (s) => s.liveChangePercent,
  );
  expect(sorted.first.buySignal, 'blue');
});
```

若 `sortStrategyStocksForDisplay` 为 private，改为 `@visibleForTesting static` 方法。

- [ ] **Step 2: Run test — expect FAIL**

Run: `cd trading_app && flutter test test/t0_strategy_view_model_test.dart --name 'mergeQuotes'`

- [ ] **Step 3: Implement client-side sort helper**

在 `t0_strategy_view_model.dart` 增加：

```dart
@visibleForTesting
static int buySignalSortRank(String buySignal) =>
    buySignal == 'blue' ? 0 : 1;

@visibleForTesting
static List<T0StrategyStock> sortStrategyStocksForDisplay(
  List<T0StrategyStock> list, {
  required double? Function(T0StrategyStock s) liveChangePercent,
  bool preview = false,
}) {
  final out = List<T0StrategyStock>.from(list);
  out.sort((a, b) {
    final br = buySignalSortRank(a.buySignal).compareTo(buySignalSortRank(b.buySignal));
    if (br != 0) return br;
    if (preview) {
      final am = liveChangePercent(a) != null;
      final bm = liveChangePercent(b) != null;
      if (am != bm) return am ? -1 : 1;
      if (am && bm) {
        return liveChangePercent(b)!.compareTo(liveChangePercent(a)!);
      }
      return 0;
    }
    // 非预览：后端已排；保持相对顺序（稳定 0）
    return 0;
  });
  return out;
}
```

修改 `_mergeQuotes`：

```dart
    merged.sort((a, b) {
      final br = buySignalSortRank(a.buySignal).compareTo(buySignalSortRank(b.buySignal));
      if (br != 0) return br;
      final am = a.liveChangePercent != null;
      final bm = b.liveChangePercent != null;
      if (am != bm) return am ? -1 : 1;
      if (!am) return 0;
      return b.liveChangePercent!.compareTo(a.liveChangePercent!);
    });
```

在 `_applyResponse` 解析 `rawList` / 历史 `results` 后**不必**再排序（信任后端）；仅 `_mergeQuotes` 必须改。

- [ ] **Step 4: Run Flutter tests**

Run: `cd trading_app && flutter test test/t0_strategy_view_model_test.dart`

Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add trading_app/lib/features/radar/presentation/radar_list/t0_strategy_view_model.dart trading_app/test/t0_strategy_view_model_test.dart
git commit -m "feat(flutter): keep blue buy signal first during candidate preview"
```

---

### Task 4: 端到端冒烟

**Files:** 无代码变更

- [ ] **Step 1: 重启 backend**

```bash
export PATH="$PWD/.tools/go/bin:$PATH"
go build -o /tmp/go-stock-server-mac ./cmd/server
# 停旧进程后启动
cd /Users/vb/Projects/go-stock && /tmp/go-stock-server-mac
```

- [ ] **Step 2: 验证 API 顺序**

```bash
curl -s 'http://localhost:8080/api/t0-selection?archived=1&date=2026-08-11' | \
  python3 -c "import json,sys; rs=json.load(sys.stdin)['results']; print([r['股票名称'] for r in rs if r.get('买入信号')=='blue'][:3]); print('first blue idx', next((i for i,r in enumerate(rs) if r.get('买入信号')=='blue'), -1))"
```

Expected: `first blue idx` 为 **0**（若该日有 blue；武汉凡谷 `ZT|ZT|DT` 应为首位）

- [ ] **Step 3: Flutter Web**

```bash
cd trading_app && flutter build web
```

浏览器硬刷新 → 主板策略 → 选 2026-08-11 → 确认蓝点股票在列表最上方

- [ ] **Step 4: Commit**（若有 web 产物纳入部署流程则按项目惯例；默认 **不** commit `build/web`）

---

### Task 5: 更新标记优先排序 spec

**Files:**
- Create: `docs/superpowers/specs/2026-08-25-t0-blue-first-sort-design.md`

- [ ] **Step 1: Write spec**

```markdown
# 主板策略蓝色买入信号优先排序

## 目标
`买入信号=blue` 的股票在所有客户端列表中排在最前。

## 排序键（依次）
1. blue 优先
2. 标记非空优先（沿用 2026-08-12）
3. T0开盘涨幅(%) 降序
4. 稳定排序

## 竞价预览附加键（Flutter，09:15–09:25）
在 blue 之后：有实时行情优先；组内按实时涨幅降序。

## 不变
- 磁盘 selection JSON 写入顺序
- 行内 chip UI
```

- [ ] **Step 2: Commit**

```bash
git add docs/superpowers/specs/2026-08-25-t0-blue-first-sort-design.md
git commit -m "docs: add blue-first sort spec for T0 strategy list"
```

---

## Spec Coverage Checklist

| 需求 | Task |
|------|------|
| blue 全局最前 | Task 1 |
| 标记+涨幅次级规则不变 | Task 1 |
| candidates 预览列表 | Task 2 |
| 09:15–09:25 实时轮询不打乱 blue | Task 3 |
| 归档/正式选股 API | Task 1（已有 `sortT0ResultsForClient` 出口） |
| 不改磁盘 JSON | Task 1 Global Constraints |
| 文档 | Task 5 |

## Self-Review

- **Spec coverage:** 全部需求已映射 Task 1–5；无 TBD。
- **Placeholder scan:** 无占位符；候选集成测试注明 mock 过重时的降级方案。
- **Type consistency:** `BuySignalBlue` / `buySignal == 'blue'` 前后端一致。

## Execution Handoff

Plan complete and saved to `docs/superpowers/plans/2026-08-25-t0-blue-first-sort.md`. Two execution options:

**1. Subagent-Driven (recommended)** — 每 Task 派独立 subagent，Task 间人工/Agent 复核，迭代快

**2. Inline Execution** — 本会话用 executing-plans 按 Task 批量执行并在检查点暂停

**Which approach?**
