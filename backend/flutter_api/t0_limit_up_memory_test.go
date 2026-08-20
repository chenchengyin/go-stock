package flutter_api

import (
	"reflect"
	"testing"
)

func memBar(date string, high, close float64) dailyBar {
	return dailyBar{Date: date, High: high, Close: close}
}

func TestIsLimitUpMemoryDay(t *testing.T) {
	const p = 10.0
	cases := []struct {
		name string
		bar  dailyBar
		want bool
	}{
		{"收盘刚好9.89", memBar("d", 10.989, 10.989), true},
		{"收盘9.88不算涨停也不算破板", memBar("d", 10.988, 10.988), false},
		{"破板最高10收盘5", memBar("d", 11.0, 10.5), true},
		{"最高9.86收盘更低不算破板", memBar("d", 10.986, 10.98), false},
		{"最高9.89收盘更低算破板", memBar("d", 10.989, 10.98), true},
		{"封死涨停走收盘门槛", memBar("d", 11.0, 11.0), true},
		{"夹缝收盘9.86", memBar("d", 10.986, 10.986), false},
		{"冲涨停后收跌停", memBar("d", 11.0, 9.01), true},
		{"最高0仍可靠收盘9.89", memBar("d", 0, 10.989), true},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got := isLimitUpMemoryDay(p, c.bar, t0LimitUpCloseRet)
			if got != c.want {
				t.Fatalf("got %v want %v", got, c.want)
			}
		})
	}
	if isLimitUpMemoryDay(0, memBar("d", 11, 11), t0LimitUpCloseRet) {
		t.Fatal("prevClose==0 must be false")
	}
}

func TestLimitUpMemoryWindowAndDates(t *testing.T) {
	// 8 根：仅最早候选日（bars[1] vs bars[0]）收盘涨停。旧过滤切 7 根会漏掉这一天。
	hist := []dailyBar{
		memBar("2026-08-10", 10, 10),
		memBar("2026-08-11", 10.989, 10.989),
		memBar("2026-08-12", 11, 11),
		memBar("2026-08-13", 11, 11),
		memBar("2026-08-14", 11, 11),
		memBar("2026-08-17", 11, 11),
		memBar("2026-08-18", 11, 11),
		memBar("2026-08-19", 11, 11),
	}
	if !hasLimitUpMemory(hist, 7, t0LimitUpCloseRet) {
		t.Fatal("oldest of 7 days sealed limit-up must count")
	}
	got := collectLimitUpMemoryDates(hist, 7, t0LimitUpCloseRet)
	want := []string{"2026-08-11"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("dates=%v want %v", got, want)
	}

	// 只有破板、没收盘涨停
	brokenOnly := []dailyBar{
		memBar("2026-08-18", 10, 10),
		memBar("2026-08-19", 11.0, 10.5),
	}
	if !hasLimitUpMemory(brokenOnly, 7, t0LimitUpCloseRet) {
		t.Fatal("broken limit-up must pass")
	}
	if got := collectLimitUpMemoryDates(brokenOnly, 7, t0LimitUpCloseRet); !reflect.DeepEqual(got, []string{"2026-08-19"}) {
		t.Fatalf("broken dates=%v", got)
	}

	// 破板只认前一交易日：更早那天破板、昨日普通、近7日无收盘涨停 → 不算记忆
	olderBroken := []dailyBar{
		memBar("2026-08-17", 10, 10),
		memBar("2026-08-18", 11.0, 10.5),
		memBar("2026-08-19", 10.5, 10.5),
	}
	if hasLimitUpMemory(olderBroken, 7, t0LimitUpCloseRet) {
		t.Fatal("broken limit-up before yesterday must not count")
	}
	if got := collectLimitUpMemoryDates(olderBroken, 7, t0LimitUpCloseRet); len(got) != 0 {
		t.Fatalf("older broken dates=%v want none", got)
	}

	// 夹缝不得因这一天命中
	gap := []dailyBar{
		memBar("2026-08-18", 10, 10),
		memBar("2026-08-19", 10.986, 10.986),
	}
	if hasLimitUpMemory(gap, 7, t0LimitUpCloseRet) {
		t.Fatal("gap 9.86 must not count")
	}

	if formatLimitUpDates(nil) != "-" {
		t.Fatal("empty dates must be -")
	}
	four := []string{"a", "b", "c", "d"}
	if got := formatLimitUpDates(four); got != "b, c, d" {
		t.Fatalf("last3=%q", got)
	}
}

func TestHasLimitUpMemoryAgreesWithCollect(t *testing.T) {
	hists := [][]dailyBar{
		{memBar("d0", 10, 10), memBar("d1", 11, 10.5)},
		{memBar("d0", 10, 10), memBar("d1", 10.986, 10.986)},
		{memBar("d0", 10, 10), memBar("d1", 10.989, 10.989)},
		{memBar("d0", 10, 10), memBar("d1", 11, 10.5), memBar("d2", 10.5, 10.5)},
		nil,
		{memBar("d0", 10, 10)},
	}
	for i, h := range hists {
		has := hasLimitUpMemory(h, 7, t0LimitUpCloseRet)
		n := len(collectLimitUpMemoryDates(h, 7, t0LimitUpCloseRet))
		if has != (n > 0) {
			t.Fatalf("case %d has=%v n=%d", i, has, n)
		}
	}
}

func TestFilterLimitUpRecentUsesMemoryRules(t *testing.T) {
	makeStock := func(code string) t0Stock {
		return t0Stock{Code: "sz." + code, ShortCode: code, Name: code}
	}
	cache := map[string][]dailyBar{
		"000001": {memBar("2026-08-18", 10, 10), memBar("2026-08-19", 11.0, 10.5)},
		"000002": {memBar("2026-08-18", 10, 10), memBar("2026-08-19", 10.986, 10.986)},
		"000003": {memBar("2026-08-18", 10, 10), memBar("2026-08-19", 10.989, 10.989)},
		"000004": {
			memBar("2026-08-17", 10, 10),
			memBar("2026-08-18", 11.0, 10.5),
			memBar("2026-08-19", 10.5, 10.5),
		},
	}
	in := []t0Stock{makeStock("000001"), makeStock("000002"), makeStock("000003"), makeStock("000004")}
	got := filterLimitUpRecent(in, cache, t0LimitUpMemoryDays, t0LimitUpCloseRet)
	if len(got) != 2 {
		t.Fatalf("got %d stocks, want 2 (broken + sealed)", len(got))
	}
	names := map[string]bool{}
	for _, s := range got {
		names[s.ShortCode] = true
	}
	if !names["000001"] || !names["000003"] || names["000002"] || names["000004"] {
		t.Fatalf("unexpected set: %+v", names)
	}
}
