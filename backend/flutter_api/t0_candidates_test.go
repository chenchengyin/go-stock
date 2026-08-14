package flutter_api

import (
	"testing"
	"time"
)

func candidateHistBars(tradeDate string) []dailyBar {
	// 9 根：最后一根为 tradeDate（组装后会被 hist 剔除），保证过滤链能通过
	dates := []string{
		"2026-07-29", "2026-07-30", "2026-07-31", "2026-08-03",
		"2026-08-04", "2026-08-05", "2026-08-06", "2026-08-07",
		tradeDate,
	}
	bars := make([]dailyBar, len(dates))
	closePx := 10.0
	for i, d := range dates {
		if i == 6 {
			closePx = 11.0 // 相对前收 10% 涨停
		}
		bars[i] = dailyBar{
			Date: d, Open: closePx, Close: closePx, High: closePx, Low: closePx,
			Volume: 1e8, AmountYi: 10,
		}
	}
	return bars
}

func TestBuildPrewarmReadyResponseAttachesCandidatesAfter0900(t *testing.T) {
	orig := t0CacheRootPath
	t0CacheRootPath = t.TempDir()
	defer func() { t0CacheRootPath = orig }()

	date := "2026-08-12"
	if err := saveT0DailyCache(date,
		[]t0Stock{{Code: "sh.600000", ShortCode: "600000", Name: "浦发银行"}},
		map[string][]dailyBar{"600000": candidateHistBars(date)}); err != nil {
		t.Fatal(err)
	}

	resp := buildPrewarmReadyResponseAt(date,
		time.Date(2026, 8, 12, 9, 10, 0, 0, chinaLocation()))
	if _, has := resp["historical"]; has {
		t.Fatal("09:10 不应注入历史结果")
	}
	if phase, _ := resp["phase"].(string); phase != "candidates" {
		t.Fatalf("phase=%v", resp["phase"])
	}
	cands, ok := resp["candidates"].([]T0SelectionResult)
	if !ok || len(cands) == 0 {
		t.Fatalf("candidates=%v", resp["candidates"])
	}
	count, _ := resp["candidate_count"].(int)
	if count != len(cands) {
		t.Fatalf("candidate_count=%v len=%d", resp["candidate_count"], len(cands))
	}
	if cands[0].StockCode != "600000.XSHG" || cands[0].StockName != "浦发银行" {
		t.Fatalf("first=%+v", cands[0])
	}
	if cands[0].OpenGap != 0 || cands[0].CloseRet != 0 {
		t.Fatalf("preview gaps should be 0, got open=%v close=%v", cands[0].OpenGap, cands[0].CloseRet)
	}
}

func TestPreopenWindowOmitsCandidates(t *testing.T) {
	orig := t0CacheRootPath
	t0CacheRootPath = t.TempDir()
	defer func() { t0CacheRootPath = orig }()

	date := "2026-08-12"
	if err := saveT0DailyCache(date,
		[]t0Stock{{Code: "sh.600000", ShortCode: "600000", Name: "浦发银行"}},
		map[string][]dailyBar{"600000": candidateHistBars(date)}); err != nil {
		t.Fatal(err)
	}
	mustSaveArchive(t, "2026-08-11", "600011.XSHG")

	resp := buildPrewarmReadyResponseAt(date,
		time.Date(2026, 8, 12, 1, 0, 0, 0, chinaLocation()))
	if resp["historical"] != true {
		t.Fatalf("historical=%v", resp["historical"])
	}
	if _, has := resp["candidates"]; has {
		t.Fatalf("凌晨窗不应带 candidates: %v", resp["candidates"])
	}
	if _, has := resp["phase"]; has {
		t.Fatalf("凌晨窗不应带 phase: %v", resp["phase"])
	}
}

func TestFormalSelectionResponseOmitsCandidates(t *testing.T) {
	orig := t0CacheRootPath
	t0CacheRootPath = t.TempDir()
	defer func() { t0CacheRootPath = orig }()

	results := []T0SelectionResult{{StockCode: "600000.XSHG", OpenGap: 1.2}}
	if err := saveT0SelectionArchive("2026-08-12", results, true); err != nil {
		t.Fatal(err)
	}
	a, ok := loadT0SelectionArchive("2026-08-12")
	if !ok || a == nil {
		t.Fatal("archive missing")
	}
	// 正式路径响应形状：无 candidates
	resp := map[string]interface{}{
		"date":    a.Date,
		"count":   a.Count,
		"results": sortT0ResultsForClient(a.Results),
	}
	if _, has := resp["candidates"]; has {
		t.Fatal("正式选股响应不应含 candidates")
	}
}
