package flutter_api

import (
	"time"
)

const t0PrevDayLookbackDays = 5

func hasDailyBarOnDate(bars []dailyBar, date string) bool {
	for _, b := range bars {
		if b.Date == date {
			return true
		}
	}
	return false
}

func isValidTradingDay(date string) bool {
	if isT0DailyCacheFilePresent(date) {
		return true
	}
	bars := fetchDailyKLine("600000", date)
	return hasDailyBarOnDate(bars, date)
}

func resolvePrevTradingDay(tradeDate string) (string, bool) {
	start, err := time.Parse("2006-01-02", tradeDate)
	if err != nil {
		return "", false
	}
	d := start.AddDate(0, 0, -1)
	for i := 0; i < t0PrevDayLookbackDays; i++ {
		for d.Weekday() == time.Saturday || d.Weekday() == time.Sunday {
			d = d.AddDate(0, 0, -1)
			i++
			if i >= t0PrevDayLookbackDays {
				return "", false
			}
		}
		candidate := d.Format("2006-01-02")
		if isValidTradingDay(candidate) {
			return candidate, true
		}
		d = d.AddDate(0, 0, -1)
	}
	return "", false
}

func needsPrevDayBackfill(tradeDate string, now time.Time) (string, bool) {
	if !isPreopenPrevResultWindow(now, tradeDate) {
		return "", false
	}
	prev, ok := resolvePrevTradingDay(tradeDate)
	if !ok {
		return "", false
	}
	if _, found := loadT0SelectionArchive(prev); found {
		return prev, false
	}
	return prev, true
}
