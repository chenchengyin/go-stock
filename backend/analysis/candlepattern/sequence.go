package candlepattern

func BuildPatternLabels(hist []DailyBar, window int) ([]BarType, bool) {
	if len(hist) < window {
		return nil, false
	}
	seg := hist[len(hist)-window:]
	out := make([]BarType, window)
	for i, bar := range seg {
		idx := len(hist) - window + i
		prevClose := hist[idx-1].Close
		out[i] = ClassifyDailyBar(prevClose, bar)
	}
	return out, true
}

func FormatPattern(types []BarType) string {
	if len(types) == 0 {
		return ""
	}
	s := string(types[0])
	for i := 1; i < len(types); i++ {
		s += "|" + string(types[i])
	}
	return s
}
