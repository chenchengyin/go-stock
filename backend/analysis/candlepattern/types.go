package candlepattern

type BarType string

const (
	BarZT   BarType = "ZT"
	BarDT   BarType = "DT"
	BarPB   BarType = "PB"
	BarYX   BarType = "YX"
	BarYXN  BarType = "YXN"
	BarSY   BarType = "SY"
	BarXY   BarType = "XY"
	BarMY   BarType = "MY"
	BarMYIN BarType = "MYIN"
	BarDY   BarType = "DY"
	BarDYIN BarType = "DYIN"
	BarXX   BarType = "XX"
)

type DailyBar struct {
	Date     string
	Open     float64
	Close    float64
	High     float64
	Low      float64
	Volume   float64
	AmountYi float64
}

type StockMeta struct {
	Code        string
	ShortCode   string
	Name        string
	MarketCapYi float64
}

type DailyCache struct {
	TradeDate string
	Stocks    []StockMeta
	Daily     map[string][]DailyBar
}

type Observation struct {
	ShortCode  string
	Pattern    string
	Window     int
	NextYang   bool
	T0Gap      float64
	T0PnL      float64
	InT0Subset bool
}

type PatternStat struct {
	Pattern        string  `json:"pattern"`
	Window         int     `json:"window"`
	SampleCount    int     `json:"sample_count"`
	NextYangRate   float64 `json:"next_yang_rate"`
	T0SubsetCount  int     `json:"t0_subset_count"`
	T0WinRate2p5   float64 `json:"t0_win_rate_2p5"`
	T0AvgPnL       float64 `json:"t0_avg_pnl"`
	T0MedianPnL    float64 `json:"t0_median_pnl"`
	Insufficient   bool    `json:"insufficient"`
}

type HourBar struct {
	Time  string
	Open  float64
	Close float64
	High  float64
	Low   float64
}

type HourBarType string

const (
	HourZT   HourBarType = "H_ZT"
	HourDT   HourBarType = "H_DT"
	HourY    HourBarType = "H_Y"
	HourYIN  HourBarType = "H_YIN"
	HourDOJI HourBarType = "H_DOJI"
	HourXX   HourBarType = "H_XX"
)

type HourlyCrossStat struct {
	DailyPattern  string  `json:"daily_pattern"`
	HourlyPattern string  `json:"hourly_pattern"`
	Window        int     `json:"window"`
	HourlyTail    int     `json:"hourly_tail"`
	SampleCount   int     `json:"sample_count"`
	NextYangRate  float64 `json:"next_yang_rate"`
	T0SubsetCount int     `json:"t0_subset_count"`
	T0WinRate2p5  float64 `json:"t0_win_rate_2p5"`
	T0AvgPnL      float64 `json:"t0_avg_pnl"`
	Insufficient  bool    `json:"insufficient"`
}
