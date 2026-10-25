package flutter_api

import (
	"encoding/json"
	"fmt"
	"math"
	"os"
	"path/filepath"
	"time"
)

// Historical provider evidence is kept separate from captured point-in-time
// pools. Its membership is a reconstruction and must never become confirmed.
type t0HistoricalPoolReconstruction struct {
	SchemaVersion           int                   `json:"schema_version"`
	Date                    string                `json:"date"`
	ReferenceDate           string                `json:"reference_date"`
	Source                  string                `json:"source"`
	Method                  string                `json:"method"`
	EvidenceComplete        bool                  `json:"evidence_complete"`
	UniverseCount           int                   `json:"universe_count"`
	FetchedCodeCount        int                   `json:"fetched_code_count"`
	Stocks                  []t0Stock             `json:"stocks"`
	UnresolvedCodes         []string              `json:"unresolved_codes"`
	ThresholdUncertainCodes []string              `json:"threshold_uncertain_codes"`
	Daily                   map[string][]dailyBar `json:"daily,omitempty"`
}

func readT0HistoricalPoolReconstruction(date string) (*t0HistoricalPoolReconstruction, error) {
	parsed, err := time.Parse("2006-01-02", date)
	if err != nil || parsed.Format("2006-01-02") != date {
		return nil, fmt.Errorf("历史补采日期无效")
	}
	path := filepath.Join(t0CacheRootPath, "t0", "reconstructed_pools", "pool_"+date+".json")
	b, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var r t0HistoricalPoolReconstruction
	if err = json.Unmarshal(b, &r); err != nil {
		return nil, err
	}
	reference, referenceErr := time.Parse("2006-01-02", r.ReferenceDate)
	if r.SchemaVersion != 1 || r.Date != date || referenceErr != nil || reference.Format("2006-01-02") != r.ReferenceDate || r.ReferenceDate >= date || !r.EvidenceComplete || r.UniverseCount < len(r.Stocks) || r.FetchedCodeCount != r.UniverseCount || r.Source != "baostock_dated_history" || r.Method != "prior_close_float_cap_from_volume_turnover" {
		return nil, fmt.Errorf("历史补采来源、日期或采集完整性无效: %s", date)
	}
	if err = validateT0PoolArchive(t0PoolArchive{SchemaVersion: 1, Date: date, Source: r.Source, Complete: true, Stocks: r.Stocks}, date); err != nil {
		return nil, err
	}
	for _, stock := range r.Stocks {
		if stock.MarketCapYi < t0MinMarketCapYi || stock.MarketCapYi > t0MaxMarketCapYi {
			return nil, fmt.Errorf("历史补采市值越界: %s", stock.ShortCode)
		}
	}
	for code, bars := range r.Daily {
		previous := ""
		for _, bar := range bars {
			day, dateErr := time.Parse("2006-01-02", bar.Date)
			if dateErr != nil || day.Format("2006-01-02") != bar.Date || bar.Date <= previous {
				return nil, fmt.Errorf("历史补采日线日期无效: %s", code)
			}
			previous = bar.Date
			for _, value := range []float64{bar.Open, bar.High, bar.Low, bar.Close, bar.Volume, bar.AmountYi} {
				if math.IsNaN(value) || math.IsInf(value, 0) || value < 0 {
					return nil, fmt.Errorf("历史补采日线数值无效: %s", code)
				}
			}
			if bar.Open <= 0 || bar.Low <= 0 || bar.High < math.Max(bar.Open, math.Max(bar.Close, bar.Low)) || bar.Low > math.Min(bar.Open, bar.Close) || bar.Close <= 0 || !bar.AmountVerified || bar.AmountSource != "baostock_daily" || bar.VolumeUnit != "hands" || bar.AmountUnit != "yuan" {
				return nil, fmt.Errorf("历史补采日线价格/单位/来源无效: %s", code)
			}
		}
	}
	return &r, nil
}

func (r *t0HistoricalPoolReconstruction) warning() t0DataWarning {
	message := "历史补采重建：按历史证券名单及前一交易日流通市值估算，非当日原始股票池，结果仅供复盘参考"
	if unresolved := len(r.UnresolvedCodes) + len(r.ThresholdUncertainCodes); unresolved > 0 {
		message += fmt.Sprintf("；%d只股票市值证据不足或接近筛选边界", unresolved)
	}
	return t0DataWarning{Code: "T0_HISTORY_POOL_RECONSTRUCTED", Message: message, Phase: "history", Source: r.Source, Expected: r.UniverseCount, Actual: r.FetchedCodeCount}
}
