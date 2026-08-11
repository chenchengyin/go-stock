package flutter_api

import "testing"

func TestPickPrevDayTag(t *testing.T) {
	cases := []struct {
		name              string
		high, open, close float64
		want              string
	}{
		{"涨停破板", 10.0, 2.0, 5.0, "涨停破板"},
		{"最高刚好9.85且收盘更低", 9.85, 1.0, 9.84, "涨停破板"},
		{"封死涨停不算破板", 10.0, 2.0, 10.0, ""},
		{"前一天跌停", 1.0, -2.0, -9.9, "前一天跌停"},
		{"前一天大阴线", 3.0, 5.0, 0.5, "前一天大阴线"},
		{"破板优先于大阴线", 10.0, 8.0, 3.0, "涨停破板"},
		{"跌停优先于大阴线", 2.0, 1.0, -10.0, "前一天跌停"},
		{"破板且跌停则清空", 10.0, 2.0, -10.0, ""},
		{"无标记", 3.0, 1.0, 0.5, ""},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got := pickPrevDayTag(c.high, c.open, c.close)
			if got != c.want {
				t.Fatalf("pickPrevDayTag(%v,%v,%v)=%q want %q", c.high, c.open, c.close, got, c.want)
			}
		})
	}
}
