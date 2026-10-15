package flutter_api

import (
	"fmt"
	"sync"
	"time"

	"go-stock/backend/logger"
)

const t0PrevDayLookbackDays = 20

var tradingDayDailyFetcher = fetchDailyKLine

func hasDailyBarOnDate(bars []dailyBar, date string) bool {
	for _, b := range bars {
		if b.Date == date {
			return true
		}
	}
	return false
}

func isValidTradingDay(date string) bool {
	// A pool/cache can exist on a holiday and contain only older daily bars.
	if shared, err := loadMainboardSharedGob(); err == nil && len(shared.Dates) > 0 &&
		date >= shared.Dates[0] && date <= shared.Dates[len(shared.Dates)-1] {
		return sharedGobHasDate(shared, date)
	}
	if cached, ok := readT0DailyCachePayloadFromDisk(date); ok {
		for _, bars := range cached.Daily {
			if bar, _ := dailyBarOnDate(bars, date); bar.Close > 0 {
				return true
			}
		}
	}
	// Outside the shared calendar, missing/empty fallback bars are not proof
	// of a closed session (today's prewarm cache can contain only yesterday).
	bars := tradingDayDailyFetcher("600000", date)
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

func resolveNextTradingDay(tradeDate, throughDate string) (string, bool) {
	start, err := time.Parse("2006-01-02", tradeDate)
	if err != nil {
		return "", false
	}
	through, err := time.Parse("2006-01-02", throughDate)
	if err != nil || !start.Before(through) {
		return "", false
	}

	d := start.AddDate(0, 0, 1)
	for i := 0; i < t0PrevDayLookbackDays && !d.After(through); i++ {
		if d.Weekday() != time.Saturday && d.Weekday() != time.Sunday {
			candidate := d.Format("2006-01-02")
			if isValidTradingDay(candidate) {
				return candidate, true
			}
		}
		d = d.AddDate(0, 0, 1)
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

var (
	t0BackfillMu      sync.Mutex
	t0BackfillRunning = map[string]bool{} // key = today
)

func isPrevDayBackfillInProgress(today string) bool {
	p := getT0WarmProgress(today)
	return p.Status == t0WarmStatusWarming && p.BackfillDate != ""
}

func ensurePrevTradingDayBackfillStarted(today string, now time.Time) {
	prev, need := needsPrevDayBackfill(today, now)
	if !need {
		return
	}
	t0BackfillMu.Lock()
	if t0BackfillRunning[today] {
		t0BackfillMu.Unlock()
		return
	}
	t0BackfillRunning[today] = true
	t0BackfillMu.Unlock()

	updateT0WarmProgress(today, func(p *t0WarmProgress) {
		p.Status = t0WarmStatusWarming
		p.BackfillDate = prev
		p.BackfillPhase = "daily"
		if p.StartedAt.IsZero() {
			p.StartedAt = time.Now()
		}
	})
	go runT0PrevDayBackfillJob(today, prev)
}

func syncBackfillDailyProgress(today, prevDate string) {
	prevProg := getT0WarmProgress(prevDate)
	updateT0WarmProgress(today, func(p *t0WarmProgress) {
		p.BackfillDate = prevDate
		p.BackfillPhase = "daily"
		p.StockCount = prevProg.StockCount
		p.DailyFetched = prevProg.DailyFetched
		p.DailyTotal = prevProg.DailyTotal
		p.CandidateCount = prevProg.CandidateCount
	})
}

func runT0PrevDayBackfillJob(today, prevDate string) {
	defer func() {
		t0BackfillMu.Lock()
		delete(t0BackfillRunning, today)
		t0BackfillMu.Unlock()
	}()

	logger.SugaredLogger.Infof("[T0补全] 开始补全前一交易日 %s (today=%s)", prevDate, today)

	if !isT0DailyCacheFilePresent(prevDate) {
		tryStartT0Prewarm(prevDate)
		deadline := time.Now().Add(30 * time.Minute)
		for time.Now().Before(deadline) {
			syncBackfillDailyProgress(today, prevDate)
			if isT0DailyCacheFilePresent(prevDate) &&
				getT0WarmProgress(prevDate).Status == t0WarmStatusReady {
				break
			}
			if getT0WarmProgress(prevDate).Status == t0WarmStatusFailed {
				prevErr := getT0WarmProgress(prevDate).Err
				updateT0WarmProgress(today, func(p *t0WarmProgress) {
					p.Status = t0WarmStatusFailed
					p.Err = fmt.Sprintf("补全 %s 日线失败: %s", prevDate, prevErr)
					p.BackfillDate = prevDate
					p.BackfillPhase = "daily"
				})
				tryStartT0Prewarm(today)
				return
			}
			time.Sleep(500 * time.Millisecond)
		}
		if !isT0DailyCacheFilePresent(prevDate) {
			updateT0WarmProgress(today, func(p *t0WarmProgress) {
				p.Status = t0WarmStatusFailed
				p.Err = fmt.Sprintf("补全 %s 日线超时", prevDate)
			})
			tryStartT0Prewarm(today)
			return
		}
	}

	updateT0WarmProgress(today, func(p *t0WarmProgress) {
		p.BackfillPhase = "selection"
	})
	results, err := RunT0Selection(prevDate)
	if err != nil {
		logger.SugaredLogger.Warnf("[T0补全] 选股 %s: %v，写入空归档", prevDate, err)
		results = []T0SelectionResult{}
	}
	if saveErr := saveT0SelectionArchive(prevDate, results, false); saveErr != nil {
		logger.SugaredLogger.Warnf("[T0补全] 归档写入失败 %s: %v", prevDate, saveErr)
	}

	updateT0WarmProgress(today, func(p *t0WarmProgress) {
		p.BackfillPhase = "close_refresh"
	})
	if _, refreshErr := refreshSelectionCloseRet(prevDate, false); refreshErr != nil {
		logger.SugaredLogger.Warnf("[T0补全] refresh_close %s: %v", prevDate, refreshErr)
	}

	updateT0WarmProgress(today, func(p *t0WarmProgress) {
		p.Status = t0WarmStatusIdle
		p.BackfillDate = ""
		p.BackfillPhase = ""
	})
	logger.SugaredLogger.Infof("[T0补全] 完成 %s", prevDate)
	tryStartT0Prewarm(today)
}
