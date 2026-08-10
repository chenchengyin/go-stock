package flutter_api

import (
	"fmt"
	"go-stock/backend/data"
	"go-stock/backend/models"
	"math"
	"net/http"
	"strconv"
	"strings"
	"time"
)

type ShortTermEmotion struct {
	Score           int                         `json:"score"`
	Phase           string                      `json:"phase"`
	Action          string                      `json:"action"`
	RiskLevel       string                      `json:"riskLevel"`
	SuggestedWeight string                      `json:"suggestedWeight"`
	MainTheme       string                      `json:"mainTheme"`
	UpdateTime      string                      `json:"updateTime"`
	IsTrading       bool                        `json:"isTrading"`
	Explanation     string                      `json:"explanation"`
	Dashboard       []ShortTermEmotionMetric    `json:"dashboard"`
	Components      []ShortTermEmotionComponent `json:"components"`
	RiskSignals     []ShortTermEmotionSignal    `json:"riskSignals"`
	IntradayEvents  []ShortTermEmotionEvent     `json:"intradayEvents"`
	IntradayTrend   []ShortTermEmotionTrend     `json:"intradayTrend"`
}

type ShortTermEmotionMetric struct {
	Name  string `json:"name"`
	Value string `json:"value"`
	Note  string `json:"note"`
	Tone  string `json:"tone"`
}

type ShortTermEmotionComponent struct {
	Name   string `json:"name"`
	Score  int    `json:"score"`
	Weight int    `json:"weight"`
	Note   string `json:"note"`
}

type ShortTermEmotionSignal struct {
	Name  string `json:"name"`
	Level string `json:"level"`
	Note  string `json:"note"`
	Tone  string `json:"tone"`
}

type ShortTermEmotionEvent struct {
	Time  string `json:"time"`
	Title string `json:"title"`
	Level string `json:"level"`
}

type ShortTermEmotionTrend struct {
	Time         string  `json:"time"`
	UpCount      int     `json:"upCount"`
	DownCount    int     `json:"downCount"`
	RedRate      float64 `json:"redRate"`
	EmotionIndex float64 `json:"emotionIndex"`
	LimitRatio   float64 `json:"limitRatio"`
	LimitUp      int     `json:"limitUp"`
	LimitDown    int     `json:"limitDown"`
}

type ShortTermEmotionInput struct {
	Now                time.Time
	IsTrading          bool
	UpCount            int
	DownCount          int
	LimitUp            int
	LimitDown          int
	BreakLimitCount    int
	SealedLimitCount   int
	MaxBoard           int
	Board3PlusCount    int
	BullishChanges     int
	BearishChanges     int
	MainTheme          string
	MainThemeScore     int
	TurnoverText       string
	VolumeScore        int
	IntradayTrend      []ShortTermEmotionTrend
	PreviousCloseScore *int
}

type uplimitHotSummary struct {
	MainTheme        string
	MainThemeScore   int
	MaxBoard         int
	Board3PlusCount  int
	SealedLimitCount int
}

type stockChangeSummary struct {
	BullishChanges  int
	BearishChanges  int
	BreakLimitCount int
}

func GetShortTermEmotion(isTrading bool) ShortTermEmotion {
	loc, _ := time.LoadLocation("Asia/Shanghai")
	now := time.Now().In(loc)

	marketApi := data.NewMarketStatisticApi()
	stats := marketApi.GetTodayData()
	if len(stats) == 0 {
		return BuildShortTermEmotionEmpty(now, "暂无市场统计数据")
	}
	latest := stats[len(stats)-1]

	changes := data.NewStockChangesApi().GetStockChanges(shortTermEmotionChangeTypes(), 0, 500)
	changeSummary := summarizeStockChanges(changes)
	dataDate := resolveShortTermEmotionDataDate(now, stats)
	previousCloseScore := resolvePreviousCloseScore(marketApi.GetLatestBeforeDate(dataDate), dataDate)
	hotSummary := normalizeUplimitHot(data.NewMarketNewsApi().GetUplimitHot(dataDate, 20))
	volume := GetShortTermVolume(now, isTrading)

	return CalculateShortTermEmotion(ShortTermEmotionInput{
		Now:                now,
		IsTrading:          isTrading,
		UpCount:            latest.UpCount,
		DownCount:          latest.DownCount,
		LimitUp:            latest.LimitUp,
		LimitDown:          latest.LimitDown,
		BreakLimitCount:    changeSummary.BreakLimitCount,
		SealedLimitCount:   choosePositive(hotSummary.SealedLimitCount, latest.LimitUp),
		MaxBoard:           hotSummary.MaxBoard,
		Board3PlusCount:    hotSummary.Board3PlusCount,
		BullishChanges:     changeSummary.BullishChanges,
		BearishChanges:     changeSummary.BearishChanges,
		MainTheme:          normalizeMainTheme(hotSummary.MainTheme),
		MainThemeScore:     choosePositive(hotSummary.MainThemeScore, 30),
		TurnoverText:       volume.TurnoverText,
		VolumeScore:        volume.VolumeScore,
		IntradayTrend:      buildIntradayTrend(stats),
		PreviousCloseScore: previousCloseScore,
	})
}

func handleShortTermEmotion(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}
	WriteJSON(w, GetShortTermEmotion(isTradingTime()))
}

func BuildShortTermEmotionEmpty(now time.Time, reason string) ShortTermEmotion {
	return ShortTermEmotion{
		Score:           0,
		Phase:           "数据不足",
		Action:          "谨慎观察",
		RiskLevel:       "未知",
		SuggestedWeight: "0成",
		MainTheme:       "无明显主线",
		UpdateTime:      now.Format("15:04:05"),
		IsTrading:       false,
		Explanation:     reason + "，暂不生成短线情绪判断。",
		Dashboard:       []ShortTermEmotionMetric{},
		Components:      []ShortTermEmotionComponent{},
		RiskSignals: []ShortTermEmotionSignal{
			{Name: "数据完整性", Level: "警惕", Note: reason, Tone: "warning"},
		},
		IntradayEvents: []ShortTermEmotionEvent{},
		IntradayTrend:  []ShortTermEmotionTrend{},
	}
}

func CalculateShortTermEmotion(input ShortTermEmotionInput) ShortTermEmotion {
	upRatio := calculateUpRatio(input.UpCount, input.DownCount)
	breakRate := calculateBreakRate(input.BreakLimitCount, input.SealedLimitCount)
	limitRatio := calculateLimitRatio(input.LimitUp, input.LimitDown)

	widthScore := calculateWidthScore(upRatio)
	limitScore := calculateLimitScore(input.LimitUp, input.LimitDown, breakRate)
	boardScore := clampInt(input.MaxBoard*10+input.Board3PlusCount*4, 0, 100)
	changeScore := calculateChangeScore(input.BullishChanges, input.BearishChanges)
	themeScore := clampInt(input.MainThemeScore, 0, 100)
	volumeScore := clampInt(input.VolumeScore, 0, 100)

	components := []ShortTermEmotionComponent{
		{Name: "宽度情绪", Score: widthScore, Weight: 25, Note: fmt.Sprintf("红盘率 %.0f%%", upRatio)},
		{Name: "涨跌停质量", Score: limitScore, Weight: 25, Note: fmt.Sprintf("涨停 %d / 跌停 %d，炸板率 %.0f%%", input.LimitUp, input.LimitDown, breakRate)},
		{Name: "连板生态", Score: boardScore, Weight: 20, Note: fmt.Sprintf("最高 %d 板，3板以上 %d 只", input.MaxBoard, input.Board3PlusCount)},
		{Name: "异动强弱", Score: changeScore, Weight: 15, Note: fmt.Sprintf("多头 %d / 空头 %d", input.BullishChanges, input.BearishChanges)},
		{Name: "板块主线", Score: themeScore, Weight: 10, Note: normalizeMainTheme(input.MainTheme)},
		{Name: "指数量能", Score: volumeScore, Weight: 5, Note: normalizeTurnoverText(input.TurnoverText)},
	}

	riskPenalty := calculateRiskPenalty(breakRate, input.LimitDown, input.BearishChanges, input.BullishChanges)
	score := clampInt(weightedComponentScore(components)-riskPenalty, 0, 100)
	highRisk := isHighShortTermRisk(breakRate, input.LimitDown, input.BearishChanges, input.BullishChanges)
	phase, action, riskLevel, suggestedWeight := classifyShortTermEmotion(score, highRisk, input.LimitDown, input.PreviousCloseScore)
	mainTheme := normalizeMainTheme(input.MainTheme)

	return ShortTermEmotion{
		Score:           score,
		Phase:           phase,
		Action:          action,
		RiskLevel:       riskLevel,
		SuggestedWeight: suggestedWeight,
		MainTheme:       mainTheme,
		UpdateTime:      input.Now.Format("15:04:05"),
		IsTrading:       input.IsTrading,
		Explanation:     buildShortTermExplanation(score, phase, action, mainTheme, breakRate),
		Dashboard: []ShortTermEmotionMetric{
			{Name: "红盘率", Value: fmt.Sprintf("%.0f%%", upRatio), Note: fmt.Sprintf("%d 涨 / %d 跌", input.UpCount, input.DownCount), Tone: toneByScore(widthScore)},
			{Name: "涨跌停", Value: fmt.Sprintf("%d / %d", input.LimitUp, input.LimitDown), Note: fmt.Sprintf("涨跌停比 %.1f", limitRatio), Tone: toneByScore(limitScore)},
			{Name: "炸板率", Value: fmt.Sprintf("%.0f%%", breakRate), Note: fmt.Sprintf("炸板 %d / 封板 %d", input.BreakLimitCount, input.SealedLimitCount), Tone: toneByBreakRate(breakRate)},
			{Name: "最高连板", Value: fmt.Sprintf("%d板", input.MaxBoard), Note: fmt.Sprintf("3板以上 %d 只", input.Board3PlusCount), Tone: toneByScore(boardScore)},
			{Name: "异动强弱", Value: fmt.Sprintf("%d / %d", input.BullishChanges, input.BearishChanges), Note: "多头 / 空头", Tone: toneByScore(changeScore)},
			{Name: "指数量能", Value: normalizeTurnoverText(input.TurnoverText), Note: fmt.Sprintf("量能分 %d", volumeScore), Tone: toneByScore(volumeScore)},
		},
		Components:     components,
		RiskSignals:    buildRiskSignals(breakRate, input.LimitDown, input.BearishChanges, input.BullishChanges, mainTheme),
		IntradayEvents: buildIntradayEvents(input.Now, score, phase),
		IntradayTrend:  input.IntradayTrend,
	}
}

func resolveShortTermEmotionDataDate(now time.Time, stats []models.MarketStatistic) string {
	for i := len(stats) - 1; i >= 0; i-- {
		if strings.TrimSpace(stats[i].DataDate) != "" {
			return stats[i].DataDate
		}
	}
	return now.Format("2006-01-02")
}

func buildIntradayTrend(stats []models.MarketStatistic) []ShortTermEmotionTrend {
	if len(stats) == 0 {
		return []ShortTermEmotionTrend{}
	}

	order := make([]string, 0, len(stats))
	byTime := make(map[string]ShortTermEmotionTrend, len(stats))
	for _, stat := range stats {
		if strings.TrimSpace(stat.DataTime) == "" {
			continue
		}
		if _, exists := byTime[stat.DataTime]; !exists {
			order = append(order, stat.DataTime)
		}
		byTime[stat.DataTime] = ShortTermEmotionTrend{
			Time:         stat.DataTime,
			UpCount:      stat.UpCount,
			DownCount:    stat.DownCount,
			RedRate:      stat.UpRatio,
			EmotionIndex: stat.UpDownRatio,
			LimitRatio:   stat.LimitRatio,
			LimitUp:      stat.LimitUp,
			LimitDown:    stat.LimitDown,
		}
	}

	points := make([]ShortTermEmotionTrend, 0, len(order))
	for _, dataTime := range order {
		points = append(points, byTime[dataTime])
	}
	return points
}

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

func calculateUpRatio(upCount, downCount int) float64 {
	total := upCount + downCount
	if total <= 0 {
		return 0
	}
	return float64(upCount) / float64(total) * 100
}

func shortTermEmotionChangeTypes() []int {
	return []int{
		8201, 8202, 8193, 4, 32, 64, 8207, 8209, 8211, 8213, 8215,
		8204, 8203, 8194, 8, 16, 128, 8208, 8210, 8212, 8214, 8216,
	}
}

func summarizeStockChanges(resp *data.StockChangesResponse) stockChangeSummary {
	if resp == nil {
		return stockChangeSummary{}
	}
	bullish := map[string]bool{
		"火箭发射": true, "快速反弹": true, "大笔买入": true, "封涨停板": true,
		"打开跌停板": true, "有大买盘": true, "竞价上涨": true, "高开5日线": true,
		"向上缺口": true, "60日新高": true, "60日大幅上涨": true,
	}
	bearish := map[string]bool{
		"加速下跌": true, "高台跳水": true, "大笔卖出": true, "封跌停板": true,
		"有大卖盘": true, "竞价下跌": true, "低开5日线": true,
		"向下缺口": true, "60日新低": true, "60日大幅下跌": true,
	}

	var summary stockChangeSummary
	for _, item := range resp.Data {
		if bullish[item.TypeName] {
			summary.BullishChanges++
		}
		if bearish[item.TypeName] {
			summary.BearishChanges++
		}
		if item.TypeName == "打开涨停板" {
			summary.BreakLimitCount++
		}
	}
	return summary
}

func normalizeUplimitHot(raw map[string]any) uplimitHotSummary {
	dataMap, ok := raw["data"].(map[string]any)
	if !ok {
		return uplimitHotSummary{}
	}

	summary := uplimitHotSummary{
		MaxBoard: intFromAny(dataMap["max_count"]),
	}

	if plateList, ok := dataMap["plate"].([]any); ok && len(plateList) > 0 {
		if firstPlate, ok := plateList[0].([]any); ok && len(firstPlate) >= 3 {
			summary.MainTheme = stringFromAny(firstPlate[0])
			summary.MainThemeScore = intFromAny(firstPlate[2])
		}
	}

	if banInfo, ok := dataMap["ban_info"].(map[string]any); ok {
		for levelText, item := range banInfo {
			level, err := strconv.Atoi(levelText)
			if err != nil || level < 3 {
				continue
			}
			if itemMap, ok := item.(map[string]any); ok {
				summary.Board3PlusCount += intFromAny(itemMap["count"])
			}
		}
	}

	if stocks := stringFromAny(dataMap["stocks"]); stocks != "" {
		parts := strings.Split(stocks, ",")
		for _, part := range parts {
			if strings.TrimSpace(part) != "" {
				summary.SealedLimitCount++
			}
		}
	}

	return summary
}

func intFromAny(value any) int {
	switch v := value.(type) {
	case int:
		return v
	case int64:
		return int(v)
	case float64:
		return int(v)
	case float32:
		return int(v)
	case string:
		i, _ := strconv.Atoi(strings.TrimSpace(v))
		return i
	default:
		return 0
	}
}

func stringFromAny(value any) string {
	if value == nil {
		return ""
	}
	return strings.TrimSpace(fmt.Sprint(value))
}

func choosePositive(primary int, fallback int) int {
	if primary > 0 {
		return primary
	}
	return fallback
}

func calculateBreakRate(breakLimitCount, sealedLimitCount int) float64 {
	total := breakLimitCount + sealedLimitCount
	if total <= 0 {
		return 0
	}
	return float64(breakLimitCount) / float64(total) * 100
}

func calculateLimitRatio(limitUp, limitDown int) float64 {
	if limitDown > 0 {
		return float64(limitUp) / float64(limitDown)
	}
	if limitUp > 0 {
		return float64(limitUp)
	}
	return 0
}

func calculateLimitScore(limitUp, limitDown int, breakRate float64) int {
	if limitUp <= 0 && limitDown <= 0 {
		return 25
	}

	total := limitUp + limitDown
	ratioScore := float64(0)
	if total > 0 {
		ratioScore = float64(limitUp) / float64(total) * 100
	}

	downPenalty := float64(limitDown) * 0.3
	breakPenalty := breakRate * 0.7

	score := ratioScore - downPenalty - breakPenalty
	if limitUp > limitDown && limitDown > 30 {
		score -= float64(limitDown-30) * 0.4
	}
	if limitDown > 15 && float64(limitUp)/math.Max(float64(limitDown), 1) > 3 {
		score -= float64(limitDown-15) * 0.5
	}
	if limitUp >= 80 && limitDown <= 10 {
		score += math.Min(float64(limitUp-80)*0.25, 5)
	}
	return clampInt(int(math.Round(score)), 0, 100)
}

func calculateWidthScore(upRatio float64) int {
	if upRatio <= 0 {
		return 0
	}
	if upRatio >= 85 {
		return 100
	}
	points := []struct {
		ratio float64
		score float64
	}{
		{ratio: 0, score: 0},
		{ratio: 20, score: 12},
		{ratio: 30, score: 25},
		{ratio: 40, score: 40},
		{ratio: 50, score: 50},
		{ratio: 60, score: 60},
		{ratio: 70, score: 75},
		{ratio: 80, score: 90},
		{ratio: 85, score: 100},
	}
	for i := 1; i < len(points); i++ {
		left := points[i-1]
		right := points[i]
		if upRatio <= right.ratio {
			progress := (upRatio - left.ratio) / (right.ratio - left.ratio)
			score := left.score + progress*(right.score-left.score)
			return clampInt(int(math.Round(score)), 0, 100)
		}
	}
	return 100
}

func calculateChangeScore(bullishChanges, bearishChanges int) int {
	total := bullishChanges + bearishChanges
	if total <= 0 {
		return 50
	}
	return clampInt(int(math.Round(float64(bullishChanges)/float64(total)*100)), 0, 100)
}

func calculateRiskPenalty(breakRate float64, limitDown, bearishChanges, bullishChanges int) int {
	penalty := 0
	if breakRate >= 30 {
		penalty += 8
	}
	if limitDown >= 20 {
		penalty += 12
	}
	if bearishChanges > bullishChanges {
		penalty += 10
	}
	return penalty
}

func weightedComponentScore(components []ShortTermEmotionComponent) int {
	totalWeight := 0
	totalScore := 0
	for _, component := range components {
		totalWeight += component.Weight
		totalScore += component.Score * component.Weight
	}
	if totalWeight <= 0 {
		return 0
	}
	return int(math.Round(float64(totalScore) / float64(totalWeight)))
}

func isHighShortTermRisk(breakRate float64, limitDown, bearishChanges, bullishChanges int) bool {
	return breakRate >= 30 || limitDown >= 20 || bearishChanges > bullishChanges
}

const (
	shortTermLowPhaseScore    = 35
	shortTermRepairPhaseScore = 55
	shortTermRetreatDrop      = 10
)

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
		phase := resolveGranvillePhase(score, previousCloseScore)
		if score < shortTermLowPhaseScore {
			return phase, "谨慎观察", "高", "0成"
		}
		return phase, "谨慎观察", "中等偏高", "0-2成"
	}
}

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

func normalizeMainTheme(mainTheme string) string {
	if mainTheme == "" {
		return "无明显主线"
	}
	return mainTheme
}

func normalizeTurnoverText(turnoverText string) string {
	if turnoverText == "" {
		return "实时成交额待接入"
	}
	return turnoverText
}

func buildShortTermExplanation(score int, phase, action, mainTheme string, breakRate float64) string {
	return fmt.Sprintf("当前短线情绪 %d 分，处于%s，建议%s。主线为%s，炸板率 %.0f%%，按中等偏避坑模型处理。", score, phase, action, mainTheme, breakRate)
}

func buildRiskSignals(breakRate float64, limitDown, bearishChanges, bullishChanges int, mainTheme string) []ShortTermEmotionSignal {
	signals := make([]ShortTermEmotionSignal, 0, 4)
	if breakRate >= 30 {
		signals = append(signals, ShortTermEmotionSignal{Name: "炸板风险", Level: "警惕", Note: fmt.Sprintf("炸板率 %.0f%%，避免追后排", breakRate), Tone: "warning"})
	}
	if limitDown >= 20 {
		signals = append(signals, ShortTermEmotionSignal{Name: "跌停扩散", Level: "高", Note: fmt.Sprintf("跌停 %d 只，亏钱效应扩散", limitDown), Tone: "danger"})
	}
	if bearishChanges > bullishChanges {
		signals = append(signals, ShortTermEmotionSignal{Name: "空头异动", Level: "警惕", Note: fmt.Sprintf("空头异动 %d 超过多头 %d", bearishChanges, bullishChanges), Tone: "warning"})
	}
	if mainTheme == "无明显主线" {
		signals = append(signals, ShortTermEmotionSignal{Name: "主线清晰度", Level: "警惕", Note: "无明显主线，轮动行情少追涨", Tone: "warning"})
	}
	if len(signals) == 0 {
		signals = append(signals, ShortTermEmotionSignal{Name: "短线风险", Level: "可控", Note: "未触发核心避坑条件", Tone: "success"})
	}
	return signals
}

func buildIntradayEvents(now time.Time, score int, phase string) []ShortTermEmotionEvent {
	return []ShortTermEmotionEvent{
		{Time: now.Format("15:04"), Title: fmt.Sprintf("当前情绪 %d 分，%s", score, phase), Level: "info"},
	}
}

func toneByScore(score int) string {
	if score >= 70 {
		return "positive"
	}
	if score >= 45 {
		return "warning"
	}
	return "negative"
}

func toneByBreakRate(rate float64) string {
	if rate >= 45 {
		return "negative"
	}
	if rate >= 30 {
		return "warning"
	}
	return "positive"
}

func clampInt(value, min, max int) int {
	if value < min {
		return min
	}
	if value > max {
		return max
	}
	return value
}
