package flutter_api

import (
	"testing"
	"time"
)

func TestShouldAutoPrewarmT0(t *testing.T) {
	loc := chinaLocation()

	cases := []struct {
		name string
		now  time.Time
		want bool
	}{
		{"周一 00:00 窗口起点", time.Date(2026, 8, 10, 0, 0, 0, 0, loc), true},
		{"周一 08:59 仍在窗口内", time.Date(2026, 8, 10, 8, 59, 0, 0, loc), true},
		{"周一 09:00 窗口已结束", time.Date(2026, 8, 10, 9, 0, 0, 0, loc), false},
		{"周一 09:20 交给请求触发", time.Date(2026, 8, 10, 9, 20, 0, 0, loc), false},
		{"周五 00:30 在窗口内", time.Date(2026, 8, 14, 0, 30, 0, 0, loc), true},
		{"周六 01:00 非交易日", time.Date(2026, 8, 15, 1, 0, 0, 0, loc), false},
		{"周日 08:00 非交易日", time.Date(2026, 8, 16, 8, 0, 0, 0, loc), false},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := shouldAutoPrewarmT0(c.now); got != c.want {
				t.Fatalf("shouldAutoPrewarmT0(%s) = %v, want %v", c.now.Format(time.RFC3339), got, c.want)
			}
		})
	}
}

// UTC 输入也要按上海时区判定：UTC 周一 23:00 == 上海周二 07:00，属于窗口内。
func TestShouldAutoPrewarmT0_ConvertsToShanghai(t *testing.T) {
	utcMondayNight := time.Date(2026, 8, 10, 23, 0, 0, 0, time.UTC)
	if !shouldAutoPrewarmT0(utcMondayNight) {
		t.Fatal("UTC 周一 23:00（上海周二 07:00）应在主动预热窗口内")
	}

	utcSundayNight := time.Date(2026, 8, 15, 23, 0, 0, 0, time.UTC)
	if shouldAutoPrewarmT0(utcSundayNight) {
		t.Fatal("UTC 周六 23:00（上海周日 07:00）不应在主动预热窗口内")
	}
}
