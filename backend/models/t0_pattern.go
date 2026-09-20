package models

import "time"

type T0PatternStat struct {
	ID        uint      `gorm:"primaryKey"`
	Pattern   string    `gorm:"size:128;uniqueIndex:idx_pattern_window"`
	Window    int       `gorm:"uniqueIndex:idx_pattern_window"`
	T0N       int       `gorm:"column:t0_n"`
	WinRate   float64   `gorm:"column:win_rate"`
	FailRate  float64   `gorm:"column:fail_rate"`
	AvgPnL    float64   `gorm:"column:avg_pnl"`
	MedPnL    float64   `gorm:"column:med_pnl"`
	BatchID   string    `gorm:"size:64;column:batch_id"`
	UpdatedAt time.Time `gorm:"column:updated_at"`
}

func (T0PatternStat) TableName() string { return "t0_pattern_stats" }

type T0PatternConfig struct {
	ID           uint    `gorm:"primaryKey"`
	BlueMinWin   float64 `gorm:"column:blue_min_win"`
	BlueMaxFail  float64 `gorm:"column:blue_max_fail"`
	GreenMaxFail float64 `gorm:"column:green_max_fail"`
	GreenMinWin  float64 `gorm:"column:green_min_win"`
	RedMinFail   float64 `gorm:"column:red_min_fail"`
	RedMaxWin    float64 `gorm:"column:red_max_win"`
	MinSamples   int     `gorm:"column:min_samples"`
	// PurpleMinEarn is the minimum true earn rate for the purple strategy.
	// Keep the existing column name so existing databases remain compatible.
	PurpleMinEarn    float64   `gorm:"column:purple_min_win"`
	PurpleMinSamples int       `gorm:"column:purple_min_samples"`
	BatchID          string    `gorm:"size:64;column:batch_id"`
	UpdatedAt        time.Time `gorm:"column:updated_at"`
}

func (T0PatternConfig) TableName() string { return "t0_pattern_config" }

func DefaultT0PatternConfig(batchID string) T0PatternConfig {
	return T0PatternConfig{
		ID:               1,
		BlueMinWin:       45, // 全池约 26%；N≥10 中 ~5% 形态可达此档
		BlueMaxFail:      40,
		GreenMaxFail:     45,
		GreenMinWin:      30,
		RedMinFail:       52,
		RedMaxWin:        22,
		MinSamples:       10,
		PurpleMinEarn:    70,
		PurpleMinSamples: 3,
		BatchID:          batchID,
		UpdatedAt:        time.Now(),
	}
}

func (c T0PatternConfig) EffectivePurpleMinEarn() float64 {
	if c.PurpleMinEarn > 0 {
		return c.PurpleMinEarn
	}
	return DefaultT0PatternConfig("").PurpleMinEarn
}

func (c T0PatternConfig) EffectivePurpleMinSamples() int {
	if c.PurpleMinSamples > 0 {
		return c.PurpleMinSamples
	}
	return DefaultT0PatternConfig("").PurpleMinSamples
}
