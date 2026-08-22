package candlepattern

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"go-stock/backend/data"
)

type HourlyFetcher interface {
	Fetch60Min(shortCode string, days int) ([]HourBar, error)
}

type EastMoneyHourlyFetcher struct {
	api *data.EastMoneyKLineApi
}

func NewEastMoneyHourlyFetcher() *EastMoneyHourlyFetcher {
	return &EastMoneyHourlyFetcher{api: data.NewEastMoneyKLineApi(nil)}
}

func (f *EastMoneyHourlyFetcher) Fetch60Min(shortCode string, days int) ([]HourBar, error) {
	raw := f.api.GetMinuteKLine(shortCode, data.KLineType60Min, days)
	if raw == nil || len(*raw) == 0 {
		return nil, fmt.Errorf("no hourly data for %s", shortCode)
	}
	out := make([]HourBar, 0, len(*raw))
	for _, kd := range *raw {
		b, ok := parseHourBar(kd)
		if ok {
			out = append(out, b)
		}
	}
	if len(out) == 0 {
		return nil, fmt.Errorf("no parseable hourly bars for %s", shortCode)
	}
	return out, nil
}

func parseHourBar(kd data.KLineData) (HourBar, bool) {
	parse := func(s string) (float64, bool) {
		s = strings.TrimSpace(s)
		if s == "" {
			return 0, false
		}
		v, err := strconv.ParseFloat(s, 64)
		return v, err == nil
	}
	o, okO := parse(kd.Open)
	c, okC := parse(kd.Close)
	h, okH := parse(kd.High)
	l, okL := parse(kd.Low)
	if !okO || !okC || !okH || !okL {
		return HourBar{}, false
	}
	return HourBar{Time: kd.Day, Open: o, Close: c, High: h, Low: l}, true
}

func hourlyCachePath(cacheRoot, shortCode, tradeDate string) string {
	return filepath.Join(cacheRoot, "t0", "pattern", "hourly",
		fmt.Sprintf("hourly_3d_%s_%s.json", shortCode, tradeDate))
}

func LoadOrFetchHourly3D(cacheRoot, shortCode, tradeDate string, fetcher HourlyFetcher) ([]HourBar, error) {
	path := hourlyCachePath(cacheRoot, shortCode, tradeDate)
	if b, err := os.ReadFile(path); err == nil {
		var bars []HourBar
		if err := json.Unmarshal(b, &bars); err == nil && len(bars) > 0 {
			return bars, nil
		}
	}
	bars, err := fetcher.Fetch60Min(shortCode, 8)
	if err != nil {
		return nil, err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return bars, nil
	}
	if b, err := json.Marshal(bars); err == nil {
		_ = os.WriteFile(path, b, 0o644)
	}
	return bars, nil
}

func tradingDatesBefore(bars []DailyBar, tradeDate string, n int) []string {
	var dates []string
	for i := len(bars) - 1; i >= 0; i-- {
		if bars[i].Date >= tradeDate {
			continue
		}
		dates = append(dates, bars[i].Date)
		if len(dates) == n {
			break
		}
	}
	return dates
}

func filterHourlyByDates(hourly []HourBar, dates []string) []HourBar {
	if len(dates) == 0 {
		return nil
	}
	allowed := map[string]struct{}{}
	for _, d := range dates {
		allowed[d] = struct{}{}
	}
	out := make([]HourBar, 0, len(hourly))
	for _, h := range hourly {
		day := h.Time
		if len(day) >= 10 {
			day = day[:10]
		}
		if _, ok := allowed[day]; ok {
			out = append(out, h)
		}
	}
	return out
}

func classifyHourlyBars(bars []HourBar) []HourBarType {
	if len(bars) == 0 {
		return nil
	}
	out := make([]HourBarType, len(bars))
	prevClose := bars[0].Open
	if prevClose <= 0 {
		prevClose = bars[0].Close
	}
	for i, b := range bars {
		if i > 0 {
			prevClose = bars[i-1].Close
		}
		out[i] = ClassifyHourBar(prevClose, b.Open, b.Close)
	}
	return out
}

func tailHourPattern(bars []HourBar, tailN int) (string, bool) {
	filtered := bars
	types := classifyHourlyBars(filtered)
	if len(types) < tailN {
		return "", false
	}
	types = types[len(types)-tailN:]
	return FormatHourPattern(types), true
}
