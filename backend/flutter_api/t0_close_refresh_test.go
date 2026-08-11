package flutter_api

import (
	"testing"
	"time"
)

func TestShouldRefreshSelectionClose(t *testing.T) {
	loc := chinaLocation()
	cases := []struct {
		name      string
		updatedAt string
		now       time.Time
		want      bool
	}{
		{"周一15:05且未刷新", "", time.Date(2026, 8, 10, 15, 5, 0, 0, loc), true},
		{"周一15:04太早", "", time.Date(2026, 8, 10, 15, 4, 0, 0, loc), false},
		{"周一收盘后更晚也可补刷", "", time.Date(2026, 8, 10, 22, 0, 0, 0, loc), true},
		{"已刷新跳过", "2026-08-10T15:06:00+08:00", time.Date(2026, 8, 10, 15, 30, 0, 0, loc), false},
		{"周六不刷", "", time.Date(2026, 8, 15, 15, 10, 0, 0, loc), false},
		{"周日不刷", "", time.Date(2026, 8, 16, 15, 10, 0, 0, loc), false},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := shouldRefreshSelectionClose(c.now, c.updatedAt); got != c.want {
				t.Fatalf("got %v want %v", got, c.want)
			}
		})
	}
}

func TestPatchSelectionCloseRets(t *testing.T) {
	results := []T0SelectionResult{
		{StockCode: "000537.XSHE", StockName: "绿发电力", CloseRet: 5.53, PrevClose: 8.32},
		{StockCode: "002194.XSHE", StockName: "武汉凡谷", CloseRet: 2.6, PrevClose: 10},
		{StockCode: "605179.XSHG", StockName: "一鸣食品", CloseRet: 9.67, PrevClose: 25.12},
		{StockCode: "600188.XSHG", StockName: "兖矿能源", CloseRet: 1.42, PrevClose: 20},
	}
	quotes := map[string]t0Realtime{
		"000537": {Close: 8.77, PrevClose: 8.32},
		"002194": {Close: 0, PrevClose: 10}, // 价格为0 → 保留旧值
		"600188": {Close: 21, PrevClose: 0}, // 腾讯昨收缺失 → 回退归档前收
		// 605179 行情缺失 → 保留旧值
	}

	updated, kept := patchSelectionCloseRets(results, quotes)
	if updated != 2 || kept != 2 {
		t.Fatalf("updated=%d kept=%d", updated, kept)
	}
	if got, want := results[0].CloseRet, round2((8.77-8.32)/8.32*100); got != want {
		t.Fatalf("000537 CloseRet=%v want %v", got, want)
	}
	if got, want := results[3].CloseRet, round2((21.0-20.0)/20.0*100); got != want {
		t.Fatalf("600188 CloseRet=%v want %v", got, want)
	}
	if results[1].CloseRet != 2.6 || results[2].CloseRet != 9.67 {
		t.Fatalf("kept values mutated: %v %v", results[1].CloseRet, results[2].CloseRet)
	}
}

func TestPatchSelectionTags(t *testing.T) {
	results := []T0SelectionResult{
		{StockCode: "600001.XSHG"}, // 破板
		{StockCode: "000002.XSHE"}, // 跌停
		{StockCode: "000003.XSHE"}, // 无标记
		{StockCode: "000004.XSHE"}, // 缓存缺失
	}
	daily := map[string][]dailyBar{
		"600001": {
			{Date: "2026-08-07", Close: 10},
			{Date: "2026-08-10", Open: 10.2, High: 11.0, Close: 10.6},
			{Date: "2026-08-11", Open: 10.7, High: 11.5, Close: 11.2}, // 当日K应被剔除
		},
		"000002": {
			{Date: "2026-08-07", Close: 10},
			{Date: "2026-08-10", Open: 9.5, High: 9.6, Close: 9.0},
		},
		"000003": {
			{Date: "2026-08-07", Close: 10},
			{Date: "2026-08-10", Open: 10.1, High: 10.3, Close: 10.2},
		},
	}

	tagged, missing := patchSelectionTags(results, daily, "2026-08-11")
	if tagged != 2 || missing != 1 {
		t.Fatalf("tagged=%d missing=%d", tagged, missing)
	}
	want := []string{"涨停破板", "前一天跌停", "", ""}
	for i, w := range want {
		if results[i].Tag != w {
			t.Fatalf("results[%d].Tag=%q want %q", i, results[i].Tag, w)
		}
	}
}

func TestRound2HandlesNegatives(t *testing.T) {
	cases := map[float64]float64{
		-6.7214: -6.72,
		-6.715:  -6.72,
		6.715:   6.72,
		9.994:   9.99,
		-0.004:  0,
	}
	for in, want := range cases {
		if got := round2(in); got != want {
			t.Fatalf("round2(%v)=%v want %v", in, got, want)
		}
	}
}

func TestT0ShortCodeFromResultCode(t *testing.T) {
	cases := map[string]string{
		"600188.XSHG": "600188",
		"000537.XSHE": "000537",
		"002194":      "002194",
	}
	for in, want := range cases {
		if got := t0ShortCodeFromResultCode(in); got != want {
			t.Fatalf("t0ShortCodeFromResultCode(%q)=%q want %q", in, got, want)
		}
	}
}
