package data

import (
	"go-stock/backend/db"
	"go-stock/backend/models"
	"time"

	"gorm.io/gorm/clause"
)

type StockChangeHistoryService struct{}

func NewStockChangeHistoryService() *StockChangeHistoryService {
	return &StockChangeHistoryService{}
}

func (s *StockChangeHistoryService) SaveStockChanges(items []StockChangeItem) error {
	if len(items) == 0 {
		return nil
	}

	today := time.Now().Format("2006-01-02")
	var histories []models.StockChangeHistory

	for _, item := range items {
		history := models.StockChangeHistory{
			ChangeTime: item.Time,
			ChangeDate: today,
			StockCode:  item.Code,
			StockName:  item.Name,
			Market:     item.Market,
			ChangeType: item.ChangeType,
			TypeName:   item.TypeName,
			Volume:     item.Volume,
			Price:      item.Price,
			ChangeRate: item.ChangeRate,
			Amount:     item.Amount,
		}
		histories = append(histories, history)
	}

	return db.Dao.Clauses(clause.OnConflict{
		Columns:   []clause.Column{{Name: "change_date"}, {Name: "stock_code"}, {Name: "change_time"}},
		DoNothing: true,
	}).CreateInBatches(histories, 100).Error
}

func (s *StockChangeHistoryService) SaveStockChangesWithDedup(items []StockChangeItem) (int, error) {
	if len(items) == 0 {
		return 0, nil
	}

	today := time.Now().Format("2006-01-02")
	var histories []models.StockChangeHistory
	for _, item := range items {
		history := models.StockChangeHistory{
			ChangeTime: item.Time,
			ChangeDate: today,
			StockCode:  item.Code,
			StockName:  item.Name,
			Market:     item.Market,
			ChangeType: item.ChangeType,
			TypeName:   item.TypeName,
			Volume:     item.Volume,
			Price:      item.Price,
			ChangeRate: item.ChangeRate,
			Amount:     item.Amount,
		}
		histories = append(histories, history)
	}

	if len(histories) == 0 {
		return 0, nil
	}

	result := db.Dao.Clauses(clause.OnConflict{
		Columns:   []clause.Column{{Name: "change_date"}, {Name: "stock_code"}, {Name: "change_time"}, {Name: "change_type"}, {Name: "price"}, {Name: "change_rate"}, {Name: "amount"}, {Name: "volume"}},
		DoNothing: true,
	}).CreateInBatches(histories, 100)

	if result.Error != nil {
		return 0, result.Error
	}

	return int(result.RowsAffected), nil
}

func (s *StockChangeHistoryService) SaveStockChange(item StockChangeItem) error {
	today := time.Now().Format("2006-01-02")
	history := models.StockChangeHistory{
		ChangeTime: item.Time,
		ChangeDate: today,
		StockCode:  item.Code,
		StockName:  item.Name,
		Market:     item.Market,
		ChangeType: item.ChangeType,
		TypeName:   item.TypeName,
		Volume:     item.Volume,
		Price:      item.Price,
		ChangeRate: item.ChangeRate,
		Amount:     item.Amount,
	}
	return db.Dao.Create(&history).Error
}

func (s *StockChangeHistoryService) GetHistoryList(query models.StockChangeHistoryQuery) (*models.StockChangeHistoryPageData, error) {
	if query.Page <= 0 {
		query.Page = 1
	}
	if query.PageSize <= 0 || query.PageSize > 100 {
		query.PageSize = 50
	}

	dbQuery := db.Dao.Model(&models.StockChangeHistory{})

	if query.StockCode != "" {
		dbQuery = dbQuery.Where("stock_code LIKE ?", "%"+query.StockCode+"%")
	}
	if query.StockName != "" {
		dbQuery = dbQuery.Where("stock_name LIKE ?", "%"+query.StockName+"%")
	}
	if query.ChangeType > 0 {
		dbQuery = dbQuery.Where("change_type = ?", query.ChangeType)
	}
	if len(query.ChangeTypes) > 0 {
		dbQuery = dbQuery.Where("change_type IN ?", query.ChangeTypes)
	}
	if query.TypeName != "" {
		dbQuery = dbQuery.Where("type_name = ?", query.TypeName)
	}
	if query.StartDate != "" {
		dbQuery = dbQuery.Where("change_date >= ?", query.StartDate)
	}
	if query.EndDate != "" {
		dbQuery = dbQuery.Where("change_date <= ?", query.EndDate)
	}

	var total int64
	if err := dbQuery.Count(&total).Error; err != nil {
		return nil, err
	}

	var list []models.StockChangeHistory
	offset := (query.Page - 1) * query.PageSize
	if err := dbQuery.Order("created_at DESC").Offset(offset).Limit(query.PageSize).Find(&list).Error; err != nil {
		return nil, err
	}

	totalPages := int(total) / query.PageSize
	if int(total)%query.PageSize > 0 {
		totalPages++
	}

	return &models.StockChangeHistoryPageData{
		List:       list,
		Total:      total,
		Page:       query.Page,
		PageSize:   query.PageSize,
		TotalPages: totalPages,
	}, nil
}

func (s *StockChangeHistoryService) DeleteOldData(days int) error {
	cutoffDate := time.Now().AddDate(0, 0, -days).Format("2006-01-02")
	return db.Dao.Where("change_date < ?", cutoffDate).Delete(&models.StockChangeHistory{}).Error
}

func (s *StockChangeHistoryService) GetStockChangeStats(startDate, endDate string) (map[string]interface{}, error) {
	dbQuery := db.Dao.Model(&models.StockChangeHistory{})
	if startDate != "" {
		dbQuery = dbQuery.Where("change_date >= ?", startDate)
	}
	if endDate != "" {
		dbQuery = dbQuery.Where("change_date <= ?", endDate)
	}

	var totalCount int64
	if err := dbQuery.Count(&totalCount).Error; err != nil {
		return nil, err
	}

	type TypeCount struct {
		TypeName string
		Count    int64
	}
	var typeCounts []TypeCount
	if err := db.Dao.Model(&models.StockChangeHistory{}).
		Select("type_name, count(*) as count").
		Where("change_date >= ? AND change_date <= ?", startDate, endDate).
		Group("type_name").
		Order("count DESC").
		Find(&typeCounts).Error; err != nil {
		return nil, err
	}

	return map[string]interface{}{
		"totalCount": totalCount,
		"typeCounts": typeCounts,
	}, nil
}
