package t0reference

import "time"

func IsWeekendDate(date string) bool {
	parsed, err := time.Parse("2006-01-02", date)
	if err != nil {
		return false
	}
	return parsed.Weekday() == time.Saturday || parsed.Weekday() == time.Sunday
}
