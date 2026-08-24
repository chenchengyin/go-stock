package flutter_api

import (
	"go-stock/backend/analysis/candlepattern"
	"go-stock/backend/db"
	"go-stock/backend/models"
)

const (
	BuySignalGreen        = "green"
	BuySignalYellow       = "yellow"
	BuySignalRed          = "red"
	BuySignalInsufficient = "insufficient"

	t0PatternWindow = 3
)

func signalFromRates(winPct, failPct float64, t0N int, cfg models.T0PatternConfig) string {
	_ = cfg
	if t0N <= 0 {
		return BuySignalInsufficient
	}
	if failPct <= cfg.GreenMaxFail && winPct >= cfg.GreenMinWin {
		return BuySignalGreen
	}
	if failPct > cfg.RedMinFail || winPct < cfg.RedMaxWin {
		return BuySignalRed
	}
	return BuySignalYellow
}

func loadPatternConfig() models.T0PatternConfig {
	if db.Dao == nil {
		return models.DefaultT0PatternConfig("")
	}
	var cfg models.T0PatternConfig
	if err := db.Dao.First(&cfg, 1).Error; err != nil {
		return models.DefaultT0PatternConfig("")
	}
	return cfg
}

func lookupPatternStat(pattern string, window int) (models.T0PatternStat, bool) {
	if db.Dao == nil {
		return models.T0PatternStat{}, false
	}
	var st models.T0PatternStat
	if err := db.Dao.Where("pattern = ? AND window = ?", pattern, window).First(&st).Error; err != nil {
		return models.T0PatternStat{}, false
	}
	return st, true
}

func patternFromHist(hist []dailyBar) string {
	if len(hist) < t0PatternWindow+1 {
		return ""
	}
	bars := make([]candlepattern.DailyBar, len(hist))
	for i, b := range hist {
		bars[i] = candlepattern.DailyBar{
			Date:     b.Date,
			Open:     b.Open,
			Close:    b.Close,
			High:     b.High,
			Low:      b.Low,
			Volume:   b.Volume,
			AmountYi: b.AmountYi,
		}
	}
	labels, ok := candlepattern.BuildPatternLabels(bars, t0PatternWindow)
	if !ok {
		return ""
	}
	return candlepattern.FormatPattern(labels)
}

func enrichResultWithPattern(r *T0SelectionResult, hist []dailyBar) {
	pat := patternFromHist(hist)
	r.Pattern = pat
	if pat == "" {
		r.BuySignal = BuySignalInsufficient
		return
	}
	st, ok := lookupPatternStat(pat, t0PatternWindow)
	if !ok {
		r.BuySignal = BuySignalInsufficient
		return
	}
	cfg := loadPatternConfig()
	r.PatternT0N = st.T0N
	r.PatternWinPct = st.WinRate
	r.PatternFailPct = st.FailRate
	r.BuySignal = signalFromRates(st.WinRate, st.FailRate, st.T0N, cfg)
}

func enrichArchivedResults(tradeDate string, results []T0SelectionResult) []T0SelectionResult {
	if len(results) == 0 {
		return results
	}
	need := false
	for _, r := range results {
		if r.BuySignal == "" {
			need = true
			break
		}
	}
	if !need {
		return results
	}
	_, dailyCache, _, err := loadOrFetchT0Daily(tradeDate)
	if err != nil {
		return results
	}
	histCache := make(map[string][]dailyBar, len(dailyCache))
	for sc, bars := range dailyCache {
		histCache[sc] = histBarsBeforeTradeDate(bars, tradeDate)
	}
	out := make([]T0SelectionResult, len(results))
	copy(out, results)
	for i := range out {
		if out[i].BuySignal != "" {
			continue
		}
		sc := t0ShortCodeFromResultCode(out[i].StockCode)
		if hist, ok := histCache[sc]; ok {
			enrichResultWithPattern(&out[i], hist)
		}
	}
	return out
}
