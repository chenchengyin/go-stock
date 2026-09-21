package models

import "time"

// T0ReferenceObservation stores one source snapshot for a stock and T0 date.
// Source versions are immutable; IsActive identifies the version used by the
// current analysis run for a (trade_date, stock_code) pair.
type T0ReferenceObservation struct {
	ID            uint      `gorm:"primaryKey"`
	TradeDate     string    `gorm:"size:10;index:idx_t0_reference_date_code;uniqueIndex:idx_t0_reference_source"`
	StockCode     string    `gorm:"size:32;index:idx_t0_reference_date_code;uniqueIndex:idx_t0_reference_source"`
	T0PrevClose   float64   `gorm:"column:t0_prev_close"`
	T0Open        float64   `gorm:"column:t0_open"`
	T0High        float64   `gorm:"column:t0_high"`
	T0Low         float64   `gorm:"column:t0_low"`
	T0Close       float64   `gorm:"column:t0_close"`
	T0Volume      float64   `gorm:"column:t0_volume"`
	T0AmountYi    float64   `gorm:"column:t0_amount_yi"`
	T0OpenGap     float64   `gorm:"column:t0_open_gap"`
	EntryPrice    float64   `gorm:"column:entry_price"`
	EntryGap      float64   `gorm:"column:entry_gap"`
	EntrySource   string    `gorm:"size:32;column:entry_source"`
	T0PnL         float64   `gorm:"column:t0_pnl"`
	DataStatus    string    `gorm:"size:16;column:data_status"`
	UniverseBatch string    `gorm:"size:128;column:universe_batch"`
	SourceBatch   string    `gorm:"size:128;uniqueIndex:idx_t0_reference_source"`
	SourceHash    string    `gorm:"size:128;column:source_hash"`
	IsActive      bool      `gorm:"column:is_active;index:idx_t0_reference_active"`
	CreatedAt     time.Time `gorm:"column:created_at"`
	UpdatedAt     time.Time `gorm:"column:updated_at"`
}

func (T0ReferenceObservation) TableName() string { return "t0_reference_observations" }

type T0ReferenceBar struct {
	ID            uint      `gorm:"primaryKey"`
	ObservationID uint      `gorm:"column:observation_id;uniqueIndex:idx_t0_reference_observation_offset"`
	Offset        int       `gorm:"uniqueIndex:idx_t0_reference_observation_offset"`
	BarDate       string    `gorm:"size:10;column:bar_date"`
	PrevClose     float64   `gorm:"column:prev_close"`
	Open          float64   `gorm:"column:open"`
	High          float64   `gorm:"column:high"`
	Low           float64   `gorm:"column:low"`
	Close         float64   `gorm:"column:close"`
	Volume        float64   `gorm:"column:volume"`
	AmountYi      float64   `gorm:"column:amount_yi"`
	SourceBatch   string    `gorm:"size:128;column:source_batch"`
	CreatedAt     time.Time `gorm:"column:created_at"`
	UpdatedAt     time.Time `gorm:"column:updated_at"`
}

func (T0ReferenceBar) TableName() string { return "t0_reference_bars" }

type T0ReferenceRule struct {
	ID                uint      `gorm:"primaryKey"`
	RuleKey           string    `gorm:"size:256;uniqueIndex:idx_t0_reference_rule_key"`
	RuleKind          string    `gorm:"size:32;column:rule_kind"`
	Name              string    `gorm:"size:256"`
	LookbackStart     int       `gorm:"column:lookback_start"`
	LookbackEnd       int       `gorm:"column:lookback_end"`
	DefinitionVersion string    `gorm:"size:32;column:definition_version"`
	ConditionJSON     string    `gorm:"type:text;column:condition_json"`
	EntryJSON         string    `gorm:"type:text;column:entry_json"`
	ManualRank        int       `gorm:"column:manual_rank"`
	MinSamples        int       `gorm:"column:min_samples"`
	DeepRed           bool      `gorm:"column:deep_red"`
	Enabled           bool      `gorm:"column:enabled;index:idx_t0_reference_rule_enabled"`
	CreatedAt         time.Time `gorm:"column:created_at"`
	UpdatedAt         time.Time `gorm:"column:updated_at"`
}

func (T0ReferenceRule) TableName() string { return "t0_reference_rules" }

type T0ReferenceRuleStat struct {
	ID            uint      `gorm:"primaryKey"`
	RuleID        uint      `gorm:"column:rule_id;uniqueIndex:idx_t0_reference_stat_scope;index:idx_t0_reference_stat_rule_period"`
	BatchID       string    `gorm:"size:128;column:batch_id;uniqueIndex:idx_t0_reference_stat_scope"`
	PeriodKey     string    `gorm:"size:32;column:period_key;uniqueIndex:idx_t0_reference_stat_scope;index:idx_t0_reference_stat_rule_period"`
	DateStart     string    `gorm:"size:10;column:date_start"`
	DateEnd       string    `gorm:"size:10;column:date_end"`
	SampleCount   int       `gorm:"column:sample_count"`
	ProfitWinRate float64   `gorm:"column:profit_win_rate"`
	TargetRate    float64   `gorm:"column:target_rate"`
	LossRate      float64   `gorm:"column:loss_rate"`
	AvgPnL        float64   `gorm:"column:avg_pnl"`
	MedianPnL     float64   `gorm:"column:median_pnl"`
	ResearchTier  string    `gorm:"size:16;column:research_tier"`
	UpdatedAt     time.Time `gorm:"column:updated_at"`
}

func (T0ReferenceRuleStat) TableName() string { return "t0_reference_rule_stats" }
