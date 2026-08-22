package candlepattern

const (
	limitUpCloseRet = 9.89
	brokenLimitRet  = 9.85
	limitDownRet    = -9.9
	dojiPct         = 0.5
	smallBodyPct    = 2.5
	mediumBodyPct   = 6.0
)

func pctChange(from, to float64) float64 {
	if from <= 0 {
		return 0
	}
	return (to - from) / from * 100
}

func bodyPct(prevClose, open, close float64) float64 {
	if prevClose <= 0 {
		return 0
	}
	return (close - open) / prevClose * 100
}

func ClassifyDailyBar(prevClose float64, bar DailyBar) BarType {
	if prevClose <= 0 {
		return BarXX
	}
	closeRet := pctChange(prevClose, bar.Close)
	highRet := pctChange(prevClose, bar.High)
	body := bodyPct(prevClose, bar.Open, bar.Close)
	absBody := body
	if absBody < 0 {
		absBody = -absBody
	}

	if closeRet >= limitUpCloseRet {
		return BarZT
	}
	if closeRet <= limitDownRet {
		return BarDT
	}
	if highRet >= limitUpCloseRet && closeRet < brokenLimitRet {
		return BarPB
	}
	if absBody < dojiPct {
		if bar.Close >= bar.Open {
			return BarYX
		}
		return BarYXN
	}
	if body >= dojiPct && body <= smallBodyPct {
		return BarSY
	}
	if body <= -dojiPct && body >= -smallBodyPct {
		return BarXY
	}
	if body > smallBodyPct && body <= mediumBodyPct {
		return BarMY
	}
	if body < -smallBodyPct && body >= -mediumBodyPct {
		return BarMYIN
	}
	if body > mediumBodyPct {
		return BarDY
	}
	if body < -mediumBodyPct {
		return BarDYIN
	}
	return BarXX
}
