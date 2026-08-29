package flutter_api

import "testing"

func TestSortT0ResultsForClient(t *testing.T) {
	in := []T0SelectionResult{
		{StockCode: "A", OpenGap: 1.0, Tag: ""},
		{StockCode: "B", OpenGap: 2.5, Tag: "涨停破板"},
		{StockCode: "C", OpenGap: 3.0, Tag: ""},
		{StockCode: "D", OpenGap: 0.5, Tag: "前一天跌停"},
		{StockCode: "E", OpenGap: 2.5, Tag: ""},
	}

	got := sortT0ResultsForClient(in)

	// 有标记的 B、D 在前，组内按 OpenGap 降序：B(2.5) 先于 D(0.5)
	// 无标记 C(3.0) > A(1.0)/E(2.5)：C、E、A
	wantOrder := []string{"B", "D", "C", "E", "A"}
	if len(got) != len(wantOrder) {
		t.Fatalf("len=%d want %d", len(got), len(wantOrder))
	}
	for i, code := range wantOrder {
		if got[i].StockCode != code {
			t.Fatalf("pos %d = %s want %s (full: %+v)", i, got[i].StockCode, code, codes(got))
		}
	}

	// 不得修改输入切片顺序
	if in[0].StockCode != "A" || in[1].StockCode != "B" {
		t.Fatalf("input mutated: %+v", codes(in))
	}
}

func TestSortT0ResultsForClientTagPriority(t *testing.T) {
	in := []T0SelectionResult{
		{StockCode: "X", OpenGap: 2.0, Tag: "前一天大阴线"},
		{StockCode: "Y", OpenGap: 2.0, Tag: "前一天跌停"},
		{StockCode: "Z", OpenGap: 2.0, Tag: "涨停破板"},
	}
	got := sortT0ResultsForClient(in)
	// 标签顺序：涨停破板 > 前一天跌停 > 前一天大阴线。
	want := []string{"Z", "Y", "X"}
	for i, code := range want {
		if got[i].StockCode != code {
			t.Fatalf("pos %d = %s want %s", i, got[i].StockCode, code)
		}
	}
}

func TestSortT0ResultsForClientBlueFirst(t *testing.T) {
	in := []T0SelectionResult{
		{StockCode: "A", OpenGap: 3.0, Tag: "涨停破板", BuySignal: BuySignalGreen},
		{StockCode: "B", OpenGap: 1.0, Tag: "", BuySignal: BuySignalBlue},
		{StockCode: "C", OpenGap: 2.5, Tag: "", BuySignal: BuySignalRed},
		{StockCode: "D", OpenGap: 0.5, Tag: "前一天跌停", BuySignal: BuySignalBlue},
	}
	got := sortT0ResultsForClient(in)
	// blue 最前；组内仍有标记优先：D(蓝+标记) 先于 B(蓝)；再 A(绿+标记)、C(红)
	want := []string{"D", "B", "A", "C"}
	for i, code := range want {
		if got[i].StockCode != code {
			t.Fatalf("pos %d = %s want %s (full %v)", i, got[i].StockCode, code, codes(got))
		}
	}
}

func TestSortT0ResultsForClientBlueThenOpenGap(t *testing.T) {
	in := []T0SelectionResult{
		{StockCode: "L", OpenGap: 1.0, BuySignal: BuySignalBlue},
		{StockCode: "H", OpenGap: 2.8, BuySignal: BuySignalBlue},
	}
	got := sortT0ResultsForClient(in)
	want := []string{"H", "L"}
	for i, code := range want {
		if got[i].StockCode != code {
			t.Fatalf("pos %d = %s want %s", i, got[i].StockCode, code)
		}
	}
}

func TestSortT0ResultsForClientSignalThenTagPriority(t *testing.T) {
	in := []T0SelectionResult{
		{StockCode: "N", OpenGap: 9.0, BuySignal: BuySignalRed},
		{StockCode: "Y", OpenGap: 0.3, Tag: "前一天大阴线", BuySignal: BuySignalRed},
		{StockCode: "O", OpenGap: 0.6, BuySignal: BuySignalOrange},
		{StockCode: "G", OpenGap: 0.2, BuySignal: BuySignalGreen},
		{StockCode: "D", OpenGap: 0.4, Tag: "前一天跌停", BuySignal: BuySignalRed},
		{StockCode: "B", OpenGap: 1.0, BuySignal: BuySignalBlue},
		{StockCode: "P", OpenGap: 0.1, Tag: "涨停破板", BuySignal: BuySignalRed},
	}

	got := sortT0ResultsForClient(in)
	want := []string{"B", "O", "G", "P", "D", "Y", "N"}
	for i, code := range want {
		if got[i].StockCode != code {
			t.Fatalf("pos %d = %s want %s (full %v)", i, got[i].StockCode, code, codes(got))
		}
	}
}

func codes(rs []T0SelectionResult) []string {
	out := make([]string, len(rs))
	for i, r := range rs {
		out[i] = r.StockCode
	}
	return out
}
