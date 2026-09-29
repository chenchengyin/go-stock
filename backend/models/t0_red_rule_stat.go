package models

import "time"

// T0RedRuleStat is a verified T+1-close research aggregate, separate from
// legacy T0-close reference statistics.
type T0RedRuleStat struct {
	ID            uint   `gorm:"primaryKey"`
	RuleKey       string `gorm:"size:128;uniqueIndex:idx_t0_red_stat_scope"`
	RuleVersion   int    `gorm:"uniqueIndex:idx_t0_red_stat_scope"`
	EntrySource   string `gorm:"size:32;uniqueIndex:idx_t0_red_stat_scope"`
	ExitHorizon   string `gorm:"size:32;uniqueIndex:idx_t0_red_stat_scope"`
	PeriodKey     string `gorm:"size:32;uniqueIndex:idx_t0_red_stat_scope"`
	DataVersion   string `gorm:"size:64;uniqueIndex:idx_t0_red_stat_scope"`
	BatchID       string `gorm:"size:128;uniqueIndex:idx_t0_red_stat_scope"`
	DateStart     string `gorm:"size:10"`
	DateEnd       string `gorm:"size:10"`
	SampleCount   int
	PositiveCount int
	TargetCount   int
	ProfitWinRate float64
	TargetRate    float64
	AvgPnL        float64 `gorm:"column:avg_pnl"`
	Status        string  `gorm:"size:16"`
	UpdatedAt     time.Time
}

func (T0RedRuleStat) TableName() string { return "t0_red_rule_stats" }
