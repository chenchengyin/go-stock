# T0 竞价前候选预览 Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** 预热 ready 后主接口附带当日 `candidates`；Flutter 在 09:15–09:25 按客户端实时行情排序展示全量候选，09:25 后切回正式 0.01%–3% `results`。

**Architecture:** 服务端在非凌晨窗的 prewarm ready 响应中附带与 `candidate_count` 同源的 `candidates` 数组（不拉竞价行情）。客户端 `T0StrategyViewModel` 用显式四态状态机；候选预览态复用 `/api/stock-realtime` 拉涨幅并本地降序排序。

**Tech Stack:** Go (`backend/flutter_api`)、Flutter/Dart (`trading_app`)

## Global Constraints

- 00:00–09:00：可附前日 `results`，**禁止**附当日 `candidates`。
- 任意响应最多一份名单：历史 `results` / `candidates` / 正式 `results`。
- 候选预览的实时涨幅只来自客户端 `/api/stock-realtime`，选股接口不附竞价涨幅。
- ≥09:25 正式路径保持 `filterOpenGap(0.01, 3.0)`，响应不带 `candidates`。
- 文档：`docs/superpowers/specs/2026-08-14-t0-auction-candidate-preview-design.md`

---

## File Map

| File | Responsibility |
|------|----------------|
| `backend/flutter_api/t0_selection.go` | `buildT0CandidateList`；ready 响应附 `candidates`/`phase` |
| `backend/flutter_api/t0_candidates_test.go` | 服务端候选字段与凌晨窗互斥测试 |
| `trading_app/lib/features/radar/presentation/radar_list/t0_strategy_view_model.dart` | 四态状态机、行情轮询、排序 |
| `trading_app/lib/features/radar/presentation/radar_list/radar_page.dart` | 预览 UI 文案/涨幅展示 |
| `trading_app/test/t0_strategy_view_model_test.dart` | 客户端状态机与排序测试 |
| `trading_app/lib/features/radar/data/radar_repository.dart` | 复用 `fetchRealtimeQuotes`（一般不改） |

---

### Task 1: 服务端构建并附带 `candidates`

**Files:**
- Modify: `backend/flutter_api/t0_selection.go`（`buildPrewarmReadyResponseAt`、新增 `buildT0CandidateList` / 条目组装）
- Test: `backend/flutter_api/t0_candidates_test.go`（新建）

**Interfaces:**
- Produces: `buildT0CandidateList(tradeDate string) (candidates []T0SelectionResult, count int, ok bool)`  
  - `ok=false`：无日线缓存  
  - 条目含代码/名称/成交额/昨收/昨涨跌/涨停日期/MA20/标记；开盘/收盘涨幅为 0  
- Produces: ready JSON 在非 `isPreopenPrevResultWindow` 时含 `"phase":"candidates"` 与 `"candidates":[...]`，且 `candidate_count == len(candidates)`

- [ ] **Step 1: 写失败测试 — 非凌晨窗 ready 含 candidates**

在 `t0_candidates_test.go` 写入临时缓存根、写入最小 daily gob（或直接构造 stocks+daily 调内部函数），冻结 `now` 到今日 09:10，断言 `buildPrewarmReadyResponseAt` 返回 map 含非 nil `candidates` 且无 `historical`。

另写：`now` 在 01:00 且能找到前日归档时，响应有 `historical`/`results` 且**无** `candidates` 键（或为 nil）。

- [ ] **Step 2: 跑测试确认失败**

Run: `go test ./backend/flutter_api -run 'TestBuildPrewarmReady.*Candidate|TestPreopenWindowOmitsCandidates' -count=1 -v`  
Expected: FAIL（函数/字段尚不存在或未附带）

- [ ] **Step 3: 实现 `buildT0CandidateList` 并挂到 ready 响应**

```go
// 伪代码
func buildT0CandidateList(tradeDate string) ([]T0SelectionResult, bool) {
  cached, ok := loadT0DailyCache(tradeDate)
  if !ok { return nil, false }
  // hist + filterLimitUpRecent + filterTurnover
  // 组装 T0SelectionResult（OpenGap/CloseRet=0，Tag 用 pickPrevDayTag）
}
```

在 `buildPrewarmReadyResponseAt`：若 `isPreopenPrevResultWindow` 则保持现状；否则若 list, ok := buildT0CandidateList；ok 则 `resp["candidates"]=list`，`resp["phase"]="candidates"`，`candidate_count=len(list)`。

确保正式 `handleT0Selection` 成功分支不写 `candidates`。

- [ ] **Step 4: 跑测试确认通过**

Run: `go test ./backend/flutter_api -run 'TestBuildPrewarmReady.*Candidate|TestPreopenWindowOmitsCandidates|TestHandleT0SelectionArchived' -count=1`  
Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add backend/flutter_api/t0_selection.go backend/flutter_api/t0_candidates_test.go
git commit -m "$(cat <<'EOF'
feat(t0): attach prewarm candidates after morning window

EOF
)"
```

---

### Task 2: 客户端状态机与候选预览排序

**Files:**
- Modify: `trading_app/lib/features/radar/presentation/radar_list/t0_strategy_view_model.dart`
- Modify: `trading_app/test/t0_strategy_view_model_test.dart`
- Modify: `trading_app/lib/features/radar/presentation/radar_list/radar_page.dart`（预览文案/展示涨幅）

**Interfaces:**
- Consumes: 响应字段 `candidates`、`phase`、`prewarm`、`results`、`historical`
- Consumes: `RadarRepository.fetchRealtimeQuotes(List<String> codes)` → `changePercent`
- Produces: UI 态 `historical|waiting|candidatePreview|confirmed`；`candidatePreview` 下列表按实时涨幅降序

- [ ] **Step 1: 写失败测试 — 时钟与 candidates**

在 `t0_strategy_view_model_test.dart`：

1. 注入/可测的「现在」时间（若尚无 clock，为 ViewModel 增加可选 `DateTime Function()? now` 或 `@visibleForTesting setNowForTest`）。
2. now=09:10 + ready 含 `candidates` → 不进入列表预览（waiting），`results` 展示为空或仍非候选列表。
3. now=09:20 + 同响应 → 进入 candidatePreview，列表条数 = candidates。
4. mock 行情后排序：涨幅高的在前。
5. now=09:25 + 正式无 prewarm 的 `results` → confirmed，停止候选轮询标志为 true。

- [ ] **Step 2: 跑测试确认失败**

Run: `cd trading_app && flutter test test/t0_strategy_view_model_test.dart`  
Expected: FAIL

- [ ] **Step 3: 实现状态机 + 行情轮询**

- 解析 `candidates` 为 `T0StrategyStock`（可复用 fromJson；预览态用可变的实时涨幅字段或并行 `Map<code, double>`）。
- `candidatePreview`：对 `rawCode` 调 `fetchRealtimeQuotes`，写回展示涨幅，`sort` 降序；Timer 约 10s。
- ≥09:25：`loadResults()` 走正式路径；cancel 候选行情 Timer。
- &lt;09:15：即使有 `candidates` 也只更新 `warmProgress.candidateCount`，不 `notify` 成列表态。

- [ ] **Step 4: UI 微调**

`radar_page.dart` 策略 Tab：`candidatePreview` 时卡片主涨幅用实时涨幅，副文案「竞价预览（未确认）」；waiting ready 保持候选数量提示。

- [ ] **Step 5: 跑测试确认通过**

Run: `cd trading_app && flutter test test/t0_strategy_view_model_test.dart`  
Expected: PASS

- [ ] **Step 6: Commit**

```bash
git add trading_app/lib/features/radar/presentation/radar_list/t0_strategy_view_model.dart \
  trading_app/lib/features/radar/presentation/radar_list/radar_page.dart \
  trading_app/test/t0_strategy_view_model_test.dart
git commit -m "$(cat <<'EOF'
feat(app): preview T0 candidates with live quote sort 09:15–09:25

EOF
)"
```

---

### Task 3: 冒烟与收尾

**Files:**
- 一般无新文件；必要时更新 `docs/superpowers/specs/2026-08-14-t0-auction-candidate-preview-design.md` 若实现有小偏差

- [ ] **Step 1: 跑相关 Go 测试包**

Run: `go test ./backend/flutter_api -count=1`  
Expected: PASS（或仅跳过需网络的 Integration）

- [ ] **Step 2: 跑 Flutter T0 相关测试**

Run: `cd trading_app && flutter test test/t0_strategy_view_model_test.dart`  
Expected: PASS

- [ ] **Step 3: 手动核对清单（实现者自检）**

- 凌晨响应：有历史 results 时无 candidates  
- 09:10 ready：有 candidates，UI 仍等待  
- 09:20：列表出现且随行情重排  
- 09:26：仅 0.01%–3% 正式结果  

- [ ] **Step 4: Commit 若有文档/小修**

```bash
git status
# 若有改动再 commit；否则跳过
```

---

## 执行方式

实现时按 Task 1 → 2 → 3 顺序；每 Task 内 TDD。推荐 `superpowers:subagent-driven-development` 逐任务推进，或 `superpowers:executing-plans` 同会话执行。
