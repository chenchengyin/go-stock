# Short-Term Granville Phase Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Upgrade the short-term emotion `phase` from current-score-only labels to a Granville-style phase model using current score, previous trading day close score, and score direction.

**Architecture:** Keep the current score calculation unchanged, then add previous-close context into the pure phase classifier. Previous close score is derived from the latest `market_statistic` row before the current data date using stored breadth and limit-quality data. The Flutter model and endpoint response shape remain unchanged; the App still reads `phase`, but the value becomes more precise.

**Tech Stack:** Go backend, SQLite/GORM `market_statistic`, existing `/api/short-term-emotion`, Go tests.

## Global Constraints

- Do not change Flutter response model fields.
- Do not add a new frontend page.
- Preserve existing component scores and weights.
- Phase naming may use previous close context, but score calculation must remain current-day based.
- If previous-day context is missing, preserve conservative fallback labels.
- Use TDD: failing tests first, implementation second.
- Do not delete or rewrite existing market statistic data.

---

## File Structure

- Modify: `backend/flutter_api/short_term_emotion.go`
  - Add `PreviousCloseScore *int` to `ShortTermEmotionInput`.
  - Replace current `classifyShortTermEmotion(...)` with a Granville-style classifier.
  - Add helpers for previous close score resolution.
- Modify: `backend/flutter_api/short_term_emotion_test.go`
  - Add phase tests for `退潮初期`, `退潮`, `冰点`, `冰点修复`, and missing-context fallback.
  - Add previous close resolver tests.
- Modify: `backend/data/market_statistic_api.go`
  - Add query helper for latest rows before the current data date.

## Phase Rules

Core thresholds:

```go
const shortTermLowPhaseScore = 35
const shortTermRepairPhaseScore = 55
const shortTermRetreatDrop = 10
```

Granville-style interpretation:

```text
当前 < 35:
  昨收 < 35          => 冰点
  昨收 >= 35         => 退潮
  昨收缺失           => 冰点/退潮

当前 35-55:
  昨收 < 35          => 冰点修复
  昨收 >= 55 且 当前 <= 昨收 - 10 => 退潮初期
  其他               => 偏弱

当前 >= 55:
  保留原有 弱修复 / 活跃偏分歧 / 活跃 / 高潮分歧
```

Special high-risk case:

```go
limitDown >= 20 && score < 55
```

This still forces action/risk/weight to:

```text
谨慎观察 / 高 / 0成
```

But phase name still follows the Granville context:

```text
当前 < 35: 退潮 or 冰点
当前 35-55 且昨收低: 冰点修复, but risk remains 高 and weight 0成
当前 35-55 且昨收高且大幅下滑: 退潮初期, risk 高 and weight 0成
```

---

### Task 1: Pure Granville Phase Classifier

**Files:**
- Modify: `backend/flutter_api/short_term_emotion.go`
- Modify: `backend/flutter_api/short_term_emotion_test.go`

**Interfaces:**
- Consumes: `score int`, `highRisk bool`, `limitDown int`, `previousCloseScore *int`
- Produces: `classifyShortTermEmotion(score int, highRisk bool, limitDown int, previousCloseScore *int) (string, string, string, string)`
- Produces: `resolveGranvillePhase(score int, previousCloseScore *int) string`

- [ ] **Step 1: Write failing phase tests**

Append to `backend/flutter_api/short_term_emotion_test.go`:

```go
func TestClassifyShortTermEmotionGranvillePhases(t *testing.T) {
	lowPrevious := 28
	weakPrevious := 42
	healthyPrevious := 62

	tests := []struct {
		name               string
		score              int
		limitDown          int
		previousCloseScore *int
		wantPhase          string
		wantAction         string
		wantRisk           string
		wantWeight         string
	}{
		{
			name:               "low after healthy previous close is retreat",
			score:              29,
			limitDown:          12,
			previousCloseScore: &healthyPrevious,
			wantPhase:          "退潮",
			wantAction:         "谨慎观察",
			wantRisk:           "高",
			wantWeight:         "0成",
		},
		{
			name:               "low after low previous close is ice point",
			score:              29,
			limitDown:          12,
			previousCloseScore: &lowPrevious,
			wantPhase:          "冰点",
			wantAction:         "谨慎观察",
			wantRisk:           "高",
			wantWeight:         "0成",
		},
		{
			name:               "low without previous context keeps fallback",
			score:              29,
			limitDown:          12,
			previousCloseScore: nil,
			wantPhase:          "冰点/退潮",
			wantAction:         "谨慎观察",
			wantRisk:           "高",
			wantWeight:         "0成",
		},
		{
			name:               "low previous close and weak current is ice repair",
			score:              44,
			limitDown:          8,
			previousCloseScore: &lowPrevious,
			wantPhase:          "冰点修复",
			wantAction:         "谨慎观察",
			wantRisk:           "中等偏高",
			wantWeight:         "0-2成",
		},
		{
			name:               "healthy previous close and weak current with large drop is early retreat",
			score:              48,
			limitDown:          8,
			previousCloseScore: &healthyPrevious,
			wantPhase:          "退潮初期",
			wantAction:         "谨慎观察",
			wantRisk:           "中等偏高",
			wantWeight:         "0-2成",
		},
		{
			name:               "weak current without large drop remains weak",
			score:              44,
			limitDown:          8,
			previousCloseScore: &weakPrevious,
			wantPhase:          "偏弱",
			wantAction:         "谨慎观察",
			wantRisk:           "中等偏高",
			wantWeight:         "0-2成",
		},
		{
			name:               "limit down expansion keeps high risk but uses retreat phase",
			score:              48,
			limitDown:          25,
			previousCloseScore: &healthyPrevious,
			wantPhase:          "退潮初期",
			wantAction:         "谨慎观察",
			wantRisk:           "高",
			wantWeight:         "0成",
		},
		{
			name:               "non-low phase keeps original repair label",
			score:              58,
			limitDown:          4,
			previousCloseScore: &lowPrevious,
			wantPhase:          "弱修复",
			wantAction:         "轻仓试错",
			wantRisk:           "中",
			wantWeight:         "1-3成",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			phase, action, riskLevel, suggestedWeight := classifyShortTermEmotion(
				tt.score,
				false,
				tt.limitDown,
				tt.previousCloseScore,
			)
			if phase != tt.wantPhase {
				t.Fatalf("expected phase %s, got %s", tt.wantPhase, phase)
			}
			if action != tt.wantAction {
				t.Fatalf("expected action %s, got %s", tt.wantAction, action)
			}
			if riskLevel != tt.wantRisk {
				t.Fatalf("expected risk %s, got %s", tt.wantRisk, riskLevel)
			}
			if suggestedWeight != tt.wantWeight {
				t.Fatalf("expected weight %s, got %s", tt.wantWeight, suggestedWeight)
			}
		})
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run:

```bash
go test ./backend/flutter_api -run TestClassifyShortTermEmotionGranvillePhases -v
```

Expected:

```text
too many arguments in call to classifyShortTermEmotion
```

- [ ] **Step 3: Implement classifier constants and input field**

Modify `backend/flutter_api/short_term_emotion.go`:

```go
const (
	shortTermLowPhaseScore    = 35
	shortTermRepairPhaseScore = 55
	shortTermRetreatDrop      = 10
)
```

Add to `ShortTermEmotionInput`:

```go
PreviousCloseScore *int
```

- [ ] **Step 4: Implement Granville phase resolver**

Add to `backend/flutter_api/short_term_emotion.go`:

```go
func resolveGranvillePhase(score int, previousCloseScore *int) string {
	if score < shortTermLowPhaseScore {
		if previousCloseScore == nil {
			return "冰点/退潮"
		}
		if *previousCloseScore < shortTermLowPhaseScore {
			return "冰点"
		}
		return "退潮"
	}

	if score < shortTermRepairPhaseScore {
		if previousCloseScore != nil {
			if *previousCloseScore < shortTermLowPhaseScore {
				return "冰点修复"
			}
			if *previousCloseScore >= shortTermRepairPhaseScore &&
				score <= *previousCloseScore-shortTermRetreatDrop {
				return "退潮初期"
			}
		}
		return "偏弱"
	}

	return ""
}
```

- [ ] **Step 5: Replace classifier**

Replace `classifyShortTermEmotion(...)` in `backend/flutter_api/short_term_emotion.go`:

```go
func classifyShortTermEmotion(score int, highRisk bool, limitDown int, previousCloseScore *int) (string, string, string, string) {
	if limitDown >= 20 && score < shortTermRepairPhaseScore {
		return resolveGranvillePhase(score, previousCloseScore), "谨慎观察", "高", "0成"
	}
	switch {
	case score >= 75 && highRisk:
		return "高潮分歧", "谨慎追高", "中等偏高", "2-3成"
	case score >= 75:
		return "活跃", "正常参与", "中", "3-5成"
	case score >= 55 && highRisk:
		return "活跃偏分歧", "轻仓参与", "中等偏高", "2-4成"
	case score >= 55:
		return "弱修复", "轻仓试错", "中", "1-3成"
	default:
		return resolveGranvillePhase(score, previousCloseScore), "谨慎观察", "中等偏高", "0-2成"
	}
}
```

Then add a guard so true low scores always get high risk:

```go
phase := resolveGranvillePhase(score, previousCloseScore)
if score < shortTermLowPhaseScore {
	return phase, "谨慎观察", "高", "0成"
}
return phase, "谨慎观察", "中等偏高", "0-2成"
```

The final `default` block should be:

```go
default:
	phase := resolveGranvillePhase(score, previousCloseScore)
	if score < shortTermLowPhaseScore {
		return phase, "谨慎观察", "高", "0成"
	}
	return phase, "谨慎观察", "中等偏高", "0-2成"
}
```

Update call site:

```go
phase, action, riskLevel, suggestedWeight := classifyShortTermEmotion(score, highRisk, input.LimitDown, input.PreviousCloseScore)
```

- [ ] **Step 6: Run classifier tests**

Run:

```bash
go test ./backend/flutter_api -run TestClassifyShortTermEmotionGranvillePhases -v
```

Expected:

```text
PASS
```

---

### Task 2: Resolve Previous Trading Day Close Score

**Files:**
- Modify: `backend/data/market_statistic_api.go`
- Modify: `backend/flutter_api/short_term_emotion.go`
- Modify: `backend/flutter_api/short_term_emotion_test.go`

**Interfaces:**
- Produces: `func (a *MarketStatisticApi) GetLatestBeforeDate(date string) []models.MarketStatistic`
- Produces: `func resolvePreviousCloseScore(stats []models.MarketStatistic, currentDate string) *int`
- Produces: `func scoreMarketStatisticClose(stat models.MarketStatistic) int`

- [ ] **Step 1: Write failing resolver tests**

Append to `backend/flutter_api/short_term_emotion_test.go`:

```go
func TestResolvePreviousCloseScoreUsesLatestPreviousTradingDayClose(t *testing.T) {
	stats := []models.MarketStatistic{
		{
			DataDate:   "2026-07-06",
			DataTime:   "14:59",
			UpCount:    3200,
			DownCount:  1600,
			UpRatio:    66.7,
			LimitUp:    80,
			LimitDown:  8,
			LimitRatio: 10,
		},
		{
			DataDate:   "2026-07-06",
			DataTime:   "15:00",
			UpCount:    3400,
			DownCount:  1400,
			UpRatio:    70.8,
			LimitUp:    92,
			LimitDown:  6,
			LimitRatio: 15.3,
		},
		{
			DataDate:   "2026-07-07",
			DataTime:   "09:30",
			UpCount:    1200,
			DownCount:  3800,
			UpRatio:    24,
			LimitUp:    15,
			LimitDown:  40,
			LimitRatio: 0.4,
		},
	}

	score := resolvePreviousCloseScore(stats, "2026-07-07")
	if score == nil {
		t.Fatal("expected previous close score")
	}
	if *score < shortTermRepairPhaseScore {
		t.Fatalf("expected previous close score to be non-low, got %d", *score)
	}
}

func TestResolvePreviousCloseScoreIgnoresCurrentDateRows(t *testing.T) {
	stats := []models.MarketStatistic{
		{
			DataDate:   "2026-07-08",
			DataTime:   "15:00",
			UpCount:    3500,
			DownCount:  1200,
			UpRatio:    74.5,
			LimitUp:    100,
			LimitDown:  5,
			LimitRatio: 20,
		},
	}

	score := resolvePreviousCloseScore(stats, "2026-07-08")
	if score != nil {
		t.Fatalf("expected nil previous close score, got %d", *score)
	}
}

func TestResolvePreviousCloseScoreReturnsLowForWeakPreviousClose(t *testing.T) {
	stats := []models.MarketStatistic{
		{
			DataDate:   "2026-07-07",
			DataTime:   "15:00",
			UpCount:    900,
			DownCount:  4200,
			UpRatio:    17.6,
			LimitUp:    12,
			LimitDown:  55,
			LimitRatio: 0.2,
		},
	}

	score := resolvePreviousCloseScore(stats, "2026-07-08")
	if score == nil {
		t.Fatal("expected previous close score")
	}
	if *score >= shortTermLowPhaseScore {
		t.Fatalf("expected low previous close score, got %d", *score)
	}
}
```

- [ ] **Step 2: Run resolver tests to verify they fail**

Run:

```bash
go test ./backend/flutter_api -run 'TestResolvePreviousCloseScore' -v
```

Expected:

```text
undefined: resolvePreviousCloseScore
```

- [ ] **Step 3: Implement previous close resolver**

Add to `backend/flutter_api/short_term_emotion.go`:

```go
func resolvePreviousCloseScore(stats []models.MarketStatistic, currentDate string) *int {
	if len(stats) == 0 {
		return nil
	}

	var selected *models.MarketStatistic
	for i := range stats {
		stat := stats[i]
		if strings.TrimSpace(stat.DataDate) == "" || stat.DataDate >= currentDate {
			continue
		}
		if selected == nil ||
			stat.DataDate > selected.DataDate ||
			(stat.DataDate == selected.DataDate && stat.DataTime > selected.DataTime) {
			selected = &stat
		}
	}
	if selected == nil {
		return nil
	}

	score := scoreMarketStatisticClose(*selected)
	return &score
}

func scoreMarketStatisticClose(stat models.MarketStatistic) int {
	widthScore := calculateWidthScore(stat.UpRatio)
	limitScore := calculateLimitScore(stat.LimitUp, stat.LimitDown, 0)
	components := []ShortTermEmotionComponent{
		{Name: "宽度情绪", Score: widthScore, Weight: 50},
		{Name: "涨跌停质量", Score: limitScore, Weight: 50},
	}
	return weightedComponentScore(components)
}
```

- [ ] **Step 4: Add data query helper**

Add to `backend/data/market_statistic_api.go`:

```go
func (a *MarketStatisticApi) GetLatestBeforeDate(date string) []models.MarketStatistic {
	var data []models.MarketStatistic
	db.Dao.
		Where("data_date < ?", date).
		Order("data_date DESC, data_time DESC").
		Limit(240).
		Find(&data)
	return data
}
```

- [ ] **Step 5: Wire resolver into `GetShortTermEmotion`**

Modify `backend/flutter_api/short_term_emotion.go`:

```go
marketApi := data.NewMarketStatisticApi()
stats := marketApi.GetTodayData()
```

After:

```go
dataDate := resolveShortTermEmotionDataDate(now, stats)
```

Add:

```go
previousCloseScore := resolvePreviousCloseScore(marketApi.GetLatestBeforeDate(dataDate), dataDate)
```

Pass:

```go
PreviousCloseScore: previousCloseScore,
```

- [ ] **Step 6: Run resolver and classifier tests**

Run:

```bash
go test ./backend/flutter_api -run 'TestClassifyShortTermEmotionGranvillePhases|TestResolvePreviousCloseScore' -v
```

Expected:

```text
PASS
```

---

### Task 3: Verification And Backend Restart

**Files:**
- Modify: `backend/flutter_api/short_term_emotion.go`
- Modify: `backend/flutter_api/short_term_emotion_test.go`
- Modify: `backend/data/market_statistic_api.go`

**Interfaces:**
- Consumes: `/api/short-term-emotion`
- Produces: `phase` can now be `退潮初期`, `退潮`, `冰点`, `冰点修复`, or existing labels.

- [ ] **Step 1: Format Go files**

Run:

```bash
gofmt -w backend/flutter_api/short_term_emotion.go backend/flutter_api/short_term_emotion_test.go backend/data/market_statistic_api.go
```

Expected:

```text
no output
```

- [ ] **Step 2: Run final focused tests**

Run:

```bash
go test ./backend/flutter_api -run 'TestClassifyShortTermEmotionGranvillePhases|TestResolvePreviousCloseScore|TestCalculateShortTermEmotion|TestCalculateWidthScore|TestCalculateLimitScore|TestBuildShortTermEmotionEmpty' -v
```

Expected:

```text
PASS
```

- [ ] **Step 3: Build backend server**

Run:

```bash
go build -o /tmp/go-stock-flutter-api-server ./cmd/server
```

Expected:

```text
exit 0
```

- [ ] **Step 4: Restart local backend**

Run:

```bash
lsof -tiTCP:8080 -sTCP:LISTEN | xargs kill
python3 -c "import subprocess; log=open('/tmp/go-stock-flutter-api.log','ab', buffering=0); p=subprocess.Popen(['/tmp/go-stock-flutter-api-server'], cwd='/Users/Zhuanz/aiproject/go-stock', stdin=subprocess.DEVNULL, stdout=log, stderr=log, start_new_session=True); print(p.pid)"
```

Expected:

```text
prints new PID
```

- [ ] **Step 5: Verify API phase**

Run:

```bash
curl -s --max-time 5 http://127.0.0.1:8080/api/short-term-emotion | python3 -c "import json,sys; d=json.load(sys.stdin); print(d['score'], d['phase'], d['action'], d['riskLevel']); print(d['explanation'])"
```

Expected examples:

```text
low today + healthy previous close => 退潮
low today + low previous close => 冰点
weak today + low previous close => 冰点修复
weak today + healthy previous close + large drop => 退潮初期
```

- [ ] **Step 6: App verification**

Open Flutter App `超短情绪` Tab and refresh.

Expected:

```text
市场情绪分卡片右侧 phase shows a more precise Granville-style stage instead of always showing 冰点/退潮.
```

---

## Self-Review

**Spec coverage:** The plan covers the original `冰点/退潮` split and the follow-up Granville refinement: `退潮初期`, `退潮`, `冰点`, and `冰点修复`.

**Placeholder scan:** No TBD/TODO placeholders remain. All code snippets use concrete function names and exact paths.

**Type consistency:** `PreviousCloseScore *int` is defined on `ShortTermEmotionInput`, consumed by `classifyShortTermEmotion(...)`, and produced by `resolvePreviousCloseScore(...)`.

**Important limitation:** The repository did not previously store exact historical full emotion scores. This plan derives previous close context from saved `market_statistic` close rows using breadth and limit quality. A future task can persist full daily `ShortTermEmotion` snapshots for exact historical phase replay.
