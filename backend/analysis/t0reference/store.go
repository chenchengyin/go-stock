package t0reference

import (
	"errors"
	"fmt"

	"go-stock/backend/models"
	"gorm.io/gorm"
)

// ObservationVersion carries the provenance needed to persist one immutable
// observation version. A slice of these can be written in one transaction for
// an efficient per-day backfill.
type ObservationVersion struct {
	Observation   Observation
	SourceBatch   string
	SourceHash    string
	UniverseBatch string
}

// PersistObservation stores one immutable source version and its T-3..T-1
// bars. Replaying the same source batch is deliberately a no-op; a different
// source batch becomes the sole active version for the date and stock.
func PersistObservation(dao *gorm.DB, obs Observation, sourceBatch, sourceHash, universeBatch string) error {
	return PersistObservations(dao, []ObservationVersion{{
		Observation: obs, SourceBatch: sourceBatch, SourceHash: sourceHash, UniverseBatch: universeBatch,
	}})
}

// PersistObservations writes a batch of immutable observation versions in one
// transaction. The caller should pass at most one version per stock/date for
// a source batch.
func PersistObservations(dao *gorm.DB, versions []ObservationVersion) error {
	if dao == nil {
		return errors.New("invalid t0 reference database")
	}
	for _, version := range versions {
		if err := validateObservationVersion(version); err != nil {
			return err
		}
	}
	if len(versions) == 0 {
		return nil
	}
	return dao.Transaction(func(tx *gorm.DB) error {
		for _, version := range versions {
			if err := persistObservationTx(tx, version); err != nil {
				return err
			}
		}
		return nil
	})
}

func validateObservationVersion(version ObservationVersion) error {
	obs := version.Observation
	if version.SourceBatch == "" || obs.TradeDate == "" || obs.StockCode == "" {
		return errors.New("invalid t0 reference persistence input")
	}
	if obs.EntryPrice <= 0 || obs.T0.PrevClose <= 0 {
		return errors.New("invalid t0 reference prices")
	}
	for _, offset := range []int{-3, -2, -1} {
		if _, ok := obs.Bars[offset]; !ok {
			return fmt.Errorf("missing reference bar offset %d", offset)
		}
	}
	return nil
}

func persistObservationTx(tx *gorm.DB, version ObservationVersion) error {
	obs := version.Observation
	var existing models.T0ReferenceObservation
	err := tx.Where("trade_date = ? AND stock_code = ? AND source_batch = ?", obs.TradeDate, obs.StockCode, version.SourceBatch).
		First(&existing).Error
	if err == nil {
		return nil
	}
	if !errors.Is(err, gorm.ErrRecordNotFound) {
		return err
	}

	if err := tx.Model(&models.T0ReferenceObservation{}).
		Where("trade_date = ? AND stock_code = ? AND is_active = ?", obs.TradeDate, obs.StockCode, true).
		Update("is_active", false).Error; err != nil {
		return err
	}

	t0OpenGap := (obs.T0.Open - obs.T0.PrevClose) / obs.T0.PrevClose * 100
	entryGap := (obs.EntryPrice - obs.T0.PrevClose) / obs.T0.PrevClose * 100
	pnl := (obs.T0.Close - obs.EntryPrice) / obs.EntryPrice * 100
	row := models.T0ReferenceObservation{
		TradeDate: obs.TradeDate, StockCode: obs.StockCode,
		T0PrevClose: obs.T0.PrevClose, T0Open: obs.T0.Open,
		T0High: obs.T0.High, T0Low: obs.T0.Low, T0Close: obs.T0.Close,
		T0Volume: obs.T0.Volume, T0AmountYi: obs.T0.AmountYi, T0OpenGap: t0OpenGap,
		EntryPrice: obs.EntryPrice, EntryGap: entryGap, EntrySource: obs.EntrySource,
		T0PnL: pnl, DataStatus: "complete", UniverseBatch: version.UniverseBatch,
		SourceBatch: version.SourceBatch, SourceHash: version.SourceHash, IsActive: true,
	}
	if err := tx.Create(&row).Error; err != nil {
		return err
	}

	bars := make([]models.T0ReferenceBar, 0, 3)
	for _, offset := range []int{-3, -2, -1} {
		bar := obs.Bars[offset]
		bars = append(bars, models.T0ReferenceBar{
			ObservationID: row.ID, Offset: offset, BarDate: bar.Date,
			PrevClose: bar.PrevClose, Open: bar.Open, High: bar.High, Low: bar.Low,
			Close: bar.Close, Volume: bar.Volume, AmountYi: bar.AmountYi,
			SourceBatch: version.SourceBatch,
		})
	}
	return tx.Create(&bars).Error
}

// LoadActiveObservations reconstructs the analysis-layer observations from
// the fact tables. It is useful for offline re-analysis and deliberately only
// returns the active, complete source version for each stock/date.
func LoadActiveObservations(dao *gorm.DB) ([]Observation, error) {
	if dao == nil {
		return nil, errors.New("invalid t0 reference database")
	}
	var rows []models.T0ReferenceObservation
	if err := dao.Where("is_active = ? AND data_status = ?", true, "complete").
		Order("trade_date, stock_code").Find(&rows).Error; err != nil {
		return nil, err
	}
	observations := make([]Observation, 0, len(rows))
	for _, row := range rows {
		var storedBars []models.T0ReferenceBar
		if err := dao.Where("observation_id = ?", row.ID).Order("offset").Find(&storedBars).Error; err != nil {
			return nil, err
		}
		if len(storedBars) < 3 {
			continue
		}
		bars := make(map[int]BarSnapshot, len(storedBars))
		for _, bar := range storedBars {
			bars[bar.Offset] = BarSnapshot{
				Date: bar.BarDate, PrevClose: bar.PrevClose, Open: bar.Open,
				High: bar.High, Low: bar.Low, Close: bar.Close,
				Volume: bar.Volume, AmountYi: bar.AmountYi,
			}
		}
		observations = append(observations, Observation{
			TradeDate: row.TradeDate, StockCode: row.StockCode, Bars: bars,
			T0: BarSnapshot{
				Date: row.TradeDate, PrevClose: row.T0PrevClose, Open: row.T0Open,
				High: row.T0High, Low: row.T0Low, Close: row.T0Close,
				Volume: row.T0Volume, AmountYi: row.T0AmountYi,
			},
			EntryPrice: row.EntryPrice, EntryGap: row.EntryGap,
			EntrySource: row.EntrySource, PnL: row.T0PnL,
		})
	}
	return observations, nil
}
