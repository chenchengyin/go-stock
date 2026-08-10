package flutter_api

import (
	"errors"
	"go-stock/backend/data"
	"go-stock/backend/models"
	"testing"
	"time"
)

func TestCalculateShortTermEmotionActiveWithRisk(t *testing.T) {
	now := time.Date(2026, 7, 7, 14, 32, 18, 0, time.FixedZone("CST", 8*3600))

	result := CalculateShortTermEmotion(ShortTermEmotionInput{
		Now:              now,
		IsTrading:        true,
		UpCount:          3186,
		DownCount:        1742,
		LimitUp:          72,
		LimitDown:        6,
		BreakLimitCount:  37,
		SealedLimitCount: 72,
		MaxBoard:         6,
		Board3PlusCount:  9,
		BullishChanges:   428,
		BearishChanges:   191,
		MainTheme:        "机器人",
		MainThemeScore:   74,
		TurnoverText:     "实时成交额待接入",
		VolumeScore:      50,
	})

	if result.Phase != "活跃偏分歧" {
		t.Fatalf("expected phase 活跃偏分歧, got %s", result.Phase)
	}
	if result.Action != "轻仓参与" {
		t.Fatalf("expected action 轻仓参与, got %s", result.Action)
	}
	if result.RiskLevel != "中等偏高" {
		t.Fatalf("expected risk level 中等偏高, got %s", result.RiskLevel)
	}
	if result.Score < 55 || result.Score > 74 {
		t.Fatalf("expected score between 55 and 74, got %d", result.Score)
	}
	if len(result.Components) != 6 {
		t.Fatalf("expected 6 components, got %d", len(result.Components))
	}
	if len(result.Dashboard) == 0 {
		t.Fatal("expected dashboard metrics")
	}
	if len(result.RiskSignals) == 0 {
		t.Fatal("expected risk signals")
	}
}

func TestCalculateShortTermEmotionIcePoint(t *testing.T) {
	now := time.Date(2026, 7, 7, 10, 10, 0, 0, time.FixedZone("CST", 8*3600))

	result := CalculateShortTermEmotion(ShortTermEmotionInput{
		Now:              now,
		IsTrading:        true,
		UpCount:          650,
		DownCount:        4200,
		LimitUp:          18,
		LimitDown:        35,
		BreakLimitCount:  45,
		SealedLimitCount: 18,
		MaxBoard:         3,
		Board3PlusCount:  1,
		BullishChanges:   120,
		BearishChanges:   380,
		MainTheme:        "无明显主线",
		MainThemeScore:   20,
		TurnoverText:     "实时成交额待接入",
		VolumeScore:      40,
	})

	if result.Score > 35 {
		t.Fatalf("expected score <= 35, got %d", result.Score)
	}
	if result.Action != "谨慎观察" {
		t.Fatalf("expected action 谨慎观察, got %s", result.Action)
	}
	if result.RiskLevel != "高" {
		t.Fatalf("expected risk level 高, got %s", result.RiskLevel)
	}
}

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

func TestCalculateWidthScoreKeepsLowBreadthVisibleAndCapsStrongBreadth(t *testing.T) {
	tests := []struct {
		name    string
		upRatio float64
		want    int
	}{
		{name: "极弱宽度接近冰点", upRatio: 20, want: 12},
		{name: "低红盘率明显偏弱", upRatio: 30, want: 25},
		{name: "四成红盘偏弱但不归零", upRatio: 40, want: 40},
		{name: "五五开中性", upRatio: 50, want: 50},
		{name: "六成红盘温和偏强", upRatio: 60, want: 60},
		{name: "七成红盘加速转强", upRatio: 70, want: 75},
		{name: "八成红盘接近满分", upRatio: 80, want: 90},
		{name: "八十五以上满分", upRatio: 85, want: 100},
		{name: "极强宽度封顶", upRatio: 92, want: 100},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := calculateWidthScore(tt.upRatio)
			if got != tt.want {
				t.Fatalf("expected score %d, got %d", tt.want, got)
			}
		})
	}
}

func TestCalculateLimitScoreKeepsDivergenceVisible(t *testing.T) {
	tests := []struct {
		name      string
		limitUp   int
		limitDown int
		breakRate float64
		want      int
	}{
		{name: "无涨跌停中性偏低", limitUp: 0, limitDown: 0, breakRate: 0, want: 25},
		{name: "跌停扩散但涨停不少", limitUp: 55, limitDown: 36, breakRate: 10, want: 40},
		{name: "涨跌停接近均衡", limitUp: 40, limitDown: 40, breakRate: 12, want: 30},
		{name: "涨停明显占优", limitUp: 70, limitDown: 18, breakRate: 8, want: 67},
		{name: "强涨停生态", limitUp: 100, limitDown: 5, breakRate: 5, want: 95},
		{name: "跌停远多于涨停", limitUp: 15, limitDown: 60, breakRate: 25, want: 0},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := calculateLimitScore(tt.limitUp, tt.limitDown, tt.breakRate)
			if got != tt.want {
				t.Fatalf("expected score %d, got %d", tt.want, got)
			}
		})
	}
}

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

func TestBuildShortTermEmotionEmpty(t *testing.T) {
	now := time.Date(2026, 7, 7, 9, 0, 0, 0, time.FixedZone("CST", 8*3600))

	result := BuildShortTermEmotionEmpty(now, "暂无市场统计数据")

	if result.Score != 0 {
		t.Fatalf("expected score 0, got %d", result.Score)
	}
	if result.Phase != "数据不足" {
		t.Fatalf("expected phase 数据不足, got %s", result.Phase)
	}
	if result.Action != "谨慎观察" {
		t.Fatalf("expected action 谨慎观察, got %s", result.Action)
	}
	if result.Explanation == "" {
		t.Fatal("expected explanation")
	}
}

func TestNormalizeUplimitHot(t *testing.T) {
	raw := map[string]any{
		"code": float64(20000),
		"data": map[string]any{
			"max_count": float64(6),
			"plate": []any{
				[]any{"机器人", "BK1234", float64(74)},
				[]any{"AI算力", "BK5678", float64(52)},
			},
			"ban_info": map[string]any{
				"1": map[string]any{"count": float64(28)},
				"2": map[string]any{"count": float64(10)},
				"3": map[string]any{"count": float64(4)},
				"4": map[string]any{"count": float64(3)},
				"5": map[string]any{"count": float64(1)},
			},
			"stocks": "000001,000002,000003",
		},
	}

	result := normalizeUplimitHot(raw)

	if result.MainTheme != "机器人" {
		t.Fatalf("expected main theme 机器人, got %s", result.MainTheme)
	}
	if result.MainThemeScore != 74 {
		t.Fatalf("expected main theme score 74, got %d", result.MainThemeScore)
	}
	if result.MaxBoard != 6 {
		t.Fatalf("expected max board 6, got %d", result.MaxBoard)
	}
	if result.Board3PlusCount != 8 {
		t.Fatalf("expected 3+ board count 8, got %d", result.Board3PlusCount)
	}
	if result.SealedLimitCount != 3 {
		t.Fatalf("expected sealed limit count 3, got %d", result.SealedLimitCount)
	}
}

func TestSummarizeStockChanges(t *testing.T) {
	resp := &data.StockChangesResponse{
		Data: []data.StockChangeItem{
			{TypeName: "火箭发射"},
			{TypeName: "封涨停板"},
			{TypeName: "打开涨停板"},
			{TypeName: "高台跳水"},
			{TypeName: "大笔卖出"},
		},
	}

	result := summarizeStockChanges(resp)

	if result.BullishChanges != 2 {
		t.Fatalf("expected 2 bullish changes, got %d", result.BullishChanges)
	}
	if result.BearishChanges != 2 {
		t.Fatalf("expected 2 bearish changes, got %d", result.BearishChanges)
	}
	if result.BreakLimitCount != 1 {
		t.Fatalf("expected 1 break limit, got %d", result.BreakLimitCount)
	}
}

func TestFormatTurnoverText(t *testing.T) {
	text := formatTurnoverText(9800_0000_0000, 12600_0000_0000, "实时")

	if text != "实时 两市0.98万亿 / 预估全天1.26万亿" {
		t.Fatalf("unexpected turnover text: %s", text)
	}
}

func TestCalculateVolumeScore(t *testing.T) {
	tests := []struct {
		name          string
		totalYuan     float64
		projectedYuan float64
		want          int
	}{
		{name: "缩量", totalYuan: 6200_0000_0000, projectedYuan: 6500_0000_0000, want: 35},
		{name: "中性", totalYuan: 8200_0000_0000, projectedYuan: 8500_0000_0000, want: 50},
		{name: "放量", totalYuan: 9800_0000_0000, projectedYuan: 10800_0000_0000, want: 65},
		{name: "强放量", totalYuan: 11200_0000_0000, projectedYuan: 12800_0000_0000, want: 78},
		{name: "极强量", totalYuan: 13200_0000_0000, projectedYuan: 13800_0000_0000, want: 90},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := calculateVolumeScore(tt.totalYuan, tt.projectedYuan)
			if got != tt.want {
				t.Fatalf("expected score %d, got %d", tt.want, got)
			}
		})
	}
}

func TestProjectedTurnoverYuan(t *testing.T) {
	now := time.Date(2026, 7, 7, 10, 30, 0, 0, time.FixedZone("CST", 8*3600))

	got := projectFullDayTurnoverYuan(4200_0000_0000, now, true)

	if got < 16000_0000_0000 || got > 17000_0000_0000 {
		t.Fatalf("expected projected turnover around 1.68万亿, got %.0f", got)
	}
}

func TestSummarizeEastMoneyIndexQuotes(t *testing.T) {
	raw := eastMoneyIndexQuoteResponse{
		Data: eastMoneyIndexQuoteData{
			Diff: []eastMoneyIndexQuote{
				{Code: "000001", Name: "上证指数", Amount: 4300_0000_0000},
				{Code: "399001", Name: "深证成指", Amount: 5500_0000_0000},
				{Code: "399006", Name: "创业板指", Amount: 2200_0000_0000},
			},
		},
	}

	result := summarizeEastMoneyIndexQuotes(raw)

	if result.TotalTurnoverYuan != 9800_0000_0000 {
		t.Fatalf("expected total turnover 9800亿, got %.0f", result.TotalTurnoverYuan)
	}
	if result.ShTurnoverYuan != 4300_0000_0000 {
		t.Fatalf("expected sh turnover 4300亿, got %.0f", result.ShTurnoverYuan)
	}
	if result.SzTurnoverYuan != 5500_0000_0000 {
		t.Fatalf("expected sz turnover 5500亿, got %.0f", result.SzTurnoverYuan)
	}
	if result.CybTurnoverYuan != 2200_0000_0000 {
		t.Fatalf("expected cyb turnover 2200亿, got %.0f", result.CybTurnoverYuan)
	}
}

func TestBuildShortTermVolumeFallbackUsesCacheBeforeNeutral(t *testing.T) {
	now := time.Date(2026, 7, 7, 14, 20, 0, 0, time.FixedZone("CST", 8*3600))
	cached := ShortTermVolume{
		TotalTurnoverYuan:     9200_0000_0000,
		ProjectedTurnoverYuan: 10200_0000_0000,
		VolumeScore:           65,
		Status:                "实时",
		UpdateTime:            now.Add(-time.Minute),
	}

	result := buildShortTermVolumeFallback(now, cached, true)

	if result.Status != "缓存" {
		t.Fatalf("expected cached status, got %s", result.Status)
	}
	if result.VolumeScore != 65 {
		t.Fatalf("expected cached score 65, got %d", result.VolumeScore)
	}
	if result.TurnoverText != "缓存 两市0.92万亿 / 预估全天1.02万亿" {
		t.Fatalf("unexpected cached turnover text: %s", result.TurnoverText)
	}
}

func TestBuildShortTermVolumeFallbackUsesNeutralWithoutCache(t *testing.T) {
	now := time.Date(2026, 7, 7, 8, 0, 0, 0, time.FixedZone("CST", 8*3600))

	result := buildShortTermVolumeFallback(now, ShortTermVolume{}, false)

	if result.Status != "占位" {
		t.Fatalf("expected placeholder status, got %s", result.Status)
	}
	if result.VolumeScore != 50 {
		t.Fatalf("expected neutral score 50, got %d", result.VolumeScore)
	}
	if result.TurnoverText != "实时成交额待接入" {
		t.Fatalf("unexpected neutral turnover text: %s", result.TurnoverText)
	}
}

func TestCaptureMarketStatisticSnapshotSkipsOutsideTradingTime(t *testing.T) {
	called := false

	captured, err := captureMarketStatisticSnapshot(false, func() error {
		called = true
		return nil
	})

	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if captured {
		t.Fatal("expected no capture outside trading time")
	}
	if called {
		t.Fatal("expected fetch not to be called outside trading time")
	}
}

func TestCaptureMarketStatisticSnapshotRunsDuringTradingTime(t *testing.T) {
	called := false
	wantErr := errors.New("fetch failed")

	captured, err := captureMarketStatisticSnapshot(true, func() error {
		called = true
		return wantErr
	})

	if !captured {
		t.Fatal("expected capture attempt during trading time")
	}
	if !called {
		t.Fatal("expected fetch to be called during trading time")
	}
	if !errors.Is(err, wantErr) {
		t.Fatalf("expected fetch error %v, got %v", wantErr, err)
	}
}

func TestBuildIntradayTrendDeduplicatesByTime(t *testing.T) {
	stats := []models.MarketStatistic{
		{DataTime: "09:35", UpCount: 1200, DownCount: 3800, UpRatio: 24, UpDownRatio: 0.316, LimitRatio: 2.5, LimitUp: 25, LimitDown: 10},
		{DataTime: "09:35", UpCount: 1300, DownCount: 3700, UpRatio: 26, UpDownRatio: 0.351, LimitRatio: 3.1, LimitUp: 31, LimitDown: 10},
		{DataTime: "10:00", UpCount: 2400, DownCount: 2600, UpRatio: 48, UpDownRatio: 0.923, LimitRatio: 8.2, LimitUp: 82, LimitDown: 10},
	}

	result := buildIntradayTrend(stats)

	if len(result) != 2 {
		t.Fatalf("expected 2 deduplicated trend points, got %d", len(result))
	}
	if result[0].Time != "09:35" {
		t.Fatalf("expected first time 09:35, got %s", result[0].Time)
	}
	if result[0].UpCount != 1300 {
		t.Fatalf("expected duplicate time to keep latest up count 1300, got %d", result[0].UpCount)
	}
	if result[1].RedRate != 48 {
		t.Fatalf("expected red rate 48, got %.2f", result[1].RedRate)
	}
	if result[1].EmotionIndex != 0.923 {
		t.Fatalf("expected emotion index 0.923, got %.3f", result[1].EmotionIndex)
	}
	if result[1].LimitRatio != 8.2 {
		t.Fatalf("expected limit ratio 8.2, got %.2f", result[1].LimitRatio)
	}
	if result[1].LimitUp != 82 || result[1].LimitDown != 10 {
		t.Fatalf("expected limit up/down 82/10, got %d/%d", result[1].LimitUp, result[1].LimitDown)
	}
}

func TestResolveShortTermEmotionDataDateUsesLatestStatisticBeforeOpen(t *testing.T) {
	now := time.Date(2026, 7, 8, 1, 20, 0, 0, time.FixedZone("CST", 8*3600))
	stats := []models.MarketStatistic{
		{DataDate: "2026-06-16", DataTime: "15:19"},
		{DataDate: "2026-06-16", DataTime: "15:24"},
	}

	got := resolveShortTermEmotionDataDate(now, stats)

	if got != "2026-06-16" {
		t.Fatalf("expected latest statistic date 2026-06-16, got %s", got)
	}
}

func TestResolveShortTermEmotionDataDateFallsBackToNowWithoutStats(t *testing.T) {
	now := time.Date(2026, 7, 8, 1, 20, 0, 0, time.FixedZone("CST", 8*3600))

	got := resolveShortTermEmotionDataDate(now, nil)

	if got != "2026-07-08" {
		t.Fatalf("expected current date 2026-07-08, got %s", got)
	}
}
