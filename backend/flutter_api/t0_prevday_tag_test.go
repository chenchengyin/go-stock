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
		{"大阴线边界收盘-2%", 3.0, 2.0, -2.0, "前一天大阴线"},
		{"收盘跌超2%不算大阴线", 3.0, 1.0, -2.1, ""},
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

func TestPrevDayRetsFromHist(t *testing.T) {
	hist := []dailyBar{
		{Date: "2026-08-09", Close: 10},
		{Date: "2026-08-10", Open: 10.2, High: 11.0, Close: 10.5},
	}
	high, open, close, ok := prevDayRetsFromHist(hist)
	if !ok {
		t.Fatal("expected ok")
	}
	if round2(high) != 10 || round2(open) != 2 || round2(close) != 5 {
		t.Fatalf("got high=%v open=%v close=%v", high, open, close)
	}
	if _, _, _, ok := prevDayRetsFromHist(hist[:1]); ok {
		t.Fatal("len<2 should fail")
	}
	zeroBase := []dailyBar{
		{Date: "2026-08-09", Close: 0},
		{Date: "2026-08-10", Open: 1, High: 2, Close: 1.5},
	}
	if _, _, _, ok := prevDayRetsFromHist(zeroBase); ok {
		t.Fatal("base==0 should fail")
	}
}
