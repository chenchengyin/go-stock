package candlepattern

const (
	hourDojiPct = 0.3
)

func ClassifyHourBar(prevClose, open, close float64) HourBarType {
	if prevClose <= 0 {
		return HourXX
	}
	closeRet := pctChange(prevClose, close)
	body := bodyPct(prevClose, open, close)
	absBody := body
	if absBody < 0 {
		absBody = -absBody
	}
	if closeRet >= limitUpCloseRet {
		return HourZT
	}
	if closeRet <= limitDownRet {
		return HourDT
	}
	if absBody <= hourDojiPct {
		return HourDOJI
	}
	if close > open {
		return HourY
	}
	if close < open {
		return HourYIN
	}
	return HourXX
}

func FormatHourPattern(types []HourBarType) string {
	if len(types) == 0 {
		return ""
	}
	s := string(types[0])
	for i := 1; i < len(types); i++ {
		s += "|" + string(types[i])
	}
	return s
}
