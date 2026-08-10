package flutter_api

import (
	"encoding/json"
	"fmt"
	"go-stock/backend/data"
	"go-stock/backend/logger"
	"math"
	"sync"
	"time"
)

const (
	eastMoneyIndexQuoteURL = "https://push2.eastmoney.com/api/qt/ulist.np/get?secids=1.000001,0.399001,0.399006&fields=f12,f14,f6"
	neutralTurnoverText    = "实时成交额待接入"
)

type ShortTermVolume struct {
	TotalTurnoverYuan     float64
	ProjectedTurnoverYuan float64
	ShTurnoverYuan        float64
	SzTurnoverYuan        float64
	CybTurnoverYuan       float64
	TurnoverText          string
	VolumeScore           int
	Status                string
	UpdateTime            time.Time
}

type eastMoneyIndexQuoteResponse struct {
	Data eastMoneyIndexQuoteData `json:"data"`
}

type eastMoneyIndexQuoteData struct {
	Diff []eastMoneyIndexQuote `json:"diff"`
}

type eastMoneyIndexQuote struct {
	Code   string  `json:"f12"`
	Name   string  `json:"f14"`
	Amount float64 `json:"f6"`
}

var shortTermVolumeCache struct {
	sync.RWMutex
	volume ShortTermVolume
	has    bool
}

func GetShortTermVolume(now time.Time, isTrading bool) ShortTermVolume {
	volume, err := fetchShortTermVolume(now, isTrading)
	if err == nil && volume.TotalTurnoverYuan > 0 {
		shortTermVolumeCache.Lock()
		shortTermVolumeCache.volume = volume
		shortTermVolumeCache.has = true
		shortTermVolumeCache.Unlock()
		return volume
	}

	shortTermVolumeCache.RLock()
	cached := shortTermVolumeCache.volume
	hasCache := shortTermVolumeCache.has
	shortTermVolumeCache.RUnlock()
	if err != nil {
		logger.SugaredLogger.Warnf("GetShortTermVolume fallback to neutral: %v", err)
	}
	return buildShortTermVolumeFallback(now, cached, hasCache)
}

func buildShortTermVolumeFallback(now time.Time, cached ShortTermVolume, hasCache bool) ShortTermVolume {
	if hasCache && cached.TotalTurnoverYuan > 0 {
		cached.Status = "缓存"
		cached.TurnoverText = formatTurnoverText(cached.TotalTurnoverYuan, cached.ProjectedTurnoverYuan, cached.Status)
		return cached
	}
	return ShortTermVolume{
		TurnoverText: neutralTurnoverText,
		VolumeScore:  50,
		Status:       "占位",
		UpdateTime:   now,
	}
}

func fetchShortTermVolume(now time.Time, isTrading bool) (ShortTermVolume, error) {
	resp, err := data.SharedHTTPClient.R().
		SetHeader("Referer", "https://quote.eastmoney.com/").
		SetHeader("User-Agent", "Mozilla/5.0").
		Get(eastMoneyIndexQuoteURL)
	if err != nil {
		return ShortTermVolume{}, err
	}

	var raw eastMoneyIndexQuoteResponse
	if err := json.Unmarshal(resp.Body(), &raw); err != nil {
		return ShortTermVolume{}, err
	}

	volume := summarizeEastMoneyIndexQuotes(raw)
	if volume.TotalTurnoverYuan <= 0 {
		return ShortTermVolume{}, fmt.Errorf("empty index turnover")
	}
	volume.ProjectedTurnoverYuan = projectFullDayTurnoverYuan(volume.TotalTurnoverYuan, now, isTrading)
	volume.VolumeScore = calculateVolumeScore(volume.TotalTurnoverYuan, volume.ProjectedTurnoverYuan)
	volume.Status = "实时"
	volume.TurnoverText = formatTurnoverText(volume.TotalTurnoverYuan, volume.ProjectedTurnoverYuan, volume.Status)
	volume.UpdateTime = now
	return volume, nil
}

func summarizeEastMoneyIndexQuotes(raw eastMoneyIndexQuoteResponse) ShortTermVolume {
	var volume ShortTermVolume
	for _, item := range raw.Data.Diff {
		switch item.Code {
		case "000001":
			volume.ShTurnoverYuan = item.Amount
		case "399001":
			volume.SzTurnoverYuan = item.Amount
		case "399006":
			volume.CybTurnoverYuan = item.Amount
		default:
			switch item.Name {
			case "上证指数":
				volume.ShTurnoverYuan = item.Amount
			case "深证成指":
				volume.SzTurnoverYuan = item.Amount
			case "创业板指":
				volume.CybTurnoverYuan = item.Amount
			}
		}
	}
	volume.TotalTurnoverYuan = volume.ShTurnoverYuan + volume.SzTurnoverYuan
	return volume
}

func projectFullDayTurnoverYuan(totalYuan float64, now time.Time, isTrading bool) float64 {
	if !isTrading {
		return totalYuan
	}
	elapsed := tradingMinutesElapsed(now)
	if elapsed <= 0 {
		return totalYuan
	}
	projected := totalYuan / float64(elapsed) * 240
	if projected < totalYuan {
		return totalYuan
	}
	return projected
}

func tradingMinutesElapsed(now time.Time) int {
	hour, minute := now.Hour(), now.Minute()
	minutes := hour*60 + minute
	morningStart := 9*60 + 30
	morningEnd := 11*60 + 30
	afternoonStart := 13 * 60
	afternoonEnd := 15 * 60

	switch {
	case minutes < morningStart:
		return 0
	case minutes <= morningEnd:
		return minutes - morningStart
	case minutes < afternoonStart:
		return 120
	case minutes <= afternoonEnd:
		return 120 + minutes - afternoonStart
	default:
		return 240
	}
}

func calculateVolumeScore(totalYuan float64, projectedYuan float64) int {
	basis := math.Max(totalYuan, projectedYuan)
	switch {
	case basis >= 13000_0000_0000:
		return 90
	case basis >= 11000_0000_0000:
		return 78
	case basis >= 9000_0000_0000:
		return 65
	case basis >= 7000_0000_0000:
		return 50
	default:
		return 35
	}
}

func formatTurnoverText(totalYuan float64, projectedYuan float64, status string) string {
	if totalYuan <= 0 {
		return neutralTurnoverText
	}
	if status == "" {
		status = "实时"
	}
	if projectedYuan > totalYuan {
		return fmt.Sprintf("%s 两市%s / 预估全天%s", status, formatTurnoverWanYi(totalYuan), formatTurnoverWanYi(projectedYuan))
	}
	return fmt.Sprintf("%s 两市%s", status, formatTurnoverWanYi(totalYuan))
}

func formatTurnoverWanYi(yuan float64) string {
	return fmt.Sprintf("%.2f万亿", yuan/10000_0000_0000)
}
