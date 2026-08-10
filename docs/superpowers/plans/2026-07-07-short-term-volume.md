# Short Term Volume Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Replace the short-term emotion placeholder volume metric with real market turnover and a deterministic volume score.

**Architecture:** Add a focused `backend/flutter_api/short_term_volume.go` module that fetches EastMoney index quotes, summarizes Shanghai/Shenzhen turnover, formats display text, scores volume, and provides a cache fallback. Wire the result into `GetShortTermEmotion` without changing Flutter models or page layout.

**Tech Stack:** Go, existing `SharedHTTPClient`, existing Flutter API JSON model, current short-term emotion scorer.

## Global Constraints

- Keep new backend business logic under `backend/flutter_api`.
- Do not modify Wails/Vue frontend files.
- Keep scoring deterministic and covered by unit tests.
- Use real-time data when available, cache fallback when live fetch fails, neutral score only as last fallback.

---

### Task 1: Pure Volume Scoring and Formatting

**Files:**
- Create: `backend/flutter_api/short_term_volume.go`
- Modify: `backend/flutter_api/short_term_emotion_test.go`

**Interfaces:**
- Produces: `ShortTermVolume`, `calculateVolumeScore(totalYuan float64, projectedYuan float64) int`, `formatTurnoverText(totalYuan float64, projectedYuan float64, status string) string`

- [x] **Step 1:** Add failing tests for turnover formatting and scoring.
- [x] **Step 2:** Run focused Go tests and confirm they fail before implementation.
- [x] **Step 3:** Implement pure formatting, projection, and scoring helpers.
- [x] **Step 4:** Run focused Go tests and confirm they pass.

### Task 2: Real Quote Fetch and Emotion Wiring

**Files:**
- Modify: `backend/flutter_api/short_term_volume.go`
- Modify: `backend/flutter_api/short_term_emotion.go`
- Modify: `backend/flutter_api/short_term_emotion_test.go`
- Modify: `docs/超短情绪短线避坑说明.md`

**Interfaces:**
- Produces: `GetShortTermVolume(now time.Time, isTrading bool) ShortTermVolume`
- Consumes: `GetShortTermEmotion(isTrading bool)`

- [x] **Step 1:** Add tests for EastMoney quote parsing and cache fallback.
- [x] **Step 2:** Implement quote fetching, parsing, cache fallback, and neutral fallback.
- [x] **Step 3:** Wire `GetShortTermEmotion` to use `GetShortTermVolume`.
- [x] **Step 4:** Update the Chinese user documentation.
- [x] **Step 5:** Run Go tests and manually verify `/api/short-term-emotion`.
